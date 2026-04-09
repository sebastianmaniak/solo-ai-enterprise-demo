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
# Step 2: Install Gateway API CRDs
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
# Step 5: Install AgentGateway Enterprise
# ---------------------------------------------------------------------------
banner "Step 5: Install AgentGateway Enterprise"

helm upgrade --install enterprise-agentgateway-crds \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway-crds \
  --create-namespace --namespace agentgateway-system \
  --version v2.3.0-beta.8 --wait --timeout 120s

helm upgrade --install enterprise-agentgateway \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway \
  --namespace agentgateway-system --version v2.3.0-beta.8 \
  --set-string licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo -e "${GREEN}AgentGateway Enterprise installed.${NC}"

# ---------------------------------------------------------------------------
# Step 6: Apply Gateway and tracing
# ---------------------------------------------------------------------------
banner "Step 6: Apply Gateway and tracing"

kubectl apply -f "${SCRIPT_DIR}/manifests/gateway.yaml"
echo "Waiting for proxy deployment to be ready..."
kubectl wait --for=condition=available deployment \
  -l app.kubernetes.io/component=proxy \
  -n agentgateway-system \
  --timeout=120s 2>/dev/null || \
kubectl wait --for=condition=available deployment \
  --all \
  -n agentgateway-system \
  --timeout=120s

echo -e "${GREEN}Gateway applied and proxy ready.${NC}"

# ---------------------------------------------------------------------------
# Step 7: Install Management UI
# ---------------------------------------------------------------------------
banner "Step 7: Install Management UI"

helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace agentgateway-system --create-namespace \
  --version 0.3.14 \
  --set cluster="solo-bank-demo" \
  --set products.agentgateway.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo -e "${GREEN}Management UI installed.${NC}"

# ---------------------------------------------------------------------------
# Step 8: Configure LLM backends
# ---------------------------------------------------------------------------
banner "Step 8: Configure LLM backends"

sed "s|__OPENAI_API_KEY__|${OPENAI_API_KEY}|g" \
  "${SCRIPT_DIR}/manifests/llm-backends/openai.yaml" | kubectl apply -f -

sed "s|__ANTHROPIC_API_KEY__|${ANTHROPIC_API_KEY}|g" \
  "${SCRIPT_DIR}/manifests/llm-backends/anthropic.yaml" | kubectl apply -f -

# Ensure kagent namespace exists before creating secrets
kubectl create namespace kagent --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic anthropic-secret \
  --namespace kagent \
  --from-literal=Authorization="${ANTHROPIC_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}LLM backends configured.${NC}"

# ---------------------------------------------------------------------------
# Step 9: Install kagent Enterprise
# ---------------------------------------------------------------------------
banner "Step 9: Install kagent Enterprise"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent --version 0.3.14 --wait --timeout 120s

helm upgrade --install kagent-enterprise \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise \
  --namespace kagent --version 0.3.14 \
  --set defaultModelConfig.provider=OpenAI \
  --set defaultModelConfig.model=gpt-4o-mini \
  --set controller.enabled=true \
  --set kmcp.enabled=true \
  --wait --timeout 300s

echo -e "${GREEN}kagent Enterprise installed.${NC}"

# ---------------------------------------------------------------------------
# Step 10: Build and load Docker images
# ---------------------------------------------------------------------------
banner "Step 10: Build and load Docker images"

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
else
  warn "docs-site directory not found. Skipping bank-docs-site image build."
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
# Step 11: Deploy bank wiki and tool servers
# ---------------------------------------------------------------------------
banner "Step 11: Deploy bank wiki and tool servers"

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
# Step 12: Apply MCP routing and agents
# ---------------------------------------------------------------------------
banner "Step 12: Apply MCP routing and agents"

kubectl apply -f "${SCRIPT_DIR}/manifests/mcp/"
kubectl apply -f "${SCRIPT_DIR}/manifests/agents/"

if [ -d "${SCRIPT_DIR}/manifests/docs-site" ]; then
  kubectl apply -f "${SCRIPT_DIR}/manifests/docs-site/"
else
  warn "manifests/docs-site directory not found. Skipping docs-site manifest deployment."
fi

echo -e "${GREEN}MCP routing and agents applied.${NC}"

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
    # Try a basic connectivity check even if the health response format differs
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

# ---------------------------------------------------------------------------
# Step 14: Print access info
# ---------------------------------------------------------------------------
banner "Step 14: Access Information"

echo -e "${GREEN}Solo Bank Demo is deployed!${NC}"
echo ""
echo "Port-forward commands to access services:"
echo ""
echo "  AgentGateway Proxy (MCP endpoint):"
echo "    kubectl port-forward svc/enterprise-agentgateway -n agentgateway-system 8080:8080"
echo ""
echo "  Management UI:"
echo "    kubectl port-forward svc/management -n agentgateway-system 8090:8090"
echo "    Then open: http://localhost:8090"
echo ""
echo "  AgentRegistry:"
echo "    kubectl port-forward svc/agentregistry -n agentregistry 8081:80"
echo "    Then open: http://localhost:8081"
echo ""
echo "  kagent UI:"
echo "    kubectl port-forward svc/kagent-enterprise -n kagent 8082:80"
echo "    Then open: http://localhost:8082"
echo ""
echo "  Bank Wiki Server:"
echo "    kubectl port-forward svc/bank-wiki-server -n bank-wiki 8083:8080"
echo "    Then open: http://localhost:8083"
echo ""
if [ -d "${SCRIPT_DIR}/docs-site" ]; then
  echo "  Docs Site:"
  echo "    kubectl port-forward svc/bank-docs-site -n bank-wiki 8084:8080"
  echo "    Then open: http://localhost:8084"
  echo ""
fi
echo -e "${GREEN}Setup complete!${NC}"
