// Package orchestrator runs investigations end-to-end.
//
// The orchestrator is the heart of the control plane. For each triggered
// investigation, it:
//   1. Builds an InvestigateRequest from the alert + route + tenant
//   2. Calls adapter.Investigate() and iterates the event channel
//   3. Handles tool-call-request events by forwarding to MCP Gateway
//   4. Handles subagent-request events by invoking A2A (with all guards)
//   5. Handles approval-request events by pausing for HITL
//   6. Writes every event to audit log + stream broker
//   7. Tracks budget and enforces deadlines
//   8. Sends notifications on completion
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	sdkadapter "github.com/leloir/sdk/adapter"
	"github.com/leloir/leloir/internal/controlplane/audit"
	"github.com/leloir/leloir/internal/controlplane/registry"
	"github.com/leloir/leloir/internal/controlplane/stream"
	"github.com/leloir/leloir/internal/store"
)

// Orchestrator manages active investigations.
type Orchestrator struct {
	cfg Config

	mu     sync.Mutex
	active map[string]*InvestigationContext // by investigation ID
}

// Config holds orchestrator dependencies.
type Config struct {
	Registry      *registry.AgentRegistry
	Store         store.Store
	Audit         audit.Writer
	Broker        *stream.Broker
	MCPGatewayURL string
	LLMGatewayURL string

	// A2A guards (match v5.3 design)
	MaxInvocationDepth    int // default 5
	MaxFanOutPerAgent     int // default 3
	MaxTotalSubInvocations int // default 20
}

// InvestigationContext holds the live state of one in-progress investigation.
type InvestigationContext struct {
	ID              string
	ParentID        string // non-empty for sub-investigations (A2A)
	TenantID        string
	AgentName       string
	Started         time.Time
	Deadline        time.Time
	Depth           int // 0 for top-level, N for depth-N sub
	Budget          sdkadapter.Budget
	BudgetRemaining sdkadapter.Budget
	Cancel          context.CancelFunc
	mu              sync.Mutex
}

// New constructs an orchestrator.
func New(cfg Config) *Orchestrator {
	if cfg.MaxInvocationDepth == 0 {
		cfg.MaxInvocationDepth = 5
	}
	if cfg.MaxFanOutPerAgent == 0 {
		cfg.MaxFanOutPerAgent = 3
	}
	if cfg.MaxTotalSubInvocations == 0 {
		cfg.MaxTotalSubInvocations = 20
	}
	return &Orchestrator{
		cfg:    cfg,
		active: make(map[string]*InvestigationContext),
	}
}

// StartInvestigation kicks off a new investigation asynchronously.
// Returns the investigation ID immediately; the investigation runs in a goroutine.
func (o *Orchestrator) StartInvestigation(ctx context.Context, req StartRequest) (string, error) {
	// Validate + resolve the agent
	agent, err := o.cfg.Registry.Resolve(req.AgentName)
	if err != nil {
		return "", fmt.Errorf("resolve agent: %w", err)
	}

	investigationID := req.InvestigationID
	if investigationID == "" {
		investigationID = newInvestigationID()
	}

	// Build the SDK InvestigateRequest
	sdkReq := sdkadapter.InvestigateRequest{
		InvestigationID: investigationID,
		ParentInvestigationID: req.ParentInvestigationID,
		SessionID:      req.SessionID,
		TenantID:       req.TenantID,
		AlertContext:   req.AlertContext,
		AvailableTools: req.AvailableTools,
		Skills:         req.Skills,
		BudgetLimit:    req.Budget,
		Deadline:       req.Deadline,
		CallerContext:  req.CallerContext,
		CustomParams:   req.CustomParams,
	}

	// Track the investigation
	runCtx, cancel := context.WithDeadline(ctx, req.Deadline)
	invCtx := &InvestigationContext{
		ID:              investigationID,
		ParentID:        req.ParentInvestigationID,
		TenantID:        req.TenantID,
		AgentName:       req.AgentName,
		Started:         time.Now(),
		Deadline:        req.Deadline,
		Depth:           req.Depth,
		Budget:          req.Budget,
		BudgetRemaining: req.Budget,
		Cancel:          cancel,
	}

	o.mu.Lock()
	o.active[investigationID] = invCtx
	o.mu.Unlock()

	// Persist investigation record
	if err := o.cfg.Store.CreateInvestigation(ctx, &store.Investigation{
		ID:        investigationID,
		ParentID:  req.ParentInvestigationID,
		TenantID:  req.TenantID,
		AgentName: req.AgentName,
		Status:    "running",
		Started:   invCtx.Started,
	}); err != nil {
		cancel()
		return "", fmt.Errorf("persist investigation: %w", err)
	}

	// Audit: investigation started
	o.cfg.Audit.Write(ctx, audit.Event{
		Type:            audit.EventInvestigationStarted,
		InvestigationID: investigationID,
		TenantID:        req.TenantID,
		Details:         map[string]any{"agent": req.AgentName},
	})

	// Run the investigation in a goroutine
	go o.runInvestigation(runCtx, agent, sdkReq, invCtx)

	return investigationID, nil
}

