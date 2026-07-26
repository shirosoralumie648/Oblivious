package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"oblivious/server/internal/relay/types"
)

const (
	RelayAPITokenStatusActive  = "active"
	RelayAPITokenStatusRevoked = "revoked"
)

type RelayAPITokenRecord struct {
	ID                 string
	UserID             string
	OrganizationID     string
	UserGroup          string
	Status             string
	ModelLimitsEnabled bool
	ModelLimits        []string
	QuotaLimit         *float64
	UsedQuota          float64
	ExpiresAt          *time.Time
}

type CreateRelayAPITokenInput struct {
	UserID             string
	OrganizationID     string
	UserGroup          string
	Name               string
	ModelLimitsEnabled bool
	ModelLimits        []string
	QuotaLimit         *float64
	ExpiresAt          *time.Time
}

type CreatedRelayAPIToken struct {
	RawToken string                `json:"rawToken"`
	Token    RelayAPITokenListItem `json:"token"`
}

type RelayAPITokenListItem struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	TokenPrefix        string     `json:"tokenPrefix"`
	Status             string     `json:"status"`
	UserGroup          string     `json:"userGroup,omitempty"`
	ModelLimitsEnabled bool       `json:"modelLimitsEnabled"`
	ModelLimits        []string   `json:"modelLimits"`
	QuotaLimit         *float64   `json:"quotaLimit,omitempty"`
	UsedQuota          float64    `json:"usedQuota"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt         *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
}

type RelayAPITokenUsageItem struct {
	ID               string    `json:"id"`
	APITokenID       string    `json:"apiTokenId"`
	RequestID        string    `json:"requestId"`
	APIType          string    `json:"apiType"`
	Model            string    `json:"model"`
	ChannelID        string    `json:"channelId"`
	Provider         string    `json:"provider"`
	Status           string    `json:"status"`
	StatusCode       int       `json:"statusCode"`
	ErrorCode        string    `json:"errorCode,omitempty"`
	LatencyMS        int64     `json:"latencyMs"`
	Cost             float64   `json:"cost"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	CreatedAt        time.Time `json:"createdAt"`
}

type RelayAPITokenStore interface {
	GetRelayAPITokenByHash(ctx context.Context, tokenHash string) (RelayAPITokenRecord, error)
	TouchRelayAPIToken(ctx context.Context, tokenID string, usedAt time.Time) error
}

type APITokenAuthenticator struct {
	store RelayAPITokenStore
	now   func() time.Time
}

func NewAPITokenAuthenticator(store RelayAPITokenStore) *APITokenAuthenticator {
	return &APITokenAuthenticator{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (a *APITokenAuthenticator) AuthenticateRelayAPIToken(ctx context.Context, rawToken, model string, apiType types.APIType) (types.RelayAPITokenIdentity, error) {
	if a == nil || a.store == nil || rawToken == "" {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenInvalid
	}
	record, err := a.store.GetRelayAPITokenByHash(ctx, HashRelayAPIToken(rawToken))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenInvalid
		}
		return types.RelayAPITokenIdentity{}, err
	}
	if record.ID == "" || record.UserID == "" || record.OrganizationID == "" {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenInvalid
	}
	if record.Status != RelayAPITokenStatusActive {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenRevoked
	}
	if record.ExpiresAt != nil && !record.ExpiresAt.After(a.now()) {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenExpired
	}
	if record.ModelLimitsEnabled && !relayAPITokenAllowsModel(record.ModelLimits, model) {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenModelDenied
	}
	if record.QuotaLimit != nil && record.UsedQuota >= *record.QuotaLimit {
		return types.RelayAPITokenIdentity{}, types.ErrRelayAPITokenQuotaExceeded
	}
	_ = a.store.TouchRelayAPIToken(ctx, record.ID, a.now())
	return types.RelayAPITokenIdentity{
		TokenID:        record.ID,
		UserID:         record.UserID,
		OrganizationID: record.OrganizationID,
		UserGroup:      record.UserGroup,
	}, nil
}

func HashRelayAPIToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func relayAPITokenAllowsModel(modelLimits []string, model string) bool {
	for _, allowed := range modelLimits {
		if allowed == "*" || allowed == model || relayAPITokenWildcardMatch(allowed, model) {
			return true
		}
	}
	return false
}

func relayAPITokenWildcardMatch(pattern, model string) bool {
	if pattern == "" || model == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !containsWildcard(pattern) {
		return pattern == model
	}
	matcher := "^" + regexp.QuoteMeta(pattern) + "$"
	matcher = regexp.MustCompile(`\\\*`).ReplaceAllString(matcher, ".*")
	return regexp.MustCompile(matcher).MatchString(model)
}

func containsWildcard(pattern string) bool {
	for _, ch := range pattern {
		if ch == '*' {
			return true
		}
	}
	return false
}

