package observability

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type AlertStatus string

const (
	AlertStatusOpen         AlertStatus = "open"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
)

type AlertState struct {
	Key              string
	Status           AlertStatus
	Severity         AlertSeverity
	OriginalSeverity AlertSeverity
	Escalated        bool
	Title            string
	Component        string
	OpenedAt         time.Time
	LastOccurredAt   time.Time
	OccurrenceCount  int
	AcknowledgedAt   time.Time
	ResolvedAt       time.Time
}

type AlertDeliveryAttempt struct {
	ID           string               `json:"id"`
	AlertKey     string               `json:"alertKey"`
	Severity     AlertSeverity        `json:"severity"`
	Component    string               `json:"component"`
	Channel      AlertDeliveryChannel `json:"channel"`
	ProviderID   string               `json:"providerId"`
	ProviderKind AlertProviderKind    `json:"providerKind"`
	Delivered    bool                 `json:"delivered"`
	Error        string               `json:"error"`
	AttemptedAt  time.Time            `json:"attemptedAt"`
}

type AlertStateFilter struct {
	Status    AlertStatus
	Severity  AlertSeverity
	Component string
	KeyPrefix string
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}

type AlertDeliveryHistoryFilter struct {
	AlertKey  string
	KeyPrefix string
	From      time.Time
	To        time.Time
	Limit     int
	Offset    int
}

type RecoveryActionFilter struct {
	AlertKey   string
	KeyPrefix  string
	PolicyName string
	Component  string
	Type       RecoveryActionType
	From       time.Time
	To         time.Time
	Limit      int
	Offset     int
}

type AlertStateStore interface {
	RecordAlertOpen(ctx context.Context, event AlertEvent) (AlertState, error)
	GetAlertState(ctx context.Context, key string) (AlertState, bool, error)
	ListAlertStates(ctx context.Context, filter AlertStateFilter) ([]AlertState, error)
	AcknowledgeAlert(ctx context.Context, key string, acknowledgedAt time.Time) (AlertState, error)
	ResolveAlert(ctx context.Context, key string, resolvedAt time.Time) (AlertState, error)
	RecordNotification(ctx context.Context, event AlertEvent, window time.Duration) (bool, error)
	RecordDeliveryAttempts(ctx context.Context, event AlertEvent, results []AlertDeliveryResult) error
	ListDeliveryAttempts(ctx context.Context, filter AlertDeliveryHistoryFilter) ([]AlertDeliveryAttempt, error)
	RecordRecoveryAction(ctx context.Context, action RecoveryAction, cooldown time.Duration) (bool, RecoveryAction, error)
	ListRecoveryActions(ctx context.Context, filter RecoveryActionFilter) ([]RecoveryAction, error)
}

type InMemoryAlertStateStore struct {
	mu                sync.Mutex
	states            map[string]*alertStateRecord
	deliveryAttempts  []AlertDeliveryAttempt
	lastNotifications map[string]time.Time
	recoveryActions   []RecoveryAction
	lastRecovery      map[string]RecoveryAction
	escalationWindow  time.Duration
	escalationLimit   int
	warningOpenWindow time.Duration
}

type alertStateRecord struct {
	state       AlertState
	occurrences []time.Time
}

func NewInMemoryAlertStateStore() *InMemoryAlertStateStore {
	return &InMemoryAlertStateStore{
		states:            make(map[string]*alertStateRecord),
		deliveryAttempts:  []AlertDeliveryAttempt{},
		lastNotifications: make(map[string]time.Time),
		lastRecovery:      make(map[string]RecoveryAction),
		escalationWindow:  5 * time.Minute,
		escalationLimit:   3,
		warningOpenWindow: 30 * time.Minute,
	}
}

