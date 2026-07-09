package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestServiceListUsageLogsNormalizesFiltersAndReturnsGatewayFields(t *testing.T) {
	store := &usageLogStoreSpy{
		entries: []*UsageLogEntry{{
			ID:               "usage_1",
			OrganizationID:   "org_1",
			UserID:           "user_1",
			APITokenID:       "tok_1",
			RequestID:        "req_1",
			APIType:          "chat",
			FeatureType:      "workspace_chat",
			QuotaMode:        "relay_billing",
			Model:            "gpt-4o",
			ChannelID:        "ch_1",
			Provider:         "openai",
			Status:           "success",
			StatusCode:       200,
			LatencyMS:        41,
			Cost:             0.42,
			ChannelCost:      0.21,
			PriceSnapshot:    json.RawMessage(`{"currency":"quota","dimensions":[{"pricingEntryId":"rpe_prompt_v1"}],"totalCost":0.42}`),
			PriceCurrency:    "quota",
			PriceSource:      "initial_catalog",
			PromptTokens:     100,
			CompletionTokens: 20,
			TotalTokens:      120,
			CreatedAt:        time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}
	service := NewService(store)

	entries, total, err := service.ListUsageLogs(context.Background(), UsageLogFilter{
		OrganizationID: " org_1 ",
		UserID:         " user_1 ",
		APITokenID:     " tok_1 ",
		ChannelID:      " ch_1 ",
		FeatureType:    " workspace_chat ",
		Status:         " success ",
		Model:          " gpt-4o ",
		Provider:       " openai ",
		QuotaMode:      " relay_billing ",
		Limit:          250,
		Offset:         -10,
	})
	if err != nil {
		t.Fatalf("list usage logs: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("expected one usage log and total=1, got len=%d total=%d", len(entries), total)
	}
	if store.filter.Limit != 100 || store.filter.Offset != 0 {
		t.Fatalf("expected normalized limit=100 offset=0, got limit=%d offset=%d", store.filter.Limit, store.filter.Offset)
	}
	if store.filter.OrganizationID != "org_1" || store.filter.UserID != "user_1" || store.filter.APITokenID != "tok_1" {
		t.Fatalf("expected trimmed identity filters, got %#v", store.filter)
	}
	if store.filter.FeatureType != "workspace_chat" || store.filter.QuotaMode != "relay_billing" {
		t.Fatalf("expected trimmed usage classification filters, got %#v", store.filter)
	}
	if entries[0].RequestID != "req_1" || entries[0].Cost != 0.42 || entries[0].LatencyMS != 41 {
		t.Fatalf("expected gateway usage fields in response, got %#v", entries[0])
	}
	if entries[0].FeatureType != "workspace_chat" || entries[0].QuotaMode != "relay_billing" {
		t.Fatalf("expected usage classification fields in response, got %#v", entries[0])
	}
	if entries[0].PriceCurrency != "quota" || entries[0].PriceSource != "initial_catalog" ||
		!strings.Contains(string(entries[0].PriceSnapshot), `"pricingEntryId":"rpe_prompt_v1"`) {
		t.Fatalf("expected pricing snapshot fields in response, got %#v", entries[0])
	}
}

func TestServiceListUsageLogsAttachesRequestLogEvidenceByRequestID(t *testing.T) {
	store := &usageLogStoreSpy{
		entries: []*UsageLogEntry{
			{
				ID:        "usage_with_request_log",
				RequestID: "req_join_1",
				Model:     "gpt-4o",
			},
			{
				ID:    "usage_without_request_id",
				Model: "gpt-4o",
			},
		},
		total: 2,
	}
	evidenceStore := &requestLogEvidenceStoreSpy{
		evidence: map[string]RequestLogEvidence{
			"req_join_1": {
				RequestID:      "req_join_1",
				RequestLogID:   "550e8400-e29b-41d4-a716-446655440000",
				Service:        "relay",
				Endpoint:       "/v1/chat/completions",
				Method:         "POST",
				StatusCode:     200,
				DurationMS:     42,
				RequestTokens:  100,
				ResponseTokens: 20,
				Model:          "gpt-4o",
				CostUSD:        0.42,
				TraceID:        "650e8400-e29b-41d4-a716-446655440000",
				Timestamp:      time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store, WithRequestLogEvidenceStore(evidenceStore))

	entries, total, err := service.ListUsageLogs(context.Background(), UsageLogFilter{RequestID: " req_join_1 "})
	if err != nil {
		t.Fatalf("list usage logs: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Fatalf("expected two usage entries and total=2, got len=%d total=%d", len(entries), total)
	}
	if len(evidenceStore.requestIDs) != 1 || evidenceStore.requestIDs[0] != "req_join_1" {
		t.Fatalf("expected evidence lookup for req_join_1 only, got %#v", evidenceStore.requestIDs)
	}
	if entries[0].RequestLogEvidence == nil {
		t.Fatalf("expected request log evidence on first usage entry")
	}
	if entries[0].RequestLogEvidence.RequestLogID != "550e8400-e29b-41d4-a716-446655440000" ||
		entries[0].RequestLogEvidence.Service != "relay" ||
		entries[0].RequestLogEvidence.Endpoint != "/v1/chat/completions" ||
		entries[0].RequestLogEvidence.CostUSD != 0.42 {
		t.Fatalf("unexpected request log evidence: %+v", entries[0].RequestLogEvidence)
	}
	if entries[1].RequestLogEvidence != nil {
		t.Fatalf("expected entry without request id to have no request log evidence, got %+v", entries[1].RequestLogEvidence)
	}
}

func TestServiceGetUsageRequestLogCoverageSummarizesMissingRequestLogs(t *testing.T) {
	store := &usageLogStoreSpy{
		entries: []*UsageLogEntry{
			{ID: "usage_with_request_log", RequestID: " req_join_1 ", Model: "gpt-4o"},
			{ID: "usage_missing_request_log", RequestID: "req_missing_log", Model: "gpt-4o"},
			{ID: "usage_missing_request_id", Model: "gpt-4o"},
		},
		total: 3,
	}
	evidenceStore := &requestLogEvidenceStoreSpy{
		evidence: map[string]RequestLogEvidence{
			"req_join_1": {
				RequestID:    "req_join_1",
				RequestLogID: "550e8400-e29b-41d4-a716-446655440000",
				Service:      "relay",
				Endpoint:     "/v1/chat/completions",
				Method:       "POST",
				StatusCode:   200,
				DurationMS:   42,
				Model:        "gpt-4o",
				CostUSD:      0.42,
				Timestamp:    time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store, WithRequestLogEvidenceStore(evidenceStore))

	summary, err := service.GetUsageRequestLogCoverage(context.Background(), UsageLogFilter{
		OrganizationID: " org_1 ",
		Model:          " gpt-4o ",
		Limit:          250,
		Offset:         -10,
	})
	if err != nil {
		t.Fatalf("get usage request log coverage: %v", err)
	}
	if store.filter.OrganizationID != "org_1" || store.filter.Model != "gpt-4o" || store.filter.Limit != 100 || store.filter.Offset != 0 {
		t.Fatalf("expected normalized usage-log filter, got %#v", store.filter)
	}
	if len(evidenceStore.requestIDs) != 2 || evidenceStore.requestIDs[0] != "req_join_1" || evidenceStore.requestIDs[1] != "req_missing_log" {
		t.Fatalf("expected evidence lookup for non-empty request ids only, got %#v", evidenceStore.requestIDs)
	}
	if summary.CheckedRecords != 3 ||
		summary.UsageRowsWithRequestID != 2 ||
		summary.UsageRowsMissingRequestID != 1 ||
		summary.MatchedRequestLogRecords != 1 ||
		summary.MissingRequestLogRecords != 1 {
		t.Fatalf("unexpected request-log coverage summary: %+v", summary)
	}
	if len(summary.Issues) != 2 {
		t.Fatalf("expected two coverage issues, got %+v", summary.Issues)
	}
	if summary.Issues[0].ID != "usage_missing_request_log" ||
		summary.Issues[0].RequestID != "req_missing_log" ||
		summary.Issues[0].Issue != "missing_request_log" {
		t.Fatalf("expected missing request-log issue first, got %+v", summary.Issues)
	}
	if summary.Issues[1].ID != "usage_missing_request_id" || summary.Issues[1].Issue != "missing_request_id" {
		t.Fatalf("expected missing request-id issue second, got %+v", summary.Issues)
	}
}

func TestServiceGetUsageAnalyticsNormalizesFilterAndReturnsDimensions(t *testing.T) {
	to := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -7)
	store := &usageLogStoreSpy{
		analytics: UsageAnalytics{
			ByModel: []UsageAnalyticsBucket{{
				Dimension:    "model",
				Key:          "gpt-4o",
				RequestCount: 3,
				TotalTokens:  1200,
				TotalCost:    0.42,
			}},
			ByFeature: []UsageAnalyticsBucket{{
				Dimension:    "feature",
				Key:          "chat",
				RequestCount: 2,
				TotalTokens:  800,
				TotalCost:    0.3,
			}},
			ByUser: []UsageAnalyticsBucket{{
				Dimension:    "user",
				Key:          "user_1",
				RequestCount: 1,
				TotalTokens:  400,
				TotalCost:    0.12,
			}},
			ByTime: []UsageAnalyticsBucket{{
				Dimension:    "time",
				Key:          "2026-06-04",
				RequestCount: 1,
				TotalTokens:  400,
				TotalCost:    0.12,
				StartedAt:    to,
			}},
			ByChannel: []UsageAnalyticsBucket{{
				Dimension:    "channel",
				Key:          "ch_1",
				RequestCount: 2,
				TotalTokens:  600,
				TotalCost:    0.18,
			}},
			ByProvider: []UsageAnalyticsBucket{{
				Dimension:    "provider",
				Key:          "openai",
				RequestCount: 2,
				TotalTokens:  600,
				TotalCost:    0.18,
			}},
			CrossDimensions: []UsageAnalyticsBucket{{
				Dimension:    "model_time",
				Key:          "gpt-4o|2026-06-04",
				Primary:      "gpt-4o",
				Secondary:    "2026-06-04",
				RequestCount: 1,
				TotalTokens:  400,
				TotalCost:    0.12,
				StartedAt:    to,
			}},
		},
	}
	service := NewService(store)

	analytics, err := service.GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
		OrganizationID: " org_1 ",
		UserID:         " user_1 ",
		APIType:        " chat ",
		From:           from,
		To:             to,
		Model:          " gpt-4o ",
		ChannelID:      " ch_1 ",
		Provider:       " openai ",
		Status:         " success ",
		Limit:          250,
	})
	if err != nil {
		t.Fatalf("GetUsageAnalytics returned error: %v", err)
	}
	if store.analyticsFilter.OrganizationID != "org_1" || store.analyticsFilter.UserID != "user_1" {
		t.Fatalf("expected trimmed analytics filter, got %#v", store.analyticsFilter)
	}
	if store.analyticsFilter.APIType != "chat" || store.analyticsFilter.Model != "gpt-4o" || store.analyticsFilter.ChannelID != "ch_1" ||
		store.analyticsFilter.Provider != "openai" || store.analyticsFilter.Status != "success" {
		t.Fatalf("expected trimmed gateway analytics filters, got %#v", store.analyticsFilter)
	}
	if store.analyticsFilter.Limit != 100 {
		t.Fatalf("expected analytics limit to clamp to 100, got %d", store.analyticsFilter.Limit)
	}
	if !store.analyticsFilter.From.Equal(from) || !store.analyticsFilter.To.Equal(to) {
		t.Fatalf("expected explicit analytics range, got from=%s to=%s", store.analyticsFilter.From, store.analyticsFilter.To)
	}
	if len(analytics.ByModel) != 1 || len(analytics.ByFeature) != 1 || len(analytics.ByUser) != 1 || len(analytics.ByTime) != 1 ||
		len(analytics.ByChannel) != 1 || len(analytics.ByProvider) != 1 {
		t.Fatalf("expected all analytics dimensions, got %+v", analytics)
	}
	if analytics.ByChannel[0].Dimension != "channel" || analytics.ByChannel[0].Key != "ch_1" || analytics.ByProvider[0].Key != "openai" {
		t.Fatalf("expected channel/provider cost dimensions, got channels=%+v providers=%+v", analytics.ByChannel, analytics.ByProvider)
	}
	if len(analytics.CrossDimensions) != 1 || analytics.CrossDimensions[0].Dimension != "model_time" || analytics.CrossDimensions[0].Primary != "gpt-4o" {
		t.Fatalf("expected cross dimension analytics, got %+v", analytics.CrossDimensions)
	}
}

func TestServiceGetUsageAnalyticsNormalizesGranularity(t *testing.T) {
	store := &usageLogStoreSpy{}
	service := NewService(store)

	for _, granularity := range []string{" minute ", "week", " month "} {
		if _, err := service.GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
			Granularity: granularity,
		}); err != nil {
			t.Fatalf("GetUsageAnalytics returned error: %v", err)
		}
		if store.analyticsFilter.Granularity != strings.TrimSpace(granularity) {
			t.Fatalf("expected trimmed analytics granularity, got %q for input %q", store.analyticsFilter.Granularity, granularity)
		}
	}

	if _, err := service.GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{
		Granularity: "fortnight",
	}); err != nil {
		t.Fatalf("GetUsageAnalytics returned error: %v", err)
	}
	if store.analyticsFilter.Granularity != "day" {
		t.Fatalf("expected invalid analytics granularity to default to day, got %q", store.analyticsFilter.Granularity)
	}
}

func TestServiceGetUsageAnalyticsUsesDedicatedAnalyticsStore(t *testing.T) {
	defaultStore := &usageLogStoreSpy{
		analytics: UsageAnalytics{
			ByModel: []UsageAnalyticsBucket{{Dimension: "model", Key: "postgres"}},
		},
	}
	analyticsStore := &usageAnalyticsStoreSpy{
		analytics: UsageAnalytics{
			ByModel: []UsageAnalyticsBucket{{Dimension: "model", Key: "clickhouse"}},
		},
	}
	service := NewService(defaultStore, WithUsageAnalyticsStore(analyticsStore))

	analytics, err := service.GetUsageAnalytics(context.Background(), UsageAnalyticsFilter{Granularity: "hour"})
	if err != nil {
		t.Fatalf("GetUsageAnalytics returned error: %v", err)
	}
	if analytics.ByModel[0].Key != "clickhouse" {
		t.Fatalf("expected dedicated analytics store result, got %+v", analytics.ByModel)
	}
	if analyticsStore.filter.Granularity != "hour" {
		t.Fatalf("expected normalized filter to reach dedicated analytics store, got %#v", analyticsStore.filter)
	}
	if defaultStore.analyticsFilter.Granularity != "" {
		t.Fatalf("expected default store not to receive analytics filter, got %#v", defaultStore.analyticsFilter)
	}
}

func TestUsageAnalyticsWhereIncludesGatewayFilters(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	where, args := usageAnalyticsWhere(UsageAnalyticsFilter{
		From:      from,
		To:        to,
		APIType:   "chat",
		Model:     "gpt-4o",
		ChannelID: "ch_1",
		Provider:  "openai",
		Status:    "success",
	})

	for _, want := range []string{
		"api_type = $3",
		"model_id = $4",
		"channel_id = $5",
		"provider = $6",
		"status = $7",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("expected analytics where to include %q, got %s", want, where)
		}
	}
	if len(args) != 7 || args[2] != "chat" || args[3] != "gpt-4o" || args[4] != "ch_1" || args[5] != "openai" || args[6] != "success" {
		t.Fatalf("expected gateway analytics args after time range, got %#v", args)
	}
}

func TestServiceGetRelayUsagePriceReconciliationNormalizesFilter(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	store := &usageLogStoreSpy{
		reconciliation: &RelayUsagePriceReconciliationSummary{
			CheckedRecords:         2,
			MatchedRecords:         1,
			MissingSnapshotRecords: 1,
			Limit:                  100,
			Issues: []RelayUsagePriceReconciliationIssue{{
				ID:    "usage_missing_snapshot",
				Issue: "missing_snapshot",
			}},
		},
	}
	service := NewService(store)

	summary, err := service.GetRelayUsagePriceReconciliation(context.Background(), RelayUsagePriceReconciliationFilter{
		OrganizationID: " org_1 ",
		UserID:         " user_1 ",
		APITokenID:     " tok_1 ",
		RequestID:      " req_1 ",
		APIType:        " chat ",
		FeatureType:    " workspace_chat ",
		QuotaMode:      " relay_billing ",
		Model:          " gpt-4o ",
		ChannelID:      " ch_1 ",
		Provider:       " openai ",
		Status:         " success ",
		From:           from,
		To:             to,
		Limit:          250,
		Offset:         -10,
	})
	if err != nil {
		t.Fatalf("get relay usage price reconciliation: %v", err)
	}
	if summary == nil || summary.CheckedRecords != 2 || len(summary.Issues) != 1 {
		t.Fatalf("expected reconciliation summary from store, got %+v", summary)
	}
	if store.reconciliationFilter.OrganizationID != "org_1" ||
		store.reconciliationFilter.UserID != "user_1" ||
		store.reconciliationFilter.APITokenID != "tok_1" ||
		store.reconciliationFilter.RequestID != "req_1" ||
		store.reconciliationFilter.APIType != "chat" ||
		store.reconciliationFilter.FeatureType != "workspace_chat" ||
		store.reconciliationFilter.QuotaMode != "relay_billing" ||
		store.reconciliationFilter.Model != "gpt-4o" ||
		store.reconciliationFilter.ChannelID != "ch_1" ||
		store.reconciliationFilter.Provider != "openai" ||
		store.reconciliationFilter.Status != "success" {
		t.Fatalf("expected trimmed reconciliation filters, got %#v", store.reconciliationFilter)
	}
	if store.reconciliationFilter.Limit != 100 || store.reconciliationFilter.Offset != 0 {
		t.Fatalf("expected normalized limit=100 offset=0, got limit=%d offset=%d", store.reconciliationFilter.Limit, store.reconciliationFilter.Offset)
	}
	if !store.reconciliationFilter.From.Equal(from) || !store.reconciliationFilter.To.Equal(to) {
		t.Fatalf("expected reconciliation time range to pass through, got from=%s to=%s", store.reconciliationFilter.From, store.reconciliationFilter.To)
	}
}

type usageLogStoreSpy struct {
	Store
	filter               UsageLogFilter
	analyticsFilter      UsageAnalyticsFilter
	reconciliationFilter RelayUsagePriceReconciliationFilter
	entries              []*UsageLogEntry
	total                int
	analytics            UsageAnalytics
	reconciliation       *RelayUsagePriceReconciliationSummary
}

func (s *usageLogStoreSpy) ListUsageLogs(_ context.Context, filter UsageLogFilter) ([]*UsageLogEntry, int, error) {
	s.filter = filter
	return s.entries, s.total, nil
}

func (s *usageLogStoreSpy) GetUsageAnalytics(_ context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	s.analyticsFilter = filter
	return s.analytics, nil
}

func (s *usageLogStoreSpy) GetRelayUsagePriceReconciliation(_ context.Context, filter RelayUsagePriceReconciliationFilter) (*RelayUsagePriceReconciliationSummary, error) {
	s.reconciliationFilter = filter
	return s.reconciliation, nil
}

type usageAnalyticsStoreSpy struct {
	filter    UsageAnalyticsFilter
	analytics UsageAnalytics
}

func (s *usageAnalyticsStoreSpy) GetUsageAnalytics(_ context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	s.filter = filter
	return s.analytics, nil
}

type requestLogEvidenceStoreSpy struct {
	requestIDs []string
	evidence   map[string]RequestLogEvidence
}

func (s *requestLogEvidenceStoreSpy) ListRequestLogEvidence(_ context.Context, requestIDs []string) (map[string]RequestLogEvidence, error) {
	s.requestIDs = append([]string(nil), requestIDs...)
	return s.evidence, nil
}
