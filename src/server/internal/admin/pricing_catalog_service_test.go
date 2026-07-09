package admin

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

func TestServiceCreateRelayPricingCatalogImportBuildsDiffAndNormalizesEntries(t *testing.T) {
	effectiveFrom := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	store := &pricingCatalogStoreFake{
		active: []RelayPricingCatalogEntry{
			{
				ID:        "rpe_existing_prompt",
				APIType:   "chat",
				Model:     "gpt-4o",
				Dimension: "prompt_tokens",
				UnitCost:  0.002,
				Markup:    1,
				Currency:  "quota",
				Source:    "litellm",
				Active:    true,
			},
			{
				ID:        "rpe_existing_completion",
				APIType:   "chat",
				Model:     "gpt-4o",
				Dimension: "completion_tokens",
				UnitCost:  0.008,
				Markup:    1,
				Currency:  "quota",
				Source:    "litellm",
				Active:    true,
			},
		},
	}
	service := NewService(store)

	catalogImport, err := service.CreateRelayPricingCatalogImport(context.Background(), auth.Session{
		User: auth.User{ID: "admin_1", Email: "admin@example.test"},
	}, RelayPricingCatalogImportRequest{
		Provider:          " OpenAI ",
		Source:            "litellm",
		SourceHash:        "sha256:abc",
		DeactivateMissing: true,
		EffectiveFrom:     &effectiveFrom,
		Entries: []RelayPricingCatalogEntry{
			{
				APIType:   "chat",
				Model:     "gpt-4o",
				Dimension: "prompt_tokens",
				UnitCost:  0.003,
			},
			{
				APIType:   "chat",
				Model:     "gpt-4o-mini",
				Dimension: "prompt_tokens",
				UnitCost:  0.0002,
				Currency:  "USD",
			},
		},
	}, "203.0.113.10")
	if err != nil {
		t.Fatalf("create pricing catalog import: %v", err)
	}
	if catalogImport.Provider != "openai" || catalogImport.Source != "litellm" || catalogImport.Status != "pending" {
		t.Fatalf("expected normalized import metadata, got %+v", catalogImport)
	}
	if len(catalogImport.Entries) != 2 || catalogImport.Entries[0].ID == "" || catalogImport.Entries[0].Markup != 1 || catalogImport.Entries[1].Currency != "usd" {
		t.Fatalf("expected normalized entries with ids/defaults, got %+v", catalogImport.Entries)
	}
	if catalogImport.Diff.Added != 1 || catalogImport.Diff.Updated != 1 || catalogImport.Diff.Deactivated != 1 {
		t.Fatalf("expected add/update/deactivate diff, got %+v", catalogImport.Diff)
	}
	if store.created == nil || store.created.ImportedBy != "admin_1" || store.created.ImportedByEmail != "admin@example.test" {
		t.Fatalf("expected import to be persisted with actor evidence, got %+v", store.created)
	}
}

func TestServiceCreateRelayPricingCatalogImportRejectsInvalidEntries(t *testing.T) {
	service := NewService(&pricingCatalogStoreFake{})
	_, err := service.CreateRelayPricingCatalogImport(context.Background(), auth.Session{}, RelayPricingCatalogImportRequest{
		Provider: "openai",
		Source:   "litellm",
		Entries: []RelayPricingCatalogEntry{{
			APIType:   "chat",
			Model:     "gpt-4o",
			Dimension: "unsupported_dimension",
			UnitCost:  0.001,
		}},
	}, "")
	if err == nil {
		t.Fatal("expected invalid dimension to be rejected")
	}
}

