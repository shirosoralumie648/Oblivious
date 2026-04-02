package console

import "context"

func (s *SQLStore) GetUsageSummary(ctx context.Context, workspaceID string) (UsageSummary, error) {
	summary := UsageSummary{
		Period:   "7d",
		Requests: 0,
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(request_count), 0)
		FROM usage_records
		WHERE workspace_id = $1
		  AND created_at >= NOW() - INTERVAL '7 days'
	`, workspaceID).Scan(&summary.Requests); err != nil {
		return UsageSummary{}, err
	}

	return summary, nil
}

func (s *SQLStore) GetModelSummaries(ctx context.Context, workspaceID string) ([]ModelSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT model_id, COALESCE(SUM(request_count), 0) AS request_total
		FROM usage_records
		WHERE workspace_id = $1
		  AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY model_id
		ORDER BY request_total DESC, model_id ASC
	`, workspaceID)
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

func (s *SQLStore) GetBillingSummary(ctx context.Context, workspaceID string) (BillingSummary, error) {
	summary := BillingSummary{
		Period: "30d",
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(request_count), 0),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0)
		FROM usage_records
		WHERE workspace_id = $1
		  AND created_at >= NOW() - INTERVAL '30 days'
	`, workspaceID).Scan(&summary.Requests, &summary.InputTokens, &summary.OutputTokens); err != nil {
		return BillingSummary{}, err
	}

	summary.EstimatedCostUSD = float64(summary.InputTokens+summary.OutputTokens) * 0.000002

	return summary, nil
}
