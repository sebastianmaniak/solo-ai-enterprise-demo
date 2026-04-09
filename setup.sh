#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

banner() {
  echo ""
  echo -e "${GREEN}============================================================${NC}"
  echo -e "${GREEN}  $1${NC}"
  echo -e "${GREEN}============================================================${NC}"
  echo ""
}

warn() {
  echo -e "${YELLOW}[WARN] $1${NC}"
}

fail() {
  echo -e "${RED}[ERROR] $1${NC}" >&2
  exit 1
}

# ---------------------------------------------------------------------------
# Step 0: Prerequisites
# ---------------------------------------------------------------------------
banner "Step 0: Checking prerequisites"

for tool in docker kind kubectl helm curl jq openssl; do
  if ! command -v "${tool}" &>/dev/null; then
    fail "Required tool not found: ${tool}. Please install it and re-run."
  fi
  echo -e "${GREEN}  [OK]${NC} ${tool}"
done

for var in OPENAI_API_KEY; do
  if [[ -z "${!var:-}" ]]; then
    fail "Required environment variable not set: ${var}"
  fi
  echo -e "${GREEN}  [OK]${NC} ${var}"
done

# AGENTGATEWAY_LICENSE_KEY is optional — used for kagent Enterprise licensing
if [[ -n "${AGENTGATEWAY_LICENSE_KEY:-}" ]]; then
  echo -e "${GREEN}  [OK]${NC} AGENTGATEWAY_LICENSE_KEY"
else
  AGENTGATEWAY_LICENSE_KEY=""
  warn "AGENTGATEWAY_LICENSE_KEY not set — using empty value (trial mode)."
fi

# ---------------------------------------------------------------------------
# Step 1: Create Kind cluster
# ---------------------------------------------------------------------------
banner "Step 1: Create Kind cluster"

if kind get clusters 2>/dev/null | grep -q "^solo-bank-demo$"; then
  warn "Kind cluster 'solo-bank-demo' already exists. Skipping creation."
else
  kind create cluster --name solo-bank-demo \
    --config "${SCRIPT_DIR}/kind-config.yaml" \
    --wait 60s
  echo -e "${GREEN}Kind cluster 'solo-bank-demo' created.${NC}"
fi

# ---------------------------------------------------------------------------
# Step 2: Install Gateway API CRDs (required by KMCP)
# ---------------------------------------------------------------------------
banner "Step 2: Install Gateway API CRDs"

kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.0/standard-install.yaml
echo -e "${GREEN}Gateway API CRDs installed.${NC}"

# ---------------------------------------------------------------------------
# Step 3: Create namespaces
# ---------------------------------------------------------------------------
banner "Step 3: Create namespaces"

kubectl apply -f "${SCRIPT_DIR}/manifests/namespaces.yaml"
echo -e "${GREEN}Namespaces applied.${NC}"

# ---------------------------------------------------------------------------
# Step 4: Install AgentRegistry OSS
# ---------------------------------------------------------------------------
banner "Step 4: Install AgentRegistry OSS"

JWT_KEY=$(openssl rand -hex 32)

helm upgrade --install agentregistry \
  oci://ghcr.io/agentregistry-dev/agentregistry/charts/agentregistry \
  --namespace agentregistry --create-namespace \
  --set config.jwtPrivateKey="${JWT_KEY}" \
  --set config.enableAnonymousAuth="true" \
  --set service.type=NodePort \
  --set service.nodePorts.http=30121 \
  --set database.postgres.vectorEnabled=true \
  --set database.postgres.bundled.image.repository=pgvector \
  --set database.postgres.bundled.image.name=pgvector \
  --set database.postgres.bundled.image.tag=pg16 \
  --set image.tag=v0.3.3 \
  --wait --timeout 300s

echo -e "${GREEN}AgentRegistry OSS installed.${NC}"

# ---------------------------------------------------------------------------
# Step 5: Install kagent Enterprise CRDs
# ---------------------------------------------------------------------------
banner "Step 5: Install kagent Enterprise CRDs"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent --create-namespace --version 0.3.14 --wait --timeout 120s

