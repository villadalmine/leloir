#!/usr/bin/env bash
# cluster-up.sh — Levanta el cluster k3s para el PoC de Leloir M0
# =================================================================
# Corre como ROOT desde el HOST Fedora (fuera del toolbx).
# Usa rootful podman para poder escribir en cgroups.
#
# Contexto del problema resuelto:
#   El toolbx container corre rootless podman, que no puede crear
#   cgroup directories (/sys/fs/cgroup/kubepods) ni bridges de red.
#   La solución es correr k3s con podman rootful (root) directamente
#   en el host, exponiendo el puerto 6443 que es accesible desde
#   dentro del toolbx via 127.0.0.1.
#
# Uso:
#   sudo ./scripts/cluster-up.sh          # levantar cluster
#   sudo ./scripts/cluster-up.sh --down   # bajar y eliminar cluster
#   sudo ./scripts/cluster-up.sh --status # ver estado
#
# Prerequisitos:
#   - podman instalado en el host (Fedora lo trae por defecto)
#   - El usuario que ejecuta kubectl debe tener ~/.kube/config
#
# Después de correr este script, desde el toolbx:
#   kubectl get nodes

set -euo pipefail

# ── Configuración ─────────────────────────────────────────────────
CONTAINER_NAME="k3s-server"
K3S_IMAGE="docker.io/rancher/k3s:latest"
API_PORT="6443"
KUBECONFIG_USER="${SUDO_USER:-dalmine}"
KUBECONFIG_PATH="/home/${KUBECONFIG_USER}/.kube/config"
WAIT_TIMEOUT=60

# ── Helpers ───────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; BOLD='\033[1m'; RESET='\033[0m'
ok()   { echo -e "${GREEN}✓${RESET} $1"; }
warn() { echo -e "${YELLOW}⚠${RESET} $1"; }
fail() { echo -e "${RED}✗${RESET} $1"; exit 1; }
info() { echo -e "${BLUE}ℹ${RESET} $1"; }

# ── Verificar root ────────────────────────────────────────────────
check_root() {
    if [[ $EUID -ne 0 ]]; then
        fail "Este script debe correr como root: sudo $0"
    fi
}

# ── Bajar el cluster ──────────────────────────────────────────────
cluster_down() {
    echo -e "\n${BOLD}── Bajando cluster ──${RESET}"
    if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        podman stop "${CONTAINER_NAME}" 2>/dev/null || true
        podman rm   "${CONTAINER_NAME}" 2>/dev/null || true
        ok "Container '${CONTAINER_NAME}' eliminado"
    else
        warn "Container '${CONTAINER_NAME}' no existe"
    fi
    exit 0
}

# ── Estado ────────────────────────────────────────────────────────
cluster_status() {
    echo -e "\n${BOLD}── Estado del cluster ──${RESET}"
    if podman ps --format '{{.Names}} {{.Status}}' | grep -q "^${CONTAINER_NAME}"; then
        ok "Container corriendo"
        podman ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        echo
        if [[ -f "${KUBECONFIG_PATH}" ]]; then
            export KUBECONFIG="${KUBECONFIG_PATH}"
            kubectl get nodes 2>/dev/null || warn "kubectl no puede conectar aún"
        fi
    else
        warn "Container '${CONTAINER_NAME}' no está corriendo"
    fi
    exit 0
}

# ── Levantar cluster ──────────────────────────────────────────────
cluster_up() {
    echo -e "\n${BOLD}── Levantando cluster Leloir PoC ──${RESET}"

    # ¿Ya existe?
    if podman ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        ok "Container '${CONTAINER_NAME}' ya está corriendo"
        extract_kubeconfig
        wait_for_ready
        return
    fi

    # ¿Existe pero parado?
    if podman ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        info "Container existe pero está parado — reiniciando"
        podman start "${CONTAINER_NAME}"
        extract_kubeconfig
        wait_for_ready
        return
    fi

    info "Iniciando k3s container..."
    podman run \
        --privileged \
        --cgroupns=host \
        --tmpfs /run \
        --tmpfs /var/run \
        --name "${CONTAINER_NAME}" \
        -p "${API_PORT}:6443" \
        -p 80:80 \
        -p 443:443 \
        -d \
        "${K3S_IMAGE}" server \
        --disable traefik \
        --disable servicelb \
        --disable metrics-server \
        --kubelet-arg=feature-gates=KubeletInUserNamespace=true \
        > /dev/null

    ok "Container iniciado"

    wait_for_apiserver
    extract_kubeconfig
    wait_for_ready
}

# ── Esperar a que el API server responda ──────────────────────────
wait_for_apiserver() {
    info "Esperando API server en :${API_PORT}..."
    local elapsed=0
    until podman exec "${CONTAINER_NAME}" \
            k3s kubectl get nodes &>/dev/null; do
        sleep 2
        elapsed=$((elapsed + 2))
        if [[ $elapsed -ge $WAIT_TIMEOUT ]]; then
            fail "Timeout esperando el API server (${WAIT_TIMEOUT}s)"
        fi
        echo -n "."
    done
    echo
    ok "API server responde"
}

# ── Extraer kubeconfig ────────────────────────────────────────────
extract_kubeconfig() {
    info "Extrayendo kubeconfig para '${KUBECONFIG_USER}'..."
    mkdir -p "$(dirname "${KUBECONFIG_PATH}")"

    podman exec "${CONTAINER_NAME}" \
        cat /etc/rancher/k3s/k3s.yaml > "${KUBECONFIG_PATH}"

    chown "${KUBECONFIG_USER}:${KUBECONFIG_USER}" "${KUBECONFIG_PATH}"
    chmod 600 "${KUBECONFIG_PATH}"
    ok "Kubeconfig en ${KUBECONFIG_PATH}"
}

# ── Esperar nodo Ready ────────────────────────────────────────────
wait_for_ready() {
    info "Esperando nodo Ready..."
    local elapsed=0
    export KUBECONFIG="${KUBECONFIG_PATH}"
    until kubectl get nodes 2>/dev/null | grep -q "Ready"; do
        sleep 2
        elapsed=$((elapsed + 2))
        if [[ $elapsed -ge $WAIT_TIMEOUT ]]; then
            warn "Timeout esperando Ready — el cluster puede tardar un poco más"
            return
        fi
        echo -n "."
    done
    echo
    ok "Nodo Ready"
    echo
    kubectl get nodes
    echo
    info "Desde el toolbx: kubectl get nodes"
    info "Para bajar: sudo $0 --down"
}

# ── Main ──────────────────────────────────────────────────────────
check_root

case "${1:-}" in
    --down)   cluster_down   ;;
    --status) cluster_status ;;
    *)        cluster_up     ;;
esac
