package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type RecoveryActionType string

const (
	RecoveryActionRestart  RecoveryActionType = "restart"
	RecoveryActionScaleOut RecoveryActionType = "scale_out"
	RecoveryActionFailover RecoveryActionType = "failover"
)

type RecoveryActionStatus string

const (
	RecoveryActionRecorded  RecoveryActionStatus = "recorded"
	RecoveryActionExhausted RecoveryActionStatus = "exhausted"
)

type RecoveryPolicy struct {
	Name               string
	Severity           AlertSeverity
	Component          string
	FieldMatches       map[string]string
	ActionType         RecoveryActionType
	Cooldown           time.Duration
	RestartMaxAttempts int
	RestartWindow      time.Duration
	RestartBackoff     []time.Duration
}

type RecoveryAction struct {
	ID            string
	PolicyName    string
	AlertKey      string
	Severity      AlertSeverity
	Component     string
	Type          RecoveryActionType
	Status        RecoveryActionStatus
	Reason        string
	Attempt       int
	NextAttemptAt time.Time
	CreatedAt     time.Time
}

type RecoveryControllerOptions struct {
	StateStore AlertStateStore
	Policies   []RecoveryPolicy
	Now        func() time.Time
}

type RecoveryController struct {
	stateStore AlertStateStore
	policies   []RecoveryPolicy
	now        func() time.Time
}

type RecoveryDecision struct {
	Created bool
	Action  RecoveryAction
}

func NewRecoveryController(options RecoveryControllerOptions) *RecoveryController {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &RecoveryController{
		stateStore: options.StateStore,
		policies:   append([]RecoveryPolicy(nil), options.Policies...),
		now:        now,
	}
}

func (c *RecoveryController) HandleAlert(ctx context.Context, event AlertEvent) (RecoveryDecision, error) {
	if c == nil || c.stateStore == nil {
		return RecoveryDecision{}, errors.New("recovery controller state store is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = c.now()
	}
	key := alertKey(event)
	if key == "" {
		return RecoveryDecision{}, errors.New("alert key is required")
	}

	for _, policy := range c.policies {
		if !policyMatchesAlert(policy, event) {
			continue
		}
		action := RecoveryAction{
			ID:         policy.Name + ":" + key + ":" + event.OccurredAt.Format(time.RFC3339Nano),
			PolicyName: policy.Name,
			AlertKey:   key,
			Severity:   event.Severity,
			Component:  event.Component,
			Type:       policy.ActionType,
			Status:     RecoveryActionRecorded,
			Reason:     auditOnlyRecoveryReason(event.Title),
			CreatedAt:  event.OccurredAt,
		}
		if policy.ActionType == RecoveryActionRestart {
			planned, err := c.planRestartAction(ctx, policy, action)
			if err != nil {
				return RecoveryDecision{}, err
			}
			action = planned
		}
		created, stored, err := c.stateStore.RecordRecoveryAction(ctx, action, policy.Cooldown)
		if err != nil {
			return RecoveryDecision{}, err
		}
		return RecoveryDecision{Created: created, Action: stored}, nil
	}

	return RecoveryDecision{}, nil
}

func (c *RecoveryController) planRestartAction(ctx context.Context, policy RecoveryPolicy, action RecoveryAction) (RecoveryAction, error) {
	window := policy.RestartWindow
	if window <= 0 {
		window = 10 * time.Minute
	}
	maxAttempts := policy.RestartMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	backoff := policy.RestartBackoff
	if len(backoff) == 0 {
		backoff = []time.Duration{10 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute}
	}
	recent, err := c.stateStore.ListRecoveryActions(ctx, RecoveryActionFilter{
		AlertKey:   action.AlertKey,
		PolicyName: action.PolicyName,
		Type:       RecoveryActionRestart,
	})
	if err != nil {
		return RecoveryAction{}, err
	}
	cutoff := action.CreatedAt.Add(-window)
	action.Attempt = 1
	for _, previous := range recent {
		if previous.CreatedAt.Before(cutoff) {
			continue
		}
		action.Attempt++
	}
	if action.Attempt > maxAttempts {
		action.Status = RecoveryActionExhausted
		action.NextAttemptAt = time.Time{}
		if action.Reason == "" {
			action.Reason = "restart retry limit reached; manual intervention required"
		} else {
			action.Reason += "; restart retry limit reached; manual intervention required"
		}
		return action, nil
	}
	delay := backoff[len(backoff)-1]
	if action.Attempt-1 < len(backoff) {
		delay = backoff[action.Attempt-1]
	}
	action.NextAttemptAt = action.CreatedAt.Add(delay)
	return action, nil
}

func auditOnlyRecoveryReason(title string) string {
	title = strings.TrimSpace(title)
	auditNote := "audit-only remediation recorded; no infrastructure mutation executed"
	if title == "" {
		return auditNote
	}
	return title + "; " + auditNote
}

func policyMatchesAlert(policy RecoveryPolicy, event AlertEvent) bool {
	if policy.Name == "" || policy.ActionType == "" {
		return false
	}
	if policy.Severity != "" && policy.Severity != event.Severity {
		return false
	}
	if policy.Component != "" && policy.Component != event.Component {
		return false
	}
	for key, expected := range policy.FieldMatches {
		if strings.TrimSpace(key) == "" {
			return false
		}
		actual, ok := event.Fields[key]
		if !ok || fmt.Sprint(actual) != expected {
			return false
		}
	}
	return true
}

func cloneRecoveryAction(action RecoveryAction) RecoveryAction {
	return action
}
