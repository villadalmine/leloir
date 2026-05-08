// Command leloir-controlplane is the main Leloir control plane binary.
//
// It handles:
//   - HTTP/gRPC API for the UI
//   - CRD reconciliation (watches AgentRegistration, AlertRoute, etc.)
//   - Alert ingestion (via webhook receiver or direct)
//   - Investigation orchestration (AgentAdapter lifecycle)
//   - A2A invocation control
//   - Event streaming to UI via SSE
//   - Audit log writes
//
// Usage:
//
//	leloir-controlplane --config /etc/leloir/config.yaml
//
// See docs/configuration.md for the full config schema.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/leloir/leloir/internal/config"
	"github.com/leloir/leloir/internal/controlplane/server"
	"github.com/leloir/leloir/internal/observability"
)

var (
	configPath = flag.String("config", "/etc/leloir/config.yaml", "Path to config YAML")
	logLevel   = flag.String("log-level", "info", "Log level: debug, info, warn, error")
	version    = "dev" // set at build time via -ldflags
)

func main() {
	flag.Parse()

	logger := observability.NewLogger(*logLevel)
	slog.SetDefault(logger)

	slog.Info("starting leloir-controlplane", "version", version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err, "path", *configPath)
		os.Exit(1)
	}

	// Graceful shutdown context
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize OpenTelemetry
	shutdownOtel, err := observability.InitOTel(ctx, cfg.Observability)
	if err != nil {
		slog.Error("failed to init OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer shutdownOtel(context.Background())

	// Build and run the control plane
	srv, err := server.New(cfg)
	if err != nil {
		slog.Error("failed to construct server", "error", err)
		os.Exit(1)
	}

	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("leloir-controlplane exited cleanly")
}
