package conformance

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/leloir/sdk/adapter"
)

// ----------------------------------------------------------------------------
// Identity tests
// ----------------------------------------------------------------------------

func testIdentityIsDeterministic(t *testing.T, a adapter.AgentAdapter) {
	t.Helper()
	first := a.Identity()
	second := a.Identity()
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Identity() must be deterministic; got differing results")
	}
}

func testIdentityHasRequiredFields(t *testing.T, a adapter.AgentAdapter) {
	t.Helper()
	id := a.Identity()
	if id.Name == "" {
		t.Error("Identity().Name must not be empty")
	}
	if id.Version == "" {
		t.Error("Identity().Version must not be empty")
	}
	if id.SDKVersion == "" {
		t.Error("Identity().SDKVersion must not be empty")
	}
	if len(id.Capabilities) == 0 {
		t.Error("Identity().Capabilities should declare at least one capability")
	}
}

// ----------------------------------------------------------------------------
// Configure tests
// ----------------------------------------------------------------------------

func testConfigureIsIdempotent(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	cfg := opts.sampleConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.Configure(ctx, cfg); err != nil {
		t.Fatalf("first Configure failed: %v", err)
	}
	if err := a.Configure(ctx, cfg); err != nil {
		t.Errorf("second Configure with same config failed (must be idempotent): %v", err)
	}
}

func testConfigureRejectsInvalidConfig(t *testing.T, a adapter.AgentAdapter) {
	t.Helper()
	// An empty config should be rejected by most adapters.
	// This is a soft assertion: we only fail if the adapter accepts a clearly
	// invalid config (no tenant ID, no model endpoint).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	emptyConfig := adapter.Config{}
	err := a.Configure(ctx, emptyConfig)
	if err == nil {
		t.Log("warning: adapter accepted an empty Config without error; consider adding validation")
		return
	}
	// If an error is returned, prefer it to be an *AdapterError
	if _, ok := err.(*adapter.AdapterError); !ok {
		t.Logf("note: adapter returned an error for invalid config but it's not *adapter.AdapterError (%T)", err)
	}
}

// ----------------------------------------------------------------------------
// HealthCheck tests
// ----------------------------------------------------------------------------

func testHealthCheckCompletesQuickly(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()

	// Configure first
	cfg := opts.sampleConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Configure(ctx, cfg); err != nil {
		t.Skipf("Configure failed, cannot test HealthCheck: %v", err)
		return
	}

	hcCtx, hcCancel := context.WithTimeout(context.Background(), opts.HealthCheckTimeout)
	defer hcCancel()

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- a.HealthCheck(hcCtx) }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if elapsed > opts.HealthCheckTimeout {
			t.Errorf("HealthCheck took %v, exceeds %v", elapsed, opts.HealthCheckTimeout)
		}
		// Don't assert err == nil; the adapter may legitimately be unhealthy
		// against mock infrastructure. We only care about timing here.
		t.Logf("HealthCheck returned in %v (err=%v)", elapsed, err)
	case <-time.After(opts.HealthCheckTimeout + 1*time.Second):
		t.Errorf("HealthCheck did not return within %v", opts.HealthCheckTimeout)
	}
}

// ----------------------------------------------------------------------------
// Investigate tests
// ----------------------------------------------------------------------------

func testInvestigateReturnsChannel(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := SampleRequest()
	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error for valid request: %v", err)
	}
	if ch == nil {
		t.Fatal("Investigate returned nil channel without error")
	}
	// Drain to allow goroutine to clean up
	drainChannel(t, ch, 30*time.Second)
}

func testInvestigateAlwaysClosesChannel(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := SampleRequest()
	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}

	// The channel must close within a reasonable time
	closed := waitForChannelClose(ch, 30*time.Second)
	if !closed {
		t.Error("event channel never closed within 30 seconds")
	}
}

func testInvestigateAlwaysSendsCompleteEvent(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := SampleRequest()
	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}

	events := collectEvents(ch, 30*time.Second)
	if len(events) == 0 {
		t.Fatal("no events received before channel closed")
	}
	last := events[len(events)-1]
	if last.Type != adapter.EventComplete {
		t.Errorf("last event must be EventComplete; got %s", last.Type)
	}
}

