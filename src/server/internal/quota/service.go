package quota

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
)

// Quota 用户配额
type Quota struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Balance   float64   `json:"balance"` // 余额 (USD)
	Used      float64   `json:"used"`    // 已使用 (USD)
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BillingSession 计费会话
type BillingSession struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	ChannelID        string     `json:"channelId,omitempty"`
	Model            string     `json:"model,omitempty"`
	APIType          string     `json:"apiType,omitempty"`
	IdempotencyKey   string     `json:"idempotencyKey"`
	PreAuthorizedAmt float64    `json:"preAuthorizedAmt"`
	SettledAmt       float64    `json:"settledAmt"`
	Status           string     `json:"status"` // 'preauthorized' | 'settled' | 'refunded'
	CreatedAt        time.Time  `json:"createdAt"`
	SettledAt        *time.Time `json:"settledAt,omitempty"`
}

// Package 套餐
type Package struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	QuotaAmount  float64   `json:"quotaAmount"`
	Price        float64   `json:"price"`
	DurationDays *int      `json:"durationDays,omitempty"`
	IsActive     bool      `json:"isActive"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Subscription 订阅
type Subscription struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	PackageID string     `json:"packageId"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"startedAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// TopupOrder 充值订单
type TopupOrder struct {
	ID        string     `json:"id"`
	UserID    string     `json:"userId"`
	Amount    float64    `json:"amount"`
	Money     float64    `json:"money"`
	Status    string     `json:"status"`
	TradeNo   string     `json:"tradeNo,omitempty"`
	PaidAt    *time.Time `json:"paidAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// Store 存储接口
type Store interface {
	// Quota
	GetOrCreateQuota(ctx context.Context, userID string) (*Quota, error)
	UpdateQuotaBalance(ctx context.Context, userID string, delta float64) error

	// Billing Session
	CreateBillingSession(ctx context.Context, session *BillingSession) (*BillingSession, error)
	GetBillingSessionByIdempotencyKey(ctx context.Context, key string) (*BillingSession, error)
	SettleBillingSession(ctx context.Context, id string, settledAmt float64) error
	RefundBillingSession(ctx context.Context, id string) error

	// Package
	ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error)
	GetPackage(ctx context.Context, id string) (*Package, error)

	// Subscription
	CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error)
	ListActiveSubscriptions(ctx context.Context, userID string) ([]*Subscription, error)

	// Topup
	CreateTopupOrder(ctx context.Context, order *TopupOrder) (*TopupOrder, error)
	UpdateTopupOrderStatus(ctx context.Context, id string, status string, tradeNo string) error
}

// Service 配额服务
type Service struct {
	store Store
}

// NewService 创建 Service
func NewService(store Store) *Service {
	return &Service{store: store}
}

// GetBalance 获取用户余额
func (s *Service) GetBalance(ctx context.Context, userID string) (*Quota, error) {
	return s.store.GetOrCreateQuota(ctx, userID)
}

// PreConsume 预扣配额
// 返回 billing session ID 用于后续结算
func (s *Service) PreConsume(ctx context.Context, userID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*BillingSession, error) {
	// 检查幂等性
	if idempotencyKey != "" {
		existing, err := s.store.GetBillingSessionByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			// 已存在，返回现有会话
			return existing, nil
		}
	}

	// 获取配额
	quota, err := s.store.GetOrCreateQuota(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 检查余额
	if quota.Balance < amount {
		return nil, fmt.Errorf("insufficient balance: have %.6f, need %.6f", quota.Balance, amount)
	}

	// 预扣
	if err := s.store.UpdateQuotaBalance(ctx, userID, -amount); err != nil {
		return nil, fmt.Errorf("pre-consume failed: %w", err)
	}

	// 创建 billing session
	sessionID, err := auth.NewID("bill")
	if err != nil {
		// 回滚
		s.store.UpdateQuotaBalance(ctx, userID, amount)
		return nil, err
	}

	session := &BillingSession{
		ID:               sessionID,
		UserID:           userID,
		ChannelID:        channelID,
		Model:            model,
		APIType:          apiType,
		IdempotencyKey:   idempotencyKey,
		PreAuthorizedAmt: amount,
		Status:           "preauthorized",
		CreatedAt:        time.Now().UTC(),
	}

	created, err := s.store.CreateBillingSession(ctx, session)
	if err != nil {
		// 回滚
		s.store.UpdateQuotaBalance(ctx, userID, amount)
		return nil, fmt.Errorf("create billing session: %w", err)
	}

	return created, nil
}

// Settle 结算计费
// actualAmount 是实际消费金额，差额会退还
func (s *Service) Settle(ctx context.Context, sessionID string, actualAmount float64) error {
	// 结算 billing session
	if err := s.store.SettleBillingSession(ctx, sessionID, actualAmount); err != nil {
		return err
	}

	return nil
}

// Refund 退款
// 全额退还预扣金额
func (s *Service) Refund(ctx context.Context, sessionID string) error {
	return s.store.RefundBillingSession(ctx, sessionID)
}

// Topup 充值
func (s *Service) Topup(ctx context.Context, userID string, amount float64) error {
	return s.store.UpdateQuotaBalance(ctx, userID, amount)
}

// ListPackages 列出套餐
func (s *Service) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	return s.store.ListPackages(ctx, activeOnly)
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// GetOrCreateQuota 获取或创建配额
func (s *SQLStore) GetOrCreateQuota(ctx context.Context, userID string) (*Quota, error) {
	// 尝试获取
	var quota Quota
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, balance, used, created_at, updated_at
		FROM quotas WHERE user_id = $1
	`, userID).Scan(&quota.ID, &quota.UserID, &quota.Balance, &quota.Used, &quota.CreatedAt, &quota.UpdatedAt)

	if err == nil {
		return &quota, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get quota: %w", err)
	}

	// 创建新配额
	id, err := auth.NewID("quota")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO quotas (id, user_id, balance, used, created_at, updated_at)
		VALUES ($1, $2, 0, 0, $3, $3)
	`, id, userID, now)
	if err != nil {
		return nil, fmt.Errorf("create quota: %w", err)
	}

	return &Quota{
		ID:        id,
		UserID:    userID,
		Balance:   0,
		Used:      0,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// UpdateQuotaBalance 更新配额余额
func (s *SQLStore) UpdateQuotaBalance(ctx context.Context, userID string, delta float64) error {
	now := time.Now().UTC()

	if delta > 0 {
		// 充值
		_, err := s.db.ExecContext(ctx, `
			UPDATE quotas SET balance = balance + $2, updated_at = $3 WHERE user_id = $1
		`, userID, delta, now)
		return err
	}

	// 消费
	_, err := s.db.ExecContext(ctx, `
		UPDATE quotas SET balance = balance + $2, used = used - $2, updated_at = $3 WHERE user_id = $1
	`, userID, delta, now)
	return err
}

// CreateBillingSession 创建计费会话
func (s *SQLStore) CreateBillingSession(ctx context.Context, session *BillingSession) (*BillingSession, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_sessions (id, user_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, session.ID, session.UserID, session.ChannelID, session.Model, session.APIType, session.IdempotencyKey, session.PreAuthorizedAmt, session.Status, session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert billing session: %w", err)
	}
	return session, nil
}

// GetBillingSessionByIdempotencyKey 通过幂等键获取会话
func (s *SQLStore) GetBillingSessionByIdempotencyKey(ctx context.Context, key string) (*BillingSession, error) {
	var session BillingSession
	var settledAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt, status, created_at, settled_at
		FROM billing_sessions WHERE idempotency_key = $1
	`, key).Scan(&session.ID, &session.UserID, &session.ChannelID, &session.Model, &session.APIType,
		&session.IdempotencyKey, &session.PreAuthorizedAmt, &session.SettledAmt, &session.Status, &session.CreatedAt, &settledAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get billing session: %w", err)
	}

	if settledAt.Valid {
		session.SettledAt = &settledAt.Time
	}

	return &session, nil
}

