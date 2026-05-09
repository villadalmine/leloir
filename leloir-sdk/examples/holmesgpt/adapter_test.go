package holmesgpt_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leloir/sdk/adapter"
	"github.com/leloir/sdk/conformance"
	"github.com/leloir/sdk/examples/holmesgpt"
)

// TestConformance verifies the HolmesGPT adapter satisfies the AgentAdapter contract.
// It runs against a local mock HTTP server to avoid network dependencies.
func TestConformance(t *testing.T) {
	srv := newMockHolmesServer()
	defer srv.Close()

	a := holmesgpt.New()

	opts := conformance.DefaultOptions()
	// Provide a config with the mock server URL so Configure succeeds.
	opts.ConfigFactory = func() adapter.Config {
		cfg := conformance.SampleConfig()
		cfg.CustomConfig = map[string]any{
			"holmes": map[string]any{
				"apiBaseURL": srv.URL,
			},
		}
		return cfg
	}

	conformance.RunSuite(t, a, opts)
}

// newMockHolmesServer returns a test HTTP server that mimics Holmes's API.
func newMockHolmesServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))

		case "/api/chat":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := map[string]any{
				"analysis": "Mock root cause: container OOMKilled. Memory limit is too low. Increase resources.limits.memory to at least 512Mi.",
				"tool_calls": []map[string]any{
					{
						"tool_name":   "kubernetes_get_pod",
						"description": "kubectl get pod crash-test -n default -o json",
					},
					{
						"tool_name":   "kubernetes_count",
						"description": "kubectl get events -n default --field-selector reason=OOMKilling | wc -l",
					},
					{
						// This should be filtered out by the adapter
						"tool_name":   "TodoWrite",
						"description": "[{\"content\":\"Check memory metrics\",\"status\":\"in_progress\"}]",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
