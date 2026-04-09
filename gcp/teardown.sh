#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

banner() {
  echo ""
  echo -e "${RED}============================================================${NC}"
  echo -e "${RED}  $1${NC}"
  echo -e "${RED}============================================================${NC}"
  echo ""
}

warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
ok()   { echo -e "${GREEN}  [OK]${NC} $1"; }

banner "Tearing down Solo Bank Demo (GCP)"

echo -e "${YELLOW}This will delete:${NC}"
echo "  - GKE cluster:        ${GKE_CLUSTER} (${GCP_ZONE})"
echo "  - Artifact Registry:  ${AR_REPO} (${GCP_REGION})"
echo "  - Static IP:          solo-bank-demo-ip"
echo ""
read -p "Are you sure? (y/N) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Cancelled."
  exit 0
fi

# Delete GKE cluster
echo "Deleting GKE cluster '${GKE_CLUSTER}'..."
gcloud container clusters delete "${GKE_CLUSTER}" \
  --zone="${GCP_ZONE}" --quiet 2>/dev/null || \
  warn "Cluster not found or already deleted."
ok "GKE cluster deleted"

# Delete static IP
echo "Releasing static IP..."
gcloud compute addresses delete solo-bank-demo-ip \
  --global --quiet 2>/dev/null || \
  warn "Static IP not found or already released."
ok "Static IP released"

# Delete Artifact Registry (optional — images may be useful)
read -p "Also delete Artifact Registry repo '${AR_REPO}'? (y/N) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
  gcloud artifacts repositories delete "${AR_REPO}" \
    --location="${GCP_REGION}" --quiet 2>/dev/null || \
    warn "Artifact Registry repo not found or already deleted."
  ok "Artifact Registry deleted"
else
  echo "  Keeping Artifact Registry repo."
fi

banner "Teardown complete"
echo "  All GCP resources for Solo Bank Demo have been removed."
echo "  Don't forget to remove DNS A records for:"
echo "    - bank-demo.maniak.io"
echo "    - mgmt.bank-demo.maniak.io"
echo ""
