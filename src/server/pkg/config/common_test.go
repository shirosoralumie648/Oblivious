package config

import (
	"strings"
	"testing"
)

const productionSessionSecretForCommon = "0123456789abcdef0123456789abcdef"
const productionSecretEncryptionKeyForCommon = "abcdef0123456789abcdef0123456789"

func TestLoadCommonRejectsProductionWeakSecretEncryptionKey(t *testing.T) {
	for _, tc := range []struct {
		name        string
		key         string
		wantMessage string
	}{
		{name: "missing", key: "", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY is required"},
		{name: "default value", key: "change-me", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must not use a default value"},
		{name: "short value", key: "short-secretbox-key", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must be at least 32 characters"},
		{name: "same as session", key: productionSessionSecretForCommon, wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must be distinct from SESSION_SECRET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", "production")
			t.Setenv("DATABASE_URL", "postgres://main")
			t.Setenv("SESSION_SECRET", productionSessionSecretForCommon)
			t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", tc.key)

			_, err := LoadCommon()
			if err == nil {
				t.Fatal("expected error for weak production secret encryption key")
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q error, got %v", tc.wantMessage, err)
			}
		})
	}
}

func TestLoadCommonAcceptsProductionSecretEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("SESSION_SECRET", productionSessionSecretForCommon)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKeyForCommon)

	cfg, err := LoadCommon()
	if err != nil {
		t.Fatalf("expected production common config to load, got %v", err)
	}
	if cfg.SecretEncryptionKey != productionSecretEncryptionKeyForCommon {
		t.Fatalf("SecretEncryptionKey = %q, want configured key", cfg.SecretEncryptionKey)
	}
}

func TestLoadCommonObservabilityHTTPAlertConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_ALERTS_ENABLED", "true")
	t.Setenv("ALERT_WEBHOOK_URL", " https://ops.example.test/alerts ")
	t.Setenv("ALERT_WEBHOOK_SECRET", " alert-secret ")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED", "true")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS", "1500")
	t.Setenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS", "2500")

	cfg, err := LoadCommon()
	if err != nil {
		t.Fatalf("expected common config to load, got %v", err)
	}
	if !cfg.ObservabilityHTTPAlertsEnabled {
		t.Fatal("expected HTTP alerts to be enabled")
	}
	if cfg.AlertWebhookURL != "https://ops.example.test/alerts" || cfg.AlertWebhookSecret != "alert-secret" {
		t.Fatalf("expected trimmed alert webhook config, got url=%q secret=%q", cfg.AlertWebhookURL, cfg.AlertWebhookSecret)
	}
	if !cfg.ObservabilityHTTPRecoveryEnabled {
		t.Fatal("expected HTTP recovery to be enabled")
	}
	if cfg.ObservabilityHTTPRecoveryCooldownMS != 1500 {
		t.Fatalf("expected recovery cooldown 1500ms, got %d", cfg.ObservabilityHTTPRecoveryCooldownMS)
	}
	if cfg.ObservabilityHTTPLatencySLOThresholdMS != 2500 {
		t.Fatalf("expected latency SLO threshold 2500ms, got %d", cfg.ObservabilityHTTPLatencySLOThresholdMS)
	}
}

func TestLoadCommonObservabilityHTTPRecoveryAuditConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_ENABLED", "true")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_COOLDOWN_MS", "1750")

	cfg, err := LoadCommon()
	if err != nil {
		t.Fatalf("expected common config to load, got %v", err)
	}
	if !cfg.ObservabilityHTTPRecoveryEnabled {
		t.Fatal("expected HTTP recovery audit to be enabled")
	}
	if cfg.ObservabilityHTTPRecoveryCooldownMS != 1750 {
		t.Fatalf("expected recovery audit cooldown 1750ms, got %d", cfg.ObservabilityHTTPRecoveryCooldownMS)
	}
}

func TestLoadCommonRejectsInvalidObservabilityHTTPLatencySLOThreshold(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS", "0")

	_, err := LoadCommon()
	if err == nil {
		t.Fatal("expected error for invalid HTTP latency SLO threshold")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS") {
		t.Fatalf("expected latency SLO threshold error, got %v", err)
	}
}