type RelayAPITokenSQLStore struct {
	db *sql.DB
}

func NewRelayAPITokenSQLStore(db *sql.DB) *RelayAPITokenSQLStore {
	return &RelayAPITokenSQLStore{db: db}
}

func (s *RelayAPITokenSQLStore) CreateRelayAPIToken(ctx context.Context, input CreateRelayAPITokenInput) (CreatedRelayAPIToken, error) {
	rawToken, err := generateRelayAPIToken()
	if err != nil {
		return CreatedRelayAPIToken{}, err
	}
	prefix := rawToken
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	item := RelayAPITokenListItem{
		ID:                 uuid.NewString(),
		Name:               input.Name,
		TokenPrefix:        prefix,
		Status:             RelayAPITokenStatusActive,
		UserGroup:          strings.TrimSpace(input.UserGroup),
		ModelLimitsEnabled: input.ModelLimitsEnabled,
		ModelLimits:        input.ModelLimits,
		QuotaLimit:         input.QuotaLimit,
		CreatedAt:          time.Now().UTC(),
	}
	if input.ExpiresAt != nil {
		expiresAt := input.ExpiresAt.UTC()
		item.ExpiresAt = &expiresAt
	}
	if _, err := s.db.ExecContext(ctx, `
			INSERT INTO relay_api_tokens (
				id, user_id, organization_id, user_group, name, token_hash, token_prefix,
				status, model_limits_enabled, model_limits, quota_limit,
				expires_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
		`, item.ID, input.UserID, input.OrganizationID, item.UserGroup, input.Name, HashRelayAPIToken(rawToken), prefix,
		item.Status, item.ModelLimitsEnabled, pq.Array(item.ModelLimits), nullableFloat64(item.QuotaLimit), nullableTime(item.ExpiresAt), item.CreatedAt); err != nil {
		return CreatedRelayAPIToken{}, err
	}
	return CreatedRelayAPIToken{RawToken: rawToken, Token: item}, nil
}

