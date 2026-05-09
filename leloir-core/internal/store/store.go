// Package store defines the persistence interface for Leloir.
//
// Two implementations:
//   - postgres: production
//   - memory:   tests, M0 PoC
//
// Use Open() to get a Store based on driver name.
package store

import (
	"context"
	"fmt"
	"time"
)

// Store is the persistence interface.
type Store interface {
	// Migrate applies any pending schema migrations.
	Migrate(ctx context.Context) error

	// Close releases resources.
	Close() error

	// Investigations
	CreateInvestigation(ctx context.Context, inv *Investigation) error
	GetInvestigation(ctx context.Context, id string) (*Investigation, error)
	ListInvestigations(ctx context.Context, tenantID string, limit int, offset int) ([]*Investigation, error)
	UpdateInvestigationStatus(ctx context.Context, id string, status string, reason string) error

	// Agents (cached view of AgentRegistration CRDs)
	UpsertAgent(ctx context.Context, a *Agent) error
	ListAgents(ctx context.Context, tenantID string) ([]*Agent, error)

	// Alert routes
	UpsertAlertRoute(ctx context.Context, r *AlertRoute) error
	ListAlertRoutes(ctx context.Context, tenantID string) ([]*AlertRoute, error)

	// Audit log
	InsertAuditEvent(ctx context.Context, evt AuditEvent) error
	PurgeAuditBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// Investigation is the DB row for an investigation.
type Investigation struct {
	ID          string
	ParentID    string
	TenantID    string
	AgentName   string
	Status      string // "running" | "completed" | "failed" | "cancelled"
	Outcome     string
	Reason      string
	TotalUSD    float64
	TotalTokens int64
	Started     time.Time
	Completed   *time.Time
}

// Agent is the cached view of an AgentRegistration.
type Agent struct {
	Name     string
	TenantID string
	Version  string
	Health   string
}

// AlertRoute is the DB row for an AlertRoute CRD.
type AlertRoute struct {
	Name             string
	Namespace        string
	TenantID         string
	Enabled          bool
	Priority         int32
	MatchLabels      map[string]string
	MatchSources     []string
	AgentName        string
	Team             []string
	AllowedSources   []string
	Skills           []string
	NotifyChannels   []string
	BudgetMaxUSD     float64
	BudgetMaxTokens  int64
	BudgetMaxToolCalls int32
	TimeoutMinutes   int32
}

// AuditEvent is the DB row for an audit log entry.
type AuditEvent struct {
	Type            string
	Timestamp       time.Time
	TenantID        string
	UserSubject     string
	InvestigationID string
	AgentName       string
	Details         map[string]any
	HashPrev        string // for hash chain (corporate profile)
	HashSelf        string
}

// Open returns a Store for the given driver.
func Open(driver, dsn string) (Store, error) {
	switch driver {
	case "memory":
		return newMemoryStore(), nil
	case "postgres":
		return nil, fmt.Errorf("postgres store not yet implemented — see internal/store/postgres (M1)")
	default:
		return nil, fmt.Errorf("unknown store driver %q", driver)
	}
}