// runInvestigation is the main investigation loop.
func (o *Orchestrator) runInvestigation(
	ctx context.Context,
	agent *registry.RegisteredAgent,
	req sdkadapter.InvestigateRequest,
	invCtx *InvestigationContext,
) {
	defer o.cleanupInvestigation(invCtx.ID)

	logger := slog.With(
		"investigation_id", invCtx.ID,
		"agent", agent.Name,
		"tenant", invCtx.TenantID,
	)
	logger.Info("starting investigation")

	// Call the adapter
	eventsCh, err := agent.Adapter.Investigate(ctx, req)
	if err != nil {
		logger.Error("Investigate returned error", "error", err)
		o.markInvestigationFailed(invCtx.ID, err)
		return
	}

	// Iterate events
	for evt := range eventsCh {
		// Always audit + broadcast
		o.cfg.Audit.WriteEvent(ctx, invCtx.TenantID, evt)
		o.cfg.Broker.Publish(invCtx.ID, evt)

		// Handle events that require orchestrator action
		switch evt.Type {
		case sdkadapter.EventToolCallRequest:
			o.handleToolCallRequest(ctx, invCtx, evt)

		case sdkadapter.EventSubAgentRequest:
			o.handleSubAgentRequest(ctx, invCtx, evt)

		case sdkadapter.EventApprovalRequest:
			o.handleApprovalRequest(ctx, invCtx, evt)

		case sdkadapter.EventLLMCall:
			o.updateBudgetFromLLM(invCtx, evt)

		case sdkadapter.EventComplete:
			o.markInvestigationComplete(invCtx, evt)
			return
		}

		// Check deadline/cancellation between events
		if ctx.Err() != nil {
			logger.Info("investigation context cancelled", "error", ctx.Err())
			o.markInvestigationCancelled(invCtx.ID, ctx.Err())
			return
		}
	}

	// Channel closed without EventComplete — that's a protocol violation
	logger.Warn("adapter closed channel without EventComplete")
	o.markInvestigationFailed(invCtx.ID, errors.New("adapter closed channel without EventComplete"))
}

// Cancel cancels an active investigation.
func (o *Orchestrator) Cancel(investigationID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	inv, ok := o.active[investigationID]
	if !ok {
		return fmt.Errorf("investigation %q not active", investigationID)
	}
	inv.Cancel()
	return nil
}

// GetActive returns a snapshot of active investigations.
func (o *Orchestrator) GetActive() []string {
	o.mu.Lock()
	defer o.mu.Unlock()

	ids := make([]string, 0, len(o.active))
	for id := range o.active {
		ids = append(ids, id)
	}
	return ids
}

// ─── Internal helpers ────────────────────────────────────────────────────────

func (o *Orchestrator) cleanupInvestigation(id string) {
	o.mu.Lock()
	if inv, ok := o.active[id]; ok {
		inv.Cancel()
		delete(o.active, id)
	}
	o.mu.Unlock()
}

