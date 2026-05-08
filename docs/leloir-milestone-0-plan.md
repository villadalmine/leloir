# Milestone 0 — Leloir Bootstrap + 2-Week PoC Plan

**Project name:** **Leloir** — after Luis Federico Leloir, Argentine Nobel Laureate in Chemistry 1970, who unraveled the metabolism of sugars. A fitting metaphor for an agent that figures out the metabolism of incidents.

**Tagline:** *"The agentic incident analysis platform — like a Nobel-winning chemist for your stack."*

**Goal of M0:** validate the absolute minimum end-to-end flow before committing to M1+, AND verify availability of all Leloir-named infrastructure (GitHub org, domain, etc.).

**Duration:** 2 weeks
**Owner:** primary developer (you) + part-time review

---

## PART 1 — Leloir Naming: Verification & Setup

The naming decision is **locked: Leloir**. M0 just needs to verify availability and reserve infrastructure.

### Why Leloir (recap for project history / README)

- **Symbolic fit:** Luis Federico Leloir won the Nobel for discovering how the body's metabolism of sugars works — figuring out the *root cause* of complex biochemical processes. Perfect metaphor for incident root cause analysis.
- **Mentorship lineage:** Leloir was Houssay's disciple. Encodes the A2A pattern (agents learning from agent specialists) and the open-source ethos (knowledge passed forward).
- **Disponibility:** The only known GitHub presence is `BibliotecaLeloir`, an institutional library account with 3 academic repos. Not a conflict.
- **Pronunciation:** /le-loir/ — works in Spanish, English, French, German.
- **Length:** 6 letters. Memorable. Searchable.
- **No trademark:** family name, not a registered product mark. Academic institutions bear his name but no commercial software does.

### Verification & reservation checklist (Day 1)

For each of these, verify availability and reserve if available:

- [ ] **GitHub org:** `leloir` (preferred) → fallback `leloir-platform` → fallback `leloirhq`
- [ ] **Domain `.dev`:** `leloir.dev` (preferred) — modern .dev TLD fits open source
- [ ] **Domain `.io`:** `leloir.io` — secondary, classic OSS TLD
- [ ] **Domain `.ai`:** `leloir.ai` — likely premium-priced, check anyway
- [ ] **Trademark search USPTO:** search "Leloir" in software classes 9 + 42 at tmsearch.uspto.gov
- [ ] **Trademark search EUIPO:** same at euipo.europa.eu/eSearch
- [ ] **PyPI:** verify no package named leloir
- [ ] **NPM:** npm search leloir
- [ ] **Helm Hub / Artifact Hub:** search leloir charts
- [ ] **Slack workspace:** reserve leloir.slack.com (free tier)
- [ ] **Docker Hub:** verify leloir-platform org name available
- [ ] **GitHub Container Registry (ghcr.io):** automatic with GitHub org
- [ ] **Google search:** verify low-noise results for leloir + software/github

### Likely outcome

Given the name is a deceased Argentine scientist's family name, available is the strong default. **Estimated probability: 90% that leloir.dev + GitHub leloir org are both available today.**

### Fallback plan

If `leloir` GitHub org is taken (unlikely):
1. Try `leloir-platform`
2. Try `leloirhq`
3. Try `leloir-io`

If `leloir.dev` is taken:
1. Try `leloir.io`
2. Try `useleloir.dev`
3. Try `leloir-platform.dev`

The product name remains "Leloir" regardless. Infra naming is operational only.

### Branding considerations (for later, not blocking M0)

