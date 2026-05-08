package holmesgpt

import (
	"github.com/leloir/sdk/adapter"
)

// translateHolmesEvent converts one Holmes SSE event to a Leloir Event.
// Returns:
//   - the translated event
//   - isFinal: true if this is the final answer (after which complete should be sent)
//   - ok: false if the event type is unknown and should be skipped
func translateHolmesEvent(
	investigationID string,
	seq *adapter.SequenceCounter,
	h holmesEvent,
	budget *adapter.BudgetTracker,
) (adapter.Event, bool, bool) {
	switch h.Type {
	case "thought":
		return adapter.NewEvent(investigationID, seq.Next(),
			adapter.EventThought,
			adapter.ThoughtPayload{
				Content: h.Content,
			}), false, true

	case "tool_call":
		budget.RecordToolCall()
		return adapter.NewEvent(investigationID, seq.Next(),
			adapter.EventToolCallRequest,
			adapter.ToolCallRequestPayload{
				ToolName: h.ToolName,
				Args:     h.ToolArgs,
			}), false, true

	case "llm_call":
		budget.RecordLLMCall(h.InputTokens, h.OutputTokens, h.Cost)
		return adapter.NewEvent(investigationID, seq.Next(),
			adapter.EventLLMCall,
			adapter.LLMCallPayload{
				Model:        h.Model,
				InputTokens:  h.InputTokens,
				OutputTokens: h.OutputTokens,
				CostUSD:      h.Cost,
			}), false, true

	case "answer":
		return adapter.NewEvent(investigationID, seq.Next(),
			adapter.EventAnswer,
			adapter.AnswerPayload{
				Summary:        h.Summary,
				RootCause:      h.RootCause,
				Recommendation: h.Recommendation,
				Confidence:     h.Confidence,
			}), true, true // isFinal=true

	case "error":
		return adapter.NewEvent(investigationID, seq.Next(),
			adapter.EventError,
			adapter.ErrorPayload{
				Code:        adapter.ErrCodeAgentInternal,
				Message:     h.Error,
				Recoverable: false,
			}), false, true

	default:
		// Unknown event type — skip
		return adapter.Event{}, false, false
	}
}

// shouldRecord returns true if the Holmes event represents resource consumption
// that affects the budget.
func shouldRecord(h holmesEvent) bool {
	switch h.Type {
	case "llm_call", "tool_call":
		return true
	}
	return false
}