func (o *Orchestrator) handleToolCallRequest(ctx context.Context, invCtx *InvestigationContext, evt sdkadapter.Event) {
	// M1: forward to MCP Gateway, wait for response, send back to adapter
	// via the adapter's tool response channel.
	// This is the key integration point with the MCP Gateway.
	slog.Debug("tool call request",
		"investigation_id", invCtx.ID,
		"tool", extractToolName(evt),
	)
	// See internal/mcpgateway for the outbound path
}

func (o *Orchestrator) handleSubAgentRequest(ctx context.Context, invCtx *InvestigationContext, evt sdkadapter.Event) {
	// M4: invoke another agent via the full 4-layer budget min and defense-in-depth auth.
	// See docs/a2a-protocol.md.
	// Checks needed:
	//   1. AgentRegistration.canInvoke (Q14b)
	//   2. AlertRoute.team (Q14c)
	//   3. ApprovalPolicy if required
	//   4. Depth and fan-out limits
	//   5. Budget propagation (Q15)
	slog.Debug("A2A sub-agent request (M4)", "investigation_id", invCtx.ID)
}

func (o *Orchestrator) handleApprovalRequest(ctx context.Context, invCtx *InvestigationContext, evt sdkadapter.Event) {
	// M4: send to notification channel, wait for response, enforce timeout.
	slog.Debug("HITL approval request (M4)", "investigation_id", invCtx.ID)
}

func (o *Orchestrator) updateBudgetFromLLM(invCtx *InvestigationContext, evt sdkadapter.Event) {
	llm, ok := evt.Payload.(sdkadapter.LLMCallPayload)
	if !ok {
		return
	}
	invCtx.mu.Lock()
	defer invCtx.mu.Unlock()
	invCtx.BudgetRemaining.MaxTokens -= int64(llm.InputTokens + llm.OutputTokens)
	invCtx.BudgetRemaining.MaxUSD -= llm.CostUSD
}

func (o *Orchestrator) markInvestigationComplete(invCtx *InvestigationContext, evt sdkadapter.Event) {
	cp, _ := evt.Payload.(sdkadapter.CompletePayload)
	slog.Info("investigation complete",
		"investigation_id", invCtx.ID,
		"outcome", cp.Outcome,
		"total_cost_usd", cp.TotalCost,
		"duration_ms", time.Since(invCtx.Started).Milliseconds(),
	)
	_ = o.cfg.Store.UpdateInvestigationStatus(
		context.Background(), invCtx.ID, "completed", string(cp.Outcome),
	)
}

func (o *Orchestrator) markInvestigationFailed(id string, err error) {
	slog.Error("investigation failed", "investigation_id", id, "error", err)
	_ = o.cfg.Store.UpdateInvestigationStatus(
		context.Background(), id, "failed", err.Error(),
	)
}

func (o *Orchestrator) markInvestigationCancelled(id string, err error) {
	reason := "cancelled"
	if err != nil {
		reason = err.Error()
	}
	slog.Info("investigation cancelled", "investigation_id", id, "reason", reason)
	_ = o.cfg.Store.UpdateInvestigationStatus(
		context.Background(), id, "cancelled", reason,
	)
}

// StartRequest is the input to StartInvestigation.
type StartRequest struct {
	InvestigationID       string
	ParentInvestigationID string
	SessionID             string
	TenantID              string
	AgentName             string
	AlertContext          sdkadapter.AlertContext
	AvailableTools        []string
	Skills                []sdkadapter.SkillRef
	Budget                sdkadapter.Budget
	Deadline              time.Time
	Depth                 int
	CallerContext         sdkadapter.CallerContext
	CustomParams          map[string]any
}

// newInvestigationID generates an ID matching the spec format.
func newInvestigationID() string {
	return fmt.Sprintf("inv-%06x-%d", time.Now().UnixNano()%(1<<24), time.Now().Unix())
}

func extractToolName(evt sdkadapter.Event) string {
	if t, ok := evt.Payload.(sdkadapter.ToolCallRequestPayload); ok {
		return t.ToolName
	}
	return "unknown"
}