func (s *InMemoryAlertStateStore) RecordAlertOpen(_ context.Context, event AlertEvent) (AlertState, error) {
	if s == nil {
		return AlertState{}, errors.New("alert state store is nil")
	}
	key := alertKey(event)
	if key == "" {
		return AlertState{}, errors.New("alert key is required")
	}
	if event.Severity == "" {
		event.Severity = AlertSeverityInfo
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.states[key]
	if record == nil {
		record = &alertStateRecord{
			state: AlertState{
				Key:      key,
				Status:   AlertStatusOpen,
				OpenedAt: event.OccurredAt,
			},
		}
		s.states[key] = record
	}

	record.state.Status = AlertStatusOpen
	record.state.Severity = event.Severity
	record.state.Title = event.Title
	record.state.Component = event.Component
	record.state.LastOccurredAt = event.OccurredAt
	record.state.OccurrenceCount++
	record.state.ResolvedAt = time.Time{}

	cutoff := event.OccurredAt.Add(-s.escalationWindow)
	kept := record.occurrences[:0]
	for _, occurredAt := range record.occurrences {
		if !occurredAt.Before(cutoff) {
			kept = append(kept, occurredAt)
		}
	}
	kept = append(kept, event.OccurredAt)
	record.occurrences = kept

	if len(record.occurrences) >= s.escalationLimit {
		escalated := escalateSeverity(event.Severity)
		if escalated != event.Severity {
			record.state.OriginalSeverity = event.Severity
			record.state.Severity = escalated
			record.state.Escalated = true
		}
	}
	if shouldEscalateSustainedWarning(record.state, event.OccurredAt, s.warningOpenWindow) {
		record.state.OriginalSeverity = AlertSeverityWarning
		record.state.Severity = AlertSeverityCritical
		record.state.Escalated = true
	}

	return cloneAlertState(record.state), nil
}

func (s *InMemoryAlertStateStore) GetAlertState(_ context.Context, key string) (AlertState, bool, error) {
	if s == nil {
		return AlertState{}, false, errors.New("alert state store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.states[key]
	if record == nil {
		return AlertState{}, false, nil
	}
	return cloneAlertState(record.state), true, nil
}

func (s *InMemoryAlertStateStore) ListAlertStates(_ context.Context, filter AlertStateFilter) ([]AlertState, error) {
	if s == nil {
		return nil, errors.New("alert state store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	states := make([]AlertState, 0, len(s.states))
	for _, record := range s.states {
		if record == nil {
			continue
		}
		state := record.state
		if filter.Status != "" && state.Status != filter.Status {
			continue
		}
		if filter.Severity != "" && state.Severity != filter.Severity {
			continue
		}
		if filter.Component != "" && state.Component != filter.Component {
			continue
		}
		if filter.KeyPrefix != "" && !strings.HasPrefix(state.Key, filter.KeyPrefix) {
			continue
		}
		if !filter.From.IsZero() && state.LastOccurredAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && state.LastOccurredAt.After(filter.To) {
			continue
		}
		states = append(states, cloneAlertState(state))
	}

	sort.Slice(states, func(i, j int) bool {
		if !states[i].LastOccurredAt.Equal(states[j].LastOccurredAt) {
			return states[i].LastOccurredAt.After(states[j].LastOccurredAt)
		}
		return states[i].Key < states[j].Key
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(states) {
			return []AlertState{}, nil
		}
		states = states[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(states) {
		states = states[:filter.Limit]
	}
	return states, nil
}

func (s *InMemoryAlertStateStore) AcknowledgeAlert(_ context.Context, key string, acknowledgedAt time.Time) (AlertState, error) {
	if s == nil {
		return AlertState{}, errors.New("alert state store is nil")
	}
	if acknowledgedAt.IsZero() {
		acknowledgedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.states[key]
	if record == nil {
		return AlertState{}, errors.New("alert state not found")
	}
	record.state.Status = AlertStatusAcknowledged
	record.state.AcknowledgedAt = acknowledgedAt
	return cloneAlertState(record.state), nil
}

func (s *InMemoryAlertStateStore) ResolveAlert(_ context.Context, key string, resolvedAt time.Time) (AlertState, error) {
	if s == nil {
		return AlertState{}, errors.New("alert state store is nil")
	}
	if resolvedAt.IsZero() {
		resolvedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.states[key]
	if record == nil {
		return AlertState{}, errors.New("alert state not found")
	}
	record.state.Status = AlertStatusResolved
	record.state.ResolvedAt = resolvedAt
	return cloneAlertState(record.state), nil
}

func (s *InMemoryAlertStateStore) RecordNotification(_ context.Context, event AlertEvent, window time.Duration) (bool, error) {
	if s == nil {
		return false, errors.New("alert state store is nil")
	}
	key := alertKey(event)
	if key == "" {
		return true, nil
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	notifyKey := string(event.Severity) + ":" + key

	s.mu.Lock()
	defer s.mu.Unlock()

	if window > 0 {
		last, ok := s.lastNotifications[notifyKey]
		if ok && event.OccurredAt.Sub(last) < window {
			return false, nil
		}
	}
	s.lastNotifications[notifyKey] = event.OccurredAt
	return true, nil
}

func (s *InMemoryAlertStateStore) RecordDeliveryAttempts(_ context.Context, event AlertEvent, results []AlertDeliveryResult) error {
	if s == nil {
		return errors.New("alert state store is nil")
	}
	key := alertKey(event)
	if key == "" {
		return errors.New("alert key is required")
	}
	if event.Severity == "" {
		event.Severity = AlertSeverityInfo
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, result := range results {
		attempt := AlertDeliveryAttempt{
			ID:           makeAlertDeliveryAttemptID(key, event.OccurredAt, result.Channel, i+1),
			AlertKey:     key,
			Severity:     event.Severity,
			Component:    event.Component,
			Channel:      result.Channel,
			ProviderID:   result.ProviderID,
			ProviderKind: result.ProviderKind,
			Delivered:    result.Delivered,
			Error:        alertDeliveryResultError(result),
			AttemptedAt:  event.OccurredAt,
		}
		s.deliveryAttempts = append(s.deliveryAttempts, attempt)
	}
	return nil
}

func (s *InMemoryAlertStateStore) ListDeliveryAttempts(_ context.Context, filter AlertDeliveryHistoryFilter) ([]AlertDeliveryAttempt, error) {
	if s == nil {
		return nil, errors.New("alert state store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	attempts := make([]AlertDeliveryAttempt, 0, len(s.deliveryAttempts))
	for _, attempt := range s.deliveryAttempts {
		if filter.AlertKey != "" && attempt.AlertKey != filter.AlertKey {
			continue
		}
		if filter.KeyPrefix != "" && !strings.HasPrefix(attempt.AlertKey, filter.KeyPrefix) {
			continue
		}
		if !filter.From.IsZero() && attempt.AttemptedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && attempt.AttemptedAt.After(filter.To) {
			continue
		}
		attempts = append(attempts, cloneAlertDeliveryAttempt(attempt))
	}
	sort.Slice(attempts, func(i, j int) bool {
		if !attempts[i].AttemptedAt.Equal(attempts[j].AttemptedAt) {
			return attempts[i].AttemptedAt.After(attempts[j].AttemptedAt)
		}
		return attempts[i].ID < attempts[j].ID
	})
	if filter.Offset > 0 {
		if filter.Offset >= len(attempts) {
			return []AlertDeliveryAttempt{}, nil
		}
		attempts = attempts[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(attempts) {
		attempts = attempts[:filter.Limit]
	}
	return attempts, nil
}

func (s *InMemoryAlertStateStore) RecordRecoveryAction(_ context.Context, action RecoveryAction, cooldown time.Duration) (bool, RecoveryAction, error) {
	if s == nil {
		return false, RecoveryAction{}, errors.New("alert state store is nil")
	}
	if action.AlertKey == "" || action.PolicyName == "" {
		return false, RecoveryAction{}, errors.New("recovery action alert key and policy name are required")
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	key := action.PolicyName + ":" + action.AlertKey

	s.mu.Lock()
	defer s.mu.Unlock()

	last, ok := s.lastRecovery[key]
	if ok && cooldown > 0 && action.CreatedAt.Sub(last.CreatedAt) < cooldown {
		return false, cloneRecoveryAction(last), nil
	}
	s.recoveryActions = append(s.recoveryActions, cloneRecoveryAction(action))
	s.lastRecovery[key] = cloneRecoveryAction(action)
	return true, cloneRecoveryAction(action), nil
}

func (s *InMemoryAlertStateStore) RecoveryActions() []RecoveryAction {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := make([]RecoveryAction, len(s.recoveryActions))
	for i, action := range s.recoveryActions {
		actions[i] = cloneRecoveryAction(action)
	}
	return actions
}

func (s *InMemoryAlertStateStore) ListRecoveryActions(_ context.Context, filter RecoveryActionFilter) ([]RecoveryAction, error) {
	if s == nil {
		return nil, errors.New("alert state store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := make([]RecoveryAction, 0, len(s.recoveryActions))
	for _, action := range s.recoveryActions {
		if filter.AlertKey != "" && action.AlertKey != filter.AlertKey {
			continue
		}
		if filter.KeyPrefix != "" && !strings.HasPrefix(action.AlertKey, filter.KeyPrefix) {
			continue
		}
		if filter.PolicyName != "" && action.PolicyName != filter.PolicyName {
			continue
		}
		if filter.Component != "" && action.Component != filter.Component {
			continue
		}
		if filter.Type != "" && action.Type != filter.Type {
			continue
		}
		if !filter.From.IsZero() && action.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && action.CreatedAt.After(filter.To) {
			continue
		}
		actions = append(actions, cloneRecoveryAction(action))
	}
	sort.Slice(actions, func(i, j int) bool {
		if !actions[i].CreatedAt.Equal(actions[j].CreatedAt) {
			return actions[i].CreatedAt.After(actions[j].CreatedAt)
		}
		return actions[i].ID < actions[j].ID
	})
	if filter.Offset > 0 {
		if filter.Offset >= len(actions) {
			return []RecoveryAction{}, nil
		}
		actions = actions[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(actions) {
		actions = actions[:filter.Limit]
	}
	return actions, nil
}

func cloneAlertState(state AlertState) AlertState {
	return state
}

func cloneAlertDeliveryAttempt(attempt AlertDeliveryAttempt) AlertDeliveryAttempt {
	return attempt
}

func shouldEscalateSustainedWarning(state AlertState, occurredAt time.Time, window time.Duration) bool {
	if window <= 0 {
		return false
	}
	if state.Severity != AlertSeverityWarning || state.OpenedAt.IsZero() || occurredAt.IsZero() {
		return false
	}
	return !occurredAt.Before(state.OpenedAt.Add(window))
}
