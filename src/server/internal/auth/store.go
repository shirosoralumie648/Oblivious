package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *SQLStore) CreateUserWithWorkspace(ctx context.Context, email, passwordHash string) (Session, error) {
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

	if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, userID, email, passwordHash); err != nil {
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
		},
		WorkspaceID: workspaceID,
	}, nil
}

func (s *SQLStore) CreateSessionForUser(ctx context.Context, email, password string) (Session, error) {
	var storedPassword string
	var userID string

	if err := s.db.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE email = $1`, email).Scan(&userID, &storedPassword); err != nil {
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

	return Session{
		ExpiresAt: expiresAt,
		ID:        sessionID,
		User: User{
			Email: email,
			ID:    userID,
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
	if err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.workspace_id, s.expires_at, u.id, u.email
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > NOW()
	`, sessionID).Scan(&session.ID, &session.WorkspaceID, &session.ExpiresAt, &session.User.ID, &session.User.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}

	return session, nil
}
