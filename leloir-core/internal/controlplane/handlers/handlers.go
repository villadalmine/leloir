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
	"net/http"

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

// ─── Handlers (skeletons — M1 fills these in) ────────────────────────────────

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyHandlerBuilder(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: check store connectivity and registry status
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func ingestAlertHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: parse alert, call router, kick off orchestrator
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func listInvestigationsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: query store.ListInvestigations with tenant scope + pagination
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func getInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: fetch one investigation with events
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func streamInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		investigationID := vars["id"]
		// M1: subscribe to broker, stream via SSE
		deps.Broker.Stream(r.Context(), investigationID, w)
	}
}

func cancelInvestigationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: call orchestrator.Cancel(investigationID)
		w.WriteHeader(http.StatusNotImplemented)
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
		// M1: return AlertRoutes with stats
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func listMCPServersHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// M1: return MCPServers with health status
		w.WriteHeader(http.StatusNotImplemented)
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
