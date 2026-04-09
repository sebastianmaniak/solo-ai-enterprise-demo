#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${1:-setup.sh}"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "setup script not found: ${SCRIPT_PATH}" >&2
  exit 1
fi

block="$(
  awk '
    /# Step 6: Install Management UI/ { in_block=1 }
    /# Step 7: Install kagent Enterprise/ { exit }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

block_without_comments="$(printf '%s\n' "${block}" | rg -v '^\s*#')"

if [[ -z "${block}" ]]; then
  echo "could not locate Step 6 management block in ${SCRIPT_PATH}" >&2
  exit 1
fi

echo "${block}" | rg -q 'helm upgrade --install management' || {
  echo "expected management helm install command" >&2
  exit 1
}

if echo "${block_without_comments}" | rg -q -- '--wait'; then
  echo "management install should not use helm --wait" >&2
  exit 1
fi

for pattern in \
  'kubectl rollout status statefulset/management-clickhouse-shard0' \
  'kubectl rollout status statefulset/solo-enterprise-telemetry-collector' \
  'kubectl rollout status deployment/solo-enterprise-ui'
do
  echo "${block}" | rg -q "${pattern}" || {
    echo "missing explicit readiness check: ${pattern}" >&2
    exit 1
  }
done

echo "management step waits on concrete workloads instead of helm release wait"
