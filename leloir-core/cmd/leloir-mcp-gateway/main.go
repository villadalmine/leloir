// Command leloir-mcp-gateway is the MCP Gateway binary.
//
// It sits between adapters and MCP servers. Responsibilities:
//   - Translate transports (HTTP in → gRPC/HTTP/SSE/stdio out)
//   - Per-tenant scoping (inject tenant labels, enforce allowlists)
//   - Credential injection (tenant asks for postgres, we add the right creds)
//   - Audit every tool call
//   - Rate limiting per tenant
//   - Enforce HITL approval policies on sensitive tools
//
// Agents never call MCP servers directly. Everything goes through this gateway.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/leloir/leloir/internal/config"
	"github.com/leloir/leloir/internal/mcpgateway"
	"github.com/leloir/leloir/internal/observability"
)

var (
	configPath = flag.String("config", "/etc/leloir/mcp-gateway.yaml", "Path to config YAML")
	logLevel   = flag.String("log-level", "info", "Log level")
	version    = "dev"
)

func main() {
	flag.Parse()

	logger := observability.NewLogger(*logLevel)
	slog.SetDefault(logger)
	slog.Info("starting leloir-mcp-gateway", "version", version)

	cfg, err := config.LoadMCPGateway(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	gw, err := mcpgateway.New(cfg)
	if err != nil {
		slog.Error("failed to construct MCP Gateway", "error", err)
		os.Exit(1)
	}

	if err := gw.Run(ctx); err != nil && err != context.Canceled {
		slog.Error("MCP Gateway exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("leloir-mcp-gateway exited cleanly")
}
