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

for var in OPENAI_API_KEY ANTHROPIC_API_KEY AGENTGATEWAY_LICENSE_KEY; do
  if [[ -z "${!var:-}" ]]; then
    fail "Required environment variable not set: ${var}"
  fi
  echo -e "${GREEN}  [OK]${NC} ${var}"
done

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
# Step 3: Install AgentRegistry OSS
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
# Step 4: Install kagent Enterprise CRDs
# ---------------------------------------------------------------------------
banner "Step 5: Install kagent Enterprise CRDs"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent --create-namespace --version 0.3.14 --wait --timeout 120s

echo -e "${GREEN}kagent CRDs installed.${NC}"

# ---------------------------------------------------------------------------
# Step 5: Install Management UI (provides OIDC for kagent)
# ---------------------------------------------------------------------------
banner "Step 6: Install Management UI"

helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace kagent \
  --version 0.3.14 \
  --set cluster="solo-bank-demo" \
  --set products.kagent.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --no-hooks --timeout 300s

echo "Waiting for Management UI pods to be ready..."
kubectl wait --for=condition=available deployment/solo-enterprise-ui \
  -n kagent --timeout=180s 2>/dev/null || \
  warn "Management UI deployment not ready yet — kagent may need a restart."

echo -e "${GREEN}Management UI installed.${NC}"

# ---------------------------------------------------------------------------
# Step 6: Install kagent Enterprise
# ---------------------------------------------------------------------------
banner "Step 7: Install kagent Enterprise"

helm upgrade --install kagent-enterprise \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise \
  --namespace kagent --version 0.3.14 \
  --set defaultModelConfig.provider=OpenAI \
  --set defaultModelConfig.model=gpt-4o-mini \
  --set controller.enabled=true \
  --set kmcp.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo -e "${GREEN}kagent Enterprise installed.${NC}"

# ---------------------------------------------------------------------------
# Step 7: Create LLM API key secrets
# ---------------------------------------------------------------------------
banner "Step 8: Create LLM API key secrets"

kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic anthropic-secret \
  --namespace kagent \
  --from-literal=Authorization="${ANTHROPIC_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}LLM API key secrets created.${NC}"

# ---------------------------------------------------------------------------
# Step 7: Build and load Docker images
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

if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  echo "Building bank-docs-site..."
  docker build -t bank-docs-site:latest "${SCRIPT_DIR}/docs-site/"
fi

echo "Loading images into Kind cluster..."
kind load docker-image bank-wiki-server:latest --name solo-bank-demo
kind load docker-image bank-customer-tools:latest --name solo-bank-demo
kind load docker-image bank-policy-tools:latest --name solo-bank-demo
kind load docker-image bank-transaction-tools:latest --name solo-bank-demo

if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  kind load docker-image bank-docs-site:latest --name solo-bank-demo
fi

echo -e "${GREEN}Docker images built and loaded.${NC}"

# ---------------------------------------------------------------------------
# Step 8: Deploy bank wiki and tool servers
# ---------------------------------------------------------------------------
banner "Step 10: Deploy bank wiki and tool servers"

kubectl apply -f "${SCRIPT_DIR}/manifests/bank-wiki/"

echo "Waiting for bank-wiki-server pods..."
kubectl wait --for=condition=ready pod -l app=bank-wiki-server \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-customer-tools pods..."
kubectl wait --for=condition=ready pod -l app=bank-customer-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-policy-tools pods..."
kubectl wait --for=condition=ready pod -l app=bank-policy-tools \
  -n bank-wiki --timeout=120s

echo "Waiting for bank-transaction-tools pods..."
kubectl wait --for=condition=ready pod -l app=bank-transaction-tools \
  -n bank-wiki --timeout=120s

echo -e "${GREEN}Bank wiki and tool servers deployed.${NC}"

# ---------------------------------------------------------------------------
# Step 9: Apply MCP servers, model configs, and agents
# ---------------------------------------------------------------------------
banner "Step 11: Apply MCP servers, model configs, and agents"

kubectl apply -f "${SCRIPT_DIR}/manifests/mcp/"
kubectl apply -f "${SCRIPT_DIR}/manifests/agents/"

echo -e "${GREEN}MCP servers, model configs, and agents applied.${NC}"

# ---------------------------------------------------------------------------
# Step 10: Smoke tests
# ---------------------------------------------------------------------------
banner "Step 12: Smoke tests"

echo "Checking wiki server health..."
WIKI_POD=$(kubectl get pod -l app=bank-wiki-server -n bank-wiki \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "${WIKI_POD}" ]]; then
  if kubectl exec "${WIKI_POD}" -n bank-wiki -- \
      wget -qO- http://localhost:8080/health 2>/dev/null | grep -qi "ok\|healthy\|200\|up"; then
    echo -e "${GREEN}  [OK]${NC} Wiki server health check passed."
  else
    if kubectl exec "${WIKI_POD}" -n bank-wiki -- \
        wget -qO- http://localhost:8080/health 2>/dev/null; then
      echo -e "${GREEN}  [OK]${NC} Wiki server is responding."
    else
      warn "Wiki server health check returned unexpected response. The server may still be initializing."
    fi
  fi
else
  warn "Could not find wiki server pod for health check."
fi

echo "Checking agents exist..."
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

# ---------------------------------------------------------------------------
# Step 11: Access Information
# ---------------------------------------------------------------------------
banner "Step 13: Access Information"

echo -e "${GREEN}Solo Bank Demo is deployed!${NC}"
echo ""
echo "Port-forward commands to access services:"
echo ""
echo "  Management UI:"
echo "    kubectl port-forward svc/solo-enterprise-ui -n kagent 8090:8090"
echo "    Then open: http://localhost:8090"
echo ""
echo "  AgentRegistry:"
echo "    kubectl port-forward svc/agentregistry -n agentregistry 8081:80"
echo "    Then open: http://localhost:8081"
echo ""
echo "  kagent API:"
echo "    kubectl port-forward svc/kagent-controller -n kagent 8083:8083"
echo ""
echo "  Bank Wiki Server:"
echo "    kubectl port-forward svc/bank-wiki-server -n bank-wiki 8080:8080"
echo "    Then open: http://localhost:8080"
echo ""
if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  echo "  Docs Site (NodePort 30500):"
  echo "    Open: http://localhost:30500"
  echo ""
fi
echo -e "${GREEN}Setup complete!${NC}"
