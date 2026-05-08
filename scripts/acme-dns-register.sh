#!/usr/bin/env bash
# scripts/acme-dns-register.sh
# One-time setup: register with acme-dns.io and create the cert-manager secret.
# Output is NOT stored in git. Store credentials in a password manager.
#
# Usage:
#   ./scripts/acme-dns-register.sh                  # first-time setup
#   ./scripts/acme-dns-register.sh --add example.com  # add domain to existing account

set -euo pipefail

ACME_DNS_SERVER="https://auth.acme-dns.io"
BASE_DOMAIN="leloir.cybercirujas.club"

# Default domains that need certs (extend as you add services)
DOMAINS=(
  "grafana.${BASE_DOMAIN}"
  "poc.${BASE_DOMAIN}"
)

ADD_DOMAIN=""
if [[ "${1:-}" == "--add" && -n "${2:-}" ]]; then
  ADD_DOMAIN="$2"
fi

if ! command -v jq &>/dev/null; then
  echo "ERROR: jq is required. Install it with: sudo dnf install jq" >&2
  exit 1
fi

echo "=== acme-dns.io Registration ==="
echo "Registering new account at ${ACME_DNS_SERVER}..."
RESPONSE=$(curl -sf -X POST "${ACME_DNS_SERVER}/register")

USERNAME=$(echo "$RESPONSE" | jq -r '.username')
PASSWORD=$(echo "$RESPONSE" | jq -r '.password')
FULLDOMAIN=$(echo "$RESPONSE" | jq -r '.fulldomain')
SUBDOMAIN=$(echo "$RESPONSE" | jq -r '.subdomain')

echo "Account created: ${SUBDOMAIN}.auth.acme-dns.io"
echo ""

# ── CNAME records ─────────────────────────────────────────────────────────────
echo "=== 1. Add these CNAME records in your DNS provider ==="
echo "   (Namecheap: Advanced DNS → Add New Record)"
echo ""
echo "   Zone: ${BASE_DOMAIN%%.*}.${BASE_DOMAIN#*.}"
echo ""
for DOMAIN in "${DOMAINS[@]}"; do
  # Strip the base domain suffix to get the subdomain prefix
  PREFIX="${DOMAIN%.${BASE_DOMAIN}}"
  echo "   Host:  _acme-challenge.${PREFIX}.$(echo $BASE_DOMAIN | cut -d. -f1)"
  echo "   Type:  CNAME"
  echo "   Value: ${FULLDOMAIN}."
  echo ""
done

if [[ -n "$ADD_DOMAIN" ]]; then
  PREFIX="${ADD_DOMAIN%.${BASE_DOMAIN}}"
  echo "   Host:  _acme-challenge.${PREFIX}.$(echo $BASE_DOMAIN | cut -d. -f1)"
  echo "   Type:  CNAME"
  echo "   Value: ${FULLDOMAIN}."
  echo ""
fi

# ── Build credentials.json ────────────────────────────────────────────────────
ACCOUNT_JSON=$(jq -n \
  --arg username "$USERNAME" \
  --arg password "$PASSWORD" \
  --arg fulldomain "$FULLDOMAIN" \
  --arg subdomain "$SUBDOMAIN" \
  '{username: $username, password: $password, fulldomain: $fulldomain, subdomain: $subdomain, allowfrom: []}')

# Start with empty object and merge each domain entry
CREDENTIALS_JSON="{}"
for DOMAIN in "${DOMAINS[@]}"; do
  CREDENTIALS_JSON=$(echo "$CREDENTIALS_JSON" | jq --arg k "$DOMAIN" --argjson v "$ACCOUNT_JSON" '. + {($k): $v}')
done
if [[ -n "$ADD_DOMAIN" ]]; then
  CREDENTIALS_JSON=$(echo "$CREDENTIALS_JSON" | jq --arg k "$ADD_DOMAIN" --argjson v "$ACCOUNT_JSON" '. + {($k): $v}')
fi

# ── kubectl command ────────────────────────────────────────────────────────────
echo "=== 2. Create Kubernetes secret (run after adding CNAMEs) ==="
echo ""
cat <<KUBECTL
kubectl create secret generic acme-dns-account \\
  --namespace cert-manager \\
  --from-literal=credentials.json='${CREDENTIALS_JSON}'
KUBECTL
echo ""

# ── Save locally (gitignored) ─────────────────────────────────────────────────
SAVE_PATH="$(dirname "$0")/../acme-dns-account.json"
echo "$CREDENTIALS_JSON" > "$SAVE_PATH"
echo "=== Credentials saved to acme-dns-account.json (gitignored) ==="
echo "    Store a backup in your password manager."
echo ""
echo "DONE. After adding CNAMEs and creating the secret:"
echo "  ArgoCD will sync cert-manager and the ClusterIssuers."
echo "  First certificate request takes ~60s."
