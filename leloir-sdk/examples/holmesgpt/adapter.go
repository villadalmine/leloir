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

// Adapter wraps HolmesGPT as a Leloir AgentAdapter.
type Adapter struct {
	config     adapter.Config
	holmesURL  string
	httpClient *http.Client
}

// Compile-time check.
var _ adapter.AgentAdapter = (*Adapter)(nil)

// New returns a new HolmesGPT adapter.
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
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
			"*", // Holmes accepts any MCP source
		},
		Tags: map[string]string{
			"license":  "Apache-2.0",
			"upstream": "https://github.com/HolmesGPT/holmesgpt",
			"category": "incident-response",
		},
	}
}

// Configure validates the config and establishes connection to HolmesGPT.
func (a *Adapter) Configure(ctx context.Context, config adapter.Config) error {
	holmesConfig, ok := config.CustomConfig["holmes"].(map[string]any)
	if !ok {
		return adapter.NewConfigError(
			"customConfig.holmes",
			"required (a map with apiBaseURL key)",
		)
	}

	apiURL, ok := holmesConfig["apiBaseURL"].(string)
	if !ok || apiURL == "" {
		return adapter.NewConfigError(
			"customConfig.holmes.apiBaseURL",
			"required (string)",
		)
	}

	a.config = config
	a.holmesURL = apiURL

	if config.ModelConfig.Endpoint == "" {
		return adapter.NewConfigError(
			"modelConfig.endpoint",
			"required",
		)
	}

	return nil
}

// HealthCheck verifies HolmesGPT is reachable.
func (a *Adapter) HealthCheck(ctx context.Context) error {
	if a.holmesURL == "" {
		return &adapter.AdapterError{
			Code:    adapter.ErrCodeUnhealthy,
			Message: "adapter not configured",
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
			Message: fmt.Sprintf("Holmes returned status %d", resp.StatusCode),
		}
	}
	return nil
}

// Investigate forwards the request to HolmesGPT and translates its streaming
// response into Leloir Events.
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

// Shutdown closes idle connections.
func (a *Adapter) Shutdown(ctx context.Context) error {
	a.httpClient.CloseIdleConnections()
	return nil
}

// runInvestigation is the core investigation loop. Separated from Investigate
// for clarity and panic-recovery wrapping.
func (a *Adapter) runInvestigation(
	ctx context.Context,
	req adapter.InvestigateRequest,
	ch chan<- adapter.Event,
	seq *adapter.SequenceCounter,
	budget *adapter.BudgetTracker,
) {
	// Build the prompt for Holmes
	holmesReq := buildHolmesRequest(req, a.config)

	// Call Holmes streaming API
	sseCh, err := a.callHolmesStreaming(ctx, holmesReq)
	if err != nil {
		sendError(ch, req.InvestigationID, seq, adapter.ErrCodeLLMUnavailable, err.Error(), false, ctx.Done())
		sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeError, "failed to call Holmes", 0, 0, ctx.Done())
		return
	}

	// Translate Holmes SSE events into Leloir Events
	for holmesEvt := range sseCh {
		// Check context cancellation between events
		select {
		case <-ctx.Done():
			sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeCancelled, "context cancelled", 0, 0, ctx.Done())
			return
		default:
		}

		// Translate based on event type
		leloirEvt, isFinal, ok := translateHolmesEvent(req.InvestigationID, seq, holmesEvt, budget)
		if !ok {
			continue // unknown event type, skip
		}

		if !adapter.SafeSendEvent(ch, leloirEvt, ctx.Done()) {
			return
		}

		// Check budget after every event that consumes resources
		if shouldRecord(holmesEvt) {
			if can, reason := budget.CanContinue(); !can {
				tokensUsed, usdSpent, _, _ := budget.Snapshot()
				sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeBudgetExhausted, reason, tokensUsed, usdSpent, ctx.Done())
				return
			}
			// Emit budget warnings if a threshold was crossed
			if resource, threshold := budget.CheckThreshold(); resource != "" {
				tokensUsed, usdSpent, _, _ := budget.Snapshot()
				_ = adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
					adapter.EventBudgetWarning,
					adapter.BudgetWarningPayload{
						Resource:    resource,
						UsedPercent: threshold,
						Used:        budgetResourceUsed(resource, tokensUsed, usdSpent),
						Limit:       budgetResourceLimit(resource, req.BudgetLimit),
					}), ctx.Done())
			}
		}

		if isFinal {
			tokensUsed, usdSpent, _, _ := budget.Snapshot()
			sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeSuccess, "", tokensUsed, usdSpent, ctx.Done())
			return
		}
	}

	// SSE stream ended without a final event — treat as no_result
	tokensUsed, usdSpent, _, _ := budget.Snapshot()
	sendComplete(ch, req.InvestigationID, seq, adapter.OutcomeNoResult, "Holmes stream ended without answer", tokensUsed, usdSpent, ctx.Done())
}

// recoverPanic is deferred in the investigation goroutine to ensure panics
// don't crash the adapter.
func recoverPanic(ch chan<- adapter.Event, investigationID string, seq *adapter.SequenceCounter) {
	if r := recover(); r != nil {
		// Best-effort: try to send error + complete events, but don't block
		// (since we may not have a healthy ctx anymore).
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

func budgetResourceUsed(resource string, tokens int64, usd float64) float64 {
	switch resource {
	case "tokens":
		return float64(tokens)
	case "usd":
		return usd
	}
	return 0
}

func budgetResourceLimit(resource string, b adapter.Budget) float64 {
	switch resource {
	case "tokens":
		return float64(b.MaxTokens)
	case "usd":
		return b.MaxUSD
	case "tool_calls":
		return float64(b.MaxToolCalls)
	}
	return 0
}
