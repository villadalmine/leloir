# Leloir — AgentAdapter SDK Technical Specification

**Project:** Leloir (Agentic Incident Analysis Platform)
**Document:** AgentAdapter SDK v1 — Technical Specification
**Status:** Draft for review · informs M1-M3 implementation
**Version:** 1.0
**Date:** April 19, 2026
**Audience:** developers building adapters (in-tree or out-of-tree), Leloir core maintainers

---

## Table of Contents

1. Purpose & Design Principles
2. The Contract (Go interface)
3. Type Definitions
4. Lifecycle & State Machine
5. Streaming Protocol (Event types)
6. Error Model
7. Sub-Agent Invocation (A2A)
8. Budget & Cost Model
9. Tool Access Through MCP Gateway
10. Configuration & Secrets
11. Observability (OpenTelemetry contract)
12. Testing — Conformance Suite
13. Versioning Policy & Backward Compatibility
14. Reference Implementation Walkthrough — HolmesGPT
15. Reference Implementation Walkthrough — Custom Agent (the cookbook)
16. Anti-patterns & Common Mistakes
17. FAQ

---

## 1. Purpose & Design Principles

### 1.1 Purpose

The `AgentAdapter` interface is **the only contract** between Leloir's control plane and any AI agent. Implementing it makes any agent — open source or proprietary, Go or Python or Rust, in-process or sandboxed — a first-class citizen of the platform.

### 1.2 Why a thin contract matters

The SDK is deliberately small (5 methods + supporting types). Every piece of complexity that doesn't belong in the contract is pushed elsewhere:

