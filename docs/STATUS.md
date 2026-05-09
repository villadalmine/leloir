# Leloir — Project Status

**Última actualización:** 9 de mayo de 2026
**Fase:** M0 PoC COMPLETADO + kickstart de M1 implementado (control plane + SDK + wiring base).

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

### Incidente resuelto — ArgoCD OAuth login sin permisos (9 mayo 2026)
- **Síntoma:** login con GitHub exitoso pero UI vacía/sin acceso a apps.
- **Causa raíz:** RBAC evaluaba claims distintos (Dex `federated_claims.user_id` / scopes por default) y la policy no matcheaba correctamente contra el usuario autenticado.
- **Fix persistente en git:** 
  - `deploy/apps/argocd-config/argocd-rbac-cm.yaml` → `scopes: '[groups, email]'` + binding explícito por email.
  - `deploy/apps/argocd-config/argocd-cm.yaml` → Dex GitHub con `getUserInfo: true` y `userNameKey: login`.
  - Se removió `oidc.config` porque rompía login (`unsupported protocol scheme ""`).
- **Lección operativa:** no parchear solo live en cluster; con ArgoCD self-heal los cambios manuales se revierten. Primero commit/push en `deploy/apps/argocd-config/` y luego refresh/sync.

### Incidente resuelto — HolmesGPT `Sync: Unknown` en ArgoCD (9 mayo 2026)
- **Síntoma:** app `holmesgpt` en `Unknown/NotSync` aunque los pods estaban sanos.
- **Causa raíz:** `ComparisonError` de ArgoCD por `env` duplicado (`LOG_LEVEL`) en el `Deployment` de Holmes.
- **Fix persistente en git:** `deploy/apps/holmesgpt/values.yaml` ahora usa `holmes.logLevel: "WARNING"` (campo nativo del chart) y elimina `LOG_LEVEL` de `holmes.additionalEnvVars`.
- **Estado final validado:** todas las apps del namespace `argocd` en `Synced/Healthy`.

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
- `holmes.logLevel: WARNING` — suprime AI reasoning logs en stdout del pod sin duplicar `env`
- `OVERRIDE_MAX_OUTPUT_TOKEN: 4096` — limita output para menor latencia
- `OVERRIDE_MAX_CONTENT_SIZE: 128000` — silencia warnings de LiteLLM sobre modelos desconocidos

---

## 🖥️ Cómo prender todo desde cero (runbook)

El cluster k3s corre dentro de un container `podman` en esta máquina Fedora.
Cuando apagás la máquina, el container queda **parado pero no eliminado** — todo el estado de Kubernetes se preserva. No hay que reinstalar nada.

---

### Caso normal: reinicio / apagado y prendido

```bash
# Desde fuera del toolbx, como root
sudo ./scripts/cluster-up.sh

# Verificar que todo levantó (~2 min)
kubectl get pods -A
```

Eso es todo. ArgoCD sincroniza solo si hubo cambios en git.

```bash
# Verificar que los 3 dominios responden
curl -sI https://poc.leloir.cybercirujas.club | head -2
curl -sI https://argocd.leloir.cybercirujas.club | head -2
curl -sI https://grafana.leloir.cybercirujas.club | head -2
```

---

### Caso excepcional: cluster destruido o máquina nueva

Solo si corriste `--down` (borra el container y todo su estado) o migrás a otra máquina.

#### A) DNS — registros en Namecheap

**URL:** https://ap.www.namecheap.com/domains/domaincontrolpanel/cybercirujas.club/advancedns

**A records** (apuntan a la IP pública donde corre k3s):

| Host | Tipo | Valor |
|---|---|---|
| `argocd.leloir` | A Record | `81.207.69.100` |
| `grafana.leloir` | A Record | `81.207.69.100` |
| `poc.leloir` | A Record | `81.207.69.100` |

**CNAME records** (para renovación automática de TLS via acme-dns — no tocar):

| Host | Tipo | Valor |
|---|---|---|
| `_acme-challenge.argocd.leloir` | CNAME | `58fe7dc9-4c6a-448c-b36f-616af8a45f39.auth.acme-dns.io` |
| `_acme-challenge.grafana.leloir` | CNAME | `2ac45694-c387-43f2-8f26-6b46c7de85cb.auth.acme-dns.io` |

> Si necesitás regenerar las cuentas acme-dns (cuenta nueva → nuevo CNAME):
> ```bash
> ./scripts/acme-dns-register.sh
> # Imprime los nuevos CNAMEs a pegar en Namecheap
> # Guarda las credenciales en acme-dns-account.json (backupear en password manager)
> ```

#### B) GitHub OAuth Apps — 3 apps a crear

**URL:** https://github.com/settings/developers → "OAuth Apps" → "New OAuth App"

**App 1 — ArgoCD**
```
Name:         Leloir ArgoCD
Homepage URL: https://argocd.leloir.cybercirujas.club
Callback URL: https://argocd.leloir.cybercirujas.club/api/dex/callback
```

**App 2 — Grafana**
```
Name:         Leloir Grafana
Homepage URL: https://grafana.leloir.cybercirujas.club
Callback URL: https://grafana.leloir.cybercirujas.club/login/github
```

**App 3 — PoC**
```
Name:         Leloir PoC
Homepage URL: https://poc.leloir.cybercirujas.club
Callback URL: https://poc.leloir.cybercirujas.club/oauth2/callback
```

Cargar los client ID/secret en el cluster:
```bash
./scripts/github-oauth-setup.sh      # App 1 (ArgoCD) + App 2 (Grafana) en un solo paso
./scripts/github-poc-oauth-setup.sh  # App 3 (PoC)
```

#### C) GitHub PAT para ghcr.io

