package admin

import (
	"context"
	"testing"
	"time"
)

func TestServiceListAPITokensNormalizesFiltersAndReturnsOperationalFields(t *testing.T) {
	store := &apiTokenStoreSpy{
		tokens: []*APITokenEntry{{
			ID:                 "tok_1",
			OrganizationID:     "org_1",
			UserID:             "user_1",
			UserEmail:          "user@example.com",
			Name:               "Production key",
			TokenPrefix:        "oblv_abc",
			Status:             "active",
			UserGroup:          "vip",
			ModelLimitsEnabled: true,
			ModelLimits:        []string{"gpt-4o"},
			QuotaLimit:         float64Ptr(50),
			UsedQuota:          12.5,
			RequestCount:       7,
			TotalCost:          1.25,
			CreatedAt:          time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}
	service := NewService(store)

	tokens, total, err := service.ListAPITokens(context.Background(), APITokenFilter{
		OrganizationID: " org_1 ",
		UserID:         " user_1 ",
		Status:         " active ",
		UserGroup:      " vip ",
		Search:         " Production ",
		Model:          " gpt-4o ",
		Limit:          250,
		Offset:         -5,
	})
	if err != nil {
		t.Fatalf("list api tokens: %v", err)
	}
	if total != 1 || len(tokens) != 1 {
		t.Fatalf("expected one token and total=1, got len=%d total=%d", len(tokens), total)
	}
	if store.filter.Limit != 100 || store.filter.Offset != 0 {
		t.Fatalf("expected normalized limit=100 offset=0, got limit=%d offset=%d", store.filter.Limit, store.filter.Offset)
	}
	if store.filter.OrganizationID != "org_1" || store.filter.UserID != "user_1" || store.filter.Status != "active" || store.filter.UserGroup != "vip" || store.filter.Model != "gpt-4o" {
		t.Fatalf("expected trimmed filters, got %#v", store.filter)
	}
	if tokens[0].UserEmail != "user@example.com" || tokens[0].UserGroup != "vip" || tokens[0].RequestCount != 7 || tokens[0].TotalCost != 1.25 {
		t.Fatalf("expected operational token fields, got %#v", tokens[0])
	}
}

type apiTokenStoreSpy struct {
	Store
	filter APITokenFilter
	tokens []*APITokenEntry
	total  int
}

func (s *apiTokenStoreSpy) ListAPITokens(_ context.Context, filter APITokenFilter) ([]*APITokenEntry, int, error) {
	s.filter = filter
	return s.tokens, s.total, nil
}

func float64Ptr(value float64) *float64 {
	return &value
}
