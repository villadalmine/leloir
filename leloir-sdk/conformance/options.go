package conformance

import "time"

// Options configures the conformance suite.
//
// Pass to RunSuite to customize behavior. Use DefaultOptions() for sensible defaults.
type Options struct {
	// MockLLM, if true, indicates the adapter is configured to use a mock LLM
	// (the suite will not require real network access for LLM calls).
	// Implementations should set this in their adapter via CustomConfig.
	MockLLM bool

	// MockToolResponses are canned tool call results the conformance suite
	// will return when the adapter requests a tool. Keyed by tool name.
	MockToolResponses map[string]any

	// HealthCheckTimeout is how long the suite waits for HealthCheck to return.
	// Default: 5 seconds (matches the contract).
	HealthCheckTimeout time.Duration

	// ShutdownTimeout is how long the suite waits for Shutdown to return.
	// Default: 30 seconds.
	ShutdownTimeout time.Duration

	// CancellationGracePeriod is how long the suite waits for the adapter to
	// honor ctx.Done() and close the event channel. Default: 5 seconds.
	CancellationGracePeriod time.Duration

	// SmallBudget is the budget used in budget-exhaustion tests.
	// Default: tiny (forces immediate exhaustion).
	SmallBudget Budget

	// SkipPanicRecoveryTest disables the panic recovery test.
	// Some adapter authors prefer to test this manually.
	SkipPanicRecoveryTest bool

	// MaxConcurrentInvestigations is the parallelism for the concurrent-safety test.
	// Default: 5.
	MaxConcurrentInvestigations int
}

// Budget is re-exported from adapter to avoid forcing the user to import both.
// It exactly mirrors adapter.Budget.
type Budget struct {
	MaxTokens         int64
	MaxUSD            float64
	MaxToolCalls      int
	MaxSubInvocations int
}

// DefaultOptions returns sensible defaults for most adapters.
func DefaultOptions() Options {
	o := Options{}
	o.applyDefaults()
	return o
}

func (o *Options) applyDefaults() {
	if o.HealthCheckTimeout == 0 {
		o.HealthCheckTimeout = 5 * time.Second
	}
	if o.ShutdownTimeout == 0 {
		o.ShutdownTimeout = 30 * time.Second
	}
	if o.CancellationGracePeriod == 0 {
		o.CancellationGracePeriod = 5 * time.Second
	}
	if o.SmallBudget == (Budget{}) {
		// Tiny budget that should be exhausted on the first LLM call
		o.SmallBudget = Budget{
			MaxTokens:    10,
			MaxUSD:       0.0001,
			MaxToolCalls: 1,
		}
	}
	if o.MaxConcurrentInvestigations == 0 {
		o.MaxConcurrentInvestigations = 5
	}
	if o.MockToolResponses == nil {
		o.MockToolResponses = map[string]any{}
	}
}
