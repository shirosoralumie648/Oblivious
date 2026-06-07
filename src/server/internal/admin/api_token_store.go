package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"oblivious/server/internal/relay"
)

// APITokenStore defines admin-wide Relay API token inspection operations.
type APITokenStore interface {
	ListAPITokens(ctx context.Context, filter APITokenFilter) ([]*APITokenEntry, int, error)
	RevokeAPIToken(ctx context.Context, tokenID string) error
}

func apiTokenWhere(filter APITokenFilter) (string, []any) {
	var conditions []string
	var args []any
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	if filter.OrganizationID != "" {
		add("tok.organization_id = $%d", filter.OrganizationID)
	}
	if filter.UserID != "" {
		add("tok.user_id = $%d", filter.UserID)
	}
	if filter.Status != "" {
		add("tok.status = $%d", filter.Status)
	}
	if filter.UserGroup != "" {
		add("tok.user_group = $%d", filter.UserGroup)
	}
	if filter.Search != "" {
		args = append(args, filter.Search)
		searchArg := len(args)
		conditions = append(conditions, fmt.Sprintf(
			"(tok.name ILIKE '%%' || $%d || '%%' OR tok.token_prefix ILIKE '%%' || $%d || '%%' OR u.email ILIKE '%%' || $%d || '%%')",
			searchArg,
			searchArg,
			searchArg,
		))
	}
	if filter.Model != "" {
		add("(tok.model_limits_enabled = false OR $%d = ANY(tok.model_limits))", filter.Model)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func apiTokenUsageStatsCTE() string {
	return `
		SELECT
			api_token_id,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(cost), 0) AS total_cost
		FROM usage_records
		WHERE api_token_id IS NOT NULL AND api_token_id <> ''
		GROUP BY api_token_id
	`
}

func (s *SQLStore) ListAPITokens(ctx context.Context, filter APITokenFilter) ([]*APITokenEntry, int, error) {
	filter = normalizeAPITokenFilter(filter)
	where, args := apiTokenWhere(filter)

	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM relay_api_tokens tok
		INNER JOIN users u ON u.id = tok.user_id
		`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count api tokens: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			tok.id,
			tok.organization_id,
			tok.user_id,
			u.email,
				tok.name,
				tok.token_prefix,
				tok.status,
				COALESCE(tok.user_group, ''),
				tok.model_limits_enabled,
				tok.model_limits,
			tok.quota_limit,
			tok.used_quota,
			COALESCE(usage_stats.request_count, 0),
			COALESCE(usage_stats.total_cost, 0),
			tok.expires_at,
			tok.last_used_at,
			tok.created_at,
			tok.revoked_at
		FROM relay_api_tokens tok
		INNER JOIN users u ON u.id = tok.user_id
		LEFT JOIN (`+apiTokenUsageStatsCTE()+`) usage_stats ON usage_stats.api_token_id = tok.id
		`+where+`
		ORDER BY tok.created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*APITokenEntry
	for rows.Next() {
		var token APITokenEntry
		var quotaLimit sql.NullFloat64
		var expiresAt, lastUsedAt, revokedAt sql.NullTime
		if err := rows.Scan(
			&token.ID,
			&token.OrganizationID,
			&token.UserID,
			&token.UserEmail,
			&token.Name,
			&token.TokenPrefix,
			&token.Status,
			&token.UserGroup,
			&token.ModelLimitsEnabled,
			pq.Array(&token.ModelLimits),
			&quotaLimit,
			&token.UsedQuota,
			&token.RequestCount,
			&token.TotalCost,
			&expiresAt,
			&lastUsedAt,
			&token.CreatedAt,
			&revokedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan api token: %w", err)
		}
		if quotaLimit.Valid {
			token.QuotaLimit = &quotaLimit.Float64
		}
		if expiresAt.Valid {
			token.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}
		if revokedAt.Valid {
			token.RevokedAt = &revokedAt.Time
		}
		tokens = append(tokens, &token)
	}

	return tokens, total, rows.Err()
}

func (s *SQLStore) RevokeAPIToken(ctx context.Context, tokenID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE relay_api_tokens
		SET status = $2, revoked_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = $3
	`, tokenID, relay.RelayAPITokenStatusRevoked, relay.RelayAPITokenStatusActive)
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api token rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
