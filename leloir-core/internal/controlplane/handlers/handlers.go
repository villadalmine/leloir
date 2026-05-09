// Package handlers implements the HTTP/REST API of the Leloir control plane.
//
// Endpoints (versioned under /api/v1/):
//
//	GET    /healthz                         liveness
//	GET    /readyz                          readiness (store + registry ready)
//	POST   /api/v1/alerts                   ingest alert (from webhook receiver)
//	GET    /api/v1/investigations           list investigations (paginated)
//	GET    /api/v1/investigations/{id}      get one investigation
//	GET    /api/v1/investigations/{id}/stream  SSE live events
//	POST   /api/v1/investigations/{id}/cancel  cancel active investigation
//	GET    /api/v1/agents                   list registered agents
//	GET    /api/v1/agents/{name}            get one agent
//	GET    /api/v1/routes                   list alert routes
//	GET    /api/v1/mcp-servers              list registered MCP servers
//	GET    /api/v1/tenants                  list tenants (admin only)
//	GET    /api/v1/audit                    query audit log
//	GET    /metrics                         Prometheus metrics
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	sdkadapter "github.com/leloir/sdk/adapter"
	"github.com/gorilla/mux"
	"github.com/leloir/leloir/internal/controlplane/audit"
	"github.com/leloir/leloir/internal/controlplane/orchestrator"
	"github.com/leloir/leloir/internal/controlplane/registry"
	"github.com/leloir/leloir/internal/controlplane/routing"
	"github.com/leloir/leloir/internal/controlplane/stream"
	"github.com/leloir/leloir/internal/store"
)

// Deps bundles everything the handlers need.
type Deps struct {
	Store        store.Store
	Registry     *registry.AgentRegistry
	Router       *routing.Engine
	Orchestrator *orchestrator.Orchestrator
	Broker       *stream.Broker
	Audit        audit.Writer
}

// NewRouter constructs the HTTP router with all routes registered.
func NewRouter(deps Deps) http.Handler {
	r := mux.NewRouter()

	// Liveness / readiness
	r.HandleFunc("/healthz", healthHandler).Methods(http.MethodGet)
	r.HandleFunc("/readyz", readyHandlerBuilder(deps)).Methods(http.MethodGet)

	// API v1
	api := r.PathPrefix("/api/v1").Subrouter()

	// Apply middleware: auth, logging, tenant scoping
	api.Use(loggingMiddleware)
	api.Use(authMiddleware)
	api.Use(tenantScopeMiddleware)

	// Alert ingestion
	api.HandleFunc("/alerts", ingestAlertHandler(deps)).Methods(http.MethodPost)

	// Investigations
	api.HandleFunc("/investigations", listInvestigationsHandler(deps)).Methods(http.MethodGet)
	api.HandleFunc("/investigations/{id}", getInvestigationHandler(deps)).Methods(http.MethodGet)
	api.HandleFunc("/investigations/{id}/stream", streamInvestigationHandler(deps)).Methods(http.MethodGet)
	api.HandleFunc("/investigations/{id}/cancel", cancelInvestigationHandler(deps)).Methods(http.MethodPost)

	// Agents
	api.HandleFunc("/agents", listAgentsHandler(deps)).Methods(http.MethodGet)
	api.HandleFunc("/agents/{name}", getAgentHandler(deps)).Methods(http.MethodGet)

	// Routes
	api.HandleFunc("/routes", listRoutesHandler(deps)).Methods(http.MethodGet)

	// MCP servers
	api.HandleFunc("/mcp-servers", listMCPServersHandler(deps)).Methods(http.MethodGet)

	// Audit
	api.HandleFunc("/audit", queryAuditHandler(deps)).Methods(http.MethodGet)

	return r
}

// ─── alertPayload — the normalized alert format POSTed to /api/v1/alerts ──────
//
// This is the internal format produced by the webhook receiver. It can also be
// POSTed directly for testing (no Alertmanager required).

type alertPayload struct {
	Source      string            `json:"source"`      // e.g. "alertmanager"
	SourceID    string            `json:"sourceID"`    // fingerprint or ID
	Severity    string            `json:"severity"`    // "critical", "warning", "info"
	Title       string            `json:"title"`       // alertname
	Description string            `json:"description"` // summary annotation
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	FiredAt     int64             `json:"firedAt"` // unix seconds
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyHandlerBuilder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agents := deps.Registry.List()
		if len(agents) == 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "no agents registered",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ready",
			"agents": len(agents),
		})
	}
}

func ingestAlertHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload alertPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, fmt.Sprintf("invalid alert payload: %v", err), http.StatusBadRequest)
			return
		}
		if payload.Title == "" && payload.Source == "" {
			http.Error(w, "alert must have at least source and title", http.StatusBadRequest)
			return
		}

		tenantID := TenantFromContext(r.Context())

		alert := routing.Alert{
			ID:          fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			TenantID:    tenantID,
			Source:      payload.Source,
			SourceID:    payload.SourceID,
			Severity:    payload.Severity,
			Title:       payload.Title,
			Description: payload.Description,
			Labels:      payload.Labels,
			Annotations: payload.Annotations,
			FiredAt:     payload.FiredAt * 1000, // seconds → millis
		}

		match, err := deps.Router.Resolve(r.Context(), alert)
		if err != nil {
			if errors.Is(err, routing.ErrNoMatchingRoute) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
					"error": "no matching AlertRoute for this alert",
				})
				return
			}
			http.Error(w, fmt.Sprintf("routing error: %v", err), http.StatusInternalServerError)
			return
		}

		deadline := time.Now().Add(10 * time.Minute)
		if match.TimeoutMinutes > 0 {
			deadline = time.Now().Add(time.Duration(match.TimeoutMinutes) * time.Minute)
		}

		firedAt := time.Unix(payload.FiredAt, 0).UTC()
		if payload.FiredAt == 0 {
			firedAt = time.Now().UTC()
		}

		invID, err := deps.Orchestrator.StartInvestigation(r.Context(), orchestrator.StartRequest{
			TenantID:  tenantID,
			AgentName: match.Agent.Name,
			AlertContext: sdkadapter.AlertContext{
				Source:      payload.Source,
				SourceID:    payload.SourceID,
				Severity:    payload.Severity,
				Title:       payload.Title,
				Description: payload.Description,
				Labels:      payload.Labels,
				Annotations: payload.Annotations,
				FiredAt:     firedAt,
			},
			Budget: sdkadapter.Budget{
				MaxTokens:    match.BudgetTokens,
				MaxUSD:       match.BudgetUSD,
				MaxToolCalls: int(match.Route.BudgetMaxToolCalls),
			},
			Deadline: deadline,
			CallerContext: sdkadapter.CallerContext{
				Type:        "alert",
				UserSubject: UserFromContext(r.Context()),
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("start investigation: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]string{
			"investigation_id": invID,
			"agent":            match.Agent.Name,
			"stream":           fmt.Sprintf("/api/v1/investigations/%s/stream", invID),
		})
	}
}

func listInvestigationsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		limit := parseIntParam(r, "limit", 50)
		offset := parseIntParam(r, "offset", 0)

		invs, err := deps.Store.ListInvestigations(r.Context(), tenantID, limit, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf("list investigations: %v", err), http.StatusInternalServerError)
			return
		}
		if invs == nil {
			invs = []*store.Investigation{}
		}
		writeJSON(w, http.StatusOK, invs)
	}
}

func getInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		inv, err := deps.Store.GetInvestigation(r.Context(), vars["id"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, inv)
	}
}

func streamInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		investigationID := vars["id"]
		deps.Broker.Stream(r.Context(), investigationID, w)
	}
}

func cancelInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if err := deps.Orchestrator.Cancel(vars["id"]); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	}
}

func listAgentsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agents := deps.Registry.List()
		writeJSON(w, http.StatusOK, agents)
	}
}

func getAgentHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		agent, err := deps.Registry.Get(vars["name"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, agent)
	}
}

func listRoutesHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := TenantFromContext(r.Context())
		routes, err := deps.Store.ListAlertRoutes(r.Context(), tenantID)
		if err != nil {
			http.Error(w, fmt.Sprintf("list routes: %v", err), http.StatusInternalServerError)
			return
		}
		if routes == nil {
			routes = []*store.AlertRoute{}
		}
		writeJSON(w, http.StatusOK, routes)
	}
}

func listMCPServersHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: return MCPServers with health status
		writeJSON(w, http.StatusOK, []any{})
	}
}

func queryAuditHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M2: query audit log with filters
		w.WriteHeader(http.StatusNotImplemented)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func parseIntParam(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
