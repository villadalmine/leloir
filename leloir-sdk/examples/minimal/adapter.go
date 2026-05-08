// Package minimal demonstrates the smallest possible working AgentAdapter.
// It does not call any LLM or tool — it just simulates an investigation flow
// to show the protocol shape.
//
// Use this as a starting template for new adapters. Copy this file, then
// progressively replace the simulation with calls to your real agent.
package minimal

import (
	"context"
	"time"

	"github.com/leloir/sdk/adapter"
)

// Adapter is a minimal AgentAdapter implementation.
type Adapter struct {
	config adapter.Config
}

// Compile-time check that we implement the interface.
var _ adapter.AgentAdapter = (*Adapter)(nil)

// New returns a new minimal adapter.
func New() *Adapter {
	return &Adapter{}
}

// Identity returns metadata about this agent.
func (a *Adapter) Identity() adapter.AgentIdentity {
	return adapter.AgentIdentity{
		Name:        "minimal",
		Version:     "0.1.0",
		SDKVersion:  adapter.SDKVersion,
		Description: "Minimal reference adapter for learning the AgentAdapter contract.",
		Capabilities: []adapter.Capability{
			adapter.CapabilityGeneric,
		},
		SupportedSourceTypes: []string{"*"},
		Tags: map[string]string{
			"category": "reference",
			"license":  "Apache-2.0",
		},
	}
}

// Configure validates and stores the configuration.
func (a *Adapter) Configure(ctx context.Context, config adapter.Config) error {
	if config.TenantID == "" {
		return adapter.NewConfigError("tenantID", "required")
	}
	if config.ModelConfig.Endpoint == "" {
		return adapter.NewConfigError("modelConfig.endpoint", "required")
	}
	a.config = config
	return nil
}

// HealthCheck always returns nil (we have no external dependency).
func (a *Adapter) HealthCheck(ctx context.Context) error {
	return nil
}

// Investigate simulates an investigation. In a real adapter, this is where
// you call your actual AI agent.
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

	ch := make(chan adapter.Event, 64)
	seq := adapter.NewSequenceCounter()
	budget := adapter.NewBudgetTracker(req.BudgetLimit)

	go func() {
		// Always close the channel and recover from panic
		defer close(ch)
		defer func() {
			if r := recover(); r != nil {
				_ = adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
					adapter.EventComplete,
					adapter.CompletePayload{
						Outcome: adapter.OutcomeError,
						Reason:  "internal panic recovered",
					}), ctx.Done())
			}
		}()

		// 1. Send a thought
		if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
			adapter.EventThought,
			adapter.ThoughtPayload{
				Content: "Starting investigation of: " + req.AlertContext.Title,
			}), ctx.Done()) {
			return
		}

		// 2. Simulate an LLM call (record it for budget)
		budget.RecordLLMCall(50, 30, 0.001)
		if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
			adapter.EventLLMCall,
			adapter.LLMCallPayload{
				Model:        a.config.ModelConfig.Model,
				InputTokens:  50,
				OutputTokens: 30,
				CostUSD:      0.001,
				DurationMS:   150,
			}), ctx.Done()) {
			return
		}

		// 3. Check budget — if exhausted, complete with budget_exhausted
		if ok, reason := budget.CanContinue(); !ok {
			adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
				adapter.EventComplete,
				adapter.CompletePayload{
					Outcome: adapter.OutcomeBudgetExhausted,
					Reason:  reason,
				}), ctx.Done())
			return
		}

		// 4. Check deadline
		if time.Now().After(req.Deadline) {
			adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
				adapter.EventComplete,
				adapter.CompletePayload{
					Outcome: adapter.OutcomeDeadlineExceeded,
				}), ctx.Done())
			return
		}

		// 5. Check context cancellation
		select {
		case <-ctx.Done():
			adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
				adapter.EventComplete,
				adapter.CompletePayload{
					Outcome: adapter.OutcomeCancelled,
				}), ctx.Done())
			return
		default:
		}

		// 6. Send the answer
		tokensUsed, usdSpent, _, _ := budget.Snapshot()
		if !adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
			adapter.EventAnswer,
			adapter.AnswerPayload{
				Summary:        "Synthetic analysis (minimal adapter): " + req.AlertContext.Title,
				RootCause:      "This is a reference implementation; no real analysis performed.",
				Recommendation: "Replace this adapter with a real agent integration.",
				Confidence:     0.0,
			}), ctx.Done()) {
			return
		}

		// 7. Send complete
		adapter.SafeSendEvent(ch, adapter.NewEvent(req.InvestigationID, seq.Next(),
			adapter.EventComplete,
			adapter.CompletePayload{
				Outcome:     adapter.OutcomeSuccess,
				TotalCost:   usdSpent,
				TotalTokens: tokensUsed,
			}), ctx.Done())
	}()

	return ch, nil
}

// Shutdown releases any resources. For the minimal adapter, there's nothing to do.
func (a *Adapter) Shutdown(ctx context.Context) error {
	return nil
}
