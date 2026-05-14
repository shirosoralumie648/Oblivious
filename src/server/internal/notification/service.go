package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
)

// Notification 通知
type Notification struct {
	ID        string         `json:"id"`
	UserID    string         `json:"userId"`
	Type      string         `json:"type"`     // 'info' | 'warning' | 'error' | 'success'
	Category  string         `json:"category"` // 'billing' | 'agent' | 'system' | 'mcp'
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	IsRead    bool           `json:"isRead"`
	ActionURL string         `json:"actionUrl,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	ReadAt    *time.Time     `json:"readAt,omitempty"`
}

// CreateNotificationRequest 创建通知请求
type CreateNotificationRequest struct {
	Type      string         `json:"type"`
	Category  string         `json:"category"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	ActionURL string         `json:"actionUrl,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Store 存储接口
type Store interface {
	Create(ctx context.Context, notification *Notification) (*Notification, error)
	Get(ctx context.Context, id string) (*Notification, error)
	List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*Notification, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id string) error
	GetUnreadCount(ctx context.Context, userID string) (int, error)
}

// Service 通知服务
type Service struct {
	store Store
}

// NewService 创建 Service
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Create 创建通知
func (s *Service) Create(ctx context.Context, userID string, req *CreateNotificationRequest) (*Notification, error) {
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	id, err := auth.NewID("notif")
	if err != nil {
		return nil, err
	}

	notifType := req.Type
	if notifType == "" {
		notifType = "info"
	}
	category := req.Category
	if category == "" {
		category = "system"
	}

	notification := &Notification{
		ID:        id,
		UserID:    userID,
		Type:      notifType,
		Category:  category,
		Title:     req.Title,
		Message:   req.Message,
		IsRead:    false,
		ActionURL: req.ActionURL,
		Metadata:  req.Metadata,
		CreatedAt: time.Now().UTC(),
	}

	return s.store.Create(ctx, notification)
}

// List 列出通知
func (s *Service) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.store.List(ctx, userID, unreadOnly, limit, offset)
}

// Get returns a notification by ID. A missing notification returns nil, nil.
func (s *Service) Get(ctx context.Context, id string) (*Notification, error) {
	return s.store.Get(ctx, id)
}

// MarkRead 标记已读
func (s *Service) MarkRead(ctx context.Context, id string) error {
	return s.store.MarkRead(ctx, id)
}

// MarkAllRead 标记所有已读
func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	return s.store.MarkAllRead(ctx, userID)
}

// Delete 删除通知
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

// GetUnreadCount 获取未读数量
func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return s.store.GetUnreadCount(ctx, userID)
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// Create 创建通知
func (s *SQLStore) Create(ctx context.Context, notification *Notification) (*Notification, error) {
	metadataJSON, _ := json.Marshal(notification.Metadata)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notifications (id, user_id, type, category, title, message, is_read, action_url, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, notification.ID, notification.UserID, notification.Type, notification.Category,
		notification.Title, notification.Message, notification.IsRead, notification.ActionURL,
		metadataJSON, notification.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	return notification, nil
}

// Get 获取通知
func (s *SQLStore) Get(ctx context.Context, id string) (*Notification, error) {
	var n Notification
	var metadataJSON []byte
	var actionURL sql.NullString
	var readAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, type, category, title, message, is_read, action_url, metadata, created_at, read_at
		FROM notifications WHERE id = $1
	`, id).Scan(&n.ID, &n.UserID, &n.Type, &n.Category, &n.Title, &n.Message, &n.IsRead,
		&actionURL, &metadataJSON, &n.CreatedAt, &readAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notification: %w", err)
	}

	n.ActionURL = actionURL.String
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &n.Metadata)
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}

	return &n, nil
}

// List 列出通知
func (s *SQLStore) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*Notification, error) {
	query := `
		SELECT id, user_id, type, category, title, message, is_read, action_url, metadata, created_at, read_at
		FROM notifications WHERE user_id = $1`
	args := []any{userID}
	argIdx := 2

	if unreadOnly {
		query += fmt.Sprintf(" AND is_read = false")
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		var n Notification
		var metadataJSON []byte
		var actionURL sql.NullString
		var readAt sql.NullTime

		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Category, &n.Title, &n.Message,
			&n.IsRead, &actionURL, &metadataJSON, &n.CreatedAt, &readAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		n.ActionURL = actionURL.String
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &n.Metadata)
		}
		if readAt.Valid {
			n.ReadAt = &readAt.Time
		}
		notifications = append(notifications, &n)
	}

	return notifications, rows.Err()
}

// MarkRead 标记已读
func (s *SQLStore) MarkRead(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET is_read = true, read_at = $2 WHERE id = $1
	`, id, now)
	return err
}

// MarkAllRead 标记所有已读
func (s *SQLStore) MarkAllRead(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET is_read = true, read_at = $2 WHERE user_id = $1 AND is_read = false
	`, userID, now)
	return err
}

// Delete 删除通知
func (s *SQLStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notifications WHERE id = $1`, id)
	return err
}

// GetUnreadCount 获取未读数量
func (s *SQLStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false
	`, userID).Scan(&count)
	return count, err
}
