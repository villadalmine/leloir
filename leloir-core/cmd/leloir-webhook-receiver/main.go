// Command leloir-webhook-receiver is the thin webhook ingester for Leloir.
//
// It accepts webhooks from external sources (Alertmanager primarily) and
// forwards them to the control plane's internal API. Running it as a
// separate process lets the control plane sit behind strict auth while
// webhooks enter through a more permissive edge.
//
// Responsibilities:
//   - Parse Alertmanager webhook format (plus others via plugins)
//   - Normalize alerts into Leloir's internal Alert type
//   - Apply ingress filters (dedup, drop noise)
//   - Forward to control plane /internal/alerts endpoint
//
// For M0/M1, this binary is optional — the control plane can accept
// webhooks directly. This separate receiver exists for M4+ where
// ingress hardening becomes important.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/leloir/leloir/internal/config"
	"github.com/leloir/leloir/internal/observability"
	"github.com/leloir/leloir/internal/webhook"
)

var (
	configPath = flag.String("config", "/etc/leloir/webhook.yaml", "Path to config YAML")
	logLevel   = flag.String("log-level", "info", "Log level")
	version    = "dev"
)

func main() {
	flag.Parse()

	logger := observability.NewLogger(*logLevel)
	slog.SetDefault(logger)
	slog.Info("starting leloir-webhook-receiver", "version", version)

	cfg, err := config.LoadWebhook(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rcv, err := webhook.New(cfg)
	if err != nil {
		slog.Error("failed to construct webhook receiver", "error", err)
		os.Exit(1)
	}

	if err := rcv.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("webhook receiver exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("leloir-webhook-receiver exited cleanly")
}
