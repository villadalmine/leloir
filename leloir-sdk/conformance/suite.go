// Package conformance provides a test suite that any AgentAdapter implementation
// can run to verify it satisfies the contract.
//
// Usage in adapter test file:
//
//	func TestConformance(t *testing.T) {
//	    a := myadapter.New()
//	    conformance.RunSuite(t, a, conformance.DefaultOptions())
//	}
//
// The suite verifies critical contract properties: channel discipline, context
// cancellation, budget enforcement, panic recovery, and more.
package conformance

import (
	"testing"

	"github.com/leloir/sdk/adapter"
)

// RunSuite executes all conformance tests against the given adapter.
// Tests that are not applicable for the given Options will be skipped.
func RunSuite(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()

	if a == nil {
		t.Fatal("conformance: adapter is nil")
	}

	opts.applyDefaults()

	// Identity tests don't need configure
	t.Run("Identity_IsDeterministic", func(t *testing.T) {
		testIdentityIsDeterministic(t, a)
	})
	t.Run("Identity_HasRequiredFields", func(t *testing.T) {
		testIdentityHasRequiredFields(t, a)
	})

	// Configuration tests
	t.Run("Configure_IsIdempotent", func(t *testing.T) {
		testConfigureIsIdempotent(t, a, opts)
	})
	t.Run("Configure_RejectsInvalidConfig", func(t *testing.T) {
		testConfigureRejectsInvalidConfig(t, a)
	})

	// Health check tests
	t.Run("HealthCheck_CompletesQuickly", func(t *testing.T) {
		testHealthCheckCompletesQuickly(t, a, opts)
	})

	// Investigation tests (the bulk)
	t.Run("Investigate_ReturnsChannel", func(t *testing.T) {
		testInvestigateReturnsChannel(t, a, opts)
	})
	t.Run("Investigate_AlwaysClosesChannel", func(t *testing.T) {
		testInvestigateAlwaysClosesChannel(t, a, opts)
	})
	t.Run("Investigate_AlwaysSendsCompleteEvent", func(t *testing.T) {
		testInvestigateAlwaysSendsCompleteEvent(t, a, opts)
	})
	t.Run("Investigate_HonorsContextCancellation", func(t *testing.T) {
		testInvestigateHonorsContextCancellation(t, a, opts)
	})
	t.Run("Investigate_HonorsBudget", func(t *testing.T) {
		testInvestigateHonorsBudget(t, a, opts)
	})
	t.Run("Investigate_HonorsDeadline", func(t *testing.T) {
		testInvestigateHonorsDeadline(t, a, opts)
	})
	t.Run("Investigate_RejectsInvalidRequest", func(t *testing.T) {
		testInvestigateRejectsInvalidRequest(t, a, opts)
	})
	t.Run("Investigate_ConcurrentSafe", func(t *testing.T) {
		testInvestigateConcurrentSafe(t, a, opts)
	})
	t.Run("Investigate_RecoversFromPanic", func(t *testing.T) {
		if opts.SkipPanicRecoveryTest {
			t.Skip("panic recovery test disabled in options")
		}
		testInvestigateRecoversFromPanic(t, a, opts)
	})

	// Lifecycle
	t.Run("Shutdown_CompletesQuickly", func(t *testing.T) {
		testShutdownCompletesQuickly(t, a, opts)
	})
}
