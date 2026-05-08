package adapter

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// BudgetTracker — thread-safe budget accounting helper for adapter authors
// ============================================================================

// BudgetTracker tracks resource usage against a Budget.
// Thread-safe; safe to call from multiple goroutines.
type BudgetTracker struct {
	mu              sync.Mutex
	limit           Budget
	tokensUsed      int64
	usdSpent        float64
	toolCallsUsed   int
	subInvocsUsed   int
	warningsEmitted map[string]bool
}

// NewBudgetTracker creates a tracker for the given budget.
func NewBudgetTracker(limit Budget) *BudgetTracker {
	return &BudgetTracker{
		limit:           limit,
		warningsEmitted: make(map[string]bool),
	}
}

// RecordLLMCall adds tokens and cost for one LLM call.
func (b *BudgetTracker) RecordLLMCall(input, output int, cost float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokensUsed += int64(input + output)
	b.usdSpent += cost
}

// RecordToolCall increments the tool call counter.
func (b *BudgetTracker) RecordToolCall() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.toolCallsUsed++
}

// RecordSubInvocation increments the sub-agent invocation counter.
func (b *BudgetTracker) RecordSubInvocation() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subInvocsUsed++
}

// CanContinue returns (true, "") if there is budget left, or (false, reason)
// if any limit has been hit.
func (b *BudgetTracker) CanContinue() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit.MaxTokens > 0 && b.tokensUsed >= b.limit.MaxTokens {
		return false, "max_tokens"
	}
	if b.limit.MaxUSD > 0 && b.usdSpent >= b.limit.MaxUSD {
		return false, "max_usd"
	}
	if b.limit.MaxToolCalls > 0 && b.toolCallsUsed >= b.limit.MaxToolCalls {
		return false, "max_tool_calls"
	}
	if b.limit.MaxSubInvocations > 0 && b.subInvocsUsed >= b.limit.MaxSubInvocations {
		return false, "max_sub_invocations"
	}
	return true, ""
}

// CheckThreshold returns the threshold (0.5, 0.8, 0.95) crossed since the last
// call, or 0 if no new threshold has been crossed. Thresholds for any single
// resource are emitted only once per BudgetTracker instance.
func (b *BudgetTracker) CheckThreshold() (resource string, threshold float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	checks := []struct {
		name  string
		used  float64
		limit float64
	}{
		{"usd", b.usdSpent, b.limit.MaxUSD},
		{"tokens", float64(b.tokensUsed), float64(b.limit.MaxTokens)},
		{"tool_calls", float64(b.toolCallsUsed), float64(b.limit.MaxToolCalls)},
	}

	for _, c := range checks {
		if c.limit <= 0 {
			continue
		}
		pct := c.used / c.limit
		for _, t := range []float64{0.95, 0.80, 0.50} {
			key := c.name + "_" + formatPct(t)
			if pct >= t && !b.warningsEmitted[key] {
				b.warningsEmitted[key] = true
				return c.name, t
			}
		}
	}
	return "", 0
}

// Snapshot returns a point-in-time view of usage.
func (b *BudgetTracker) Snapshot() (tokensUsed int64, usdSpent float64, toolCalls int, subInvocs int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokensUsed, b.usdSpent, b.toolCallsUsed, b.subInvocsUsed
}

func formatPct(t float64) string {
	switch t {
	case 0.5:
		return "50"
	case 0.8:
		return "80"
	case 0.95:
		return "95"
	default:
		return "other"
	}
}

// ============================================================================
// SequenceCounter — monotonic event sequence numbering
// ============================================================================

// SequenceCounter generates monotonic sequence numbers for events in one
// investigation. Thread-safe.
type SequenceCounter struct {
	mu  sync.Mutex
	seq int64
}

// NewSequenceCounter creates a counter starting at 0.
func NewSequenceCounter() *SequenceCounter {
	return &SequenceCounter{}
}

// Next returns the next sequence number.
func (s *SequenceCounter) Next() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

// ============================================================================
// Event helpers
// ============================================================================

// NewEvent constructs an Event with sensible defaults filled in.
// CorrelationID, Timestamp, ID, and Sequence are auto-populated.
// You provide Type and Payload (and optionally ParentEventID).
func NewEvent(investigationID string, seq int64, t EventType, payload EventPayload) Event {
	return Event{
		Type:          t,
		Timestamp:     time.Now().UTC(),
		Sequence:      seq,
		ID:            uuid.New().String(),
		CorrelationID: investigationID,
		Payload:       payload,
	}
}

// NewChildEvent is like NewEvent but sets ParentEventID.
func NewChildEvent(investigationID string, seq int64, parentEventID string, t EventType, payload EventPayload) Event {
	e := NewEvent(investigationID, seq, t, payload)
	e.ParentEventID = parentEventID
	return e
}

// SafeSendEvent sends an event to a channel, respecting ctx.Done() to avoid
// blocking if the consumer has gone away. Returns true if the event was sent.
func SafeSendEvent(ch chan<- Event, evt Event, doneCh <-chan struct{}) bool {
	select {
	case ch <- evt:
		return true
	case <-doneCh:
		return false
	}
}
