#!/usr/bin/env bash
# Remove the Account Summary Agent — deletes everything created by create.sh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

step() { echo -e "\n${RED}▸ $1${NC}"; }

step "Deleting Agent CRD..."
kubectl delete -f "${SCRIPT_DIR}/manifests/agent.yaml" --ignore-not-found

step "Deleting RemoteMCPServer CRD..."
kubectl delete -f "${SCRIPT_DIR}/manifests/remote-mcp-server.yaml" --ignore-not-found

step "Deleting Deployment and Service..."
kubectl delete -f "${SCRIPT_DIR}/manifests/deployment.yaml" --ignore-not-found

step "Removing from AgentRegistry..."
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
      wget -qO- --method=DELETE --header="${AUTH}" \
      "${AR_URL}/v0/agents/account-summary-agent" >/dev/null 2>&1 || true
    kubectl exec "${AR_POD}" -n agentregistry -- \
      wget -qO- --method=DELETE --header="${AUTH}" \
      "${AR_URL}/v0/skills/account-summary" >/dev/null 2>&1 || true
    echo -e "${GREEN}  [OK]${NC} Removed from AgentRegistry"
  fi
fi

echo ""
echo -e "${GREEN}============================================================${NC}"
echo -e "${GREEN}  Account Summary Agent removed.${NC}"
echo -e "${GREEN}============================================================${NC}"
echo ""
