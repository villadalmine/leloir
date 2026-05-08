# Leloir — Onboarding Guide

Welcome. This is for someone who is joining the project (yourself in the future, a teammate, a contributor) and needs to get productive quickly.

**Time required:** ~30 minutes if you skim, ~2 hours for full immersion.

---

## 1. The 60-second pitch

Leloir is an **open-source platform** that takes incoming alerts (from Prometheus, CloudWatch, Datadog, etc.) and routes them to AI agents (HolmesGPT, OpenCode, custom) for automated investigation and root cause analysis. It runs on Kubernetes, supports multi-tenancy, and lets agents collaborate (one agent can invoke another for specialized analysis — we call this **A2A**, agent-to-agent).

It exists because:
1. Existing tools (Robusta UI) are tied to one specific agent
2. Building a single-agent UI doesn't scale to a future where teams want their own custom agents
3. There's no open-source platform today that orchestrates multiple AI agents for incident response

The strategic differentiator is the A2A capability: an agent investigating a database incident can invoke a security specialist agent, get its analysis, integrate the result, and continue — all with unified cost tracking, audit, and approvals.

---

## 2. Mental model in three layers

```
┌──────────────────────────────────────────────────────────────┐
│  CONTROL PLANE  ←  this is what we build                     │
│  - Routing, orchestration, audit, A2A, UI, notifications     │
└──────────────────────────────────────────────────────────────┘
                             ↑
            AgentAdapter interface (5 methods, our SDK)
                             ↑
┌──────────────────────────────────────────────────────────────┐
│  AGENTS  ←  these are external (HolmesGPT, OpenCode, custom) │
│  - Each runs in its own pod, with its own LLM and MCP client │
│  - We invoke them; we don't host them                        │
└──────────────────────────────────────────────────────────────┘
                             ↓
              Tools via MCP Gateway (per-tenant scoped)
                             ↓
┌──────────────────────────────────────────────────────────────┐
│  TOOLS / SOURCES  ←  these are external too                  │
│  - Prometheus, k8s, Postgres, Loki, custom                   │
│  - Spoken to via MCP servers (HTTP/Streamable, gRPC, stdio)  │
└──────────────────────────────────────────────────────────────┘
```

The whole platform glue is **the routing logic + the AgentAdapter contract + the MCP Gateway + the audit + the UI**. Everything else (the agent, the LLM, the tool servers) is provided by the ecosystem and pluggable.

---

## 3. Key design decisions to internalize

These aren't accidents; each has a story. If you ever want to revisit, see `proposal-leloir-platform-v5.3.md` §0 "What changed" for the full evolution.

| Decision | Why |
|---|---|
| Independent control plane (no kagent) | Kagent is alpha; building on it = deprecation risk. We own our destiny. |
| Direct adapters (~150-300 lines each) instead of a unified framework | We don't know what next year's agent will look like. Translation layer is cheap; framework is expensive. |
| Each agent stays self-contained | We invoke; we don't host. They keep their own LLM, MCP, sessions. |
| MCP Gateway is mandatory between agents and tools | Per-tenant scoping, credential injection, audit, rate limiting. Bypassing = security leak. |
| LLM Gateway with cost attribution | Multi-tenant cost tracking + fallback chains + per-investigation budgets. |
| A2A is first-class, not an afterthought | This is what makes Leloir different from a single-agent UI. |
| Memory is integration-only (MCPServer CRD) | Don't lock users into one memory model. Don't carry the maintenance burden. |
| Two profiles: corporate + local | Same code, different defaults. Anyone can play at home; enterprise gets safe defaults. |
| Apache 2.0, agnostic, no commercial model | The author isn't building a product; they're building infrastructure. |

---

## 4. Your first 30 minutes — pick one

### Track A: I want to understand what we're building

1. Open `leloir-architecture.html` in a browser (5 min — visual)
2. Read `leloir-README.md` (10 min — public-facing pitch)
3. Skim `proposal-leloir-platform-v5.3.md` §1 "Executive Summary" + §2 "Reference Architecture" + §16 "Agent Orchestration & A2A" (15 min)

### Track B: I want to start coding right now

1. Read `leloir-milestone-0-plan.md` (15 min — Day 1 to Day 10)
2. Run `scripts/bootstrap.sh --check-prereqs` to see if your machine is ready (1 min)
3. Run `scripts/bootstrap.sh --extract` to unpack the SDK and control plane skeletons (1 min)
4. Open `leloir-core/cmd/leloir-controlplane/main.go` and trace through the boot flow (15 min)

### Track C: I want to understand the AgentAdapter contract (because I'll write an adapter)

1. Read `leloir-agentadapter-sdk-spec-v1.md` §1-§3 (15 min — purpose, contract, types)
2. Open `leloir-sdk/examples/holmesgpt/adapter.go` (15 min — see a real adapter)

---

## 5. Code organization

We have 3 logical components, in 3 separate packages/repos:

### `leloir/leloir` (umbrella) — `leloir-core`

The Go binaries that make up the control plane:
- `leloir-controlplane` — main service (HTTP API, routing, orchestration, audit)
- `leloir-mcp-gateway` — multi-transport MCP translator
- `leloir-webhook-receiver` — Alertmanager webhook ingester (optional separate process)

Lives in `assets/leloir-controlplane-skeleton.zip`.

### `leloir/sdk` — the AgentAdapter SDK

The Go module that adapter authors import to build adapters:
- `adapter/` — the 5-method interface + types + helpers
- `conformance/` — test suite that adapters use to verify they satisfy the contract
- `examples/minimal/` — smallest possible working adapter
- `examples/holmesgpt/` — reference adapter for HolmesGPT

