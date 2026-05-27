package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *SQLStore) CreateUserWithWorkspace(ctx context.Context, email, passwordHash string) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	userID, err := NewID("user")
	if err != nil {
		return Session{}, err
	}
	workspaceID, err := NewID("workspace")
	if err != nil {
		return Session{}, err
	}
	sessionID, err := NewID("session")
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().Add(24 * time.Hour)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, $3, 'user', $2)`, userID, email, passwordHash); err != nil {
		return Session{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, $3)`, workspaceID, userID, "Default Workspace"); err != nil {
		return Session{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id, user_id, workspace_id, expires_at) VALUES ($1, $2, $3, $4)`, sessionID, userID, workspaceID, expiresAt); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}

	return Session{
		ExpiresAt: expiresAt,
		ID:        sessionID,
		User: User{
			Email: email,
			ID:    userID,
			Name:  email,
			Role:  "user",
		},
		WorkspaceID: workspaceID,
	}, nil
}

func (s *SQLStore) CreateSessionForUser(ctx context.Context, email, password string) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var storedPassword string
	var userID string
	var userRole string
	var userName sql.NullString

	if err := s.db.QueryRowContext(ctx, `SELECT id, password_hash, COALESCE(role, 'user'), name FROM users WHERE email = $1`, email).Scan(&userID, &storedPassword, &userRole, &userName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)); err != nil {
		return Session{}, ErrInvalidCredentials
	}

	var workspaceID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1`, userID).Scan(&workspaceID); err != nil {
		return Session{}, err
	}

	sessionID, err := NewID("session")
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (id, user_id, workspace_id, expires_at) VALUES ($1, $2, $3, $4)`, sessionID, userID, workspaceID, expiresAt); err != nil {
		return Session{}, err
	}

	// Update last_login_at
	s.db.ExecContext(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, userID)

	name, role := normalizeUserFields(email, userName.String, userRole)

	return Session{
		ExpiresAt: expiresAt,
		ID:        sessionID,
		User: User{
			Email: email,
			ID:    userID,
			Name:  name,
			Role:  role,
		},
		WorkspaceID: workspaceID,
	}, nil
}

func (s *SQLStore) CreateConversation(ctx context.Context, userID string) (Conversation, error) {
	conversationID, err := NewID("conversation")
	if err != nil {
		return Conversation{}, err
	}

	now := time.Now().UTC()
	conversation := Conversation{
		CreatedAt: now,
		ID:        conversationID,
		Title:     "New conversation",
		UpdatedAt: now,
		UserID:    userID,
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO conversations (id, user_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`, conversation.ID, conversation.UserID, conversation.Title, conversation.CreatedAt, conversation.UpdatedAt); err != nil {
		return Conversation{}, err
	}

	return conversation, nil
}

func (s *SQLStore) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

func (s *SQLStore) GetConversationsByUser(ctx context.Context, userID string) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, created_at, updated_at
		FROM conversations
		WHERE user_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := []Conversation{}
	for rows.Next() {
		var conversation Conversation
		conversation.UserID = userID
		if err := rows.Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return conversations, nil
}

func (s *SQLStore) GetSession(ctx context.Context, sessionID string) (Session, error) {
	var session Session
	var userName sql.NullString
	var userRole sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.workspace_id, s.expires_at, u.id, u.email, u.name, COALESCE(u.role, 'user')
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sessionID).Scan(&session.ID, &session.WorkspaceID, &session.ExpiresAt, &session.User.ID, &session.User.Email, &userName, &userRole); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	session.User.Name, session.User.Role = normalizeUserFields(session.User.Email, userName.String, userRole.String)

	return session, nil
}

func (s *SQLStore) CreatePasswordResetToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) (bool, error) {
	var userID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	tokenID, err := NewID("reset")
	if err != nil {
		return false, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, tokenID, userID, tokenHash, expiresAt); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLStore) ConfirmPasswordReset(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID string
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, tokenHash).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidResetToken
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at = NOW() WHERE token_hash = $1`, tokenHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLStore) UseRateLimit(ctx context.Context, scope, key string, policy RateLimitPolicy, now time.Time) error {
	var windowStart time.Time
	var attempts int
	var blockedUntil sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT window_start, attempts, blocked_until
		FROM auth_rate_limits
		WHERE scope = $1 AND key = $2
	`, scope, key).Scan(&windowStart, &attempts, &blockedUntil)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if blockedUntil.Valid && blockedUntil.Time.After(now) {
		return ErrRateLimited
	}
	if errors.Is(err, sql.ErrNoRows) || now.Sub(windowStart) >= policy.Window {
		_, execErr := s.db.ExecContext(ctx, `
			INSERT INTO auth_rate_limits (scope, key, window_start, attempts, blocked_until, updated_at)
			VALUES ($1, $2, $3, 1, NULL, $3)
			ON CONFLICT (scope, key) DO UPDATE SET
				window_start = EXCLUDED.window_start,
				attempts = 1,
				blocked_until = NULL,
				updated_at = EXCLUDED.updated_at
		`, scope, key, now)
		return execErr
	}
	attempts++
	if attempts > policy.Limit {
		_, execErr := s.db.ExecContext(ctx, `
			UPDATE auth_rate_limits
			SET attempts = $3, blocked_until = $4, updated_at = $5
			WHERE scope = $1 AND key = $2
		`, scope, key, attempts, now.Add(policy.BlockDuration), now)
		if execErr != nil {
			return execErr
		}
		return ErrRateLimited
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE auth_rate_limits
		SET attempts = $3, updated_at = $4
		WHERE scope = $1 AND key = $2
	`, scope, key, attempts, now)
	return err
}

func (s *SQLStore) RotateSession(ctx context.Context, sessionID string) (Session, error) {
	var userID string
	var workspaceID string
	if err := s.db.QueryRowContext(ctx, `
		SELECT user_id, workspace_id
		FROM sessions
		WHERE id = $1 AND expires_at > NOW()
	`, sessionID).Scan(&userID, &workspaceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	newSessionID, err := NewID("session")
	if err != nil {
		return Session{}, err
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		return Session{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id, user_id, workspace_id, expires_at) VALUES ($1, $2, $3, $4)`, newSessionID, userID, workspaceID, expiresAt); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, newSessionID)
}

func (s *SQLStore) RevokeUserSessions(ctx context.Context, userID, exceptSessionID string) error {
	if exceptSessionID == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1 AND id <> $2`, userID, exceptSessionID)
	return err
}

func normalizeUserFields(email, name, role string) (string, string) {
	if strings.TrimSpace(name) == "" {
		name = email
	}
	if strings.TrimSpace(role) == "" {
		role = "user"
	}
	return name, role
}
