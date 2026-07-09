package admin

import (
	"context"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
)

// RelayPricingMaintenanceWorker records scheduled provider price freshness and
// usage-price reconciliation evidence. Price changes still require approval.
type RelayPricingMaintenanceWorker struct {
	service *Service
	config  RelayPricingMaintenanceWorkerConfig
}

type RelayPricingMaintenanceWorkerConfig struct {
	Interval             time.Duration
	Actor                auth.Session
	SyncRequest          RelayPricingCatalogSyncRequest
	ReconciliationFilter RelayUsagePriceReconciliationFilter
	OnError              func(error)
}

func NewRelayPricingMaintenanceWorker(service *Service, config RelayPricingMaintenanceWorkerConfig) *RelayPricingMaintenanceWorker {
	if config.Interval <= 0 {
		config.Interval = time.Hour
	}
	if config.ReconciliationFilter.Limit <= 0 {
		config.ReconciliationFilter.Limit = 100
	}
	return &RelayPricingMaintenanceWorker{service: service, config: config}
}

func (w *RelayPricingMaintenanceWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	w.report(w.RunOnce(ctx))
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.report(w.RunOnce(ctx))
		}
	}
}

func (w *RelayPricingMaintenanceWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.service == nil {
		return fmt.Errorf("relay pricing maintenance service is required")
	}
	var combined error
	if _, err := w.service.RunRelayPricingCatalogFreshnessSync(ctx, w.config.Actor, w.config.SyncRequest, "relay-pricing-maintenance-worker"); err != nil {
		combined = fmt.Errorf("relay pricing freshness sync: %w", err)
	}
	if _, err := w.service.RecordRelayUsagePriceReconciliationRun(ctx, w.config.Actor, w.config.ReconciliationFilter, "relay-pricing-maintenance-worker"); err != nil {
		if combined != nil {
			combined = fmt.Errorf("%v; relay pricing reconciliation: %w", combined, err)
		} else {
			combined = fmt.Errorf("relay pricing reconciliation: %w", err)
		}
	}
	return combined
}

func (w *RelayPricingMaintenanceWorker) report(err error) {
	if err != nil && w.config.OnError != nil {
		w.config.OnError(err)
	}
}

func (s *Service) RecordRelayUsagePriceReconciliationRun(ctx context.Context, actor auth.Session, filter RelayUsagePriceReconciliationFilter, ipAddress string) (*RelayPricingCatalogSyncRun, error) {
	startedAt := time.Now().UTC()
	if filter.To.IsZero() {
		filter.To = startedAt
	}
	if filter.From.IsZero() {
		filter.From = filter.To.Add(-24 * time.Hour)
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	summary, err := s.GetRelayUsagePriceReconciliation(ctx, filter)
	if err != nil {
		run, runErr := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:        "reconciliation",
			Status:     "failed",
			Error:      err.Error(),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		})
		if runErr != nil {
			return nil, runErr
		}
		return run, err
	}
	status := "succeeded"
	issueCount := summary.MissingSnapshotRecords + summary.MismatchedRecords
	if issueCount > 0 {
		status = "issues_found"
	}
	run, err := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
		Job:            "reconciliation",
		Status:         status,
		CheckedRecords: summary.CheckedRecords,
		IssueCount:     issueCount,
		Metadata: relayPricingSyncRunMetadata(map[string]any{
			"matchedRecords":         summary.MatchedRecords,
			"missingSnapshotRecords": summary.MissingSnapshotRecords,
			"mismatchedRecords":      summary.MismatchedRecords,
			"ledgerTotalCost":        summary.LedgerTotalCost,
			"snapshotTotalCost":      summary.SnapshotTotalCost,
			"deltaCost":              summary.DeltaCost,
			"from":                   filter.From,
			"to":                     filter.To,
		}),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.reconciliation.run", "relay_pricing_sync_run", run.ID, toJSON(run), ipAddress)
	return run, nil
}
