# Leloir — Instrucciones para Claude Code

## Qué es esto
Plataforma open-source de análisis agentico de incidentes para Kubernetes.
Apache 2.0. Go backend + React frontend. Nombre: Leloir (Nobel argentino de Química 1970).

## Dónde está todo
- docs/STATUS.md                          → estado actual, qué está decidido, qué falta
- docs/ONBOARDING.md                      → tour guiado para entender el proyecto
- docs/proposal-leloir-platform-v5.3.md   → design doc master (2042 líneas)
- docs/leloir-agentadapter-sdk-spec-v1.md → contrato técnico del SDK
- docs/leloir-milestone-0-plan.md         → plan día a día del PoC de 2 semanas
- leloir-core/                            → control plane Go (3 binarios)
- leloir-sdk/                             → AgentAdapter SDK Go module
- assets/leloir-crds-v1.zip              → 8 CRDs de Kubernetes + Helm profiles

## Estado actual
Diseño cerrado. Ready para Milestone 0 (PoC de 2 semanas).
Siguiente acción: seguir docs/leloir-milestone-0-plan.md Día 1.

## Decisiones ya tomadas — no re-debatir
- Nombre: Leloir. License: Apache 2.0.
- Control plane independiente (Go, sin kagent).
- AgentAdapter: 5 métodos, contrato estable, conformance suite.
- A2A first-class: canInvoke (agent) + team (route) + ApprovalPolicy + budget 4-layer min.
- Memoria: integración vía MCPServer CRD, no feature nativa en v1.0.
- Profiles: corporate (Azure AD + Vault) y local (Dex + Anthropic).
- M1: HolmesGPT only. M3: Hermes + OpenCode + SDK público. M5: Claude Code.

## Para compilar
cd leloir-core && go mod tidy && go build ./...
cd leloir-sdk && go mod tidy && go test ./...

## Próximo paso concreto
Leer docs/leloir-milestone-0-plan.md y arrancar Día 1:
verificar naming (github.com/leloir, leloir.dev), instalar kind cluster,
instalar Prometheus + HolmesGPT.