echo -e "${GREEN}kagent CRDs installed.${NC}"

# ---------------------------------------------------------------------------
# Step 6: Install Management UI (provides OIDC for kagent)
# ---------------------------------------------------------------------------
banner "Step 6: Install Management UI"

# --no-hooks: Management UI post-install hooks may timeout in Kind clusters;
# the core deployment works fine without them.
# service.type=NodePort: the chart defaults to LoadBalancer services, and
# Helm --wait is unreliable for this chart on Kind even after the workloads
# themselves are healthy. Install first, then wait on the concrete workloads.
helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace kagent \
  --version 0.3.14 \
  --set cluster="solo-bank-demo" \
  --set service.type=NodePort \
  --set products.kagent.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --no-hooks

echo "Waiting for Management UI workloads to be ready..."
kubectl rollout status statefulset/management-clickhouse-shard0 \
  -n kagent --timeout=300s
kubectl rollout status statefulset/solo-enterprise-telemetry-collector \
  -n kagent --timeout=300s
kubectl rollout status deployment/solo-enterprise-ui \
  -n kagent --timeout=300s

# Expose Management UI on NodePort 30090 (mapped in kind-config.yaml)
echo "Patching Management UI service to NodePort 30090..."
kubectl patch svc solo-enterprise-ui -n kagent --type='json' -p='[
  {"op":"replace","path":"/spec/type","value":"NodePort"},
  {"op":"replace","path":"/spec/ports/2/nodePort","value":30090}
]' 2>/dev/null || warn "Could not patch Management UI NodePort (may already be set)."

echo -e "${GREEN}Management UI installed.${NC}"

# ---------------------------------------------------------------------------
# Step 7: Install kagent Enterprise
# ---------------------------------------------------------------------------
banner "Step 7: Install kagent Enterprise"

# oidc.skipOBO=true: Skip On-Behalf-Of token generation since we don't have
# a full OIDC IdP configured. Without this, agents fail with
# "obo token handler not ready" at runtime.
helm upgrade --install kagent-enterprise \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise \
  --namespace kagent --version 0.3.14 \
  --set defaultModelConfig.provider=OpenAI \
  --set defaultModelConfig.model=gpt-4o-mini \
  --set controller.enabled=true \
  --set kmcp.enabled=true \
  --set kagent-tools.enabled=true \
  --set oidc.skipOBO=true \
  --set otel.tracing.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo -e "${GREEN}kagent Enterprise installed.${NC}"

# Fix OTEL endpoint: the chart omits the http:// scheme prefix, which
# causes gRPC resolver errors. Override the env var directly.
echo "Fixing OTEL tracing endpoint format..."
kubectl set env deployment/kagent-controller -n kagent \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://solo-enterprise-telemetry-collector.kagent.svc.cluster.local:4317
kubectl rollout status deployment/kagent-controller -n kagent --timeout=120s

# Expose kagent controller on NodePort 30083 (mapped in kind-config.yaml)
echo "Patching kagent controller service to NodePort 30083..."
kubectl patch svc kagent-controller -n kagent --type='json' -p='[
  {"op":"replace","path":"/spec/type","value":"NodePort"},
  {"op":"add","path":"/spec/ports/0/nodePort","value":30083}
]' 2>/dev/null || warn "Could not patch kagent controller NodePort (may already be set)."

# ---------------------------------------------------------------------------
# Step 8: Create LLM API key secrets
# ---------------------------------------------------------------------------
banner "Step 8: Create LLM API key secrets"

kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}LLM API key secrets created.${NC}"

# ---------------------------------------------------------------------------
# Step 9: Build and load Docker images
# ---------------------------------------------------------------------------
banner "Step 9: Build and load Docker images"

echo "Building bank-wiki-server..."
docker build -t bank-wiki-server:latest "${SCRIPT_DIR}/wiki-server/"

