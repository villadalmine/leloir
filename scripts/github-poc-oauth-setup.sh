#!/usr/bin/env bash
# scripts/github-poc-oauth-setup.sh
# Crea el secret K8s para el oauth2-proxy del PoC de Leloir.
# Correr UNA VEZ después de crear la GitHub OAuth App.
#
# Crear la GitHub OAuth App en:
#   https://github.com/settings/developers → OAuth Apps → New OAuth App
#
#   Name:         Leloir PoC
#   Homepage URL: https://poc.leloir.cybercirujas.club
#   Callback URL: https://poc.leloir.cybercirujas.club/oauth2/callback
#
# Uso:
#   ./scripts/github-poc-oauth-setup.sh

set -euo pipefail

NAMESPACE="leloir-poc"
SECRET_NAME="leloir-poc-oauth"

echo "=== GitHub OAuth Setup — Leloir PoC ==="
echo ""
echo "Necesitás la GitHub OAuth App creada en:"
echo "  https://github.com/settings/developers"
echo ""
echo "  Homepage URL: https://poc.leloir.cybercirujas.club"
echo "  Callback URL: https://poc.leloir.cybercirujas.club/oauth2/callback"
echo ""

read -rp  "Client ID:     " CLIENT_ID
read -rsp "Client Secret: " CLIENT_SECRET; echo ""
echo ""

# Cookie secret: 32 bytes aleatorios en base64
COOKIE_SECRET=$(python3 -c "import secrets, base64; print(base64.b64encode(secrets.token_bytes(32)).decode())")

echo "Generando cookie secret..."
echo ""

kubectl create secret generic "${SECRET_NAME}" \
  --namespace "${NAMESPACE}" \
  --from-literal=client-id="${CLIENT_ID}" \
  --from-literal=client-secret="${CLIENT_SECRET}" \
  --from-literal=cookie-secret="${COOKIE_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "✓ Secret '${SECRET_NAME}' creado/actualizado en namespace '${NAMESPACE}'"
echo ""
echo "Reiniciando oauth2-proxy para que tome los nuevos credentials..."
kubectl rollout restart deployment -l app.kubernetes.io/name=oauth2-proxy -n "${NAMESPACE}" 2>/dev/null || true
echo ""
echo "=== DONE ==="
echo "Verificar: https://poc.leloir.cybercirujas.club"
echo "Debería pedir login con GitHub y redirigir a la UI del PoC."
