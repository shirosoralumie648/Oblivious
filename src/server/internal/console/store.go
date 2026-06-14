package console

import (
	"context"
	"time"
)

func (s *SQLStore) GetUsageSummary(ctx context.Context, organizationID, userID string) (UsageSummary, error) {
	summary := UsageSummary{
		Period:   "7d",
		Requests: 0,
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(request_count), 0)
		FROM usage_records
		WHERE organization_id = $1
		  AND user_id = $2
		  AND created_at >= NOW() - INTERVAL '7 days'
	`, organizationID, userID).Scan(&summary.Requests); err != nil {
		return UsageSummary{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			COALESCE(api_token_id, ''),
			COALESCE(request_id, ''),
			COALESCE(api_type, ''),
			model_id,
			COALESCE(provider, ''),
			COALESCE(channel_id, ''),
			COALESCE(status, ''),
			COALESCE(status_code, 0),
			COALESCE(error_code, ''),
			COALESCE(latency_ms, 0),
			COALESCE(cost, 0),
			input_tokens,
			output_tokens,
			COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens),
			created_at
		FROM usage_records
		WHERE organization_id = $1
		  AND user_id = $2
		  AND (api_type IS NOT NULL OR api_token_id IS NOT NULL OR channel_id IS NOT NULL)
		ORDER BY created_at DESC
		LIMIT 100
	`, organizationID, userID)
	if err != nil {
		return UsageSummary{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item APITokenUsageItem
		if err := rows.Scan(
			&item.ID,
			&item.APITokenID,
			&item.RequestID,
			&item.APIType,
			&item.Model,
			&item.Provider,
			&item.ChannelID,
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
			return UsageSummary{}, err
		}
		summary.Recent = append(summary.Recent, item)
	}
	if err := rows.Err(); err != nil {
		return UsageSummary{}, err
	}

	summary.ByModel, err = s.queryUsageDimensionSummary(ctx, organizationID, "model_id", "total_cost DESC, request_count DESC, key ASC", 10)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByFeature, err = s.queryUsageDimensionSummary(ctx, organizationID, "COALESCE(NULLIF(feature_type, ''), NULLIF(api_type, ''), 'chat')", "total_tokens DESC, request_count DESC, key ASC", 10)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.ByUser, err = s.queryUsageDimensionSummary(ctx, organizationID, "user_id", "total_cost DESC, request_count DESC, key ASC", 10)
	if err != nil {
		return UsageSummary{}, err
	}
	summary.TimeSeries, err = s.queryUsageTimeSeriesSummary(ctx, organizationID)
	if err != nil {
		return UsageSummary{}, err
	}

	return summary, nil
}

func (s *SQLStore) queryUsageDimensionSummary(ctx context.Context, organizationID, keyExpression, orderBy string, limit int) ([]UsageDimensionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			`+keyExpression+` AS key,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens)), 0)::int AS total_tokens,
			COALESCE(SUM(cost), 0)::double precision AS total_cost
		FROM usage_records
		WHERE organization_id = $1
		  AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY `+keyExpression+`
		ORDER BY `+orderBy+`
		LIMIT $2
	`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []UsageDimensionSummary{}
	for rows.Next() {
		var summary UsageDimensionSummary
		if err := rows.Scan(&summary.Key, &summary.RequestCount, &summary.TotalTokens, &summary.TotalCost); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *SQLStore) queryUsageTimeSeriesSummary(ctx context.Context, organizationID string) ([]UsageTimeSeriesSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			to_char(date_trunc('day', created_at), 'YYYY-MM-DD') AS bucket,
			COALESCE(SUM(request_count), 0)::int AS request_count,
			COALESCE(SUM(COALESCE(NULLIF(total_tokens, 0), input_tokens + output_tokens)), 0)::int AS total_tokens,
			COALESCE(SUM(cost), 0)::double precision AS total_cost
		FROM usage_records
		WHERE organization_id = $1
		  AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY date_trunc('day', created_at)
		ORDER BY date_trunc('day', created_at) ASC
		LIMIT 31
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []UsageTimeSeriesSummary{}
	for rows.Next() {
		var summary UsageTimeSeriesSummary
		if err := rows.Scan(&summary.Bucket, &summary.RequestCount, &summary.TotalTokens, &summary.TotalCost); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (s *SQLStore) GetModelSummaries(ctx context.Context, organizationID string) ([]ModelSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT model_id, COALESCE(SUM(request_count), 0) AS request_total
		FROM usage_records
		WHERE organization_id = $1
		  AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY model_id
		ORDER BY request_total DESC, model_id ASC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := []ModelSummary{}
	for rows.Next() {
		var model ModelSummary
		if err := rows.Scan(&model.ID, &model.Requests); err != nil {
			return nil, err
		}
		model.Label = model.ID
		models = append(models, model)
	}

	return models, rows.Err()
}

func (s *SQLStore) GetBillingSummary(ctx context.Context, organizationID string) (BillingSummary, error) {
	summary := BillingSummary{
		Period: "30d",
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0)
		FROM usage_records
		WHERE organization_id = $1
		  AND created_at >= NOW() - INTERVAL '30 days'
	`, organizationID).Scan(&summary.Requests, &summary.InputTokens, &summary.OutputTokens); err != nil {
		return BillingSummary{}, err
	}

	summary.EstimatedCostUSD = float64(summary.InputTokens+summary.OutputTokens) * 0.000002
	summary.CurrentSpendUSD = summary.EstimatedCostUSD
	summary.CreditLimitUSD = 100
	summary.BalanceUSD = summary.CreditLimitUSD - summary.CurrentSpendUSD
	now := time.Now().UTC()
	summary.NextInvoice = &BillingInvoiceSummary{
		ID:        now.Format("draft-2006-01"),
		Status:    "draft",
		AmountUSD: summary.CurrentSpendUSD,
		DueAt:     nextMonthStart(now).Format(time.RFC3339),
	}

	return summary, nil
}

func (s *SQLStore) ListBillingInvoices(ctx context.Context, organizationID string) ([]BillingInvoiceSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			status,
			CASE
				WHEN amount_paid > 0 THEN amount_paid
				ELSE amount_due
			END AS amount_usd,
			COALESCE(period_end, updated_at, created_at) AS due_at,
			COALESCE(hosted_invoice_url, '') AS hosted_invoice_url,
			COALESCE(invoice_pdf, '') AS invoice_pdf
		FROM billing_invoices
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT 25
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := []BillingInvoiceSummary{}
	for rows.Next() {
		var invoice BillingInvoiceSummary
		var dueAt time.Time
		if err := rows.Scan(&invoice.ID, &invoice.Status, &invoice.AmountUSD, &dueAt, &invoice.HostedInvoiceURL, &invoice.InvoicePDF); err != nil {
			return nil, err
		}
		invoice.DueAt = dueAt.UTC().Format(time.RFC3339)
		invoices = append(invoices, invoice)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(invoices) > 0 {
		return invoices, nil
	}

	summary, err := s.GetBillingSummary(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if summary.NextInvoice == nil {
		return []BillingInvoiceSummary{}, nil
	}
	return []BillingInvoiceSummary{*summary.NextInvoice}, nil
}

func nextMonthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}
