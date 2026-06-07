package failure

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")
	ErrInvalidStrategy    = errors.New("invalid failure strategy")
)

// Strategy represents a node failure handling strategy.
type Strategy string

const (
	StrategyAutoRetry      Strategy = "auto_retry"
	StrategyPauseOnFailure Strategy = "pause_on_failure"
	StrategySkipOnFailure  Strategy = "skip_on_failure"
	StrategyFailureBranch  Strategy = "failure_branch"
)

// Action represents the action to take after evaluating a failure strategy.
type Action string

const (
	ActionRetry    Action = "retry"
	ActionPause    Action = "pause"
	ActionSkip     Action = "skip"
	ActionBranch   Action = "branch"
	ActionFail     Action = "fail"
)

// NodeFailureContext provides context about a node failure.
type NodeFailureContext struct {
	NodeID     string
	NodeType   string
	Attempt    int
	MaxRetries int
	Error      error
	StartedAt  time.Time
	FailedAt   time.Time
}

// FailureDecision is the result of evaluating a failure handler.
type FailureDecision struct {
	Action       Action     `json:"action"`
	NextNodeID   string     `json:"nextNodeId,omitempty"`
	RetryAt      *time.Time `json:"retryAt,omitempty"`
	BackoffDelay time.Duration `json:"backoffDelay,omitempty"`
	Message      string     `json:"message,omitempty"`
	Skipped      bool       `json:"skipped"`
}

// Handler evaluates node failures and determines the appropriate action.
type Handler struct {
	defaultStrategy  Strategy
	defaultRetries   int
	initialDelay     time.Duration
	maxDelay         time.Duration
	backoffFactor    float64
	retryableErrors  []string
}

// HandlerOption configures a failure Handler.
type HandlerOption func(*Handler)

// WithDefaultStrategy sets the default failure strategy.
func WithDefaultStrategy(strategy Strategy) HandlerOption {
	return func(h *Handler) {
		h.defaultStrategy = strategy
	}
}

// WithDefaultRetries sets the default maximum retry count.
func WithDefaultRetries(retries int) HandlerOption {
	return func(h *Handler) {
		if retries > 0 {
			h.defaultRetries = retries
		}
	}
}

// WithBackoff configures exponential backoff parameters.
func WithBackoff(initialDelay, maxDelay time.Duration, factor float64) HandlerOption {
	return func(h *Handler) {
		if initialDelay > 0 {
			h.initialDelay = initialDelay
		}
		if maxDelay > 0 {
			h.maxDelay = maxDelay
		}
		if factor > 1 {
			h.backoffFactor = factor
		}
	}
}

// WithRetryableErrors sets the list of error patterns that can be retried.
func WithRetryableErrors(patterns []string) HandlerOption {
	return func(h *Handler) {
		h.retryableErrors = make([]string, 0, len(patterns))
		for _, p := range patterns {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				h.retryableErrors = append(h.retryableErrors, trimmed)
			}
		}
	}
}

// NewHandler creates a new failure handler with the given options.
func NewHandler(options ...HandlerOption) *Handler {
	h := &Handler{
		defaultStrategy: StrategyPauseOnFailure,
		defaultRetries:  3,
		initialDelay:    time.Second,
		maxDelay:        5 * time.Minute,
		backoffFactor:   2.0,
	}
	for _, opt := range options {
		if opt != nil {
			opt(h)
		}
	}
	return h
}

// Handle evaluates a node failure and returns a decision.
func (h *Handler) Handle(ctx NodeFailureContext, strategy Strategy, failureBranchNodeID string) (*FailureDecision, error) {
	if h == nil {
		return nil, fmt.Errorf("failure handler is nil")
	}

	strategy = normalizeStrategy(strategy, h.defaultStrategy)

	switch strategy {
	case StrategyAutoRetry:
		return h.handleAutoRetry(ctx)
	case StrategySkipOnFailure:
		return h.handleSkipOnFailure(ctx), nil
	case StrategyFailureBranch:
		return h.handleFailureBranch(ctx, failureBranchNodeID), nil
	default:
		return h.handlePauseOnFailure(ctx), nil
	}
}

