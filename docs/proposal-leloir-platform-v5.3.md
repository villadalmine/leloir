# Leloir — Agentic Incident Analysis Platform — Complete Consolidated Design
## Proposal v5.3 — open source, agnostic, A2A-first, multi-transport, memory-as-integration

**Project name:** Leloir (after Luis Federico Leloir, Argentine Nobel Laureate in Chemistry 1970)
**Document status:** Draft for review · supersedes v5.2
**Version:** 5.3
**Date:** April 21, 2026
**License intent:** Apache 2.0 (open source, community-friendly)

---

## Table of Contents

0. What changed from prior versions
1. Executive Summary
2. Reference Architecture
3. Deployment Topologies & Bootstrap
4. Agent Integration (AgentAdapter contract)
5. Source Integration (MCP + Trust Tiers + Pluggable Transports)
6. Skills System (visibility, versioning, signing)
7. LLM Gateway & Cost Model
8. HITL — Human in the Loop Workflows
9. Secrets Management
10. Encryption End-to-End (with gRPC native mTLS notes)
11. Audit Log (DORA / Compliance grade)
12. Prompt Injection Defenses
13. Observability End-to-End (OpenTelemetry)
14. Disaster Recovery & Self-Monitoring
15. AgentAdapter Versioning Policy
16. Agent Orchestration & A2A (first-class feature)
17. Open Source Strategy
18. Dual-Environment Profiles
19. **Memory & Knowledge Persistence Strategy** (NEW in v5.3)
20. Tech Stack (consolidated)
21. Modular Delivery Plan (build small, grow incrementally)
22. Risks & Open Questions (12 questions all resolved)
23. Comparison v2/v3/v4/v5/v5.1/v5.2/v5.3
24. Next Steps

---

## 0. What changed from prior versions

### From v4 to v5.0
v4 established the core architectural thesis: **direct adapters to each agent's native interface, no framework dependency**. v5.0 added the operational, security, observability, and compliance layers that turn it into a production-ready design:

- **LLM Gateway** for cost attribution, budgeting, fallback, caching
- **HITL workflows** with `ApprovalPolicy` CRD
- **Secrets management** layer (Vault + External Secrets Operator) — agents never see credentials
- **Encryption end-to-end** spec (5 layers, mTLS, OAuth2, at-rest, egress control)
- **DORA-grade audit log** (hash chain, WORM tier, segregation of duties, SIEM export)
- **Prompt injection defenses** (7 layers)
- **End-to-end observability** with OpenTelemetry
- **Disaster recovery** (lightweight — service is on-demand, not SLO-critical)
- **AgentAdapter versioning policy** (semver, contract tests, deprecation rules)
- **Modular delivery plan** designed to validate each layer incrementally
- Bootstrap explicitly clarified for both deployment modes

### From v5.0 to v5.1 — Multi-transport MCP

In January-February 2026, Google Cloud announced a major contribution to MCP: **gRPC as a pluggable transport** alongside the existing JSON-RPC over HTTP/SSE. The MCP maintainers accepted the direction toward pluggable transports. This means three transports will coexist for the foreseeable future:

- **stdio** — local processes (Claude Code style sandboxed agents)
- **Streamable HTTP** (current default, replaces older SSE) — community standard, broad compatibility
- **gRPC over HTTP/2** — enterprise-grade: native mTLS, true bidirectional streaming, Protobuf type safety, ~3-4x performance vs JSON-RPC at scale

v5.1 changed the design to be **transport-agnostic from day one**: the MCP Gateway becomes a multi-transport translator. Agents speak HTTP inbound; Gateway translates to gRPC, Streamable HTTP, SSE, or stdio outbound per MCP server.

### From v5.1 to v5.2 — Open source, agent orchestration, dual-environment

After answering all 12 strategic open questions with the customer, three major design directions emerged that v5.2 incorporates:

**1. Open source by design (Q8)**
- Apache 2.0 license, agnostic, extensible — not a commercial product
- Repo structure, naming, contribution model designed for public collaboration from day 1
- New Section 17: Open Source Strategy
- Adds ~2 weeks across the timeline (CI public setup, docs site, contribution guides, quickstart)

**2. Agent-to-Agent orchestration as first-class feature (Q11)**
- Customer wants three patterns supported: custom adapters, agents invoking other agents, and OpenCode-style nested subagents
- This is the **strategic differentiator** of the platform — no other open-source project covers all three
- New Section 16: Agent Orchestration & A2A
- Defense in depth for A2A authorization: agent policy (Q14b) AND route team (Q14c) combined
- Budget propagation as 4-layer min: tenant > route > parent > caller hint (Q15)
- Loop prevention: depth caps + fan-out caps + cycle detection (Q16)
- Adds ~2-3 weeks distributed in M3-M5

**3. Dual-environment by design (derived from Q4 + Q6)**
- Same codebase runs in two profiles: **corporate** (Azure OpenAI + GitHub Copilot + Azure AD) and **local** (Anthropic + OpenRouter + Dex). Switch via Helm values
- New Section 18: Dual-Environment Profiles
- Profile presets in Helm chart from M1 (no extra time)

**Sustainability / scaling decisions:**
- Q9: no fixed tenant count — design for arbitrary scale, document capacity tiers
- Q10 confirmed: analysis-only in v1.0, integrate with external on-call
- Q12 confirmed: gRPC for internal MCPs in M3-M4

**Updated total timeline: ~25-30 weeks to v1.0** (vs 22-26 in v5.1). The increase comes from:
- ~2 weeks for proper open source quality bar
- ~2-3 weeks for A2A as first-class feature

Modular delivery preserved: every milestone still ships testable value every 4-6 weeks.

### From v5.2 to v5.3 — Memory & Knowledge Persistence Strategy

After evaluating MemPalace (highest-scoring AI memory system per LongMemEval, MIT-licensed, MCP-compatible) for potential integration into Leloir, the design crystallized around a clear principle: **memory is an integration concern, not a core feature.**

This is added as new Section 19. The key decisions:

**Decision 1: Leloir does NOT own a memory subsystem in v1.0**
- Each agent (HolmesGPT, Hermes, OpenCode, Claude Code, custom) manages its own conversation state — they know their context window best
- The control plane handles audit (compliance-grade, DORA WORM), which is different from "memory"
- Skills handle prescriptive knowledge ("how to investigate X")
- These three together cover ~95% of what "memory" means in operational AI

**Decision 2: External memory systems integrate via the existing MCPServer CRD**
- Any memory system that exposes an MCP server (MemPalace, Zep/Graphiti, Mem0, custom) is a first-class extension point with zero core changes
- Operator opts in per-tenant by registering an MCPServer
- Agents discover memory tools through normal MCP tool listing
- Clean separation: Leloir provides the wiring, memory vendors provide the substance

**Decision 3: Cross-investigation memory (per-tenant) is a candidate native feature for v1.1**
- Specifically: "we've seen this incident pattern before; here's what root cause it had last time"
- Implementable simply with Postgres (no external dependency)
- Single internal MCP server: `leloir.similar_past_incidents`
- Built only if M0-M5 user feedback demands it

**Why not adopt MemPalace tightly into core:**
- Too young (v3.1.0 released April 9, 2026 — under 2 weeks at evaluation time)
- Designed for single-user scenarios, not multi-tenant SaaS-style deployments
- Python stack adds operational complexity (Leloir core is Go)
- Locks Leloir into an opinionated taxonomy (wings/rooms/closets) that may not fit incident response
- Adopting any memory framework tight = lock-in risk that contradicts the agnostic ethos

**What this gives users:**
- Out of the box: working incident analysis with no memory layer (cleaner default)
- One YAML to add MemPalace if desired (home/dev users likely want this)
- One YAML to add Zep if enterprise (managed, supported, SOC2)
- Optional native cross-investigation memory in v1.1 if users ask for it

This is documented as Section 19 with full integration examples.

**No timeline impact** — this is documentation + leveraging existing MCPServer extensibility. Section 19 explains the integration pattern without requiring new code.

---

## 1. Executive Summary

We build an **open-source, agnostic, incident-driven control plane and UI** for AI-assisted ops on Kubernetes. Apache 2.0 licensed, designed for both home/personal experimentation and enterprise deployment.

**Three core principles:**

1. **No framework lock-in.** Direct integration to each agent's native protocol via thin adapters (~150-300 lines each). Anyone can write a new adapter with our SDK.
2. **Each agent stays self-contained.** We invoke them — we don't host them. They keep their own runtime, multi-LLM, MCP client, sessions.
3. **Agent orchestration is first-class.** Any agent can invoke any other agent, with budget propagation, audit, HITL, and loop prevention. This is the platform's strategic differentiator.

**Strategic positioning (the README pitch):**

> **The only open-source platform where any AI agent can orchestrate any other AI agent to resolve operational incidents — with unified cost tracking, audit, approvals, and security across all of them.**

**The first deliverable** wires HolmesGPT + Prometheus + Teams/Telegram in a minimal end-to-end flow. Each subsequent milestone adds one well-defined capability and is independently shippable. The platform supports two deployment profiles from day 1: corporate (Azure OpenAI, Azure AD, strict audit) and local (Anthropic, Dex, experimentation-friendly).

**Cost to build:** ~25-30 weeks to v1.0 with a small team (3-4 people part-time). Modular delivery means usable value lands every 4-6 weeks.

---

## 2. Reference Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  UI — React + Vite (multi-tenant, incident-focused)                  │
│  Alert Inbox │ Investigations │ Routes │ Channels │ Audit │ Costs    │
└─────────────────────────────┬────────────────────────────────────────┘
                              │  REST + WebSocket/SSE  (TLS 1.3)
┌─────────────────────────────▼────────────────────────────────────────┐
│  Control Plane (Go) — entirely ours, deliberately minimal            │
│                                                                      │
│  ┌────────────┐  ┌────────────┐  ┌──────────────────────────────┐    │
│  │ Alert      │  │ Routing    │  │ Notification fan-out         │    │
│  │ ingestion  │→ │ engine     │→ │ Teams│Telegram│Slack│Webhook │    │
│  └────────────┘  └─────┬──────┘  └──────────────────────────────┘    │
│                        │                                             │
│  ┌─────────────────────▼───────────────────────────────────────┐     │
│  │ Agent Adapter Registry (5-method interface, semver-managed) │     │
│  │   Holmes │ Hermes │ OpenCode │ Claude Code │ Custom SDK     │     │
│  └─────────────────────┬───────────────────────────────────────┘     │
│                        │                                             │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ HITL Engine    │ Audit Logger    │ Cost Attribution         │     │
│  │ (ApprovalPol)  │ (hash-chained,  │ (per tenant/agent/inv)   │     │
│  │                │  WORM, SIEM)    │                          │     │
│  └─────────────────────────────────────────────────────────────┘     │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐     │
│  │ Auth: OIDC + per-NS RBAC + Service Account TokenReview      │     │
│  └─────────────────────────────────────────────────────────────┘     │
└────┬───────────────┬──────────────────┬──────────────────────────────┘
     │               │                  │