func (s *RelayAPITokenSQLStore) ListRelayAPITokens(ctx context.Context, organizationID, userID string) ([]RelayAPITokenListItem, error) {
	rows, err := s.db.QueryContext(ctx, `
			SELECT id, name, token_prefix, status, COALESCE(user_group, ''), model_limits_enabled, model_limits,
			       quota_limit, used_quota, expires_at, last_used_at, created_at, revoked_at
		FROM relay_api_tokens
		WHERE organization_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`, organizationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []RelayAPITokenListItem{}
	for rows.Next() {
		item, err := scanRelayAPITokenListItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *RelayAPITokenSQLStore) RevokeRelayAPIToken(ctx context.Context, organizationID, userID, tokenID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET status = $4, revoked_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND organization_id = $2 AND user_id = $3 AND status = $5
	`, tokenID, organizationID, userID, RelayAPITokenStatusRevoked, RelayAPITokenStatusActive)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *RelayAPITokenSQLStore) GetRelayAPITokenByHash(ctx context.Context, tokenHash string) (RelayAPITokenRecord, error) {
	var record RelayAPITokenRecord
	var quotaLimit sql.NullFloat64
	var expiresAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
			SELECT id, user_id, organization_id, COALESCE(user_group, ''), status, model_limits_enabled, model_limits,
			       quota_limit, used_quota, expires_at
		FROM relay_api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&record.ID,
		&record.UserID,
		&record.OrganizationID,
		&record.UserGroup,
		&record.Status,
		&record.ModelLimitsEnabled,
		pq.Array(&record.ModelLimits),
		&quotaLimit,
		&record.UsedQuota,
		&expiresAt,
	); err != nil {
		return RelayAPITokenRecord{}, err
	}
	if quotaLimit.Valid {
		record.QuotaLimit = &quotaLimit.Float64
	}
	if expiresAt.Valid {
		record.ExpiresAt = &expiresAt.Time
	}
	return record, nil
}

func (s *RelayAPITokenSQLStore) TouchRelayAPIToken(ctx context.Context, tokenID string, usedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET last_used_at = $2, updated_at = $2
		WHERE id = $1
	`, tokenID, usedAt.UTC())
	return err
}

func (s *RelayAPITokenSQLStore) PreAuthorizeRelayAPITokenQuota(ctx context.Context, tokenID string, amount float64) error {
	if amount <= 0 {
		return nil
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET used_quota = used_quota + $2,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = $3
		  AND (quota_limit IS NULL OR used_quota + $2 <= quota_limit)
	`, tokenID, amount, RelayAPITokenStatusActive)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return types.ErrRelayAPITokenQuotaExceeded
	}
	return nil
}

func (s *RelayAPITokenSQLStore) SettleRelayAPITokenQuota(ctx context.Context, tokenID string, preauthorizedAmount, actualAmount float64) error {
	delta := actualAmount - preauthorizedAmount
	if delta == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET used_quota = GREATEST(used_quota + $2, 0),
		    updated_at = NOW()
		WHERE id = $1
	`, tokenID, delta)
	return err
}

func (s *RelayAPITokenSQLStore) RefundRelayAPITokenQuota(ctx context.Context, tokenID string, amount float64) error {
	if amount <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET used_quota = GREATEST(used_quota - $2, 0),
		    updated_at = NOW()
		WHERE id = $1
	`, tokenID, amount)
	return err
}

// RefundRelayAPITokenQuotaOnce decrements used_quota and records a
// compensation receipt in one SQL transaction. Repeating the call with the
// same scopeKey is idempotent: if the receipt already exists and matches
// tokenID/amount the function returns nil. A mismatch returns
// ErrQuotaCompensationReceiptMismatch.
func (s *RelayAPITokenSQLStore) RefundRelayAPITokenQuotaOnce(ctx context.Context, tokenID string, amount float64, scopeKey string) error {
	if amount <= 0 || scopeKey == "" || tokenID == "" {
		return ErrQuotaCompensationInvalidRequest
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var storedTokenID string
	var storedAmount float64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO relay_api_token_quota_refund_receipts (scope_key_digest, api_token_id, amount, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (scope_key_digest) DO NOTHING
		RETURNING api_token_id, amount
	`, scopeKey, tokenID, amount).Scan(&storedTokenID, &storedAmount)
	inserted := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT api_token_id, amount
			FROM relay_api_token_quota_refund_receipts
			WHERE scope_key_digest = $1
		`, scopeKey).Scan(&storedTokenID, &storedAmount)
	}
	if err != nil {
		return err
	}
	if storedTokenID != tokenID || storedAmount != amount {
		return ErrQuotaCompensationReceiptMismatch
	}
	if inserted {
		// Only decrement quota when we are the first to record the receipt.
		if _, err := tx.ExecContext(ctx, `
			UPDATE relay_api_tokens
			SET used_quota = GREATEST(used_quota - $2, 0),
			    updated_at = NOW()
			WHERE id = $1
		`, tokenID, amount); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *RelayAPITokenSQLStore) ListRelayAPITokenUsage(ctx context.Context, organizationID, userID, tokenID string) ([]RelayAPITokenUsageItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ur.id,
			ur.api_token_id,
			COALESCE(ur.request_id, ''),
			COALESCE(ur.api_type, ''),
			ur.model_id,
			COALESCE(ur.channel_id, ''),
			COALESCE(ur.provider, ''),
			COALESCE(ur.status, ''),
			COALESCE(ur.status_code, 0),
			COALESCE(ur.error_code, ''),
			COALESCE(ur.latency_ms, 0),
			COALESCE(ur.cost, 0),
			ur.input_tokens,
			ur.output_tokens,
			COALESCE(ur.total_tokens, ur.input_tokens + ur.output_tokens),
			ur.created_at
		FROM usage_records ur
		INNER JOIN relay_api_tokens tok
			ON tok.id = ur.api_token_id
			AND tok.organization_id = $1
			AND tok.user_id = $2
		WHERE ur.organization_id = $1
		  AND ur.api_token_id = $3
		ORDER BY ur.created_at DESC
		LIMIT 100
	`, organizationID, userID, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []RelayAPITokenUsageItem{}
	for rows.Next() {
		var item RelayAPITokenUsageItem
		if err := rows.Scan(
			&item.ID,
			&item.APITokenID,
			&item.RequestID,
			&item.APIType,
			&item.Model,
			&item.ChannelID,
			&item.Provider,
			&item.Status,
			&item.StatusCode,
			&item.ErrorCode,
			&item.LatencyMS,
			&item.Cost,
			&item.PromptTokens,
			&item.CompletionTokens,
			&item.TotalTokens,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type relayAPITokenRows interface {
	Scan(dest ...any) error
}

func scanRelayAPITokenListItem(row relayAPITokenRows) (RelayAPITokenListItem, error) {
	var item RelayAPITokenListItem
	var quotaLimit sql.NullFloat64
	var expiresAt, lastUsedAt, revokedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.TokenPrefix,
		&item.Status,
		&item.UserGroup,
		&item.ModelLimitsEnabled,
		pq.Array(&item.ModelLimits),
		&quotaLimit,
		&item.UsedQuota,
		&expiresAt,
		&lastUsedAt,
		&item.CreatedAt,
		&revokedAt,
	); err != nil {
		return RelayAPITokenListItem{}, err
	}
	if quotaLimit.Valid {
		item.QuotaLimit = &quotaLimit.Float64
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		item.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		item.RevokedAt = &revokedAt.Time
	}
	return item, nil
}

func generateRelayAPIToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "obv_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func nullableFloat64(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
