package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CommonConfig struct {
	Env        string
	DatabaseURL string

	SessionSecret       string
	SessionCookieName   string
	SessionCookieSecure bool

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	ObservabilityRequestLogBackend      string
	ClickHouseDSN                       string
	ClickHouseDriver                    string
	ObservabilityHTTPAlertsEnabled      bool
	AlertWebhookURL                     string
	AlertWebhookSecret                  string
	ObservabilityHTTPRecoveryEnabled    bool
	ObservabilityHTTPRecoveryCooldownMS int
}

func LoadCommon() (CommonConfig, error) {
	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "development"
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return CommonConfig{}, fmt.Errorf("DATABASE_URL is required")
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		return CommonConfig{}, fmt.Errorf("SESSION_SECRET is required")
	}

	sessionCookieName := strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = "oblivious_session"
	}

	sessionCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")), "true")

	redisAddr, redisPassword, redisDB, err := loadRedisConfig()
	if err != nil {
		return CommonConfig{}, err
	}

	observabilityRequestLogBackend := strings.ToLower(strings.TrimSpace(os.Getenv("OBSERVABILITY_REQUEST_LOG_BACKEND")))
	if observabilityRequestLogBackend == "" {
		observabilityRequestLogBackend = "none"
	}
	if observabilityRequestLogBackend != "none" && observabilityRequestLogBackend != "clickhouse" {
		return CommonConfig{}, fmt.Errorf("invalid OBSERVABILITY_REQUEST_LOG_BACKEND: %q", observabilityRequestLogBackend)
	}

	clickHouseDSN := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
	if observabilityRequestLogBackend == "clickhouse" && clickHouseDSN == "" {
		return CommonConfig{}, fmt.Errorf("CLICKHOUSE_DSN is required when OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse")
	}

	clickHouseDriver := strings.TrimSpace(os.Getenv("CLICKHOUSE_DRIVER"))
	if clickHouseDriver == "" {
		clickHouseDriver = "clickhouse"
	}

	observabilityHTTPAlertsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_ALERTS_ENABLED")), "true")
	alertWebhookURL := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL"))
	alertWebhookSecret := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_SECRET"))

	observabilityHTTPRecoveryEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED")), "true")
	observabilityHTTPRecoveryCooldownMS := 300000
	if raw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return CommonConfig{}, fmt.Errorf("invalid OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS: %q", raw)
		}
		observabilityHTTPRecoveryCooldownMS = parsed
	}

	return CommonConfig{
		Env:                                 env,
		DatabaseURL:                         databaseURL,
		SessionSecret:                       sessionSecret,
		SessionCookieName:                   sessionCookieName,
		SessionCookieSecure:                 sessionCookieSecure,
		RedisAddr:                           redisAddr,
		RedisPassword:                       redisPassword,
		RedisDB:                             redisDB,
		ObservabilityRequestLogBackend:      observabilityRequestLogBackend,
		ClickHouseDSN:                       clickHouseDSN,
		ClickHouseDriver:                    clickHouseDriver,
		ObservabilityHTTPAlertsEnabled:      observabilityHTTPAlertsEnabled,
		AlertWebhookURL:                     alertWebhookURL,
		AlertWebhookSecret:                  alertWebhookSecret,
		ObservabilityHTTPRecoveryEnabled:    observabilityHTTPRecoveryEnabled,
		ObservabilityHTTPRecoveryCooldownMS: observabilityHTTPRecoveryCooldownMS,
	}, nil
}