┌────▼───────────┐ ┌─▼─────────────┐ ┌──▼──────────────┐
│ MCP Gateway    │ │ LLM Gateway   │ │ Secrets Layer   │
│ (multi-        │ │ (LiteLLM)     │ │ (Vault + ESO)   │
│  transport     │ │  cost meter,  │ │  per-tenant     │
│  translator)   │ │  fallback,    │ │  paths, no      │
│                │ │  cache,       │ │  credential     │
│ Inbound:       │ │  budget cap)  │ │  ever in agent  │
│  HTTP/JSON-RPC │ └───┬───────┬───┘ └─────────────────┘
│  from agents   │     │       │
│                │     │       └─► OpenAI │ Anthropic │ Bedrock │ Azure
│ Outbound (per  │     └────────► Nous Portal │ OpenRouter │ Ollama
│  MCP server):  │                  (provider real, audited per call)
│  • gRPC        │
│  • Streamable  │
│    HTTP        │
│  • SSE legacy  │
│  • stdio       │
│                │
│ Per call:      │
│  auth, scope,  │
│  cred inject,  │
│  audit, rate   │
│  limit, HITL   │
└──┬────┬──┬─────┘
   │    │  │
   │    │  └──► External MCPs (GitHub, Notion, etc.) — usually HTTP
   │    │       egress controlled, OAuth2 tokens injected per-tenant
   │    │
   │    └─────► Vendor-vetted MCPs (k8s, Prometheus official) — HTTP today,
   │             may migrate to gRPC as upstream supports it
   │
   └──────────► Internal MCPs (custom Postgres, custom VMware, etc.)
                — gRPC native (M3+) for performance + native mTLS
                       │
       ┌───────────────┼───────────────┬─────────────┐
       ▼               ▼               ▼             ▼
   ┌─────────┐   ┌─────────┐   ┌──────────────┐ ┌────────┐
   │ Holmes  │   │ Hermes  │   │ OpenCode/CC  │ │ Custom │
   │ (own    │   │ (own    │   │ (own runtime,│ │ agent  │
   │ runtime,│   │ runtime,│   │ skills, MCP) │ │ (your  │
   │ skills) │   │ skills) │   │              │ │  way)  │
   └─────────┘   └─────────┘   └──────────────┘ └────────┘

  Each agent: own runtime, own skills, own MCP client (HTTP-based today),
  own session state. We orchestrate them — we don't host them.

  Agents always speak HTTP/JSON-RPC to our Gateway.
  The Gateway translates to whatever transport each MCP server uses.
  This decouples agent ecosystem maturity from MCP server ecosystem maturity.
```

---

## 3. Deployment Topologies & Bootstrap

### 3.1 Mode A — Centralized Multi-Tenant

**One control plane + UI** for the whole cluster. **One URL** for all tenants. OIDC authenticates users, RBAC scopes them to namespaces, all queries filtered by tenant.

```
Cluster
├── ns: agentic-ops             ← single shared install
│   ├── control-plane (deployment)
│   ├── ui (deployment)
│   ├── llm-gateway (deployment)
│   ├── mcp-gateway (deployment)
│   ├── postgres (statefulset, ours)
│   └── ingress → https://agentic-ops.company.com  ← ONE URL
│
├── ns: tenant-a                ← tenant's own namespace
│   ├── hermes-sre (deployment, agent)
│   ├── holmes-prod (deployment, agent)
│   └── postgres-mcp (deployment, MCP server with tenant-A creds)
│
├── ns: tenant-b
│   ├── holmes-prod (deployment, agent)
│   └── postgres-mcp (deployment, MCP server with tenant-B creds)
│
└── ns: shared-mcp              ← global MCPs (k8s, Prometheus)
    ├── kubernetes-mcp
    └── prometheus-mcp
```

**User experience:** alice@tenant-a goes to `agentic-ops.company.com`, logs in via OIDC, sees only tenant-a's alerts/sessions/runbooks/costs. bob@tenant-b sees only tenant-b's. All cross-tenant data flow is gated by the control plane's RBAC layer.

**Bootstrap order:**

1. Install cert-manager (TLS infra)
2. Install External Secrets Operator + connect to Vault
3. Install our Helm chart in `agentic-ops` namespace (control plane, UI, gateways, Postgres)
4. Apply cluster-wide CRDs (Tenant, AlertRoute, ApprovalPolicy, MCPServer, SkillSource, etc.)
5. Per tenant: create namespace, RoleBindings, deploy agents + MCPs via tenant's own Helm release
6. Configure Alertmanager to webhook our control plane
7. First test alert end-to-end

### 3.2 Mode B — Per-Namespace Isolated

**Stack replicated inside each tenant namespace.** One URL per tenant. Total isolation by Kubernetes primitives.

```
Cluster
├── ns: tenant-a                ← full stack, isolated
│   ├── control-plane (theirs)
│   ├── ui (theirs)
│   ├── llm-gateway (theirs, with their LLM keys from their Vault path)
│   ├── mcp-gateway (theirs)
│   ├── postgres (theirs)
│   ├── hermes-sre, holmes (their agents)
│   ├── postgres-mcp, github-mcp (their MCPs only)
│   └── ingress → https://agentic-ops-a.company.com  ← URL of tenant-a only
│
├── ns: tenant-b                ← independent stack
│   └── ...
│       └── ingress → https://agentic-ops-b.company.com
```

**User experience:** tenant-a's users go to `agentic-ops-a.company.com`. They cannot see, query, or even discover tenant-b. NetworkPolicy denies cross-namespace traffic.

**Bootstrap order:**

1. Cluster-wide: cert-manager, External Secrets Operator, CRDs
2. Per tenant: one-shot installer creates namespace + everything inside it from a single Helm command
3. Tenant admin self-provisions their `MCPServer`, `SkillSource`, `AlertRoute` resources
4. Each tenant's Alertmanager points to their own ingress

### 3.3 Choosing between the two

| Factor | Mode A (Centralized) | Mode B (Per-NS) |
|---|---|---|
| Resource cost | Low (shared) | High (replicated) |
| Setup complexity | One-time + per-tenant onboarding | One-shot per tenant |
| Isolation | RBAC + control plane logic | Hard (K8s primitives) |
| Ideal for | NSaaS, shared platform team | Sensitive workloads, regulated tenants, large tenants |
| Failure blast radius | Whole cluster impacted | One tenant only |

Both ship in the same Helm chart with `mode: centralized | per-namespace`.

---

## 4. Agent Integration — AgentAdapter Contract

### 4.1 The interface (versioned, semver-managed)

```go
// AgentAdapter v1 - stable contract, additive changes only
type AgentAdapter interface {
    // Identity
    Name() string
    Version() string
    Capabilities() []Capability

    // Lifecycle
    HealthCheck(ctx context.Context) error
    Configure(ctx context.Context, config map[string]any) error

    // Core: invoke an investigation, stream events back
    Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error)
}

type InvestigateRequest struct {
    InvestigationID string
    AlertContext    map[string]any  // sanitized alert payload
    AvailableTools  []string        // which MCP tools the route allows
    Tenant          string
    SessionID       string          // for follow-up chat
    Runbook         string          // optional, pre-loaded skill content
    BudgetLimit     CostBudget      // hard cap for this investigation
}

type Event struct {
    Type      EventType  // thought | tool_call | tool_result | answer | error | hitl_request
    Timestamp time.Time
    Payload   any
}
```

### 4.2 Per-agent adapters (sketches)

| Agent | Native interface | Adapter size | Phase |
|---|---|---|---|
| HolmesGPT | HTTP API (`POST /api/chat` + SSE) | ~150 lines | M1 |
| Hermes Agent | HTTP gateway | ~200 lines | M3 |
| OpenCode | Native client/server protocol | ~200 lines | M3 |
| Claude Code | Spawned process (stdio + JSON output) | ~250 lines | M4 |
| Custom (your team) | Implement the interface | ~150 lines + tests | M3 (SDK ships) |

### 4.3 Adapter contract tests

Each adapter ships with a test suite that verifies:
- Investigation completes within budget
- Streaming events arrive in expected order
- Tool calls respect the allowedTools list
- Errors are mapped to canonical Event types
- Health check correctly detects unhealthy agent
- Configure() is idempotent

These tests are part of CI; an adapter cannot merge without them.

---

## 5. Source Integration — MCP + Trust Tiers

### 5.1 Trust tiers (per source)

| Tier | What | Examples | Policy |
|---|---|---|---|
| **internal-hosted** | Built and run by us, in our cluster | Custom Postgres MCP with tenant scoping, custom VMware MCP | mTLS, standard audit, generous rate limit |
| **vendor-vetted** | Third-party, version-pinned, in our cluster | Official k8s MCP, official Prometheus MCP | Pinned to digest, periodic security review, mTLS |
| **external** | SaaS-hosted, accessed over public internet | GitHub MCP, Notion MCP | OAuth2, full audit (args + responses), strict rate limit, egress allowlist, optional HITL |

### 5.2 MCPServer CRD (with pluggable transport)

```yaml
apiVersion: agenticops.io/v1
kind: MCPServer
metadata:
  name: postgres-tenant-a
  namespace: tenant-a
spec:
  trustTier: internal-hosted
  transport:
    type: grpc                              # grpc | streamable-http | sse | stdio
    endpoint: postgres-mcp.tenant-a.svc.cluster.local:9090
    protoDescriptor: postgres-mcp.binpb     # optional, for gRPC reflection
    tls:
      mode: mTLS                            # required for grpc/streamable-http
      caConfigMap: internal-ca
      clientCertSecret: gateway-client-cert
  visibility: namespace                     # or 'global'
  credentialRef:                            # gateway looks up credentials per call
    vaultPath: secret/tenants/a/postgres
    rotation: 90d
  policies:
    rateLimit: 60/min
    auditLevel: standard                    # 'full' for externals
    allowedTools: ["query", "explain", "stat"]
    forbiddenTools: ["execute_ddl"]
```

**Examples by transport type:**

```yaml
# Internal MCP, gRPC native (M3+) — best for performance + native mTLS
transport:
  type: grpc
  endpoint: postgres-mcp.tenant-a.svc.cluster.local:9090
  tls: { mode: mTLS, caConfigMap: internal-ca, clientCertSecret: gw-cert }

# Vendor-vetted MCP, current HTTP standard
transport:
  type: streamable-http
  endpoint: https://kubernetes-mcp.shared-mcp.svc.cluster.local:8443/mcp
  tls: { mode: mTLS, caConfigMap: internal-ca, clientCertSecret: gw-cert }

# Legacy MCP server still on SSE (backward compatibility)
transport:
  type: sse
  endpoint: https://legacy-mcp.example.com:8443/mcp/sse
  tls: { mode: tls }                        # standard TLS, no mTLS

# External MCP (GitHub, Notion, etc.) — HTTP, OAuth2 from Vault
transport:
  type: streamable-http
  endpoint: https://api.githubcopilot.com/mcp/
  tls: { mode: tls }
auth:
  type: oauth2
  secretRef: github-oauth-tenant-a
policies:
  auditLevel: full                          # log args + responses
  egressAllowlist: [api.githubcopilot.com]

# Local sandboxed MCP for Claude Code style usage (M5+)
transport:
  type: stdio
  command: ["mcp-server-fs"]
  args: ["--root", "/sandbox/skills"]
```

### 5.3 Why multi-transport matters

| Scenario | Best transport | Why |
|---|---|---|
| Internal high-throughput (DB queries, k8s API, log streaming) | gRPC | 3-4x perf, native mTLS, type safety, backpressure |
| Public/external SaaS APIs | Streamable HTTP | Public infra is HTTP, any CDN/proxy works |
| Legacy MCP servers we don't control | SSE | Backward compatibility |
| Sandboxed local tools | stdio | No network surface, perfect isolation |

The MCP Gateway picks the right transport per call without any change in the agent — agents always speak HTTP/JSON-RPC inbound to the Gateway.

### 5.4 Phased source integration

| Source | MCP server | Phase |
|---|---|---|
| Kubernetes | community / built-in | M1 |
| Prometheus + Alertmanager | community | M1 |
| Loki | community | M3 |
| PostgreSQL / MySQL | community | M3 |
| VMware vSphere | **we build a thin wrapper** | Backlog (post v1.0) |
| AWS CloudWatch | community | M4 |
| Azure Monitor | community | M4 |
| Datadog / Grafana | community | M4 |
| OpsGenie / PagerDuty | community | M4 |
| Custom internal API | **MCP SDK template we ship** | M4+ |

---

## 6. Skills System

Skills are reusable instruction packages that live inside each agent's filesystem. Format compatible with `agentskills.io` (Claude / OpenCode / Hermes share this).

### 6.1 SkillSource CRD (mirror of MCPServer for symmetry)

```yaml
apiVersion: agenticops.io/v1
kind: SkillSource
metadata:
  name: company-sre-skills
