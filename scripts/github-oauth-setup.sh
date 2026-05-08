#!/usr/bin/env bash
# scripts/github-oauth-setup.sh
# Crea los K8s secrets para GitHub OAuth (ArgoCD y Grafana).
# Los credentials NO van a git. Correr una vez por cluster.
#
# Pre-requisito: crear dos GitHub OAuth Apps en
#   https://github.com/settings/developers → OAuth Apps → New OAuth App
#
#   App 1 — ArgoCD:
#     Name: Leloir ArgoCD
#     Homepage: https://argocd.leloir.cybercirujas.club
#     Callback URL: https://argocd.leloir.cybercirujas.club/api/dex/callback
#
#   App 2 — Grafana:
#     Name: Leloir Grafana
#     Homepage: https://grafana.leloir.cybercirujas.club
#     Callback URL: https://grafana.leloir.cybercirujas.club/login/github
#
# Uso:
#   ./scripts/github-oauth-setup.sh

set -euo pipefail

echo "=== GitHub OAuth Setup para Leloir ==="
echo ""
echo "Necesitás dos GitHub OAuth Apps creadas."
echo "Si no las creaste todavía, abrí: https://github.com/settings/developers"
echo ""

read -rp "ArgoCD — Client ID:     " ARGOCD_CLIENT_ID
read -rsp "ArgoCD — Client Secret: " ARGOCD_CLIENT_SECRET; echo ""
echo ""
read -rp "Grafana — Client ID:     " GRAFANA_CLIENT_ID
read -rsp "Grafana — Client Secret: " GRAFANA_CLIENT_SECRET; echo ""
echo ""

# ── ArgoCD: patch argocd-secret ───────────────────────────────────────────────
echo "Configurando secret de ArgoCD (dex GitHub)..."
kubectl patch secret argocd-secret -n argocd \
  --type=merge \
  -p "{\"data\":{
    \"dex.github.clientID\": \"$(echo -n "$ARGOCD_CLIENT_ID" | base64 -w0)\",
    \"dex.github.clientSecret\": \"$(echo -n "$ARGOCD_CLIENT_SECRET" | base64 -w0)\"
  }}"
echo "✓ argocd-secret actualizado"

# ── Grafana: crear/reemplazar secret ─────────────────────────────────────────
echo "Configurando secret de Grafana..."
kubectl create secret generic grafana-github-oauth \
  --namespace prometheus \
  --from-literal=client-id="$GRAFANA_CLIENT_ID" \
  --from-literal=client-secret="$GRAFANA_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl apply -f -
echo "✓ grafana-github-oauth creado/actualizado"

echo ""
echo "=== DONE ==="
echo "ArgoCD se recarga solo al detectar el cambio en argocd-cm."
echo "Grafana necesita un rollout restart:"
echo "  kubectl rollout restart deployment prometheus-grafana -n prometheus"
