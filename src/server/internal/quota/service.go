package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/ratelimit"
)

// Quota 用户配额
type Quota struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	UserID         string    `json:"userId"`
	Balance        float64   `json:"balance"` // 余额 (USD)
	Used           float64   `json:"used"`    // 已使用 (USD)
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// BillingSession 计费会话
type BillingSession struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationId"`
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
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description,omitempty"`
	QuotaAmount         float64   `json:"quotaAmount"`
	TokenQuota          int       `json:"tokenQuota"`
	Price               float64   `json:"price"`
	ModelAccess         []string  `json:"modelAccess"`
	AgentLimit          int       `json:"agentLimit"`
	MaxTokensPerRequest int       `json:"maxTokensPerRequest"`
	DurationDays        *int      `json:"durationDays,omitempty"`
	IsActive            bool      `json:"isActive"`
	IsPublic            bool      `json:"isPublic"`
	SortOrder           int       `json:"sortOrder"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// Subscription 订阅
type Subscription struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	UserID         string     `json:"userId"`
	PackageID      string     `json:"packageId"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"startedAt"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// TopupOrder 充值订单
type TopupOrder struct {
	ID                        string     `json:"id"`
	OrganizationID            string     `json:"organizationId"`
	UserID                    string     `json:"userId"`
	PaymentIntentID           string     `json:"paymentIntentId,omitempty"`
	ProviderCheckoutSessionID string     `json:"providerCheckoutSessionId,omitempty"`
	Amount                    float64    `json:"amount"`
	Money                     float64    `json:"money"`
	Status                    string     `json:"status"`
	TradeNo                   string     `json:"tradeNo,omitempty"`
	RefundedAmount            float64    `json:"refundedAmount"`
	PaidAt                    *time.Time `json:"paidAt,omitempty"`
	CreatedAt                 time.Time  `json:"createdAt"`
}

// Store 存储接口
type Store interface {
	// Quota
	GetOrCreateQuota(ctx context.Context, userID, organizationID string) (*Quota, error)
	UpdateQuotaBalance(ctx context.Context, userID, organizationID string, delta float64) error

	// Billing Session
	CreateBillingSession(ctx context.Context, session *BillingSession) (*BillingSession, error)
	GetBillingSessionByIdempotencyKey(ctx context.Context, key, organizationID string) (*BillingSession, error)
	SettleBillingSession(ctx context.Context, id, organizationID string, settledAmt float64) error
	RefundBillingSession(ctx context.Context, id, organizationID string) error

	// Package
	ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error)
	GetPackage(ctx context.Context, id string) (*Package, error)

	// Subscription
	CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error)
	ListActiveSubscriptions(ctx context.Context, userID, organizationID string) ([]*Subscription, error)

	// Topup
	CreateTopupOrder(ctx context.Context, order *TopupOrder) (*TopupOrder, error)
	UpdateTopupOrderCheckoutSession(ctx context.Context, organizationID, paymentIntentID string, providerCheckoutSessionID string) error
	MarkTopupOrderFailedByPaymentIntent(ctx context.Context, organizationID, paymentIntentID string) error
	UpdateTopupOrderStatus(ctx context.Context, id, organizationID string, status string, tradeNo string) error

	// Usage limits
	SaveUsageLimitSettings(ctx context.Context, settings UsageLimitSettings) (*UsageLimitSettings, error)
	ResolveUsageLimitSettings(ctx context.Context, organizationID, userID string) (UsageLimitSettings, error)
	ListUsageLimitSettings(ctx context.Context, organizationID string) ([]UsageLimitSettings, error)
}

var ErrUsageLimited = errors.New("usage limit exceeded")

const defaultUsageLimitWindowSeconds = 60
const (
	quotaScopeOrganization = "organization"
	quotaScopeUser         = "user"
)

type UsageLimitDimension string

const (
	UsageLimitDimensionConcurrent    UsageLimitDimension = "concurrent"
	UsageLimitDimensionTokens        UsageLimitDimension = "tokens"
	UsageLimitDimensionRequestTokens UsageLimitDimension = "request_tokens"
)

type UsageLimit struct {
	OrganizationID        string
	UserID                string
	MaxConcurrentRequests int
	MaxTokensPerWindow    int
	MaxTokensPerRequest   int
}

