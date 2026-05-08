# Leloir — Project Status

**Date of this snapshot:** May 8, 2026
**Phase:** M0 PoC COMPLETADO. Cluster productivo corriendo. Listo para iniciar M1.

---

## ✅ Lo que está corriendo hoy (infraestructura real)

### Cluster k3s (kind) — single-node

| Componente | Namespace | URL | Estado |
|---|---|---|---|
| ArgoCD | `argocd` | https://argocd.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| Grafana | `prometheus` | https://grafana.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| Prometheus | `prometheus` | ClusterIP interno | ✅ Up |
| HolmesGPT | `holmesgpt` | ClusterIP interno | ✅ Up |
| Leloir PoC | `leloir-poc` | https://poc.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| cert-manager | `cert-manager` | — | ✅ Up, letsencrypt-prod |
| ingress-nginx | `ingress-nginx` | 81.207.69.100 | ✅ Up |

### DNS (Namecheap — cybercirujas.club)

| Registro | Tipo | Destino |
|---|---|---|
| `argocd.leloir` | A | 81.207.69.100 |
| `grafana.leloir` | A | 81.207.69.100 |
| `poc.leloir` | A | 81.207.69.100 |
| `_acme-challenge.grafana.leloir` | CNAME | `2ac45694-c387-43f2-8f26-6b46c7de85cb.auth.acme-dns.io` |
| `_acme-challenge.argocd.leloir` | CNAME | `58fe7dc9-4c6a-448c-b36f-616af8a45f39.auth.acme-dns.io` |

