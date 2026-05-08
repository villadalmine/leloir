# Leloir — Project Status

**Date of this snapshot:** April 28, 2026
**Phase:** End of design phase. Ready to start Milestone 0 (PoC).

---

## ✅ What's decided

### Strategic (12/12 questions resolved)

| # | Decision | Resolved as |
|---|---|---|
| Q1 | Architecture | Independent control plane (no kagent dependency) |
| Q2 | Day-1 agents | HolmesGPT only in M1 |
| Q3 | Day-1 sources | Prometheus + Kubernetes (M1); AWS/Azure (M5); VMware backlog |
| Q4 | LLM providers | Corporate: Azure OpenAI + GitHub Copilot. Local: Anthropic + OpenRouter |
| Q5 | Default mode | Centralized in M1-M2; per-namespace evaluated in M5 |
| Q6 | OIDC IdP | Both Azure AD and Dex from M1 |
| Q7 | Compliance | Pragmatic baseline for v1.0; harden iteratively |
| Q8 | OSS or internal | Open source, Apache 2.0, agnostic, no commercial model |
| Q9 | Tenant scale | No fixed limit, designed for arbitrary scale |
| Q10 | On-call workflows | Analysis-only in v1.0; integrate with external on-call |
| Q11 | Custom agents | First-class A2A; SDK is strategic deliverable in M3 |
| Q12 | gRPC adoption | HTTP default in M1-M2; gRPC for internal MCPs in M3-M4 |

### Architecture sub-decisions

- **A2A authorization (Q14):** defense in depth — `AgentRegistration.canInvoke` (b) AND `AlertRoute.team` (c) must both permit
- **A2A budget propagation (Q15):** 4-layer min — tenant > route > parent_remaining > caller_hint
- **A2A loop prevention (Q16):** depth=5, fan-out=3, total=20 + cycle detection + hard error default
- **Memory strategy:** integration via MCPServer CRD, no native memory subsystem; v1.1 candidate for cross-investigation memory if user demand justifies

### Operational

- **Project name:** Leloir ✅
- **License:** Apache 2.0 ✅
- **Target stack:** Go (backend) + React (frontend) + Postgres ✅
- **Deployment:** Helm chart with `profile: corporate | local | experimental` ✅

---

## 🔴 What's pending (must resolve before or during M0)

| # | Item | When | How |
|---|---|---|---|
| P1 | Verify `github.com/leloir` org availability | Day 1 of M0 | `gh api orgs/leloir` (404 = available); fallback `leloir-platform` |
| P2 | Verify `leloir.dev` domain availability | Day 1 of M0 | Google Domains / Cloudflare Registrar |
| P3 | Trademark search "Leloir" in IT classes | Day 1 of M0 | tmsearch.uspto.gov + euipo.europa.eu |
| P4 | Azure OpenAI access for corporate testing | Start request Day 1 (can take weeks) | Internal IT ticket; meanwhile use GitHub Copilot via `gh` token |
| P5 | Team formed for M1 | By Day 10 of M0 | 1 Go backend + 1 React frontend + 1 platform/SRE + part-time PM |
| P6 | Stakeholder signoff on v5.3 design | Before/during M0 | Send `proposal-leloir-platform-v5.3.md` + executive summary |

---

## 🟡 What's deferred (decide later, won't block M0-M1)

| # | Item | When to decide | Notes |
|---|---|---|---|
| D1 | Specific compliance regime (DORA/SOC2/HIPAA) | Before v1.1 audit hardening | Affects WORM Object Lock retention; default to SOC2-ready in v1.0 |
| D2 | VMware MCP wrapper priority | Backlog grooming after v1.0 | Currently moved out of v1.0 scope |
| D3 | CNCF Sandbox application | Post-v1.0 if community traction | Optional; not part of v1.0 plan |
| D4 | Native cross-investigation memory feature | Post-v1.0 if user demand | Designed in v5.3 §19.6, ~500 lines if built |
| D5 | Sidecar adapters (non-Go) for SDK | SDK v2 / platform M5+ | Currently Go-only adapters |
| D6 | Public KubeCon-style talk submission | Post-v1.0 | Marketing decision |

---

## 📅 What's next (immediate)

### This week — preparation

1. **Read** `docs/proposal-leloir-platform-v5.3.md` end-to-end if you haven't (or skim the §0 "What changed" sections to understand the journey)
2. **Send** the design doc + executive summary to stakeholders for formal signoff (P6)
3. **Start** the Azure OpenAI access request (P4 — can take weeks)
4. **Identify** the team for M1 (P5)

### Next week — M0 Day 1

1. Run `scripts/bootstrap.sh --check-naming` to verify availability of:
   - GitHub org `leloir`
   - Domains `leloir.dev`, `leloir.io`
   - Trademark search
2. If all pass: register everything (org, domains, Slack workspace `leloir.slack.com`)
3. Run `scripts/bootstrap.sh --extract --init-git` to set up the working repo
4. Set up kind cluster + Prometheus + HolmesGPT (Day 1-3 of M0 plan)

### Weeks 1-2 — M0 execution

Follow the day-by-day plan in `docs/leloir-milestone-0-plan.md`:
- Days 1-3: naming + cluster + sources + Holmes installed
- Days 4-6: webhook receiver + browser viewer + notifications
- Days 7-8: polish + stress tests + cost measurement
- Days 9-10: CONTRACT.md + demo + retro + M1 sharpening

### After M0

If M0 succeeds (3 successful demos in a row + stakeholder signoff): **start M1** with the production-shape MVP (~5-6 weeks).

If M0 surfaces problems: pause, document in `LEARNINGS.md`, adjust v5.3 design where needed, re-plan.

---

## 🚨 Hard gates before starting M1

All of these must be true:

- [ ] M0 end-to-end demo works reliably (3 successful demos in a row)
- [ ] Project name finalized; GitHub org + domain + Slack registered
- [ ] CONTRACT.md written documenting the Holmes API integration
- [ ] M0 cost (in time and surprises) within ~30% of estimate
- [ ] Stakeholder signoff on v5.3 design + M1 commit
- [ ] Team identified for M1 (or commitment to keep going solo)

If any gate fails, **pause before M1 and resolve** — that's exactly why M0 exists.

---

## 📊 Master roadmap snapshot

| Milestone | Duration | Status | Key deliverable |
|---|---|---|---|
| **M0 — PoC** | 2 weeks | ⬜ Ready to start | End-to-end demo: alert → Holmes → Teams |
| **M1 — Production-shape MVP** | 5-6 weeks | ⬜ Planned | Single-tenant production-quality system + public OSS repo |
| **M2 — Multi-tenancy** | 4-5 weeks | ⬜ Planned | OIDC, RBAC, audit, MCP Gateway scoping, Skills v1 |
| **M3 — Cost + SDK + A2A foundations** | 5-6 weeks | ⬜ Planned | LLM Gateway, TenantBudget, public Go SDK, 3 adapters |
| **M4 — Safety + A2A v1** | 5-6 weeks | ⬜ Planned | HITL, Vault, prompt injection defenses, A2A Pattern B |
| **M5 — Ecosystem + A2A polish** | 5-6 weeks | ⬜ Planned | Claude Code adapter, cloud sources, tree view UI, per-NS mode |
| **M6 — Hardening + v1.0** | 3-4 weeks | ⬜ Planned | Security review, OSS docs polish, public v1.0 release |

**Total to v1.0:** ~25-30 weeks. **Modular delivery:** value lands every 4-6 weeks.

---

*This file is a living document. Update it after each milestone closes, when decisions change, or when a pending item resolves.*