- **Logo concept:** simple molecular/sugar-chain icon (referencing Leloir's research) or a tree (referencing A2A and the Houssay→Leloir mentorship lineage)
- **Color palette:** azure blue + clean white (Argentine flag homage)
- **Voice:** technical but approachable, with subtle Argentine cultural pride
- **README header:** include a short biographical mention of Luis Federico Leloir with link to his Nobel page (https://www.nobelprize.org/prizes/chemistry/1970/leloir/) — gives the project a story and educational hook

---

## PART 2 — Phase 0 Detailed Plan (10 working days)

### Goals (success criteria for the PoC)

By end of day 10, you should be able to demonstrate:

1. ✅ **Real Prometheus alert** fires from a sandbox cluster
2. ✅ **HolmesGPT** (deployed standalone via official Helm) receives the alert context
3. ✅ **Holmes investigates** using k8s + Prometheus tools (real MCP / native tools)
4. ✅ **Stream of investigation** appears in a browser tab in real time (SSE)
5. ✅ **Final answer + cost** displayed
6. ✅ **One notification** sent to Teams (or Telegram, or both)
7. ✅ All of the above runs from a single `make demo` or equivalent
8. ✅ The codebase is small, throwaway-quality, but **documents the contract** between control plane and HolmesGPT

If any of these fail or are surprisingly hard, that's exactly what M0 is for: discover before committing to M1.

### Out of scope for M0 (do NOT build these)

- Multi-tenancy (single user, single tenant only)
- Authentication (no OIDC, no login, hardcoded user)
- Persistence beyond in-memory (no Postgres, no audit log)
- Adapter abstraction (talk directly to Holmes API, no `AgentAdapter` interface yet)
- LLM Gateway (Holmes uses its own configured provider directly)
- MCP Gateway (use whatever MCPs Holmes's Helm chart deploys)
- Multi-agent / A2A (single agent only)
- HITL (no approvals)
- Skills system (use Holmes's built-in runbooks)
- Helm chart (raw kubectl + a docker-compose for local UI is fine)
- React UI proper (a single static HTML + vanilla JS is enough)
- Testing infrastructure (manual testing fine)
- Multiple environments
- Anything mentioned in the v5.2 doc beyond this list

**The goal is to PROVE THE FLOW WORKS, not to build the platform.** Throwaway code is the deliverable.

### Day-by-day plan

#### Day 1 — Naming + Cluster setup

**Morning:**
- Check availability of top 5 candidate names (Part 1 above)
- Pick name (or commit to picking by EOD day 3)
- Create placeholder GitHub repo (private initially): `<name>-platform`

**Afternoon:**
- Stand up local Kubernetes cluster:
  ```bash
  kind create cluster --name leloir-poc --config=kind-config.yaml
  ```
- Verify kubectl works
- Install Helm CLI

**Acceptance:**
- ✅ At least 3 names verified available, decision deferred to day 3 max
- ✅ `kubectl get nodes` shows running cluster
- ✅ Empty git repo with README placeholder

#### Day 2 — Install Prometheus + sample workload

**Morning:**
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prom prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace
```

**Afternoon:**
- Deploy a sample app that you can break to trigger an alert:
  - Simple Flask/Go HTTP service
  - Configure as scrape target
  - Add a PrometheusRule that fires when, e.g., HTTP errors > threshold
- Verify Alertmanager UI shows the alert when you break the app

**Acceptance:**
- ✅ Prometheus + Alertmanager + Grafana all running
- ✅ A real alert visible in Alertmanager UI when you intentionally break the app
- ✅ Alertmanager configured to webhook (target TBD on day 4)

#### Day 3 — Install HolmesGPT, basic test

**Morning:**
- Install HolmesGPT via official Helm chart:
  ```bash
  helm repo add robusta https://robusta-charts.storage.googleapis.com
  helm install holmes robusta/holmes --namespace holmes --create-namespace \
    --set additionalEnvVars[0].name=AZURE_API_KEY \
    --set additionalEnvVars[0].value=$AZURE_API_KEY \
    --set additionalEnvVars[1].name=AZURE_API_BASE \
    --set additionalEnvVars[1].value=$AZURE_API_BASE \
    --set llm=azure
  ```
  (use Azure OpenAI per Q4.1 — your corporate setup)

**Afternoon:**
- Test Holmes via CLI / port-forward to its HTTP API:
  ```bash
  kubectl port-forward -n holmes svc/holmes 8080:80
  curl -X POST http://localhost:8080/api/chat -d '{"ask": "what pods are unhealthy?"}'
  ```
- Verify Holmes can reach the cluster, calls k8s tools, returns analysis
- **Lock naming decision today** if not done

**Acceptance:**
- ✅ Holmes deployed and healthy
- ✅ Manual API call to Holmes returns sensible AI-generated analysis
- ✅ Project name FINAL, GitHub org created (private still ok), Slack workspace registered

#### Day 4 — Webhook receiver (the throwaway 100-line Go service)

This is the **core deliverable** — proving you can wire Alertmanager → your code → Holmes.

**Morning:**
Write a single Go file `cmd/poc/main.go`:

```go
// Receives Alertmanager webhooks
// Calls Holmes API
// Holds the SSE connection open
// Streams events to a single browser tab via SSE
// No auth, no TLS, no DB, no nothing. 100-200 lines max.

package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    // ...
)

type AlertmanagerPayload struct {
    Alerts []Alert `json:"alerts"`
}

type Alert struct {
    Status      string            `json:"status"`
    Labels      map[string]string `json:"labels"`
    Annotations map[string]string `json:"annotations"`
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Decode Alertmanager payload
    // 2. For each alert, build a prompt for Holmes
    // 3. POST to Holmes API
    // 4. Read SSE stream from Holmes
    // 5. Forward each chunk to all connected browser SSE clients
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
    // Standard SSE handler
    // Subscribe to internal event broadcast
}

func main() {
    http.HandleFunc("/webhook", handleWebhook)
    http.HandleFunc("/events", handleSSE)
    http.HandleFunc("/", serveStaticHTML)
    log.Fatal(http.ListenAndServe(":3000", nil))
}
```

**Afternoon:**
- Configure Alertmanager to webhook to your service:
  ```yaml
  receivers:
    - name: leloir-poc
      webhook_configs:
        - url: http://host.docker.internal:3000/webhook
  ```
- Test by triggering the alert from day 2

**Acceptance:**
- ✅ Webhook endpoint receives real Alertmanager payloads (logged to console)
- ✅ Alert payload is forwarded to Holmes (verify in Holmes logs)

#### Day 5 — Browser viewer (single-page HTML)

**Morning:**
Write `static/index.html` — single page, no framework:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Leloir PoC</title>
  <style>
    body { font-family: sans-serif; padding: 20px; max-width: 900px; }
    .event { padding: 8px; border-bottom: 1px solid #eee; }
    .thought { color: #666; font-style: italic; }
    .tool { color: #0a7; }
    .answer { color: #000; font-weight: bold; }
  </style>
</head>
<body>
  <h1>Leloir PoC — Live Investigation Stream</h1>
  <div id="events"></div>
  <script>
    const evtSource = new EventSource('/events');
    evtSource.onmessage = (e) => {
      const data = JSON.parse(e.data);
      const div = document.createElement('div');
      div.className = 'event ' + (data.type || '');
      div.textContent = `[${data.type}] ${data.text}`;
      document.getElementById('events').appendChild(div);
    };
  </script>
</body>
</html>
```

**Afternoon:**
- End-to-end test: trigger alert → see it stream in browser
- Iterate on the SSE event format until it's readable

**Acceptance:**
- ✅ Open `http://localhost:3000` in browser, see live investigation as it happens
- ✅ Holmes "thinking" steps, tool calls, and final answer all visible

#### Day 6 — Notification to Teams (or Telegram)

Pick whichever is easier to set up for you. Teams webhook is usually 5 minutes.

**Morning:**
- Create an Incoming Webhook in Teams (or Bot Token in Telegram)
- Add to environment variable `TEAMS_WEBHOOK_URL`

**Afternoon:**
- Extend the Go service: when Holmes returns final answer, also POST to Teams webhook
- Format as Adaptive Card (Teams) or Markdown (Telegram)

**Acceptance:**
- ✅ When Holmes finishes investigation, a notification appears in Teams/Telegram
- ✅ Notification contains: alert summary, Holmes's root cause, recommended action

#### Day 7 — Polish + reproducibility

This is the day to make it not just "works on my machine" but reproducible.

- Write a `Makefile`:
  ```makefile
  .PHONY: setup demo clean

  setup:
      kind create cluster --name leloir-poc --config=kind.yaml
      helm install prom ...
      helm install holmes ...

  demo:
      kubectl apply -f testdata/sample-app.yaml
      kubectl apply -f testdata/break-app.yaml  # triggers alert
      go run cmd/poc/main.go

  clean:
      kind delete cluster --name leloir-poc
  ```
- Document required environment variables in a `.env.example`
- Test on a second machine if possible (or fresh VM)

**Acceptance:**
- ✅ `make setup && make demo` works from a clean clone of the repo
- ✅ README documents the demo with screenshots / GIF

#### Day 8 — Stress test + edge cases

Find what breaks. M0 is the cheapest place to discover problems.

Test these scenarios:
- Multiple alerts firing simultaneously (does the SSE stream handle parallelism?)
- Holmes API errors (timeout, rate limit) — what happens?
- Holmes returns very long response (truncation? memory?)
- Browser disconnects mid-stream — does the Go service leak goroutines?
- Alertmanager retries the webhook — duplicate processing?
- Test with both Azure OpenAI **and** GitHub Copilot to verify both work
- What does cost actually look like for one investigation? (Token usage, USD)

**Document each finding in a `LEARNINGS.md` file.** This becomes input to M1 design.

**Acceptance:**
- ✅ At least 5 scenarios tested and documented
- ✅ At least 3 surprises / issues identified for M1 to address
- ✅ Cost per investigation measured (e.g., "$0.02 per investigation with GPT-4o")

#### Day 9 — Document the contract

The PoC code is throwaway, but the **contract between our control plane and Holmes** is the deliverable that informs M1.

Write `CONTRACT.md`:
- What HTTP endpoints does Holmes expose?
- What's the request/response schema?
- How is streaming done (SSE format, event types)?
- How do we configure Holmes (env vars, ConfigMap, what)?
- What MCP servers does Holmes ship with by default? Which did we use?
- What's the LLM cost model (tokens in/out, which model)?
- What's the latency profile (median investigation time, p99)?
- What auth/security does Holmes API have? (Probably none = problem to solve in M1)

This document is what makes the AgentAdapter v1 interface in M1 well-grounded instead of speculative.

**Acceptance:**
- ✅ `CONTRACT.md` written, reviewed, makes M1 adapter design obvious

#### Day 10 — Demo + retro + M1 sharpening

**Morning:** Live demo to stakeholders (you + customer + tech lead if applicable).

Demo script:
1. "Here's a fresh kind cluster, 30 seconds ago empty."
2. `make setup` — show installation in real time
3. `make demo` — break the sample app
4. Open browser tab — watch alert flow in
5. Watch Holmes investigate live in the browser
6. Show Teams notification arriving
7. Total wall clock time: ideally <5 minutes from "fresh cluster" to "notification"

**Afternoon:** Retrospective + M1 estimate refinement

- What was easier than expected?
- What was harder?
- What should change in M1 design?
- Update timeline estimate for M1 with actual data
- Decide if any v5.2 design assumptions need revision

**Acceptance:**
- ✅ Live demo successful
- ✅ Stakeholders signoff on continuing to M1
- ✅ M1 plan refined with actual data from M0
- ✅ Updated v5.2 if needed (probably as v5.3 with M0 learnings appendix)

### Deliverables of M0 (final checklist)

By end of day 10, the repo contains:

- [ ] `README.md` with name, mission, quickstart
- [ ] `Makefile` with `setup`, `demo`, `clean`
- [ ] `cmd/poc/main.go` — the throwaway Go service (~200 lines)
- [ ] `static/index.html` — the throwaway browser viewer
- [ ] `kind.yaml` — kind cluster config
- [ ] `testdata/` — sample app + breakage YAMLs
- [ ] `helm/values.yaml` — values used for Holmes installation
- [ ] `.env.example` — required env vars (Azure key, Teams webhook)
- [ ] `CONTRACT.md` — the Holmes-to-platform contract
- [ ] `LEARNINGS.md` — discoveries during M0
- [ ] `M1-PLAN.md` — refined M1 plan with M0 data

Plus, in your project tracker:

- [ ] Project name FINAL with GitHub org / Slack / domains registered
- [ ] LICENSE (Apache 2.0) committed to repo
- [ ] First public README (when you're ready to make repo public)

### Milestone exit criteria — go/no-go for M1

Hard gates that must be true before starting M1:

1. ✅ End-to-end demo works reliably (3 successful demos in a row)
2. ✅ Project name is locked, infrastructure (org, domain, etc.) reserved
3. ✅ Contract with Holmes documented
4. ✅ M0 cost (in time and surprises) was within ~30% of estimate
5. ✅ Stakeholder signoff
6. ✅ Team identified for M1 (or you committed to keep going solo)

If any of these fail, **pause before M1 and resolve**. M0 exists exactly to surface these problems before they cost months.

---

## Risk register specific to M0

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| All preferred names are taken | Medium | Low | Have 5+ candidates, fallback to `<theme>-ops` suffix |
| Azure OpenAI access slow to provision | High | High | Start access request day 1; use OpenAI direct as fallback for PoC only (not for M1) |
| Holmes API has undocumented quirks | Medium | Medium | M0 is exactly to surface these; document in CONTRACT.md |
| kind cluster too small for Holmes | Low | Low | kind supports multi-node; allocate 4 CPU / 8GB to Docker |
| Alertmanager webhook config tricky | Low | Low | Well-documented; example in plan |
| Demo unreliable for live presentation | Medium | High | Practice 3 times day 9-10 before live demo; have video backup |
| Teams webhook blocked by corporate firewall | Medium | Low | Telegram as fallback; or just show notification in browser |

---

## What M0 is explicitly NOT

It is NOT:
- A scoped-down version of M1
- A throwaway demo for marketing
- Production-quality code
- The right time to optimize architecture
- The right time to add features

It IS:
- A learning vehicle to validate v5.2 assumptions
- A risk reduction step before committing to ~25 weeks of work
- A way to discover unknowns about Holmes / Prometheus / Alertmanager integration
- A naming decision deadline
- A signal: if M0 is hard, M1 will be much harder

---

*If M0 succeeds: clear runway to M1 with sharp estimates, validated contract, name locked, infrastructure ready.*
*If M0 surfaces big problems: better to know now than at week 12.*
