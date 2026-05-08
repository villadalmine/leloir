package adapter

import "fmt"

// ============================================================================
// Error codes (canonical)
// ============================================================================

// Configuration errors (returned by Configure)
const (
	ErrCodeInvalidConfig        = "invalid_config"
	ErrCodeMissingRequiredField = "missing_required_field"
	ErrCodeUnsupportedFeature   = "unsupported_feature"
)

// Runtime errors (returned by Investigate or sent as EventError)
const (
	ErrCodeLLMUnavailable   = "llm_unavailable"
	ErrCodeLLMRateLimited   = "llm_rate_limited"
	ErrCodeBudgetExhausted  = "budget_exhausted"
	ErrCodeDeadlineExceeded = "deadline_exceeded"
	ErrCodeToolUnavailable  = "tool_unavailable"
	ErrCodeToolDenied       = "tool_denied"
	ErrCodeApprovalDenied   = "approval_denied"
	ErrCodeApprovalTimeout  = "approval_timeout"
	ErrCodeSubAgentFailed   = "subagent_failed"
	ErrCodeAgentInternal    = "agent_internal_error"
)

// Health errors (returned by HealthCheck)
const (
	ErrCodeUnhealthy = "unhealthy"
	ErrCodeDegraded  = "degraded"
)

// ============================================================================
// AdapterError type
// ============================================================================

// AdapterError is the standard error type returned by adapter methods.
// It implements the error interface and supports unwrapping.
type AdapterError struct {
	// Code is one of the canonical codes above.
	Code string

	// Message is a human-readable description.
	Message string

	// Cause is an optional underlying error.
	Cause error

	// Details is optional extra context (passed to UI / audit log).
	Details map[string]any
}

func (e *AdapterError) Error() string {
	if e == nil {
		return "<nil AdapterError>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap supports errors.Is and errors.As.
func (e *AdapterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ============================================================================
// Convenience constructors
// ============================================================================

// NewConfigError builds an *AdapterError for a configuration problem.
func NewConfigError(field, message string) *AdapterError {
	return &AdapterError{
		Code:    ErrCodeMissingRequiredField,
		Message: fmt.Sprintf("%s: %s", field, message),
		Details: map[string]any{"field": field},
	}
}

// NewInternalError builds an *AdapterError for an unexpected internal error.
// Useful in deferred recovers.
func NewInternalError(message string, cause error) *AdapterError {
	return &AdapterError{
		Code:    ErrCodeAgentInternal,
		Message: message,
		Cause:   cause,
	}
}

// NewBudgetExhaustedError builds an *AdapterError for budget exhaustion.
func NewBudgetExhaustedError(resource string, used, limit float64) *AdapterError {
	return &AdapterError{
		Code:    ErrCodeBudgetExhausted,
		Message: fmt.Sprintf("budget exhausted on %s: %.2f / %.2f", resource, used, limit),
		Details: map[string]any{
			"resource": resource,
			"used":     used,
			"limit":    limit,
		},
	}
}
