package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CommonConfig struct {
	Env         string
	DatabaseURL string
	DBMode      string

	SessionSecret       string
	SessionCookieName   string
	SessionCookieSecure bool
	SecretEncryptionKey string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	ObservabilityRequestLogBackend         string
	ClickHouseDSN                          string
	ClickHouseDriver                       string
	ObservabilityHTTPAlertsEnabled         bool
	AlertWebhookURL                        string
	AlertWebhookSecret                     string
	ObservabilityHTTPLatencySLOThresholdMS int
	ObservabilityHTTPRecoveryEnabled       bool
	ObservabilityHTTPRecoveryCooldownMS    int
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
	dbMode := strings.ToLower(strings.TrimSpace(os.Getenv("OBLIVIOUS_DB_MODE")))
	if dbMode == "" {
		dbMode = "monolith"
	}
	switch dbMode {
	case "monolith", "dual_write", "microservices":
	default:
		return CommonConfig{}, fmt.Errorf("invalid OBLIVIOUS_DB_MODE: %q", dbMode)
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		return CommonConfig{}, fmt.Errorf("SESSION_SECRET is required")
	}
	secretEncryptionKey := strings.TrimSpace(os.Getenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY"))

	sessionCookieName := strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = "oblivious_session"
	}

	sessionCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")), "true")
	if err := validateProductionSecretEncryptionConfig(env, sessionSecret, secretEncryptionKey); err != nil {
		return CommonConfig{}, err
	}

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
	observabilityHTTPLatencySLOThresholdMS := 5000
	if raw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return CommonConfig{}, fmt.Errorf("invalid OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS: %q", raw)
		}
		observabilityHTTPLatencySLOThresholdMS = parsed
	}

	observabilityHTTPRecoveryEnabledRaw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_ENABLED"))
	if observabilityHTTPRecoveryEnabledRaw == "" {
		observabilityHTTPRecoveryEnabledRaw = strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED"))
	}
	observabilityHTTPRecoveryEnabled := strings.EqualFold(observabilityHTTPRecoveryEnabledRaw, "true")
	observabilityHTTPRecoveryCooldownMS := 300000
	observabilityHTTPRecoveryCooldownName := "OBSERVABILITY_HTTP_RECOVERY_AUDIT_COOLDOWN_MS"
	raw := strings.TrimSpace(os.Getenv(observabilityHTTPRecoveryCooldownName))
	if raw == "" {
		observabilityHTTPRecoveryCooldownName = "OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS"
		raw = strings.TrimSpace(os.Getenv(observabilityHTTPRecoveryCooldownName))
	}
	if raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 {
			return CommonConfig{}, fmt.Errorf("invalid %s: %q", observabilityHTTPRecoveryCooldownName, raw)
		}
		observabilityHTTPRecoveryCooldownMS = parsed
	}

	return CommonConfig{
		Env:                                    env,
		DatabaseURL:                            databaseURL,
		DBMode:                                 dbMode,
		SessionSecret:                          sessionSecret,
		SessionCookieName:                      sessionCookieName,
		SessionCookieSecure:                    sessionCookieSecure,
		SecretEncryptionKey:                    secretEncryptionKey,
		RedisAddr:                              redisAddr,
		RedisPassword:                          redisPassword,
		RedisDB:                                redisDB,
		ObservabilityRequestLogBackend:         observabilityRequestLogBackend,
		ClickHouseDSN:                          clickHouseDSN,
		ClickHouseDriver:                       clickHouseDriver,
		ObservabilityHTTPAlertsEnabled:         observabilityHTTPAlertsEnabled,
		AlertWebhookURL:                        alertWebhookURL,
		AlertWebhookSecret:                     alertWebhookSecret,
		ObservabilityHTTPLatencySLOThresholdMS: observabilityHTTPLatencySLOThresholdMS,
		ObservabilityHTTPRecoveryEnabled:       observabilityHTTPRecoveryEnabled,
		ObservabilityHTTPRecoveryCooldownMS:    observabilityHTTPRecoveryCooldownMS,
	}, nil
}

func withServiceDatabaseURL(common CommonConfig, envKey string) CommonConfig {
	if common.DBMode == "monolith" {
		return common
	}
	if databaseURL := strings.TrimSpace(os.Getenv(envKey)); databaseURL != "" {
		common.DatabaseURL = databaseURL
	}
	return common
}

func validateProductionSecretEncryptionConfig(env, sessionSecret, secretEncryptionKey string) error {
	if !strings.EqualFold(env, "production") {
		return nil
	}
	trimmedKey := strings.TrimSpace(secretEncryptionKey)
	if trimmedKey == "" {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY is required when APP_ENV=production")
	}
	normalizedKey := strings.ToLower(trimmedKey)
	switch normalizedKey {
	case "change-me", "changeme", "test-secret", "secret", "dev-secret", "development":
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must not use a default value when APP_ENV=production")
	}
	if len(trimmedKey) < 32 {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must be at least 32 characters when APP_ENV=production")
	}
	if trimmedKey == strings.TrimSpace(sessionSecret) {
		return fmt.Errorf("OBLIVIOUS_SECRET_ENCRYPTION_KEY must be distinct from SESSION_SECRET when APP_ENV=production")
	}
	return nil
}
