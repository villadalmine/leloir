package holmesgpt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/leloir/sdk/adapter"
)

// holmesAPIRequest is the payload Holmes expects on POST /api/chat.
type holmesAPIRequest struct {
	Ask            string            `json:"ask"`
	AvailableTools []string          `json:"available_tools,omitempty"`
	Stream         bool              `json:"stream"`
	ModelConfig    holmesModelConfig `json:"model_config,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
}

// holmesModelConfig is Holmes's representation of the LLM endpoint.
// It maps to OpenAI-compatible API config; the LLM Gateway speaks this dialect.
type holmesModelConfig struct {
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// holmesEvent is one SSE event from Holmes's stream.
// We accept a flexible shape to tolerate Holmes API evolution.
type holmesEvent struct {
	Type           string         `json:"type"`
	Content        string         `json:"content,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolArgs       map[string]any `json:"tool_args,omitempty"`
	Model          string         `json:"model,omitempty"`
	InputTokens    int            `json:"input_tokens,omitempty"`
	OutputTokens   int            `json:"output_tokens,omitempty"`
	Cost           float64        `json:"cost,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	RootCause      string         `json:"root_cause,omitempty"`
	Recommendation string         `json:"recommendation,omitempty"`
	Confidence     float64        `json:"confidence,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// buildHolmesRequest converts a Leloir InvestigateRequest into the format
// HolmesGPT's HTTP API expects.
func buildHolmesRequest(req adapter.InvestigateRequest, cfg adapter.Config) holmesAPIRequest {
	prompt := buildPrompt(req)
	return holmesAPIRequest{
		Ask:            prompt,
		AvailableTools: req.AvailableTools,
		Stream:         true,
		SessionID:      req.SessionID,
		ModelConfig: holmesModelConfig{
			Provider: "openai", // OpenAI-compatible (LLM Gateway speaks this)
			Endpoint: cfg.ModelConfig.Endpoint,
			APIKey:   cfg.ModelConfig.APIKey,
			Model:    cfg.ModelConfig.Model,
		},
	}
}

// buildPrompt assembles the prompt text for Holmes from the alert context.
// The alert content is wrapped in untrusted markers to defend against
// prompt injection.
func buildPrompt(req adapter.InvestigateRequest) string {
	var sb strings.Builder
	sb.WriteString("Investigate the following alert. Use available tools to gather evidence and determine root cause.\n\n")
	sb.WriteString("<alert>\n")
	sb.WriteString("source: ")
	sb.WriteString(req.AlertContext.Source)
	sb.WriteString("\nseverity: ")
	sb.WriteString(req.AlertContext.Severity)
	sb.WriteString("\ntitle: ")
	sb.WriteString(req.AlertContext.Title)
	sb.WriteString("\n\n<untrusted-content>\n")
	sb.WriteString(req.AlertContext.Description)
	sb.WriteString("\n</untrusted-content>\n")
	sb.WriteString("</alert>\n")

	if len(req.AlertContext.Labels) > 0 {
		sb.WriteString("\n<labels>\n")
		for k, v := range req.AlertContext.Labels {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
		sb.WriteString("</labels>\n")
	}

	for _, skill := range req.Skills {
		sb.WriteString("\n<skill name=\"")
		sb.WriteString(skill.Name)
		sb.WriteString("\">\n")
		sb.WriteString(skill.Content)
		sb.WriteString("\n</skill>\n")
	}

	return sb.String()
}

// callHolmesStreaming POSTs to Holmes's /api/chat endpoint and returns a
// channel of parsed SSE events. The channel is closed when the stream ends
// or an error occurs.
func (a *Adapter) callHolmesStreaming(ctx context.Context, req holmesAPIRequest) (<-chan holmesEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.holmesURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("construct request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call Holmes: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("Holmes returned status %d", resp.StatusCode)
	}

	out := make(chan holmesEvent, 32)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		parseSSE(ctx, resp.Body, out)
	}()
	return out, nil
}

// parseSSE reads Server-Sent Events from r and pushes parsed holmesEvents to out.
// Honors ctx cancellation.
func parseSSE(ctx context.Context, r io.Reader, out chan<- holmesEvent) {
	scanner := bufio.NewScanner(r)
	// Accept lines up to 1MB
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var dataBuf bytes.Buffer

	flush := func() {
		if dataBuf.Len() == 0 {
			return
		}
		var evt holmesEvent
		if err := json.Unmarshal(dataBuf.Bytes(), &evt); err == nil {
			select {
			case out <- evt:
			case <-ctx.Done():
			}
		}
		dataBuf.Reset()
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		if line == "" {
			// Blank line marks end of event
			flush()
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			dataBuf.WriteString(line[len("data: "):])
		} else if strings.HasPrefix(line, "data:") {
			dataBuf.WriteString(line[len("data:"):])
		}
		// Lines that don't start with "data:" (event:, id:, retry:) are ignored
	}
	// Flush any trailing event without a blank line
	flush()
}
