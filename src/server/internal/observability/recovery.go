package observability

import (
	"context"
	"errors"
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
	RecoveryActionRecorded RecoveryActionStatus = "recorded"
)

type RecoveryPolicy struct {
	Name       string
	Severity   AlertSeverity
	Component  string
	ActionType RecoveryActionType
	Cooldown   time.Duration
}

type RecoveryAction struct {
	ID         string
	PolicyName string
	AlertKey   string
	Severity   AlertSeverity
	Component  string
	Type       RecoveryActionType
	Status     RecoveryActionStatus
	Reason     string
	CreatedAt  time.Time
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
			Reason:     event.Title,
			CreatedAt:  event.OccurredAt,
		}
		created, stored, err := c.stateStore.RecordRecoveryAction(ctx, action, policy.Cooldown)
		if err != nil {
			return RecoveryDecision{}, err
		}
		return RecoveryDecision{Created: created, Action: stored}, nil
	}

	return RecoveryDecision{}, nil
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
	return true
}

func cloneRecoveryAction(action RecoveryAction) RecoveryAction {
	return action
}
