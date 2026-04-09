#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${1:-setup.sh}"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "setup script not found: ${SCRIPT_PATH}" >&2
  exit 1
fi

block="$(
  awk '
    /# Step 10: Deploy bank wiki and tool servers/ { in_block=1 }
    /echo -e "\$\{GREEN\}Bank wiki and tool servers deployed\.\$\{NC\}"/ { print; exit }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

if [[ -z "${block}" ]]; then
  echo "could not locate Step 10 bank wiki block in ${SCRIPT_PATH}" >&2
  exit 1
fi

if echo "${block}" | rg -q 'kubectl wait --for=condition=Ready pod -l app='; then
  echo "bank wiki step should not wait on pods by label" >&2
  exit 1
fi

for pattern in \
  'kubectl rollout status deployment/bank-wiki-server' \
  'kubectl rollout status deployment/bank-customer-tools' \
  'kubectl rollout status deployment/bank-policy-tools' \
  'kubectl rollout status deployment/bank-transaction-tools' \
  'kubectl rollout status deployment/bank-status-tools' \
  'kubectl rollout status deployment/bank-incident-tools' \
  'kubectl rollout status deployment/bank-docs-site'
do
  echo "${block}" | rg -q "${pattern}" || {
    echo "missing explicit readiness check: ${pattern}" >&2
    exit 1
  }
done

echo "bank wiki step waits on concrete deployments instead of racing pod creation"
