package holmesgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/leloir/sdk/adapter"
)

// holmesAPIRequest is the payload Holmes expects on POST /api/chat.
// Holmes currently returns a synchronous JSON response (no SSE streaming).
// See docs/CONTRACT.md for the full API specification.
type holmesAPIRequest struct {
	Ask   string `json:"ask"`
	Model string `json:"model,omitempty"` // Holmes model alias, e.g. "gemma4-31b"
}

// holmesAPIResponse is the JSON response from POST /api/chat.
type holmesAPIResponse struct {
	Analysis  string     `json:"analysis"`  // final answer in markdown
	Detail    string     `json:"detail"`    // non-empty only on error
	ToolCalls []toolCall `json:"tool_calls"`
}

type toolCall struct {
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
}

// buildHolmesRequest assembles the payload from a Leloir InvestigateRequest.
func buildHolmesRequest(req adapter.InvestigateRequest, cfg adapter.Config) holmesAPIRequest {
	return holmesAPIRequest{
		Ask:   buildPrompt(req),
		Model: cfg.ModelConfig.Model,
	}
}

// buildPrompt converts the alert context to the question text Holmes expects.
// Alert content is wrapped in untrusted markers to help Holmes distinguish
// platform instructions from potentially-injected alert payloads.
func buildPrompt(req adapter.InvestigateRequest) string {
	var b bytes.Buffer
	b.WriteString("Investigate the following alert. Use available tools to gather evidence and determine the root cause. Suggest a concrete remediation.\n\n")
	b.WriteString("<alert>\n")
	fmt.Fprintf(&b, "source: %s\n", req.AlertContext.Source)
	fmt.Fprintf(&b, "severity: %s\n", req.AlertContext.Severity)
	fmt.Fprintf(&b, "title: %s\n", req.AlertContext.Title)
	b.WriteString("\n<untrusted-content>\n")
	b.WriteString(req.AlertContext.Description)
	b.WriteString("\n</untrusted-content>\n")
	b.WriteString("</alert>\n")

	if len(req.AlertContext.Labels) > 0 {
		b.WriteString("\n<labels>\n")
		for k, v := range req.AlertContext.Labels {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
		}
		b.WriteString("</labels>\n")
	}

	for _, skill := range req.Skills {
		fmt.Fprintf(&b, "\n<skill name=%q>\n%s\n</skill>\n", skill.Name, skill.Content)
	}

	return b.String()
}

// callHolmes POSTs to Holmes's /api/chat and returns the parsed response.
// Holmes returns a synchronous JSON response — no streaming in the current version.
// The HTTP client timeout is set to 5 minutes to match holmesApiTimeout in Helm.
func (a *Adapter) callHolmes(ctx context.Context, req holmesAPIRequest) (*holmesAPIResponse, error) {
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

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call Holmes: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Holmes response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &adapter.AdapterError{
			Code:    adapter.ErrCodeLLMRateLimited,
			Message: "Holmes rate limited (429) — retry later",
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Holmes returned HTTP %d: %s", resp.StatusCode, raw)
	}

	var hr holmesAPIResponse
	if err := json.Unmarshal(raw, &hr); err != nil {
		return nil, fmt.Errorf("unmarshal Holmes response: %w (raw: %.200s)", err, raw)
	}
	if hr.Detail != "" {
		return nil, fmt.Errorf("Holmes error: %s", hr.Detail)
	}
	if hr.Analysis == "" {
		return nil, fmt.Errorf("Holmes returned empty analysis (raw: %.200s)", raw)
	}
	return &hr, nil
}
