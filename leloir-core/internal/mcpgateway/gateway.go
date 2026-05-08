// Package mcpgateway implements the MCP Gateway.
//
// The Gateway is the only path between adapters and MCP tool servers. It:
//   - Accepts HTTP/JSON-RPC (or HTTP/Streamable) from adapters on its inbound side
//   - Translates to the MCP server's native transport (gRPC, Streamable HTTP, SSE, stdio)
//   - Scopes each request to the caller's tenant (injects tenant labels, enforces allowlists)
//   - Injects credentials from Vault/K8s Secrets (agents never see raw creds)
//   - Audits every tool call
//   - Enforces rate limits per tenant
//   - Applies ApprovalPolicy for sensitive tools (pauses for HITL)
//
// Architecture (inbound):
//
//	adapter → POST /mcp/{tenant}/{mcp-server}/{tool} {args}
//	  ↓
//	Gateway validates: tenant, allowlist, rate limit, approval
//	  ↓
//	Gateway resolves MCPServer CRD → transport type
//	  ↓
//	Gateway injects credentials from secret
//	  ↓
//	Gateway makes outbound call (gRPC|HTTP|SSE|stdio)
//	  ↓
//	Gateway logs audit event
//	  ↓
//	Returns result to adapter
package mcpgateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/leloir/leloir/internal/config"
)

// Gateway is the MCP Gateway service.
type Gateway struct {
	cfg *config.MCPGatewayConfig

	// Transport clients (populated on first use)
	httpClient       *http.Client
	streamableClient *http.Client
	// grpcClient, stdioLauncher: M3+

	// Policy engine (rate limiting, approval, allowlist)
	policy PolicyEngine

	// Audit writer (via HTTP to control plane)
	auditClient *AuditClient

	httpSrv *http.Server
}

// PolicyEngine enforces tool call policies.
// Pluggable so tests can stub it.
type PolicyEngine interface {
	// Allow checks if a given (tenant, mcp-server, tool) call is permitted.
	Allow(ctx context.Context, req *ToolCallRequest) (*PolicyDecision, error)
}

// ToolCallRequest is what the adapter sends inbound.
type ToolCallRequest struct {
	TenantID     string
	MCPServer    string
	Tool         string
	Args         map[string]any
	CallerAgent  string
	CallerInvID  string // investigation ID for correlation
	Caller       string // user subject or "agent:<name>" for A2A
}

// PolicyDecision describes whether a call is allowed, and if so, under what terms.
type PolicyDecision struct {
	Allow           bool
	RequireApproval bool
	Reason          string
	TTLSeconds      int // for rate limit caching
}

// New constructs a Gateway.
func New(cfg *config.MCPGatewayConfig) (*Gateway, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	gw := &Gateway{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		streamableClient: &http.Client{
			Timeout: 120 * time.Second, // streaming can take longer
		},
		policy:      newDefaultPolicy(cfg),
		auditClient: newAuditClient(cfg.ControlPlaneURL),
	}

	gw.httpSrv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           gw.buildRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return gw, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (g *Gateway) Run(ctx context.Context) error {
	slog.Info("starting MCP Gateway", "addr", g.cfg.ListenAddr)

	errCh := make(chan error, 1)
	go func() {
		if err := g.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("MCP Gateway shutting down")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return g.httpSrv.Shutdown(shutdownCtx)
}

// buildRouter assembles the HTTP router for the gateway.
func (g *Gateway) buildRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/mcp/", g.handleToolCall) // /mcp/{tenant}/{server}/{tool}
	return mux
}

// handleToolCall is the main inbound handler. M1 will expand this significantly.
func (g *Gateway) handleToolCall(w http.ResponseWriter, r *http.Request) {
	// Parse path: /mcp/{tenant}/{server}/{tool}
	// M1: full parsing + validation
	// M1: call g.policy.Allow(ctx, req)
	// M1: resolve MCPServer CRD
	// M1: build outbound request, inject credentials
	// M1: dispatch to transport-specific client
	// M1: audit the call
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintln(w, "MCP Gateway: tool call path pending M1 implementation")
}

// ─── Default policy engine (M1 stub) ─────────────────────────────────────────

type defaultPolicy struct {
	cfg *config.MCPGatewayConfig
}

func newDefaultPolicy(cfg *config.MCPGatewayConfig) PolicyEngine {
	return &defaultPolicy{cfg: cfg}
}

func (p *defaultPolicy) Allow(ctx context.Context, req *ToolCallRequest) (*PolicyDecision, error) {
	// M1: check allowlist (from MCPServer CRD)
	// M1: check rate limit (bucket per tenant)
	// M4: check approval policy (return RequireApproval if needed)
	return &PolicyDecision{Allow: true}, nil
}

// ─── Audit client (writes audit events back to control plane) ───────────────

// AuditClient sends audit events to the control plane's internal audit endpoint.
type AuditClient struct {
	controlPlaneURL string
	httpClient      *http.Client
}

func newAuditClient(url string) *AuditClient {
	return &AuditClient{
		controlPlaneURL: url,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
	}
}

// Log records a tool call. Best-effort; logs but doesn't fail the call on audit error.
func (a *AuditClient) Log(ctx context.Context, event map[string]any) {
	// M1: POST to /internal/audit on control plane
	if a.controlPlaneURL == "" {
		slog.Debug("audit event dropped (no control plane URL)", "event", event)
		return
	}
	// M1 implementation pending
}
