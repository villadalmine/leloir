#!/usr/bin/env bash
# scripts/build-poc.sh
# Builds the leloir-poc Docker image and pushes to ghcr.io.
#
# Pre-requisito: autenticarse con ghcr.io
#   echo $GITHUB_TOKEN | podman login ghcr.io -u villadalmine --password-stdin
#
# Uso:
#   ./scripts/build-poc.sh              # build + push :latest
#   ./scripts/build-poc.sh --tag v1.0   # build + push con tag específico
#   ./scripts/build-poc.sh --local      # build solo, no push (para k3s local)

set -euo pipefail

IMAGE="ghcr.io/villadalmine/leloir-poc"
TAG="latest"
PUSH=true
LOAD_K3S=false

while [[ "${1:-}" != "" ]]; do
  case "$1" in
    --tag)   TAG="$2"; shift ;;
    --local) PUSH=false; LOAD_K3S=true ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
  shift
done

cd "$(dirname "$0")/../poc"

echo "=== Build: ${IMAGE}:${TAG} ==="
podman build -t "${IMAGE}:${TAG}" .

if $LOAD_K3S; then
  echo "=== Importando imagen en k3s ==="
  podman save "${IMAGE}:${TAG}" | sudo podman exec -i k3s-server ctr images import -
  echo "✓ Imagen disponible en k3s como ${IMAGE}:${TAG}"
fi

if $PUSH; then
  echo "=== Push a ghcr.io ==="
  podman push "${IMAGE}:${TAG}"
  echo "✓ Imagen subida: ${IMAGE}:${TAG}"
fi

echo ""
echo "Próximo paso: actualizar deploy/apps/leloir-poc/values.yaml con el nuevo tag"
echo "  image.tag: ${TAG}"
