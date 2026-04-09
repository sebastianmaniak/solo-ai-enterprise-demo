#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="solo-bank-demo"

echo "Deleting Kind cluster: ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}"

echo "Removing Docker images..."
docker rmi bank-wiki-server:latest bank-customer-tools:latest bank-policy-tools:latest bank-transaction-tools:latest bank-docs-site:latest 2>/dev/null || true

echo "Done. Cluster and images removed."