echo "Building bank-customer-tools..."
docker build -t bank-customer-tools:latest \
  -f "${SCRIPT_DIR}/mcp-tools/customer-tools/Dockerfile" \
  "${SCRIPT_DIR}/mcp-tools/"

echo "Building bank-policy-tools..."
docker build -t bank-policy-tools:latest \
  -f "${SCRIPT_DIR}/mcp-tools/policy-tools/Dockerfile" \
  "${SCRIPT_DIR}/mcp-tools/"

echo "Building bank-transaction-tools..."
docker build -t bank-transaction-tools:latest \
  -f "${SCRIPT_DIR}/mcp-tools/transaction-tools/Dockerfile" \
  "${SCRIPT_DIR}/mcp-tools/"

echo "Building bank-status-tools..."
docker build -t bank-status-tools:latest \
  -f "${SCRIPT_DIR}/mcp-tools/status-tools/Dockerfile" \
  "${SCRIPT_DIR}/mcp-tools/"

echo "Building bank-incident-tools..."
docker build -t bank-incident-tools:latest \
  -f "${SCRIPT_DIR}/mcp-tools/incident-tools/Dockerfile" \
  "${SCRIPT_DIR}/mcp-tools/"

if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  echo "Building bank-docs-site..."
  docker build -t bank-docs-site:latest "${SCRIPT_DIR}/docs-site/"
fi

echo "Loading images into Kind cluster..."
kind load docker-image bank-wiki-server:latest --name solo-bank-demo
kind load docker-image bank-customer-tools:latest --name solo-bank-demo
kind load docker-image bank-policy-tools:latest --name solo-bank-demo
kind load docker-image bank-transaction-tools:latest --name solo-bank-demo
kind load docker-image bank-status-tools:latest --name solo-bank-demo
kind load docker-image bank-incident-tools:latest --name solo-bank-demo

if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  kind load docker-image bank-docs-site:latest --name solo-bank-demo
fi

echo -e "${GREEN}Docker images built and loaded.${NC}"

# ---------------------------------------------------------------------------
# Step 10: Deploy bank wiki and tool servers
# ---------------------------------------------------------------------------
banner "Step 10: Deploy bank wiki and tool servers"

kubectl apply -f "${SCRIPT_DIR}/manifests/bank-wiki/"

echo "Waiting for bank-wiki-server pods..."
kubectl wait --for=condition=Ready pod -l app=bank-wiki-server \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-customer-tools pods..."
kubectl wait --for=condition=Ready pod -l app=bank-customer-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-policy-tools pods..."
kubectl wait --for=condition=Ready pod -l app=bank-policy-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-transaction-tools pods..."
kubectl wait --for=condition=Ready pod -l app=bank-transaction-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-status-tools pods..."
kubectl wait --for=condition=Ready pod -l app=bank-status-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-incident-tools pods..."
kubectl wait --for=condition=Ready pod -l app=bank-incident-tools \
  -n bank-wiki --timeout=120s

echo -e "${GREEN}Bank wiki and tool servers deployed.${NC}"

# ---------------------------------------------------------------------------
# Step 11: Apply MCP servers, model configs, and agents
# ---------------------------------------------------------------------------
banner "Step 11: Apply MCP servers, model configs, and agents"

kubectl apply -f "${SCRIPT_DIR}/manifests/mcp/"
kubectl apply -f "${SCRIPT_DIR}/manifests/agents/"

echo "Waiting for agents to be ready..."
sleep 5
kubectl get agents -n kagent 2>/dev/null || true

echo -e "${GREEN}MCP servers, model configs, and agents applied.${NC}"

# ---------------------------------------------------------------------------
# Step 12: Populate AgentRegistry catalog
# ---------------------------------------------------------------------------
banner "Step 12: Populate AgentRegistry catalog"

