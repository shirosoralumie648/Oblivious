package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AlertProviderDeliverySinkResolverOptions struct {
	ProviderStore AlertProviderConfigStore
	HTTPClient    *http.Client
}

type AlertProviderDeliverySinkResolver struct {
	providerStore    AlertProviderConfigStore
	httpClient       *http.Client
	recipientLimiter *AlertRecipientDeliveryLimiter
}

func NewAlertProviderDeliverySinkResolver(options AlertProviderDeliverySinkResolverOptions) *AlertProviderDeliverySinkResolver {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &AlertProviderDeliverySinkResolver{
		providerStore:    options.ProviderStore,
		httpClient:       client,
		recipientLimiter: NewAlertRecipientDeliveryLimiter(time.Hour),
	}
}

func (r *AlertProviderDeliverySinkResolver) SinksForChannel(ctx context.Context, channel AlertDeliveryChannel) ([]AlertDeliverySink, error) {
	if r == nil || r.providerStore == nil {
		return nil, nil
	}
	configs, err := r.providerStore.ListAlertProviderConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list alert provider configs: %w", err)
	}
	sinks := make([]AlertDeliverySink, 0, len(configs))
	for _, config := range configs {
		if config.Status != AlertProviderStatusActive || config.Channel != channel {
			continue
		}
		sink, err := r.sinkForProvider(config)
		if err != nil {
			return nil, err
		}
		if sink != nil {
			sinks = append(sinks, sink)
		}
	}
	return sinks, nil
}

func (r *AlertProviderDeliverySinkResolver) sinkForProvider(config AlertProviderConfig) (AlertDeliverySink, error) {
	return alertProviderDeliverySinkForConfig(config, r.httpClient, r.recipientLimiter)
}

func alertProviderDeliverySinkForConfig(config AlertProviderConfig, client *http.Client, limiter *AlertRecipientDeliveryLimiter) (AlertDeliverySink, error) {
	switch config.Kind {
	case AlertProviderKindSMTP:
		return NewSMTPAlertDeliverySink(SMTPAlertDeliverySinkOptions{Provider: config}), nil
	case AlertProviderKindSlackWebhook, AlertProviderKindFeishuWebhook, AlertProviderKindDingTalk, AlertProviderKindWeComWebhook:
		return NewIMWebhookAlertDeliverySink(IMWebhookAlertDeliverySinkOptions{
			Provider:   config,
			HTTPClient: client,
		}), nil
	case AlertProviderKindPagerDuty, AlertProviderKindOpsgenie, AlertProviderKindAliyunMonitor, AlertProviderKindTencentCloud:
		return NewThirdPartyAlertDeliverySink(ThirdPartyAlertDeliverySinkOptions{
			Provider:   config,
			HTTPClient: client,
		}), nil
	case AlertProviderKindTwilioSMS, AlertProviderKindAliyunSMS:
		return NewSMSAlertDeliverySink(SMSAlertDeliverySinkOptions{
			Provider:   config,
			HTTPClient: client,
			Limiter:    limiter,
		}), nil
	case AlertProviderKindPhone:
		return NewPhoneAlertDeliverySink(PhoneAlertDeliverySinkOptions{
			Provider:   config,
			HTTPClient: client,
			Limiter:    limiter,
		}), nil
	default:
		return nil, nil
	}
}

const (
	defaultPagerDutyEventsAPIURL = "https://events.pagerduty.com/v2/enqueue"
	defaultOpsgenieAlertsAPIURL  = "https://api.opsgenie.com/v2/alerts"
	defaultTwilioAPIBaseURL      = "https://api.twilio.com"
	defaultAliyunSMSAPIURL       = "https://dysmsapi.aliyuncs.com"
	smsHourlyRecipientLimit      = 5
	phoneHourlyRecipientLimit    = 1
)

var ErrAlertRecipientHourlyLimit = errors.New("alert recipient hourly limit reached")

type AlertRecipientDeliveryLimiter struct {
	mu     sync.Mutex
	window time.Duration
	sentAt map[string][]time.Time
}

