package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/releasecontract"
)

type ContractLoader interface {
	Load(context.Context, string, string, string) (releasecontract.AuthoredContractV1, error)
}

type FileContractLoader struct{}

func (FileContractLoader) Load(ctx context.Context, repoRoot, contractPath, schemaPath string) (releasecontract.AuthoredContractV1, error) {
	return releasecontract.Load(ctx, repoRoot, contractPath, schemaPath)
}

type EntrypointPreflightOptions struct {
	RepoRoot     string
	ContractPath string
	SchemaPath   string
	ProfileID    string
	Contracts    ContractLoader
	Identities   buildinfo.IdentityProvider
	Profiles     releasecontract.ProfileResolver
}

type ResolvedEntrypointInputs struct {
	contract releasecontract.AuthoredContractV1
	profile  releasecontract.DeploymentProfile
	identity buildinfo.BuildIdentityV1
}

func (i ResolvedEntrypointInputs) Contract() releasecontract.AuthoredContractV1 {
	return mustCloneEntrypointValue(i.contract)
}

func (i ResolvedEntrypointInputs) Profile() releasecontract.DeploymentProfile {
	return mustCloneEntrypointValue(i.profile)
}

func (i ResolvedEntrypointInputs) Identity() buildinfo.BuildIdentityV1 {
	return i.identity
}

type EntrypointContinuation func(context.Context, ResolvedEntrypointInputs) error

func RunEntrypoint(ctx context.Context, id releasecontract.EntrypointID, options EntrypointPreflightOptions, normalStartup EntrypointContinuation) error {
	if ctx == nil {
		return fmt.Errorf("entrypoint preflight: context is required")
	}
	if strings.TrimSpace(options.RepoRoot) == "" || strings.TrimSpace(options.ContractPath) == "" || strings.TrimSpace(options.SchemaPath) == "" || strings.TrimSpace(options.ProfileID) == "" {
		return fmt.Errorf("entrypoint preflight: repo root, contract path, schema path, and profile are required")
	}
	if options.Contracts == nil || options.Identities == nil || options.Profiles == nil || normalStartup == nil {
		return fmt.Errorf("entrypoint preflight: contract loader, identity provider, profile resolver, and continuation are required")
	}

	contract, err := options.Contracts.Load(ctx, options.RepoRoot, options.ContractPath, options.SchemaPath)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: load contract: %w", err)
	}
	profile, err := options.Profiles.ResolveCommittedProfile(ctx, options.RepoRoot, options.ContractPath, options.SchemaPath, options.ProfileID)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: resolve profile: %w", err)
	}
	identity, err := options.Identities.Resolve(ctx, options.RepoRoot, options.ContractPath, options.SchemaPath)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: resolve build identity: %w", err)
	}
	if err := validateEntrypointInputs(contract, profile, identity, options.ProfileID, id); err != nil {
		return err
	}

	contractCopy, err := cloneEntrypointValue(contract)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: freeze contract: %w", err)
	}
	profileCopy, err := cloneEntrypointValue(profile)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: freeze profile: %w", err)
	}
	return normalStartup(ctx, ResolvedEntrypointInputs{contract: contractCopy, profile: profileCopy, identity: identity})
}

func validateEntrypointInputs(contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, identity buildinfo.BuildIdentityV1, profileID string, entrypointID releasecontract.EntrypointID) error {
	if err := buildinfo.ValidateIdentity(identity); err != nil {
		return fmt.Errorf("entrypoint preflight: validate build identity: %w", err)
	}
	digest, err := releasecontract.Digest(contract)
	if err != nil {
		return fmt.Errorf("entrypoint preflight: digest contract: %w", err)
	}
	if digest != identity.ContractDigest {
		return fmt.Errorf("entrypoint preflight: %w", &buildinfo.IdentityError{Code: buildinfo.ErrorContractDigestMismatch, Field: "contractDigest"})
	}
	if profile.ID != profileID {
		return fmt.Errorf("entrypoint preflight: resolved profile %q does not match requested profile %q", profile.ID, profileID)
	}
	var authored *releasecontract.DeploymentProfile
	for index := range contract.Profiles {
		if contract.Profiles[index].ID == profile.ID {
			authored = &contract.Profiles[index]
			break
		}
	}
	if authored == nil || !reflect.DeepEqual(*authored, profile) {
		return fmt.Errorf("entrypoint preflight: resolver profile is not the authored contract profile")
	}
	if err := releasecontract.RequireProfileEntrypoint(profile, entrypointID); err != nil {
		return fmt.Errorf("entrypoint preflight: authorize entrypoint: %w", err)
	}
	return nil
}

func cloneEntrypointValue[T any](value T) (T, error) {
	var cloned T
	encoded, err := json.Marshal(value)
	if err != nil {
		return cloned, err
	}
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return cloned, err
	}
	return cloned, nil
}

func mustCloneEntrypointValue[T any](value T) T {
	cloned, err := cloneEntrypointValue(value)
	if err != nil {
		panic(fmt.Sprintf("clone validated entrypoint input: %v", err))
	}
	return cloned
}

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