AR_URL="http://localhost:8080"
AR_POD=$(kubectl get pod -l app.kubernetes.io/name=agentregistry -n agentregistry \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "${AR_POD}" ]]; then
  echo "Registering MCP servers, skills, and agents in AgentRegistry..."

  # Get anonymous auth token
  AR_TOKEN=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- --post-data='{}' --header="Content-Type: application/json" \
    "${AR_URL}/v0/auth/none" 2>/dev/null | jq -r '.registry_token' || true)

  if [[ -n "${AR_TOKEN}" && "${AR_TOKEN}" != "null" ]]; then
    AUTH_HEADER="Authorization: Bearer ${AR_TOKEN}"

    # Register MCP servers
    for svr_data in \
      '{"$schema":"https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json","name":"io.modelcontextprotocol.anonymous/bank-customer-tools","title":"Solo Bank Customer Tools","description":"Customer data tools — lookup, search, account balances","version":"1.0.0"}' \
      '{"$schema":"https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json","name":"io.modelcontextprotocol.anonymous/bank-policy-tools","title":"Solo Bank Policy Tools","description":"Policy and rates tools — lending policies, rate tables, credit tiers","version":"1.0.0"}' \
      '{"$schema":"https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json","name":"io.modelcontextprotocol.anonymous/bank-transaction-tools","title":"Solo Bank Transaction Tools","description":"Transaction and account tools — history, details, search","version":"1.0.0"}' \
      '{"$schema":"https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json","name":"io.modelcontextprotocol.anonymous/bank-status-tools","title":"Solo Bank Status Tools","description":"Application status monitoring and datacenter health tools","version":"1.0.0"}' \
      '{"$schema":"https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json","name":"io.modelcontextprotocol.anonymous/bank-incident-tools","title":"Solo Bank Incident Tools","description":"Incident management and IT ticketing tools","version":"1.0.0"}'; do
      kubectl exec "${AR_POD}" -n agentregistry -- \
        wget -qO- --post-data="${svr_data}" \
        --header="Content-Type: application/json" \
        --header="${AUTH_HEADER}" \
        "${AR_URL}/v0/servers" >/dev/null 2>&1 || true
    done
    echo -e "${GREEN}  [OK]${NC} MCP servers registered"

    # Register skills
    for skill_data in \
      '{"name":"customer-lookup","title":"Customer Lookup","description":"Look up customer profiles by name or ID and search across the customer database","version":"1.0.0","category":"banking"}' \
      '{"name":"policy-compliance-check","title":"Policy Compliance Check","description":"Check bank policies, lending rules, KYC/AML requirements, and rate schedules","version":"1.0.0","category":"compliance"}' \
      '{"name":"transaction-analysis","title":"Transaction Analysis","description":"Analyze transaction history, search for suspicious patterns, and review account details","version":"1.0.0","category":"banking"}' \
      '{"name":"mortgage-rate-quote","title":"Mortgage Rate Quote","description":"Generate personalized mortgage rate quotes based on credit score and salary","version":"1.0.0","category":"lending"}' \
      '{"name":"k8s-operations","title":"Kubernetes Operations","description":"Cluster health monitoring, pod troubleshooting, service connectivity diagnostics","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"helm-deployment","title":"Helm Deployment Management","description":"Helm release management, upgrades, rollbacks, and chart configuration","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"it-support","title":"IT Support","description":"Internal IT ticket handling, system troubleshooting, and cross-team coordination","version":"1.0.0","category":"operations"}' \
      '{"name":"infra-support","title":"Infrastructure Support","description":"Multi-domain infrastructure coordination across Kubernetes, Helm, and IT systems","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"app-monitoring","title":"Application Monitoring","description":"Monitor health and performance of core banking applications","version":"1.0.0","category":"operations"}' \
      '{"name":"datacenter-health","title":"Datacenter Health","description":"Check status of all Solo Bank datacenters and infrastructure","version":"1.0.0","category":"operations"}' \
      '{"name":"incident-tracking","title":"Incident Tracking","description":"Track and manage active incidents across all banking systems","version":"1.0.0","category":"operations"}' \
      '{"name":"ticket-management","title":"Ticket Management","description":"Track IT tickets, access requests, and change management","version":"1.0.0","category":"operations"}' \
      '{"name":"ops-center","title":"Operations Center","description":"Unified operations coordination across monitoring, incidents, and IT support","version":"1.0.0","category":"operations"}'; do
      kubectl exec "${AR_POD}" -n agentregistry -- \
        wget -qO- --post-data="${skill_data}" \
        --header="Content-Type: application/json" \
        --header="${AUTH_HEADER}" \
        "${AR_URL}/v0/skills" >/dev/null 2>&1 || true
    done
    echo -e "${GREEN}  [OK]${NC} Skills registered"

    # Register agents
    for agent_data in \
      '{"name":"bank-triage-agent","title":"Solo Bank Triage Agent","description":"Front-door triage agent that routes customer inquiries to the appropriate specialist","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-customer-service-agent","title":"Solo Bank Customer Service Agent","description":"Handles account inquiries, balance checks, and transaction questions","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o-mini","skills":[{"name":"customer-lookup","registrySkillName":"customer-lookup"},{"name":"transaction-analysis","registrySkillName":"transaction-analysis"}]}' \
      '{"name":"bank-mortgage-advisor-agent","title":"Solo Bank Mortgage Advisor Agent","description":"Provides personalized mortgage rate quotes, refinancing guidance, and lending requirements","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"customer-lookup","registrySkillName":"customer-lookup"},{"name":"mortgage-rate-quote","registrySkillName":"mortgage-rate-quote"}]}' \
      '{"name":"bank-compliance-agent","title":"Solo Bank Compliance Agent","description":"Internal compliance officer for policy audits, fraud review, and regulatory checks","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"customer-lookup","registrySkillName":"customer-lookup"},{"name":"policy-compliance-check","registrySkillName":"policy-compliance-check"},{"name":"transaction-analysis","registrySkillName":"transaction-analysis"}]}' \
      '{"name":"bank-k8s-agent","title":"Solo Bank Kubernetes Agent","description":"Infrastructure operations specialist for cluster health monitoring and troubleshooting","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"k8s-operations","registrySkillName":"k8s-operations"}]}' \
      '{"name":"bank-helm-agent","title":"Solo Bank Helm Agent","description":"Helm deployment specialist for release management, upgrades, and configuration","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o-mini","skills":[{"name":"helm-deployment","registrySkillName":"helm-deployment"}]}' \
      '{"name":"bank-it-agent","title":"Solo Bank IT Support Agent","description":"IT support lead for ticket handling, system troubleshooting, and cross-team coordination","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"customer-lookup","registrySkillName":"customer-lookup"},{"name":"transaction-analysis","registrySkillName":"transaction-analysis"},{"name":"it-support","registrySkillName":"it-support"}]}' \
      '{"name":"bank-infra-support-agent","title":"Solo Bank Infrastructure Support Agent","description":"Multi-domain coordinator for Kubernetes, Helm, and IT operations — diagnoses cross-layer issues","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"k8s-operations","registrySkillName":"k8s-operations"},{"name":"helm-deployment","registrySkillName":"helm-deployment"},{"name":"it-support","registrySkillName":"it-support"},{"name":"infra-support","registrySkillName":"infra-support"}]}' \
      '{"name":"bank-ops-agent","title":"Solo Bank Operations Monitor Agent","description":"Real-time monitoring of banking applications, datacenter health, and system-wide metrics","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"app-monitoring","registrySkillName":"app-monitoring"},{"name":"datacenter-health","registrySkillName":"datacenter-health"}]}' \
      '{"name":"bank-incident-agent","title":"Solo Bank Incident Manager Agent","description":"Tracks incidents, manages IT tickets, and coordinates incident response across all banking systems","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"incident-tracking","registrySkillName":"incident-tracking"},{"name":"ticket-management","registrySkillName":"ticket-management"}]}' \
      '{"name":"bank-ops-center-agent","title":"Solo Bank Operations Center Agent","description":"Multi-agent coordinator combining system monitoring, incident management, and IT support","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"ops-center","registrySkillName":"ops-center"},{"name":"app-monitoring","registrySkillName":"app-monitoring"},{"name":"incident-tracking","registrySkillName":"incident-tracking"}]}'; do
      kubectl exec "${AR_POD}" -n agentregistry -- \
        wget -qO- --post-data="${agent_data}" \
        --header="Content-Type: application/json" \
        --header="${AUTH_HEADER}" \
        "${AR_URL}/v0/agents" >/dev/null 2>&1 || true
    done
    echo -e "${GREEN}  [OK]${NC} Agents registered"
  else
    warn "Could not get AgentRegistry auth token. Skipping catalog population."
  fi
