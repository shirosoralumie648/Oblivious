package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/admin"
)

func TestAdminReconciliationHandlerGetsRelayUsagePriceReconciliationWithFilters(t *testing.T) {
	store := &reconciliationHTTPStore{}
	handler := newAdminHandler(admin.NewService(store))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/reconciliation/relay-usage-prices?organizationId=org_1&userId=user_1&apiTokenID=tok_1&requestID=req_1&apiType=chat&featureType=workspace_chat&quotaMode=relay_billing&model=gpt-4o&channelID=ch_1&provider=openai&status=success&from=2026-07-01T00:00:00Z&to=2026-07-02T00:00:00Z&limit=250&offset=-1", nil)
	recorder := httptest.NewRecorder()
	handler.getRelayUsagePriceReconciliation(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected relay usage price reconciliation 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.usagePriceReconciliationFilter.OrganizationID != "org_1" ||
		store.usagePriceReconciliationFilter.UserID != "user_1" ||
		store.usagePriceReconciliationFilter.APITokenID != "tok_1" ||
		store.usagePriceReconciliationFilter.RequestID != "req_1" ||
		store.usagePriceReconciliationFilter.APIType != "chat" ||
		store.usagePriceReconciliationFilter.FeatureType != "workspace_chat" ||
		store.usagePriceReconciliationFilter.QuotaMode != "relay_billing" ||
		store.usagePriceReconciliationFilter.Model != "gpt-4o" ||
		store.usagePriceReconciliationFilter.ChannelID != "ch_1" ||
		store.usagePriceReconciliationFilter.Provider != "openai" ||
		store.usagePriceReconciliationFilter.Status != "success" {
		t.Fatalf("expected reconciliation filters to be passed, got %#v", store.usagePriceReconciliationFilter)
	}
	if store.usagePriceReconciliationFilter.Limit != 100 || store.usagePriceReconciliationFilter.Offset != 0 {
		t.Fatalf("expected clamped reconciliation pagination, got limit=%d offset=%d", store.usagePriceReconciliationFilter.Limit, store.usagePriceReconciliationFilter.Offset)
	}
	if !store.usagePriceReconciliationFilter.From.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) ||
		!store.usagePriceReconciliationFilter.To.Equal(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected parsed reconciliation time filters, got from=%s to=%s", store.usagePriceReconciliationFilter.From, store.usagePriceReconciliationFilter.To)
	}
	if !strings.Contains(recorder.Body.String(), `"missingSnapshotRecords":1`) ||
		!strings.Contains(recorder.Body.String(), `"issue":"missing_snapshot"`) {
		t.Fatalf("expected reconciliation response payload, got %s", recorder.Body.String())
	}
}

func TestAdminReconciliationHandlerGetsUsageRequestLogCoverageWithFilters(t *testing.T) {
	store := &reconciliationHTTPStore{}
	handler := newAdminHandler(admin.NewService(store, admin.WithRequestLogEvidenceStore(store)))

	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/billing/reconciliation/usage-request-logs?organizationId=org_1&userId=user_1&apiTokenID=tok_1&requestID=req_1&apiType=chat&featureType=workspace_chat&quotaMode=relay_billing&model=gpt-4o&channelID=ch_1&provider=openai&status=success&limit=250&offset=-1", nil)
	recorder := httptest.NewRecorder()
	handler.getUsageRequestLogCoverage(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage request-log coverage 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if store.usageLogFilter.OrganizationID != "org_1" ||
		store.usageLogFilter.UserID != "user_1" ||
		store.usageLogFilter.APITokenID != "tok_1" ||
		store.usageLogFilter.RequestID != "req_1" ||
		store.usageLogFilter.APIType != "chat" ||
		store.usageLogFilter.FeatureType != "workspace_chat" ||
		store.usageLogFilter.QuotaMode != "relay_billing" ||
		store.usageLogFilter.Model != "gpt-4o" ||
		store.usageLogFilter.ChannelID != "ch_1" ||
		store.usageLogFilter.Provider != "openai" ||
		store.usageLogFilter.Status != "success" {
		t.Fatalf("expected usage coverage filters to be passed, got %#v", store.usageLogFilter)
	}
	if store.usageLogFilter.Limit != 100 || store.usageLogFilter.Offset != 0 {
		t.Fatalf("expected clamped coverage pagination, got limit=%d offset=%d", store.usageLogFilter.Limit, store.usageLogFilter.Offset)
	}
	if !strings.Contains(recorder.Body.String(), `"checkedRecords":1`) ||
		!strings.Contains(recorder.Body.String(), `"matchedRequestLogRecords":1`) {
		t.Fatalf("expected request-log coverage response payload, got %s", recorder.Body.String())
	}
}

type reconciliationHTTPStore struct {
	admin.Store
	usageLogFilter                 admin.UsageLogFilter
	usagePriceReconciliationFilter admin.RelayUsagePriceReconciliationFilter
}

func (s *reconciliationHTTPStore) ListUsageLogs(ctx context.Context, filter admin.UsageLogFilter) ([]*admin.UsageLogEntry, int, error) {
	s.usageLogFilter = filter
	return []*admin.UsageLogEntry{{
		ID:        "usage_1",
		RequestID: "req_1",
		Model:     "gpt-4o",
		CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	}}, 1, nil
}

func (s *reconciliationHTTPStore) GetUsageAnalytics(ctx context.Context, filter admin.UsageAnalyticsFilter) (admin.UsageAnalytics, error) {
	return admin.UsageAnalytics{}, nil
}

func (s *reconciliationHTTPStore) GetRelayUsagePriceReconciliation(ctx context.Context, filter admin.RelayUsagePriceReconciliationFilter) (*admin.RelayUsagePriceReconciliationSummary, error) {
	s.usagePriceReconciliationFilter = filter
	return &admin.RelayUsagePriceReconciliationSummary{
		CheckedRecords:         2,
		MatchedRecords:         1,
		MissingSnapshotRecords: 1,
		Issues: []admin.RelayUsagePriceReconciliationIssue{{
			ID:        "usage_missing_snapshot",
			UserID:    "user_1",
			Model:     "gpt-4o",
			Cost:      0.42,
			DeltaCost: 0.42,
			Issue:     "missing_snapshot",
			CreatedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		}},
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}, nil
}

func (s *reconciliationHTTPStore) ListRequestLogEvidence(ctx context.Context, requestIDs []string) (map[string]admin.RequestLogEvidence, error) {
	return map[string]admin.RequestLogEvidence{
		"req_1": {
			RequestID:    "req_1",
			RequestLogID: "550e8400-e29b-41d4-a716-446655440000",
			Service:      "relay",
			Endpoint:     "/v1/chat/completions",
			Method:       "POST",
			StatusCode:   200,
			DurationMS:   42,
			Model:        "gpt-4o",
			CostUSD:      0.42,
			Timestamp:    time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		},
	}, nil
}
