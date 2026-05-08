// Package adapter defines the AgentAdapter interface that all Leloir agents implement.
//
// An adapter is a thin Go translation layer (~150-300 lines) between the Leloir
// control plane and any AI agent. Implementing this interface makes any agent —
// open source or proprietary, Go or sidecar-wrapped — a first-class citizen of
// the Leloir platform.
//
// The interface is deliberately small (5 methods). Everything that doesn't belong
// in the contract is pushed elsewhere: tool execution lives in the MCP Gateway,
// LLM calls go through the LLM Gateway, audit logging is the control plane's
// responsibility, etc. The adapter only translates.
//
// See https://github.com/leloir/leloir/blob/main/docs/agentadapter-sdk-spec-v1.md
// for the full design rationale.
package adapter

import (
	"context"
)

// AgentAdapter is the single contract between Leloir and any AI agent.
// Implementations may be in-process Go (recommended) or sidecar gRPC (M5+).
//
// Concurrency: implementations MUST be safe for concurrent calls to Investigate.
// Configure is never called during active investigations (the control plane
// serializes lifecycle events).
type AgentAdapter interface {
	// Identity returns metadata about the agent. Called once at registration.
	// Must be deterministic and side-effect-free.
	Identity() AgentIdentity

	// Configure is called when the agent is registered or its config changes.
	// Receives the merged configuration from AgentRegistration CRD.
	// Must be idempotent. Returns an *AdapterError if the config is invalid.
	Configure(ctx context.Context, config Config) error

	// HealthCheck returns nil if the agent is ready to handle Investigate calls.
	// Called periodically (default: every 30s) by the control plane.
	// A failure removes the agent from active routing until healthy again.
	// Must complete in under 5 seconds.
	HealthCheck(ctx context.Context) error

	// Investigate is the core method. It receives an investigation request
	// and returns a channel of events. The adapter MUST close the channel
	// when investigation is complete or context is cancelled.
	//
	// The adapter MUST honor:
	//   - ctx cancellation (terminate within 5s)
	//   - req.BudgetLimit (stop calling LLM/tools when exhausted)
	//   - req.AvailableTools (only request tools in this list)
	//   - req.Deadline (terminate at or before this time)
	//
	// Returns an error only if the request itself is invalid (e.g., empty
	// InvestigationID). Once the channel is returned, all subsequent failures
	// are reported via Event{Type: EventError} or Event{Type: EventComplete}.
	Investigate(ctx context.Context, req InvestigateRequest) (<-chan Event, error)

	// Shutdown is called when the agent is being deregistered or the
	// control plane is shutting down. The adapter must release all resources.
	// In-flight investigations should be cancelled gracefully.
	// Must complete in under 30 seconds.
	Shutdown(ctx context.Context) error
}

// SDKVersion is the version of this SDK package. Adapters report this in
// their AgentIdentity so the control plane can verify compatibility.
const SDKVersion = "1.0.0"