func TestServiceCreateRelayPricingCatalogRollbackImportBuildsPendingRestoreDiff(t *testing.T) {
	before := RelayPricingCatalogEntry{
		ID:        "rpe_old_prompt",
		APIType:   "chat",
		Model:     "gpt-4o",
		Dimension: "prompt_tokens",
		UnitCost:  0.002,
		Markup:    1,
		Currency:  "quota",
		Source:    "litellm",
		Active:    false,
	}
	after := before
	after.ID = "rpe_new_prompt"
	after.UnitCost = 0.003
	after.Active = true
	store := &pricingCatalogStoreFake{
		active: []RelayPricingCatalogEntry{after},
		imports: map[string]*RelayPricingCatalogImport{
			"rpci_original": {
				ID:       "rpci_original",
				Provider: "openai",
				Source:   "litellm",
				Status:   "approved",
				Diff: RelayPricingCatalogDiff{Entries: []RelayPricingCatalogDiffEntry{{
					Action: "update",
					Key:    "chat/gpt-4o/prompt_tokens",
					Before: &before,
					After:  &after,
				}}},
			},
		},
	}
	service := NewService(store)

	rollbackImport, err := service.CreateRelayPricingCatalogRollbackImport(context.Background(), auth.Session{
		User: auth.User{ID: "admin_1", Email: "admin@example.test"},
	}, "rpci_original", RelayPricingCatalogRollbackRequest{Notes: "restore previous gpt-4o pricing"}, "203.0.113.11")
	if err != nil {
		t.Fatalf("create rollback import: %v", err)
	}
	if rollbackImport.Status != "pending" || rollbackImport.Source != "rollback:rpci_original" || rollbackImport.Provider != "openai" {
		t.Fatalf("expected pending rollback import metadata, got %+v", rollbackImport)
	}
	if rollbackImport.Diff.Updated != 1 || len(rollbackImport.Diff.Entries) != 1 {
		t.Fatalf("expected one restore update, got %+v", rollbackImport.Diff)
	}
	if rollbackImport.Diff.Entries[0].After == nil || rollbackImport.Diff.Entries[0].After.UnitCost != 0.002 || !rollbackImport.Diff.Entries[0].After.Active {
		t.Fatalf("expected rollback to restore previous active price, got %+v", rollbackImport.Diff.Entries[0])
	}
}

func TestServiceCreateRelayPricingCatalogRollbackImportRejectsChangedCatalog(t *testing.T) {
	before := RelayPricingCatalogEntry{ID: "rpe_old", APIType: "chat", Model: "gpt-4o", Dimension: "prompt_tokens", UnitCost: 0.002, Markup: 1, Currency: "quota", Source: "litellm"}
	after := before
	after.ID = "rpe_new"
	after.UnitCost = 0.003
	current := after
	current.ID = "rpe_newer"
	current.UnitCost = 0.004
	service := NewService(&pricingCatalogStoreFake{
		active: []RelayPricingCatalogEntry{current},
		imports: map[string]*RelayPricingCatalogImport{
			"rpci_original": {
				ID:       "rpci_original",
				Provider: "openai",
				Source:   "litellm",
				Status:   "approved",
				Diff: RelayPricingCatalogDiff{Entries: []RelayPricingCatalogDiffEntry{{
					Action: "update",
					Key:    "chat/gpt-4o/prompt_tokens",
					Before: &before,
					After:  &after,
				}}},
			},
		},
	})

	_, err := service.CreateRelayPricingCatalogRollbackImport(context.Background(), auth.Session{}, "rpci_original", RelayPricingCatalogRollbackRequest{}, "")
	if err == nil {
		t.Fatal("expected changed catalog rollback conflict")
	}
}

func TestServiceRejectRelayPricingCatalogImportRecordsAudit(t *testing.T) {
	store := &pricingCatalogStoreFake{
		created: &RelayPricingCatalogImport{ID: "rpci_pending", Status: "pending"},
	}
	service := NewService(store)

	rejected, err := service.RejectRelayPricingCatalogImport(context.Background(), auth.Session{
		User: auth.User{ID: "admin_1", Email: "admin@example.test"},
	}, "rpci_pending", RelayPricingCatalogRejectRequest{Reason: "bad source hash"}, "203.0.113.12")
	if err != nil {
		t.Fatalf("reject import: %v", err)
	}
	if rejected.Status != "rejected" || store.rejectedReason != "bad source hash" {
		t.Fatalf("expected rejected import with reason, got import=%+v reason=%q", rejected, store.rejectedReason)
	}
	if len(store.auditEntries) != 1 || store.auditEntries[0].Action != "pricing.relay_catalog.import.reject" {
		t.Fatalf("expected reject audit entry, got %+v", store.auditEntries)
	}
}

