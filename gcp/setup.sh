#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Load config
source "${SCRIPT_DIR}/config.env"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

banner() {
  echo ""
  echo -e "${GREEN}============================================================${NC}"
  echo -e "${GREEN}  $1${NC}"
  echo -e "${GREEN}============================================================${NC}"
  echo ""
}

warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
fail() { echo -e "${RED}[ERROR] $1${NC}" >&2; exit 1; }
ok()   { echo -e "${GREEN}  [OK]${NC} $1"; }

# ---------------------------------------------------------------------------
# Step 0: Prerequisites
# ---------------------------------------------------------------------------
banner "Step 0: Checking prerequisites"

for tool in docker gcloud kubectl helm curl jq openssl; do
  if ! command -v "${tool}" &>/dev/null; then
    fail "Required tool not found: ${tool}. Please install it and re-run."
  fi
  ok "${tool}"
done

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  fail "Required environment variable not set: OPENAI_API_KEY"
fi
ok "OPENAI_API_KEY"

if [[ -n "${AGENTGATEWAY_LICENSE_KEY:-}" ]]; then
  ok "AGENTGATEWAY_LICENSE_KEY"
else
  AGENTGATEWAY_LICENSE_KEY=""
  warn "AGENTGATEWAY_LICENSE_KEY not set — using empty value (trial mode)."
fi

# Verify GCP project
CURRENT_PROJECT=$(gcloud config get-value project 2>/dev/null)
if [[ "${CURRENT_PROJECT}" != "${GCP_PROJECT}" ]]; then
  echo "Setting GCP project to ${GCP_PROJECT}..."
  gcloud config set project "${GCP_PROJECT}"
fi
ok "GCP project: ${GCP_PROJECT}"

# ---------------------------------------------------------------------------
# Step 1: Enable required GCP APIs
# ---------------------------------------------------------------------------
banner "Step 1: Enable GCP APIs"

APIS=(
  container.googleapis.com
  artifactregistry.googleapis.com
  compute.googleapis.com
  dns.googleapis.com
)
for api in "${APIS[@]}"; do
  gcloud services enable "${api}" --quiet
  ok "${api}"
done

# ---------------------------------------------------------------------------
# Step 2: Create Artifact Registry repository
# ---------------------------------------------------------------------------
banner "Step 2: Create Artifact Registry"

if gcloud artifacts repositories describe "${AR_REPO}" \
    --location="${GCP_REGION}" &>/dev/null; then
  warn "Artifact Registry repo '${AR_REPO}' already exists."
else
  gcloud artifacts repositories create "${AR_REPO}" \
    --repository-format=docker \
    --location="${GCP_REGION}" \
    --description="Solo Bank Demo container images"
  ok "Created ${AR_REPO}"
fi

# Configure Docker auth for Artifact Registry
gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev" --quiet
ok "Docker auth configured for Artifact Registry"

# ---------------------------------------------------------------------------
# Step 3: Build and push Docker images
# ---------------------------------------------------------------------------
banner "Step 3: Build and push Docker images"

IMAGE_PREFIX="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT}/${AR_REPO}"

# Build and push standard images
for img_spec in \
  "bank-wiki-server:${PROJECT_DIR}/wiki-server/" \
  "bank-docs-site:${PROJECT_DIR}/docs-site/"; do
  img="${img_spec%%:*}"
  ctx="${img_spec#*:}"
  echo "Building ${img}..."
  docker build -t "${IMAGE_PREFIX}/${img}:latest" "${ctx}"
  docker push "${IMAGE_PREFIX}/${img}:latest"
  ok "${img} pushed"
done

# Build and push MCP tool images (use shared context with specific Dockerfile)
for tool in customer-tools policy-tools transaction-tools status-tools incident-tools; do
  img="bank-${tool}"
  echo "Building ${img}..."
  docker build -t "${IMAGE_PREFIX}/${img}:latest" \
    -f "${PROJECT_DIR}/mcp-tools/${tool}/Dockerfile" "${PROJECT_DIR}/mcp-tools/"
  docker push "${IMAGE_PREFIX}/${img}:latest"
  ok "${img} pushed"
done

# ---------------------------------------------------------------------------
# Step 4: Create GKE cluster
# ---------------------------------------------------------------------------
banner "Step 4: Create GKE cluster"

if gcloud container clusters describe "${GKE_CLUSTER}" \
    --zone="${GCP_ZONE}" &>/dev/null 2>&1; then
  warn "GKE cluster '${GKE_CLUSTER}' already exists."
else
  gcloud container clusters create "${GKE_CLUSTER}" \
    --zone="${GCP_ZONE}" \
    --machine-type="${GKE_MACHINE_TYPE}" \
    --num-nodes="${GKE_NODE_COUNT}" \
    --enable-ip-alias \
    --release-channel=regular \
    --workload-pool="${GCP_PROJECT}.svc.id.goog" \
    --quiet
  ok "GKE cluster '${GKE_CLUSTER}' created"
