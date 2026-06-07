package observability

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type AlertProviderKind string

const (
	AlertProviderKindSMTP          AlertProviderKind = "smtp"
	AlertProviderKindSlackWebhook  AlertProviderKind = "slack_webhook"
	AlertProviderKindFeishuWebhook AlertProviderKind = "feishu_webhook"
	AlertProviderKindDingTalk      AlertProviderKind = "dingtalk_webhook"
	AlertProviderKindWeComWebhook  AlertProviderKind = "wecom_webhook"
	AlertProviderKindTwilioSMS     AlertProviderKind = "twilio_sms"
	AlertProviderKindAliyunSMS     AlertProviderKind = "aliyun_sms"
	AlertProviderKindPhone         AlertProviderKind = "phone"
	AlertProviderKindPagerDuty     AlertProviderKind = "pagerduty"
	AlertProviderKindOpsgenie      AlertProviderKind = "opsgenie"
	AlertProviderKindAliyunMonitor AlertProviderKind = "aliyun_monitor"
	AlertProviderKindTencentCloud  AlertProviderKind = "tencent_cloud_monitor"
)

type AlertProviderStatus string

const (
	AlertProviderStatusActive   AlertProviderStatus = "active"
	AlertProviderStatusDisabled AlertProviderStatus = "disabled"
)

const RedactedAlertProviderSecret = "********"