type UsageLimitSettings struct {
	OrganizationID        string    `json:"organizationId"`
	UserID                string    `json:"userId,omitempty"`
	QuotaMode             string    `json:"quotaMode,omitempty"`
	MaxConcurrentRequests int       `json:"maxConcurrentRequests"`
	WindowSeconds         int       `json:"windowSeconds"`
	MaxTokensPerWindow    int       `json:"maxTokensPerWindow"`
	MaxTokensPerRequest   int       `json:"maxTokensPerRequest"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type UsageLease struct {
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId,omitempty"`
}

type UsageLimitError struct {
	Dimension  UsageLimitDimension
	Limit      int
	Current    int
	Remaining  int
	RetryAfter time.Duration
}

func (e *UsageLimitError) Error() string {
	return fmt.Sprintf("usage limit exceeded: %s", e.Dimension)
}

func (e *UsageLimitError) Unwrap() error {
	return ErrUsageLimited
}

// Service 配额服务
type Service struct {
	store       Store
	rateLimiter ratelimit.RateLimiter
}

type ServiceOption func(*Service)

func WithRateLimiter(limiter ratelimit.RateLimiter) ServiceOption {
	return func(s *Service) {
		s.rateLimiter = limiter
	}
}

// NewService 创建 Service
func NewService(store Store, opts ...ServiceOption) *Service {
	service := &Service{store: store}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// GetBalance 获取用户余额
func (s *Service) GetBalance(ctx context.Context, userID, organizationID string) (*Quota, error) {
	return s.store.GetOrCreateQuota(ctx, userID, organizationID)
}

func (s *Service) BeginUsage(ctx context.Context, limit UsageLimit) (*UsageLease, error) {
	if err := validateUsageLimitIdentity(limit); err != nil {
		return nil, err
	}
	if s.rateLimiter == nil || limit.MaxConcurrentRequests <= 0 {
		return &UsageLease{OrganizationID: limit.OrganizationID, UserID: limit.UserID}, nil
	}
	if err := s.rateLimiter.Begin(ctx, usageLimitRateKey(limit), ratelimit.Limits{MaxConcurrent: limit.MaxConcurrentRequests}); err != nil {
		return nil, usageLimitError(err, UsageLimitDimensionConcurrent)
	}
	return &UsageLease{OrganizationID: limit.OrganizationID, UserID: limit.UserID}, nil
}

func (s *Service) EndUsage(ctx context.Context, lease *UsageLease) error {
	if lease == nil || s.rateLimiter == nil {
		return nil
	}
	return s.rateLimiter.End(ctx, usageLimitRateKey(UsageLimit{OrganizationID: lease.OrganizationID, UserID: lease.UserID}))
}

func (s *Service) ReserveUsageTokens(ctx context.Context, limit UsageLimit, tokens int) error {
	if err := validateUsageLimitIdentity(limit); err != nil {
		return err
	}
	if tokens <= 0 || s.rateLimiter == nil || (limit.MaxTokensPerWindow <= 0 && limit.MaxTokensPerRequest <= 0) {
		return nil
	}
	if err := s.rateLimiter.Allow(ctx, usageLimitRateKey(limit), ratelimit.Limits{
		TPM:                 limit.MaxTokensPerWindow,
		MaxTokensPerRequest: limit.MaxTokensPerRequest,
	}, ratelimit.Usage{Tokens: tokens, RequestTokens: tokens}); err != nil {
		return usageLimitError(err, usageLimitDimensionFromRateLimitError(err, UsageLimitDimensionTokens))
	}
	return nil
}

func (s *Service) SaveUsageLimitSettings(ctx context.Context, settings UsageLimitSettings) (*UsageLimitSettings, error) {
	normalized, err := normalizeUsageLimitSettings(settings)
	if err != nil {
		return nil, err
	}
	return s.store.SaveUsageLimitSettings(ctx, normalized)
}

func (s *Service) ResolveUsageLimit(ctx context.Context, organizationID, userID string) (UsageLimit, error) {
	if organizationID == "" {
		return UsageLimit{}, fmt.Errorf("organization_id is required")
	}
	settings, err := s.store.ResolveUsageLimitSettings(ctx, organizationID, userID)
	if err != nil {
		return UsageLimit{}, err
	}
	return UsageLimit{
		OrganizationID:        organizationID,
		UserID:                settings.UserID,
		MaxConcurrentRequests: settings.MaxConcurrentRequests,
		MaxTokensPerWindow:    settings.MaxTokensPerWindow,
		MaxTokensPerRequest:   s.resolveSubscriptionRequestTokenCap(ctx, organizationID, userID, settings.MaxTokensPerRequest),
	}, nil
}

func (s *Service) resolveSubscriptionRequestTokenCap(ctx context.Context, organizationID, userID string, configured int) int {
	if configured > 0 || userID == "" {
		return configured
	}
	subscriptions, err := s.store.ListActiveSubscriptions(ctx, userID, organizationID)
	if err != nil {
		return configured
	}
	cap := configured
	for _, sub := range subscriptions {
		if sub == nil || sub.PackageID == "" {
			continue
		}
		pkg, err := s.store.GetPackage(ctx, sub.PackageID)
		if err != nil || pkg == nil || !pkg.IsActive {
			continue
		}
		if pkg.MaxTokensPerRequest > cap {
			cap = pkg.MaxTokensPerRequest
		}
	}
	return cap
}

func (s *Service) ListUsageLimitSettings(ctx context.Context, organizationID string) ([]UsageLimitSettings, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	return s.store.ListUsageLimitSettings(ctx, organizationID)
}

func validateUsageLimitIdentity(limit UsageLimit) error {
	if limit.OrganizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	return nil
}

func normalizeUsageLimitSettings(settings UsageLimitSettings) (UsageLimitSettings, error) {
	if settings.OrganizationID == "" {
		return UsageLimitSettings{}, fmt.Errorf("organization_id is required")
	}
	if settings.MaxConcurrentRequests < 0 {
		return UsageLimitSettings{}, fmt.Errorf("max_concurrent_requests must be non-negative")
	}
	if settings.MaxTokensPerWindow < 0 {
		return UsageLimitSettings{}, fmt.Errorf("max_tokens_per_window must be non-negative")
	}
	if settings.MaxTokensPerRequest < 0 {
		return UsageLimitSettings{}, fmt.Errorf("max_tokens_per_request must be non-negative")
	}
	if settings.MaxConcurrentRequests == 0 && settings.MaxTokensPerWindow == 0 && settings.MaxTokensPerRequest == 0 {
		return UsageLimitSettings{}, fmt.Errorf("at least one usage limit must be greater than zero")
	}
	if settings.WindowSeconds < 0 {
		return UsageLimitSettings{}, fmt.Errorf("window_seconds must be non-negative")
	}
	if settings.WindowSeconds == 0 {
		settings.WindowSeconds = defaultUsageLimitWindowSeconds
	}
	switch settings.QuotaMode {
	case "":
		settings.QuotaMode = usageLimitQuotaMode(settings.UserID)
	case "organization":
		if settings.UserID != "" {
			return UsageLimitSettings{}, fmt.Errorf("user_id must be empty for organization quota mode")
		}
	case "user":
		if settings.UserID == "" {
			return UsageLimitSettings{}, fmt.Errorf("user_id is required for user quota mode")
		}
	default:
		return UsageLimitSettings{}, fmt.Errorf("quota_mode must be organization or user")
	}
	return settings, nil
}

func usageLimitQuotaMode(userID string) string {
	if userID == "" {
		return "organization"
	}
	return "user"
}

func usageLimitRateKey(limit UsageLimit) ratelimit.Key {
	tokenID := limit.UserID
	if tokenID == "" {
		tokenID = limit.OrganizationID
	}
	return ratelimit.Key{
		ChannelID: "quota",
		Model:     limit.OrganizationID,
		TokenID:   tokenID,
	}
}

func usageLimitError(err error, dimension UsageLimitDimension) error {
	var limitErr *ratelimit.LimitError
	if errors.As(err, &limitErr) {
		return &UsageLimitError{
			Dimension:  dimension,
			Limit:      limitErr.Limit,
			Current:    limitErr.Current,
			Remaining:  limitErr.Remaining,
			RetryAfter: limitErr.RetryAfter,
		}
	}
	if errors.Is(err, ratelimit.ErrRateLimited) {
		return &UsageLimitError{Dimension: dimension}
	}
	return err
}

func usageLimitDimensionFromRateLimitError(err error, fallback UsageLimitDimension) UsageLimitDimension {
	var limitErr *ratelimit.LimitError
	if errors.As(err, &limitErr) && limitErr.Dimension == ratelimit.DimensionRequestTokens {
		return UsageLimitDimensionRequestTokens
	}
	return fallback
}

// PreConsume 预扣配额
// 返回 billing session ID 用于后续结算
func (s *Service) PreConsume(ctx context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*BillingSession, error) {
	ctx, span := observability.StartSpan(ctx, "quota.preconsume", observability.String("quota.stage", "preauthorization"))
	defer span.End()

	if organizationID == "" {
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, fmt.Errorf("organization_id is required")
	}

	// 检查幂等性
	if idempotencyKey != "" {
		existing, err := s.store.GetBillingSessionByIdempotencyKey(ctx, idempotencyKey, organizationID)
		if err != nil {
			metrics.RecordQuotaSettlementFailure("preauthorization")
			return nil, err
		}
		if existing != nil {
			// 已存在，返回现有会话
			return existing, nil
		}
	}

	// 获取配额
	quota, err := s.store.GetOrCreateQuota(ctx, userID, organizationID)
	if err != nil {
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, err
	}

	// 检查余额
	if quota.Balance < amount {
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, fmt.Errorf("insufficient balance: have %.6f, need %.6f", quota.Balance, amount)
	}

	// 预扣
	if err := s.store.UpdateQuotaBalance(ctx, userID, organizationID, -amount); err != nil {
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, fmt.Errorf("pre-consume failed: %w", err)
	}

	// 创建 billing session
	sessionID, err := auth.NewID("bill")
	if err != nil {
		// 回滚
		s.store.UpdateQuotaBalance(ctx, userID, organizationID, amount)
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, err
	}

	session := &BillingSession{
		ID:               sessionID,
		OrganizationID:   organizationID,
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
		s.store.UpdateQuotaBalance(ctx, userID, organizationID, amount)
		metrics.RecordQuotaSettlementFailure("preauthorization")
		return nil, fmt.Errorf("create billing session: %w", err)
	}

	metrics.RecordBillingLifecycleEvent("quota_preauthorization", "preauthorized")
	return created, nil
}

// Settle 结算计费
// actualAmount 是实际消费金额，差额会退还
func (s *Service) Settle(ctx context.Context, organizationID, sessionID string, actualAmount float64) error {
	ctx, span := observability.StartSpan(ctx, "quota.settlement", observability.String("quota.stage", "settlement"))
	defer span.End()

	if organizationID == "" {
		metrics.RecordQuotaSettlementFailure("settlement")
		return fmt.Errorf("organization_id is required")
	}

	// 结算 billing session
	if err := s.store.SettleBillingSession(ctx, sessionID, organizationID, actualAmount); err != nil {
		metrics.RecordQuotaSettlementFailure("settlement")
		return err
	}

	metrics.RecordBillingLifecycleEvent("quota_settlement", "settled")
	return nil
}

// Refund 退款
// 全额退还预扣金额
func (s *Service) Refund(ctx context.Context, organizationID, sessionID string) error {
	ctx, span := observability.StartSpan(ctx, "quota.refund", observability.String("quota.stage", "refund"))
	defer span.End()

	if organizationID == "" {
		metrics.RecordQuotaSettlementFailure("refund")
		return fmt.Errorf("organization_id is required")
	}

	if err := s.store.RefundBillingSession(ctx, sessionID, organizationID); err != nil {
		metrics.RecordQuotaSettlementFailure("refund")
		return err
	}
	metrics.RecordBillingLifecycleEvent("quota_refund", "refunded")
	return nil
}

// Topup 充值
func (s *Service) Topup(ctx context.Context, userID, organizationID string, amount float64) error {
	if organizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	if _, err := s.store.GetOrCreateQuota(ctx, userID, organizationID); err != nil {
		return err
	}
	return s.store.UpdateQuotaBalance(ctx, userID, organizationID, amount)
}

func (s *Service) CreatePendingTopup(ctx context.Context, userID, organizationID, paymentIntentID string, amount float64, money float64) (*TopupOrder, error) {
	if organizationID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	id, err := auth.NewID("topup")
	if err != nil {
		return nil, err
	}
	order := &TopupOrder{
		ID:              id,
		OrganizationID:  organizationID,
		UserID:          userID,
		PaymentIntentID: paymentIntentID,
		Amount:          amount,
		Money:           money,
		Status:          "pending",
		CreatedAt:       time.Now().UTC(),
	}
	return s.store.CreateTopupOrder(ctx, order)
}

func (s *Service) SetTopupCheckoutSession(ctx context.Context, organizationID, paymentIntentID string, providerCheckoutSessionID string) error {
	if organizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	if paymentIntentID == "" {
		return fmt.Errorf("payment_intent_id is required")
	}
	if providerCheckoutSessionID == "" {
		return fmt.Errorf("provider_checkout_session_id is required")
	}
	return s.store.UpdateTopupOrderCheckoutSession(ctx, organizationID, paymentIntentID, providerCheckoutSessionID)
}

func (s *Service) MarkTopupCheckoutFailed(ctx context.Context, organizationID, paymentIntentID string) error {
	if organizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	if paymentIntentID == "" {
		return fmt.Errorf("payment_intent_id is required")
	}
	return s.store.MarkTopupOrderFailedByPaymentIntent(ctx, organizationID, paymentIntentID)
}

// ListPackages 列出套餐
func (s *Service) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	return s.store.ListPackages(ctx, activeOnly)
}

