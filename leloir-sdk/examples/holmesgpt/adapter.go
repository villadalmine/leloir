// Package holmesgpt is a reference AgentAdapter implementation for HolmesGPT,
// the CNCF Sandbox AI agent for cloud-native incident analysis.
//
// This is the canonical example of how to wrap an existing HTTP-based AI agent
// with the Leloir AgentAdapter contract.
//
// Reference: https://github.com/HolmesGPT/holmesgpt
package holmesgpt

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/leloir/sdk/adapter"
)

// buildVersion is set at compile time via -ldflags.
var buildVersion = "0.1.0-dev"

// maxToolDescLen is the maximum length of a tool call description emitted as
// a thought event. Mirrors the PoC behavior (120 chars + ellipsis).
const maxToolDescLen = 120

// Adapter wraps HolmesGPT as a Leloir AgentAdapter.
type Adapter struct {
	config     adapter.Config
	holmesURL  string
	httpClient *http.Client
}

// Compile-time check.
var _ adapter.AgentAdapter = (*Adapter)(nil)

// New returns a new HolmesGPT adapter.
// The HTTP client timeout is 5 minutes to match holmesApiTimeout in the Helm chart.
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Identity returns metadata about HolmesGPT.
func (a *Adapter) Identity() adapter.AgentIdentity {
	return adapter.AgentIdentity{
		Name:        "holmesgpt",
		Version:     buildVersion,
		SDKVersion:  adapter.SDKVersion,
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
			"*",
		},
		Tags: map[string]string{
			"license":  "Apache-2.0",
			"upstream": "https://github.com/HolmesGPT/holmesgpt",
			"category": "incident-response",
		},
	}
}

// Configure validates config and stores the Holmes API base URL.
// Required: customConfig.holmes.apiBaseURL (string).
// Optional: modelConfig.model (Holmes model alias, e.g. "gemma4-31b").
func (a *Adapter) Configure(ctx context.Context, config adapter.Config) error {
	holmesConfig, ok := config.CustomConfig["holmes"].(map[string]any)
	if !ok {
		return adapter.NewConfigError(
			"customConfig.holmes",
			"required (map with apiBaseURL key)",
		)
	}

	apiURL, ok := holmesConfig["apiBaseURL"].(string)
	if !ok || apiURL == "" {
		return adapter.NewConfigError(
			"customConfig.holmes.apiBaseURL",
			"required (string, e.g. http://holmesgpt-holmes.holmesgpt:80)",
		)
	}

	a.config = config
	a.holmesURL = apiURL
	return nil
}

// HealthCheck verifies HolmesGPT is reachable via GET /health.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.holmesURL == "" {
		return &adapter.AdapterError{
			Code:    adapter.ErrCodeUnhealthy,
			Message: "adapter not configured (call Configure first)",
		}
	}

	hcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(hcCtx, http.MethodGet, a.holmesURL+"/health", nil)
	if err != nil {
		return &adapter.AdapterError{
			Code:    adapter.ErrCodeUnhealthy,
			Message: "failed to construct health request",
			Cause:   err,
		}
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return &adapter.AdapterError{
			Code:    adapter.ErrCodeUnhealthy,
			Message: "Holmes API unreachable",
			Cause:   err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &adapter.AdapterError{
			Code:    adapter.ErrCodeUnhealthy,
			Message: fmt.Sprintf("Holmes health check returned %d", resp.StatusCode),
		}
	}
	return nil
}

// Investigate calls HolmesGPT synchronously and streams the translated events.
// Holmes returns a single JSON response; the adapter converts it to Leloir events.
func (a *Adapter) Investigate(
	ctx context.Context,
	req adapter.InvestigateRequest,
) (<-chan adapter.Event, error) {
	if req.InvestigationID == "" {
		return nil, &adapter.AdapterError{
			Code:    adapter.ErrCodeInvalidConfig,
			Message: "InvestigationID required",
		}
	}
	if a.holmesURL == "" {
		return nil, &adapter.AdapterError{
			Code:    adapter.ErrCodeInvalidConfig,
			Message: "adapter not configured (call Configure first)",
		}
	}

	ch := make(chan adapter.Event, 64)
	seq := adapter.NewSequenceCounter()
	budget := adapter.NewBudgetTracker(req.BudgetLimit)

	go func() {
		defer close(ch)
		defer recoverPanic(ch, req.InvestigationID, seq)
		a.runInvestigation(ctx, req, ch, seq, budget)
	}()

	return ch, nil
}