func TestParseLiteLLMPricingCatalogFiltersProviderAndChecksRequiredModels(t *testing.T) {
	body := []byte(`{
		"gpt-4o": {
			"litellm_provider": "openai",
			"mode": "chat",
			"input_cost_per_token": 0.000002,
			"output_cost_per_token": 0.000008
		},
		"text-embedding-3-small": {
			"litellm_provider": "openai",
			"mode": "embedding",
			"input_cost_per_token": 0.00000002
		},
		"claude-3-5-sonnet": {
			"litellm_provider": "anthropic",
			"mode": "chat",
			"input_cost_per_token": 0.000003,
			"output_cost_per_token": 0.000015
		}
	}`)

	entries, skipped, err := parseLiteLLMPricingCatalog(body, liteLLMParseOptions{
		Provider:       "openai",
		Source:         "litellm",
		RequiredModels: []string{"gpt-4o", "text-embedding-3-small"},
	})
	if err != nil {
		t.Fatalf("parse LiteLLM pricing: %v", err)
	}
	if skipped != 1 {
		t.Fatalf("expected one skipped non-openai row, got %d", skipped)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 pricing entries, got %+v", entries)
	}
	if entries[0].Model != "gpt-4o" || entries[0].APIType != "chat" || entries[0].Dimension != "prompt_tokens" || entries[0].UnitCost != 0.000002 {
		t.Fatalf("unexpected first parsed entry: %+v", entries[0])
	}
	if entries[2].Model != "text-embedding-3-small" || entries[2].APIType != "embeddings" || entries[2].Dimension != "prompt_tokens" {
		t.Fatalf("unexpected embedding entry: %+v", entries[2])
	}

	_, _, err = parseLiteLLMPricingCatalog(body, liteLLMParseOptions{
		Provider:       "openai",
		Source:         "litellm",
		RequiredModels: []string{"gpt-4o", "missing-model"},
	})
	if err == nil {
		t.Fatal("expected missing required model to fail")
	}
}

func TestServiceCreatesPendingRelayPricingCatalogImportFromLiteLLMSync(t *testing.T) {
	store := &pricingCatalogStoreFake{}
	service := NewService(store)

	catalogImport, err := service.CreateRelayPricingCatalogImportFromLiteLLMSync(context.Background(), auth.Session{
		User: auth.User{ID: "admin_1", Email: "admin@example.test"},
	}, RelayPricingCatalogSyncRequest{
		Provider: "openai",
		SourceJSON: []byte(`{
			"gpt-4o": {
				"litellm_provider": "openai",
				"mode": "chat",
				"input_cost_per_token": 0.000002,
				"output_cost_per_token": 0.000008
			}
		}`),
		RequiredModels: []string{"gpt-4o"},
	}, "203.0.113.13")
	if err != nil {
		t.Fatalf("create sync import: %v", err)
	}
	if catalogImport.Status != "pending" || catalogImport.Provider != "openai" || catalogImport.Source != "litellm" {
		t.Fatalf("expected pending litellm import, got %+v", catalogImport)
	}
	if catalogImport.SourceHash == "" || len(catalogImport.Entries) != 2 || catalogImport.Diff.Added != 2 {
		t.Fatalf("expected hashed two-entry pending import, got %+v", catalogImport)
	}
	if store.approvedImport != nil {
		t.Fatalf("sync must not approve imports automatically, got %+v", store.approvedImport)
	}
	if len(store.syncRuns) != 1 || store.syncRuns[0].Status != "pending_import" || store.syncRuns[0].ImportID != catalogImport.ID {
		t.Fatalf("expected sync run to point at pending import, got %+v", store.syncRuns)
	}
	if len(store.auditEntries) != 2 || store.auditEntries[1].Action != "pricing.relay_catalog.sync.create" {
		t.Fatalf("expected create and sync audit entries, got %+v", store.auditEntries)
	}
}