// SettleBillingSession 结算会话
func (s *SQLStore) SettleBillingSession(ctx context.Context, id string, settledAmt float64) error {
	now := time.Now().UTC()

	// 获取会话信息
	var session BillingSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, pre_authorized_amt, status FROM billing_sessions WHERE id = $1
	`, id).Scan(&session.UserID, &session.PreAuthorizedAmt, &session.Status)

	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}

	// 计算退款金额
	refundAmt := session.PreAuthorizedAmt - settledAmt

	// 更新会话状态
	_, err = s.db.ExecContext(ctx, `
		UPDATE billing_sessions SET status = 'settled', settled_amt = $2, settled_at = $3 WHERE id = $1
	`, id, settledAmt, now)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// 如果有差额，退还
	if refundAmt > 0 {
		if err := s.UpdateQuotaBalance(ctx, session.UserID, refundAmt); err != nil {
			return fmt.Errorf("refund difference: %w", err)
		}
	}

	return nil
}

// RefundBillingSession 退款会话
func (s *SQLStore) RefundBillingSession(ctx context.Context, id string) error {
	now := time.Now().UTC()

	// 获取会话信息
	var session BillingSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, pre_authorized_amt, status FROM billing_sessions WHERE id = $1
	`, id).Scan(&session.UserID, &session.PreAuthorizedAmt, &session.Status)

	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}

	// 更新会话状态
	_, err = s.db.ExecContext(ctx, `
		UPDATE billing_sessions SET status = 'refunded', settled_at = $2 WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// 退还全额
	if err := s.UpdateQuotaBalance(ctx, session.UserID, session.PreAuthorizedAmt); err != nil {
		return fmt.Errorf("refund: %w", err)
	}

	return nil
}

// ListPackages 列出套餐
func (s *SQLStore) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	query := `SELECT id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at FROM packages`
	if activeOnly {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY sort_order ASC, created_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		var p Package
		var durationDays sql.NullInt64
		var description sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &description, &p.QuotaAmount, &p.Price, &durationDays, &p.IsActive, &p.SortOrder, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}

		p.Description = description.String
		if durationDays.Valid {
			days := int(durationDays.Int64)
			p.DurationDays = &days
		}
		packages = append(packages, &p)
	}

	return packages, rows.Err()
}

// GetPackage 获取套餐
func (s *SQLStore) GetPackage(ctx context.Context, id string) (*Package, error) {
	var p Package
	var durationDays sql.NullInt64
	var description sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, quota_amount, price, duration_days, is_active, sort_order, created_at
		FROM packages WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &description, &p.QuotaAmount, &p.Price, &durationDays, &p.IsActive, &p.SortOrder, &p.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get package: %w", err)
	}

	p.Description = description.String
	if durationDays.Valid {
		days := int(durationDays.Int64)
		p.DurationDays = &days
	}

	return &p, nil
}

// CreateSubscription 创建订阅
func (s *SQLStore) CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, user_id, package_id, status, started_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sub.ID, sub.UserID, sub.PackageID, sub.Status, sub.StartedAt, sub.ExpiresAt, sub.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	return sub, nil
}

// ListActiveSubscriptions 列出活跃订阅
func (s *SQLStore) ListActiveSubscriptions(ctx context.Context, userID string) ([]*Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, package_id, status, started_at, expires_at, created_at
		FROM subscriptions WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		var sub Subscription
		var expiresAt sql.NullTime

		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.PackageID, &sub.Status, &sub.StartedAt, &expiresAt, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		if expiresAt.Valid {
			sub.ExpiresAt = &expiresAt.Time
		}
		subs = append(subs, &sub)
	}

	return subs, rows.Err()
}

// CreateTopupOrder 创建充值订单
func (s *SQLStore) CreateTopupOrder(ctx context.Context, order *TopupOrder) (*TopupOrder, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO topup_orders (id, user_id, amount, money, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, order.ID, order.UserID, order.Amount, order.Money, order.Status, order.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert topup order: %w", err)
	}
	return order, nil
}

// UpdateTopupOrderStatus 更新充值订单状态
func (s *SQLStore) UpdateTopupOrderStatus(ctx context.Context, id string, status string, tradeNo string) error {
	now := time.Now().UTC()

	var paidAt interface{}
	if status == "paid" {
		paidAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE topup_orders SET status = $2, trade_no = $3, paid_at = $4 WHERE id = $1
	`, id, status, tradeNo, paidAt)
	return err
}
