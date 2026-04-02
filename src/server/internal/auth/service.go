package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrSessionNotFound = errors.New("session not found")

type User struct {
	Email string
	ID    string
}

type Conversation struct {
	CreatedAt time.Time
	ID        string
	Title     string
	UpdatedAt time.Time
	UserID    string
}

type Session struct {
	ExpiresAt   time.Time
	ID          string
	User        User
	WorkspaceID string
}

type Store interface {
	CreateConversation(ctx context.Context, userID string) (Conversation, error)
	CreateUserWithWorkspace(ctx context.Context, email, passwordHash string) (Session, error)
	CreateSessionForUser(ctx context.Context, email, passwordHash string) (Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetConversationsByUser(ctx context.Context, userID string) ([]Conversation, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		now:   time.Now,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (Session, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, err
	}

	return s.store.CreateUserWithWorkspace(ctx, email, string(passwordHash))
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	return s.store.CreateSessionForUser(ctx, email, password)
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.store.DeleteSession(ctx, sessionID)
}

func (s *Service) Session(ctx context.Context, sessionID string) (Session, error) {
	return s.store.GetSession(ctx, sessionID)
}

func (s *Service) ListConversations(ctx context.Context, userID string) ([]Conversation, error) {
	return s.store.GetConversationsByUser(ctx, userID)
}

func (s *Service) StartConversation(ctx context.Context, userID string) (Conversation, error) {
	return s.store.CreateConversation(ctx, userID)
}

func NewID(prefix string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buffer)), nil
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