func (s *Service) GetPackage(ctx context.Context, id string) (*Package, error) {
	return s.store.GetPackage(ctx, id)
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) quotaBalanceScope(ctx context.Context, userID, organizationID string) (string, error) {
	if userID == "" {
		return quotaScopeOrganization, nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM concurrency_limits
			WHERE organization_id = $1 AND user_id = $2
			UNION ALL
			SELECT 1 FROM token_rate_limits
			WHERE organization_id = $1 AND user_id = $2
			LIMIT 1
		)
	`, organizationID, userID).Scan(&exists); err != nil {
		return "", fmt.Errorf("resolve quota balance scope: %w", err)
	}
	if exists {
		return quotaScopeUser, nil
	}
	return quotaScopeOrganization, nil
}

// GetOrCreateQuota 获取或创建配额
func (s *SQLStore) GetOrCreateQuota(ctx context.Context, userID, organizationID string) (*Quota, error) {
	scope, err := s.quotaBalanceScope(ctx, userID, organizationID)
	if err != nil {
		return nil, err
	}
	// 尝试获取
	var quota Quota
	err = s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, balance, used, created_at, updated_at
		FROM quotas
		WHERE organization_id = $1
		  AND scope = $2
		  AND ($2 = 'organization' OR user_id = $3)
	`, organizationID, scope, userID).Scan(&quota.ID, &quota.OrganizationID, &quota.UserID, &quota.Balance, &quota.Used, &quota.CreatedAt, &quota.UpdatedAt)

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
		INSERT INTO quotas (id, organization_id, user_id, scope, balance, used, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, 0, $5, $5)
		ON CONFLICT DO NOTHING
	`, id, organizationID, userID, scope, now)
	if err != nil {
		return nil, fmt.Errorf("create quota: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, balance, used, created_at, updated_at
		FROM quotas
		WHERE organization_id = $1
		  AND scope = $2
		  AND ($2 = 'organization' OR user_id = $3)
	`, organizationID, scope, userID).Scan(&quota.ID, &quota.OrganizationID, &quota.UserID, &quota.Balance, &quota.Used, &quota.CreatedAt, &quota.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get created quota: %w", err)
	}

	return &quota, nil
}

