// Package audit writes tamper-evident audit log entries.
//
// Levels (from v5.3 spec):
//   - hot tier: Postgres, 90d retention in corporate profile, 7d in local
//   - warm tier: object storage, 1y retention (corporate only)
//   - cold tier: WORM / Object Lock (corporate only, toggled by compliance regime)
//
// In corporate profile, each event is hash-chained with the previous event for
// tamper evidence. Hash chain is verified periodically by a background sweeper.
package audit

import (
	"context"
	"log/slog"
	"time"

	sdkadapter "github.com/leloir/sdk/adapter"
	"github.com/leloir/leloir/internal/config"
	"github.com/leloir/leloir/internal/store"
)

// EventType categorizes audit entries.
type EventType string

const (
	EventInvestigationStarted   EventType = "investigation.started"
	EventInvestigationCompleted EventType = "investigation.completed"
	EventInvestigationFailed    EventType = "investigation.failed"

	EventToolCallRequested EventType = "tool.call_requested"
	EventToolCallApproved  EventType = "tool.call_approved"
	EventToolCallDenied    EventType = "tool.call_denied"
	EventToolCallExecuted  EventType = "tool.call_executed"

	EventA2AInvocationRequested EventType = "a2a.invocation_requested"
	EventA2AInvocationApproved  EventType = "a2a.invocation_approved"
	EventA2AInvocationDenied    EventType = "a2a.invocation_denied"

	EventApprovalRequested EventType = "approval.requested"
	EventApprovalGranted   EventType = "approval.granted"
	EventApprovalDenied    EventType = "approval.denied"

	EventBudgetExceeded EventType = "budget.exceeded"
	EventLimitHit       EventType = "orchestration.limit_hit"
	EventCycleDetected  EventType = "orchestration.cycle_detected"

	EventAgentRegistered   EventType = "agent.registered"
	EventAgentDeregistered EventType = "agent.deregistered"
	EventAgentHealthChange EventType = "agent.health_changed"
)

// Event is a single audit log entry.
type Event struct {
	Type            EventType
	Timestamp       time.Time
	TenantID        string
	UserSubject     string // the authenticated user (if any)
	InvestigationID string
	AgentName       string
	Details         map[string]any
}

// Writer is the audit log interface.
type Writer interface {
	// Write records one arbitrary audit event.
	Write(ctx context.Context, evt Event) error

	// WriteEvent persists an SDK event as audit entry (convenience).
	WriteEvent(ctx context.Context, tenantID string, evt sdkadapter.Event) error

	// Run starts background workers (retention sweeper, hash chain verifier).
	Run(ctx context.Context)

	// Query returns recent audit entries with optional filters.
	Query(ctx context.Context, filter QueryFilter) ([]Event, error)
}

// QueryFilter describes audit query criteria.
type QueryFilter struct {
	TenantID        string
	InvestigationID string
	EventType       EventType
	Since           time.Time
	Until           time.Time
	Limit           int
}

// New constructs a Writer based on the audit config.
func New(cfg config.AuditConfig, st store.Store) (Writer, error) {
	// M1: simple Postgres-backed writer (no hash chain)
	// M2: add hash chain if cfg.HashChain.Enabled
	// M4: add warm tier upload if cfg.WarmStorage.Enabled
	return &defaultWriter{cfg: cfg, store: st}, nil
}

type defaultWriter struct {
	cfg   config.AuditConfig
	store store.Store
}

func (w *defaultWriter) Write(ctx context.Context, evt Event) error {
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	return w.store.InsertAuditEvent(ctx, store.AuditEvent{
		Type:            string(evt.Type),
		Timestamp:       evt.Timestamp,
		TenantID:        evt.TenantID,
		UserSubject:     evt.UserSubject,
		InvestigationID: evt.InvestigationID,
		AgentName:       evt.AgentName,
		Details:         evt.Details,
	})
}

func (w *defaultWriter) WriteEvent(ctx context.Context, tenantID string, sdkEvt sdkadapter.Event) error {
	return w.Write(ctx, Event{
		Type:            EventType("sdk." + string(sdkEvt.Type)),
		Timestamp:       sdkEvt.Timestamp,
		TenantID:        tenantID,
		InvestigationID: sdkEvt.CorrelationID,
		Details: map[string]any{
			"sequence":      sdkEvt.Sequence,
			"sdk_event_id":  sdkEvt.ID,
			"parent_id":     sdkEvt.ParentEventID,
			"payload":       sdkEvt.Payload,
		},
	})
}

func (w *defaultWriter) Query(ctx context.Context, filter QueryFilter) ([]Event, error) {
	// M2: translate filter to store query
	return nil, nil
}

func (w *defaultWriter) Run(ctx context.Context) {
	// M2: periodic retention sweep
	// M2: hash chain verification
	retentionTicker := time.NewTicker(1 * time.Hour)
	defer retentionTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retentionTicker.C:
			cutoff := time.Now().AddDate(0, 0, -w.cfg.HotRetentionDays)
			if n, err := w.store.PurgeAuditBefore(ctx, cutoff); err != nil {
				slog.Warn("audit retention purge failed", "error", err)
			} else if n > 0 {
				slog.Info("audit retention purge", "rows_removed", n, "cutoff", cutoff)
			}
		}
	}
}