spec:
  source:
    type: oci                          # oci | git | http | filesystem
    url: oci://harbor.company.local/skills/sre
    digest: sha256:abc123...           # PINNED, immutable
    pullSecret: harbor-creds
    verifySignature: true              # Cosign mandatory
  trustTier: internal-hosted
  visibility: global                   # or 'namespace'
  refreshInterval: never               # immutable per pod lifetime
```

### 6.2 Visibility & isolation matrix (same as MCPs — symmetric model)

| | Mode A (Centralized) | Mode B (Per-NS, isolated) |
|---|---|---|
| `visibility: global` | All tenants see them | Hidden if `isolation.excludeGlobal: true` |
| `visibility: namespace` | Only that NS's agents see them | Only that NS's agents see them |

### 6.3 Versioning — immutable runtime, mutable storage

The same pattern Kubernetes uses for container images:

- An init container resolves all applicable `SkillSource`s, verifies signatures, mounts them as a read-only `emptyDir` into the agent pod.
- The agent reads from that filesystem for its entire pod lifetime.
- Changes to `SkillSource` in the cluster do **not** affect running pods.
- On pod restart (rollout, OOM, manual), init container re-resolves and may pick up new versions.

**Versioning schemes** (in order of safety):

```yaml
tag: latest                              # ❌ not for prod
tag: v1.2.3                              # ✓ prod baseline
digest: sha256:abc...                    # ✓✓ paranoid, audit-ideal
```

**Rollout patterns:**
- **Manual:** admin bumps `SkillSource` → deliberate agent rollout
- **Canary:** 10% of agent pods get v1.3.0 → 24h eval metrics → 100%
- **Per-tenant pinning:** tenant A on v1.2.3, tenant B on v1.3.0 (each owns its upgrade)
- **Rollback:** revert SkillSource → restart agents → done in minutes

**Audit trace at agent startup:**

```json
{
  "event_type": "agent.startup",
  "agent": "hermes-sre",
  "pod": "hermes-sre-7d8f9-xqz2k",
  "skills_loaded": [
    { "source": "company-sre-skills", "digest": "sha256:abc...", "count": 47 },
    { "source": "tenant-a-private", "digest": "sha256:def...", "count": 12 }
  ],
  "mcp_servers_available": [...]
}
```

When auditors ask "which playbook ran during incident X 8 months ago?", the answer is bit-exact.

---

## 7. LLM Gateway & Cost Model

### 7.1 The pattern

Every agent's LLM client is configured to point at our **LLM Gateway** instead of the provider directly. The gateway is OpenAI/Anthropic/Bedrock-compatible, so agents don't change their code — only their endpoint config.

```
Agent                         LLM Gateway                    Provider
─────                         ───────────                    ────────
holmes-prod ───OpenAI API─►   LiteLLM proxy   ──audited──►   OpenAI
hermes-sre  ───OpenAI API─►   - count tokens                 Nous Portal
opencode    ───Anthropic──►   - attribute to tenant          Anthropic
                              - check budget
                              - cache lookup
                              - apply fallback
                              - emit cost event
```

### 7.2 Implementation

**LiteLLM Proxy** (Apache 2.0, mature, self-hosted). Out of the box:
- 100+ providers (OpenAI, Anthropic, Bedrock, Azure, Vertex, Ollama, Nous Portal, OpenRouter, etc.)
- Token counting + cost calculation per provider's pricing
- Per-key budget limits (we map keys to tenants)
- Semantic caching (Redis backend)
- Fallback chains (`gpt-4 → claude-3 → gpt-3.5` if upstream fails)
- OpenTelemetry traces and Prometheus metrics
- API key rotation

We deploy LiteLLM and write thin glue: a `TenantBudget` reconciler that syncs our CRD to LiteLLM's budget API.

### 7.3 TenantBudget CRD

```yaml
apiVersion: agenticops.io/v1
kind: TenantBudget
metadata:
  name: tenant-a-monthly