// UpdateQuotaBalance 更新配额余额
func (s *SQLStore) UpdateQuotaBalance(ctx context.Context, userID, organizationID string, delta float64) error {
	now := time.Now().UTC()
	scope, err := s.quotaBalanceScope(ctx, userID, organizationID)
	if err != nil {
		return err
	}

	if delta > 0 {
		// 充值
		_, err := s.db.ExecContext(ctx, `
			UPDATE quotas SET balance = balance + $4, updated_at = $5
			WHERE organization_id = $1
			  AND scope = $2
			  AND ($2 = 'organization' OR user_id = $3)
		`, organizationID, scope, userID, delta, now)
		return err
	}

	// 消费
	_, err = s.db.ExecContext(ctx, `
		UPDATE quotas SET balance = balance + $4, used = used - $4, updated_at = $5
		WHERE organization_id = $1
		  AND scope = $2
		  AND ($2 = 'organization' OR user_id = $3)
	`, organizationID, scope, userID, delta, now)
	return err
}

// CreateBillingSession 创建计费会话
func (s *SQLStore) CreateBillingSession(ctx context.Context, session *BillingSession) (*BillingSession, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO billing_sessions (id, organization_id, user_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, session.ID, session.OrganizationID, session.UserID, session.ChannelID, session.Model, session.APIType, session.IdempotencyKey, session.PreAuthorizedAmt, session.Status, session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert billing session: %w", err)
	}
	return session, nil
}