fi

# Get credentials
gcloud container clusters get-credentials "${GKE_CLUSTER}" \
  --zone="${GCP_ZONE}"
ok "kubectl configured for GKE cluster"

# ---------------------------------------------------------------------------
# Step 5: Reserve static IP for Ingress
# ---------------------------------------------------------------------------
banner "Step 5: Reserve static IP"

if gcloud compute addresses describe solo-bank-demo-ip --global &>/dev/null 2>&1; then
  warn "Static IP 'solo-bank-demo-ip' already exists."
else
  gcloud compute addresses create solo-bank-demo-ip --global
  ok "Static IP reserved"
fi

STATIC_IP=$(gcloud compute addresses describe solo-bank-demo-ip \
  --global --format="value(address)")
echo -e "${GREEN}  Static IP: ${STATIC_IP}${NC}"

# ---------------------------------------------------------------------------
# Step 6: Install Gateway API CRDs
# ---------------------------------------------------------------------------
banner "Step 6: Install Gateway API CRDs"

kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.0/standard-install.yaml
ok "Gateway API CRDs installed"

# ---------------------------------------------------------------------------
# Step 7: Create namespaces
# ---------------------------------------------------------------------------
banner "Step 7: Create namespaces"

kubectl apply -f "${PROJECT_DIR}/manifests/namespaces.yaml"
ok "Namespaces applied"

# ---------------------------------------------------------------------------
# Step 8: Install AgentRegistry OSS
# ---------------------------------------------------------------------------
banner "Step 8: Install AgentRegistry OSS"

JWT_KEY=$(openssl rand -hex 32)

helm upgrade --install agentregistry \
  oci://ghcr.io/agentregistry-dev/agentregistry/charts/agentregistry \
  --namespace agentregistry --create-namespace \
  --set config.jwtPrivateKey="${JWT_KEY}" \
  --set config.enableAnonymousAuth="true" \
  --set database.postgres.vectorEnabled=true \
  --set database.postgres.bundled.image.repository=pgvector \
  --set database.postgres.bundled.image.name=pgvector \
  --set database.postgres.bundled.image.tag=pg16 \
  --set image.tag=v0.3.3 \
  --wait --timeout 300s

ok "AgentRegistry installed"

# ---------------------------------------------------------------------------
# Step 9: Install kagent Enterprise CRDs
# ---------------------------------------------------------------------------
banner "Step 9: Install kagent Enterprise CRDs"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent --create-namespace --version 0.3.14 --wait --timeout 120s

ok "kagent CRDs installed"

# ---------------------------------------------------------------------------
# Step 10: Install Management UI
# ---------------------------------------------------------------------------
banner "Step 10: Install Management UI"

helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace kagent \
  --version 0.3.14 \
  --set cluster="${GKE_CLUSTER}" \
  --set products.kagent.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --no-hooks --wait --timeout 300s

echo "Waiting for Management UI pods..."
kubectl wait --for=condition=Available deployment/solo-enterprise-ui \
  -n kagent --timeout=180s 2>/dev/null || \
  warn "Management UI deployment not ready yet."

ok "Management UI installed"

# ---------------------------------------------------------------------------
# Step 11: Install kagent Enterprise
# ---------------------------------------------------------------------------
banner "Step 11: Install kagent Enterprise"

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

ok "kagent Enterprise installed"

# Fix OTEL endpoint
echo "Fixing OTEL tracing endpoint format..."
kubectl set env deployment/kagent-controller -n kagent \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://solo-enterprise-telemetry-collector.kagent.svc.cluster.local:4317
kubectl rollout status deployment/kagent-controller -n kagent --timeout=120s

# ---------------------------------------------------------------------------
# Step 12: Create LLM API key secrets
# ---------------------------------------------------------------------------
banner "Step 12: Create LLM API key secrets"

kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

ok "OpenAI secret created"

# ---------------------------------------------------------------------------
# Step 13: Deploy bank wiki and MCP tool servers
# ---------------------------------------------------------------------------
banner "Step 13: Deploy bank wiki and MCP tool servers"

