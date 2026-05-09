// Package webhook implements the Alertmanager-compatible webhook receiver.
//
// It accepts webhooks, normalizes them into Leloir's internal Alert format,
// and forwards them to the control plane for routing.
//
// This is a thin separate process (not strictly required for M0-M1; the
// control plane can accept webhooks directly). It exists as a separate
// binary to allow:
//   - ingress hardening (the control plane can be on a more restricted network)
//   - horizontal scaling of webhook ingestion separately from orchestration
//   - format plugins (Slack, PagerDuty, etc.) without bloating the control plane
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/leloir/leloir/internal/config"
)

// Receiver is the webhook ingester.
type Receiver struct {
	cfg        *config.WebhookConfig
	httpSrv    *http.Server
	httpClient *http.Client
}

// New constructs a Receiver.
func New(cfg *config.WebhookConfig) (*Receiver, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	r := &Receiver{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.ForwardTimeout},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/webhook/alertmanager", r.handleAlertmanager)
	mux.HandleFunc("/webhook/slack", r.handleSlack)         // M5
	mux.HandleFunc("/webhook/pagerduty", r.handlePagerDuty) // M5

	r.httpSrv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return r, nil
}

// Run starts the HTTP server.
func (r *Receiver) Run(ctx context.Context) error {
	slog.Info("starting webhook receiver", "addr", r.cfg.ListenAddr)
	errCh := make(chan error, 1)
	go func() {
		if err := r.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return r.httpSrv.Shutdown(shutdownCtx)
}

// AlertmanagerPayload is the Alertmanager v4 webhook format (stripped).
type AlertmanagerPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []AlertmanagerAlert `json:"alerts"`
}

// AlertmanagerAlert is one alert from the AM payload.
type AlertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// handleAlertmanager ingests Alertmanager webhooks.
func (r *Receiver) handleAlertmanager(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, r.cfg.MaxRequestSize)

	var payload AlertmanagerPayload
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("decode: %v", err), http.StatusBadRequest)
		return
	}

	// Only act on firing alerts (ignore "resolved")
	for _, a := range payload.Alerts {
		if a.Status != "firing" {
			continue
		}
		// Normalize and forward
		if err := r.forward(req.Context(), normalizeAlert(a)); err != nil {
			slog.Error("forward failed", "error", err)
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// normalizeAlert converts an Alertmanager alert to Leloir's internal format.
func normalizeAlert(a AlertmanagerAlert) map[string]any {
	// The control plane's /api/v1/alerts expects this shape
	return map[string]any{
		"source":      "alertmanager",
		"sourceID":    a.Fingerprint,
		"severity":    a.Labels["severity"],
		"title":       a.Labels["alertname"],
		"description": a.Annotations["summary"],
		"labels":      a.Labels,
		"annotations": a.Annotations,
		"firedAt":     a.StartsAt.UTC().Unix(),
	}
}

// forward posts the normalized alert to the control plane.
func (r *Receiver) forward(ctx context.Context, alert map[string]any) error {
	body, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.cfg.ForwardTo+"/api/v1/alerts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	return nil
}

// Stubs for M5 format plugins
func (r *Receiver) handleSlack(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
func (r *Receiver) handlePagerDuty(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
