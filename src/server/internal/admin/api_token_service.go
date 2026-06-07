package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"oblivious/server/internal/auth"
)

func normalizeAPITokenFilter(filter APITokenFilter) APITokenFilter {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.OrganizationID = strings.TrimSpace(filter.OrganizationID)
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.UserGroup = strings.TrimSpace(filter.UserGroup)
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Model = strings.TrimSpace(filter.Model)
	return filter
}

func (s *Service) ListAPITokens(ctx context.Context, filter APITokenFilter) ([]*APITokenEntry, int, error) {
	return s.store.ListAPITokens(ctx, normalizeAPITokenFilter(filter))
}

func (s *Service) RevokeAPIToken(ctx context.Context, actor auth.Session, tokenID, ipAddress string) error {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return sql.ErrNoRows
	}
	if err := s.store.RevokeAPIToken(ctx, tokenID); err != nil {
		return err
	}
	changes, _ := json.Marshal(map[string]string{"status": "revoked"})
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "api_token.revoke", "api_token", tokenID, string(changes), ipAddress)
	return nil
}