func NewAlertRecipientDeliveryLimiter(window time.Duration) *AlertRecipientDeliveryLimiter {
	if window <= 0 {
		window = time.Hour
	}
	return &AlertRecipientDeliveryLimiter{
		window: window,
		sentAt: make(map[string][]time.Time),
	}
}

func (l *AlertRecipientDeliveryLimiter) Allow(channel AlertDeliveryChannel, providerID, recipient string, occurredAt time.Time, limit int) bool {
	if l == nil || limit <= 0 {
		return true
	}
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return false
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	key := strings.Join([]string{string(channel), strings.TrimSpace(providerID), recipient}, ":")
	cutoff := occurredAt.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	entries := l.sentAt[key]
	kept := entries[:0]
	for _, entry := range entries {
		if !entry.Before(cutoff) {
			kept = append(kept, entry)
		}
	}
	if len(kept) >= limit {
		l.sentAt[key] = kept
		return false
	}
	kept = append(kept, occurredAt)
	l.sentAt[key] = kept
	return true
}

type SMSAlertDeliverySinkOptions struct {
	Provider   AlertProviderConfig
	HTTPClient *http.Client
	Limiter    *AlertRecipientDeliveryLimiter
}

type SMSAlertDeliverySink struct {
	provider        AlertProviderConfig
	client          *http.Client
	limiter         *AlertRecipientDeliveryLimiter
	accountSID      string
	authToken       string
	fromNumber      string
	recipients      []string
	apiURL          string
	accessKeyID     string
	accessKeySecret string
	signName        string
	templateCode    string
}

func NewSMSAlertDeliverySink(options SMSAlertDeliverySinkOptions) *SMSAlertDeliverySink {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	config := options.Provider.Config
	return &SMSAlertDeliverySink{
		provider:        cloneAlertProviderConfig(options.Provider),
		client:          client,
		limiter:         options.Limiter,
		accountSID:      strings.TrimSpace(config["account_sid"]),
		authToken:       strings.TrimSpace(config["auth_token"]),
		fromNumber:      strings.TrimSpace(config["from_number"]),
		recipients:      parseAlertRecipients(config["recipients"]),
		apiURL:          strings.TrimSpace(config["api_url"]),
		accessKeyID:     strings.TrimSpace(config["access_key_id"]),
		accessKeySecret: strings.TrimSpace(config["access_key_secret"]),
		signName:        strings.TrimSpace(config["sign_name"]),
		templateCode:    strings.TrimSpace(config["template_code"]),
	}
}

func (s *SMSAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelSMS
}

func (s *SMSAlertDeliverySink) ProviderID() string {
	if s == nil {
		return ""
	}
	return s.provider.ID
}

func (s *SMSAlertDeliverySink) ProviderKind() AlertProviderKind {
	if s == nil {
		return ""
	}
	return s.provider.Kind
}

func (s *SMSAlertDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.client == nil {
		return ErrAlertDeliverySinkMissing
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("SMS alert provider %s is missing recipients", s.provider.ID)
	}

	var deliveryErr error
	sent := 0
	for _, recipient := range s.recipients {
		if !s.limiter.Allow(AlertDeliveryChannelSMS, s.provider.ID, recipient, event.OccurredAt, smsHourlyRecipientLimit) {
			continue
		}
		var err error
		switch s.provider.Kind {
		case AlertProviderKindTwilioSMS:
			err = s.deliverTwilioSMS(ctx, recipient, event)
		case AlertProviderKindAliyunSMS:
			err = s.deliverAliyunSMS(ctx, recipient, event)
		default:
			err = fmt.Errorf("unsupported SMS alert provider kind: %s", s.provider.Kind)
		}
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
			continue
		}
		sent++
	}
	if sent == 0 && deliveryErr == nil {
		return fmt.Errorf("%w: %s", ErrAlertRecipientHourlyLimit, s.provider.ID)
	}
	return deliveryErr
}