spec:
  tenant: tenant-a
  monthly:
    softLimit: 500          # USD - alert admin via channel
    hardLimit: 1000         # USD - LLM gateway returns 429
  perInvestigation:
    hardLimit: 5            # USD - cuts off runaway agent
  perAgent:
    hermes-sre: { softLimit: 200 }
    holmes-prod: { softLimit: 200 }
    claude-code-security: { softLimit: 100 }
  alertChannels:
    softBreach: [slack:#finops]
    hardBreach: [slack:#sre-oncall, teams:platform-team]
```

### 7.4 Cost dashboard

Standard view per tenant:
- Spend MTD vs budget (gauge)
- Top 5 agents by spend
- Top 10 investigations by cost
- Cache hit rate (lower spend if higher)
- Fallback events (cost spikes if primary provider is down)
- Token usage trend (7d / 30d / 90d)

Drill-down: click an investigation → see its full trace, every LLM call, every tool call, every token, cost attribution exact.

---

## 8. HITL — Human in the Loop Workflows

### 8.1 ApprovalPolicy CRD

```yaml
apiVersion: agenticops.io/v1
kind: ApprovalPolicy
metadata:
  name: production-writes
spec:
  scope:
    tenants: [tenant-a, tenant-b]
    environments: [production]

  tiers:
    autoApprove:
      tools:
        - "*.query"
        - "*.get"
        - "*.list"
        - "*.logs"
        - "prometheus.*"

    singleApproval:
      tools:
        - "kubernetes.exec"
        - "kubernetes.scale"
        - "kubernetes.restart"
      approvers:
        groups: [sre-oncall]
      channel:
        type: slack
        target: "#sre-approvals"
      timeout: 10m
      onTimeout: deny
      ui: true                   # also surfaces in our UI

    dualApproval:
      tools:
        - "*.delete"
        - "kubernetes.patch"
        - "iam.*"
      approvers:
        groups: [sre-oncall, sre-lead]
        requireDistinct: true    # 2 different people
      channel:
        type: slack
        target: "#sre-critical"
      timeout: 30m
      onTimeout: deny
      requireRationale: true     # approver must comment why

    blocked:
      tools: ["*.purge", "namespace.delete"]
      reason: "Disabled in production. Use kubectl manually."
```

### 8.2 The flow (mechanically)

1. Agent decides to call `kubernetes.exec` → request hits MCP Gateway
2. Gateway looks up matching `ApprovalPolicy` for tenant + tool
3. Gateway pauses the call, creates `ApprovalRequest`:
   ```json
   {
     "id": "appr-7f3a",
     "agent": "hermes-sre",
     "investigation": "inv-2026-04-19-postgres-down",
     "tool": "kubernetes.exec",
     "args": { "pod": "...", "command": ["kill", "-9", "1"] },
     "rationale": "<agent's natural-language explanation>",
     "blast_radius": { "namespace": "tenant-a", "scope": "single pod" },
     "timeout": "2026-04-19T14:42:01Z"
   }
   ```
4. Notification to Slack/Teams/UI with `[Approve]` / `[Deny]` buttons
5. Approver clicks → callback hits our control plane
6. Control plane signs the approval, gateway forwards original call
7. Audit log records: who approved, when, rationale, full request

### 8.3 Out-of-band channel design

Approvals **must** arrive via a different channel than the agent operates on. Defense against compromised agent attempting self-approval. Slack/Teams are perfect because the human is required, no automation can short-circuit them.

---

## 9. Secrets Management

**Core principle: agents never see credentials. Ever. Not for GitHub, not for Postgres, not for AWS, not for the LLM provider.**

### 9.1 Three storage layers

| Layer | What lives there | Who reads |
|---|---|---|
| **HashiCorp Vault** (or AWS SM / Azure KV / GCP SM) | Source of truth for all secrets, per-tenant paths (`secret/tenants/<tenant>/<service>`) | Only External Secrets Operator + LLM Gateway service account + MCP Gateway service account |
| **Kubernetes Secrets** | Synced subset that pods need | MCP servers + LLM Gateway pods (mounted as files or env) |
| **Agents** | NOTHING | — |

### 9.2 External Secrets Operator (ESO)

CNCF graduated project. Watches `ExternalSecret` resources, pulls from Vault, syncs to K8s Secrets, refreshes on schedule.

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: github-token-tenant-a
  namespace: agentic-ops
spec:
  refreshInterval: 15m
  secretStoreRef: vault-tenant-a
  target:
    name: github-token-tenant-a-k8s
  data:
    - secretKey: token
      remoteRef:
        key: secret/tenants/a/github
        property: pat
```

### 9.3 Two patterns of credential injection

**Pattern A — MCP server owns its credentials** (internal sources, where tenant has only one DB)

The MCP server pod mounts its own K8s Secret. Agent calls `postgres.query("SELECT...")`, never sees connection string. Connection lifecycle managed by the MCP server.

**Pattern B — Gateway injects credentials per-call** (external sources, per-tenant tokens)

```
Agent ──github.create_pr({title:"fix"})──► MCP Gateway
                                              │
                                              │ lookup: tenant=A, tool=github
                                              │ secret/tenants/a/github → glpat_abc
                                              │
                                              ├──► forwards to GitHub MCP with
                                              │    Authorization: Bearer glpat_abc
                                              │
                                              ▼
                                          GitHub API
```

### 9.4 Where each credential type lives

| Credential | Storage | Consumer |
|---|---|---|
| LLM API keys (OpenAI, Anthropic, Bedrock) | Vault `secret/llm/<provider>` | LLM Gateway only |
| Database creds (per-tenant Postgres) | Vault `secret/tenants/<t>/postgres` | Postgres MCP server pod |
| GitHub PATs / App tokens (per-tenant) | Vault `secret/tenants/<t>/github` | MCP Gateway (injected per call) |
| Cloud creds (AWS/Azure/GCP) | Workload Identity / IRSA / Federated Identity | MCP servers via service account, no static tokens |
| K8s API access (per agent) | ServiceAccount token (short-lived, projected) | k8s MCP server with appropriately scoped Role |
| TLS certs (mTLS internal) | cert-manager + internal CA | All pods automatically |
| OIDC client secret (UI auth) | Vault `secret/agentic-ops/oidc` | UI/control plane |

### 9.5 Rotation

- LLM keys: 90d via Vault rotation policy → ESO syncs → LLM Gateway picks up on next request (no restart)
- DB creds: dynamic via Vault DB engine (15min leases)
- Cloud creds: never rotated manually — ephemeral via Workload Identity
- Service account tokens: bound, projected, expire automatically (K8s native)

---

## 10. Encryption End-to-End

Five layers, all mandatory for production:

| Layer | Mechanism | Scope |
|---|---|---|
| **Transport (TLS 1.3)** | HTTPS for every endpoint, modern cipher suites only | All hops, inside and outside cluster |
| **mTLS internal** | Service mesh (Istio/Linkerd) sidecars OR cert-manager + manual mTLS config — **OR gRPC native mTLS for gRPC MCP servers** | Pod-to-pod inside `agentic-ops` and tenant namespaces |
| **App-layer auth (externals)** | OAuth2 / API tokens via Vault → injected by Gateway | External MCP servers, LLM providers |
| **At-rest** | etcd encryption-at-rest, Postgres TLS + disk encryption, S3 SSE-KMS, Cosign signatures on artifacts | All persisted data |
| **Egress control** | NetworkPolicy + egress firewall, allowlist of external destinations from `MCPServer`/`LLMConfig` CRDs | All outbound from cluster |

### 10.1 Transport-specific mTLS notes

**For gRPC transports (M3+):** mTLS is native in HTTP/2 / gRPC stack — no service mesh required. Configuration is per-MCPServer CRD (`tls.mode: mTLS` + cert refs). Gives strong auth without sidecar complexity, and works in environments where service mesh is not available (edge, air-gapped, restricted clusters).

**For HTTP transports (Streamable HTTP, SSE):** mTLS must come from one of:
- Service mesh (Istio/Linkerd auto-inject) — easiest in shared platform clusters
- cert-manager + manual nginx/envoy config in the MCP server pod — OK for small deployments
- ALB/Gateway with mTLS termination — works for ingress but not pod-to-pod

**Trade-off:** if you don't have a service mesh, gRPC transport is dramatically easier to secure to mTLS than HTTP. This is one more reason internal high-throughput MCPs (DBs, k8s) should migrate to gRPC in M3-M4.

```yaml
# auto-generated NetworkPolicy example
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: mcp-gateway-egress
spec:
  podSelector: { matchLabels: { app: mcp-gateway } }
  egress:
    - to:                                        # internal MCPs only
        - namespaceSelector: { matchLabels: { mcp: "true" } }
    - to:                                        # external allowlist
        - ipBlock: { cidr: 140.82.114.0/24 }     # github
      ports: [{ protocol: TCP, port: 443 }]
```

---

## 11. Audit Log — DORA / Compliance Grade

### 11.1 Four pillars

**Completeness:** every event in the chain (alert → routing → agent invocation → tool calls → LLM calls → notifications → approvals) shares a `correlation_id` and is loggable as a single trace.

**Immutability:** hash-chained entries, append-only DB, periodic anchor to WORM storage (S3 Object Lock Compliance mode or Azure Immutable Blob).

**Retention:**
- Hot tier (Postgres / ClickHouse): 90 days
- Warm tier (object storage): 1 year
- Cold WORM tier: 5-7 years (DORA financial: 5y; HIPAA: 6y)

**Trusted time:** NTP on all nodes, monotonic sequence numbers per entry.

### 11.2 Event schema (canonical)

```json
{
  "timestamp": "2026-04-19T14:32:01.234567Z",
  "monotonic_seq": 1234567890,
  "correlation_id": "inv-7f3a9c-2026-04-19",
  "parent_id": "inv-7f3a9c-2026-04-19",
  "event_type": "tool.invoke",
  "actor": {
    "type": "agent",
    "name": "hermes-sre",
    "version": "1.4.2@sha256:abc...",
    "service_account": "system:serviceaccount:tenant-a:hermes",
    "session_id": "sess-9c2d"
  },
  "subject": {
    "tenant": "tenant-a",
    "namespace": "tenant-a",
    "user_oidc_sub": "alice@company.com",
    "alert_id": "alert-postgres-pool-2026041914"
  },
  "action": {
    "tool": "postgres.query",
    "mcp_server": "postgres-tenant-a",
    "mcp_trust_tier": "internal-hosted",
    "args_hash": "sha256:def...",
    "args_redacted": "SELECT count(*) FROM pg_stat_activity..."
  },
  "outcome": {
    "status": "success",
    "duration_ms": 47,
    "rows_returned": 1
  },
  "context": {
    "source_ip": "10.42.3.7",
    "cluster": "prod-eu-west",
    "request_id": "req-abc123"
  },
  "integrity": {
    "prev_hash": "sha256:111...",
    "self_hash": "sha256:222..."
  }
}
```

### 11.3 SIEM export

Stream via OpenTelemetry collector → Kafka → Splunk/Elastic/Datadog/Chronicle in near real-time. Provides a tamper-evident copy outside the cluster (defense against insider threat) and lets the SOC use existing detection rules.

### 11.4 Segregation of duties

- Service account that writes audit cannot read
- Auditor role can read but cannot write or delete
- Cluster admin cannot delete WORM-tier records (enforced by S3 Object Lock — even root has no DELETE)

---

## 12. Prompt Injection Defenses (7 layers)

The threat: hostile content (in alert payloads, logs, fetched pages) attempts to override the agent's instructions.

| Layer | Defense | What it stops |
|---|---|---|
| 1 — Input sanitization | Length caps, control char stripping, Unicode NFKC, HTML escaping, pattern detection | Obvious injection attempts, oversize payloads |
| 2 — Structural separation | Wrap untrusted content in `<alert_payload trust="untrusted">…</alert_payload>` with explicit "treat as data, not instructions" rule | Most LLMs respect this with modern frontier models |
| 3 — **Least privilege on tools** | `AlertRoute.toolPolicy` declares allowed tools; MCP Gateway enforces; investigations get READ-only by default | **The most important layer.** Agent decides X but can't execute X |
| 4 — Dual-LLM pattern | Quarantined LLM extracts structured facts from raw alert; privileged LLM only sees structured data | High-stakes routes; eliminates indirect injection |
| 5 — Sandboxing | NetworkPolicy locks agent to MCP Gateway; read-only rootfs; dropped caps; no SA tokens | Agent compromise → blast radius zero outside its policy |
| 6 — Detection / observability | Audit anomalies; alert if agent calls a tool it never called before for that route | SOC catches novel attacks |
| 7 — Skill signature verification | All `SkillSource`s require Cosign-verified signatures | Attacker can't get malicious runbooks loaded |

**Mindset:** assume injection will happen. Design so that when it does, **damage is bounded** because agents have read-only tools, everything is audited, and any dangerous action requires HITL.

---

## 13. Observability End-to-End (OpenTelemetry)

### 13.1 The trace that crosses everything

A single distributed trace follows an alert from ingestion to notification. Spans:

```
[ alert.received ]                                              ← our control plane
  └─[ alert.routed ]                                            ← routing engine
      └─[ agent.invoked ] (adapter span, Holmes/Hermes/etc.)    ← adapter
          └─[ agent.investigation ] (inside the agent itself)
              ├─[ llm.call (gpt-4) ]                            ← LLM Gateway
              │   └─[ provider.openai.completions ]
              ├─[ tool.call (postgres.query) ]                  ← MCP Gateway
              │   ├─[ approval.requested ]                      ← HITL (if needed)
              │   ├─[ approval.granted ]
              │   └─[ mcp.postgres.query ]                      ← MCP server
              └─[ llm.call (gpt-4) ]
          └─[ agent.answer ]
      └─[ notification.fanout ]
          ├─[ teams.sent ]
          └─[ telegram.sent ]
```

Each span has:
- `correlation_id` matching the audit log
- `tenant`, `agent`, `investigation_id` attributes
- Cost data (tokens, USD) on LLM spans
- Tool args hash on MCP spans

### 13.2 Stack

- **OpenTelemetry SDK** (Go for our control plane, Python/JS auto-instrumentation for agents)
- **OTel Collector** (in-cluster) for batching/processing/routing
- **Tempo / Jaeger** for trace storage
- **Prometheus** for metrics (RED: rate, errors, duration)
- **Loki** for logs
- **Grafana** for unified UI

Customer can swap any of these for their existing stack — OTel is the standardized interface.

### 13.3 Self-monitoring metrics (operational)

```
# Health
agentic_ops_control_plane_up{instance}
agentic_ops_mcp_gateway_up
agentic_ops_llm_gateway_up

# Throughput
agentic_ops_alerts_received_total{tenant}
agentic_ops_investigations_started_total{tenant, agent}
agentic_ops_investigations_completed_total{tenant, agent, outcome}

# Latency
agentic_ops_investigation_duration_seconds{tenant, agent}
agentic_ops_llm_call_duration_seconds{provider, model}
agentic_ops_tool_call_duration_seconds{mcp_server, tool}

# Cost (for budget alerts)
agentic_ops_llm_cost_usd_total{tenant, agent, model}
agentic_ops_budget_remaining_usd{tenant}

# Security
agentic_ops_hitl_approvals_total{tenant, decision}
agentic_ops_audit_chain_breaks_total          # MUST be zero, alert if not
```

---

## 14. Disaster Recovery & Self-Monitoring

### 14.1 Service classification

**Agentic-Ops-Platform is an on-demand assistance service, NOT on the critical alert path.**

- Alerts continue to be received and notified by upstream (Alertmanager, Opsgenie, etc.) regardless of our state.
- If we are down, AI-assisted investigation is unavailable; humans handle alerts the traditional way.
- This is **explicit and acceptable**, and dramatically simplifies the DR design.

### 14.2 RTO / RPO targets

| Metric | Target |
|---|---|
| RTO (recovery time) | 15 minutes |
| RPO (data loss tolerance) | 0 for config (CRDs in etcd), 1 hour for history (last backup) |
| Availability target | 99% (~7.2h/month downtime) — adequate for assistance |
| HA strategy | Single-region, multi-AZ via standard K8s deployment patterns. No cross-region replication. |

### 14.3 Backup strategy

- **CRDs and Kubernetes config:** continuous via Velero or equivalent, encrypted offsite
- **Postgres:** nightly full backup + WAL streaming for PITR (1-hour RPO)
- **Audit log WORM tier:** already immutable in S3, no backup needed
- **Vault:** standard Vault DR (snapshot replication to secondary region for read-only access)

### 14.4 Failure modes and graceful degradation

| Component down | Behavior | User impact |
|---|---|---|
| Control plane | Alerts queue at ingress; processed when back | Investigations delayed |
| LLM Gateway | All LLM calls fail | All agents fail; UI shows "LLM unavailable" |
| MCP Gateway | All tool calls fail | Agents return partial results / error |
| Single MCP server | Tools from that source unavailable; others work | Agents get less context but still run |
| Single agent type | Routes for that agent fail; routes for others work | Partial degradation |
| Postgres | History/audit writes fail; runs degrade to in-memory | Read of history unavailable |
| UI | API still works, no graphical access | Use CLI or API directly |

### 14.5 Self-monitoring & alerting

The control plane runs its own Prometheus alert rules and routes them to a **fallback notification channel** that does NOT depend on our own notification system:

```yaml
# values.yaml
selfMonitoring:
  enabled: true
  fallbackChannel:
    type: webhook
    url: https://hooks.company.com/agentic-ops-down
    # OR direct PagerDuty / Opsgenie integration
  alertRules:
    - name: ControlPlaneDown
      condition: up{job="agentic-ops-control-plane"} == 0
      for: 2m
    - name: AuditChainBroken
      condition: agentic_ops_audit_chain_breaks_total > 0
      for: 0s                    # immediate
    - name: BudgetHardBreach
      condition: agentic_ops_tenant_budget_remaining_usd < 0
      for: 0s
```

---

## 15. AgentAdapter Versioning Policy

The interface IS the contract with every adapter (ours and customer-built). Breaking it costs customers money.

### 15.1 Semver applied

- **MAJOR**: incompatible changes to the interface (remove/rename methods, change signatures). Requires migration guide, 6-month deprecation window.
- **MINOR**: additive (new optional method with default impl, new optional field in `InvestigateRequest`, new `EventType`). Backward compatible.
- **PATCH**: bug fixes in shared utilities, no interface change.

### 15.2 Backward compatibility rules

- New methods ship with a default implementation in a base struct (`BaseAdapter`); old adapters embed it and gain the method automatically as a no-op.
- New fields in request/event structs are optional; adapters can ignore them safely.
- Removed methods get marked `// Deprecated: removed in v2` for two minor versions before actual removal.

### 15.3 Contract test suite

Published as a Go module. Any adapter — ours or customer's — runs:

```bash
go test github.com/agentic-ops/adapter-conformance@v1.x
```

It verifies:
- All required methods implemented
- Streaming behavior conforms (events arrive, channel closes properly)
- Tool call respects `AvailableTools` allowlist
- Budget enforcement (returns error when `BudgetLimit` exceeded)
- Event type mapping consistent
- Health check correctly returns errors

CI in our project runs these against every bundled adapter. Customers run them in their own CI for custom adapters.

### 15.4 Compatibility matrix in docs

```
Control Plane v1.x supports adapters: v1.x (current), v0.x (deprecated)
Control Plane v2.x will support:      v2.x (current), v1.x (deprecated)
```

Adapters and control plane can be upgraded independently within a major version.

---

## 16. Agent Orchestration & A2A (first-class feature)

The platform's strategic differentiator. Three patterns of agent collaboration are supported.

### 16.1 The three patterns

**Pattern A — Custom adapters (anyone builds an agent)**

Anyone implements the `AgentAdapter` interface (5 methods, ~150-300 lines), packages it in a container, registers via `AgentRegistration` CRD. Standard agent ready for use. Already covered in Section 4.

**Pattern B — Agent invokes Agent (cross-agent orchestration via control plane)**

A primary agent decides it needs specialized help and invokes another registered agent. The platform mediates: budget propagation, audit, HITL, loop detection.

```
Holmes investigating DB incident
    │
    ├─ tool: postgres.query → 423 active connections (limit 500)
    ├─ tool: kubernetes.logs → "max_connections exceeded"
    │
    └─ tool: agentic_ops.invoke_agent({
          agent: "security-specialist",
          context: "...",
          ask: "Is this connection pattern compromised?",
          max_cost_usd: 2.00,
          max_duration_sec: 60
       })
       │
       └─→ Platform validates + invokes Security Agent as sub-investigation
            │
            ├─ Security Agent runs its own investigation
            ├─ Has its own MCP tools (network MCP, audit MCP)
            ├─ Budget enforced (min of: tenant, route, parent remaining, hint)
            ├─ Audit nested with parent_correlation_id
            │
            └─→ Returns analysis to caller
       │
       Holmes integrates result, continues
```

**Pattern C — Subagents inside an agent (agent-internal nesting)**

OpenCode and similar agents natively support spawning subagents inside their own process. The platform doesn't orchestrate this — it happens inside the adapter — but the platform observes, audits, and costs it.

```
OpenCode investigating complex issue
  └─ spawns internal subagent "log-analyzer" (within OpenCode)
      └─ spawns internal subagent "diff-summarizer" (within OpenCode)

Platform sees:
  - OTel traces capture sub-spans
  - Audit log records subagent_spawned events
  - LLM Gateway attributes tokens to parent investigation
  - Budget enforced on total (parent + all subagents)
```

The OpenCode adapter (M3) maps OpenCode's internal sub-conversations to platform events.

### 16.2 A2A authorization — defense in depth

A sub-agent invocation is allowed **only if all four layers say yes**:

1. **Agent policy** (Q14b — global capability of the caller agent)
2. **Route team** (Q14c — contextual permission for the incident type)
3. **Approval policy** (HITL where required)
4. **Tenant scoping + RBAC** (no cross-tenant invocation)

```yaml
# Layer 1: Agent policy (set per agent, by agent admin)
apiVersion: agenticops.io/v1
kind: AgentRegistration
metadata:
  name: holmes-prod
spec:
  adapter:
    image: robustadev/holmes:0.24.3
  canInvoke:                    # whitelist (default: ["*"])
    - dba-specialist
    - security-specialist
    - cost-optimizer
  cannotInvoke:                 # blacklist (always wins)
    - claude-code-destructive   # never, regardless of route

---
# Layer 2: Route team (set per alert type, by operator)
apiVersion: agenticops.io/v1
kind: AlertRoute
metadata:
  name: prod-database-incidents
spec:
  match:
    labels: { type: database, severity: critical }
  agent: holmes-prod
  team:                         # who plays in this incident
    - dba-specialist
    - security-specialist
  allowedSources: [postgres-mcp, prometheus-mcp]
  notify: [teams-dba]

---
# Layer 3: Approval policy (per tool/action, optionally per A2A)
apiVersion: agenticops.io/v1
kind: ApprovalPolicy
metadata:
  name: cross-agent-invocations
spec:
  scope:
    a2a: true
  tiers:
    autoApprove:
      callers: [holmes-prod]
      targets: [dba-specialist, cost-optimizer]
    singleApproval:
      callers: ["*"]
      targets: [security-specialist]   # always needs human approval
      channel: slack:#sre-approvals
```

**Defaults are permissive (`canInvoke: ["*"]`)** to make experimentation easy. Operators tighten as needed.

### 16.3 Budget propagation — 4-layer min

When a sub-agent is invoked, the effective budget is **the minimum of all four**:

```
effective_sub_budget = min(
    tenant_budget_remaining,           # global cap
    route_budget_remaining,            # route operator cap
    parent_investigation_remaining,    # what's left of the parent's bucket
    caller_hint                        # caller agent's per-call hint
)
```

**The caller hint is a ceiling, not a floor.** A caller cannot exceed external policy by asking for more.

```go
// What an adapter does to invoke another agent
type SubAgentInvocation struct {
    Target       string
    Context      string
    Ask          string
    BudgetHint   *CostBudget        // optional, caller's per-call ceiling
    DurationHint *time.Duration     // optional, caller's time ceiling
}
```

Smart agents give more budget early in an investigation (high uncertainty) and less budget late (verification only).

### 16.4 Loop & fan-out prevention

```yaml
# values.yaml defaults
agentOrchestration:
  maxInvocationDepth: 5         # depth >= 5 → hard error
  maxFanOutPerAgent: 3          # one agent cannot invoke >3 sub-agents in parallel
  maxTotalSubInvocations: 20    # entire investigation cannot exceed 20 sub-calls

  cycleDetection:
    enabled: true
    onCycle: hardError          # alternative: pause for HITL approval

  onLimitHit:
    default: hardError          # caller gets clean error
    overridable: true           # ApprovalPolicy can require human approval to extend
```

When a limit is hit:
- Audit log: `event_type: agent.limit_exceeded` with details
- Caller gets explicit error with reason
- Caller can adapt (continue without sub-info, or escalate)
- HITL override available for critical-prod routes

### 16.5 UI: investigation tree view

Rather than linear conversation, the UI shows incident as a tree:

```
Investigation: postgres-down-2026-04-19  [$3.42 / $5.00 budget]
│
├─ [Holmes-prod] "Detected connection pool exhaustion..."
│   ├─ tool: postgres.query → 423 connections (limit 500)
│   ├─ tool: kubernetes.logs → max_connections errors
│   │
│   └─ ↳ INVOKED: security-specialist  [$0.87 / $2.00 hint]
│       ├─ "Analyzing connection sources..."
│       ├─ tool: network.flow_logs → 87% from one pod IP
│       ├─ tool: kubernetes.describe_pod → recently deployed v2.3
│       └─ "Pattern: connection leak in v2.3, NOT attack"
│
├─ [Holmes-prod] "Confirmed: not security event."
└─ [Holmes-prod] "Root cause: connection leak. Recommend rollback to v2.2"
```

Click any node → see full audit, cost, MCP calls, LLM calls.

### 16.6 What this enables

- **Specialization**: a generalist agent (Holmes) consults specialists (DBA, security) without operators having to anticipate every combination
- **Reusability**: a "compliance checker" agent built once is invokable from any other agent
- **Composition**: complex incidents resolved by teams of agents, not single super-agent
- **Open ecosystem**: anyone publishing an open-source agent makes it available to all platform users

This is the strategic differentiator vs Robusta UI (single-agent), kagent (no incident orchestration), and DIY scripts (no governance).

---

## 17. Open Source Strategy

The platform is built **as** an open-source project, not "released to open source after the fact". Decisions cascading from this:

### 17.1 License

**Apache 2.0** — the standard for open-source infrastructure. Permits use, modification, distribution, commercial use. Patent grant protects users and contributors. Compatible with the rest of the CNCF/Kubernetes ecosystem.

### 17.2 Naming

The project name will:
- Not include any registered trademark (Holmes, Hermes, Claude, Robusta, Harness, Kagent, OpenAI, Anthropic, GitHub, etc.)
- Be searchable, memorable, and have an available domain + GitHub org name
- Be decided in Milestone 0 — current placeholder: `agentic-ops` / `Agentic-Ops-Platform`

Candidate naming directions for M0 brainstorm:
- "AgentRouter" — emphasizes orchestration
- "IncidentForge" — emphasizes the use case
- "Sentinel-AI" or similar — emphasizes detection/response
- Something neutral like "OpsAgent" / "AgentMesh"

### 17.3 Repository structure (from M1)

```
github.com/<org>/agentic-ops-platform/
├── README.md                  # the pitch + 5-min quickstart
├── LICENSE                    # Apache 2.0 full text
├── CONTRIBUTING.md            # how to contribute, dev setup, testing
├── CODE_OF_CONDUCT.md         # standard Contributor Covenant
├── SECURITY.md                # vulnerability reporting process
├── CHANGELOG.md               # semver-tracked changes
├── ROADMAP.md                 # public roadmap, milestone tracking
│
├── cmd/                       # binary entry points (control plane, gateway, etc.)
├── pkg/                       # public Go packages (importable)
├── internal/                  # private code
├── api/                       # CRD definitions (Kubebuilder/controller-gen)
├── ui/                        # React frontend
│
├── adapters/                  # in-tree reference adapters
│   ├── holmesgpt/
│   ├── hermes/
│   ├── opencode/
│   └── claude-code/
│
├── sdk/                       # the public SDK for custom adapters
│   ├── adapter/               # Go interface, base types
│   ├── conformance/           # contract tests as importable module
│   └── examples/              # 3-5 reference custom adapters
│
├── docs/                      # mkdocs/docusaurus site
│   ├── quickstart.md
│   ├── architecture.md
│   ├── howto-write-adapter.md
│   ├── howto-write-mcp-server.md
│   └── ...
│
├── helm/                      # official Helm chart
│   ├── agentic-ops-platform/
│   └── values.profiles/       # corporate.yaml, local.yaml, experimental.yaml
│
├── deploy/                    # Kustomize overlays, kind config, demo setup
├── examples/                  # example AlertRoutes, MCPServers, full demos
└── .github/
    ├── workflows/             # public CI
    ├── ISSUE_TEMPLATE/
    │   ├── bug_report.md
    │   ├── feature_request.md
    │   └── agent_integration.md
    └── PULL_REQUEST_TEMPLATE.md
```

### 17.4 Packaging & distribution

| Artifact | Where |
|---|---|
| Container images | `ghcr.io/<org>/agentic-ops-platform/*` (free for OSS) |
| Helm chart | OCI registry: `oci://ghcr.io/<org>/charts/agentic-ops-platform` |
| Helm repo legacy | GitHub Pages: `<org>.github.io/agentic-ops-platform` |
| Go SDK module | `github.com/<org>/agentic-ops-platform/sdk` |
| Documentation | `<org>.github.io/agentic-ops-platform/docs` or custom domain |
| Demo cluster | `kind create cluster --config deploy/kind/demo.yaml` |

### 17.5 Plugin extensibility

Three ways to extend the platform without forking:

**1. In-tree adapters (PR to main repo)**
- Best for adapters that benefit the whole community
- Subject to contract tests + maintainer review
- Ship in core release, get free CI/security updates

**2. Out-of-tree adapters (separate repo)**
- For private/proprietary agents
- Implement the SDK interface, build container, register via CRD
- Compatible with stable SDK versions (semver promise)

**3. Sidecar adapters (gRPC, future)**
- For adapters in non-Go languages
- Run as sidecar to control plane, communicate via gRPC
- Documented but not first-class until M5+

### 17.6 Community model

- **Issues + Discussions** on GitHub for everything
- **Maintainers**: starts with the building team; documented path to add maintainers based on contribution
- **No commercial pressure**: no enterprise edition, no paid tier, no "open core" trick
- **Donation-ready** but not solicited: if it succeeds, consider CNCF Sandbox eventually
- **No secret roadmap**: ROADMAP.md is the truth, public discussion shapes it

### 17.7 Quality bar (extra work to be done right)

Open source quality means:
- Public CI from day 1 (GitHub Actions)
- Cosign-signed container images
- SBOM generation for every release
- Security policy + responsible disclosure
- Documentation that doesn't assume context
- Examples that actually work
- Demo that runs on a laptop in 5 minutes

**Adds ~2 weeks distributed:** ~3 days in M1 (repo + CI), ~1 week in M3 (SDK packaging + docs), ~1 week in M6 (polish + quickstart + contributing guide).

---

## 18. Dual-Environment Profiles

The same Helm chart, the same binary, runs in two distinct profiles. This is a feature of the open-source distribution: anyone can run it at home, or in their company.

### 18.1 The two profiles

**`profile: corporate`** — for enterprise deployments
- LLM providers limited to compliance-allowed list (e.g., Azure OpenAI, GitHub Copilot)
- OIDC: enterprise IdP (Azure AD, Okta, etc.)
- Audit: full hash chain + WORM + SIEM export
- Secrets: Vault required (no K8s Secrets shortcut)
- mTLS: mandatory cluster-wide
- NetworkPolicies: strict, egress allowlist enforced
- Budgets: required per tenant
- HITL: required for write actions

**`profile: local`** — for home/dev/experimentation
- LLM providers: anything (Anthropic direct, OpenRouter, Ollama, mock)
- OIDC: Dex with static users / GitHub OAuth (or skip with single-user mode)
- Audit: minimal, short retention
- Secrets: K8s Secrets OK (simpler)
- mTLS: optional
- NetworkPolicies: relaxed
- Budgets: optional, soft warnings only
- HITL: optional

**`profile: experimental`** (sub-mode of local) — for trying brand-new things
- All `local` defaults plus
- Pre-release adapters allowed
- Unsigned skills permitted
- Verbose debug logging on
- Budget enforcement off

### 18.2 How profiles are applied

In `values.yaml`:

```yaml
# Choose one
profile: corporate    # or: local, experimental

# Override anything from the profile defaults
overrides:
  llm:
    additionalProviders:
      - name: my-custom-llm
        ...
  audit:
    retention: 180d
```

The Helm chart loads the matching `values.profiles/<profile>.yaml` first, then applies user overrides. This makes it impossible to accidentally enable a dev-only setting in production: the corporate profile sets `audit.retention: 1y` minimum, and any override below the floor produces a Helm install error.

### 18.3 Why this matters

- **Anyone can play at home** — install on a Mac with kind, Anthropic API key, in 10 minutes
- **Enterprises get safe defaults** — no "be sure to enable X" footguns
- **Same code path** — bugs found at home hit the same code as production
- **Open-source success metric** — projects that "only work on the maintainer's setup" die. Profiles force testing on both.

---

## 19. Memory & Knowledge Persistence Strategy

### 19.1 The framing problem

"Memory" in agentic systems is overloaded. Before deciding what to build, we distinguish four distinct kinds:

| # | Type | What it means | Example | Where it lives in Leloir |
|---|---|---|---|---|
| 1 | **Intra-investigation memory** | Context the agent maintains within a single investigation | Holmes remembers what tools it called 30 seconds ago | Inside the agent (every adapter handles this) |
| 2 | **Session memory / follow-ups** | User asks consecutive questions about the same investigation | "What did the previous query show?" | `SessionID` in InvestigateRequest; agent handles |
| 3 | **Cross-investigation memory (per-tenant)** | "Last time we saw this pattern, root cause was X" | Pod restart loop → similar incident from 3 weeks ago | **Not in v1.0; candidate for v1.1 (see 19.5)** |
| 4 | **User/agent specialist memory** | "This tenant prefers PostgreSQL"; "this agent learned to detect X pattern" | Per-tenant preferences, learned heuristics | **Not in v1.0; available as integration (see 19.4)** |

Types 1 and 2 are already solved by each agent. Types 3 and 4 are where "memory features" usually get over-engineered.

### 19.2 Core principle: memory is an integration, not a feature

Leloir's v1.0 does NOT ship a built-in memory subsystem. Instead, the platform provides extension points (MCPServer CRD) so operators can plug in any external memory system that exposes an MCP server. This is consistent with the platform's overall philosophy: be agnostic, don't lock users into one approach.

**What this means concretely:**
- Out of the box: agents work without persistent cross-session memory
- Operators who want memory: register one or more memory MCP servers
- Each tenant decides independently
- Memory vendors (MemPalace, Zep, Mem0, custom) compete on merits, not on Leloir lock-in

### 19.3 Why not build memory into core

**Reasons:**

1. **Each agent already manages its own memory better than we could.** Holmes, OpenCode, Claude Code each have sophisticated context handling. Forcing them into our memory model would degrade them.

2. **Memory ≠ audit log.** Our audit log is compliance-grade (DORA WORM, hash chain, 7-year retention). Memory is for the agent to recall things; audit is legal evidence. Fundamentally different requirements.

3. **Memory ≠ skills.** Skills are prescriptive knowledge curated by operators ("here's how to investigate a DB incident"). Memory is descriptive accumulation from past investigations. Complementary, not interchangeable.

4. **Multi-tenant memory is hard.** Most memory systems assume single-user. Making them safely multi-tenant adds significant complexity and surface area for cross-tenant leaks.

5. **Memory is a fast-moving research area.** ChromaDB, Weaviate, Qdrant, Zep, Mem0, MemPalace, LangGraph memory, Letta, Cognee — the landscape changes monthly. Tight integration = constant churn. Loose integration via MCP = stability.

6. **Agnostic > opinionated.** Telling users "you must use our memory model" contradicts the agnostic ethos that defines the project.

### 19.4 How to integrate external memory systems

The MCPServer CRD already supports this without any code changes. Example for MemPalace:

```yaml
# Step 1: Deploy MemPalace as a sidecar or standalone pod (out of scope for Leloir)
# In tenant-a's namespace:
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mempalace-tenant-a
  namespace: tenant-a
spec:
  template:
    spec:
      containers:
        - name: mempalace
          image: ghcr.io/mempalace/mempalace:3.1.0
          # MemPalace MCP server config
          env:
            - name: MCP_TRANSPORT
              value: stdio  # or streamable-http
          volumeMounts:
            - name: mempalace-data
              mountPath: /var/lib/mempalace

---
# Step 2: Register the memory MCP server with Leloir
apiVersion: leloir.dev/v1
kind: MCPServer
metadata:
  name: tenant-a-memory
  namespace: tenant-a
spec:
  trustTier: internal-hosted    # we control the deployment
  visibility: namespace          # only tenant-a sees this
  transport:
    type: streamable-http
    endpoint: http://mempalace-tenant-a:8080
  allowedTools:
    # READ-only by default — agents can search/recall
    - mempalace_search
    - mempalace_kg_query
    - mempalace_list_wings
    - mempalace_list_rooms
    - mempalace_get_taxonomy
    - mempalace_traverse
    # Optionally allow write (with HITL recommended)
    # - mempalace_add_drawer
    # - mempalace_kg_add
  policies:
    rateLimit: 100/min
    requireApproval:
      - mempalace_add_drawer    # writes need approval
      - mempalace_kg_invalidate

---
# Step 3 (optional): expose memory writes to specific agents only via route
apiVersion: leloir.dev/v1
kind: AlertRoute
metadata:
  name: tenant-a-incidents-with-memory
spec:
  match:
    labels: { tenant: tenant-a }
  agent: holmes-prod
  allowedSources:
    - tenant-a-memory          # this route's agent can use memory tools
    - prometheus-mcp
    - kubernetes-mcp
```

That's it. Holmes investigating an incident sees `mempalace_search` as a tool, calls it, gets relevant past context, integrates into reasoning. **Zero changes to Leloir core. Operator opt-in. Per-tenant scoped.**

### 19.5 Recommended memory integrations (curated list for documentation)

In `docs/integrations/memory.md`, Leloir will document:

| System | License | Best for | Trade-offs |
|---|---|---|---|
| **MemPalace** | MIT | Home/dev users, single-tenant per pod, free | Young (v3.1, April 2026); Python; designed for single-user |
| **Zep / Graphiti** | Apache 2.0 (Graphiti) / SaaS (Zep) | Enterprise, temporal KG, managed option | Cloud option costs $25+/mo; Neo4j dependency for self-host |
| **Mem0** | Apache 2.0 | Simple API, broad LLM compat | Less feature-rich for complex queries |
| **Letta (formerly MemGPT)** | Apache 2.0 | Self-managing memory agents | Heavier abstraction; more for agent-as-product |
| **Cognee** | Apache 2.0 | Knowledge-graph focused | Newer, less battle-tested |
| **Custom** | — | Existing internal knowledge bases | Build your own MCP wrapper around your DB |

The documentation gives operators a decision matrix, not a mandate. Each link has a sample MCPServer YAML.

### 19.6 When (if ever) to build native memory in Leloir

**Cross-investigation pattern memory (Type 3)** is the most likely candidate to become a native feature, but only if:

- Multiple users in M0-M5 explicitly request it
- External memory systems prove operationally awkward for the use case
- The benefit is clear enough to justify another moving part

If built (target: v1.1, post-v1.0 release), it would be intentionally minimal:

```
Database: PostgreSQL table `investigation_insights`
Schema:
  tenant_id           uuid
  alert_pattern_hash  text       -- hash of normalized alert labels
  alert_pattern_text  text       -- human-readable summary
  root_cause          text       -- agent's conclusion
  recommendation      text
  confidence          numeric
  occurrences         int
  last_seen           timestamptz
  investigation_ids   uuid[]     -- links to audit log
  
Internal MCP server: `leloir.memory`
Tools:
  - similar_past_incidents(alert_context) → list of matches
  - record_investigation_outcome(investigation_id, root_cause, confidence)
  
Behavior:
  - Always per-tenant scoped (no cross-tenant)
  - Pattern matching via vector similarity on alert labels
  - 90-day rolling window (drop older insights to keep relevance fresh)
```

This covers ~80% of "useful cross-investigation memory" in ~500 lines of Go and one Postgres table. **No external dependency. Multi-tenant native. Operationally trivial.**

It is NOT planned for v1.0. It's a v1.1 candidate based on real user demand.

### 19.7 What about user/agent specialist memory (Type 4)?

Examples:
- "This tenant prefers PostgreSQL recommendations over MySQL"
- "Agent X has learned that Y pattern usually means Z"
- "User prefers brief summaries vs detailed analysis"

**Leloir's stance:** these are agent-specific concerns. The agent knows best how to model and use them. Leloir provides:

- The MCPServer extension point if the agent wants persistent memory (via integration, see 19.4)
- The CustomConfig field in AgentRegistration for static per-tenant preferences ("this tenant defaults to verbose responses")
- The audit log + Skills system for everything else

We do NOT build a "user preferences" system. That's product creep that solves nothing the existing tools don't already solve.

### 19.8 Summary

**v1.0 of Leloir:**
- ✅ Each agent handles its own intra-investigation and session memory
- ✅ MCPServer CRD allows registering any external memory system per tenant (zero core changes needed)
- ✅ Documentation lists curated memory integration options (MemPalace, Zep, Mem0, etc.)
- ❌ No native memory subsystem
- ❌ No tight coupling to any specific memory vendor

**v1.1+ of Leloir (if user demand justifies):**
- 🟡 Optional native cross-investigation memory (Postgres-based, ~500 lines, single internal MCP server)

**Never planned:**
- ❌ Replacing per-agent memory with platform-managed memory
- ❌ Tight integration with any specific memory framework
- ❌ Built-in user preferences system

This keeps the surface area small, respects the agnostic ethos, and lets users choose what fits their environment.

---

## 20. Tech Stack (consolidated)

| Layer | Choice | Rationale |
|---|---|---|
| Frontend | React + Vite + TypeScript + Tailwind + shadcn/ui | Fast iteration, polished UX |
| Backend (control plane) | **Go** | k8s-native, single binary, strong concurrency for streaming, easy adapter authoring |
| Persistence | PostgreSQL | Routes, alerts, sessions, audit (hot tier) |
| Hot audit tier | PostgreSQL | 90d retention, fast queries |
| Cold audit tier | S3 Object Lock (Compliance mode) — corporate profile | WORM, 5-7 year retention |
| Realtime | SSE primary, WebSocket fallback | Stream agent events to UI |
| **LLM Gateway** | LiteLLM (Apache 2.0) | Cost meter, fallback, cache, multi-provider |
| **MCP Gateway** | Custom thin Go service, multi-transport | Per-tenant scoping + credential injection + transport translation |
| **Secrets** | HashiCorp Vault + External Secrets Operator (corporate) / K8s Secrets (local) | Industry standard, rotation, audit |
| Service mesh (optional) | Istio or Linkerd | Auto mTLS for HTTP transports; if customer doesn't have one, use cert-manager + manual mTLS, or rely on gRPC native mTLS for internal MCPs |
| Cert management | cert-manager + internal CA | Auto-rotation, mTLS certs |
| Auth | OIDC + Kubernetes TokenReview (Azure AD / Dex / Okta / Keycloak / Google) | Cloud-agnostic |
| Observability | OpenTelemetry SDK + OTel Collector + Tempo + Prometheus + Loki + Grafana | Standard, swappable |
| Backup | Velero (K8s state) + pg_basebackup + WAL-G (Postgres PITR) | Standard tooling |
| Packaging | Helm chart with `profile: corporate \| local \| experimental` | One artifact, multiple environments |
| CI/CD | GitHub Actions, multi-arch images, Cosign-signed, SBOM generated | Supply chain hygiene + OSS quality bar |

### Our CRDs (consolidated)

- `Tenant` — tenant definition, namespace mapping
- `AlertRoute` — alert → agent + sources + team + channels
- `NotificationChannel` — Teams/Telegram/Slack/Webhook configs
- `MCPServer` — MCP source with trust tier + transport
- `SkillSource` — skill repos with visibility
- `AgentRegistration` — agent definition with `canInvoke`/`cannotInvoke`
- `TenantBudget` — cost limits
- `ApprovalPolicy` — HITL rules (covers tools and A2A)

---

## 21. Modular Delivery Plan — Build Small, Grow Incrementally

The principle: **every milestone delivers something testable end-to-end and useful by itself.** No one waits 6 months to see the system work. Each milestone is a small but complete vertical slice.

### Milestone 0 — Skeleton PoC (2 weeks)

**Goal:** prove the absolute minimum end-to-end flow works before investing further.

- HolmesGPT installed standalone (official Helm chart)
- 100-line Go service that:
  - Receives a webhook (no auth, no validation — throwaway)
  - Calls HolmesGPT HTTP API directly
  - Streams response to a single browser SSE endpoint
- Single-page HTML to view the stream
- One real Prometheus alert flowing through

**You can demo:** an alert fires, AI investigates, response streams to your screen.

### Milestone 1 — Production-shape MVP (5-6 weeks)

**Goal:** the same flow but production-grade, single-tenant, with proper OSS foundations.

- React UI with proper alert inbox + investigation timeline
- Go control plane with proper webhook receiver, ingestion, routing engine
- HolmesGPT adapter implementing the v1 AgentAdapter interface
- MCP Gateway thin v1 — **designed with pluggable transport interface from day one**, but only the Streamable HTTP transport implemented (the only one widely supported by community MCPs today)
- `AlertRoute`, `NotificationChannel`, `MCPServer` CRDs (with `transport` block, even if only `streamable-http` is wired up)
- Teams + Telegram notifiers
- Postgres for history
- Helm chart with `profile: corporate | local` presets
- TLS everywhere
- Basic observability (Prometheus metrics + structured logs)
- **OSS foundations** (~3 days): public GitHub repo, Apache 2.0 license, CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md, GitHub Actions CI public, container images on ghcr.io with Cosign signing, basic README with quickstart

**You can demo:** real production alert flows, agent investigates, routes to right channel, history queryable in UI, anyone can clone the repo and run a local demo with kind in 10 minutes.

### Milestone 2 — Multi-tenancy (4-5 weeks)

**Goal:** safe to onboard real tenants.

- OIDC authentication
- `Tenant` CRD with namespace mapping
- Per-NS RBAC enforcement
- MCP Gateway per-tenant scoping (the critical security work)
- Audit log (hot tier in Postgres, structured schema)
- Skills system v1 (`SkillSource` CRD, signature verification, immutable per pod)
- `mode: per-namespace` added to Helm chart

**You can demo:** two tenants share the cluster, can't see each other, all actions audited.

### Milestone 3 — Cost, second/third agents, SDK, A2A foundations (5-6 weeks)

**Goal:** financial visibility + prove the abstraction holds + open the platform to community contributions + lay groundwork for A2A.

- LLM Gateway integration (LiteLLM)
- `TenantBudget` CRD + cost dashboard in UI
- Second adapter: **Hermes Agent** (proves interface holds for very different agent)
- Third adapter: **OpenCode** (proves client/server pattern; **OpenCode adapter implements Pattern C — observes nested subagents, maps them to platform events**)
- **Custom Adapter SDK published as Go module** with:
  - Public `AgentAdapter` interface
  - Reference adapter implementation
  - Conformance test suite (`go test github.com/<org>/agentic-ops-platform/sdk/conformance`)
  - "How to write an adapter" documentation
  - 2-3 example adapters in `sdk/examples/`
- **MCP Gateway: gRPC outbound transport added** + stdio transport for local sandboxed servers
- Audit log warm tier (object storage)
- **A2A foundations (Pattern B prep):** `agentic_ops.invoke_agent` tool registered in MCP Gateway as a special internal MCP server. Not enforced/orchestrated yet — just plumbing.

**You can demo:** budgets enforced, runaway agents stopped, three different agent types running, cost visible per tenant, gRPC MCP server connected transparently, **anyone in the community can write a custom adapter in 1-2 days following the SDK guide**.

### Milestone 4 — Safety, scale, A2A v1 (5-6 weeks)

**Goal:** safe enough for write actions + first version of cross-agent orchestration.

- HITL workflow (`ApprovalPolicy` CRD, Slack/Teams approvals, UI)
- Secrets management integration (Vault + ESO for corporate profile)
- Prompt injection defenses (sanitization, structural separation, dual-LLM optional)
- Audit log WORM tier (S3 Object Lock — corporate profile)
- SIEM export
- Sources expansion: Loki, Postgres, MySQL MCPs
- **Internal MCPs we ship are now gRPC-native** (custom Postgres MCP with tenant scoping). Native mTLS, no service mesh dependency, ~3-4x throughput vs HTTP equivalent.
- **A2A v1 — Pattern B implemented:**
  - SDK extended with sub-agent invocation API
  - `AgentRegistration.canInvoke` / `cannotInvoke` policies enforced
  - Budget propagation (4-layer min: tenant > route > parent > caller hint)
  - Audit log captures parent_correlation_id for nested investigations
  - `ApprovalPolicy` extended for cross-agent invocations
  - Cycle detection (hard error on loop)
  - Hard cap enforcement (depth, fan-out, total)

**You can demo:** agent attempts a destructive action, SRE approves in Slack, action executes, fully audited; secrets never leave Vault; **Holmes investigating a database incident invokes a security-specialist sub-agent for analysis, both costs roll up to the parent investigation, full audit trail with nested correlation IDs**.

### Milestone 5 — Ecosystem, A2A polish, advanced features (5-6 weeks)

**Goal:** breadth + production-ready agent orchestration.

- Claude Code adapter (sandboxed, stdio transport)
- AWS CloudWatch + Azure Monitor + Datadog + Opsgenie sources
- Routing engine v2 (label-based, fallback chains)
- Slack channel
- Runbook editor in UI
- End-to-end OpenTelemetry traces with Tempo + Grafana
- Self-monitoring + DR alerts to fallback channel
- **A2A polish:**
  - Investigation tree view in UI (visual hierarchy of parent + sub-agents)
  - Per-agent and per-route invocation analytics dashboard
  - HITL override path for limit hits (extend depth on critical incidents)
  - Cross-tenant invocation explicitly blocked + audit alerted
  - Performance: parallel sub-agent invocations honor `maxFanOutPerAgent`
- Per-NS deployment mode added to Helm chart (for tenants needing hard isolation)

**You can demo:** alert from CloudWatch → routed to Hermes → Hermes consults Claude Code subagent for code review → finds root cause → opens GitHub PR (with HITL approval); UI shows the entire investigation tree with cost per branch.

### Milestone 6 — Hardening, docs polish, v1.0 release (3-4 weeks)

**Goal:** ship a release the community can trust.

- Load test (especially MCP Gateway under multi-tenant fan-out + A2A under nested investigations)
- Security review (multi-tenant RBAC, MCP scoping, sandbox escape, prompt injection penetration, A2A authorization defense in depth)
- Disaster recovery drill (simulate failures, verify RTO)
- Backup/restore verification
- **OSS quality bar (~1 week):**
  - Complete documentation site (mkdocs/docusaurus)
  - 5-minute quickstart guide that actually works on a fresh laptop
  - SDK guide for writing custom adapters with a tutorial
  - Architecture deep-dive docs
  - Troubleshooting + FAQ
  - SBOM generation + supply chain hygiene
  - Security policy + responsible disclosure
- Apache 2.0 v1.0 release with proper changelog
- Public announcement (blog post, optional KubeCon-style talk submission)

### Total: ~25-30 weeks to v1.0

**Critically, value is delivered every 4-6 weeks:**
- Week 2: working PoC
- Week 8: usable single-tenant product, public OSS repo
- Week 13: multi-tenant, can onboard real teams
- Week 19: cost-controlled, multi-agent, **community can write custom adapters**
- Week 25: safe for write actions, **A2A orchestration working**
- Week 31: full ecosystem with Claude Code, runbooks, tree view UI
- Week 35: v1.0 shipped

If at any milestone the customer wants to pause and run on what exists, that's a viable end state. The OSS-first design means the community can also continue from any point.

---

## 22. Risks & Resolved Decisions

### Risks (consolidated, with A2A additions)

| Risk | Mitigation |
|---|---|
| We own all adapters | Thin (~150-300 lines), contract tests in CI, version pinning per agent |
| MCP Gateway bug = cross-tenant leak | Dedicated security review, fuzz testing, small focused codebase |
| LLM cost runaway across multiple agents | LLM Gateway with hard budgets per tenant/investigation/agent from day 1; A2A budget propagation as 4-layer min |
| Prompt injection via alert content | 7-layer defense; least-privilege tools is the main bet |
| Adapter sprawl (many quirky agents) | Tier model: core / supported / community |
| Vault/secrets compromise | Defense in depth: Workload Identity over static secrets where possible, audit on every secret read |
| Audit log tampering | Hash chain + WORM + SIEM external copy |
| Skill update breaks all agents at once | Immutable per pod lifetime, canary rollouts, per-tenant pinning |
| Frontier LLM provider disruption | LLM Gateway fallback chains: Azure → Copilot (corporate); Anthropic → OpenRouter (local) |
| OSS agent project pivots/dies | Adapters are small; replace agent without changing control plane |
| MCP transport ecosystem fragmentation (gRPC vs HTTP vs stdio) | Pluggable transport interface in Gateway from M1 |
| gRPC adoption requires new team skills | Phased: HTTP-only in M1-M2, gRPC for internal MCPs only in M3-M4 |
| gRPC-MCP spec still in flux | Pin to stable spec version, contribute upstream, fall back to HTTP if spec breaks |
| **A2A fan-out attack** (compromised agent invokes 100 sub-agents to drain budget/DoS) | **maxFanOutPerAgent (3 default), maxTotalSubInvocations (20 default), per-call BudgetHint as ceiling not floor** |
| **A2A infinite loops** (A invokes B invokes A) | **Cycle detection on call stack + maxInvocationDepth (5 default), hard error by default, audit alert** |
| **A2A privilege escalation** (low-priv agent invokes high-priv agent to do something forbidden) | **Defense in depth: AgentRegistration.canInvoke (Q14b) AND AlertRoute.team (Q14c) AND ApprovalPolicy must all permit** |
| **OSS sustainability** (no commercial model means risk of single-maintainer abandonment) | **Document maintainer succession, accept community contributions actively, design for forkability, consider CNCF Sandbox if traction** |
| **OSS naming conflict** (chosen name might collide with existing project) | **Final naming decision in M0 with trademark search; placeholder names used during development** |
| **Dual-environment drift** (corporate profile breaks because only local is tested by maintainers) | **CI runs both profiles on every PR; quickstart demo uses local profile so it's exercised constantly** |
| **Memory feature creep** (pressure to build native memory subsystem) | **Decision documented: memory is integration not feature; MCPServer CRD covers extensibility; v1.1 native option pre-scoped if user demand justifies** |
| **External memory dependency rot** (if a recommended memory system gets abandoned) | **Curated list maintained in docs/integrations/memory.md; users can swap any MCP-compatible memory system without core changes** |

### All 12 strategic questions — resolved

| # | Question | Resolution |
|---|---|---|
| Q1 | Fully independent control plane? | **YES** — confirmed thesis from v4/v5 |
| Q2 | Day-1 agents in MVP | **HolmesGPT only in M1**; Hermes + OpenCode in M3 |
| Q3 | Day-1 sources beyond Prometheus | **Prometheus + Kubernetes in M1; AWS/Azure cloud in M5; VMware moved to backlog (post v1.0)** |
| Q4 | Approved LLM providers | **Corporate: Azure OpenAI (default) + GitHub Copilot (fallback). Local: Anthropic + OpenRouter + any.** Two profiles in same Helm chart. |
| Q5 | Default mode | **Centralized in M1-M2; per-NS evaluated and added in M5** |
| Q6 | OIDC IdP | **Both Azure AD and Dex supported from M1** (configurable via values). LDAP for Dex deferred. |
| Q7 | Compliance regime | **Pragmatic baseline in v1.0** (90d hot / 1y warm, hash chain, SoD, encryption everywhere). **Endurecimiento iterativo in v1.1+** when specific regime is confirmed (DORA/SOC2/etc.) |
| Q8 | OSS or internal | **Open source from day 1, Apache 2.0, agnostic, extensible** — no commercial model |
| Q9 | Number of tenants year 1 | **No fixed limit** — designed for arbitrary scale, capacity tiers documented |
| Q10 | On-call workflows in scope | **Analysis-only in v1.0**; integrate with external on-call (PagerDuty/Opsgenie webhooks), not replace |
| Q11 | Custom in-house agents | **First-class A2A as the strategic differentiator.** SDK as primary deliverable in M3. Three patterns: custom adapters, agent-to-agent, nested subagents. |
| Q12 | gRPC adoption strategy | **Option (b)**: HTTP default in M1-M2, gRPC for internal MCPs in M3-M4 |

### Sub-decisions resolved

| # | Sub-decision | Resolution |
|---|---|---|
| Q4.1 | Default LLM in corporate prod | Azure OpenAI primary, GitHub Copilot fallback |
| Q4.2 | Cluster topology for dev vs prod | Same cluster, separate namespaces (`agentic-ops-prod`, `agentic-ops-dev`) |
| Q4.3 | GitHub Copilot API approved for agents | YES — confirmed corporate-legal alongside Azure OIDC |
| Q6.1 | M1 IdP: Azure AD or Dex first | Both supported simultaneously from M1, configurable per deployment |
| Q6.2 | Dev LDAP setup | Static users / GitHub OAuth in Dex initially; LDAP integration deferred |
| Q14 | A2A authorization model | **(b) + (c) combined as defense in depth** — agent policy AND route team must permit |
| Q15 | Sub-agent budget propagation | **4-layer min**: tenant > route > parent_remaining > caller's BudgetHint |
| Q16 | Loop & fan-out protection | **Hard cap (depth 5, fan-out 3, total 20) + cycle detection + hard error default + HITL override optional** |

### Remaining items for Milestone 0 to decide

These are tactical, not strategic — they get answered in M0 / early M1 without blocking the design:

- Final project name (with trademark search)
- Final GitHub org name + domain availability
- Initial set of contributing maintainers
- Specific Azure OpenAI deployment to use (which model, which region)
- Specific GitHub OAuth app for Dex local profile

---

---

## 23. Comparison v2 / v3 / v4 / v5 / v5.1 / v5.2 / v5.3

| Dimension | v2 | v3 (kagent) | v4 | v5 | v5.1 | v5.2 | **v5.3** |
|---|---|---|---|---|---|---|---|
| Framework dependency | None | Kagent (alpha) | None | None | None | None | **None** |
| Custom agent flexibility | High | Limited | High | High | High | High + first-class A2A | **High + first-class A2A** |
| Multi-LLM | We build | Kagent | Per-agent | LLM Gateway | LLM Gateway | LLM Gateway, dual profile | **LLM Gateway, dual profile** |
| MCP transport | HTTP/SSE | HTTP/SSE | HTTP/SSE | Streamable HTTP | Pluggable | Pluggable | **Pluggable** |
| MCP Gateway | We build | Kagent | We build (thin) | + Per-tenant cred injection | + Multi-transport translator | + Multi-transport + A2A internal tool | **+ Multi-transport + A2A + memory MCPs** |
| Cost model | Implicit | Implicit | Implicit | TenantBudget CRD | TenantBudget CRD | TenantBudget + 4-layer A2A propagation | **Same** |
| HITL | Mentioned | Mentioned | Mentioned | ApprovalPolicy CRD | ApprovalPolicy CRD | ApprovalPolicy + A2A approvals | **+ Memory write approvals** |
| Secrets | Generic | Generic | Generic | Vault + ESO | Vault + ESO | Vault + ESO (corp) / K8s Secrets (local) | **Same** |
| Encryption | Mentioned | Mentioned | Mentioned | 5-layer spec | 5-layer + gRPC native mTLS | 5-layer + gRPC native mTLS | **Same** |
| Audit | Standard | Standard | Standard | DORA-grade | DORA-grade | Pragmatic baseline + DORA-grade in corporate | **Same** |
| Prompt injection | Mentioned | Mentioned | Mentioned | 7-layer defense | 7-layer defense | 7-layer + A2A privilege escalation prevention | **Same** |
| Observability | Generic OTel | Generic OTel | Generic OTel | End-to-end traces | End-to-end traces | End-to-end + investigation tree view | **Same** |
| DR | Not addressed | Not addressed | Not addressed | Explicit (lightweight) | Explicit (lightweight) | Explicit (lightweight) | **Same** |
| Adapter versioning | Not addressed | Not addressed | Not addressed | Semver + tests | Semver + tests | Semver + tests + public SDK module | **Same** |
| Agent orchestration (A2A) | Not addressed | Mentioned | Mentioned | Mentioned | Mentioned | First-class: 3 patterns, defense in depth | **Same** |
| Open source quality | Not addressed | N/A | Not addressed | Not addressed | Not addressed | Apache 2.0, public CI, SBOM, Cosign, docs site | **Same** |
| Dual environment | Not addressed | N/A | Not addressed | Not addressed | Not addressed | Profile presets in Helm chart from M1 | **Same** |
| **Memory strategy** | **Not addressed** | Not addressed | Not addressed | Not addressed | Not addressed | Not explicitly addressed | **Explicitly designed: integration via MCPServer CRD; native v1.1 candidate** |
| **Memory documentation** | **Not addressed** | N/A | Not addressed | Not addressed | Not addressed | Not addressed | **docs/integrations/memory.md with curated vendor matrix** |
| Modular delivery | High level | High level | High level | 6 milestones | 6 milestones | 6 milestones, more A2A focus | **Same as v5.2** |
| Time to MVP | ~6-8 wks | ~5-6 wks | ~6-7 wks | ~6-8 wks | ~6-8 wks | ~6-8 wks | **~6-8 wks** |
| Time to v1.0 | ~22-27 wks | ~16-21 wks | ~20-24 wks | ~22-26 wks | ~22-26 wks | ~25-30 wks | **~25-30 wks (no change; memory is doc-only)** |
| Production-ready? | Partial | Partial | Mostly | Yes | Yes + future-proof | Yes + future-proof + community-ready | **Yes + extensible memory model** |
| Strategic differentiator | None clear | "On kagent" | Independence | Production-grade | Transport-agnostic | "The only OSS where any agent can orchestrate any other agent" | **Same + clear memory extensibility story** |

**The v5.3 trade:** zero additional development time vs v5.2, in exchange for clarity about a question that would have come up in user feedback anyway: "does this thing have memory?" The answer is now documented and intentional, not accidental.

---

---

## 24. Next Steps

The design phase is **complete**. All 12 strategic questions and 8 sub-decisions are resolved. We are ready to execute.

### Immediate next steps

1. **Final customer signoff** on this v5.2 document.
2. **Milestone 0 kickoff** (2-week PoC):
   - Validate the absolute minimum end-to-end flow before committing to Phase 1
   - Decide final project name (with trademark search) and GitHub org
   - Stand up HolmesGPT + a 100-line Go webhook receiver + browser SSE viewer
   - Demo a real Prometheus alert flowing through to a screen
3. **Set up working infrastructure**:
   - Public GitHub repo (initially in draft / private until naming finalized in M0)
   - Working channel (Slack/Discord/Teams) for daily collaboration
   - Weekly checkpoint cadence with stakeholders
4. **Recruit team** for Milestones 1+:
   - 1 Go backend developer (control plane, adapters, gateways)
   - 1 React frontend developer (UI)
   - 1 platform / SRE engineer (Helm, K8s, observability, security)
   - Part-time PM / tech lead

### After Milestone 0

- Formally lock the design (or refine based on PoC learnings)
- Commit to Milestone 1 with sharper estimates
- Begin public OSS repo work in parallel with M1 development
- Final naming + branding decisions

### Decision deferred to natural moments later

These will be decided when the project reaches the right stage, not now:

- Specific compliance regime (DORA/SOC2/ISO27001) — decide before v1.1 audit hardening
- VMware MCP wrapper priority — backlog grooming after v1.0
- CNCF Sandbox application — consider after v1.0 if community traction warrants

---

*v5.3 prepared after evaluating MemPalace and similar memory systems for potential integration. Conclusion: memory is an integration concern handled via existing MCPServer extensibility, not a core feature. This keeps Leloir agnostic, lets users choose their memory vendor, and preserves the small-surface-area principle. Native cross-investigation memory is a v1.1 candidate based on real user demand, scoped at ~500 lines of Go + Postgres if built. Total Leloir strategy: build an open-source, agnostic, A2A-first agentic ops platform where any AI agent can orchestrate any other AI agent to resolve operational incidents — with unified cost tracking, audit, approvals, security, and pluggable memory. Apache 2.0 licensed, runs at home or in enterprise via dual-environment profiles, transport-agnostic MCP Gateway for ecosystem future-proofing, modular delivery so value lands every 4-6 weeks, ~25-30 weeks to v1.0.*
