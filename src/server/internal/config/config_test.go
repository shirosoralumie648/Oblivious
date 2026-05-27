package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_TIMEOUT_MS", "")
	t.Setenv("MODEL_DEFAULT_NAME", "")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_SUCCESS_URL", "")
	t.Setenv("STRIPE_CANCEL_URL", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Fatalf("expected default env development, got %s", cfg.Env)
	}
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("expected empty origins, got %v", cfg.CORSAllowedOrigins)
	}
	if cfg.SessionCookieName != "oblivious_session" {
		t.Fatalf("expected default cookie name, got %s", cfg.SessionCookieName)
	}
	if cfg.SessionCookieSecure {
		t.Fatal("expected secure cookie default false")
	}
	if cfg.ModelDefaultName != "demo-reply" {
		t.Fatalf("expected default model name demo-reply, got %s", cfg.ModelDefaultName)
	}
	if cfg.LLMTimeoutMS != 30000 {
		t.Fatalf("expected default llm timeout 30000, got %d", cfg.LLMTimeoutMS)
	}
	if cfg.StripeSecretKey != "" || cfg.StripeWebhookSecret != "" {
		t.Fatal("expected stripe secrets to default empty")
	}
}

func TestLoadStripeConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_phase17")
	t.Setenv("STRIPE_SUCCESS_URL", "https://app.example.test/success")
	t.Setenv("STRIPE_CANCEL_URL", "https://app.example.test/cancel")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_phase17")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.StripeSecretKey != "sk_test_phase17" {
		t.Fatalf("expected stripe secret key, got %s", cfg.StripeSecretKey)
	}
	if cfg.StripeSuccessURL != "https://app.example.test/success" || cfg.StripeCancelURL != "https://app.example.test/cancel" {
		t.Fatalf("expected stripe URLs, got success=%s cancel=%s", cfg.StripeSuccessURL, cfg.StripeCancelURL)
	}
	if cfg.StripeWebhookSecret != "whsec_phase17" {
		t.Fatalf("expected stripe webhook secret, got %s", cfg.StripeWebhookSecret)
	}
}

func TestLoadRejectsInvalidLLMTimeout(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("LLM_TIMEOUT_MS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid llm timeout")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("SERVER_PORT", "abc")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "test-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing database url")
	}
}

func TestLoadRejectsMissingSessionSecret(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing session secret")
	}
}