type AlertProviderConfig struct {
	ID        string
	Kind      AlertProviderKind
	Channel   AlertDeliveryChannel
	Name      string
	Status    AlertProviderStatus
	Config    map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AlertProviderConfigView struct {
	ID        string               `json:"id"`
	Kind      AlertProviderKind    `json:"kind"`
	Channel   AlertDeliveryChannel `json:"channel"`
	Name      string               `json:"name"`
	Status    AlertProviderStatus  `json:"status"`
	Config    map[string]string    `json:"config"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type AlertProviderTestResult struct {
	ProviderID string               `json:"providerId"`
	Kind       AlertProviderKind    `json:"kind"`
	Channel    AlertDeliveryChannel `json:"channel"`
	OK         bool                 `json:"ok"`
	Message    string               `json:"message"`
	TestedAt   time.Time            `json:"testedAt"`
}

type AlertProviderConfigStore interface {
	GetAlertProviderConfig(ctx context.Context, id string) (AlertProviderConfig, bool, error)
	ListAlertProviderConfigs(ctx context.Context) ([]AlertProviderConfig, error)
	SaveAlertProviderConfig(ctx context.Context, config AlertProviderConfig) (AlertProviderConfig, error)
}

type InMemoryAlertProviderConfigStore struct {
	mu      sync.Mutex
	configs map[string]AlertProviderConfig
}

func NewInMemoryAlertProviderConfigStore(initial ...AlertProviderConfig) *InMemoryAlertProviderConfigStore {
	store := &InMemoryAlertProviderConfigStore{configs: make(map[string]AlertProviderConfig)}
	for _, config := range initial {
		normalized, err := NormalizeAlertProviderConfig(config)
		if err != nil {
			continue
		}
		store.configs[normalized.ID] = normalized
	}
	return store
}

func (s *InMemoryAlertProviderConfigStore) GetAlertProviderConfig(_ context.Context, id string) (AlertProviderConfig, bool, error) {
	if s == nil {
		return AlertProviderConfig{}, false, errors.New("alert provider config store is nil")
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return AlertProviderConfig{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	config, ok := s.configs[trimmedID]
	if !ok {
		return AlertProviderConfig{}, false, nil
	}
	return cloneAlertProviderConfig(config), true, nil
}

func (s *InMemoryAlertProviderConfigStore) ListAlertProviderConfigs(context.Context) ([]AlertProviderConfig, error) {
	if s == nil {
		return nil, errors.New("alert provider config store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	configs := make([]AlertProviderConfig, 0, len(s.configs))
	for _, config := range s.configs {
		configs = append(configs, cloneAlertProviderConfig(config))
	}
	sort.Slice(configs, func(i, j int) bool {
		if configs[i].Name != configs[j].Name {
			return configs[i].Name < configs[j].Name
		}
		return configs[i].ID < configs[j].ID
	})
	return configs, nil
}

func (s *InMemoryAlertProviderConfigStore) SaveAlertProviderConfig(_ context.Context, config AlertProviderConfig) (AlertProviderConfig, error) {
	if s == nil {
		return AlertProviderConfig{}, errors.New("alert provider config store is nil")
	}
	normalized, err := NormalizeAlertProviderConfig(config)
	if err != nil {
		return AlertProviderConfig{}, err
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.configs[normalized.ID]; ok && !existing.CreatedAt.IsZero() {
		normalized.CreatedAt = existing.CreatedAt
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = now
	}
	normalized.UpdatedAt = now
	s.configs[normalized.ID] = cloneAlertProviderConfig(normalized)
	return cloneAlertProviderConfig(normalized), nil
}

func NormalizeAlertProviderConfig(config AlertProviderConfig) (AlertProviderConfig, error) {
	config.ID = strings.TrimSpace(config.ID)
	if config.ID == "" {
		return AlertProviderConfig{}, errors.New("alert provider id is required")
	}
	config.Name = strings.TrimSpace(config.Name)
	if config.Name == "" {
		return AlertProviderConfig{}, errors.New("alert provider name is required")
	}
	channel, err := alertProviderChannel(config.Kind)
	if err != nil {
		return AlertProviderConfig{}, err
	}
	config.Channel = channel
	if config.Status == "" {
		config.Status = AlertProviderStatusActive
	}
	if !isValidAlertProviderStatus(config.Status) {
		return AlertProviderConfig{}, fmt.Errorf("invalid alert provider status: %s", config.Status)
	}
	config.Config = normalizeAlertProviderConfigMap(config.Config)
	if err := validateAlertProviderRequiredConfig(config.Kind, config.Config); err != nil {
		return AlertProviderConfig{}, err
	}
	return cloneAlertProviderConfig(config), nil
}

func AlertProviderConfigToView(config AlertProviderConfig) AlertProviderConfigView {
	return AlertProviderConfigView{
		ID:        config.ID,
		Kind:      config.Kind,
		Channel:   config.Channel,
		Name:      config.Name,
		Status:    config.Status,
		Config:    RedactAlertProviderConfig(config.Config),
		CreatedAt: config.CreatedAt,
		UpdatedAt: config.UpdatedAt,
	}
}

func AlertProviderConfigsToViews(configs []AlertProviderConfig) []AlertProviderConfigView {
	views := make([]AlertProviderConfigView, 0, len(configs))
	for _, config := range configs {
		views = append(views, AlertProviderConfigToView(config))
	}
	return views
}

func BuildAlertProviderTestResult(config AlertProviderConfig, testedAt time.Time) AlertProviderTestResult {
	ok := config.Status == AlertProviderStatusActive
	message := "provider configuration validated"
	if !ok {
		message = "provider is disabled"
	}
	return AlertProviderTestResult{
		ProviderID: config.ID,
		Kind:       config.Kind,
		Channel:    config.Channel,
		OK:         ok,
		Message:    message,
		TestedAt:   testedAt,
	}
}

func RedactAlertProviderConfig(config map[string]string) map[string]string {
	redacted := make(map[string]string, len(config))
	for key, value := range config {
		if IsAlertProviderSecretConfigKey(key) && strings.TrimSpace(value) != "" {
			redacted[key] = RedactedAlertProviderSecret
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func IsAlertProviderSecretConfigKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "webhook_url") ||
		strings.Contains(normalized, "routing_key") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "private_key")
}

func alertProviderChannel(kind AlertProviderKind) (AlertDeliveryChannel, error) {
	switch kind {
	case AlertProviderKindSMTP:
		return AlertDeliveryChannelEmail, nil
	case AlertProviderKindSlackWebhook, AlertProviderKindFeishuWebhook, AlertProviderKindDingTalk, AlertProviderKindWeComWebhook:
		return AlertDeliveryChannelIM, nil
	case AlertProviderKindTwilioSMS, AlertProviderKindAliyunSMS:
		return AlertDeliveryChannelSMS, nil
	case AlertProviderKindPhone:
		return AlertDeliveryChannelPhone, nil
	case AlertProviderKindPagerDuty, AlertProviderKindOpsgenie, AlertProviderKindAliyunMonitor, AlertProviderKindTencentCloud:
		return AlertDeliveryChannelThirdParty, nil
	default:
		return "", fmt.Errorf("invalid alert provider kind: %s", kind)
	}
}

func isValidAlertProviderStatus(status AlertProviderStatus) bool {
	switch status {
	case AlertProviderStatusActive, AlertProviderStatusDisabled:
		return true
	default:
		return false
	}
}

func validateAlertProviderRequiredConfig(kind AlertProviderKind, config map[string]string) error {
	requiredFields := map[AlertProviderKind][]string{
		AlertProviderKindSMTP:          {"smtp_host", "smtp_port", "username", "password", "from_email", "recipients"},
		AlertProviderKindSlackWebhook:  {"webhook_url"},
		AlertProviderKindFeishuWebhook: {"webhook_url"},
		AlertProviderKindDingTalk:      {"webhook_url"},
		AlertProviderKindWeComWebhook:  {"webhook_url"},
		AlertProviderKindTwilioSMS:     {"account_sid", "auth_token", "from_number", "recipients"},
		AlertProviderKindAliyunSMS:     {"access_key_id", "access_key_secret", "sign_name", "template_code", "recipients"},
		AlertProviderKindPhone:         {"provider", "phone_numbers"},
		AlertProviderKindPagerDuty:     {"routing_key"},
		AlertProviderKindOpsgenie:      {"api_key"},
		AlertProviderKindAliyunMonitor: {"webhook_url"},
		AlertProviderKindTencentCloud:  {"webhook_url"},
	}
	for _, field := range requiredFields[kind] {
		if strings.TrimSpace(config[field]) == "" {
			return fmt.Errorf("alert provider %s requires config.%s", kind, field)
		}
	}
	if kind == AlertProviderKindPhone {
		switch strings.ToLower(strings.TrimSpace(config["provider"])) {
		case "twilio":
			for _, field := range []string{"account_sid", "auth_token", "from_number"} {
				if strings.TrimSpace(config[field]) == "" {
					return fmt.Errorf("alert provider %s requires config.%s for Twilio phone delivery", kind, field)
				}
			}
		default:
			return fmt.Errorf("alert provider %s has unsupported config.provider: %s", kind, config["provider"])
		}
	}
	return nil
}

func normalizeAlertProviderConfigMap(config map[string]string) map[string]string {
	normalized := make(map[string]string, len(config))
	for key, value := range config {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalized[trimmedKey] = strings.TrimSpace(value)
	}
	return normalized
}

func cloneAlertProviderConfig(config AlertProviderConfig) AlertProviderConfig {
	config.Config = cloneAlertProviderConfigMap(config.Config)
	return config
}

func cloneAlertProviderConfigMap(config map[string]string) map[string]string {
	if config == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(config))
	for key, value := range config {
		cloned[key] = value
	}
	return cloned
}
