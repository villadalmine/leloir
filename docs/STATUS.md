# Leloir — Project Status

**Última actualización:** 8 de mayo de 2026
**Fase:** M0 PoC COMPLETADO. Infraestructura productiva operativa. Pendiente 3 gates formales antes de M1.

---

## ✅ Infraestructura productiva (corriendo hoy)

### Cluster k3s — single-node en VPS (81.207.69.100)

| Componente | Namespace | URL | Estado |
|---|---|---|---|
| ArgoCD | `argocd` | https://argocd.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| Grafana | `prometheus` | https://grafana.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| Prometheus | `prometheus` | ClusterIP interno | ✅ Up |
| HolmesGPT | `holmesgpt` | ClusterIP interno | ✅ Up, OpenRouter |
| Leloir PoC | `leloir-poc` | https://poc.leloir.cybercirujas.club | ✅ Up, cert prod, GitHub OAuth |
| cert-manager | `cert-manager` | — | ✅ Up, letsencrypt-prod |
| ingress-nginx | `ingress-nginx` | 81.207.69.100 | ✅ Up |
| oauth2-proxy | `leloir-poc` | interno | ✅ Up |

### DNS (Namecheap — cybercirujas.club)

| Registro | Tipo | Destino |
|---|---|---|
| `argocd.leloir` | A | 81.207.69.100 |
| `grafana.leloir` | A | 81.207.69.100 |
| `poc.leloir` | A | 81.207.69.100 |
| `_acme-challenge.grafana.leloir` | CNAME | `2ac45694-c387-43f2-8f26-6b46c7de85cb.auth.acme-dns.io` |
| `_acme-challenge.argocd.leloir` | CNAME | `58fe7dc9-4c6a-448c-b36f-616af8a45f39.auth.acme-dns.io` |

