#!/usr/bin/env bash
# Leloir Bootstrap Script
# =======================
# Automates Day 1 of Milestone 0:
#   - Verifies prerequisites (Go, Docker, kubectl, helm, gh)
#   - Checks GitHub org and domain availability for "leloir"
#   - Unpacks the SDK and control plane skeletons
#   - Initializes a working git repo with the first commit
#
# Usage:
#   ./bootstrap.sh                  # run all steps interactively
#   ./bootstrap.sh --check-prereqs  # only verify tools
#   ./bootstrap.sh --check-naming   # only verify GitHub org + domains
#   ./bootstrap.sh --extract        # only unpack skeletons
#   ./bootstrap.sh --init-git       # only init git + first commit
#   ./bootstrap.sh --help           # show this help

set -euo pipefail

# ─── Color helpers ──────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

ok()    { echo -e "${GREEN}✓${RESET} $1"; }
warn()  { echo -e "${YELLOW}⚠${RESET} $1"; }
fail()  { echo -e "${RED}✗${RESET} $1"; }
info()  { echo -e "${BLUE}ℹ${RESET} $1"; }
title() { echo -e "\n${BOLD}── $1 ──${RESET}"; }

# ─── Defaults ────────────────────────────────────────────────────────────────
BUNDLE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSETS_DIR="$BUNDLE_DIR/assets"
WORK_DIR="${LELOIR_WORK_DIR:-$HOME/leloir-workspace}"

DO_PREREQS=true
DO_NAMING=true
DO_EXTRACT=true
DO_GIT_INIT=true

# ─── Argument parsing ───────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --check-prereqs)
            DO_NAMING=false; DO_EXTRACT=false; DO_GIT_INIT=false
            ;;
        --check-naming)
            DO_PREREQS=false; DO_EXTRACT=false; DO_GIT_INIT=false
            ;;
        --extract)
            DO_PREREQS=false; DO_NAMING=false; DO_GIT_INIT=false
            ;;
        --init-git)
            DO_PREREQS=false; DO_NAMING=false; DO_EXTRACT=false
            ;;
        --work-dir)
            WORK_DIR="$2"; shift
            ;;
        --help|-h)
            grep '^#' "$0" | head -20
            exit 0
            ;;
        *)
            fail "Unknown argument: $1"
            exit 1
            ;;
    esac
    shift
done

