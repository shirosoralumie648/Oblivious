package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrSessionNotFound = errors.New("session not found")
var ErrRateLimited = errors.New("rate limited")
var ErrInvalidResetToken = errors.New("invalid password reset token")

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
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
	CreatePasswordResetToken(ctx context.Context, email, tokenHash string, expiresAt time.Time) (bool, error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetConversationsByUser(ctx context.Context, userID string) ([]Conversation, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ConfirmPasswordReset(ctx context.Context, tokenHash, passwordHash string) error
	UseRateLimit(ctx context.Context, scope, key string, policy RateLimitPolicy, now time.Time) error
	RotateSession(ctx context.Context, sessionID string) (Session, error)
	RevokeUserSessions(ctx context.Context, userID, exceptSessionID string) error
}

type Service struct {
	store Store
	now   func() time.Time
}

type RateLimitPolicy struct {
	Limit         int
	Window        time.Duration
	BlockDuration time.Duration
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		now:   time.Now,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (Session, error) {
	if err := ValidatePasswordPolicy(password); err != nil {
		return Session{}, err
	}
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

func (s *Service) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	rawToken, tokenHash, err := newToken()
	if err != nil {
		return "", err
	}
	created, err := s.store.CreatePasswordResetToken(ctx, strings.ToLower(strings.TrimSpace(email)), tokenHash, s.now().Add(time.Hour))
	if err != nil {
		return "", err
	}
	if !created {
		return "", nil
	}
	return rawToken, nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.ConfirmPasswordReset(ctx, hashToken(rawToken), string(passwordHash))
}

func (s *Service) CheckRateLimit(ctx context.Context, scope, key string, policy RateLimitPolicy) error {
	if policy.Limit <= 0 {
		return errors.New("rate limit policy limit is required")
	}
	if policy.Window <= 0 {
		return errors.New("rate limit policy window is required")
	}
	if policy.BlockDuration <= 0 {
		return errors.New("rate limit policy block duration is required")
	}
	return s.store.UseRateLimit(ctx, scope, key, policy, s.now())
}

func (s *Service) RotateSession(ctx context.Context, sessionID string) (Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		return Session{}, errors.New("session id is required")
	}
	return s.store.RotateSession(ctx, sessionID)
}

func (s *Service) RevokeUserSessions(ctx context.Context, userID, exceptSessionID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user id is required")
	}
	return s.store.RevokeUserSessions(ctx, userID, exceptSessionID)
}

func ValidatePasswordPolicy(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	classes := 0
	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			symbol = true
		}
	}
	for _, ok := range []bool{lower, upper, digit, symbol} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("password must include at least three character classes")
	}
	return nil
}

func NewID(prefix string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(buffer)), nil
}

func newToken() (string, string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(buffer)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