// GetBillingSessionByIdempotencyKey 通过幂等键获取会话
func (s *SQLStore) GetBillingSessionByIdempotencyKey(ctx context.Context, key, organizationID string) (*BillingSession, error) {
	var session BillingSession
	var settledAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt, status, created_at, settled_at
		FROM billing_sessions WHERE idempotency_key = $1 AND organization_id = $2
	`, key, organizationID).Scan(&session.ID, &session.OrganizationID, &session.UserID, &session.ChannelID, &session.Model, &session.APIType,
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
func (s *SQLStore) SettleBillingSession(ctx context.Context, id, organizationID string, settledAmt float64) error {
	now := time.Now().UTC()

	// 获取会话信息
	var session BillingSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, organization_id, pre_authorized_amt, status FROM billing_sessions WHERE id = $1 AND organization_id = $2
	`, id, organizationID).Scan(&session.UserID, &session.OrganizationID, &session.PreAuthorizedAmt, &session.Status)

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
		UPDATE billing_sessions SET status = 'settled', settled_amt = $3, settled_at = $4 WHERE id = $1 AND organization_id = $2
	`, id, organizationID, settledAmt, now)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// 如果有差额，退还
	if refundAmt > 0 {
		if err := s.UpdateQuotaBalance(ctx, session.UserID, session.OrganizationID, refundAmt); err != nil {
			return fmt.Errorf("refund difference: %w", err)
		}
	}

	return nil
}

// RefundBillingSession 退款会话
func (s *SQLStore) RefundBillingSession(ctx context.Context, id, organizationID string) error {
	now := time.Now().UTC()

	// 获取会话信息
	var session BillingSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, organization_id, pre_authorized_amt, status FROM billing_sessions WHERE id = $1 AND organization_id = $2
	`, id, organizationID).Scan(&session.UserID, &session.OrganizationID, &session.PreAuthorizedAmt, &session.Status)

	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if session.Status != "preauthorized" {
		return fmt.Errorf("session already settled or refunded")
	}

	// 更新会话状态
	_, err = s.db.ExecContext(ctx, `
		UPDATE billing_sessions SET status = 'refunded', settled_at = $3 WHERE id = $1 AND organization_id = $2
	`, id, organizationID, now)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// 退还全额
	if err := s.UpdateQuotaBalance(ctx, session.UserID, session.OrganizationID, session.PreAuthorizedAmt); err != nil {
		return fmt.Errorf("refund: %w", err)
	}

	return nil
}

