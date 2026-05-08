// Package adapter is the public API for building Leloir AgentAdapters.
//
// # Overview
//
// An AgentAdapter translates between the Leloir control plane and any
// AI agent (HolmesGPT, OpenCode, Hermes, custom). The interface is small —
// 5 methods — and the SDK pushes everything that's not protocol translation
// out of the adapter's concern: tool execution to the MCP Gateway, LLM calls
// to the LLM Gateway, audit/persistence to the control plane.
//
// # Quick start
//
//	package main
//
//	import (
//	    "context"
//	    "github.com/leloir/sdk/adapter"
//	)
//
//	type MyAdapter struct {
//	    config adapter.Config
//	}
//
//	var _ adapter.AgentAdapter = (*MyAdapter)(nil)
//
//	func (a *MyAdapter) Identity() adapter.AgentIdentity { ... }
//	func (a *MyAdapter) Configure(ctx context.Context, c adapter.Config) error { ... }
//	func (a *MyAdapter) HealthCheck(ctx context.Context) error { ... }
//	func (a *MyAdapter) Investigate(ctx context.Context, r adapter.InvestigateRequest) (<-chan adapter.Event, error) { ... }
//	func (a *MyAdapter) Shutdown(ctx context.Context) error { ... }
//
// # Helpers
//
// The package provides utilities for common patterns:
//
//   - BudgetTracker: thread-safe budget accounting with threshold detection
//   - SequenceCounter: monotonic event sequence numbers
//   - NewEvent / NewChildEvent: construct events with defaults filled in
//   - SafeSendEvent: send to a channel respecting cancellation
//   - NewConfigError / NewInternalError / NewBudgetExhaustedError: standard error builders
//
// # Conformance
//
// Use the github.com/leloir/sdk/conformance package to verify your adapter
// satisfies the contract:
//
//	func TestConformance(t *testing.T) {
//	    a := myadapter.New()
//	    conformance.RunSuite(t, a, conformance.DefaultOptions())
//	}
//
// # Examples
//
// See github.com/leloir/sdk/examples/minimal for a smallest-possible example
// and github.com/leloir/sdk/examples/holmesgpt for a real-world reference.
//
// # Versioning
//
// This SDK follows SemVer. The current version is exposed as adapter.SDKVersion.
// Breaking changes to the interface require a major version bump and a
// deprecation period of at least 2 minor versions.
package adapter
