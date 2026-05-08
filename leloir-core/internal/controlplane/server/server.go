// Package server wires up the Leloir control plane.
//
// The server orchestrates the following subsystems:
//   - HTTP API (for UI)
//   - Alert ingestion
//   - Routing engine (alert → AlertRoute → agent)
//   - Agent registry (tracks AgentRegistration health)
//   - Investigation orchestrator (lifecycle of one investigation)
//   - Event streamer (SSE → UI)
//   - Audit log writer
//   - Kubernetes controller (watches CRDs)
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/leloir/leloir/internal/config"
	"github.com/leloir/leloir/internal/controlplane/audit"
	"github.com/leloir/leloir/internal/controlplane/handlers"
	"github.com/leloir/leloir/internal/controlplane/orchestrator"
	"github.com/leloir/leloir/internal/controlplane/registry"
	"github.com/leloir/leloir/internal/controlplane/routing"
	"github.com/leloir/leloir/internal/controlplane/stream"
	"github.com/leloir/leloir/internal/store"
)

// Server is the top-level control plane.
type Server struct {
	cfg *config.ControlPlaneConfig

	store    store.Store
	audit    audit.Writer
	registry *registry.AgentRegistry
	router   *routing.Engine
	orch     *orchestrator.Orchestrator
	stream   *stream.Broker

	httpSrv *http.Server
}

// New constructs a Server from config. All subsystems are created here,
// but nothing starts until Run is called.
func New(cfg *config.ControlPlaneConfig) (*Server, error) {
	// Store (persistence)
	st, err := store.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	// Audit writer
	aud, err := audit.New(cfg.Audit, st)
	if err != nil {
		return nil, fmt.Errorf("build audit writer: %w", err)
	}

	// Agent registry (tracks AgentRegistration CRDs)
	reg := registry.New(st)

	// Routing engine (alert → AlertRoute → agent)
	rt := routing.New(st, reg)

	// Event streamer (per-investigation SSE pub/sub)
	br := stream.NewBroker()

	// Orchestrator (runs an investigation end-to-end)
	orch := orchestrator.New(orchestrator.Config{
		Registry:         reg,
		Store:            st,
		Audit:            aud,
		Broker:           br,
		MCPGatewayURL:    cfg.MCPGateway.Endpoint,
		LLMGatewayURL:    cfg.LLMGateway.Endpoint,
	})

	// HTTP handlers
	mux := handlers.NewRouter(handlers.Deps{
		Store:        st,
		Registry:     reg,
		Router:       rt,
		Orchestrator: orch,
		Broker:       br,
		Audit:        aud,
	})

	httpSrv := &http.Server{
		Addr:              cfg.API.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Server{
		cfg:      cfg,
		store:    st,
		audit:    aud,
		registry: reg,
		router:   rt,
		orch:     orch,
		stream:   br,
		httpSrv:  httpSrv,
	}, nil
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	slog.Info("starting control plane",
		"profile", s.cfg.Profile,
		"httpAddr", s.cfg.API.HTTPAddr,
	)

	// Apply any pending migrations
	if err := s.store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate store: %w", err)
	}

	// Start background workers
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Registry health check loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.registry.Run(ctx)
	}()

	// Audit retention sweeper
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.audit.Run(ctx)
	}()

	// HTTP server
	wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()
		slog.Info("HTTP server listening", "addr", s.cfg.API.HTTPAddr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			cancel()
		}
	}()

	// Wait for shutdown signal or HTTP error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		slog.Error("HTTP server failed", "error", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("HTTP shutdown error", "error", err)
	}

	// Cancel workers and wait
	cancel()
	wg.Wait()

	if err := s.store.Close(); err != nil {
		slog.Warn("store close error", "error", err)
	}

	return ctx.Err()
}
