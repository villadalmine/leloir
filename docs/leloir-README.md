# Leloir

**The open-source agentic incident analysis platform.**

> Named after [Luis Federico Leloir](https://www.nobelprize.org/prizes/chemistry/1970/leloir/biographical/) — Argentine Nobel Laureate in Chemistry (1970), who unraveled the metabolism of sugars. We unravel the metabolism of incidents.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://go.dev)
[![Kubernetes 1.28+](https://img.shields.io/badge/Kubernetes-1.28+-326CE5.svg)](https://kubernetes.io)
[![CNCF Ecosystem](https://img.shields.io/badge/CNCF-ecosystem-blue)](https://cncf.io)

---

When a Prometheus alert fires, someone has to figure out what's wrong. Today that means an engineer waking up, logging into multiple systems, manually correlating metrics, logs and Kubernetes state — and often spending 20 minutes just to understand the problem.

Leloir automates that investigation. A configured AI agent examines your infrastructure, calls your observability tools, and returns a structured root cause analysis — before you've finished your first coffee.

```
Alert fires at 3:47 AM
        │
        ▼
 Alertmanager webhook
        │
        ▼
   Leloir routes                     "postgres-down → Holmes agent"
        │
        ▼
 Holmes investigates                 calls kubernetes.get_pod
                                     calls prometheus.query
                                     calls postgres.pg_stat_activity
        │
        ▼
 Root cause identified               "Connection pool exhausted.
                                      payment-service v2.3 has a
                                      connection leak."
        │
        ▼
 Teams / Slack notified              Full analysis + recommendation
```

---

## What makes Leloir different

**Agent-agnostic.** HolmesGPT, OpenCode, Hermes, Claude Code, or your own custom agent. The `AgentAdapter` interface (5 methods) makes any agent a first-class citizen. [Write your own adapter in a day.](docs/howto-write-adapter.md)

**Agent-to-agent orchestration (A2A).** A generalist agent (Holmes) can invoke a specialist agent (your DBA agent, your security agent) mid-investigation and integrate the results. Budget, audit, and approvals flow through transparently.

**Multi-tenant from day one.** Each team has its own namespace, OIDC identity, budget caps, and tool access. Kubernetes-native, no shared state between tenants.

**Pluggable everything.** LLM providers (Azure OpenAI, Anthropic, GitHub Copilot, OpenRouter), MCP sources (Prometheus, Kubernetes, Postgres, your own), notification channels (Teams, Slack, Telegram, Webhook), memory systems (via MCP integration). One Helm chart, configured via values.

**Works at home, works in enterprise.** Flip `profile: local` for a laptop demo (Anthropic + Dex). Flip `profile: corporate` for Azure AD + Vault + WORM audit + mTLS. Same binary.

**No commercial model.** Apache 2.0. No paid tier. No "open core" trick. Fork it, modify it, run it, contribute back if you want.

---

## Quickstart (home cluster, ~10 minutes)

**Prerequisites:** [kind](https://kind.sigs.k8s.io/), [kubectl](https://kubernetes.io/docs/tasks/tools/), [Helm 3](https://helm.sh/docs/intro/install/)

```bash
# 1. Create a local cluster
kind create cluster --name leloir-dev

# 2. Add the Leloir Helm repo
helm repo add leloir https://leloir.github.io/leloir
helm repo update

# 3. Install with local profile
helm install leloir leloir/leloir \
  --namespace leloir --create-namespace \
  --values https://leloir.dev/quickstart-local-values.yaml \
  --set llm.anthropic.apiKey=$ANTHROPIC_API_KEY

# 4. Port-forward the UI
kubectl port-forward -n leloir svc/leloir-ui 3000:80

# 5. Open http://localhost:3000
```

You'll see a live dashboard. To trigger a demo investigation:

```bash
# Fire a synthetic alert
kubectl apply -f https://leloir.dev/examples/demo-alert.yaml
```

Watch the agent investigate it in real time.

---

## Production install (Helm)

```bash
helm install leloir leloir/leloir \
  --namespace leloir --create-namespace \
  --values my-corporate-values.yaml
```

Minimum `my-corporate-values.yaml` for corporate deployment:

```yaml
profile: corporate

oidc:
  issuer: https://login.microsoftonline.com/<tenant-id>/v2.0
  clientId: <azure-app-client-id>
  clientSecretRef: leloir-oidc-secret

llm:
  defaultProvider: azure-openai-corp
  providers:
    - name: azure-openai-corp
      type: azure
      apiBase: https://your-company.openai.azure.com
      deployment: gpt-4o
      apiKeySecretRef: azure-openai-secret
    - name: github-copilot
      type: github_copilot
      tokenSecretRef: github-copilot-token

postgres:
  host: your-postgres-host
  database: leloir
  credentialsSecretRef: leloir-postgres-secret
```

See the [full configuration reference](docs/configuration.md).

---

## Core concepts

### Agents

An **agent** is an AI system that investigates incidents. Leloir ships adapters for:

| Agent | What it does | Adapter |
|---|---|---|
| [HolmesGPT](https://github.com/HolmesGPT/holmesgpt) | Cloud-native incident analysis | `adapter-holmesgpt` (M1) |
| [Hermes Agent](https://github.com/nous-research/hermes) | Open-weights, local-capable | `adapter-hermes` (M3) |
| [OpenCode](https://github.com/opencode-ai/opencode) | Code + infra agentic tool | `adapter-opencode` (M3) |
| [Claude Code](https://github.com/anthropics/claude-code) | Agentic coding + analysis | `adapter-claude-code` (M5) |

**Write your own** using the [AgentAdapter SDK](https://github.com/leloir/sdk):

```go
import "github.com/leloir/sdk/adapter"

type MyDBAgent struct{}
var _ adapter.AgentAdapter = (*MyDBAgent)(nil)

func (a *MyDBAgent) Identity() adapter.AgentIdentity { ... }
func (a *MyDBAgent) Configure(ctx context.Context, c adapter.Config) error { ... }
func (a *MyDBAgent) HealthCheck(ctx context.Context) error { ... }
func (a *MyDBAgent) Investigate(ctx context.Context, r adapter.InvestigateRequest) (<-chan adapter.Event, error) { ... }
func (a *MyDBAgent) Shutdown(ctx context.Context) error { ... }
```

### Alert Routes

An `AlertRoute` tells Leloir: "when this kind of alert fires, use this agent, these tools, and notify these channels":

```yaml
apiVersion: leloir.dev/v1alpha1
kind: AlertRoute
metadata:
  name: critical-db-incidents
  namespace: my-team
spec:
  match:
    labels: { severity: critical, type: database }
  agent: holmes-prod
  allowedSources: [prometheus-mcp, kubernetes-mcp, postgres-mcp]
  notify: [teams-sre]
  budget:
    maxUSD: 3.0
```

### MCP Sources

Agents call tools through the [MCP Gateway](docs/mcp-gateway.md). Register any MCP-compatible server:

```yaml
apiVersion: leloir.dev/v1alpha1
kind: MCPServer
metadata:
  name: prometheus-mcp
spec:
  trustTier: internal-hosted
  transport:
    type: streamable-http
    endpoint: http://prometheus-mcp.svc:8080
```

Supported out of the box: Prometheus, Kubernetes, Loki, PostgreSQL, MySQL, AWS CloudWatch, Azure Monitor. [Add your own.](docs/howto-write-mcp-server.md)

### A2A — Agent Orchestration

Agents can invoke other agents mid-investigation. The platform handles budget propagation, audit, and approvals transparently:

```
Holmes investigating DB incident
  │
  └─ Calls: security-specialist
      "Is this connection pattern a security event?"
      → "No. Connection leak in v2.3, not an attack."
  │
  └─ Integrates result, continues investigation
```

Budget propagates: tenant → route → parent investigation → caller hint. Loops and fan-out are automatically detected and blocked. [Full A2A documentation.](docs/a2a.md)

### HITL — Human in the Loop

Before destructive actions, Leloir pauses for human approval:

```yaml
apiVersion: leloir.dev/v1alpha1
kind: ApprovalPolicy
metadata:
  name: write-protection
spec:
  tools:
    requireApproval:
      - tools: ["kubernetes.exec", "postgres.execute"]
        channel: teams-sre
        timeoutMinutes: 30
```

---

## Architecture

```
                  Alertmanager / Prometheus
                          │
                          │ webhook
                          ▼
          ┌───────────────────────────────┐
          │       Leloir Control Plane     │
          │                               │
          │  ┌─────────┐  ┌───────────┐  │
          │  │ Routing │  │  Audit    │  │
          │  │ Engine  │  │  Log      │  │
          │  └────┬────┘  └───────────┘  │
          │       │                       │
          │  ┌────▼────────────────────┐  │
          │  │    AgentAdapter (SDK)   │  │
          │  │  Holmes / Hermes / ...  │  │
          │  └────┬────────────────────┘  │
          │       │  tool calls           │
          │  ┌────▼────┐  ┌───────────┐  │
          │  │   MCP   │  │    LLM    │  │
          │  │ Gateway │  │  Gateway  │  │
          │  └────┬────┘  └─────┬─────┘  │
          └───────┼─────────────┼─────────┘
                  │             │
        Prometheus,K8s,    Azure OpenAI,
        Postgres, Loki     Anthropic, ...
```

[Full architecture documentation →](docs/architecture.md)

---

## Configuration profiles

| | `profile: local` | `profile: corporate` |
|---|---|---|
| LLM providers | Anthropic, OpenRouter, any | Azure OpenAI + GitHub Copilot |
| OIDC | Dex (static users / GitHub OAuth) | Azure AD / Okta / any OIDC |
| Secrets | Kubernetes Secrets | Vault + External Secrets Operator |
| Audit retention | 7d hot | 90d hot / 1y warm / WORM optional |
| mTLS | Optional | Required cluster-wide |
| Budgets | Soft warnings | Hard enforcement |
| HITL | Optional | Required for write actions |

Helm value presets ship in `crds/helm-profiles/`. Pass one as `-f values.profiles/corporate.yaml` and override specifics in your own `my-values.yaml`.

---

## Kubernetes CRDs

Leloir is configured entirely via Kubernetes custom resources — no config files, no databases to pre-seed.

```bash
# Install all CRDs
kubectl apply -k https://github.com/leloir/leloir/crds

# The 8 CRDs
kubectl get crds | grep leloir.dev
# agentregistrations.leloir.dev    ← register an AI agent
# alertroutes.leloir.dev           ← alert → agent + sources + notifications
# mcpservers.leloir.dev            ← tool sources (Prometheus, k8s, Postgres, memory…)
# notificationchannels.leloir.dev  ← Teams / Slack / Telegram
# approvalpolicies.leloir.dev      ← HITL rules for tools + A2A invocations
# tenantbudgets.leloir.dev         ← LLM cost caps per tenant
# skillsources.leloir.dev          ← runbook / skill repos
# tenants.leloir.dev               ← tenant + namespace mapping
```

See [`crds/sample-db-incident-response.yaml`](crds/sample-db-incident-response.yaml) for a full working example: Tenant + HolmesGPT + Prometheus MCP + Teams + Skills + AlertRoute — ready to `kubectl apply`.

---

## Project status

| Milestone | Status | Delivers |
|---|---|---|
| M0 — PoC | 🟡 In progress | End-to-end flow: alert → Holmes → Teams |
| M1 — MVP | ⬜ Planned | Production-ready single-tenant, OSS repo |
| M2 — Multi-tenant | ⬜ Planned | OIDC, RBAC, MCP Gateway scoping |
| M3 — Cost + SDK | ⬜ Planned | LLM Gateway, budget, public SDK, 3 adapters |
| M4 — Safety | ⬜ Planned | HITL, Vault, audit WORM |
| M5 — Ecosystem | ⬜ Planned | A2A, Claude Code, cloud sources, UI tree |
| M6 — v1.0 | ⬜ Planned | Hardening, docs, public release |

Target: **~25-30 weeks to v1.0** from start of M1.

---

## Contributing

We welcome contributions. Some good first areas:

- **New agent adapters** — see [`sdk/examples/`](https://github.com/leloir/sdk/examples) for the template
- **New MCP server integrations** — follow the [MCP source guide](docs/howto-write-mcp-server.md)
- **Documentation improvements** — always needed
- **Bug reports** — open an issue

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

Community: [Slack workspace](https://leloir.slack.com) · [Issues](https://github.com/leloir/leloir/issues) · [Discussions](https://github.com/leloir/leloir/discussions)

---

## License

[Apache 2.0](LICENSE) — use it, modify it, distribute it. No paid tier, no commercial restrictions.

---

## About the name

[Luis Federico Leloir](https://www.nobelprize.org/prizes/chemistry/1970/leloir/biographical/) was an Argentine biochemist who won the Nobel Prize in Chemistry in 1970 for discovering how organisms metabolize sugars — figuring out the root cause of complex biochemical processes at the molecular level.

He was a disciple of Bernardo Houssay (Nobel 1947) and later mentored César Milstein (Nobel 1984) — a lineage of scientific discovery that mirrors the agent-orchestrates-agent (A2A) model at the heart of this platform.

He built his institute with private funds when the government withdrew support, never stopped working, and returned his Nobel prize money to his research team. He'd have liked open source.