// Shutdown closes idle HTTP connections.
func (a *Adapter) Shutdown(ctx context.Context) error {
	a.httpClient.CloseIdleConnections()
	return nil
}

// runInvestigation calls Holmes and converts the response to Leloir events.
func (a *Adapter) runInvestigation(
	ctx context.Context,
	req adapter.InvestigateRequest,
	ch chan<- adapter.Event,
	seq *adapter.SequenceCounter,
	budget *adapter.BudgetTracker,
) {
	// Announce the investigation start
	if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
		adapter.EventThought,
		adapter.ThoughtPayload{
			Content: "Investigating: " + req.AlertContext.Title,
		}), ctx.Done()) {
		return
	}

	// Call Holmes (synchronous — see CONTRACT.md)
	holmesReq := buildHolmesRequest(req, a.config)
	hr, err := a.callHolmes(ctx, holmesReq)
	if err != nil {
		sendError(ch, req.InvestigationID, seq, adapter.ErrCodeLLMUnavailable, err.Error(), false, ctx.Done())
		sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeError, err.Error(), 0, 0, ctx.Done())
		return
	}

	// Emit tool call thoughts (filter Holmes internal bookkeeping tools)
	for _, tc := range hr.ToolCalls {
		if tc.ToolName == "TodoWrite" || tc.ToolName == "TodoRead" {
			continue
		}
		select {
		case <-ctx.Done():
			sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeCancelled, "context cancelled", 0, 0, ctx.Done())
			return
		default:
		}
		desc := tc.Description
		if len(desc) > maxToolDescLen {
			desc = desc[:maxToolDescLen] + "…"
		}
		budget.RecordToolCall()
		if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
			adapter.EventThought,
			adapter.ThoughtPayload{Content: tc.ToolName + ": " + desc}), ctx.Done()) {
			return
		}

		if ok, reason := budget.CanContinue(); !ok {
			tokensUsed, usdSpent, _, _ := budget.Snapshot()
			sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeBudgetExhausted, reason, tokensUsed, usdSpent, ctx.Done())
			return
		}
	}

	// Emit the final answer
	if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
		adapter.EventAnswer,
		adapter.AnswerPayload{
			Summary:    hr.Analysis,
			RootCause:  hr.Analysis,
			Confidence: 0.7,
		}), ctx.Done()) {
		return
	}

	tokensUsed, usdSpent, _, _ := budget.Snapshot()
	sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeSuccess, "", tokensUsed, usdSpent, ctx.Done())
}

// recoverPanic ensures adapter goroutine panics never crash the control plane.
func recoverPanic(ch chan<- adapter.Event, investigationID string, seq *adapter.SequenceCounter) {
	if r := recover(); r != nil {
		select {
		case ch <- adapter.NewEvent(investigationID, seq.Next(), adapter.EventError,
			adapter.ErrorPayload{
				Code:        adapter.ErrCodeAgentInternal,
				Message:     fmt.Sprintf("adapter panic: %v", r),
				Recoverable: false,
			}):
		default:
		}
		select {
		case ch <- adapter.NewEvent(investigationID, seq.Next(), adapter.EventComplete,
			adapter.CompletePayload{
				Outcome: adapter.OutcomeError,
				Reason:  "internal panic",
			}):
		default:
		}
	}
}

func sendError(ch chan<- adapter.Event, investigationID string, seq *adapter.SequenceCounter, code, message string, recoverable bool, doneCh <-chan struct{}) {
	adapter.SafeSendEvent(ch, adapter.NewEvent(investigationID, seq.Next(), adapter.EventError,
		adapter.ErrorPayload{
			Code:        code,
			Message:     message,
			Recoverable: recoverable,
		}), doneCh)
}

func sendComplete(ch chan<- adapter.Event, investigationID string, seq *adapter.SequenceCounter, outcome adapter.CompleteOutcome, reason string, tokens int64, cost float64, doneCh <-chan struct{}) {
	adapter.SafeSendEvent(ch, adapter.NewEvent(investigationID, seq.Next(), adapter.EventComplete,
		adapter.CompletePayload{
			Outcome:     outcome,
			Reason:      reason,
			TotalTokens: tokens,
			TotalCost:   cost,
		}), doneCh)
}