### Certificados TLS
- Todos **letsencrypt-prod** (CA: Let's Encrypt R12)
- Renovación automática via cert-manager + acme-dns (DNS-01)
- Credenciales acme-dns en `acme-dns-account.json` (gitignored) y secret `acme-dns-account` en `cert-manager`

### Autenticación
- **ArgoCD**: GitHub OAuth via Dex. Local admin DESHABILITADO (`admin.enabled: "false"`). Solo `villadalmine` tiene rol Admin.
- **Grafana**: GitHub OAuth. Login form DESHABILITADO (`disable_login_form: true`). Solo `villadalmine` → Admin.
- **PoC**: GitHub OAuth via oauth2-proxy. Solo `villadalmine@gmail.com` autorizado.

### HolmesGPT — modelos configurados
| Alias | Modelo OpenRouter | Estado |
|---|---|---|
| `gemma4-31b` | `google/gemma-4-31b-it:free` | ✅ Funciona |
| `nemotron-super` | `nvidia/nemotron-3-super-120b-a12b:free` | ✅ Funciona |
| `qwen3-80b` | `qwen/qwen3-next-80b-a3b-instruct:free` | ⚠️ Rate-limited frecuente |

**Modelo activo en PoC:** `nemotron-super` (primary) → `gemma4-31b` (fallback automático en 429)

### CI/CD
- GitHub Actions: `.github/workflows/build-poc.yaml` — build + push a `ghcr.io/villadalmine/leloir-poc:latest` en cada push a `poc/**`
- Secret requerido: `GHCR_PAT` (Classic PAT con `write:packages` + `repo`)
- ArgoCD syncea automáticamente desde `github.com/villadalmine/leloir` rama `main`

---

## ✅ Lo que está decidido (sin re-debatir)

### Estratégico (12/12 preguntas resueltas)

| # | Decisión | Resuelto como |
|---|---|---|
| Q1 | Arquitectura | Control plane independiente (sin kagent) |
| Q2 | Agentes Day-1 | HolmesGPT only en M1 |
| Q3 | Fuentes Day-1 | Prometheus + Kubernetes (M1); AWS/Azure (M5) |
| Q4 | LLM providers | Corporate: Azure OpenAI. Local: Anthropic + OpenRouter |
| Q5 | Modo default | Centralizado en M1-M2; per-namespace evaluado en M5 |
| Q6 | OIDC IdP | Azure AD + Dex desde M1 |
| Q7 | Compliance | Baseline pragmático v1.0; hardening iterativo |
| Q8 | OSS o interno | Open source, Apache 2.0 |
| Q9 | Escala tenant | Sin límite fijo, diseñado para escala arbitraria |
| Q10 | On-call workflows | Solo análisis en v1.0; integrar con on-call externo |
| Q11 | Custom agents | A2A first-class; SDK es deliverable estratégico en M3 |
| Q12 | gRPC | HTTP default M1-M2; gRPC para MCPs internos M3-M4 |

### Sub-decisiones de arquitectura
- **A2A authorization:** defense in depth — `AgentRegistration.canInvoke` AND `AlertRoute.team` ambos deben permitir
- **A2A budget propagation:** 4-layer min — tenant > route > parent_remaining > caller_hint
- **A2A loop prevention:** depth=5, fan-out=3, total=20 + cycle detection + hard error default
- **Memory strategy:** integración via MCPServer CRD; no subsistema nativo de memoria en v1.0

---

## ✅ Gates de M0 — estado actual

| Gate | Estado |
|---|---|
| Demo end-to-end funciona (alerta → Holmes → respuesta en UI) | ✅ Funciona |
| Infraestructura con certs prod y OAuth real | ✅ Hecho |
| HolmesGPT integrado con fallback de modelo | ✅ Hecho |
| CI/CD para imagen PoC | ✅ GitHub Actions |
| Dominio operativo | ✅ `leloir.cybercirujas.club` |
| CONTRACT.md con la API de Holmes | ⬜ Pendiente de redactar |
| 3 demos exitosas seguidas | ⬜ Pendiente de validar |
| Stakeholder signoff en v5.3 | ⬜ Pendiente |
| Equipo para M1 identificado | ⬜ Pendiente |

---

## 🔴 Pendiente inmediato (antes de M1)

| # | Item |
|---|---|
| P1 | Redactar `CONTRACT.md` documentando la API de HolmesGPT (`/api/chat`, campos, modelos) |
| P2 | Hacer 3 demos exitosas seguidas con alertas reales |
| P3 | Decidir si el cluster sigue en k3s/kind o migrar a k3s nativo para M1 |
| P4 | Azure OpenAI access request (puede tardar semanas) |
| P5 | Identificar equipo para M1 |
| P6 | Stakeholder signoff en `proposal-leloir-platform-v5.3.md` |

---

## 🟡 Deferred (decidir más adelante)

| # | Item | Cuándo |
|---|---|---|
| D1 | Régimen de compliance específico (DORA/SOC2/HIPAA) | Antes de v1.1 |
| D2 | VMware MCP wrapper | Backlog post-v1.0 |
| D3 | CNCF Sandbox | Post-v1.0 si hay tracción |
| D4 | Memoria cross-investigation nativa | Post-v1.0 si hay demanda |
| D5 | Sidecar adapters no-Go para SDK | SDK v2 / M5+ |
| D6 | Talk en KubeCon | Post-v1.0 |

---

## 📅 Siguiente paso concreto

**Iniciar M1** — production-shape MVP.

Prerrequisitos antes de arrancar:
1. Leer `docs/proposal-leloir-platform-v5.3.md` §0 (cambios) + §1-3 (arquitectura core)
2. Redactar `CONTRACT.md` con la API de HolmesGPT como está hoy
3. Hacer 3 demos exitosas del PoC

M1 entregables principales:
- Control plane Go (`leloir-core/`) con los 3 binarios compilando y testeados
- CRDs aplicados al cluster (en `assets/leloir-crds-v1.zip`)
- SDK Go (`leloir-sdk/`) con conformance suite pasando
- UI React básica

---

## 📊 Roadmap maestro

| Milestone | Duración | Estado | Deliverable clave |
|---|---|---|---|
| **M0 — PoC** | 2 semanas | ✅ COMPLETADO | Demo: alerta → Holmes → respuesta en UI. Infra productiva. |
| **M1 — MVP production-shape** | 5-6 semanas | ⬜ Listo para iniciar | Sistema single-tenant prod-quality + repo OSS público |
| **M2 — Multi-tenancy** | 4-5 semanas | ⬜ Planificado | OIDC, RBAC, audit, MCP Gateway scoping, Skills v1 |
| **M3 — Cost + SDK + A2A** | 5-6 semanas | ⬜ Planificado | LLM Gateway, TenantBudget, Go SDK público, 3 adapters |
| **M4 — Safety + A2A v1** | 5-6 semanas | ⬜ Planificado | HITL, Vault, prompt injection defenses, A2A Pattern B |
| **M5 — Ecosystem + A2A polish** | 5-6 semanas | ⬜ Planificado | Claude Code adapter, cloud sources, tree view UI |
| **M6 — Hardening + v1.0** | 3-4 weeks | ⬜ Planificado | Security review, OSS docs, public v1.0 release |

**Total a v1.0:** ~25-30 semanas. **Entrega modular:** valor cada 4-6 semanas.

---

## 🗂️ Archivos clave

| Archivo | Contenido |
|---|---|
| `docs/proposal-leloir-platform-v5.3.md` | Design doc master (2042 líneas) — fuente de verdad de arquitectura |
| `docs/leloir-agentadapter-sdk-spec-v1.md` | Contrato técnico del SDK |
| `docs/leloir-milestone-0-plan.md` | Plan día a día del PoC |
| `deploy/` | Helm charts + ArgoCD ApplicationSets — infra actual |
| `leloir-core/` | Control plane Go (3 binarios) — pendiente de desarrollo M1 |
| `leloir-sdk/` | AgentAdapter SDK Go module — pendiente de desarrollo M1 |
| `poc/` | PoC Go app — webhook receiver + SSE UI + Holmes client |
| `assets/leloir-crds-v1.zip` | 8 CRDs de Kubernetes + Helm profiles |
| `acme-dns-account.json` | Credenciales acme-dns (gitignored — backupear en password manager) |

---

*Documento vivo. Actualizar después de cada milestone, cuando cambian decisiones, o cuando se resuelven pendientes.*