// ListPackages 列出套餐
func (s *SQLStore) ListPackages(ctx context.Context, activeOnly bool) ([]*Package, error) {
	query := `SELECT id, name, description, quota_amount, token_quota, price, model_access, agent_limit, max_tokens_per_request, duration_days, is_active, is_public, sort_order, created_at, updated_at FROM packages`
	if activeOnly {
		query += ` WHERE is_active = true AND is_public = true`
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
		var modelAccess []string

		if err := rows.Scan(&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price, pq.Array(&modelAccess), &p.AgentLimit, &p.MaxTokensPerRequest, &durationDays, &p.IsActive, &p.IsPublic, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan package: %w", err)
		}

		p.Description = description.String
		p.ModelAccess = modelAccess
		if p.ModelAccess == nil {
			p.ModelAccess = []string{}
		}
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
	var modelAccess []string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, quota_amount, token_quota, price, model_access, agent_limit, max_tokens_per_request, duration_days, is_active, is_public, sort_order, created_at, updated_at
		FROM packages WHERE id = $1
	`, id).Scan(&p.ID, &p.Name, &description, &p.QuotaAmount, &p.TokenQuota, &p.Price, pq.Array(&modelAccess), &p.AgentLimit, &p.MaxTokensPerRequest, &durationDays, &p.IsActive, &p.IsPublic, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get package: %w", err)
	}

	p.Description = description.String
	p.ModelAccess = modelAccess
	if p.ModelAccess == nil {
		p.ModelAccess = []string{}
	}
	if durationDays.Valid {
		days := int(durationDays.Int64)
		p.DurationDays = &days
	}

	return &p, nil
}

func (s *SQLStore) SaveUsageLimitSettings(ctx context.Context, settings UsageLimitSettings) (*UsageLimitSettings, error) {
	normalized, err := normalizeUsageLimitSettings(settings)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	normalized.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin usage limit settings transaction: %w", err)
	}
	defer tx.Rollback()

	if err := upsertConcurrencyLimit(ctx, tx, normalized, now); err != nil {
		return nil, err
	}
	if err := upsertTokenRateLimit(ctx, tx, normalized, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit usage limit settings: %w", err)
	}

	return &normalized, nil
}

func upsertConcurrencyLimit(ctx context.Context, tx *sql.Tx, settings UsageLimitSettings, now time.Time) error {
	id, err := auth.NewID("clim")
	if err != nil {
		return err
	}
	if settings.UserID == "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO concurrency_limits (id, organization_id, user_id, max_concurrent_requests, current_concurrent, updated_at)
			VALUES ($1, $2, NULL, $3, 0, $4)
			ON CONFLICT (organization_id) WHERE user_id IS NULL
			DO UPDATE SET max_concurrent_requests = EXCLUDED.max_concurrent_requests, updated_at = EXCLUDED.updated_at
		`, id, settings.OrganizationID, settings.MaxConcurrentRequests, now)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO concurrency_limits (id, organization_id, user_id, max_concurrent_requests, current_concurrent, updated_at)
			VALUES ($1, $2, $3, $4, 0, $5)
			ON CONFLICT (organization_id, user_id) WHERE user_id IS NOT NULL
			DO UPDATE SET max_concurrent_requests = EXCLUDED.max_concurrent_requests, updated_at = EXCLUDED.updated_at
		`, id, settings.OrganizationID, settings.UserID, settings.MaxConcurrentRequests, now)
	}
	if err != nil {
		return fmt.Errorf("upsert concurrency limit: %w", err)
	}
	return nil
}

func upsertTokenRateLimit(ctx context.Context, tx *sql.Tx, settings UsageLimitSettings, now time.Time) error {
	id, err := auth.NewID("tlim")
	if err != nil {
		return err
	}
	windowSeconds := settings.WindowSeconds
	if (settings.MaxTokensPerWindow > 0 || settings.MaxTokensPerRequest > 0) && windowSeconds <= 0 {
		windowSeconds = defaultUsageLimitWindowSeconds
	}
	if settings.UserID == "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO token_rate_limits (id, organization_id, user_id, window_seconds, max_tokens_per_window, max_tokens_per_request, current_window_tokens, updated_at)
			VALUES ($1, $2, NULL, $3, $4, $5, 0, $6)
			ON CONFLICT (organization_id) WHERE user_id IS NULL
			DO UPDATE SET window_seconds = EXCLUDED.window_seconds,
				max_tokens_per_window = EXCLUDED.max_tokens_per_window,
				max_tokens_per_request = EXCLUDED.max_tokens_per_request,
				updated_at = EXCLUDED.updated_at
		`, id, settings.OrganizationID, windowSeconds, settings.MaxTokensPerWindow, settings.MaxTokensPerRequest, now)
	} else {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO token_rate_limits (id, organization_id, user_id, window_seconds, max_tokens_per_window, max_tokens_per_request, current_window_tokens, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 0, $7)
			ON CONFLICT (organization_id, user_id) WHERE user_id IS NOT NULL
			DO UPDATE SET window_seconds = EXCLUDED.window_seconds,
				max_tokens_per_window = EXCLUDED.max_tokens_per_window,
				max_tokens_per_request = EXCLUDED.max_tokens_per_request,
				updated_at = EXCLUDED.updated_at
		`, id, settings.OrganizationID, settings.UserID, windowSeconds, settings.MaxTokensPerWindow, settings.MaxTokensPerRequest, now)
	}
	if err != nil {
		return fmt.Errorf("upsert token rate limit: %w", err)
	}
	return nil
}

func (s *SQLStore) ResolveUsageLimitSettings(ctx context.Context, organizationID, userID string) (UsageLimitSettings, error) {
	settings, err := s.ListUsageLimitSettings(ctx, organizationID)
	if err != nil {
		return UsageLimitSettings{}, err
	}
	var organizationSettings *UsageLimitSettings
	for i := range settings {
		if settings[i].UserID == userID && userID != "" {
			return settings[i], nil
		}
		if settings[i].UserID == "" {
			organizationSettings = &settings[i]
		}
	}
	if organizationSettings != nil {
		return *organizationSettings, nil
	}
	return UsageLimitSettings{OrganizationID: organizationID, UserID: userID, QuotaMode: usageLimitQuotaMode(userID)}, nil
}

func (s *SQLStore) ListUsageLimitSettings(ctx context.Context, organizationID string) ([]UsageLimitSettings, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			COALESCE(c.organization_id, t.organization_id) AS organization_id,
			COALESCE(c.user_id, t.user_id, '') AS user_id,
			COALESCE(c.max_concurrent_requests, 0) AS max_concurrent_requests,
			COALESCE(t.window_seconds, 0) AS window_seconds,
			COALESCE(t.max_tokens_per_window, 0) AS max_tokens_per_window,
			COALESCE(t.max_tokens_per_request, 0) AS max_tokens_per_request,
			GREATEST(
				COALESCE(c.updated_at, '-infinity'::timestamptz),
				COALESCE(t.updated_at, '-infinity'::timestamptz)
			) AS updated_at
		FROM concurrency_limits c
		FULL OUTER JOIN token_rate_limits t
			ON c.organization_id = t.organization_id
			AND COALESCE(c.user_id, '') = COALESCE(t.user_id, '')
		WHERE COALESCE(c.organization_id, t.organization_id) = $1
		ORDER BY
			CASE WHEN COALESCE(c.user_id, t.user_id) IS NULL THEN 0 ELSE 1 END,
			COALESCE(c.user_id, t.user_id, '') ASC
	`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list usage limit settings: %w", err)
	}
	defer rows.Close()

	var settings []UsageLimitSettings
	for rows.Next() {
		var item UsageLimitSettings
		if err := rows.Scan(
			&item.OrganizationID,
			&item.UserID,
			&item.MaxConcurrentRequests,
			&item.WindowSeconds,
			&item.MaxTokensPerWindow,
			&item.MaxTokensPerRequest,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage limit settings: %w", err)
		}
		item.QuotaMode = usageLimitQuotaMode(item.UserID)
		settings = append(settings, item)
	}
	return settings, rows.Err()
}