func (s *SMSAlertDeliverySink) deliverTwilioSMS(ctx context.Context, recipient string, event AlertEvent) error {
	if s.accountSID == "" || s.authToken == "" || s.fromNumber == "" {
		return fmt.Errorf("Twilio SMS alert provider %s is missing account_sid, auth_token, or from_number", s.provider.ID)
	}
	form := url.Values{}
	form.Set("To", recipient)
	form.Set("From", s.fromNumber)
	form.Set("Body", alertSMSBody(event))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, twilioMessagesEndpoint(s.apiURL, s.accountSID), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build Twilio SMS request for provider %s: %w", s.provider.ID, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(s.accountSID, s.authToken)
	return s.doProviderRequest(request, "Twilio SMS")
}

func (s *SMSAlertDeliverySink) deliverAliyunSMS(ctx context.Context, recipient string, event AlertEvent) error {
	if s.accessKeyID == "" || s.accessKeySecret == "" || s.signName == "" || s.templateCode == "" {
		return fmt.Errorf("Aliyun SMS alert provider %s is missing credential or template configuration", s.provider.ID)
	}
	endpoint := strings.TrimSpace(s.apiURL)
	if endpoint == "" {
		endpoint = defaultAliyunSMSAPIURL
	}
	payload := map[string]any{
		"access_key_id": s.accessKeyID,
		"sign_name":     s.signName,
		"template_code": s.templateCode,
		"phone_numbers": recipient,
		"template_params": map[string]any{
			"title":     alertThirdPartyTitle(event),
			"message":   strings.TrimSpace(event.Message),
			"severity":  string(event.Severity),
			"component": strings.TrimSpace(event.Component),
			"alert_key": alertKey(event),
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Aliyun SMS payload for provider %s: %w", s.provider.ID, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build Aliyun SMS request for provider %s: %w", s.provider.ID, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Aliyun-Access-Key", s.accessKeyID)
	request.Header.Set("X-Oblivious-Aliyun-Secret", s.accessKeySecret)
	return s.doProviderRequest(request, "Aliyun SMS")
}

func (s *SMSAlertDeliverySink) doProviderRequest(request *http.Request, providerName string) error {
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("deliver %s alert provider %s: %w", providerName, s.provider.ID, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("%s alert provider %s returned %d: %s", providerName, s.provider.ID, response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

type PhoneAlertDeliverySinkOptions struct {
	Provider   AlertProviderConfig
	HTTPClient *http.Client
	Limiter    *AlertRecipientDeliveryLimiter
}

type PhoneAlertDeliverySink struct {
	provider     AlertProviderConfig
	client       *http.Client
	limiter      *AlertRecipientDeliveryLimiter
	callProvider string
	accountSID   string
	authToken    string
	fromNumber   string
	phoneNumbers []string
	apiURL       string
}

func NewPhoneAlertDeliverySink(options PhoneAlertDeliverySinkOptions) *PhoneAlertDeliverySink {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	config := options.Provider.Config
	return &PhoneAlertDeliverySink{
		provider:     cloneAlertProviderConfig(options.Provider),
		client:       client,
		limiter:      options.Limiter,
		callProvider: strings.ToLower(strings.TrimSpace(config["provider"])),
		accountSID:   strings.TrimSpace(config["account_sid"]),
		authToken:    strings.TrimSpace(config["auth_token"]),
		fromNumber:   strings.TrimSpace(config["from_number"]),
		phoneNumbers: parseAlertRecipients(config["phone_numbers"]),
		apiURL:       strings.TrimSpace(config["api_url"]),
	}
}

func (s *PhoneAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelPhone
}

func (s *PhoneAlertDeliverySink) ProviderID() string {
	if s == nil {
		return ""
	}
	return s.provider.ID
}

func (s *PhoneAlertDeliverySink) ProviderKind() AlertProviderKind {
	if s == nil {
		return ""
	}
	return s.provider.Kind
}

func (s *PhoneAlertDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.client == nil {
		return ErrAlertDeliverySinkMissing
	}
	if len(s.phoneNumbers) == 0 {
		return fmt.Errorf("phone alert provider %s is missing phone_numbers", s.provider.ID)
	}
	if s.callProvider != "twilio" {
		return fmt.Errorf("unsupported phone alert provider %q", s.callProvider)
	}

	var deliveryErr error
	sent := 0
	for _, phoneNumber := range s.phoneNumbers {
		if !s.limiter.Allow(AlertDeliveryChannelPhone, s.provider.ID, phoneNumber, event.OccurredAt, phoneHourlyRecipientLimit) {
			continue
		}
		if err := s.deliverTwilioCall(ctx, phoneNumber, event); err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
			continue
		}
		sent++
	}
	if sent == 0 && deliveryErr == nil {
		return fmt.Errorf("%w: %s", ErrAlertRecipientHourlyLimit, s.provider.ID)
	}
	return deliveryErr
}

func (s *PhoneAlertDeliverySink) deliverTwilioCall(ctx context.Context, phoneNumber string, event AlertEvent) error {
	if s.accountSID == "" || s.authToken == "" || s.fromNumber == "" {
		return fmt.Errorf("Twilio phone alert provider %s is missing account_sid, auth_token, or from_number", s.provider.ID)
	}
	form := url.Values{}
	form.Set("To", phoneNumber)
	form.Set("From", s.fromNumber)
	form.Set("Twiml", twilioAlertCallTwiml(event))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, twilioCallsEndpoint(s.apiURL, s.accountSID), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build Twilio phone request for provider %s: %w", s.provider.ID, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(s.accountSID, s.authToken)

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("deliver Twilio phone alert provider %s: %w", s.provider.ID, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("Twilio phone alert provider %s returned %d: %s", s.provider.ID, response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func twilioMessagesEndpoint(apiURL, accountSID string) string {
	return twilioEndpoint(apiURL, accountSID, "Messages.json")
}

func twilioCallsEndpoint(apiURL, accountSID string) string {
	return twilioEndpoint(apiURL, accountSID, "Calls.json")
}

func twilioEndpoint(apiURL, accountSID, resource string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if base == "" {
		base = defaultTwilioAPIBaseURL
	}
	return base + "/2010-04-01/Accounts/" + url.PathEscape(accountSID) + "/" + resource
}

func alertSMSBody(event AlertEvent) string {
	return alertIMText(event)
}

func twilioAlertCallTwiml(event AlertEvent) string {
	return "<Response><Say>" + html.EscapeString(alertSMSBody(event)) + "</Say></Response>"
}

type ThirdPartyAlertDeliverySinkOptions struct {
	Provider   AlertProviderConfig
	HTTPClient *http.Client
}

type ThirdPartyAlertDeliverySink struct {
	provider   AlertProviderConfig
	endpoint   string
	routingKey string
	apiKey     string
	client     *http.Client
}

func NewThirdPartyAlertDeliverySink(options ThirdPartyAlertDeliverySinkOptions) *ThirdPartyAlertDeliverySink {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	config := options.Provider.Config
	return &ThirdPartyAlertDeliverySink{
		provider:   cloneAlertProviderConfig(options.Provider),
		endpoint:   thirdPartyProviderEndpoint(options.Provider.Kind, config),
		routingKey: strings.TrimSpace(config["routing_key"]),
		apiKey:     strings.TrimSpace(config["api_key"]),
		client:     client,
	}
}

func (s *ThirdPartyAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelThirdParty
}

func (s *ThirdPartyAlertDeliverySink) ProviderID() string {
	if s == nil {
		return ""
	}
	return s.provider.ID
}

func (s *ThirdPartyAlertDeliverySink) ProviderKind() AlertProviderKind {
	if s == nil {
		return ""
	}
	return s.provider.Kind
}

func (s *ThirdPartyAlertDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.client == nil {
		return ErrAlertDeliverySinkMissing
	}
	if s.endpoint == "" {
		return ErrAlertWebhookEndpointMissing
	}
	if s.provider.Kind == AlertProviderKindPagerDuty && s.routingKey == "" {
		return fmt.Errorf("PagerDuty alert provider %s is missing routing_key", s.provider.ID)
	}
	if s.provider.Kind == AlertProviderKindOpsgenie && s.apiKey == "" {
		return fmt.Errorf("Opsgenie alert provider %s is missing api_key", s.provider.ID)
	}

	body, err := json.Marshal(thirdPartyAlertPayload(s.provider.Kind, s.routingKey, event))
	if err != nil {
		return fmt.Errorf("encode third-party alert payload for provider %s: %w", s.provider.ID, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build third-party alert request for provider %s: %w", s.provider.ID, err)
	}
	request.Header.Set("Content-Type", "application/json")
	if s.provider.Kind == AlertProviderKindOpsgenie {
		request.Header.Set("Authorization", "GenieKey "+s.apiKey)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("deliver third-party alert provider %s: %w", s.provider.ID, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("third-party alert provider %s returned %d: %s", s.provider.ID, response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func thirdPartyProviderEndpoint(kind AlertProviderKind, config map[string]string) string {
	switch kind {
	case AlertProviderKindPagerDuty:
		if endpoint := strings.TrimSpace(config["api_url"]); endpoint != "" {
			return endpoint
		}
		return defaultPagerDutyEventsAPIURL
	case AlertProviderKindOpsgenie:
		if endpoint := strings.TrimSpace(config["api_url"]); endpoint != "" {
			return endpoint
		}
		return defaultOpsgenieAlertsAPIURL
	case AlertProviderKindAliyunMonitor, AlertProviderKindTencentCloud:
		return strings.TrimSpace(config["webhook_url"])
	default:
		return ""
	}
}

func thirdPartyAlertPayload(kind AlertProviderKind, routingKey string, event AlertEvent) map[string]any {
	switch kind {
	case AlertProviderKindPagerDuty:
		key := alertKey(event)
		return map[string]any{
			"routing_key":  routingKey,
			"event_action": "trigger",
			"dedup_key":    key,
			"payload": map[string]any{
				"summary":        alertThirdPartySummary(event),
				"source":         alertThirdPartySource(event),
				"severity":       pagerDutySeverity(event.Severity),
				"custom_details": thirdPartyAlertDetails(event),
			},
		}
	case AlertProviderKindOpsgenie:
		return map[string]any{
			"message":     alertThirdPartySummary(event),
			"alias":       alertKey(event),
			"description": strings.TrimSpace(event.Message),
			"priority":    opsgeniePriority(event.Severity),
			"details":     thirdPartyAlertDetails(event),
		}
	default:
		payload := map[string]any{
			"provider":  string(kind),
			"alert_key": alertKey(event),
			"severity":  string(event.Severity),
			"title":     alertThirdPartyTitle(event),
			"message":   strings.TrimSpace(event.Message),
			"component": strings.TrimSpace(event.Component),
		}
		if !event.OccurredAt.IsZero() {
			payload["occurred_at"] = event.OccurredAt.UTC().Format(time.RFC3339Nano)
		}
		return payload
	}
}

func thirdPartyAlertDetails(event AlertEvent) map[string]any {
	details := map[string]any{
		"alert_key": alertKey(event),
		"component": strings.TrimSpace(event.Component),
		"message":   strings.TrimSpace(event.Message),
		"severity":  string(event.Severity),
	}
	if !event.OccurredAt.IsZero() {
		details["occurred_at"] = event.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	for key, value := range event.Fields {
		if strings.TrimSpace(key) != "" {
			details[key] = value
		}
	}
	return details
}

func alertThirdPartySummary(event AlertEvent) string {
	title := alertThirdPartyTitle(event)
	message := strings.TrimSpace(event.Message)
	if message == "" {
		return title
	}
	return title + ": " + message
}

func alertThirdPartyTitle(event AlertEvent) string {
	if title := strings.TrimSpace(event.Title); title != "" {
		return title
	}
	if key := alertKey(event); key != "" {
		return key
	}
	return "Operational alert"
}

func alertThirdPartySource(event AlertEvent) string {
	if component := strings.TrimSpace(event.Component); component != "" {
		return component
	}
	return "oblivious"
}

func pagerDutySeverity(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityCritical:
		return "critical"
	case AlertSeverityWarning:
		return "warning"
	case AlertSeverityInfo:
		return "info"
	default:
		return "info"
	}
}

func opsgeniePriority(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityCritical:
		return "P1"
	case AlertSeverityWarning:
		return "P2"
	case AlertSeverityInfo:
		return "P3"
	default:
		return "P4"
	}
}

type SMTPAlertDeliverySinkOptions struct {
	Provider AlertProviderConfig
}

type SMTPAlertDeliverySink struct {
	provider   AlertProviderConfig
	host       string
	port       string
	username   string
	password   string
	fromEmail  string
	recipients []string
}

func NewSMTPAlertDeliverySink(options SMTPAlertDeliverySinkOptions) *SMTPAlertDeliverySink {
	config := options.Provider.Config
	return &SMTPAlertDeliverySink{
		provider:   cloneAlertProviderConfig(options.Provider),
		host:       strings.TrimSpace(config["smtp_host"]),
		port:       strings.TrimSpace(config["smtp_port"]),
		username:   strings.TrimSpace(config["username"]),
		password:   config["password"],
		fromEmail:  strings.TrimSpace(config["from_email"]),
		recipients: parseSMTPRecipients(config["recipients"]),
	}
}

func (s *SMTPAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelEmail
}

func (s *SMTPAlertDeliverySink) ProviderID() string {
	if s == nil {
		return ""
	}
	return s.provider.ID
}

func (s *SMTPAlertDeliverySink) ProviderKind() AlertProviderKind {
	if s == nil {
		return ""
	}
	return s.provider.Kind
}

func (s *SMTPAlertDeliverySink) Deliver(_ context.Context, event AlertEvent) error {
	if s == nil {
		return ErrAlertDeliverySinkMissing
	}
	if s.host == "" || s.port == "" || s.fromEmail == "" || len(s.recipients) == 0 {
		return fmt.Errorf("SMTP alert provider %s is missing envelope configuration", s.provider.ID)
	}
	address := net.JoinHostPort(s.host, s.port)
	var auth smtp.Auth
	if s.username != "" || s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	if err := smtp.SendMail(address, auth, s.fromEmail, s.recipients, smtpAlertMessage(s.fromEmail, s.recipients, event)); err != nil {
		return fmt.Errorf("deliver SMTP alert provider %s: %w", s.provider.ID, err)
	}
	return nil
}

func parseSMTPRecipients(value string) []string {
	return parseAlertRecipients(value)
}

func parseAlertRecipients(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	recipients := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	return recipients
}

func smtpAlertMessage(fromEmail string, recipients []string, event AlertEvent) []byte {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = alertKey(event)
	}
	if title == "" {
		title = "Operational alert"
	}
	subject := fmt.Sprintf("[%s] %s", strings.ToUpper(string(event.Severity)), title)
	body := []string{
		alertIMText(event),
	}
	if key := alertKey(event); key != "" {
		body = append(body, "", "alert_key: "+key)
	}
	if !event.OccurredAt.IsZero() {
		body = append(body, "occurred_at: "+event.OccurredAt.UTC().Format(time.RFC3339Nano))
	}
	headers := []string{
		"From: " + sanitizeSMTPHeader(fromEmail),
		"To: " + sanitizeSMTPHeader(strings.Join(recipients, ", ")),
		"Subject: " + sanitizeSMTPHeader(subject),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		strings.Join(body, "\r\n"),
	}
	return []byte(strings.Join(headers, "\r\n"))
}

func sanitizeSMTPHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func ProbeAlertProviderConfig(ctx context.Context, config AlertProviderConfig, testedAt time.Time, client *http.Client) AlertProviderTestResult {
	result := BuildAlertProviderTestResult(config, testedAt)
	if config.Status != AlertProviderStatusActive {
		return result
	}
	sink, err := alertProviderDeliverySinkForConfig(config, client, nil)
	if err != nil {
		result.OK = false
		result.Message = err.Error()
		return result
	}
	if sink == nil {
		return result
	}
	if err := sink.Deliver(ctx, AlertEvent{
		Key:        "alert-provider-test:" + strings.TrimSpace(config.ID),
		Severity:   AlertSeverityInfo,
		Title:      "Test alert provider: " + strings.TrimSpace(config.Name),
		Message:    "This is a test alert from Oblivious.",
		Component:  "observability",
		OccurredAt: testedAt,
	}); err != nil {
		result.OK = false
		result.Message = "provider delivery test failed: " + err.Error()
		return result
	}
	result.Message = "provider configuration validated"
	return result
}

type IMWebhookAlertDeliverySinkOptions struct {
	Provider   AlertProviderConfig
	HTTPClient *http.Client
}

type IMWebhookAlertDeliverySink struct {
	provider   AlertProviderConfig
	webhookURL string
	client     *http.Client
}

func NewIMWebhookAlertDeliverySink(options IMWebhookAlertDeliverySinkOptions) *IMWebhookAlertDeliverySink {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &IMWebhookAlertDeliverySink{
		provider:   cloneAlertProviderConfig(options.Provider),
		webhookURL: strings.TrimSpace(options.Provider.Config["webhook_url"]),
		client:     client,
	}
}

func (s *IMWebhookAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelIM
}

func (s *IMWebhookAlertDeliverySink) ProviderID() string {
	if s == nil {
		return ""
	}
	return s.provider.ID
}

func (s *IMWebhookAlertDeliverySink) ProviderKind() AlertProviderKind {
	if s == nil {
		return ""
	}
	return s.provider.Kind
}

func (s *IMWebhookAlertDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.client == nil {
		return ErrAlertDeliverySinkMissing
	}
	if s.webhookURL == "" {
		return ErrAlertWebhookEndpointMissing
	}
	body, err := json.Marshal(imAlertWebhookPayload(s.provider.Kind, event))
	if err != nil {
		return fmt.Errorf("encode IM alert payload for provider %s: %w", s.provider.ID, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build IM alert webhook request for provider %s: %w", s.provider.ID, err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("deliver IM alert provider %s: %w", s.provider.ID, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("IM alert provider %s returned %d: %s", s.provider.ID, response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func imAlertWebhookPayload(kind AlertProviderKind, event AlertEvent) map[string]any {
	text := alertIMText(event)
	switch kind {
	case AlertProviderKindFeishuWebhook:
		return map[string]any{
			"msg_type": "text",
			"content": map[string]any{
				"text": text,
			},
		}
	case AlertProviderKindDingTalk:
		return map[string]any{
			"msgtype": "text",
			"text": map[string]any{
				"content": text,
			},
		}
	case AlertProviderKindWeComWebhook:
		return map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]any{
				"content": text,
			},
		}
	default:
		payload := map[string]any{
			"text": text,
		}
		if key := alertKey(event); key != "" {
			payload["alert_key"] = key
		}
		if strings.TrimSpace(event.Component) != "" {
			payload["component"] = strings.TrimSpace(event.Component)
		}
		payload["severity"] = string(event.Severity)
		if !event.OccurredAt.IsZero() {
			payload["occurred_at"] = event.OccurredAt.UTC().Format(time.RFC3339Nano)
		}
		return payload
	}
}

func alertIMText(event AlertEvent) string {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = alertKey(event)
	}
	if title == "" {
		title = "Operational alert"
	}
	message := strings.TrimSpace(event.Message)
	parts := []string{fmt.Sprintf("[%s] %s", strings.ToUpper(string(event.Severity)), title)}
	if message != "" {
		parts = append(parts, message)
	}
	if strings.TrimSpace(event.Component) != "" {
		parts = append(parts, "component: "+strings.TrimSpace(event.Component))
	}
	return strings.Join(parts, "\n")
}

var _ AlertDeliverySinkResolver = (*AlertProviderDeliverySinkResolver)(nil)
var _ AlertDeliverySink = (*IMWebhookAlertDeliverySink)(nil)
var _ AlertProviderDeliveryMetadata = (*IMWebhookAlertDeliverySink)(nil)
var _ AlertDeliverySink = (*SMTPAlertDeliverySink)(nil)
var _ AlertProviderDeliveryMetadata = (*SMTPAlertDeliverySink)(nil)
var _ AlertDeliverySink = (*ThirdPartyAlertDeliverySink)(nil)
var _ AlertProviderDeliveryMetadata = (*ThirdPartyAlertDeliverySink)(nil)
var _ AlertDeliverySink = (*SMSAlertDeliverySink)(nil)
var _ AlertProviderDeliveryMetadata = (*SMSAlertDeliverySink)(nil)
var _ AlertDeliverySink = (*PhoneAlertDeliverySink)(nil)
var _ AlertProviderDeliveryMetadata = (*PhoneAlertDeliverySink)(nil)
