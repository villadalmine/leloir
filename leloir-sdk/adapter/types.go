package adapter

import (
	"time"
)

// ============================================================================
// Identity
// ============================================================================

// AgentIdentity is metadata about an agent. Returned by AgentAdapter.Identity().
type AgentIdentity struct {
	// Name is the unique identifier for this agent type, e.g. "holmesgpt".
	// Lowercase, kebab-case. Must match RFC 1123 label format.
	Name string

	// Version is the SemVer of this adapter implementation, e.g. "1.4.2".
	Version string

	// SDKVersion is the AgentAdapter contract version this adapter implements.
	// Should equal the SDKVersion constant from this package.
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
	CapabilityKubernetes Capability = "kubernetes"
	CapabilityPrometheus Capability = "prometheus-alerts"
	CapabilityDatabase   Capability = "database-incidents"
	CapabilityNetwork    Capability = "network"
	CapabilitySecurity   Capability = "security-incidents"
	CapabilityCost       Capability = "cost-optimization"
	CapabilityCompliance Capability = "compliance"
	CapabilityGeneric    Capability = "generic"
	// Custom capabilities are allowed; namespace with a dot, e.g. "mycompany.dba"
)

// ============================================================================
// Configuration
// ============================================================================

// Config is the merged configuration passed to AgentAdapter.Configure().
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

	// SkillsPath is a read-only filesystem mount path containing
	// resolved skills (from SkillSource CRDs). Adapter loads from here.
	SkillsPath string

	// CustomConfig is adapter-specific settings from
	// AgentRegistration.spec.customConfig (passthrough YAML/JSON).
	CustomConfig map[string]any

	// SecretsPath is a read-only mount of secrets the adapter may need.
	// Most adapters need NONE — credentials are injected by gateways.
	// Exception: some adapters need a startup token to register with their
	// own runtime.
	SecretsPath string

	// ObservabilityConfig is OpenTelemetry endpoint info.
	ObservabilityConfig ObservabilityConfig
}

// ModelConfig describes the LLM endpoint the adapter should use.
// The adapter calls this endpoint as if it were an OpenAI-compatible API;
// the LLM Gateway handles provider translation.
type ModelConfig struct {
	// Provider key configured in LLM Gateway, e.g. "azure-openai-corp".
	Provider string
	// Model name as known by the provider, e.g. "gpt-4o".
	Model string
	// Endpoint is the LLM Gateway URL the adapter calls. OpenAI-compatible.
	Endpoint string
	// APIKey is what the adapter sends as Authorization header to the gateway.
	// The gateway translates this to the real provider credential.
	APIKey string
	// MaxTokensPerCall is a hint for the agent's per-call cap.
	MaxTokensPerCall int
}

// ObservabilityConfig holds OpenTelemetry settings.
type ObservabilityConfig struct {
	OTLPEndpoint string
	ServiceName  string
	Environment  string // "production", "staging", "local"
}

// ============================================================================
// Investigation Request
// ============================================================================

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
	// Adapter still has access to all skills via Config.SkillsPath.
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

// CallerContext describes who initiated an investigation.
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

// SkillRef is a reference to a loaded skill.
type SkillRef struct {
	// Name of the skill, e.g. "investigate-db-incident".
	Name string
	// Source is where it came from, e.g. "company-sre-skills@v1.2.3".
	Source string
	// Content is the SKILL.md content (already loaded for convenience).
	Content string
}

// Budget caps for an investigation.
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

// ============================================================================
// Events (the streaming protocol)
// ============================================================================

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

// EventType discriminates the kind of event.
type EventType string

const (
	// EventThought: the agent's reasoning step. Content is natural language.
	EventThought EventType = "thought"

	// EventToolCallRequest: the agent wants to call a tool.
	// The control plane forwards to MCP Gateway.
	EventToolCallRequest EventType = "tool.request"

	// EventToolCallResponse: result of a tool call (success or error).
	// Generated by the control plane, NOT by the adapter.
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
	// For fatal errors, the adapter should also send EventComplete.
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
	eventPayload() // sealed
}

