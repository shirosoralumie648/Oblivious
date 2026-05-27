package auth

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestPasswordPolicyResetAndSessionRevocation(t *testing.T) {
	database := testAuthDatabase(t)
	resetAuthSecurityTestTables(t, database)
	service := NewService(NewSQLStore(database))

	if _, err := service.Register(context.Background(), "weak@example.com", "secret"); err == nil {
		t.Fatal("expected weak registration password to be rejected")
	}

	session, err := service.Register(context.Background(), "secure@example.com", "StrongerPass1!")
	if err != nil {
		t.Fatalf("register strong password: %v", err)
	}
	secondSession, err := service.Login(context.Background(), "secure@example.com", "StrongerPass1!")
	if err != nil {
		t.Fatalf("login second session: %v", err)
	}

	resetToken, err := service.RequestPasswordReset(context.Background(), "secure@example.com")
	if err != nil {
		t.Fatalf("request password reset: %v", err)
	}
	if resetToken == "" {
		t.Fatal("expected password reset token for existing user")
	}

	if err := service.ConfirmPasswordReset(context.Background(), resetToken, "short"); err == nil {
		t.Fatal("expected weak reset password to be rejected")
	}
	if err := service.ConfirmPasswordReset(context.Background(), resetToken, "EvenStrongerPass2!"); err != nil {
		t.Fatalf("confirm password reset: %v", err)
	}

	if _, err := service.Session(context.Background(), session.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected first session to be revoked, got %v", err)
	}
	if _, err := service.Session(context.Background(), secondSession.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected second session to be revoked, got %v", err)
	}
	if _, err := service.Login(context.Background(), "secure@example.com", "StrongerPass1!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, err := service.Login(context.Background(), "secure@example.com", "EvenStrongerPass2!"); err != nil {
		t.Fatalf("expected new password login to work: %v", err)
	}
}

func TestSQLRateLimiterPersistsBlocks(t *testing.T) {
	database := testAuthDatabase(t)
	resetAuthSecurityTestTables(t, database)
	service := NewService(NewSQLStore(database))

	policy := RateLimitPolicy{
		Limit:         2,
		Window:        time.Minute,
		BlockDuration: 5 * time.Minute,
	}

	for i := 0; i < 2; i++ {
		if err := service.CheckRateLimit(context.Background(), "auth.login", "127.0.0.1:login@example.com", policy); err != nil {
			t.Fatalf("attempt %d should be allowed: %v", i+1, err)
		}
	}
	if err := service.CheckRateLimit(context.Background(), "auth.login", "127.0.0.1:login@example.com", policy); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected third attempt to be rate limited, got %v", err)
	}

	reloadedService := NewService(NewSQLStore(database))
	if err := reloadedService.CheckRateLimit(context.Background(), "auth.login", "127.0.0.1:login@example.com", policy); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected persisted rate limit block, got %v", err)
	}
}

func testAuthDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for auth integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	return database
}

func resetAuthSecurityTestTables(t *testing.T, database *sql.DB) {
	t.Helper()

	statements := []string{
		`DROP TABLE IF EXISTS password_reset_tokens`,
		`DROP TABLE IF EXISTS auth_rate_limits`,
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS workspaces`,
		`DROP TABLE IF EXISTS users`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMPTZ
		)`,
		`CREATE TABLE workspaces (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE auth_rate_limits (
			scope TEXT NOT NULL,
			key TEXT NOT NULL,
			window_start TIMESTAMPTZ NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			blocked_until TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (scope, key)
		)`,
		`CREATE TABLE password_reset_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare auth security test database: %v", err)
		}
	}
}
