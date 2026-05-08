# HolmesGPT Adapter (Reference Implementation)

This is the canonical reference adapter showing how to wrap an existing HTTP-based AI agent with the Leloir `AgentAdapter` contract.

**Reference:** [HolmesGPT on GitHub](https://github.com/HolmesGPT/holmesgpt)

## What this adapter does

1. Receives `InvestigateRequest` from Leloir control plane
2. Builds an OpenAI-style prompt from the alert + skills, wrapping untrusted content for prompt injection defense
3. POSTs to HolmesGPT's `/api/chat` endpoint with `stream: true`
4. Parses Holmes's SSE event stream
5. Translates each Holmes event into the corresponding Leloir `Event` type
6. Tracks budget and emits `EventBudgetWarning` at 50/80/95% thresholds
7. Sends `EventComplete` with the right `Outcome` when done

## File overview

| File | Purpose |
|---|---|
| `adapter.go` | The `AgentAdapter` interface implementation (Identity, Configure, HealthCheck, Investigate, Shutdown) |
| `client.go` | HTTP client + SSE parser |
| `events.go` | Translation layer between Holmes events and Leloir events |

Total: ~400 lines. This is representative of what a real adapter looks like.

## Usage

```go
import "github.com/leloir/sdk/examples/holmesgpt"

a := holmesgpt.New()
// Register a as an AgentRegistration in your Leloir cluster
```

## Required CustomConfig

The adapter expects this in `AgentRegistration.spec.customConfig`:

```yaml
customConfig:
  holmes:
    apiBaseURL: "http://holmes.holmes.svc.cluster.local"
```

## Running conformance tests

```bash
cd examples/holmesgpt
go test -v
```

Note: full conformance tests require a real Holmes instance. To run only the
contract tests that don't need network access, use `-short`:

```bash
go test -short -v
```

## Patterns demonstrated

This adapter is the canonical reference for:

- **Channel discipline:** `defer close(ch)` and `recoverPanic(ch, ...)` ensure
  the channel always closes even on internal errors
- **Context cancellation:** check `ctx.Done()` between every event
- **Budget tracking:** use `adapter.BudgetTracker` to record costs and check
  exhaustion
- **Threshold warnings:** emit `EventBudgetWarning` at 50%/80%/95% so the UI
  can show a meter
- **Error handling:** distinguish recoverable vs fatal errors
- **Prompt injection defense:** wrap untrusted alert content in `<untrusted-content>`
  markers
- **Streaming:** translate vendor SSE format into Leloir's standard Event stream
