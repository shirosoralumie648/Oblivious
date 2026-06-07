package alerting

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidRuleName    = errors.New("alert rule name is required")
	ErrInvalidRuleID      = errors.New("alert rule id is required")
	ErrInvalidCondition   = errors.New("alert rule condition is required")
	ErrInvalidSeverity    = errors.New("severity must be debug, info, warning, or critical")
	ErrRuleNotFound       = errors.New("alert rule not found")
	ErrRuleAlreadyExists  = errors.New("alert rule already exists")
	ErrInvalidMetricName  = errors.New("metric name is required")
	ErrInvalidThreshold   = errors.New("threshold is required")
	ErrInvalidDuration    = errors.New("evaluation duration must be positive")
)

type Severity string

const (
	SeverityDebug    Severity = "debug"
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type ComparisonOperator string

const (
	ComparisonGreaterThan          ComparisonOperator = "gt"
	ComparisonGreaterThanOrEqual   ComparisonOperator = "gte"
	ComparisonLessThan             ComparisonOperator = "lt"
	ComparisonLessThanOrEqual      ComparisonOperator = "lte"
	ComparisonEqual                ComparisonOperator = "eq"
	ComparisonNotEqual             ComparisonOperator = "neq"
)

type RuleStatus string

const (
	RuleStatusPending  RuleStatus = "pending"
	RuleStatusFiring   RuleStatus = "firing"
	RuleStatusResolved RuleStatus = "resolved"
	RuleStatusDisabled RuleStatus = "disabled"
)

type AlertRule struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description,omitempty"`
	Severity       Severity          `json:"severity"`
	MetricName     string            `json:"metricName"`
	Operator       ComparisonOperator `json:"operator"`
	Threshold      float64           `json:"threshold"`
	Duration       time.Duration     `json:"duration"`
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Enabled        bool              `json:"enabled"`
	Status         RuleStatus        `json:"status"`
	LastEvaluated  *time.Time        `json:"lastEvaluated,omitempty"`
	LastFired      *time.Time        `json:"lastFired,omitempty"`
	FireCount      int               `json:"fireCount"`
	OrganizationID string            `json:"organizationId"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type RuleEvaluationResult struct {
	RuleID         string        `json:"ruleId"`
	RuleName       string        `json:"ruleName"`
	Severity       Severity      `json:"severity"`
	Fired          bool          `json:"fired"`
	CurrentValue   float64       `json:"currentValue"`
	Threshold      float64       `json:"threshold"`
	Operator       ComparisonOperator `json:"operator"`
	Duration       time.Duration `json:"duration"`
	EvaluatedAt    time.Time     `json:"evaluatedAt"`
	Message        string        `json:"message,omitempty"`
}

type MetricProvider interface {
	QueryMetric(ctx context.Context, metricName string, labels map[string]string, duration time.Duration) (float64, error)
}

type RuleStore interface {
	CreateRule(ctx context.Context, rule AlertRule) (AlertRule, error)
	GetRule(ctx context.Context, id string) (AlertRule, error)
	ListRules(ctx context.Context, organizationID string) ([]AlertRule, error)
	UpdateRule(ctx context.Context, rule AlertRule) (AlertRule, error)
	DeleteRule(ctx context.Context, id string) error
	UpdateRuleStatus(ctx context.Context, id string, status RuleStatus, now time.Time) error
}

type AlertRuleEngine struct {
	mu             sync.Mutex
	store          RuleStore
	metricProvider MetricProvider
	now            func() time.Time
	onAlertFired   func(ctx context.Context, result RuleEvaluationResult)
	onAlertResolved func(ctx context.Context, result RuleEvaluationResult)
}

type AlertRuleEngineConfig struct {
	Store          RuleStore
	MetricProvider MetricProvider
	Now            func() time.Time
	OnAlertFired   func(ctx context.Context, result RuleEvaluationResult)
	OnAlertResolved func(ctx context.Context, result RuleEvaluationResult)
}

func NewAlertRuleEngine(config AlertRuleEngineConfig) *AlertRuleEngine {
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AlertRuleEngine{
		store:          config.Store,
		metricProvider: config.MetricProvider,
		now:            now,
		onAlertFired:   config.OnAlertFired,
		onAlertResolved: config.OnAlertResolved,
	}
}

func (e *AlertRuleEngine) CreateRule(ctx context.Context, rule AlertRule) (AlertRule, error) {
	normalized, err := normalizeAlertRule(rule)
	if err != nil {
		return AlertRule{}, err
	}

	if e.store == nil {
		return AlertRule{}, errors.New("rule store is not configured")
	}

	return e.store.CreateRule(ctx, normalized)
}

func (e *AlertRuleEngine) UpdateRule(ctx context.Context, rule AlertRule) (AlertRule, error) {
	normalized, err := normalizeAlertRule(rule)
	if err != nil {
		return AlertRule{}, err
	}

	if e.store == nil {
		return AlertRule{}, errors.New("rule store is not configured")
	}

	return e.store.UpdateRule(ctx, normalized)
}

func (e *AlertRuleEngine) DeleteRule(ctx context.Context, id string) error {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return ErrInvalidRuleID
	}

	if e.store == nil {
		return errors.New("rule store is not configured")
	}

	return e.store.DeleteRule(ctx, normalizedID)
}

func (e *AlertRuleEngine) GetRule(ctx context.Context, id string) (AlertRule, error) {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" {
		return AlertRule{}, ErrInvalidRuleID
	}

	if e.store == nil {
		return AlertRule{}, errors.New("rule store is not configured")
	}

	return e.store.GetRule(ctx, normalizedID)
}

func (e *AlertRuleEngine) ListRules(ctx context.Context, organizationID string) ([]AlertRule, error) {
	normalizedOrg := strings.TrimSpace(organizationID)
	if normalizedOrg == "" {
		return nil, errors.New("organization is required")
	}

	if e.store == nil {
		return nil, errors.New("rule store is not configured")
	}

	rules, err := e.store.ListRules(ctx, normalizedOrg)
	if err != nil {
		return nil, err
	}

	if rules == nil {
		return []AlertRule{}, nil
	}

	return rules, nil
}

func (e *AlertRuleEngine) EvaluateRule(ctx context.Context, rule AlertRule) (RuleEvaluationResult, error) {
	now := e.now()

	if e.metricProvider == nil {
		return RuleEvaluationResult{}, errors.New("metric provider is not configured")
	}

	currentValue, err := e.metricProvider.QueryMetric(ctx, rule.MetricName, rule.Labels, rule.Duration)
	if err != nil {
		return RuleEvaluationResult{}, err
	}

	result := RuleEvaluationResult{
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		Severity:     rule.Severity,
		Threshold:    rule.Threshold,
		Operator:     rule.Operator,
		Duration:     rule.Duration,
		CurrentValue: currentValue,
		EvaluatedAt:  now,
	}

	result.Fired = evaluateCondition(currentValue, rule.Operator, rule.Threshold)
	result.Message = buildEvaluationMessage(rule, currentValue, result.Fired)

	if e.store != nil {
		newStatus := RuleStatusResolved
		if result.Fired {
			newStatus = RuleStatusFiring
		}

		_ = e.store.UpdateRuleStatus(ctx, rule.ID, newStatus, now)
	}

	if result.Fired && e.onAlertFired != nil {
		e.onAlertFired(ctx, result)
	} else if !result.Fired && rule.Status == RuleStatusFiring && e.onAlertResolved != nil {
		e.onAlertResolved(ctx, result)
	}

	return result, nil
}

func (e *AlertRuleEngine) EvaluateAllRules(ctx context.Context, organizationID string) ([]RuleEvaluationResult, error) {
	rules, err := e.ListRules(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	results := make([]RuleEvaluationResult, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		result, err := e.EvaluateRule(ctx, rule)
		if err != nil {
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

func evaluateCondition(value float64, operator ComparisonOperator, threshold float64) bool {
	switch operator {
	case ComparisonGreaterThan:
		return value > threshold
	case ComparisonGreaterThanOrEqual:
		return value >= threshold
	case ComparisonLessThan:
		return value < threshold
	case ComparisonLessThanOrEqual:
		return value <= threshold
	case ComparisonEqual:
		return value == threshold
	case ComparisonNotEqual:
		return value != threshold
	default:
		return false
	}
}

func buildEvaluationMessage(rule AlertRule, currentValue float64, fired bool) string {
	if !fired {
		return ""
	}

	return fmt.Sprintf("Alert %s: %s is %s (threshold: %s %s)",
		rule.Name, rule.MetricName,
		formatFloat(currentValue), string(rule.Operator), formatFloat(rule.Threshold))
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func normalizeAlertRule(rule AlertRule) (AlertRule, error) {
	normalized := rule
	normalized.ID = strings.TrimSpace(rule.ID)
	normalized.Name = strings.TrimSpace(rule.Name)
	normalized.Description = strings.TrimSpace(rule.Description)
	normalized.MetricName = strings.TrimSpace(rule.MetricName)
	normalized.OrganizationID = strings.TrimSpace(rule.OrganizationID)

	if normalized.ID == "" {
		return AlertRule{}, ErrInvalidRuleID
	}
	if normalized.Name == "" {
		return AlertRule{}, ErrInvalidRuleName
	}
	if normalized.MetricName == "" {
		return AlertRule{}, ErrInvalidMetricName
	}
	if !isValidSeverity(normalized.Severity) {
		return AlertRule{}, ErrInvalidSeverity
	}
	if !isValidComparisonOperator(normalized.Operator) {
		return AlertRule{}, ErrInvalidCondition
	}
	if normalized.Duration <= 0 {
		return AlertRule{}, ErrInvalidDuration
	}

	if normalized.Labels == nil {
		normalized.Labels = make(map[string]string)
	}
	if normalized.Annotations == nil {
		normalized.Annotations = make(map[string]string)
	}

	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now().UTC()
	}
	normalized.UpdatedAt = time.Now().UTC()

	normalized.Status = RuleStatusPending
	normalized.Enabled = true

	return normalized, nil
}

func isValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityDebug, SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

func isValidComparisonOperator(op ComparisonOperator) bool {
	switch op {
	case ComparisonGreaterThan, ComparisonGreaterThanOrEqual,
		ComparisonLessThan, ComparisonLessThanOrEqual,
		ComparisonEqual, ComparisonNotEqual:
		return true
	default:
		return false
	}
}
