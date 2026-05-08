package conformance

import (
	"time"

	"github.com/leloir/sdk/adapter"
)

// SampleConfig returns a Config that adapters can use to satisfy Configure.
// Adapters that need adapter-specific CustomConfig should override the
// CustomConfig field as needed in their tests.
func SampleConfig() adapter.Config {
	return adapter.Config{
		TenantID: "conformance-tenant",
		ModelConfig: adapter.ModelConfig{
			Provider:         "mock",
			Model:            "mock-model",
			Endpoint:         "http://mock-llm-gateway:8080",
			APIKey:           "mock-api-key",
			MaxTokensPerCall: 1000,
		},
		MCPGatewayEndpoint: "http://mock-mcp-gateway:8080",
		SkillsPath:         "/tmp/conformance-skills",
		SecretsPath:        "/tmp/conformance-secrets",
		CustomConfig:       map[string]any{},
		ObservabilityConfig: adapter.ObservabilityConfig{
			OTLPEndpoint: "http://mock-otel:4317",
			ServiceName:  "conformance-test",
			Environment:  "test",
		},
	}
}

// SampleRequest returns an InvestigateRequest suitable for conformance tests.
// Use opts to override fields for specific test scenarios.
func SampleRequest(opts ...func(*adapter.InvestigateRequest)) adapter.InvestigateRequest {
	req := adapter.InvestigateRequest{
		InvestigationID: "inv-conformance-" + time.Now().Format("20060102150405"),
		TenantID:        "conformance-tenant",
		AlertContext: adapter.AlertContext{
			Source:      "alertmanager",
			SourceID:    "test-alert-1",
			Severity:    "warning",
			Title:       "Test alert",
			Description: "A synthetic alert for conformance testing.",
			Labels: map[string]string{
				"alertname": "TestAlert",
				"severity":  "warning",
			},
			FiredAt: time.Now().UTC(),
		},
		AvailableTools: []string{
			"kubernetes.get_pod",
			"prometheus.query",
		},
		BudgetLimit: adapter.Budget{
			MaxTokens:         100000,
			MaxUSD:            5.0,
			MaxToolCalls:      50,
			MaxSubInvocations: 10,
		},
		Deadline: time.Now().Add(5 * time.Minute),
		CallerContext: adapter.CallerContext{
			Type: "alert",
		},
	}

	for _, opt := range opts {
		opt(&req)
	}
	return req
}

// WithSmallBudget is an option for SampleRequest to use a tiny budget.
func WithSmallBudget(b Budget) func(*adapter.InvestigateRequest) {
	return func(r *adapter.InvestigateRequest) {
		r.BudgetLimit = adapter.Budget{
			MaxTokens:         b.MaxTokens,
			MaxUSD:            b.MaxUSD,
			MaxToolCalls:      b.MaxToolCalls,
			MaxSubInvocations: b.MaxSubInvocations,
		}
	}
}

// WithDeadline is an option for SampleRequest to override the deadline.
func WithDeadline(d time.Time) func(*adapter.InvestigateRequest) {
	return func(r *adapter.InvestigateRequest) {
		r.Deadline = d
	}
}

// WithInvestigationID is an option for SampleRequest to set an explicit ID.
func WithInvestigationID(id string) func(*adapter.InvestigateRequest) {
	return func(r *adapter.InvestigateRequest) {
		r.InvestigationID = id
	}
}