# Apply manifests with image paths patched for Artifact Registry
# Also strip NodePort settings (GKE uses Ingress instead)
for manifest in "${PROJECT_DIR}"/manifests/bank-wiki/*.yaml; do
  sed \
    -e "s|image: bank-wiki-server:latest|image: ${IMAGE_PREFIX}/bank-wiki-server:latest|g" \
    -e "s|image: bank-customer-tools:latest|image: ${IMAGE_PREFIX}/bank-customer-tools:latest|g" \
    -e "s|image: bank-policy-tools:latest|image: ${IMAGE_PREFIX}/bank-policy-tools:latest|g" \
    -e "s|image: bank-transaction-tools:latest|image: ${IMAGE_PREFIX}/bank-transaction-tools:latest|g" \
    -e "s|image: bank-status-tools:latest|image: ${IMAGE_PREFIX}/bank-status-tools:latest|g" \
    -e "s|image: bank-incident-tools:latest|image: ${IMAGE_PREFIX}/bank-incident-tools:latest|g" \
    -e "s|image: bank-docs-site:latest|image: ${IMAGE_PREFIX}/bank-docs-site:latest|g" \
    -e "s|imagePullPolicy: Never|imagePullPolicy: Always|g" \
    -e "s|type: NodePort|type: ClusterIP|g" \
    -e "/nodePort:/d" \
    "${manifest}" | kubectl apply -f -
done

echo "Waiting for pods..."
for app in bank-wiki-server bank-customer-tools bank-policy-tools \
           bank-transaction-tools bank-status-tools bank-incident-tools; do
  kubectl wait --for=condition=Ready pod -l "app=${app}" \
    -n bank-wiki --timeout=120s
  ok "${app} ready"
done

# Deploy docs-site with patched image
if [ -f "${PROJECT_DIR}/manifests/bank-wiki/docs-site.yaml" ]; then
  # Already handled above, just wait
  kubectl wait --for=condition=Ready pod -l app=bank-docs-site \
    -n bank-wiki --timeout=120s 2>/dev/null || \
    warn "docs-site pod not ready yet"
  ok "bank-docs-site ready"
fi

# ---------------------------------------------------------------------------
# Step 14: Apply MCP servers, model configs, and agents
# ---------------------------------------------------------------------------
banner "Step 14: Apply MCP servers, model configs, and agents"

kubectl apply -f "${PROJECT_DIR}/manifests/mcp/"
kubectl apply -f "${PROJECT_DIR}/manifests/agents/"

echo "Waiting for agents to be ready..."
sleep 10
kubectl get agents -n kagent 2>/dev/null || true

ok "MCP servers and agents applied"

# ---------------------------------------------------------------------------
# Step 15: Configure Ingress
# ---------------------------------------------------------------------------
banner "Step 15: Configure Ingress"

# Apply ingress manifests
kubectl apply -f "${SCRIPT_DIR}/manifests/ingress.yaml"

ok "Ingress configured"
echo ""
echo "  Google-managed SSL certificates will take 10-30 minutes to provision."
echo "  Check status with: kubectl get managedcertificate -A"

# ---------------------------------------------------------------------------
# Step 16: Populate AgentRegistry
# ---------------------------------------------------------------------------
banner "Step 16: Populate AgentRegistry catalog"

AR_URL="http://localhost:8080"
AR_POD=$(kubectl get pod -l app.kubernetes.io/name=agentregistry -n agentregistry \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "${AR_POD}" ]]; then
  echo "Registering MCP servers, skills, and agents..."

  AR_TOKEN=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- "${AR_URL}/v0/auth/none" \
    --post-data='{}' --header="Content-Type: application/json" 2>/dev/null \
    | jq -r '.registry_token' || true)

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
    ok "MCP servers registered"

    # Register skills
    for skill_data in \
      '{"name":"customer-lookup","title":"Customer Lookup","description":"Look up customer profiles by name or ID","version":"1.0.0","category":"banking"}' \
      '{"name":"policy-compliance-check","title":"Policy Compliance","description":"Check bank policies and lending rules","version":"1.0.0","category":"compliance"}' \
      '{"name":"transaction-analysis","title":"Transaction Analysis","description":"Analyze transaction history and patterns","version":"1.0.0","category":"banking"}' \
      '{"name":"mortgage-rate-quote","title":"Mortgage Rate Quote","description":"Generate personalized mortgage rate quotes","version":"1.0.0","category":"lending"}' \
      '{"name":"k8s-operations","title":"Kubernetes Operations","description":"Cluster health and troubleshooting","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"helm-deployment","title":"Helm Deployment","description":"Helm release management","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"it-support","title":"IT Support","description":"IT ticket handling and troubleshooting","version":"1.0.0","category":"operations"}' \
      '{"name":"infra-support","title":"Infrastructure Support","description":"Multi-domain infrastructure coordination","version":"1.0.0","category":"infrastructure"}' \
      '{"name":"app-monitoring","title":"App Monitoring","description":"Monitor banking application health","version":"1.0.0","category":"operations"}' \
      '{"name":"datacenter-health","title":"Datacenter Health","description":"Check datacenter status across regions","version":"1.0.0","category":"operations"}' \
      '{"name":"incident-tracking","title":"Incident Tracking","description":"Track active incidents","version":"1.0.0","category":"operations"}' \
      '{"name":"ticket-management","title":"Ticket Management","description":"Track IT tickets","version":"1.0.0","category":"operations"}' \
      '{"name":"ops-center","title":"Operations Center","description":"Unified operations coordination","version":"1.0.0","category":"operations"}'; do
      kubectl exec "${AR_POD}" -n agentregistry -- \
        wget -qO- --post-data="${skill_data}" \
        --header="Content-Type: application/json" \
        --header="${AUTH_HEADER}" \
        "${AR_URL}/v0/skills" >/dev/null 2>&1 || true
    done
    ok "Skills registered"

    # Register agents
    for agent_data in \
      '{"name":"bank-triage-agent","title":"Triage Agent","description":"Routes inquiries to the right specialist","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-customer-service-agent","title":"Customer Service Agent","description":"Account inquiries and balance checks","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o-mini"}' \
      '{"name":"bank-mortgage-advisor-agent","title":"Mortgage Advisor Agent","description":"Mortgage rate quotes and lending guidance","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-compliance-agent","title":"Compliance Agent","description":"Policy audits and regulatory reviews","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-k8s-agent","title":"Kubernetes Agent","description":"Cluster health and pod diagnostics","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-helm-agent","title":"Helm Agent","description":"Helm release management and upgrades","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o-mini"}' \
      '{"name":"bank-it-agent","title":"IT Support Agent","description":"IT troubleshooting and cross-team coordination","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-infra-support-agent","title":"Infrastructure Support Agent","description":"Multi-domain coordinator for K8s, Helm, and IT","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-ops-agent","title":"Operations Monitor Agent","description":"Real-time app and datacenter monitoring","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-incident-agent","title":"Incident Manager Agent","description":"Incident tracking and IT ticket management","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}' \
      '{"name":"bank-ops-center-agent","title":"Operations Center Agent","description":"Multi-agent coordinator for ops, incidents, and IT","version":"1.0.0","image":"kagent","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o"}'; do
      kubectl exec "${AR_POD}" -n agentregistry -- \
        wget -qO- --post-data="${agent_data}" \
        --header="Content-Type: application/json" \
        --header="${AUTH_HEADER}" \
        "${AR_URL}/v0/agents" >/dev/null 2>&1 || true
    done
    ok "Agents registered"
  else
    warn "Could not get AgentRegistry auth token."
  fi
else
  warn "AgentRegistry pod not found."
fi

# ---------------------------------------------------------------------------
# Step 17: Smoke tests
# ---------------------------------------------------------------------------
banner "Step 17: Smoke tests"

AGENT_COUNT=$(kubectl get agents -n kagent --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "${AGENT_COUNT}" -gt 0 ]]; then
  ok "Found ${AGENT_COUNT} agent(s)"
  kubectl get agents -n kagent 2>/dev/null || true
else
  warn "No agents found yet."
fi

MCP_COUNT=$(kubectl get remotemcpservers -n kagent --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "${MCP_COUNT}" -gt 0 ]]; then
  ok "Found ${MCP_COUNT} RemoteMCPServer(s)"
else
  warn "No RemoteMCPServers found."
fi

# ---------------------------------------------------------------------------
# Step 18: DNS Instructions & Access Info
# ---------------------------------------------------------------------------
banner "Step 18: Access Information"

STATIC_IP=$(gcloud compute addresses describe solo-bank-demo-ip \
  --global --format="value(address)" 2>/dev/null || echo "<pending>")

echo -e "${GREEN}Solo Bank Demo is deployed on GKE!${NC}"
echo ""
echo "  GCP Project:  ${GCP_PROJECT}"
echo "  GKE Cluster:  ${GKE_CLUSTER} (${GCP_ZONE})"
echo "  Static IP:    ${STATIC_IP}"
echo ""
echo -e "${YELLOW}=== DNS RECORDS REQUIRED ===${NC}"
echo ""
echo "  Add these A records to your DNS provider for maniak.io:"
echo ""
echo "    bank-demo.maniak.io      →  A  ${STATIC_IP}"
echo "    mgmt.bank-demo.maniak.io →  A  ${STATIC_IP}"
echo ""
echo "  After DNS propagates and SSL certificates provision (~10-30 min):"
echo ""
echo "    Docs Site:      https://bank-demo.maniak.io"
echo "    Management UI:  https://mgmt.bank-demo.maniak.io"
echo ""
echo "  Check certificate status:"
echo "    kubectl get managedcertificate -A"
echo ""
echo "  While waiting for DNS/TLS, use port-forward:"
echo "    kubectl port-forward svc/bank-docs-site -n bank-wiki 8080:8080"
echo "    kubectl port-forward svc/solo-enterprise-ui -n kagent 8090:8090"
echo ""
echo -e "${GREEN}Setup complete!${NC}"