**URL:** https://github.com/settings/tokens → "Generate new token (classic)"

```
Note:       Leloir ghcr push
Expiration: No expiration
Scopes:     ✅ write:packages   ✅ read:packages   ✅ repo
```

Usarlo en dos lugares:
1. **GitHub Actions secret** → https://github.com/villadalmine/leloir/settings/secrets/actions → `GHCR_PAT`
2. **K8s pull secret** para que los pods puedan bajar la imagen (ver paso 4)

#### D) OpenRouter API key

**URL:** https://openrouter.ai/settings/keys → "Create Key"

El valor va en `deploy/apps/holmesgpt/values.yaml`:
```yaml
additionalEnvVars:
  - name: OPENAI_API_KEY
    value: "sk-or-v1-XXXXXXXXX"
```

---

#### Pasos para rebuild completo

```bash
# 1. Cluster vacío
sudo ./scripts/cluster-up.sh

# 2. ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=120s

# 3. Secret acme-dns (necesario para que cert-manager emita certs)
kubectl create namespace cert-manager
kubectl create secret generic acme-dns-account \
  --namespace cert-manager \
  --from-file=credentials.json=./acme-dns-account.json \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. OAuth secrets
./scripts/github-oauth-setup.sh      # ArgoCD + Grafana
./scripts/github-poc-oauth-setup.sh  # PoC

# 5. Pull secret para imagen del PoC
kubectl create namespace leloir-poc
kubectl create secret docker-registry ghcr-pull-secret \
  --namespace leloir-poc \
  --docker-server=ghcr.io \
  --docker-username=villadalmine \
  --docker-password=<GHCR_PAT> \
  --dry-run=client -o yaml | kubectl apply -f -

# 6. Aplicar ApplicationSets → ArgoCD instala todo lo demás
kubectl apply -f deploy/

# 7. Esperar certs (~5 min)
kubectl get certificates -A -w
```

---

### Comandos útiles de diagnóstico

```bash
# Estado general (solo pods con problema)
kubectl get pods -A | grep -Ev "Running|Completed"

# Logs del PoC
kubectl logs -n leloir-poc deploy/leloir-poc-leloir --tail=50 -f

# Logs de HolmesGPT
kubectl logs -n holmesgpt deploy/holmesgpt-holmes --tail=50 -f

# Estado de certificados TLS
kubectl get certificates -A

# Sync de ArgoCD
kubectl get applications -n argocd

# Bajar el cluster (preserva estado del container)
sudo ./scripts/cluster-up.sh --down

# Estado del container k3s
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
| `CONTRACT.md` con la API de Holmes | ✅ Hecho (`docs/CONTRACT.md`) |
| 3 demos exitosas seguidas | ✅ Flujo E2E validado en entorno local + VPS |
| Stakeholder signoff en v5.3 | ✅ Continuidad aprobada para ejecutar plan M1 |
| Equipo para M1 identificado | ✅ Ejecución inicial en modo solo-dev |

---

## ✅ Kickstart M1 completado (9 mayo 2026)

| # | Item | Resultado |
|---|---|---|
| M1-1 | `docs/CONTRACT.md` | ✅ Redactado con contrato real de Holmes `/api/chat` |
| M1-2 | `POST /api/v1/alerts` + routing + orchestrator | ✅ Implementado en control plane |
| M1-3 | `GET /api/v1/investigations/{id}/stream` (SSE) | ✅ Implementado con broker |
| M1-4 | Holmes adapter (`leloir-sdk/examples/holmesgpt`) | ✅ Completado + conformance pasando |
| M1-5 | Wiring de control plane (registro adapter + routes por config) | ✅ Implementado en `server` + `config.local.yaml` |

Commits de referencia del kickstart:
- `5274617` `feat(core): implement alert ingestion and investigation endpoints`
- `b991fa3` `feat(sdk): align Holmes adapter with synchronous API contract`
- `fd3bb36` `feat(core): wire adapter registration and route seeding from config`

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

**Entrar en M1 ejecución iterativa sobre base ya implementada.**

### Orden recomendado (próxima iteración):
1. Persistencia de investigaciones y eventos en store duradero (Postgres para profile no-local)
2. Completar handlers faltantes (`/routes` enriched, `/mcp-servers`, `/audit` query)
3. Integrar webhook receiver externo con retries/idempotencia y tests de contrato
4. Fortalecer adapter Holmes (budget warnings + mapping de evidencia estructurada)
5. Preparar demo M1 del flujo completo con UI consumiendo SSE del control plane (no del PoC legacy)

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
| **M0 — PoC** | 2 semanas | ✅ **COMPLETADO** | Demo: alerta → Holmes → respuesta en UI. Infra productiva con TLS/OAuth/CI. |
| **M1 — MVP production-shape** | 5-6 semanas | 🟡 **EN PROGRESO (kickstart completado)** | Control plane Go + CRDs + SDK + UI React. Single-tenant prod-quality. |
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
| `deploy/apps/holmesgpt/values.yaml` | Config HolmesGPT: modelos, `holmes.logLevel`, token limits |
| `deploy/apps/leloir-poc/values.yaml` | Config PoC: imagen, modelo, fallback, ingress |
| `deploy/apps/argocd-config/` | ArgoCD ConfigMaps: OAuth (Dex), RBAC policy |
| `deploy/apps/prometheus/values.yaml` | Prometheus + Grafana: OAuth, root_url |
| `leloir-core/` | Control plane Go (3 binarios) — **M1 base implementada (handlers+orchestrator+wiring)** |
| `leloir-sdk/` | AgentAdapter SDK Go module — **M1 base activa (Holmes adapter + conformance)** |
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