// CreateSubscription 创建订阅
func (s *SQLStore) CreateSubscription(ctx context.Context, sub *Subscription) (*Subscription, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (id, organization_id, user_id, package_id, status, started_at, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sub.ID, sub.OrganizationID, sub.UserID, sub.PackageID, sub.Status, sub.StartedAt, sub.ExpiresAt, sub.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	return sub, nil
}

// ListActiveSubscriptions 列出活跃订阅
func (s *SQLStore) ListActiveSubscriptions(ctx context.Context, userID, organizationID string) ([]*Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, package_id, status, started_at, expires_at, created_at
		FROM subscriptions WHERE user_id = $1 AND organization_id = $2 AND status = 'active'
		ORDER BY created_at DESC
	`, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*Subscription
	for rows.Next() {
		var sub Subscription
		var expiresAt sql.NullTime

		if err := rows.Scan(&sub.ID, &sub.OrganizationID, &sub.UserID, &sub.PackageID, &sub.Status, &sub.StartedAt, &expiresAt, &sub.CreatedAt); err != nil {
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
		INSERT INTO topup_orders (id, organization_id, user_id, amount, money, status, payment_intent_id, provider_checkout_session_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9)
	`, order.ID, order.OrganizationID, order.UserID, order.Amount, order.Money, order.Status, order.PaymentIntentID, order.ProviderCheckoutSessionID, order.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert topup order: %w", err)
	}
	return order, nil
}

