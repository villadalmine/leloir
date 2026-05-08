// Package routing implements the alert → AlertRoute → agent resolution.
//
// The routing engine evaluates an incoming Alert against all active
// AlertRoute CRDs in priority order and returns the best match.
//
// Matching is label-based (the simplest, most debuggable model). A route
// matches if:
//   1. It's enabled
//   2. All labels in spec.match.labels exist on the alert with equal values
//   3. The alert source is in spec.match.sources (or sources is empty)
//
// Priority: higher spec.priority evaluated first. Ties broken by name (stable).
package routing

import (
	"context"
	"fmt"
	"sort"

	"github.com/leloir/leloir/internal/controlplane/registry"
	"github.com/leloir/leloir/internal/store"
)

// Alert is the internal normalized alert representation.
type Alert struct {
	ID          string
	TenantID    string
	Source      string            // e.g. "alertmanager"
	SourceID    string
	Severity    string
	Title       string
	Description string
	Labels      map[string]string
	Annotations map[string]string
	FiredAt     int64 // unix millis
}

// Match describes the resolved routing decision.
type Match struct {
	Route          *store.AlertRoute
	Agent          *store.Agent
	AllowedSources []string
	Skills         []string
	NotifyChannels []string
	BudgetUSD      float64
	BudgetTokens   int64
	TimeoutMinutes int32
}

// Engine routes alerts to AlertRoutes.
type Engine struct {
	store    store.Store
	registry *registry.AgentRegistry
}

// New constructs a routing Engine.
func New(st store.Store, reg *registry.AgentRegistry) *Engine {
	return &Engine{store: st, registry: reg}
}

// Resolve returns the best-matching route for the alert, or an error if none.
// Returns ErrNoMatchingRoute if no AlertRoute matches.
func (e *Engine) Resolve(ctx context.Context, alert Alert) (*Match, error) {
	routes, err := e.store.ListAlertRoutes(ctx, alert.TenantID)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}

	// Filter enabled + matching routes
	candidates := make([]*store.AlertRoute, 0, len(routes))
	for _, r := range routes {
		if !r.Enabled {
			continue
		}
		if !matchLabels(r.MatchLabels, alert.Labels) {
			continue
		}
		if !matchSources(r.MatchSources, alert.Source) {
			continue
		}
		candidates = append(candidates, r)
	}

	if len(candidates) == 0 {
		return nil, ErrNoMatchingRoute
	}

	// Sort by priority desc, name asc (stable)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].Name < candidates[j].Name
	})

	best := candidates[0]

	// Resolve the agent reference
	agent, err := e.registry.Get(best.AgentName)
	if err != nil {
		return nil, fmt.Errorf("resolve agent %q for route %q: %w",
			best.AgentName, best.Name, err)
	}

	return &Match{
		Route:          best,
		Agent:          agent,
		AllowedSources: best.AllowedSources,
		Skills:         best.Skills,
		NotifyChannels: best.NotifyChannels,
		BudgetUSD:      best.BudgetMaxUSD,
		BudgetTokens:   best.BudgetMaxTokens,
		TimeoutMinutes: best.TimeoutMinutes,
	}, nil
}

// ─── Label matching helpers ──────────────────────────────────────────────────

func matchLabels(required, actual map[string]string) bool {
	for k, v := range required {
		if actual[k] != v {
			return false
		}
	}
	return true
}

func matchSources(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true // no restriction
	}
	for _, s := range allowed {
		if s == actual {
			return true
		}
	}
	return false
}

// ErrNoMatchingRoute is returned when no AlertRoute matches an alert.
var ErrNoMatchingRoute = fmt.Errorf("no matching AlertRoute for alert")
