#!/usr/bin/env bash
# Create the Account Summary Agent — builds, deploys, and registers everything.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GREEN='\033[0;32m'
NC='\033[0m'

step() { echo -e "\n${GREEN}▸ $1${NC}"; }

step "Building Docker image..."
docker build -t account-summary-tools:latest "${SCRIPT_DIR}/mcp-server/"

step "Loading image into Kind cluster..."
kind load docker-image account-summary-tools:latest --name solo-bank-demo

step "Deploying MCP tool server to bank-wiki namespace..."
kubectl apply -f "${SCRIPT_DIR}/manifests/deployment.yaml"
kubectl wait --for=condition=ready pod -l app=account-summary-tools \
  -n bank-wiki --timeout=60s

step "Creating RemoteMCPServer CRD..."
kubectl apply -f "${SCRIPT_DIR}/manifests/remote-mcp-server.yaml"

step "Creating Agent CRD..."
kubectl apply -f "${SCRIPT_DIR}/manifests/agent.yaml"

step "Waiting for agent to be ready..."
sleep 5
kubectl get agents -n kagent account-summary-agent

step "Registering in AgentRegistry..."
AR_URL="http://agentregistry.agentregistry.svc.cluster.local:8080"
AR_POD=$(kubectl get pod -l app.kubernetes.io/name=agentregistry -n agentregistry \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

if [[ -n "${AR_POD}" ]]; then
  AR_TOKEN=$(kubectl exec "${AR_POD}" -n agentregistry -- \
    wget -qO- --post-data='{}' --header="Content-Type: application/json" \
    "${AR_URL}/v0/auth/none" 2>/dev/null | jq -r '.registry_token' || true)

  if [[ -n "${AR_TOKEN}" && "${AR_TOKEN}" != "null" ]]; then
    AUTH="Authorization: Bearer ${AR_TOKEN}"

    kubectl exec "${AR_POD}" -n agentregistry -- \
      wget -qO- --post-data='{"name":"account-summary","title":"Account Summary","description":"Generate brief customer account summaries","version":"1.0.0","category":"banking"}' \
      --header="Content-Type: application/json" --header="${AUTH}" \
      "${AR_URL}/v0/skills" >/dev/null 2>&1 || true

    kubectl exec "${AR_POD}" -n agentregistry -- \
      wget -qO- --post-data='{"name":"account-summary-agent","title":"Solo Bank Account Summary Agent","description":"Quick financial briefings for bank staff","version":"1.0.0","image":"kagent-dev/kagent/app:0.8.0","language":"python","framework":"kagent","modelProvider":"OpenAI","modelName":"gpt-4o","skills":[{"name":"account-summary","registrySkillName":"account-summary"}]}' \
      --header="Content-Type: application/json" --header="${AUTH}" \
      "${AR_URL}/v0/agents" >/dev/null 2>&1 || true

    echo -e "${GREEN}  [OK]${NC} Registered in AgentRegistry"
  fi
fi

echo ""
echo -e "${GREEN}============================================================${NC}"
echo -e "${GREEN}  Account Summary Agent is live!${NC}"
echo -e "${GREEN}============================================================${NC}"
echo ""
echo "  Open the Management UI and select 'account-summary-agent':"
echo ""
echo "    open http://localhost:30090"
echo ""
echo "  Try: \"Give me a summary for Maria Garcia\""
echo ""
echo "  To remove: ./sample-agent/cleanup.sh"
echo ""