else
  warn "AgentRegistry pod not found. Skipping catalog population."
fi

echo -e "${GREEN}AgentRegistry catalog populated.${NC}"

# ---------------------------------------------------------------------------
# Step 13: Smoke tests
# ---------------------------------------------------------------------------
banner "Step 13: Smoke tests"

echo "Checking wiki server health..."
WIKI_POD=$(kubectl get pod -l app=bank-wiki-server -n bank-wiki \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "${WIKI_POD}" ]]; then
  if kubectl exec "${WIKI_POD}" -n bank-wiki -- \
      wget -qO- http://localhost:8080/health 2>/dev/null | grep -qi "ok\|healthy\|200\|up"; then
    echo -e "${GREEN}  [OK]${NC} Wiki server health check passed."
  else
    warn "Wiki server health check returned unexpected response."
  fi
else
  warn "Could not find wiki server pod for health check."
fi

echo "Checking agents..."
AGENT_COUNT=$(kubectl get agents -n kagent --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "${AGENT_COUNT}" -gt 0 ]]; then
  echo -e "${GREEN}  [OK]${NC} Found ${AGENT_COUNT} agent(s) in namespace 'kagent'."
  kubectl get agents -n kagent 2>/dev/null || true
else
  warn "No agents found in namespace 'kagent'. They may still be initializing."