func (s *SQLStore) UpdateTopupOrderCheckoutSession(ctx context.Context, organizationID, paymentIntentID string, providerCheckoutSessionID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE topup_orders
		SET provider_checkout_session_id = $3
		WHERE organization_id = $1 AND payment_intent_id = $2
	`, organizationID, paymentIntentID, providerCheckoutSessionID)
	if err != nil {
		return fmt.Errorf("update topup checkout session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("topup checkout rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SQLStore) MarkTopupOrderFailedByPaymentIntent(ctx context.Context, organizationID, paymentIntentID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE topup_orders
		SET status = 'failed'
		WHERE organization_id = $1 AND payment_intent_id = $2 AND status = 'pending'
	`, organizationID, paymentIntentID)
	if err != nil {
		return fmt.Errorf("mark topup order failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("topup order failed rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateTopupOrderStatus 更新充值订单状态
func (s *SQLStore) UpdateTopupOrderStatus(ctx context.Context, id, organizationID string, status string, tradeNo string) error {
	now := time.Now().UTC()

	var paidAt interface{}
	if status == "paid" {
		paidAt = now
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE topup_orders SET status = $3, trade_no = $4, paid_at = $5 WHERE id = $1 AND organization_id = $2
	`, id, organizationID, status, tradeNo, paidAt)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("topup order status rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