Lives in `assets/leloir-sdk-v1-skeleton.zip`.

### `leloir/leloir/crds` — the Kubernetes CRDs

The 8 CRDs that configure the platform:
- `Tenant`, `AgentRegistration`, `AlertRoute`, `MCPServer`, `NotificationChannel`, `ApprovalPolicy`, `TenantBudget`, `SkillSource`

Plus Helm value profiles for `corporate` and `local` deployments.

Lives in `assets/leloir-crds-v1.zip`.

---

## 6. The 5-method AgentAdapter contract (memorize this)

```go
type AgentAdapter interface {
    Identity() AgentIdentity
    Configure(ctx context.Context, config Config) error
    HealthCheck(ctx context.Context) error
    Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error)
    Shutdown(ctx context.Context) error
}
```

That's the entire contract. Adapter authors implement these 5 methods. The control plane calls them. Events stream out as a channel of typed messages (Thought, ToolCallRequest, LLMCall, SubAgentRequest, Answer, Complete, etc.).

If you understand these 5 methods, you understand 80% of the platform.

---

## 7. Common questions & answers

**Q: Why Go for the control plane?**
A: Kubernetes-native ecosystem, single binary, strong concurrency for streaming events, simple to write adapters in. Same language as the SDK.

**Q: Why React for the UI?**
A: Standard, polished, lots of available components, frontend devs know it. Not interesting.

**Q: Can I write an adapter in Python?**
A: Not in v1 — the SDK is Go in-process. We plan a sidecar gRPC option for SDK v2 (M5+ on the platform side) that allows Python/Rust/Node adapters. For now, write a thin Go wrapper that shells out.

**Q: Does this depend on cloud services?**
A: No. Runs on any Kubernetes cluster. The corporate profile uses Vault + ESO + S3 Object Lock; the local profile uses K8s Secrets + nothing fancy.

**Q: Who pays for LLM costs?**
A: The operator. The platform tracks costs per tenant/investigation/agent and enforces budgets, but doesn't pay anything itself.

**Q: How does memory work?**
A: It doesn't, in v1.0 (by design). Each agent handles its own session memory. If you want cross-investigation memory, register an external memory MCP server (MemPalace, Zep, Mem0) and the agents will use it as a tool. We may add a tiny native cross-investigation memory feature in v1.1 if users ask for it.

**Q: How does HITL work?**
A: `ApprovalPolicy` CRD specifies which tool calls (or A2A invocations) need human approval. When triggered, an event goes to a Slack/Teams channel; humans click approve/deny; the agent continues. Implemented in M4.

**Q: How does A2A actually work?**
A: An agent emits an `EventSubAgentRequest` with target agent + context + ask + budget hint. The control plane validates 4 layers (canInvoke, route team, approval policy, budget min). If allowed, it invokes the target agent as a sub-investigation. The result comes back as `EventSubAgentResponse`. See §16 of the v5.3 doc.

**Q: What's the smallest unit of work I can ship and demo?**
A: M0 — 2 weeks, end-to-end alert flow, ~200 lines of Go + 1 HTML file. See `leloir-milestone-0-plan.md`.

---

## 8. People to know

- **Project initiator:** [you / your name]
- **Inspiration:** Luis Federico Leloir (Argentine Nobel Laureate in Chemistry, 1970)
- **External projects we depend on or integrate with:**
  - HolmesGPT (CNCF Sandbox) — first adapter target
  - LiteLLM (Apache 2.0) — LLM Gateway implementation
  - Vault — secrets backend (corporate)
  - cert-manager — TLS automation
  - Prometheus / Alertmanager — primary alert source
- **Inspiration (read these later):**
  - Robusta UI — the closest existing tool (single-agent)
  - kagent — alpha framework we considered building on (rejected)
  - Pi (pi.dev) — minimal coding agent harness (different domain, but interesting design philosophy)
  - Letta, Zep, MemPalace, Mem0 — memory systems we DON'T integrate (memory is opt-in via MCP)

---

## 9. Where you might get stuck

| Symptom | Likely cause | Action |
|---|---|---|
| "I don't understand the architecture" | Skipped the v5.3 design doc | Read §1, §2, §16 of `proposal-leloir-platform-v5.3.md` |
| "I don't know what to code first" | Skipped the M0 plan | Read `leloir-milestone-0-plan.md` Day 4 |
| "What's the shape of an adapter?" | Skipped the SDK spec | Read `leloir-agentadapter-sdk-spec-v1.md` §2-§3 + see the holmesgpt example |
| "Should this be a feature or an integration?" | Look at the memory decision (§19 of v5.3) for the canonical answer pattern | Pluggable via MCPServer CRD wins almost always |
| "How does multi-tenancy work?" | Read `proposal-leloir-platform-v5.3.md` §3 + §11 | TenantID flows through context; MCP Gateway scopes everything |

---

## 10. When in doubt, default to these principles

1. **Be agnostic.** If a decision locks users into a vendor, find another way.
2. **Be small.** If the SDK adds a method, you'd better have a strong reason. Same for CRDs, events, error codes.
3. **Be testable.** Conformance suite is the bar. If a behavior isn't tested, it isn't real.
4. **Be observable.** Every event in audit. Every span in OTel. Cost attributed everywhere.
5. **Be safe by default.** Corporate profile must be production-ready out of the box. Local profile must be frictionless.

---

*Welcome aboard. The hardest part of this project isn't the code — it's holding the line on these principles when someone asks for a feature that violates them. The good news: 80% of design decisions have already been made and documented. Now we build.*