# ─── Phase 1: Prerequisites ─────────────────────────────────────────────────
check_prereqs() {
    title "Phase 1: Prerequisites"

    local missing=()

    check_tool() {
        local tool="$1"
        local install_hint="${2:-}"
        if command -v "$tool" >/dev/null 2>&1; then
            local version=$("$tool" --version 2>&1 | head -1 || echo "unknown")
            ok "$tool found: $version"
        else
            fail "$tool not found"
            [ -n "$install_hint" ] && info "  Install: $install_hint"
            missing+=("$tool")
        fi
    }

    check_tool go "https://go.dev/dl/ (need 1.22+)"
    check_tool docker "https://docs.docker.com/get-docker/"
    check_tool kubectl "https://kubernetes.io/docs/tasks/tools/"
    check_tool helm "https://helm.sh/docs/intro/install/"
    check_tool kind "go install sigs.k8s.io/kind@latest  (or brew install kind)"
    check_tool gh "https://cli.github.com/"
    check_tool git ""
    check_tool jq "https://jqlang.github.io/jq/download/"
    check_tool unzip ""

    # Verify Go version
    if command -v go >/dev/null 2>&1; then
        local go_ver=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1)
        local major=$(echo "$go_ver" | cut -d. -f1 | sed 's/go//')
        local minor=$(echo "$go_ver" | cut -d. -f2)
        if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 22 ]; }; then
            warn "Go version $go_ver detected; need 1.22+ for the SDK"
        fi
    fi

    if [ ${#missing[@]} -ne 0 ]; then
        echo
        fail "Missing tools: ${missing[*]}"
        info "Install the missing tools and re-run this script."
        return 1
    fi

    ok "All prerequisites present"
    return 0
}

# ─── Phase 2: Naming verification ───────────────────────────────────────────
check_naming() {
    title "Phase 2: Naming Verification"

    info "Checking availability of 'leloir' across platforms..."
    echo

    # GitHub org check
    if command -v gh >/dev/null 2>&1; then
        if gh api orgs/leloir 2>/dev/null | jq -r '.login' 2>/dev/null | grep -qi '^leloir$'; then
            warn "github.com/leloir is TAKEN. Fallback options:"
            info "  1. github.com/leloir-platform"
            info "  2. github.com/leloirhq"
            info "  3. github.com/leloir-io"
        else
            ok "github.com/leloir appears AVAILABLE"
            info "  → reserve at: https://github.com/account/organizations/new"
        fi
    else
        warn "gh CLI not authenticated; skipping GitHub org check"
        info "  Manually verify at https://github.com/leloir"
    fi

    echo

    # Domain checks (best-effort; whois isn't always reliable)
    info "Manually verify these domains (whois isn't always conclusive):"
    info "  → https://domains.google.com/registrar/search?searchTerm=leloir.dev"
    info "  → https://domains.google.com/registrar/search?searchTerm=leloir.io"
    info "  → https://domains.google.com/registrar/search?searchTerm=leloir.ai"
    echo
    info "Other things to verify on Day 1 (~30 min total):"
    info "  □ Trademark search USPTO: https://tmsearch.uspto.gov"
    info "  □ Trademark search EUIPO: https://euipo.europa.eu/eSearch"
    info "  □ Slack workspace: https://slack.com/get-started (try leloir.slack.com)"
    info "  □ NPM:    npm view leloir 2>&1 | grep -q 'not found' && echo AVAILABLE"
    info "  □ PyPI:   curl -s https://pypi.org/pypi/leloir/json | grep -q 'Not Found'"

    echo
    info "Background context: Luis Federico Leloir (1906-1987), Argentine Nobel"
    info "Laureate in Chemistry. Family name; not a trademark. Available on"
    info "GitHub at time of design doc (only BibliotecaLeloir, an unrelated"
    info "library account, was found)."
}

# ─── Phase 3: Extract skeletons ─────────────────────────────────────────────
extract_skeletons() {
    title "Phase 3: Extract Skeletons"

    if [ -d "$WORK_DIR" ]; then
        warn "Work directory $WORK_DIR already exists"
        read -rp "Overwrite? (y/N): " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            info "Skipping extraction"
            return 0
        fi
        rm -rf "$WORK_DIR"
    fi

    mkdir -p "$WORK_DIR"

    if [ -f "$ASSETS_DIR/leloir-controlplane-skeleton.zip" ]; then
        info "Extracting control plane skeleton..."
        unzip -q "$ASSETS_DIR/leloir-controlplane-skeleton.zip" -d "$WORK_DIR/"
        ok "leloir-core/ extracted"
    else
        fail "control plane skeleton not found at $ASSETS_DIR/leloir-controlplane-skeleton.zip"
        return 1
    fi

    if [ -f "$ASSETS_DIR/leloir-sdk-v1-skeleton.zip" ]; then
        info "Extracting SDK skeleton..."
        unzip -q "$ASSETS_DIR/leloir-sdk-v1-skeleton.zip" -d "$WORK_DIR/"
        ok "leloir-sdk/ extracted"
    else
        warn "SDK skeleton not found at $ASSETS_DIR/leloir-sdk-v1-skeleton.zip"
    fi

    if [ -f "$ASSETS_DIR/leloir-crds-v1.zip" ]; then
        info "Extracting CRDs..."
        mkdir -p "$WORK_DIR/leloir-core/deploy/crds"
        unzip -q "$ASSETS_DIR/leloir-crds-v1.zip" -d "$WORK_DIR/leloir-core/deploy/"
        ok "deploy/crds/ extracted"
    fi

    # Copy reference docs into the working repo
    info "Copying reference docs into leloir-core/docs/..."
    mkdir -p "$WORK_DIR/leloir-core/docs"
    cp "$BUNDLE_DIR/docs/proposal-leloir-platform-v5.3.md" "$WORK_DIR/leloir-core/docs/design-v5.3.md" 2>/dev/null || true
    cp "$BUNDLE_DIR/docs/leloir-agentadapter-sdk-spec-v1.md" "$WORK_DIR/leloir-core/docs/sdk-spec.md" 2>/dev/null || true
    cp "$BUNDLE_DIR/docs/leloir-milestone-0-plan.md" "$WORK_DIR/leloir-core/docs/m0-plan.md" 2>/dev/null || true
    cp "$BUNDLE_DIR/docs/leloir-architecture.html" "$WORK_DIR/leloir-core/docs/architecture.html" 2>/dev/null || true

    ok "Workspace ready at $WORK_DIR"
    info "  $WORK_DIR/leloir-core/   ← control plane (3 binaries)"
    info "  $WORK_DIR/leloir-sdk/    ← AgentAdapter SDK (Go module)"
}

# ─── Phase 4: Init git ──────────────────────────────────────────────────────
init_git() {
    title "Phase 4: Initialize Git Repo"

    if [ ! -d "$WORK_DIR/leloir-core" ]; then
        fail "$WORK_DIR/leloir-core not found. Run --extract first."
        return 1
    fi

    cd "$WORK_DIR/leloir-core"

    if [ -d .git ]; then
        warn ".git already exists in $WORK_DIR/leloir-core; skipping init"
        return 0
    fi

    info "Initializing git repository..."
    git init -q

    # Set sensible defaults if user hasn't
    git config --local user.name "$(git config --global user.name 2>/dev/null || echo 'Leloir Developer')"
    git config --local user.email "$(git config --global user.email 2>/dev/null || echo 'developer@leloir.dev')"

    git checkout -q -b main 2>/dev/null || git checkout -q main 2>/dev/null || true

    git add -A
    git commit -q -m "Initial commit: control plane skeleton from design v5.3

Includes:
- 3 binaries: controlplane, mcp-gateway, webhook-receiver
- Go module skeleton (~2,100 lines, all subsystems scaffolded)
- CRDs for AgentRegistration, AlertRoute, MCPServer, etc.
- Helm value profiles (corporate + local)
- CI workflow + Dockerfile + Makefile
- Reference design docs in docs/

Generated from leloir-bundle bootstrap script.
Next: follow docs/m0-plan.md for the 2-week PoC."

    ok "Initial commit created"
    info "  Branch: main"
    info "  Working directory: $WORK_DIR/leloir-core"

    echo
    info "Next steps:"
    info "  1. Verify GitHub org 'leloir' is registered (or fallback)"
    info "  2. Create the remote repo: gh repo create leloir/leloir --private"
    info "  3. Add remote: git remote add origin git@github.com:leloir/leloir.git"
    info "  4. Push: git push -u origin main"
    info "  5. Open docs/m0-plan.md and start Day 1"
}

# ─── Final summary ──────────────────────────────────────────────────────────
print_summary() {
    title "Summary"
    echo
    info "Bundle directory: $BUNDLE_DIR"
    info "Workspace:        $WORK_DIR"
    echo

    if [ -d "$WORK_DIR/leloir-core" ]; then
        ok "Control plane skeleton ready"
    fi
    if [ -d "$WORK_DIR/leloir-sdk" ]; then
        ok "SDK skeleton ready"
    fi
    if [ -d "$WORK_DIR/leloir-core/.git" ]; then
        ok "Git repo initialized"
    fi

    echo
    info "What to do next:"
    info "  cd $WORK_DIR/leloir-core"
    info "  cat docs/m0-plan.md"
    info "  # Start Day 1: verify naming, set up kind cluster, install Prometheus + HolmesGPT"
    echo
}

# ─── Main ───────────────────────────────────────────────────────────────────
main() {
    echo
    echo -e "${BOLD}Leloir Bootstrap${RESET}"
    echo "Agentic incident analysis platform — Day 1 of Milestone 0"
    echo

    if $DO_PREREQS; then
        check_prereqs || exit 1
    fi
    if $DO_NAMING; then
        check_naming || true  # don't fail on naming check
    fi
    if $DO_EXTRACT; then
        extract_skeletons || exit 1
    fi
    if $DO_GIT_INIT; then
        init_git || exit 1
    fi

    print_summary
}

main "$@"
