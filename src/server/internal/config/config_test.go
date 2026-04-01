package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

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
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("SERVER_PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}