// handleAutoRetry implements automatic retry with exponential backoff.
func (h *Handler) handleAutoRetry(ctx NodeFailureContext) (*FailureDecision, error) {
	maxRetries := ctx.MaxRetries
	if maxRetries <= 0 {
		maxRetries = h.defaultRetries
	}

	if ctx.Attempt >= maxRetries {
		return nil, fmt.Errorf("%w: node %s failed after %d attempts: %v", ErrMaxRetriesExceeded, ctx.NodeID, ctx.Attempt, ctx.Error)
	}

	if !h.isRetryable(ctx.Error) {
		return &FailureDecision{
			Action:  ActionFail,
			Message: fmt.Sprintf("non-retryable error: %v", ctx.Error),
		}, nil
	}

	backoff := h.calculateBackoff(ctx.Attempt)
	retryAt := time.Now().UTC().Add(backoff)

	return &FailureDecision{
		Action:       ActionRetry,
		RetryAt:      &retryAt,
		BackoffDelay: backoff,
		Message:      fmt.Sprintf("retry attempt %d/%d after %s", ctx.Attempt+1, maxRetries, backoff),
	}, nil
}

// handlePauseOnFailure pauses the execution waiting for user decision.
func (h *Handler) handlePauseOnFailure(ctx NodeFailureContext) *FailureDecision {
	return &FailureDecision{
		Action:  ActionPause,
		Message: fmt.Sprintf("node %s failed: %v (paused for user decision)", ctx.NodeID, ctx.Error),
	}
}

// handleSkipOnFailure skips the failed node and continues execution.
func (h *Handler) handleSkipOnFailure(ctx NodeFailureContext) *FailureDecision {
	return &FailureDecision{
		Action:  ActionSkip,
		Skipped: true,
		Message: fmt.Sprintf("node %s failed: %v (skipped)", ctx.NodeID, ctx.Error),
	}
}

// handleFailureBranch routes execution to a failure branch node.
func (h *Handler) handleFailureBranch(ctx NodeFailureContext, branchNodeID string) *FailureDecision {
	branchNodeID = strings.TrimSpace(branchNodeID)
	if branchNodeID == "" {
		return h.handlePauseOnFailure(ctx)
	}
	return &FailureDecision{
		Action:     ActionBranch,
		NextNodeID: branchNodeID,
		Message:    fmt.Sprintf("node %s failed: %v (branching to %s)", ctx.NodeID, ctx.Error, branchNodeID),
	}
}

// calculateBackoff computes the exponential backoff delay for a given attempt.
func (h *Handler) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return h.initialDelay
	}

	factor := h.backoffFactor
	if factor <= 1 {
		factor = 2.0
	}

	delay := float64(h.initialDelay) * math.Pow(factor, float64(attempt))
	maxDelay := float64(h.maxDelay)
	if maxDelay <= 0 {
		maxDelay = float64(5 * time.Minute)
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

// isRetryable checks if the error matches any retryable error patterns.
func (h *Handler) isRetryable(err error) bool {
	if err == nil {
		return true
	}
	if len(h.retryableErrors) == 0 {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	for _, pattern := range h.retryableErrors {
		if strings.Contains(errMsg, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// NormalizeStrategy normalizes a strategy string to a standard Strategy value.
func NormalizeStrategy(raw string) Strategy {
	return normalizeStrategy(Strategy(raw), StrategyPauseOnFailure)
}

func normalizeStrategy(raw Strategy, defaultStrategy Strategy) Strategy {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case "auto_retry", "autoretry":
		return StrategyAutoRetry
	case "skip_on_failure", "skiponfailure", "skip":
		return StrategySkipOnFailure
	case "failure_branch", "failurebranch", "branch":
		return StrategyFailureBranch
	case "pause_on_failure", "pauseonfailure", "pause":
		return StrategyPauseOnFailure
	case "":
		return defaultStrategy
	default:
		return defaultStrategy
	}
}