func TestRunRelayPricingCatalogFreshnessSyncRecordsUnchangedWithoutPendingImport(t *testing.T) {
	store := &pricingCatalogStoreFake{
		active: []RelayPricingCatalogEntry{
			{ID: "rpe_prompt", APIType: "chat", Model: "gpt-4o", Dimension: "prompt_tokens", UnitCost: 0.000002, Markup: 1, Currency: "quota", Source: "litellm", Active: true},
			{ID: "rpe_completion", APIType: "chat", Model: "gpt-4o", Dimension: "completion_tokens", UnitCost: 0.000008, Markup: 1, Currency: "quota", Source: "litellm", Active: true},
		},
	}
	service := NewService(store)

	run, err := service.RunRelayPricingCatalogFreshnessSync(context.Background(), auth.Session{
		User: auth.User{ID: "system", Email: "system@example.test"},
	}, RelayPricingCatalogSyncRequest{
		Provider: "openai",
		SourceJSON: []byte(`{
			"gpt-4o": {
				"litellm_provider": "openai",
				"mode": "chat",
				"input_cost_per_token": 0.000002,
				"output_cost_per_token": 0.000008
			}
		}`),
		RequiredModels: []string{"gpt-4o"},
	}, "worker")
	if err != nil {
		t.Fatalf("run freshness sync: %v", err)
	}
	if run.Status != "unchanged" || run.ImportID != "" || run.EntryCount != 2 {
		t.Fatalf("expected unchanged freshness run without import, got %+v", run)
	}
	if store.created != nil {
		t.Fatalf("unchanged scheduled freshness sync must not create pending import, got %+v", store.created)
	}
	if len(store.auditEntries) != 1 || store.auditEntries[0].Action != "pricing.relay_catalog.sync.freshness_unchanged" {
		t.Fatalf("expected unchanged sync audit, got %+v", store.auditEntries)
	}
}

func TestRunRelayPricingCatalogFreshnessSyncCreatesPendingImportForMaterialDiff(t *testing.T) {
	store := &pricingCatalogStoreFake{
		active: []RelayPricingCatalogEntry{
			{ID: "rpe_prompt", APIType: "chat", Model: "gpt-4o", Dimension: "prompt_tokens", UnitCost: 0.000001, Markup: 1, Currency: "quota", Source: "litellm", Active: true},
		},
	}
	service := NewService(store)

	run, err := service.RunRelayPricingCatalogFreshnessSync(context.Background(), auth.Session{
		User: auth.User{ID: "system", Email: "system@example.test"},
	}, RelayPricingCatalogSyncRequest{
		Provider: "openai",
		SourceJSON: []byte(`{
			"gpt-4o": {
				"litellm_provider": "openai",
				"mode": "chat",
				"input_cost_per_token": 0.000002,
				"output_cost_per_token": 0.000008
			}
		}`),
		RequiredModels: []string{"gpt-4o"},
	}, "worker")
	if err != nil {
		t.Fatalf("run freshness sync: %v", err)
	}
	if run.Status != "pending_import" || run.ImportID == "" || run.EntryCount != 2 {
		t.Fatalf("expected pending import freshness run, got %+v", run)
	}
	if store.created == nil || store.created.Status != "pending" || store.created.Diff.Updated != 1 || store.created.Diff.Added != 1 {
		t.Fatalf("expected material diff pending import, got %+v", store.created)
	}
	if store.approvedImport != nil {
		t.Fatalf("scheduled freshness must not approve imports automatically, got %+v", store.approvedImport)
	}
}

