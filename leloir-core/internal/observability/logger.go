// Package observability provides shared logging and OpenTelemetry setup.
package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/leloir/leloir/internal/config"
)

// NewLogger returns a structured JSON logger configured for the given level.
func NewLogger(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     lv,
		AddSource: false,
	}))
}

// ShutdownFunc is returned by InitOTel and should be deferred to flush traces.
type ShutdownFunc func(context.Context) error

// InitOTel initializes OpenTelemetry tracing.
// Returns a no-op shutdown if OTel is disabled.
func InitOTel(ctx context.Context, cfg config.ObservabilityConfig) (ShutdownFunc, error) {
	if !cfg.OTLP.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	// M2: set up actual OTel SDK with OTLP exporter pointing at cfg.OTLP.Endpoint
	// For M1 skeleton, return a no-op.
	slog.Info("OpenTelemetry init (skeleton — M2 wires up OTLP exporter)",
		"endpoint", cfg.OTLP.Endpoint,
	)
	return func(context.Context) error { return nil }, nil
}