### TLS / Certificados
- Todos **letsencrypt-prod** (CA: Let's Encrypt R12)
- Renovación automática: cert-manager + acme-dns (DNS-01)
- Credenciales acme-dns en `acme-dns-account.json` (gitignored) y secret `acme-dns-account` en `cert-manager`

### Autenticación
- **ArgoCD**: GitHub OAuth via Dex. Admin local DESHABILITADO. Solo `villadalmine` → Admin.
- **Grafana**: GitHub OAuth. Login form deshabilitado. Solo `villadalmine` → Admin.
- **PoC**: GitHub OAuth via oauth2-proxy. Solo `villadalmine@gmail.com` autorizado.

---

## ✅ PoC — funcionalidades implementadas

### Backend Go (`poc/main.go`)
- Webhook receiver para Alertmanager (formato `amPayload`)
- SSE broadcast hub (múltiples clientes, sin goroutine leaks)
- Cliente Holmes con retry automático en rate-limit 429
- Fallback de modelo: primary → fallback si RateLimitError
- Tool calls de Holmes filtrados y enviados como eventos `thinking` (sin `TodoWrite`/`TodoRead`)
- Truncado de descripciones de tool calls a 120 chars

### Frontend (`poc/static/index.html`)
- UI dark mode, responsive
- Pregunta manual al Holmes desde la UI
- SSE live stream con tipos: `connected`, `info`, `thinking`, `answer`, `error`
- Botón "🧠 Thinking" toggle: muestra/oculta tool calls de Holmes (ON/OFF visual claro)
- Botón "Limpiar" — limpia el log de eventos
- Botón "Logout" — invalida cookie oauth2-proxy + redirige a github.com/logout
- Guard contra doble-submit en el formulario
- Prevención de duplicados SSE (cierra conexión vieja antes de abrir nueva)
- Markdown rendering en respuestas de Holmes

### CI/CD (`.github/workflows/build-poc.yaml`)
- Trigger: push a `poc/**` o `dispatch` manual
- Build imagen: `ghcr.io/villadalmine/leloir-poc:<sha-XXXXXXX>` + `latest`
- Auth: `GHCR_PAT` (Classic PAT, `write:packages` + `repo`)
- Post-build: auto-update `deploy/apps/leloir-poc/values.yaml` con nuevo tag SHA y push `[skip ci]`
- ArgoCD syncea automáticamente → deploy sin intervención humana

### HolmesGPT — modelos configurados

| Alias | Modelo OpenRouter | Estado |
|---|---|---|
| `nemotron-super` | `nvidia/nemotron-3-super-120b-a12b:free` | ✅ Primary |
| `gemma4-31b` | `google/gemma-4-31b-it:free` | ✅ Fallback |
| `qwen3-80b` | `qwen/qwen3-next-80b-a3b-instruct:free` | ⚠️ Descartado — rate-limited frecuente |

**Configuración activa en HolmesGPT:**
- `LOG_LEVEL: WARNING` — suprime AI reasoning logs en stdout del pod
- `OVERRIDE_MAX_OUTPUT_TOKEN: 4096` — limita output para menor latencia
- `OVERRIDE_MAX_CONTENT_SIZE: 128000` — silencia warnings de LiteLLM sobre modelos desconocidos

---

## 🖥️ Cómo prender todo desde cero (runbook)

El cluster k3s corre dentro de un container `podman` en esta laptop.
Cuando apagás la laptop, el container queda **parado pero no eliminado** — todo el estado de Kubernetes se preserva. ArgoCD y los Helm charts no necesitan reinstalarse.

### Caso normal: reinicio de laptop / apagado y prendido

```bash
# 1. Levantar el cluster (desde fuera del toolbx, como root)
sudo ./scripts/cluster-up.sh

# 2. Verificar que todo levantó (puede tardar ~2 min)
kubectl get pods -A
```

Eso es todo. ArgoCD detecta si hay cambios en git y sincroniza solo.

### Verificar que los servicios públicos responden

```bash
curl -sI https://poc.leloir.cybercirujas.club | head -3
curl -sI https://argocd.leloir.cybercirujas.club | head -3
curl -sI https://grafana.leloir.cybercirujas.club | head -3
```

---

### Caso excepcional: cluster destruido o máquina nueva

Solo necesario si corriste `sudo ./scripts/cluster-up.sh --down` o si cambiás de máquina. Los secrets no están en git y hay que recrearlos.

**Paso 1 — Levantar cluster limpio**
```bash
sudo ./scripts/cluster-up.sh
```

**Paso 2 — Instalar ArgoCD**
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

**Paso 3 — Aplicar los apps de ArgoCD desde el repo**
```bash
# ArgoCD levanta todo lo demás solo via GitOps
kubectl apply -f deploy/argocd-appset.yaml   # o el ApplicationSet que tengas
```

**Paso 4 — Recrear secrets (los únicos que no están en git)**

```bash
# Secret acme-dns para cert-manager (renovación TLS automática)
# El archivo acme-dns-account.json está en la raíz del repo (gitignored)
# Backupearlo en el password manager
kubectl create secret generic acme-dns-account \
  --namespace cert-manager \
  --from-file=acme-dns-account.json \
  --dry-run=client -o yaml | kubectl apply -f -

# OAuth para ArgoCD (Dex) y Grafana
./scripts/github-oauth-setup.sh
# Pide: ArgoCD client ID/secret y Grafana client ID/secret
# (los encontrás en https://github.com/settings/developers)

# OAuth para el PoC (oauth2-proxy)
./scripts/github-poc-oauth-setup.sh
# Pide: PoC client ID/secret
# (GitHub OAuth App: callback = https://poc.leloir.cybercirujas.club/oauth2/callback)

# Pull secret para ghcr.io (imagen del PoC)
kubectl create secret docker-registry ghcr-pull-secret \
  --namespace leloir-poc \
  --docker-server=ghcr.io \
  --docker-username=villadalmine \
  --docker-password=<TU_GHCR_PAT> \
  --dry-run=client -o yaml | kubectl apply -f -

# API key de OpenRouter para HolmesGPT
# (está en deploy/apps/holmesgpt/values.yaml como OPENAI_API_KEY)
# Si fue borrado del values.yaml, consultar el password manager
```

**Paso 5 — Rollout para que tomen los nuevos secrets**
```bash
kubectl rollout restart deployment -n argocd
kubectl rollout restart deployment -n prometheus
kubectl rollout restart deployment -n holmesgpt
kubectl rollout restart deployment -n leloir-poc
```

---

### Comandos útiles de diagnóstico

```bash
# Ver estado general
kubectl get pods -A | grep -v Running | grep -v Completed

# Ver logs del PoC
kubectl logs -n leloir-poc deploy/leloir-poc-leloir --tail=50 -f

# Ver logs de HolmesGPT
kubectl logs -n holmesgpt deploy/holmesgpt-holmes --tail=50 -f

# Ver estado de certificados
kubectl get certificates -A
kubectl get certificaterequests -A

# Ver sync de ArgoCD
kubectl get applications -n argocd

# Bajar el cluster (preserva estado)
sudo ./scripts/cluster-up.sh --down

# Ver estado del container k3s
sudo ./scripts/cluster-up.sh --status
```

---

## 🔍 Hallazgos técnicos de M0 (input para M1)

### Sobre la API de HolmesGPT (`/api/chat`)

**Lo que expone el endpoint hoy:**
```json
{
  "analysis": "respuesta final en markdown",
  "tool_calls": [
    { "tool_name": "kubernetes_count", "description": "kubectl get pods | jq ..." },
    { "tool_name": "TodoWrite", "description": "..." }
  ]
}
```

**Lo que NO expone (solo en logs del servidor):**
- Bloques `AI reasoning:` — chain-of-thought del modelo entre tool calls
- Output de cada tool call individual (tablas de resultados)
- Intermediate state del `TodoWrite` (lista de tareas interna de Holmes)

**Consecuencia para M1:** Para streaming real del razonamiento necesitaría un endpoint SSE en HolmesGPT o acceso a logs del pod. Por ahora mostramos los `tool_calls` finales como proxy de "thinking".

### Sobre modelos OpenRouter free tier
- Rate limits son agresivos y rotativos — ningún modelo es confiable solo
- El fallback implementado (primary → fallback en 429) es necesario y funciona
- Para producción M1: necesita LLM Gateway con múltiples providers o API key de pago

### Sobre la arquitectura del PoC
- SSE broadcast funciona bien para single-tenant
- Para multi-tenant M1: necesita isolation por tenant (canales separados, no broadcast global)
- Holmes API no tiene autenticación → expuesto solo dentro del cluster (ClusterIP), bien

---

## 📋 Gates de M0 — estado actual

| Gate | Estado |
|---|---|
| Demo end-to-end funciona (alerta → Holmes → respuesta en UI) | ✅ Funciona |
| Infraestructura con certs prod y OAuth real | ✅ Hecho |
| HolmesGPT integrado con fallback de modelo | ✅ Hecho |
| CI/CD para imagen PoC (build + push + deploy auto) | ✅ GitHub Actions |
| Dominio operativo | ✅ `leloir.cybercirujas.club` |
| `CONTRACT.md` con la API de Holmes | ⬜ **Pendiente de redactar** |
| 3 demos exitosas seguidas | ⬜ **Pendiente de validar** |
| Stakeholder signoff en v5.3 | ⬜ Pendiente (o confirmar si aplica) |
| Equipo para M1 identificado | ⬜ Pendiente (o confirmar si solo) |

---

## 🔴 Pendiente inmediato (antes de arrancar M1)

| # | Item | Detalle |
|---|---|---|
| P1 | Redactar `CONTRACT.md` | Documentar `/api/chat`: request/response schema, campos, modelos, latencia, tool_calls, limitaciones de streaming |
| P2 | 3 demos exitosas seguidas | Disparar alerta real → ver investigación → ver respuesta. Documentar en `LEARNINGS.md` |
| P3 | Decidir cluster para M1 | ¿Sigue en k3s/kind single-node? ¿Migrar a multi-node k3s nativo? |
| P4 | LLM para M1 | OpenRouter free no es suficiente para producción. ¿Azure OpenAI? ¿API key de pago? |
| P5 | Stakeholder/equipo | ¿Solo o con equipo? Decisión formal antes de comprometer 25 semanas |
| P6 | Actualizar Helm chart versions | Revisar ArtifactHub para ArgoCD, Prometheus, HolmesGPT, cert-manager |

---

## 🟡 Deferred (decidir más adelante)

| # | Item | Cuándo |
|---|---|---|
| D1 | Streaming real del AI reasoning de Holmes | M1 si HolmesGPT expone SSE endpoint |
| D2 | Régimen de compliance específico (DORA/SOC2/HIPAA) | Antes de v1.1 |
| D3 | VMware MCP wrapper | Backlog post-v1.0 |
| D4 | CNCF Sandbox | Post-v1.0 si hay tracción |
| D5 | Memoria cross-investigation nativa | Post-v1.0 si hay demanda |
| D6 | Sidecar adapters no-Go para SDK | SDK v2 / M5+ |
| D7 | Talk en KubeCon | Post-v1.0 |
| D8 | Notificaciones a Teams/Telegram | Estaba en M0 plan, quedó sin hacer |

---

## 📅 Siguiente paso concreto

**Completar los 3 gates pendientes de M0, luego iniciar M1.**

### Orden recomendado:
1. **`CONTRACT.md`** — 2-3 horas. Formato libre, capturar lo que descubrimos sobre Holmes API
2. **3 demos** — disparar `KubePodCrashLooping` o similar real, documentar en `LEARNINGS.md`
3. **Decisión de equipo/cluster** — sync corto, definir antes de M1 day 1
4. **Iniciar M1** — leer `docs/proposal-leloir-platform-v5.3.md` §0-3, luego arrancar control plane Go

### M1 entregables principales (preview):
- `leloir-core/` compilando: `leloir-operator`, `leloir-api`, `leloir-gateway` (3 binarios)
- CRDs aplicados al cluster (en `assets/leloir-crds-v1.zip`)
- `leloir-sdk/` con conformance suite pasando (5 métodos AgentAdapter)
- UI React básica reemplazando el HTML estático del PoC
- Single-tenant, production-quality (con auth, con audit log)

---

## 📊 Roadmap maestro

| Milestone | Duración | Estado | Deliverable clave |
|---|---|---|---|
| **M0 — PoC** | 2 semanas | ✅ **COMPLETADO** (3 gates formales pendientes) | Demo: alerta → Holmes → respuesta en UI. Infra productiva con TLS/OAuth/CI. |
| **M1 — MVP production-shape** | 5-6 semanas | ⬜ Listo para iniciar post gates | Control plane Go + CRDs + SDK + UI React. Single-tenant prod-quality. |
| **M2 — Multi-tenancy** | 4-5 semanas | ⬜ Planificado | OIDC, RBAC, audit log, MCP Gateway scoping, Skills v1 |
| **M3 — Cost + SDK + A2A** | 5-6 semanas | ⬜ Planificado | LLM Gateway, TenantBudget, Go SDK público, 3 adapters (HolmesGPT, Hermes, OpenCode) |
| **M4 — Safety + A2A v1** | 5-6 semanas | ⬜ Planificado | HITL, Vault, prompt injection defenses, A2A Pattern B |
| **M5 — Ecosystem + A2A polish** | 5-6 semanas | ⬜ Planificado | Claude Code adapter, cloud sources (AWS/Azure), tree view UI |
| **M6 — Hardening + v1.0** | 3-4 semanas | ⬜ Planificado | Security review, OSS docs completos, public v1.0 release |

**Total a v1.0:** ~25-30 semanas desde inicio de M1.
**Valor cada milestone:** cada entrega es deployable y usable de forma independiente.

---

## 🗂️ Archivos clave

| Archivo | Contenido |
|---|---|
| `docs/proposal-leloir-platform-v5.3.md` | Design doc master (2042 líneas) — fuente de verdad de arquitectura |
| `docs/leloir-agentadapter-sdk-spec-v1.md` | Contrato técnico del SDK AgentAdapter |
| `docs/leloir-milestone-0-plan.md` | Plan día a día del PoC original |
| `docs/STATUS.md` | Este archivo — estado actual del proyecto |
| `deploy/` | Helm charts + ArgoCD ApplicationSets — infra actual productiva |
| `deploy/apps/holmesgpt/values.yaml` | Config HolmesGPT: modelos, LOG_LEVEL, token limits |
| `deploy/apps/leloir-poc/values.yaml` | Config PoC: imagen, modelo, fallback, ingress |
| `deploy/apps/argocd-config/` | ArgoCD ConfigMaps: OAuth (Dex), RBAC policy |
| `deploy/apps/prometheus/values.yaml` | Prometheus + Grafana: OAuth, root_url |
| `leloir-core/` | Control plane Go (3 binarios) — **pendiente, empieza en M1** |
| `leloir-sdk/` | AgentAdapter SDK Go module — **pendiente, empieza en M1** |
| `poc/` | PoC Go app — webhook + SSE UI + Holmes client + thinking toggle |
| `poc/main.go` | Backend Go del PoC |
| `poc/static/index.html` | UI del PoC (dark mode, thinking toggle, SSE live stream) |
| `.github/workflows/build-poc.yaml` | CI: build + push ghcr.io + update values.yaml |
| `assets/leloir-crds-v1.zip` | 8 CRDs de Kubernetes + Helm profiles — aplicar en M1 |
| `acme-dns-account.json` | Credenciales acme-dns (gitignored — backupear en password manager) |

---

## 🔑 Secrets / credenciales activas

| Secret | Dónde | Usado por |
|---|---|---|
| `GHCR_PAT` | GitHub repo secrets | GitHub Actions — push a ghcr.io |
| `acme-dns-account` | k8s secret `cert-manager` | cert-manager DNS-01 challenge |
| `ghcr-pull-secret` | k8s secret `leloir-poc` | Pod pull de ghcr.io |
| `dex-github-client` | k8s secret `argocd` | ArgoCD GitHub OAuth |
| Grafana GitHub OAuth | Helm values (en cluster) | Grafana GitHub OAuth |
| OpenRouter API key | HolmesGPT deployment | LiteLLM provider |

---

*Documento vivo. Actualizar después de cada sesión de trabajo, cuando cambian decisiones, o cuando se resuelven pendientes.*
