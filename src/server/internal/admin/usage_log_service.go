package admin

import (
	"context"
	"strings"
	"time"
)

func normalizeUsageLogFilter(filter UsageLogFilter) UsageLogFilter {
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
	filter.APITokenID = strings.TrimSpace(filter.APITokenID)
	filter.RequestID = strings.TrimSpace(filter.RequestID)
	filter.APIType = strings.TrimSpace(filter.APIType)
	filter.FeatureType = strings.TrimSpace(filter.FeatureType)
	filter.QuotaMode = strings.TrimSpace(filter.QuotaMode)
	filter.Model = strings.TrimSpace(filter.Model)
	filter.ChannelID = strings.TrimSpace(filter.ChannelID)
	filter.Provider = strings.TrimSpace(filter.Provider)
	filter.Status = strings.TrimSpace(filter.Status)
	return filter
}

func (s *Service) ListUsageLogs(ctx context.Context, filter UsageLogFilter) ([]*UsageLogEntry, int, error) {
	entries, total, err := s.store.ListUsageLogs(ctx, normalizeUsageLogFilter(filter))
	if err != nil {
		return nil, 0, err
	}
	if s.requestLogEvidenceStore == nil || len(entries) == 0 {
		return entries, total, nil
	}
	requestIDs := uniqueUsageLogRequestIDs(entries)
	if len(requestIDs) == 0 {
		return entries, total, nil
	}
	evidenceByRequestID, err := s.requestLogEvidenceStore.ListRequestLogEvidence(ctx, requestIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if evidence, ok := evidenceByRequestID[strings.TrimSpace(entry.RequestID)]; ok {
			evidenceCopy := evidence
			entry.RequestLogEvidence = &evidenceCopy
		}
	}
	return entries, total, nil
}

func uniqueUsageLogRequestIDs(entries []*UsageLogEntry) []string {
	seen := map[string]struct{}{}
	requestIDs := []string{}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		requestID := strings.TrimSpace(entry.RequestID)
		if requestID == "" {
			continue
		}
		if _, ok := seen[requestID]; ok {
			continue
		}
		seen[requestID] = struct{}{}
		requestIDs = append(requestIDs, requestID)
	}
	return requestIDs
}

func normalizeUsageAnalyticsFilter(filter UsageAnalyticsFilter) UsageAnalyticsFilter {
	filter.OrganizationID = strings.TrimSpace(filter.OrganizationID)
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.APIType = strings.TrimSpace(filter.APIType)
	filter.FeatureType = strings.TrimSpace(filter.FeatureType)
	filter.QuotaMode = strings.TrimSpace(filter.QuotaMode)
	filter.Model = strings.TrimSpace(filter.Model)
	filter.ChannelID = strings.TrimSpace(filter.ChannelID)
	filter.Provider = strings.TrimSpace(filter.Provider)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Granularity = normalizeUsageAnalyticsGranularity(filter.Granularity)
	if filter.Limit <= 0 {
		filter.Limit = 10
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.To.IsZero() {
		filter.To = time.Now().UTC()
	}
	if filter.From.IsZero() || !filter.From.Before(filter.To) {
		filter.From = filter.To.AddDate(0, 0, -30)
	}
	return filter
}

func normalizeUsageAnalyticsGranularity(granularity string) string {
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "second", "minute", "hour", "day", "week", "month":
		return strings.ToLower(strings.TrimSpace(granularity))
	default:
		return "day"
	}
}

func (s *Service) GetUsageAnalytics(ctx context.Context, filter UsageAnalyticsFilter) (UsageAnalytics, error) {
	normalized := normalizeUsageAnalyticsFilter(filter)
	if s.usageAnalyticsStore != nil {
		return s.usageAnalyticsStore.GetUsageAnalytics(ctx, normalized)
	}
	return s.store.GetUsageAnalytics(ctx, normalized)
}