| Concern | Where it lives |
|---|---|
| LLM provider connection | Inside the agent (or LLM Gateway routes) |
| Conversation state | Inside the agent |
| Tool execution | MCP Gateway (agent only requests, never executes) |
| Skill loading | Inside the agent (filesystem mount) |
| Budget enforcement | Leloir control plane (the adapter just receives the limit) |
| Approval flows (HITL) | Leloir control plane + MCP Gateway |
| Audit logging | Leloir control plane (adapter emits events, doesn't log) |
| Authentication | Leloir control plane (adapter receives identity context) |
| Multi-tenancy | Leloir control plane (adapter receives tenant context) |

**Principle:** if a concern can be solved without the adapter knowing about it, it MUST be solved without the adapter knowing about it. Less knowledge in the adapter = less coupling = adapter authors have less to learn.

### 1.3 Design principles

1. **Minimal surface.** Five methods, no more. Adding a method is a major version bump.
2. **Streaming-first.** Investigations are inherently streaming (LLM outputs, tool calls). The contract reflects that.
3. **No business logic in the adapter.** Adapters translate; they don't decide policy.
4. **Errors as values.** No panics, no exceptions through the boundary. Every failure mode is enumerated.
5. **Context-driven cancellation.** Standard Go `context.Context` everywhere. When the platform cancels, the adapter cancels its agent within 5 seconds.
6. **Observable.** OpenTelemetry context propagated; spans created by adapter are nested under platform spans automatically.
7. **Testable.** Conformance suite published as Go module. Any adapter can verify compliance with `go test`.
8. **Non-Go agents welcome via sidecar.** Native Go is the in-process path; gRPC sidecar is the path for Python/Rust/Node agents (M5+).

---

## 2. The Contract (Go interface)

```go
// Package adapter defines the AgentAdapter interface that all Leloir agents implement.
package adapter

import (
    "context"
    "time"
)

// AgentAdapter is the single contract between Leloir and any AI agent.
// Implementations may be in-process Go (recommended) or sidecar gRPC (M5+).
type AgentAdapter interface {
    // Identity returns metadata about the agent. Called once at registration.
    // Must be deterministic and side-effect-free.
    Identity() AgentIdentity

    // Configure is called when the agent is registered or its config changes.
    // Receives the merged configuration from AgentRegistration CRD.
    // Must be idempotent. Returns ConfigError if the config is invalid.
    Configure(ctx context.Context, config Config) error

    // HealthCheck returns nil if the agent is ready to handle Investigate calls.
    // Called periodically (default: every 30s) by the control plane.
    // A failure removes the agent from active routing until healthy again.
    // Must complete in under 5 seconds.
    HealthCheck(ctx context.Context) error

    // Investigate is the core method. It receives an investigation request
    // and returns a channel of events. The adapter MUST close the channel
    // when investigation is complete or context is cancelled.
    //
    // The adapter MUST honor:
    //   - ctx cancellation (terminate within 5s)
    //   - req.BudgetLimit (stop calling LLM/tools when exhausted)
    //   - req.AvailableTools (only request tools in this list)
    //   - req.Deadline (terminate at or before this time)
    Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error)

    // Shutdown is called when the agent is being deregistered or the
    // control plane is shutting down. The adapter must release all resources.
    // Must complete in under 30 seconds.
    Shutdown(ctx context.Context) error
}
```

That's the entire contract. Five methods. Everything else is data structures and protocols.

---

## 3. Type Definitions

### 3.1 Identity

```go
// AgentIdentity is metadata about an agent. Returned by Identity().
type AgentIdentity struct {
    // Name is the unique identifier for this agent type, e.g. "holmesgpt".
    // Lowercase, kebab-case. Must match RFC 1123 label format.
    Name string

    // Version is the SemVer of this adapter implementation, e.g. "1.4.2".
    // Used for compatibility checks and audit logging.
    Version string

    // SDKVersion is the AgentAdapter contract version this adapter implements.
    // Must be a valid SemVer matching a released SDK version.
    SDKVersion string

    // Description is a one-line human-readable description.
    Description string

    // Capabilities declares what kinds of investigations this agent handles.
    // Used by the routing engine to match alerts to agents.
    Capabilities []Capability

    // SupportedSourceTypes lists the MCP source types this agent can consume.
    // Wildcard "*" means it accepts any source.
    SupportedSourceTypes []string

    // Tags are arbitrary labels for filtering and discovery.
    Tags map[string]string
}

// Capability describes a domain the agent can investigate.
type Capability string

const (
    CapabilityKubernetes  Capability = "kubernetes"
    CapabilityPrometheus  Capability = "prometheus-alerts"
    CapabilityDatabase    Capability = "database-incidents"
    CapabilityNetwork     Capability = "network"
    CapabilitySecurity    Capability = "security-incidents"
    CapabilityCost        Capability = "cost-optimization"
    CapabilityCompliance  Capability = "compliance"
    CapabilityGeneric     Capability = "generic"
    // Custom capabilities are allowed; namespace with a dot, e.g. "mycompany.dba"
)
```

### 3.2 Configuration

```go
// Config is the merged configuration passed to Configure().
// Fields populated from AgentRegistration CRD + tenant defaults + secrets.
type Config struct {
    // TenantID identifies the tenant this agent instance serves.
    TenantID string

    // ModelConfig describes which LLM the agent should use.
    // The adapter MUST route LLM calls through the LLM Gateway endpoint
    // specified here, not directly to the provider.
    ModelConfig ModelConfig

    // MCPGatewayEndpoint is where the agent sends all MCP tool requests.
    // The agent MUST NOT call MCP servers directly.
    MCPGatewayEndpoint string

    // SkillSources is a read-only filesystem mount path containing
    // resolved skills (from SkillSource CRDs). Adapter loads from here.
    SkillsPath string

    // CustomConfig is adapter-specific settings from
    // AgentRegistration.spec.customConfig (passthrough YAML/JSON).
    CustomConfig map[string]any

    // SecretsPath is a read-only mount of secrets the adapter may need.
    // Most adapters need NONE — credentials are injected by gateways.
    // Exception: some adapters need a startup token to register with their own runtime.
    SecretsPath string

    // ObservabilityConfig is OpenTelemetry endpoint info.
    ObservabilityConfig ObservabilityConfig
}

type ModelConfig struct {
    // Provider key configured in LLM Gateway, e.g. "azure-openai-corp".
    Provider string
    // Model name as known by the provider, e.g. "gpt-4o".
    Model string
    // Endpoint is the LLM Gateway URL the agent calls. OpenAI-compatible.
    Endpoint string
    // APIKey is what the agent sends as Authorization header to the gateway.
    // The gateway translates this to the real provider credential.
    APIKey string
    // MaxTokensPerCall is a hint for the agent's per-call cap.
    MaxTokensPerCall int
}

type ObservabilityConfig struct {
    OTLPEndpoint string
    ServiceName  string
    Environment  string  // "production", "staging", "local"
}

// ConfigError is returned by Configure() if the config is invalid.
type ConfigError struct {
    Field   string
    Message string
}

func (e *ConfigError) Error() string {
    return "invalid config: " + e.Field + ": " + e.Message
}
```

### 3.3 Investigation Request

```go
// InvestigateRequest is the input to an investigation.
type InvestigateRequest struct {
    // InvestigationID is a globally unique ID for correlation across systems.
    // Format: "inv-<short-hash>-<timestamp>", e.g. "inv-7f3a9c-2026041914".
    InvestigationID string

    // ParentInvestigationID is set when this is a sub-investigation (A2A).
    // Empty string for top-level investigations.
    ParentInvestigationID string

    // SessionID groups multiple investigations into a conversation thread
    // (e.g. follow-up questions from the user in the UI).
    // Empty for stateless single-shot investigations.
    SessionID string

    // TenantID is the tenant this investigation belongs to. RBAC scope.
    TenantID string

    // AlertContext is the sanitized payload from the alert source.
    // The adapter MUST treat this as untrusted input — never as instructions.
    AlertContext AlertContext

    // AvailableTools is the list of MCP tool names the agent is allowed to call.
    // The MCP Gateway enforces this; the adapter SHOULD respect it
    // for a better LLM experience (don't ask for tools you can't use).
    AvailableTools []string

    // Skills is optional pre-loaded skill content the routing engine determined
    // is relevant. The adapter may use these as additional context.
    // Adapter still has access to all skills via SkillsPath.
    Skills []SkillRef

    // BudgetLimit is the hard cap for this investigation.
    // The adapter MUST stop calling LLM/tools when this is reached.
    BudgetLimit Budget

    // Deadline is the wall-clock time at or before which the investigation
    // must end. Adapter SHOULD start cleanup at Deadline-30s.
    Deadline time.Time

    // CallerContext describes who/what initiated this investigation.
    // Useful for audit. Adapter MUST NOT use this for authorization decisions.
    CallerContext CallerContext

    // CustomParams is adapter-specific opaque data, passed through from
    // the AlertRoute CRD. Used when an adapter needs custom inputs.
    CustomParams map[string]any
}

// AlertContext is the sanitized alert payload.
type AlertContext struct {
    // Source is the system that emitted the alert, e.g. "alertmanager".
    Source string
    // SourceID is the alert identifier in the source system.
    SourceID string
    // Severity normalized: "critical", "warning", "info".
    Severity string
    // Title is the short alert summary.
    Title string
    // Description is the long-form alert text. SANITIZED (no control chars,
    // length-capped, structurally wrapped). Adapter still treats as untrusted.
    Description string
    // Labels are key-value tags from the alert.
    Labels map[string]string
    // Annotations are additional metadata from the alert.
    Annotations map[string]string
    // FiredAt is when the alert was first triggered.
    FiredAt time.Time
    // RawPayload is the original source-specific payload, base64-encoded
    // if binary. Adapter MAY use this if needed but SHOULD prefer the
    // structured fields above.
    RawPayload []byte
}

type CallerContext struct {
    // Type is "alert", "user", "agent", "schedule", "api".
    Type string
    // For Type="user": OIDC subject of the user.
    UserSubject string
    // For Type="agent": the calling agent's identity (A2A).
    CallerAgent string
    // For Type="schedule": the cron schedule that triggered this.
    Schedule string
}

type SkillRef struct {
    // Name of the skill, e.g. "investigate-db-incident".
    Name string
    // Source is where it came from, e.g. "company-sre-skills@v1.2.3".
    Source string
    // Content is the SKILL.md content (already loaded for convenience).
    Content string
}

type Budget struct {
    // MaxTokens is the total LLM token budget for this investigation.
    MaxTokens int64
    // MaxUSD is the total cost budget. Whichever (tokens or USD) hits first wins.
    MaxUSD float64
    // MaxToolCalls limits the number of MCP tool invocations.
    MaxToolCalls int
    // MaxSubInvocations limits nested A2A calls (cumulative).
    MaxSubInvocations int
}
```

### 3.4 Events (the streaming protocol)

```go
// Event is a single message in the investigation stream.
// The adapter sends events through the channel returned by Investigate().
type Event struct {
    // Type discriminates which fields are populated.
    Type EventType

    // Timestamp is when this event was generated.
    Timestamp time.Time

    // Sequence is monotonically increasing per investigation.
    // Used for ordering and gap detection.
    Sequence int64

    // ID is a UUID for this specific event.
    ID string

    // CorrelationID is always equal to InvestigateRequest.InvestigationID.
    CorrelationID string

    // ParentEventID links related events (e.g., tool result -> tool call).
    ParentEventID string

    // Payload varies by Type. See type-specific structs below.
    Payload EventPayload
}

type EventType string

const (
    // EventThought: the agent's reasoning step. Content is natural language.
    EventThought EventType = "thought"

    // EventToolCallRequest: the agent wants to call a tool.
    // The control plane forwards to MCP Gateway.
    EventToolCallRequest EventType = "tool.request"

    // EventToolCallResponse: result of a tool call (success or error).
    // Generated by the control plane, NOT by the adapter.
    // (Listed here for completeness — adapters consume this from a separate channel.)
    EventToolCallResponse EventType = "tool.response"

    // EventLLMCall: the agent made an LLM call. Includes token counts + cost.
    // Used for cost tracking. Adapter SHOULD emit this after each LLM call.
    EventLLMCall EventType = "llm.call"

    // EventSubAgentRequest: the agent wants to invoke another agent (A2A).
    EventSubAgentRequest EventType = "subagent.request"

    // EventSubAgentResponse: result of a sub-agent invocation.
    EventSubAgentResponse EventType = "subagent.response"

    // EventApprovalRequest: the agent wants to do something requiring HITL.
    // Generated when the agent calls a tool with requireApproval policy.
    EventApprovalRequest EventType = "approval.request"

    // EventProgress: heartbeat / progress update with no new info.
    // Optional. Useful for long-running steps.
    EventProgress EventType = "progress"

    // EventPartialAnswer: streaming token-by-token of the final answer
    // (optional optimization for low-latency UI).
    EventPartialAnswer EventType = "answer.partial"

    // EventAnswer: the final structured answer.
    EventAnswer EventType = "answer"

    // EventError: a non-fatal error during investigation.
    // For fatal errors, return an error from Investigate() instead.
    EventError EventType = "error"

    // EventBudgetWarning: budget reaching a threshold (50%, 80%, 95%).
    EventBudgetWarning EventType = "budget.warning"

    // EventComplete: investigation finished. Always the last event.
    // Channel MUST be closed immediately after this.
    EventComplete EventType = "complete"
)

// EventPayload is the union of all type-specific payloads.
// Use a type switch on Event.Type to handle.
type EventPayload interface {
    eventPayload()  // sealed
}

// Concrete payload types

type ThoughtPayload struct {
    Content string
}
func (ThoughtPayload) eventPayload() {}

type ToolCallRequestPayload struct {
    ToolName string
    Args     map[string]any
    Reason   string  // optional natural-language justification
}
func (ToolCallRequestPayload) eventPayload() {}

type ToolCallResponsePayload struct {
    RequestEventID string
    Success        bool
    Result         any        // tool-specific
    Error          string     // populated if !Success
    DurationMS     int64
}
func (ToolCallResponsePayload) eventPayload() {}

type LLMCallPayload struct {
    Model           string
    InputTokens     int
    OutputTokens    int
    CostUSD         float64
    DurationMS      int64
    CacheHit        bool
}
func (LLMCallPayload) eventPayload() {}

type SubAgentRequestPayload struct {
    TargetAgent  string         // agent name, e.g. "security-specialist"
    Context      string         // what the caller wants the sub-agent to know
    Ask          string         // the specific question/task
    BudgetHint   *Budget        // optional, caller-suggested budget
    Reason       string         // why the agent decided to invoke this
}
func (SubAgentRequestPayload) eventPayload() {}

type SubAgentResponsePayload struct {
    RequestEventID         string
    SubInvestigationID     string
    Success                bool
    Answer                 string
    Error                  string
    CostUSD                float64
    DurationMS             int64
}
func (SubAgentResponsePayload) eventPayload() {}

type ApprovalRequestPayload struct {
    Action         string         // what's being approved, e.g. "kubernetes.exec"
    Args           map[string]any
    Rationale      string
    BlastRadius    string         // human-readable scope
}
func (ApprovalRequestPayload) eventPayload() {}

type ProgressPayload struct {
    Message    string
    PercentDone float64  // 0.0-1.0, optional (-1 if unknown)
}
func (ProgressPayload) eventPayload() {}

type PartialAnswerPayload struct {
    Token string
}
func (PartialAnswerPayload) eventPayload() {}

type AnswerPayload struct {
    Summary       string                  // 1-3 sentence executive summary
    RootCause     string                  // identified root cause
    Recommendation string                 // recommended action(s)
    Confidence    float64                 // 0.0-1.0
    Evidence      []EvidenceItem          // links to data that support the conclusion
    StructuredData map[string]any         // adapter-specific extras
}
func (AnswerPayload) eventPayload() {}

type EvidenceItem struct {
    Source       string  // e.g. "prometheus", "kubernetes"
    Description  string  // e.g. "CPU saturation in pod X over 15 minutes"
    Reference    string  // e.g. URL or query that produced this evidence
}

type ErrorPayload struct {
    Code        string  // see Error Codes section
    Message     string
    Recoverable bool    // if true, the agent will continue
    Details     map[string]any
}
func (ErrorPayload) eventPayload() {}

type BudgetWarningPayload struct {
    Resource    string  // "tokens", "usd", "tool_calls", "sub_invocations"
    UsedPercent float64 // 0.50, 0.80, 0.95
    Used        float64
    Limit       float64
}
func (BudgetWarningPayload) eventPayload() {}

type CompletePayload struct {
    Outcome     CompleteOutcome
    TotalCost   float64
    TotalTokens int64
    Reason      string  // why investigation ended
}
func (CompletePayload) eventPayload() {}

type CompleteOutcome string

const (
    OutcomeSuccess          CompleteOutcome = "success"
    OutcomePartial          CompleteOutcome = "partial"   // ran out of budget but has insights
    OutcomeNoResult         CompleteOutcome = "no_result" // couldn't find anything
    OutcomeBudgetExhausted  CompleteOutcome = "budget_exhausted"
    OutcomeDeadlineExceeded CompleteOutcome = "deadline_exceeded"
    OutcomeCancelled        CompleteOutcome = "cancelled"
    OutcomeError            CompleteOutcome = "error"
)
```

---

## 4. Lifecycle & State Machine

### 4.1 Adapter lifecycle

```
        Register CRD
              │
              ▼
        ┌───────────┐
        │ Identity()│   (called once)
        └─────┬─────┘
              │
              ▼
        ┌───────────┐
        │Configure()│   (called on registration + on config changes)
        └─────┬─────┘
              │
              ▼
        ┌───────────┐    every 30s
        │HealthCheck│◄──────────────┐
        └─────┬─────┘               │
              │                     │
              ▼                     │
        ┌────────────────────┐      │
        │     READY          │──────┘
        │ (accepts Investigate)
        └─────┬──────────────┘
              │
              ▼ on each alert
        ┌───────────────┐
        │ Investigate() │  (concurrent calls allowed)
        └─────┬─────────┘
              │
              ▼ on shutdown signal
        ┌───────────┐
        │Shutdown() │
        └───────────┘
```

### 4.2 Investigation lifecycle (within one Investigate call)

```
Investigate(ctx, req) called
    │
    ▼
┌─────────────────────────────────┐
│ Validate request                │
│ (return error if invalid)       │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ Allocate event channel (buf 64) │
│ Return channel + nil error      │
└────────────┬────────────────────┘
             │
             ▼ (in a goroutine)
┌─────────────────────────────────┐
│ Build initial prompt            │
│ Load relevant skills            │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Investigation loop:            │◄────┐
│  - Send EventThought            │     │
│  - Call LLM                     │     │
│  - Send EventLLMCall            │     │
│  - If tool needed:              │     │
│    * Send EventToolCallRequest  │     │
│    * Wait for response          │─────┤
│  - If sub-agent needed:         │     │
│    * Send EventSubAgentRequest  │     │
│    * Wait for response          │─────┤
│  - Check budget                 │     │
│  - Check ctx.Done()             │     │
│  - Loop until answer ready      │     │
└────────────┬────────────────────┘     │
             │                          │
             ▼                          │
┌─────────────────────────────────┐     │
│ Send EventAnswer                │     │
└────────────┬────────────────────┘     │
             │                          │
             ▼                          │
┌─────────────────────────────────┐     │
│ Send EventComplete              │     │
│ Close channel                   │     │
└─────────────────────────────────┘     │
                                        │
On budget exhausted, deadline,          │
or ctx cancelled:                       │
- Send EventComplete with appropriate   │
  Outcome                               │
- Close channel                         │
```

### 4.3 Concurrency contract

- Multiple `Investigate()` calls can be in-flight concurrently
- The adapter MUST be safe under concurrent calls
- Each Investigate has its own goroutine and channel
- `Configure()` is NOT called during active investigations (control plane handles serialization)
- `Shutdown()` waits for in-flight investigations to terminate (or timeout at 30s)

### 4.4 Cancellation contract

- Adapter MUST honor `ctx.Done()` within 5 seconds
- Adapter SHOULD cancel pending LLM/tool calls when ctx cancels
- Adapter MUST send `EventComplete{Outcome: OutcomeCancelled}` and close the channel
- Resource cleanup (open connections, file handles) happens in this 5-second window

---

## 5. Streaming Protocol Details

### 5.1 Channel discipline

```go
// CORRECT
func (a *MyAdapter) Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error) {
    if err := a.validate(req); err != nil {
        return nil, err
    }

    ch := make(chan Event, 64)  // buffered to avoid blocking the agent

    go func() {
        defer close(ch)  // ALWAYS close on exit
        a.runInvestigation(ctx, req, ch)
    }()

    return ch, nil
}

// WRONG — adapter doesn't close channel on error path
// WRONG — unbuffered channel blocks LLM streaming
// WRONG — returns error AND opens channel (must be one or the other)
```

### 5.2 Event ordering guarantees

- Events from one investigation are delivered in `Sequence` order
- Causally related events use `ParentEventID` (e.g., a tool response references the request)
- `EventComplete` is ALWAYS the last event
- The control plane assumes the channel close == EventComplete was sent
- Sending events after EventComplete is undefined behavior (will be ignored)

### 5.3 Event delivery semantics

- The control plane reads from the channel as fast as it can
- Backpressure: if buffer fills (64 events queued), the agent waits to send the next event
- This naturally throttles a runaway agent

### 5.4 Tool call round-trip

The cleanest part of the protocol — this is how an adapter requests a tool call:

```go
// In the adapter's investigation loop:

// 1. Send the request
ch <- Event{
    Type: EventToolCallRequest,
    Sequence: a.nextSeq(),
    ID: uuid.New().String(),
    CorrelationID: req.InvestigationID,
    Payload: ToolCallRequestPayload{
        ToolName: "kubernetes.get_pod",
        Args: map[string]any{
            "namespace": "tenant-a",
            "name": "postgres-7d8f9-xqz2k",
        },
        Reason: "Need to check pod status to diagnose CrashLoopBackOff",
    },
}

// 2. Wait for the response on the response channel.
//    The adapter received this channel via the Configure() call.
//    The control plane sends ToolCallResponsePayload events here.
response := <-a.toolResponseCh  // blocks until response arrives

// 3. Continue investigation with the result
if response.Success {
    // use response.Result
} else {
    // handle response.Error
}
```

**Note:** the response channel mechanism is a control-plane implementation detail (not in v1 of the spec). v1 implementations may use a callback or a shared map. To be finalized in M3 SDK release.

---

## 6. Error Model

### 6.1 Two kinds of errors

**Method-level errors:** returned from `Configure`, `HealthCheck`, `Investigate`, `Shutdown`. These are control-plane signals.

**Investigation-level errors:** sent as `EventError` events. These are signals to the human and to the audit log.

### 6.2 Error codes (canonical)

```go
const (
    // Configuration errors (returned by Configure)
    ErrCodeInvalidConfig         = "invalid_config"
    ErrCodeMissingRequiredField  = "missing_required_field"
    ErrCodeUnsupportedFeature    = "unsupported_feature"

    // Runtime errors (returned by Investigate or sent as EventError)
    ErrCodeLLMUnavailable        = "llm_unavailable"
    ErrCodeLLMRateLimited        = "llm_rate_limited"
    ErrCodeBudgetExhausted       = "budget_exhausted"
    ErrCodeDeadlineExceeded      = "deadline_exceeded"
    ErrCodeToolUnavailable       = "tool_unavailable"
    ErrCodeToolDenied            = "tool_denied"
    ErrCodeApprovalDenied        = "approval_denied"
    ErrCodeApprovalTimeout       = "approval_timeout"
    ErrCodeSubAgentFailed        = "subagent_failed"
    ErrCodeAgentInternal         = "agent_internal_error"

    // Health errors (returned by HealthCheck)
    ErrCodeUnhealthy             = "unhealthy"
    ErrCodeDegraded              = "degraded"
)

// Standard error type
type AdapterError struct {
    Code      string
    Message   string
    Cause     error            // optional underlying error
    Details   map[string]any   // optional extra context
}

func (e *AdapterError) Error() string {
    if e.Cause != nil {
        return e.Code + ": " + e.Message + " (caused by: " + e.Cause.Error() + ")"
    }
    return e.Code + ": " + e.Message
}

func (e *AdapterError) Unwrap() error {
    return e.Cause
}
```

### 6.3 When to use which

| Situation | What to do |
|---|---|
| Config is missing required field | Return `*AdapterError{Code: ErrCodeMissingRequiredField}` from `Configure()` |
| LLM provider unreachable at startup | Return `*AdapterError{Code: ErrCodeLLMUnavailable}` from `HealthCheck()` |
| LLM rate limit during investigation | Send `EventError{Recoverable: true}` and retry |
| LLM rate limit after retries failed | Send `EventComplete{Outcome: OutcomeError}` and close channel |
| Budget exhausted | Send `EventComplete{Outcome: OutcomeBudgetExhausted}` |
| Tool denied by MCP Gateway | Send `EventError{Code: ErrCodeToolDenied, Recoverable: true}`, agent decides whether to give up |
| Agent's own bug (panic) | Recover, send `EventError{Code: ErrCodeAgentInternal}`, send `EventComplete{Outcome: OutcomeError}` |

### 6.4 Critical: never panic across the boundary

```go
// CORRECT pattern in the investigation goroutine:
go func() {
    defer close(ch)
    defer func() {
        if r := recover(); r != nil {
            ch <- Event{
                Type: EventError,
                Payload: ErrorPayload{
                    Code: ErrCodeAgentInternal,
                    Message: fmt.Sprintf("adapter panic: %v", r),
                    Recoverable: false,
                },
            }
            ch <- Event{
                Type: EventComplete,
                Payload: CompletePayload{Outcome: OutcomeError},
            }
        }
    }()
    a.runInvestigation(ctx, req, ch)
}()
```

---

## 7. Sub-Agent Invocation (A2A)

This is **the strategic feature**. Adapters that support invoking other agents do it through this protocol.

### 7.1 The flow

```
┌────────────────────────────────────────────────────────┐
│ Adapter A's Investigate() goroutine                    │
│                                                        │
│ During investigation, agent decides:                   │
│   "I need a security specialist's analysis"            │
│                                                        │
│ Adapter A:                                             │
│   1. Sends EventSubAgentRequest                        │
│   2. Waits for response (similar to tool calls)        │
└──────────────────┬─────────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────────┐
│ Leloir Control Plane                                   │
│                                                        │
│ 1. Validates: A has canInvoke perms? B in route team?  │
│ 2. Computes effective budget: min(tenant, route,       │
│    parent_remaining, caller_hint)                      │
│ 3. Checks: cycle? depth? fan-out limits?               │
│ 4. If all pass: invokes B as sub-investigation         │
│                                                        │
│ Calls Adapter B's Investigate() with:                  │
│   ParentInvestigationID = A's investigation ID         │
│   BudgetLimit = effective budget                       │
│   AlertContext = synthesized from A's context+ask      │
│   CallerContext.Type = "agent"                         │
│   CallerContext.CallerAgent = "agent-A-name"           │
└──────────────────┬─────────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────────┐
│ Adapter B's Investigate() — runs as normal             │
│ Streams events back. Control plane forwards summary    │
│ to A as EventSubAgentResponse.                         │
└──────────────────┬─────────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────────┐
│ Adapter A receives SubAgentResponse, integrates result │
│ Continues its own investigation                        │
└────────────────────────────────────────────────────────┘
```

### 7.2 What an adapter sees as "the caller"

When invoked as a sub-agent, the adapter sees:

```go
req.ParentInvestigationID = "inv-7f3a9c-2026041914"  // not empty
req.CallerContext.Type = "agent"
req.CallerContext.CallerAgent = "holmes-prod"
req.AlertContext.Source = "agent-invocation"  // synthetic
req.AlertContext.Description = <the "ask" from the caller>
req.BudgetLimit = <reduced budget per platform rules>
```

The sub-agent doesn't need to know it's a sub-agent. It just investigates with what it's given.

### 7.3 Recursion limits

The control plane enforces:

- `maxInvocationDepth: 5` — A→B→C→D→E is OK, A→B→C→D→E→F is denied
- `maxFanOutPerAgent: 3` — A cannot invoke 4 sub-agents in parallel
- `maxTotalSubInvocations: 20` — entire investigation tree caps at 20 calls
- Cycle detection on the call stack

When a limit is hit, the adapter receives an EventSubAgentResponse with Success=false and Error containing the reason.

### 7.4 Adapters that don't support A2A

A2A is OPTIONAL for adapters. If you don't want your agent to invoke others:

- Don't emit EventSubAgentRequest
- The platform won't invoke you with `ParentInvestigationID` either, unless you opt in via `AgentRegistration.spec.supportsBeingInvoked: true`

This is fine for simple agents that only respond to alerts.

### 7.5 OpenCode-style nested subagents (Pattern C)

Different from A2A: OpenCode spawns subagents inside its own process. The adapter handles this internally and just emits subagent events for observability:

```go
// In OpenCode adapter, when OpenCode spawns an internal subagent:
ch <- Event{
    Type: EventSubAgentRequest,
    Payload: SubAgentRequestPayload{
        TargetAgent: "<internal:opencode-subagent>",  // marker
        Context: "OpenCode spawned internal log-analyzer",
        Ask: "...",
        Reason: "...",
    },
}

// When OpenCode's subagent finishes:
ch <- Event{
    Type: EventSubAgentResponse,
    Payload: SubAgentResponsePayload{
        Success: true,
        Answer: "...",
        CostUSD: 0.42,  // attributed to parent
    },
}
```

The platform sees the tree, costs roll up, audit traces are nested. But the platform is NOT involved in the orchestration — that's fully inside OpenCode.

---

## 8. Budget & Cost Model

### 8.1 Budget enforcement is the adapter's responsibility

The adapter receives `req.BudgetLimit` and MUST stop when it's exhausted. The platform tracks cost externally too (via LLM Gateway), but the adapter is the first line of defense.

### 8.2 What to track

```go
type internalBudget struct {
    tokensUsed    int64
    usdSpent      float64
    toolCallsUsed int
    subInvocsUsed int
    limit         Budget
    mu            sync.Mutex
}

func (b *internalBudget) recordLLMCall(input, output int, cost float64) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.tokensUsed += int64(input + output)
    b.usdSpent += cost
}

func (b *internalBudget) canContinue() (bool, string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.tokensUsed >= b.limit.MaxTokens {
        return false, "max_tokens"
    }
    if b.usdSpent >= b.limit.MaxUSD {
        return false, "max_usd"
    }
    if b.toolCallsUsed >= b.limit.MaxToolCalls {
        return false, "max_tool_calls"
    }
    return true, ""
}
```

### 8.3 When to emit budget warnings

After each LLM call, check:

```go
if b.usdSpent / b.limit.MaxUSD >= 0.5 && !sentWarning50 {
    ch <- Event{Type: EventBudgetWarning, Payload: BudgetWarningPayload{
        Resource: "usd", UsedPercent: 0.5, Used: b.usdSpent, Limit: b.limit.MaxUSD,
    }}
    sentWarning50 = true
}
// Same for 0.8 and 0.95
```

This lets the UI show a budget meter and gives the SRE time to extend budget if needed (via HITL override).

### 8.4 Cost attribution

The adapter reports its costs via `EventLLMCall`. The control plane uses these for:

- Cost dashboard per tenant/agent/investigation
- Budget enforcement at the platform level (defense in depth)
- Audit log (compliance evidence of resource use)

Don't fudge these numbers. The LLM Gateway also tracks them and discrepancies are detected.

---

## 9. Tool Access Through MCP Gateway

### 9.1 The adapter never calls MCP servers directly

```go
// WRONG — adapter calls MCP server directly
mcpClient := mcp.NewClient("https://kubernetes-mcp.svc.local:8443")
result := mcpClient.Call("get_pod", args)

// RIGHT — adapter sends EventToolCallRequest, control plane routes via MCP Gateway
ch <- Event{
    Type: EventToolCallRequest,
    Payload: ToolCallRequestPayload{
        ToolName: "kubernetes.get_pod",
        Args: args,
    },
}
result := <-toolResponseCh
```

### 9.2 Why this matters

Going through the MCP Gateway gives you:

- Per-tenant scoping (tenant A's call gets tenant A's data)
- Credential injection (you don't need GitHub PAT, gateway adds it)
- HITL approval workflow (gateway pauses for approval if policy requires)
- Audit logging (every tool call recorded with full context)
- Rate limiting per tenant
- Transport translation (HTTP/JSON in, gRPC out if MCP server is gRPC)

If you bypass it, you lose all of this AND you may leak credentials.

### 9.3 Tool discovery

The adapter learns available tools from `req.AvailableTools`. The control plane filters this list based on `AlertRoute.allowedSources` and `MCPServer.policies.allowedTools`.

The adapter SHOULD only request tools in this list. The MCP Gateway will reject requests for tools outside the allowlist.

---

## 10. Configuration & Secrets

### 10.1 What goes in CustomConfig

The `Config.CustomConfig` field is opaque to the platform. It's how adapter-specific settings flow from `AgentRegistration.spec.customConfig` to the adapter:

```yaml
# In AgentRegistration CRD
apiVersion: leloir.dev/v1
kind: AgentRegistration
metadata:
  name: holmesgpt-prod
spec:
  adapter:
    image: ghcr.io/leloir-platform/adapter-holmesgpt:v1.4.2
  customConfig:
    holmes:
      apiBaseURL: "http://holmes.holmes.svc.cluster.local"
      timeout: 300
      additionalRunbooksPath: "/skills/runbooks"
```

The Holmes adapter receives `Config.CustomConfig["holmes"]` as a `map[string]any` and parses it.

### 10.2 Secrets handling

Adapters SHOULD NOT need secrets for tool access (that's MCP Gateway's job). But some adapters need secrets for their own runtime, e.g., to authenticate to the agent's HTTP API.

```go
// Read secrets from the mounted SecretsPath
secretsPath := config.SecretsPath  // e.g. "/var/run/leloir/secrets"
holmesAuthToken, err := os.ReadFile(filepath.Join(secretsPath, "holmes-auth"))
```

The platform mounts these from K8s Secrets (which come from Vault via External Secrets Operator in corporate profile).

**Never log secret values.** Never include them in events. Never send them to the LLM.

---

## 11. Observability (OpenTelemetry contract)

### 11.1 Span propagation

The platform creates a parent span for the investigation and propagates it via context:

```go
func (a *MyAdapter) Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error) {
    // ctx already has the parent span; create child spans automatically
    tracer := otel.Tracer("leloir-adapter-myagent")
    ctx, span := tracer.Start(ctx, "myagent.investigate",
        trace.WithAttributes(
            attribute.String("investigation.id", req.InvestigationID),
            attribute.String("tenant.id", req.TenantID),
        ),
    )
    defer span.End()

    // All subsequent calls (LLM, internal logic) inherit this context
    // and create child spans automatically.
    ...
}
```

### 11.2 Required span attributes

Every span the adapter creates SHOULD include:

- `investigation.id` — from req.InvestigationID
- `tenant.id` — from req.TenantID
- `agent.name` — from Identity().Name
- `agent.version` — from Identity().Version

For LLM call spans:
- `llm.provider` — e.g. "azure-openai"
- `llm.model` — e.g. "gpt-4o"
- `llm.input_tokens`, `llm.output_tokens`, `llm.cost_usd`

For tool call spans:
- `tool.name`
- `tool.duration_ms`
- `tool.success` (bool)

### 11.3 Metrics

Optional but encouraged. The adapter can expose Prometheus metrics:

```
leloir_adapter_investigations_started_total{agent="holmes",tenant="..."}
leloir_adapter_investigations_completed_total{agent="...",tenant="...",outcome="success"}
leloir_adapter_investigation_duration_seconds{agent="...",tenant="..."}
leloir_adapter_llm_calls_total{agent="...",model="..."}
leloir_adapter_llm_cost_usd_total{agent="...",tenant="..."}
```

The platform aggregates these for the dashboard.

---

## 12. Testing — Conformance Suite

The conformance suite is a Go module any adapter can import and run:

```go
// go.mod of adapter
require github.com/leloir-platform/sdk/conformance v1.0.0

// In an adapter test file:
package myagent_test

import (
    "testing"
    "github.com/leloir-platform/sdk/conformance"
    "myorg/myagent"
)

func TestConformance(t *testing.T) {
    adapter := myagent.New()
    conformance.RunSuite(t, adapter, conformance.Options{
        // Provide a way for the suite to mock LLM calls
        MockLLM: true,
        // Provide a way to mock tool responses
        MockToolResponses: map[string]any{
            "kubernetes.get_pod": map[string]any{"status": "Running"},
        },
    })
}
```

### 12.1 What the suite verifies

| Test | What it checks |
|---|---|
| `Identity_IsDeterministic` | Calling Identity() twice returns the same struct |
| `Configure_RejectsInvalidConfig` | Configure() returns ConfigError for known-bad configs |
| `Configure_IsIdempotent` | Calling Configure() twice with same input has no different effect |
| `HealthCheck_CompletesQuickly` | HealthCheck() returns within 5s |
| `Investigate_ReturnsChannel` | Investigate() returns a non-nil channel and nil error for valid input |
| `Investigate_HonorsContextCancellation` | Channel closes within 5s of ctx.Cancel() |
| `Investigate_HonorsBudget` | Adapter stops calling LLM when budget exhausted, sends BudgetExhausted complete event |
| `Investigate_HonorsDeadline` | Adapter stops by Deadline, sends DeadlineExceeded complete event |
| `Investigate_AlwaysClosesChannel` | Channel is closed even on error paths |
| `Investigate_AlwaysSendsCompleteEvent` | EventComplete is the last event before close |
| `Investigate_RespectsAvailableTools` | Adapter doesn't request tools outside the AvailableTools list |
| `Investigate_HandlesToolErrors` | Adapter handles tool error responses gracefully |
| `Investigate_RecoversFromPanic` | Internal panic doesn't crash; sends ErrorPayload + Complete |
| `Investigate_ConcurrentSafe` | Multiple concurrent calls work correctly |
| `Shutdown_CompletesQuickly` | Shutdown() returns within 30s |
| `Shutdown_TerminatesInflightInvestigations` | After Shutdown(), in-flight investigations cancel |

### 12.2 Anti-tests (things that should fail)

The suite also has "negative" tests that intentionally do bad things and verify the adapter handles them:

- Calling Investigate() with invalid InvestigationID format
- Sending a request with empty TenantID
- Tool response that returns malformed JSON
- LLM that returns 500 error
- Skill mount that doesn't exist

### 12.3 CI integration

```yaml
# .github/workflows/conformance.yaml
name: AgentAdapter Conformance
on: [push, pull_request]
jobs:
  conformance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test ./... -run TestConformance -v
```

If tests pass, the adapter is conformant. If not, the PR can't merge (for in-tree adapters; for community adapters, it's a hint).

---

## 13. Versioning Policy & Backward Compatibility

### 13.1 SemVer applied

| Version bump | Triggers | Examples |
|---|---|---|
| MAJOR (1.x → 2.0) | Removed/renamed methods, changed signatures, removed event types | Renaming `Investigate` to `Run` |
| MINOR (1.0 → 1.1) | New optional method (with default), new event type, new optional field, new error code | Adding `EventBudgetWarning` |
| PATCH (1.0.0 → 1.0.1) | Bug fixes, doc improvements, no API change | Fix a typo in conformance test message |

### 13.2 Deprecation policy

When a method or field is deprecated:

1. Mark with `// Deprecated: removed in v2. Use NewMethod instead.`
2. Keep working for at least 2 minor versions before removal
3. Conformance suite emits warning when deprecated APIs are used
4. Migration guide published with the deprecation

### 13.3 Forward compatibility

New event types may be added in minor versions. Adapters that implement an older SDK MUST handle unknown event types gracefully (ignore them). The event channel uses `Event.Type` discriminator, so unknown types just don't match any switch case.

### 13.4 Compatibility matrix in docs

```
Leloir Platform v1.x supports adapters using SDK: v1.x (current), v0.x (best effort)
Leloir Platform v2.x will support: SDK v2.x (current), SDK v1.x (deprecated)
```

Adapters and platform can be upgraded independently within a major version.

---

## 14. Reference Implementation Walkthrough — HolmesGPT

**Goal:** show what a real adapter looks like for a CNCF Sandbox agent.

### 14.1 Full file structure

```
adapters/holmesgpt/
├── adapter.go          # Implements AgentAdapter
├── adapter_test.go     # Conformance tests
├── client.go           # HTTP client for Holmes API
├── events.go           # Translation between Holmes SSE events and Leloir Events
├── budget.go           # Budget tracking
├── go.mod
├── Dockerfile
└── README.md
```

### 14.2 The adapter struct

```go
package holmesgpt

import (
    "context"
    "github.com/leloir-platform/sdk/adapter"
)

type Adapter struct {
    config     adapter.Config
    holmesURL  string
    httpClient *http.Client
    healthCh   chan error
}

// Compile-time check that we implement the interface
var _ adapter.AgentAdapter = (*Adapter)(nil)

func New() *Adapter {
    return &Adapter{
        httpClient: &http.Client{Timeout: 10 * time.Second},
        healthCh:   make(chan error, 1),
    }
}
```

### 14.3 Identity

```go
func (a *Adapter) Identity() adapter.AgentIdentity {
    return adapter.AgentIdentity{
        Name:        "holmesgpt",
        Version:     buildVersion,  // injected at compile time
        SDKVersion:  "1.0.0",
        Description: "HolmesGPT — CNCF Sandbox AI agent for cloud-native incident analysis",
        Capabilities: []adapter.Capability{
            adapter.CapabilityKubernetes,
            adapter.CapabilityPrometheus,
            adapter.CapabilityGeneric,
        },
        SupportedSourceTypes: []string{
            "kubernetes-mcp",
            "prometheus-mcp",
            "loki-mcp",
            "*",  // Holmes is happy with any MCP source
        },
        Tags: map[string]string{
            "license":  "Apache-2.0",
            "upstream": "https://github.com/HolmesGPT/holmesgpt",
            "category": "incident-response",
        },
    }
}
```

### 14.4 Configure

```go
func (a *Adapter) Configure(ctx context.Context, config adapter.Config) error {
    holmesConfig, ok := config.CustomConfig["holmes"].(map[string]any)
    if !ok {
        return &adapter.AdapterError{
            Code:    adapter.ErrCodeMissingRequiredField,
            Message: "customConfig.holmes is required",
        }
    }

    apiURL, ok := holmesConfig["apiBaseURL"].(string)
    if !ok || apiURL == "" {
        return &adapter.AdapterError{
            Code:    adapter.ErrCodeMissingRequiredField,
            Message: "customConfig.holmes.apiBaseURL is required",
        }
    }

    a.config = config
    a.holmesURL = apiURL

    // Verify Holmes is reachable
    if err := a.ping(ctx); err != nil {
        return &adapter.AdapterError{
            Code:    adapter.ErrCodeLLMUnavailable,
            Message: "Holmes API unreachable",
            Cause:   err,
        }
    }

    return nil
}
```

### 14.5 HealthCheck

```go
func (a *Adapter) HealthCheck(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, "GET", a.holmesURL+"/health", nil)
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return &adapter.AdapterError{
            Code: adapter.ErrCodeUnhealthy, Cause: err,
        }
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return &adapter.AdapterError{
            Code:    adapter.ErrCodeUnhealthy,
            Message: fmt.Sprintf("Holmes returned %d", resp.StatusCode),
        }
    }
    return nil
}
```

### 14.6 Investigate (the meat)

```go
func (a *Adapter) Investigate(
    ctx context.Context,
    req adapter.InvestigateRequest,
) (<-chan adapter.Event, error) {
    // Validate
    if req.InvestigationID == "" {
        return nil, &adapter.AdapterError{
            Code:    adapter.ErrCodeInvalidConfig,
            Message: "InvestigationID required",
        }
    }

    ch := make(chan adapter.Event, 64)

    go func() {
        defer close(ch)
        defer recoverPanic(ch, req.InvestigationID)

        budget := newBudgetTracker(req.BudgetLimit)
        seq := int64(0)
        nextSeq := func() int64 { seq++; return seq }

        // Build prompt for Holmes
        prompt := buildPrompt(req)

        // POST to Holmes /api/chat with streaming
        holmesReq := holmesAPIRequest{
            Ask:           prompt,
            AvailableTools: req.AvailableTools,
            Stream:        true,
            // Holmes also takes its own model config — we pass the LLM Gateway here
            ModelConfig: holmesModelConfig{
                Provider: "openai",   // OpenAI-compatible (the Gateway speaks it)
                Endpoint: req.ModelConfig.Endpoint,
                APIKey:   req.ModelConfig.APIKey,
                Model:    req.ModelConfig.Model,
            },
        }

        sseCh, err := a.callHolmesStreaming(ctx, holmesReq)
        if err != nil {
            ch <- adapter.Event{
                Type:          adapter.EventError,
                Sequence:      nextSeq(),
                CorrelationID: req.InvestigationID,
                Timestamp:     time.Now(),
                Payload: adapter.ErrorPayload{
                    Code:        adapter.ErrCodeLLMUnavailable,
                    Message:     err.Error(),
                    Recoverable: false,
                },
            }
            ch <- adapter.Event{
                Type: adapter.EventComplete,
                Sequence: nextSeq(),
                Timestamp: time.Now(),
                Payload: adapter.CompletePayload{Outcome: adapter.OutcomeError},
            }
            return
        }

        // Translate Holmes SSE events to Leloir Events
        for holmesEvt := range sseCh {
            select {
            case <-ctx.Done():
                ch <- adapter.Event{
                    Type: adapter.EventComplete,
                    Payload: adapter.CompletePayload{Outcome: adapter.OutcomeCancelled},
                }
                return
            default:
            }

            switch holmesEvt.Type {
            case "thought":
                ch <- adapter.Event{
                    Type:          adapter.EventThought,
                    Sequence:      nextSeq(),
                    CorrelationID: req.InvestigationID,
                    Timestamp:     time.Now(),
                    Payload:       adapter.ThoughtPayload{Content: holmesEvt.Content},
                }
            case "tool_call":
                ch <- adapter.Event{
                    Type:          adapter.EventToolCallRequest,
                    Sequence:      nextSeq(),
                    CorrelationID: req.InvestigationID,
                    Timestamp:     time.Now(),
                    Payload: adapter.ToolCallRequestPayload{
                        ToolName: holmesEvt.ToolName,
                        Args:     holmesEvt.ToolArgs,
                    },
                }
            case "llm_call":
                budget.recordLLMCall(holmesEvt.InputTokens, holmesEvt.OutputTokens, holmesEvt.Cost)
                ch <- adapter.Event{
                    Type: adapter.EventLLMCall,
                    Payload: adapter.LLMCallPayload{
                        Model:        holmesEvt.Model,
                        InputTokens:  holmesEvt.InputTokens,
                        OutputTokens: holmesEvt.OutputTokens,
                        CostUSD:      holmesEvt.Cost,
                    },
                }
                if ok, reason := budget.canContinue(); !ok {
                    ch <- adapter.Event{
                        Type: adapter.EventComplete,
                        Payload: adapter.CompletePayload{
                            Outcome: adapter.OutcomeBudgetExhausted,
                            Reason:  reason,
                        },
                    }
                    return
                }
            case "answer":
                ch <- adapter.Event{
                    Type: adapter.EventAnswer,
                    Payload: adapter.AnswerPayload{
                        Summary:        holmesEvt.Summary,
                        RootCause:      holmesEvt.RootCause,
                        Recommendation: holmesEvt.Recommendation,
                        Confidence:     holmesEvt.Confidence,
                    },
                }
            }
        }

        ch <- adapter.Event{
            Type: adapter.EventComplete,
            Payload: adapter.CompletePayload{Outcome: adapter.OutcomeSuccess,
                TotalCost:   budget.usdSpent,
                TotalTokens: budget.tokensUsed,
            },
        }
    }()

    return ch, nil
}
```

### 14.7 Shutdown

```go
func (a *Adapter) Shutdown(ctx context.Context) error {
    // Holmes doesn't need explicit shutdown; it lives in its own pod.
    // Just close idle HTTP connections.
    a.httpClient.CloseIdleConnections()
    return nil
}
```

That's the entire HolmesGPT adapter — about 200-250 lines including helpers. This is the level of complexity any adapter author should expect.

---

## 15. Reference Implementation Walkthrough — Custom Agent (the cookbook)

How a Leloir user writes their own adapter for, say, a custom DBA agent built in-house.

### 15.1 Pre-requisites

- Go 1.22+
- Familiarity with HTTP/streaming
- Your custom agent exposes some kind of API (HTTP, gRPC, stdin/stdout, doesn't matter)

### 15.2 Step 1 — Bootstrap the project

```bash
mkdir my-dba-adapter && cd my-dba-adapter
go mod init github.com/myorg/my-dba-adapter
go get github.com/leloir-platform/sdk/adapter@v1.0.0
go get github.com/leloir-platform/sdk/conformance@v1.0.0
```

### 15.3 Step 2 — Implement the interface

Create `adapter.go`:

```go
package mydba

import (
    "context"
    "github.com/leloir-platform/sdk/adapter"
)

type Adapter struct {
    config adapter.Config
    // ... whatever you need to talk to your DBA agent
}

var _ adapter.AgentAdapter = (*Adapter)(nil)

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Identity() adapter.AgentIdentity {
    return adapter.AgentIdentity{
        Name:        "my-dba-agent",
        Version:     "0.1.0",
        SDKVersion:  "1.0.0",
        Description: "DBA specialist agent for PostgreSQL and MySQL incidents",
        Capabilities: []adapter.Capability{
            adapter.CapabilityDatabase,
        },
        SupportedSourceTypes: []string{"postgres-mcp", "mysql-mcp"},
    }
}

func (a *Adapter) Configure(ctx context.Context, config adapter.Config) error {
    a.config = config
    // Your custom validation
    return nil
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
    // Verify your agent is reachable
    return nil
}

func (a *Adapter) Investigate(ctx context.Context, req adapter.InvestigateRequest) (<-chan adapter.Event, error) {
    ch := make(chan adapter.Event, 64)
    go func() {
        defer close(ch)

        // Your logic to invoke your agent here.
        // For each interesting event, send to ch.

        // At the end:
        ch <- adapter.Event{
            Type: adapter.EventAnswer,
            Payload: adapter.AnswerPayload{
                Summary:   "...",
                RootCause: "...",
            },
        }
        ch <- adapter.Event{
            Type:    adapter.EventComplete,
            Payload: adapter.CompletePayload{Outcome: adapter.OutcomeSuccess},
        }
    }()
    return ch, nil
}

func (a *Adapter) Shutdown(ctx context.Context) error { return nil }
```

### 15.4 Step 3 — Conformance test

Create `adapter_test.go`:

```go
package mydba_test

import (
    "testing"
    "github.com/leloir-platform/sdk/conformance"
    "github.com/myorg/my-dba-adapter"
)

func TestConformance(t *testing.T) {
    a := mydba.New()
    conformance.RunSuite(t, a, conformance.Options{MockLLM: true})
}
```

```bash
go test -v ./...
# Should pass all tests before proceeding.
```

### 15.5 Step 4 — Build container

Create `Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o adapter ./cmd/adapter

FROM alpine:3.19
RUN adduser -D -u 10001 adapter
USER adapter
COPY --from=build /app/adapter /adapter
ENTRYPOINT ["/adapter"]
```

```bash
docker build -t myorg/my-dba-adapter:0.1.0 .
docker push myorg/my-dba-adapter:0.1.0
```

### 15.6 Step 5 — Register with Leloir

```yaml
apiVersion: leloir.dev/v1
kind: AgentRegistration
metadata:
  name: my-dba-agent
  namespace: leloir
spec:
  adapter:
    image: myorg/my-dba-adapter:0.1.0
  modelConfig: azure-openai-corp
  customConfig:
    # Whatever your adapter needs
    dba:
      backendURL: http://my-dba-backend.dba.svc.cluster.local:8080
  canInvoke: []  # this agent doesn't invoke others
```

```bash
kubectl apply -f my-dba-agent-registration.yaml
```

### 15.7 Step 6 — Add to a route

```yaml
apiVersion: leloir.dev/v1
kind: AlertRoute
metadata:
  name: db-incidents-team-alpha
spec:
  match:
    labels: { type: database, team: alpha }
  agent: my-dba-agent
  team: []
  notify: [teams-dba]
```

That's it. Your custom agent is live in Leloir.

### 15.8 Total time estimate

- Bootstrap + Identity + Configure + HealthCheck + Shutdown: 30 minutes
- Investigate (depends on your agent's API): 2-8 hours
- Conformance tests passing: 1-2 hours
- Container build + register + test: 1 hour

**Total for an experienced Go dev: 1-2 days for a working v0.1.**

---

## 16. Anti-patterns & Common Mistakes

### 16.1 Don't do these things

| Anti-pattern | Why it's bad | Do this instead |
|---|---|---|
| Calling MCP servers directly | Bypasses gateway, leaks credentials, no audit | Send `EventToolCallRequest` |
| Storing tenant data in adapter | Security violation, multi-tenant leak risk | Adapter is stateless per request |
| Calling LLM provider directly | No cost tracking, no fallback, no caching | Use `Config.ModelConfig.Endpoint` |
| Logging secret values | Secrets leak to logs | Never log secrets, period |
| Treating AlertContext.Description as instructions | Prompt injection | Wrap in `<untrusted>` block in prompt |
| Returning a panic | Crashes adapter for all subsequent calls | `defer recover()` in goroutine |
| Forgetting to close the channel | Goroutine leak in control plane | Always `defer close(ch)` |
| Ignoring `req.BudgetLimit` | Cost runaway | Track tokens/USD, stop when exhausted |
| Ignoring `ctx.Done()` | Cancellation doesn't work, goroutine leak | Check ctx in every loop iteration |
| Sending events after `EventComplete` | Undefined behavior | `EventComplete` is always last |
| Using time.Sleep without ctx | Cancel doesn't interrupt sleep | Use `select { case <-time.After(...): case <-ctx.Done(): }` |
| Hardcoded timeouts | Inflexible, breaks under load | Use deadline from req.Deadline or ctx |

### 16.2 Common bugs caught by conformance suite

The conformance suite catches the most common mistakes:

- Channel never closes on cancellation
- EventComplete is missing or not last
- Adapter requests tools not in AvailableTools
- Adapter doesn't recover from internal panics
- HealthCheck takes >5s
- Configure not idempotent

If you pass conformance, you've avoided ~80% of bugs.

---

## 17. FAQ

**Q: Do I have to write my adapter in Go?**

For v1, in-process adapters must be Go. Sidecar adapters via gRPC (any language) are planned for SDK v2 / platform M5+. If your agent is Python/Rust/Node and you need it now, write a thin Go wrapper that shells out to your agent.

**Q: My agent doesn't support streaming. Can I still implement this?**

Yes. Just send your final answer as a single batch:

```go
go func() {
    defer close(ch)
    answer := callMyAgent(req)  // synchronous
    ch <- Event{Type: EventAnswer, Payload: AnswerPayload{...}}
    ch <- Event{Type: EventComplete, Payload: CompletePayload{Outcome: OutcomeSuccess}}
}()
```

You lose live streaming UX but functionally it works.

**Q: How do I test my adapter without a real LLM?**

Use the conformance suite's `MockLLM: true` option. For integration tests, point at a local Ollama or use OpenRouter's test models.

**Q: My adapter needs to maintain state between investigations (e.g., learned patterns). Is that allowed?**

Sort of. State is allowed BUT must be:
- Per-tenant scoped (no cross-tenant leakage)
- Not security-sensitive
- Documented in your adapter's README
- Reset on `Shutdown()`

For persistent state, use a sidecar database. Don't put state in the adapter binary.

**Q: Can my adapter spawn its own subagents internally (Pattern C)?**

Yes — this is exactly what OpenCode does. Just emit `EventSubAgentRequest`/`EventSubAgentResponse` so the platform can observe + cost-attribute, but you handle orchestration internally.

**Q: What if I want my adapter to call a tool that's not in AvailableTools?**

Don't. The MCP Gateway will reject it. Your route operator decides what tools you have access to via the AlertRoute CRD.

**Q: How do I handle the platform giving me a budget too small for my agent to do anything useful?**

Send `EventComplete{Outcome: OutcomeNoResult}` with a message explaining. The platform can show this to the user, who can extend the budget via UI.

**Q: My agent has "modes" (e.g., conservative vs aggressive analysis). How do I expose that?**

Use `req.CustomParams`. Configure your routes to pass mode-specific params:

```yaml
spec:
  agent: my-agent
  customParams:
    mode: conservative
```

**Q: Can my adapter charge customers? Is there a marketplace?**

Not in v1. The platform is open source, free. If you want to charge, run your own commercial agent and let people install your adapter. The platform doesn't gate this.

**Q: My adapter discovered a security issue while investigating. Can it auto-create a Jira ticket?**

Only via tools. Tools that write require `requireApproval` policy by default in `ApprovalPolicy`. So: agent emits `EventToolCallRequest` for `jira.create_issue`, gateway pauses for human approval, human approves, ticket gets created. No way to bypass HITL from the adapter.

**Q: How do I debug my adapter in production?**

- OpenTelemetry traces (Tempo/Jaeger UI) show every step
- Audit log shows every event the adapter emitted
- Prometheus metrics show health and throughput
- The control plane has a "replay investigation" feature (M5+) that can re-run an investigation from audit log

---

## Appendix A — Naming conventions

- Agent names: lowercase kebab-case, e.g. `holmesgpt`, `my-dba-agent`
- Capabilities: lowercase kebab-case, namespace with dot for custom (e.g. `mycompany.dba`)
- Custom config keys: camelCase (matches Kubernetes convention)
- Event types: lowercase dot-separated (e.g. `tool.request`)
- Error codes: lowercase snake_case (e.g. `budget_exhausted`)

## Appendix B — Open questions for SDK v2 (post v1.0)

These are deliberately punted to v2:

1. Sidecar adapter protocol (gRPC, allows non-Go agents)
2. Streaming response from sub-agents (current spec is request/response)
3. Skill discovery API (currently filesystem-based)
4. Multi-modal payloads (images, PDFs in alerts)
5. Long-lived investigations (>1 hour) with checkpointing
6. Federated agents across clusters

These are good ideas but not needed for v1.0. Validating the v1 contract in production is the priority.

---

*This SDK spec informs M1-M3 implementation. The v1 release of the SDK as a public Go module is part of M3 deliverables. Adapters can be written against this spec starting in M3.*