type pricingCatalogStoreFake struct {
	Store
	active         []RelayPricingCatalogEntry
	imports        map[string]*RelayPricingCatalogImport
	created        *RelayPricingCatalogImport
	approvedImport *RelayPricingCatalogImport
	rejectedImport *RelayPricingCatalogImport
	rejectedReason string
	filter         RelayPricingCatalogImportFilter
	syncRuns       []*RelayPricingCatalogSyncRun
	syncRunFilter  RelayPricingCatalogSyncRunFilter
	auditEntries   []*AuditEntry
}

func (s *pricingCatalogStoreFake) ListActiveRelayPricingCatalogEntries(ctx context.Context) ([]RelayPricingCatalogEntry, error) {
	return append([]RelayPricingCatalogEntry{}, s.active...), nil
}

func (s *pricingCatalogStoreFake) CreateRelayPricingCatalogImport(ctx context.Context, catalogImport RelayPricingCatalogImport) (*RelayPricingCatalogImport, error) {
	s.created = &catalogImport
	if s.imports == nil {
		s.imports = map[string]*RelayPricingCatalogImport{}
	}
	s.imports[catalogImport.ID] = &catalogImport
	return &catalogImport, nil
}

func (s *pricingCatalogStoreFake) GetRelayPricingCatalogImport(ctx context.Context, importID string) (*RelayPricingCatalogImport, error) {
	if s.imports != nil {
		if catalogImport, ok := s.imports[importID]; ok {
			return catalogImport, nil
		}
	}
	if s.created != nil && s.created.ID == importID {
		return s.created, nil
	}
	return nil, ErrRelayPricingCatalogImportNotFound
}

func (s *pricingCatalogStoreFake) ListRelayPricingCatalogImports(ctx context.Context, filter RelayPricingCatalogImportFilter) ([]*RelayPricingCatalogImport, int, error) {
	s.filter = filter
	return []*RelayPricingCatalogImport{s.created}, 1, nil
}

func (s *pricingCatalogStoreFake) CreateRelayPricingCatalogSyncRun(ctx context.Context, run RelayPricingCatalogSyncRun) (*RelayPricingCatalogSyncRun, error) {
	s.syncRuns = append(s.syncRuns, &run)
	return &run, nil
}

func (s *pricingCatalogStoreFake) ListRelayPricingCatalogSyncRuns(ctx context.Context, filter RelayPricingCatalogSyncRunFilter) ([]*RelayPricingCatalogSyncRun, int, error) {
	s.syncRunFilter = filter
	return append([]*RelayPricingCatalogSyncRun{}, s.syncRuns...), len(s.syncRuns), nil
}

func (s *pricingCatalogStoreFake) ApproveRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail string) (*RelayPricingCatalogImport, error) {
	if s.created == nil {
		return nil, ErrRelayPricingCatalogImportNotFound
	}
	approved := *s.created
	approved.Status = "approved"
	approved.ApprovedBy = actorID
	approved.ApprovedByEmail = actorEmail
	now := time.Now().UTC()
	approved.ApprovedAt = &now
	s.approvedImport = &approved
	return &approved, nil
}

func (s *pricingCatalogStoreFake) RejectRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail, reason string) (*RelayPricingCatalogImport, error) {
	catalogImport, err := s.GetRelayPricingCatalogImport(ctx, importID)
	if err != nil {
		return nil, err
	}
	if catalogImport.Status != "pending" {
		return nil, ErrRelayPricingCatalogImportNotPending
	}
	rejected := *catalogImport
	rejected.Status = "rejected"
	s.rejectedImport = &rejected
	s.rejectedReason = reason
	return &rejected, nil
}

func (s *pricingCatalogStoreFake) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	s.auditEntries = append(s.auditEntries, entry)
	return nil
}
