// Package registry tracks registered AgentAdapter instances.
//
// It watches AgentRegistration CRDs (M2+) and maintains a live map of
// available agents with their health status. The routing engine consults
// this registry when resolving alerts.
//
// For M1, adapters are in-process Go clients that implement the SDK interface.
// For M5+, sidecar adapters (gRPC) are also tracked here.
package registry

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	sdkadapter "github.com/leloir/sdk/adapter"
	"github.com/leloir/leloir/internal/store"
)

// AgentRegistry tracks all registered agents.
type AgentRegistry struct {
	store store.Store

	mu     sync.RWMutex
	agents map[string]*RegisteredAgent // keyed by agent name

	healthTicker *time.Ticker
}

// RegisteredAgent wraps a live AgentAdapter with metadata.
type RegisteredAgent struct {
	Name     string
	TenantID string
	Adapter  sdkadapter.AgentAdapter // in-process adapter
	Identity sdkadapter.AgentIdentity
	Config   sdkadapter.Config

	// Runtime state
	Health          Health
	LastHealthCheck time.Time
	ActiveCount     int32

	// A2A policy (from AgentRegistration CRD)
	CanInvoke    []string
	CannotInvoke []string
}

// Health status of an agent.
type Health string

const (
	HealthHealthy   Health = "healthy"
	HealthDegraded  Health = "degraded"
	HealthUnhealthy Health = "unhealthy"
	HealthUnknown   Health = "unknown"
)

// New constructs an empty registry.
func New(st store.Store) *AgentRegistry {
	return &AgentRegistry{
		store:  st,
		agents: make(map[string]*RegisteredAgent),
	}
}

// Register adds a new agent to the registry. Typically called when a
// matching AgentRegistration CRD appears in the cluster.
func (r *AgentRegistry) Register(ctx context.Context, name, tenantID string, a sdkadapter.AgentAdapter, cfg sdkadapter.Config, canInvoke, cannotInvoke []string) error {
	// Configure the adapter
	if err := a.Configure(ctx, cfg); err != nil {
		return fmt.Errorf("configure agent %q: %w", name, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents[name] = &RegisteredAgent{
		Name:         name,
		TenantID:     tenantID,
		Adapter:      a,
		Identity:     a.Identity(),
		Config:       cfg,
		Health:       HealthUnknown,
		CanInvoke:    canInvoke,
		CannotInvoke: cannotInvoke,
	}
	slog.Info("agent registered",
		"name", name,
		"tenant", tenantID,
		"version", a.Identity().Version,
	)
	return nil
}

// Deregister removes an agent. Typically called on AgentRegistration deletion.
func (r *AgentRegistry) Deregister(ctx context.Context, name string) error {
	r.mu.Lock()
	a, ok := r.agents[name]
	if ok {
		delete(r.agents, name)
	}
	r.mu.Unlock()

	if !ok {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := a.Adapter.Shutdown(shutdownCtx); err != nil {
		slog.Warn("agent shutdown error", "name", name, "error", err)
	}
	slog.Info("agent deregistered", "name", name)
	return nil
}

// Get returns a registered agent by name. Returns error if not found.
func (r *AgentRegistry) Get(name string) (*store.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found in registry", name)
	}

	// Return a simplified representation (not the actual SDK adapter)
	return &store.Agent{
		Name:     a.Name,
		TenantID: a.TenantID,
		Version:  a.Identity.Version,
		Health:   string(a.Health),
	}, nil
}

// Resolve returns the live adapter for a name (for the orchestrator to invoke).
func (r *AgentRegistry) Resolve(name string) (*RegisteredAgent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	if a.Health == HealthUnhealthy {
		return nil, fmt.Errorf("agent %q is unhealthy", name)
	}
	return a, nil
}

// List returns a snapshot of all registered agents.
func (r *AgentRegistry) List() []*store.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*store.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, &store.Agent{
			Name:     a.Name,
			TenantID: a.TenantID,
			Version:  a.Identity.Version,
			Health:   string(a.Health),
		})
	}
	return out
}

// CanInvoke returns true if caller is permitted by its AgentRegistration to
// invoke target. This is layer 1 of the A2A defense-in-depth (Q14b).
func (r *AgentRegistry) CanInvoke(caller, target string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.agents[caller]
	if !ok {
		return false
	}
	// Explicit deny wins
	for _, denied := range c.CannotInvoke {
		if denied == target || denied == "*" {
			return false
		}
	}
	// Allowlist: "*" or exact match
	for _, allowed := range c.CanInvoke {
		if allowed == "*" || allowed == target {
			return true
		}
	}
	return false
}

// Run starts background health check loop. Returns when ctx is cancelled.
func (r *AgentRegistry) Run(ctx context.Context) {
	r.healthTicker = time.NewTicker(30 * time.Second)
	defer r.healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.healthTicker.C:
			r.runHealthChecks(ctx)
		}
	}
}

func (r *AgentRegistry) runHealthChecks(ctx context.Context) {
	r.mu.RLock()
	snapshot := make([]*RegisteredAgent, 0, len(r.agents))
	for _, a := range r.agents {
		snapshot = append(snapshot, a)
	}
	r.mu.RUnlock()

	for _, a := range snapshot {
		hcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := a.Adapter.HealthCheck(hcCtx)
		cancel()

		r.mu.Lock()
		if err != nil {
			a.Health = HealthUnhealthy
			slog.Warn("agent unhealthy", "name", a.Name, "error", err)
		} else {
			a.Health = HealthHealthy
		}
		a.LastHealthCheck = time.Now()
		r.mu.Unlock()
	}
}
