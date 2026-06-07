package observability

import (
	"context"
	"fmt"
	"sync"
)

type AlertRoutingRules map[AlertSeverity][]AlertDeliveryChannel

type AlertRoutingRuleStore interface {
	GetRoutingRules(ctx context.Context) (AlertRoutingRules, error)
	UpdateRoutingRules(ctx context.Context, rules AlertRoutingRules) (AlertRoutingRules, error)
}

type InMemoryAlertRoutingRuleStore struct {
	mu    sync.RWMutex
	rules AlertRoutingRules
}

func NewInMemoryAlertRoutingRuleStore(initial AlertRoutingRules) *InMemoryAlertRoutingRuleStore {
	rules, err := NormalizeAlertRoutingRules(initial)
	if err != nil {
		rules = DefaultAlertRoutingRules()
	}
	return &InMemoryAlertRoutingRuleStore{rules: rules}
}

func (s *InMemoryAlertRoutingRuleStore) GetRoutingRules(context.Context) (AlertRoutingRules, error) {
	if s == nil {
		return nil, fmt.Errorf("alert routing rule store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyAlertRoutingRules(s.rules), nil
}

func (s *InMemoryAlertRoutingRuleStore) UpdateRoutingRules(_ context.Context, rules AlertRoutingRules) (AlertRoutingRules, error) {
	if s == nil {
		return nil, fmt.Errorf("alert routing rule store is nil")
	}
	normalized, err := NormalizeAlertRoutingRules(rules)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = normalized
	return copyAlertRoutingRules(s.rules), nil
}

func DefaultAlertRoutingRules() AlertRoutingRules {
	return AlertRoutingRules{
		AlertSeverityDebug: {},
		AlertSeverityInfo: {
			AlertDeliveryChannelEmail,
		},
		AlertSeverityWarning: {
			AlertDeliveryChannelEmail,
			AlertDeliveryChannelIM,
		},
		AlertSeverityCritical: {
			AlertDeliveryChannelEmail,
			AlertDeliveryChannelIM,
			AlertDeliveryChannelSMS,
			AlertDeliveryChannelThirdParty,
			AlertDeliveryChannelPhone,
		},
	}
}

func NormalizeAlertRoutingRules(rules AlertRoutingRules) (AlertRoutingRules, error) {
	if rules == nil {
		return DefaultAlertRoutingRules(), nil
	}
	normalized := make(AlertRoutingRules, len(rules))
	for severity, channels := range rules {
		if !isValidAlertSeverity(severity) {
			return nil, fmt.Errorf("invalid alert severity: %s", severity)
		}
		normalizedChannels := make([]AlertDeliveryChannel, 0, len(channels))
		seen := make(map[AlertDeliveryChannel]struct{}, len(channels))
		for _, channel := range channels {
			if !isValidAlertRoutingChannel(channel) {
				return nil, fmt.Errorf("invalid alert delivery channel: %s", channel)
			}
			if _, ok := seen[channel]; ok {
				continue
			}
			seen[channel] = struct{}{}
			normalizedChannels = append(normalizedChannels, channel)
		}
		normalized[severity] = normalizedChannels
	}
	for severity, channels := range DefaultAlertRoutingRules() {
		if _, ok := normalized[severity]; !ok {
			normalized[severity] = copyDeliveryChannels(channels)
		}
	}
	return normalized, nil
}

func copyAlertRoutingRules(rules AlertRoutingRules) AlertRoutingRules {
	if rules == nil {
		return nil
	}
	copied := make(AlertRoutingRules, len(rules))
	for severity, channels := range rules {
		copied[severity] = copyDeliveryChannels(channels)
	}
	return copied
}

func isValidAlertSeverity(severity AlertSeverity) bool {
	switch severity {
	case AlertSeverityDebug, AlertSeverityInfo, AlertSeverityWarning, AlertSeverityCritical:
		return true
	default:
		return false
	}
}

func isValidAlertRoutingChannel(channel AlertDeliveryChannel) bool {
	switch channel {
	case AlertDeliveryChannelEmail,
		AlertDeliveryChannelIM,
		AlertDeliveryChannelSMS,
		AlertDeliveryChannelThirdParty,
		AlertDeliveryChannelPhone,
		AlertDeliveryChannelInApp:
		return true
	default:
		return false
	}
}