// ----------------------------------------------------------------------------
// Concrete payload types
// ----------------------------------------------------------------------------

// ThoughtPayload is the agent's natural language reasoning step.
type ThoughtPayload struct {
	Content string
}

func (ThoughtPayload) eventPayload() {}

// ToolCallRequestPayload is a tool invocation request from the agent.
type ToolCallRequestPayload struct {
	ToolName string
	Args     map[string]any
	Reason   string // optional natural-language justification
}

func (ToolCallRequestPayload) eventPayload() {}

// ToolCallResponsePayload is the result of a tool call (delivered to the adapter).
type ToolCallResponsePayload struct {
	RequestEventID string
	Success        bool
	Result         any    // tool-specific
	Error          string // populated if !Success
	DurationMS     int64
}

func (ToolCallResponsePayload) eventPayload() {}

// LLMCallPayload records an LLM API call for cost tracking.
type LLMCallPayload struct {
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMS   int64
	CacheHit     bool
}

func (LLMCallPayload) eventPayload() {}

// SubAgentRequestPayload requests invocation of another agent (A2A).
type SubAgentRequestPayload struct {
	TargetAgent string  // agent name, e.g. "security-specialist"
	Context     string  // what the caller wants the sub-agent to know
	Ask         string  // the specific question/task
	BudgetHint  *Budget // optional, caller-suggested budget ceiling
	Reason      string  // why the agent decided to invoke this
}

func (SubAgentRequestPayload) eventPayload() {}

// SubAgentResponsePayload is the result of a sub-agent invocation.
type SubAgentResponsePayload struct {
	RequestEventID     string
	SubInvestigationID string
	Success            bool
	Answer             string
	Error              string
	CostUSD            float64
	DurationMS         int64
}

func (SubAgentResponsePayload) eventPayload() {}

// ApprovalRequestPayload signals the need for human approval before an action.
type ApprovalRequestPayload struct {
	Action      string // what's being approved, e.g. "kubernetes.exec"
	Args        map[string]any
	Rationale   string
	BlastRadius string // human-readable scope
}

func (ApprovalRequestPayload) eventPayload() {}

// ProgressPayload is a heartbeat with no new factual info.
type ProgressPayload struct {
	Message     string
	PercentDone float64 // 0.0-1.0, optional (-1 if unknown)
}

func (ProgressPayload) eventPayload() {}

// PartialAnswerPayload streams tokens of the final answer.
type PartialAnswerPayload struct {
	Token string
}

func (PartialAnswerPayload) eventPayload() {}

// AnswerPayload is the final structured answer.
type AnswerPayload struct {
	Summary        string         // 1-3 sentence executive summary
	RootCause      string         // identified root cause
	Recommendation string         // recommended action(s)
	Confidence     float64        // 0.0-1.0
	Evidence       []EvidenceItem // links to data that support the conclusion
	StructuredData map[string]any // adapter-specific extras
}

func (AnswerPayload) eventPayload() {}

// EvidenceItem is one piece of evidence supporting the answer.
type EvidenceItem struct {
	Source      string // e.g. "prometheus", "kubernetes"
	Description string // e.g. "CPU saturation in pod X over 15 minutes"
	Reference   string // e.g. URL or query that produced this evidence
}

// ErrorPayload reports a non-fatal error.
type ErrorPayload struct {
	Code        string // see errors.go for canonical codes
	Message     string
	Recoverable bool // if true, the agent will continue
	Details     map[string]any
}

func (ErrorPayload) eventPayload() {}

// BudgetWarningPayload signals approaching budget limit.
type BudgetWarningPayload struct {
	Resource    string  // "tokens", "usd", "tool_calls", "sub_invocations"
	UsedPercent float64 // 0.50, 0.80, 0.95
	Used        float64
	Limit       float64
}

func (BudgetWarningPayload) eventPayload() {}

// CompletePayload is sent as the final event. Always last.
type CompletePayload struct {
	Outcome     CompleteOutcome
	TotalCost   float64
	TotalTokens int64
	Reason      string // why investigation ended
}

func (CompletePayload) eventPayload() {}

// CompleteOutcome enumerates how an investigation ended.
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