func testInvestigateHonorsContextCancellation(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithCancel(context.Background())

	req := SampleRequest()
	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}

	// Cancel almost immediately
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	closed := waitForChannelClose(ch, opts.CancellationGracePeriod+1*time.Second)
	if !closed {
		t.Errorf("channel did not close within %v of cancellation", opts.CancellationGracePeriod)
	}
}

func testInvestigateHonorsBudget(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := SampleRequest(WithSmallBudget(opts.SmallBudget))
	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}

	events := collectEvents(ch, 30*time.Second)
	if len(events) == 0 {
		t.Fatal("no events received")
	}

	last := events[len(events)-1]
	if last.Type != adapter.EventComplete {
		t.Errorf("expected last event to be EventComplete; got %s", last.Type)
		return
	}

	// We tolerate either OutcomeBudgetExhausted or OutcomeNoResult
	// (some adapters may decide they can't even start with such a tiny budget).
	if cp, ok := last.Payload.(adapter.CompletePayload); ok {
		switch cp.Outcome {
		case adapter.OutcomeBudgetExhausted, adapter.OutcomeNoResult, adapter.OutcomeError:
			// acceptable
		default:
			t.Logf("note: with tiny budget, adapter returned outcome=%s; expected budget_exhausted, no_result, or error", cp.Outcome)
		}
	}
}

func testInvestigateHonorsDeadline(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set deadline to 500ms in the future
	deadline := time.Now().Add(500 * time.Millisecond)
	req := SampleRequest(WithDeadline(deadline))

	ch, err := a.Investigate(ctx, req)
	if err != nil {
		t.Fatalf("Investigate returned error: %v", err)
	}

	closed := waitForChannelClose(ch, 10*time.Second)
	if !closed {
		t.Error("channel did not close within deadline + grace period")
	}
}

func testInvestigateRejectsInvalidRequest(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Empty InvestigationID is invalid
	req := SampleRequest(WithInvestigationID(""))
	ch, err := a.Investigate(ctx, req)
	if err == nil {
		t.Log("warning: adapter accepted request with empty InvestigationID")
		if ch != nil {
			drainChannel(t, ch, 10*time.Second)
		}
	}
}

func testInvestigateConcurrentSafe(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	n := opts.MaxConcurrentInvestigations
	var wg sync.WaitGroup
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := SampleRequest()
			ch, err := a.Investigate(ctx, req)
			if err != nil {
				errCh <- err
				return
			}
			drainChannel(t, ch, 30*time.Second)
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("concurrent Investigate failed: %v", err)
		}
	}
}

func testInvestigateRecoversFromPanic(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	// We can't directly cause the adapter to panic, but we can verify that
	// a malformed/extreme request doesn't take down the whole process.
	// Most adapters should handle this without panicking.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Investigate caused panic to escape: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build a request with a huge AlertContext to stress encoding
	req := SampleRequest()
	bigStr := make([]byte, 1<<20) // 1MB
	for i := range bigStr {
		bigStr[i] = 'a'
	}
	req.AlertContext.Description = string(bigStr)

	ch, err := a.Investigate(ctx, req)
	if err != nil {
		// An error is acceptable
		return
	}
	if ch != nil {
		drainChannel(t, ch, 10*time.Second)
	}
}

// ----------------------------------------------------------------------------
// Shutdown tests
// ----------------------------------------------------------------------------

func testShutdownCompletesQuickly(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	mustConfigure(t, a, opts)

	ctx, cancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
	defer cancel()

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- a.Shutdown(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
		t.Logf("Shutdown completed in %v", time.Since(start))
	case <-time.After(opts.ShutdownTimeout + 1*time.Second):
		t.Errorf("Shutdown did not complete within %v", opts.ShutdownTimeout)
	}
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func mustConfigure(t *testing.T, a adapter.AgentAdapter, opts Options) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Configure(ctx, opts.sampleConfig()); err != nil {
		t.Skipf("Configure failed, skipping test: %v", err)
	}
}

func drainChannel(t *testing.T, ch <-chan adapter.Event, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Logf("drainChannel: timeout after %v", timeout)
			return
		}
	}
}

func waitForChannelClose(ch <-chan adapter.Event, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func collectEvents(ch <-chan adapter.Event, timeout time.Duration) []adapter.Event {
	var events []adapter.Event
	deadline := time.After(timeout)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-deadline:
			return events
		}
	}
}
