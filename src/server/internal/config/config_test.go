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