fi

echo "Checking RemoteMCPServers..."
MCP_COUNT=$(kubectl get remotemcpservers -n kagent --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "${MCP_COUNT}" -gt 0 ]]; then
  echo -e "${GREEN}  [OK]${NC} Found ${MCP_COUNT} RemoteMCPServer(s) in namespace 'kagent'."
else
  warn "No RemoteMCPServers found. KMCP may still be initializing."
fi

echo "Checking AgentRegistry catalog..."
if [[ -n "${AR_POD:-}" ]]; then
  AR_AGENTS=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- "${AR_URL}/v0/agents" 2>/dev/null | jq '.metadata.count' 2>/dev/null || echo "0")
  AR_SKILLS=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- "${AR_URL}/v0/skills" 2>/dev/null | jq '.metadata.count' 2>/dev/null || echo "0")
  AR_SERVERS=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- "${AR_URL}/v0/servers" 2>/dev/null | jq '.metadata.count' 2>/dev/null || echo "0")
  echo -e "${GREEN}  [OK]${NC} AgentRegistry: ${AR_AGENTS} agents, ${AR_SKILLS} skills, ${AR_SERVERS} MCP servers"
fi

# ---------------------------------------------------------------------------
# Step 14: Access Information
# ---------------------------------------------------------------------------
banner "Step 14: Access Information"

echo -e "${GREEN}Solo Bank Demo is deployed!${NC}"
echo ""
echo "All services are available — no port-forwarding needed:"
echo ""
echo "  Management UI:     http://localhost:30090"
echo "  Bank Wiki Server:  http://localhost:30400"
echo "  AgentRegistry UI:  http://localhost:30121"
echo "  kagent API:        http://localhost:30083"
if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  echo "  Docs Site:         http://localhost:30500"
fi
echo ""
echo -e "${GREEN}Start here: ${NC}http://localhost:30500"
echo ""
echo -e "${GREEN}Setup complete!${NC}"
