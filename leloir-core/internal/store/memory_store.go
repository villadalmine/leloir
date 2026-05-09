package store

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// memoryStore is a non-persistent, thread-safe in-memory Store.
// Useful for unit tests and the M0 PoC that doesn't need a database.
type memoryStore struct {
	mu              sync.RWMutex
	investigations  map[string]*Investigation
	agents          map[string]*Agent   // key: tenant+name
	alertRoutes     map[string]*AlertRoute // key: tenant+name
	auditEvents     []AuditEvent
}

func newMemoryStore() Store {
	return &memoryStore{
		investigations: make(map[string]*Investigation),
		agents:         make(map[string]*Agent),
		alertRoutes:    make(map[string]*AlertRoute),
		auditEvents:    make([]AuditEvent, 0, 1000),
	}
}

func (m *memoryStore) Migrate(ctx context.Context) error { return nil }
func (m *memoryStore) Close() error                       { return nil }

func (m *memoryStore) CreateInvestigation(ctx context.Context, inv *Investigation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.investigations[inv.ID]; exists {
		return fmt.Errorf("investigation %q already exists", inv.ID)
	}
	copy := *inv
	m.investigations[inv.ID] = &copy
	return nil
}

func (m *memoryStore) GetInvestigation(ctx context.Context, id string) (*Investigation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inv, ok := m.investigations[id]
	if !ok {
		return nil, fmt.Errorf("investigation %q not found", id)
	}
	copy := *inv
	return &copy, nil
}

func (m *memoryStore) ListInvestigations(ctx context.Context, tenantID string, limit, offset int) ([]*Investigation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Investigation, 0)
	for _, inv := range m.investigations {
		if inv.TenantID != tenantID {
			continue
		}
		out = append(out, inv)
	}
	// Simple pagination
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (m *memoryStore) UpdateInvestigationStatus(ctx context.Context, id, status, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.investigations[id]
	if !ok {
		return fmt.Errorf("investigation %q not found", id)
	}
	inv.Status = status
	inv.Reason = reason
	if status == "completed" || status == "failed" || status == "cancelled" {
		now := time.Now()
		inv.Completed = &now
	}
	return nil
}

func (m *memoryStore) UpsertAgent(ctx context.Context, a *Agent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := a.TenantID + "/" + a.Name
	copy := *a
	m.agents[key] = &copy
	return nil
}

func (m *memoryStore) ListAgents(ctx context.Context, tenantID string) ([]*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Agent, 0)
	for _, a := range m.agents {
		if a.TenantID != tenantID {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (m *memoryStore) UpsertAlertRoute(ctx context.Context, r *AlertRoute) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := r.TenantID + "/" + r.Name
	cp := *r
	m.alertRoutes[key] = &cp
	return nil
}

func (m *memoryStore) ListAlertRoutes(ctx context.Context, tenantID string) ([]*AlertRoute, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*AlertRoute, 0)
	for _, r := range m.alertRoutes {
		if r.TenantID != tenantID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *memoryStore) InsertAuditEvent(ctx context.Context, evt AuditEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditEvents = append(m.auditEvents, evt)
	return nil
}

func (m *memoryStore) PurgeAuditBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	before := len(m.auditEvents)
	kept := make([]AuditEvent, 0, before)
	for _, e := range m.auditEvents {
		if e.Timestamp.After(cutoff) {
			kept = append(kept, e)
		}
	}
	m.auditEvents = kept
	return int64(before - len(kept)), nil
}
