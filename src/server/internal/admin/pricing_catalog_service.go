package admin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
)

var (
	ErrRelayPricingCatalogStoreUnavailable  = errors.New("relay pricing catalog store is unavailable")
	ErrRelayPricingCatalogImportNotFound    = errors.New("relay pricing catalog import not found")
	ErrRelayPricingCatalogImportNotPending  = errors.New("relay pricing catalog import is not pending")
	ErrRelayPricingCatalogImportNotApproved = errors.New("relay pricing catalog import is not approved")
	ErrRelayPricingCatalogImportConflict    = errors.New("relay pricing catalog import conflicts with current catalog")
)

type RelayPricingCatalogStore interface {
	ListActiveRelayPricingCatalogEntries(ctx context.Context) ([]RelayPricingCatalogEntry, error)
	CreateRelayPricingCatalogImport(ctx context.Context, catalogImport RelayPricingCatalogImport) (*RelayPricingCatalogImport, error)
	GetRelayPricingCatalogImport(ctx context.Context, importID string) (*RelayPricingCatalogImport, error)
	ListRelayPricingCatalogImports(ctx context.Context, filter RelayPricingCatalogImportFilter) ([]*RelayPricingCatalogImport, int, error)
	ApproveRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail string) (*RelayPricingCatalogImport, error)
	RejectRelayPricingCatalogImport(ctx context.Context, importID, actorID, actorEmail, reason string) (*RelayPricingCatalogImport, error)
}

type RelayPricingCatalogSyncRunStore interface {
	CreateRelayPricingCatalogSyncRun(ctx context.Context, run RelayPricingCatalogSyncRun) (*RelayPricingCatalogSyncRun, error)
	ListRelayPricingCatalogSyncRuns(ctx context.Context, filter RelayPricingCatalogSyncRunFilter) ([]*RelayPricingCatalogSyncRun, int, error)
}

func (s *Service) relayPricingCatalogStore() (RelayPricingCatalogStore, error) {
	store, ok := s.store.(RelayPricingCatalogStore)
	if !ok || store == nil {
		return nil, ErrRelayPricingCatalogStoreUnavailable
	}
	return store, nil
}

func (s *Service) relayPricingCatalogSyncRunStore() (RelayPricingCatalogSyncRunStore, error) {
	store, ok := s.store.(RelayPricingCatalogSyncRunStore)
	if !ok || store == nil {
		return nil, ErrRelayPricingCatalogStoreUnavailable
	}
	return store, nil
}

func (s *Service) CreateRelayPricingCatalogImport(ctx context.Context, actor auth.Session, request RelayPricingCatalogImportRequest, ipAddress string) (*RelayPricingCatalogImport, error) {
	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, err
	}
	entries, provider, source, err := normalizeRelayPricingCatalogImportRequest(request)
	if err != nil {
		return nil, err
	}
	active, err := store.ListActiveRelayPricingCatalogEntries(ctx)
	if err != nil {
		return nil, err
	}
	diff := diffRelayPricingCatalog(active, entries, source, request.DeactivateMissing)
	importID, err := auth.NewID("rpci")
	if err != nil {
		return nil, fmt.Errorf("generate relay pricing catalog import id: %w", err)
	}
	catalogImport := RelayPricingCatalogImport{
		ID:                importID,
		Provider:          provider,
		Source:            source,
		SourceHash:        strings.TrimSpace(request.SourceHash),
		Status:            "pending",
		Notes:             strings.TrimSpace(request.Notes),
		DeactivateMissing: request.DeactivateMissing,
		ImportedBy:        actor.User.ID,
		ImportedByEmail:   actor.User.Email,
		Entries:           entries,
		Diff:              diff,
		CreatedAt:         time.Now().UTC(),
	}
	created, err := store.CreateRelayPricingCatalogImport(ctx, catalogImport)
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.import.create", "relay_pricing_catalog_import", importID, toJSON(created), ipAddress)
	return created, nil
}

func (s *Service) ListRelayPricingCatalogImports(ctx context.Context, filter RelayPricingCatalogImportFilter) ([]*RelayPricingCatalogImport, int, error) {
	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, 0, err
	}
	return store.ListRelayPricingCatalogImports(ctx, normalizeRelayPricingCatalogImportFilter(filter))
}

func (s *Service) ListRelayPricingCatalogSyncRuns(ctx context.Context, filter RelayPricingCatalogSyncRunFilter) ([]*RelayPricingCatalogSyncRun, int, error) {
	store, err := s.relayPricingCatalogSyncRunStore()
	if err != nil {
		return nil, 0, err
	}
	return store.ListRelayPricingCatalogSyncRuns(ctx, normalizeRelayPricingCatalogSyncRunFilter(filter))
}

func (s *Service) ApproveRelayPricingCatalogImport(ctx context.Context, actor auth.Session, importID, ipAddress string) (*RelayPricingCatalogImport, error) {
	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, err
	}
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, fmt.Errorf("import id is required")
	}
	approved, err := store.ApproveRelayPricingCatalogImport(ctx, importID, actor.User.ID, actor.User.Email)
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.import.approve", "relay_pricing_catalog_import", importID, toJSON(approved), ipAddress)
	return approved, nil
}

func (s *Service) RejectRelayPricingCatalogImport(ctx context.Context, actor auth.Session, importID string, request RelayPricingCatalogRejectRequest, ipAddress string) (*RelayPricingCatalogImport, error) {
	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, err
	}
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, fmt.Errorf("import id is required")
	}
	rejected, err := store.RejectRelayPricingCatalogImport(ctx, importID, actor.User.ID, actor.User.Email, strings.TrimSpace(request.Reason))
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.import.reject", "relay_pricing_catalog_import", importID, toJSON(rejected), ipAddress)
	return rejected, nil
}

func (s *Service) CreateRelayPricingCatalogRollbackImport(ctx context.Context, actor auth.Session, importID string, request RelayPricingCatalogRollbackRequest, ipAddress string) (*RelayPricingCatalogImport, error) {
	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, err
	}
	importID = strings.TrimSpace(importID)
	if importID == "" {
		return nil, fmt.Errorf("import id is required")
	}
	original, err := store.GetRelayPricingCatalogImport(ctx, importID)
	if err != nil {
		return nil, err
	}
	if original.Status != "approved" {
		return nil, ErrRelayPricingCatalogImportNotApproved
	}
	active, err := store.ListActiveRelayPricingCatalogEntries(ctx)
	if err != nil {
		return nil, err
	}
	diff, entries, err := rollbackRelayPricingCatalogDiff(*original, active)
	if err != nil {
		return nil, err
	}
	rollbackID, err := auth.NewID("rpci")
	if err != nil {
		return nil, fmt.Errorf("generate relay pricing rollback import id: %w", err)
	}
	notes := strings.TrimSpace(request.Notes)
	if notes == "" {
		notes = "Rollback of " + original.ID
	}
	rollbackImport := RelayPricingCatalogImport{
		ID:              rollbackID,
		Provider:        original.Provider,
		Source:          "rollback:" + original.ID,
		SourceHash:      original.SourceHash,
		Status:          "pending",
		Notes:           notes,
		ImportedBy:      actor.User.ID,
		ImportedByEmail: actor.User.Email,
		Entries:         entries,
		Diff:            diff,
		CreatedAt:       time.Now().UTC(),
	}
	created, err := store.CreateRelayPricingCatalogImport(ctx, rollbackImport)
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.import.rollback.create", "relay_pricing_catalog_import", rollbackID, toJSON(created), ipAddress)
	return created, nil
}

func normalizeRelayPricingCatalogImportFilter(filter RelayPricingCatalogImportFilter) RelayPricingCatalogImportFilter {
	filter.Provider = strings.ToLower(strings.TrimSpace(filter.Provider))
	filter.Source = strings.TrimSpace(filter.Source)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func normalizeRelayPricingCatalogSyncRunFilter(filter RelayPricingCatalogSyncRunFilter) RelayPricingCatalogSyncRunFilter {
	filter.Job = strings.ToLower(strings.TrimSpace(filter.Job))
	filter.Provider = strings.ToLower(strings.TrimSpace(filter.Provider))
	filter.Source = strings.TrimSpace(filter.Source)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func normalizeRelayPricingCatalogImportRequest(request RelayPricingCatalogImportRequest) ([]RelayPricingCatalogEntry, string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		return nil, "", "", fmt.Errorf("provider is required")
	}
	source := strings.TrimSpace(request.Source)
	if source == "" {
		return nil, "", "", fmt.Errorf("source is required")
	}
	if len(request.Entries) == 0 {
		return nil, "", "", fmt.Errorf("entries are required")
	}
	effectiveFrom := time.Now().UTC()
	if request.EffectiveFrom != nil && !request.EffectiveFrom.IsZero() {
		effectiveFrom = request.EffectiveFrom.UTC()
	}
	seen := map[string]struct{}{}
	entries := make([]RelayPricingCatalogEntry, 0, len(request.Entries))
	for _, input := range request.Entries {
		entry, err := normalizeRelayPricingCatalogEntry(input, source, effectiveFrom)
		if err != nil {
			return nil, "", "", err
		}
		key := relayPricingCatalogKey(entry)
		if _, ok := seen[key]; ok {
			return nil, "", "", fmt.Errorf("duplicate pricing entry for %s", relayPricingCatalogDisplayKey(entry))
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, provider, source, nil
}

func normalizeRelayPricingCatalogEntry(input RelayPricingCatalogEntry, defaultSource string, defaultEffectiveFrom time.Time) (RelayPricingCatalogEntry, error) {
	apiType, ok := normalizeRelayPricingAPIType(input.APIType)
	if !ok {
		return RelayPricingCatalogEntry{}, fmt.Errorf("unsupported apiType: %s", input.APIType)
	}
	dimension, ok := normalizeRelayPricingDimension(input.Dimension)
	if !ok {
		return RelayPricingCatalogEntry{}, fmt.Errorf("unsupported dimension: %s", input.Dimension)
	}
	model := strings.TrimSpace(input.Model)
	if model == "" && apiType != relaytypes.APITypeFiles.String() {
		return RelayPricingCatalogEntry{}, fmt.Errorf("model is required for apiType %s", apiType)
	}
	if input.UnitCost <= 0 {
		return RelayPricingCatalogEntry{}, fmt.Errorf("unitCost must be positive for %s/%s/%s", apiType, model, dimension)
	}
	markup := input.Markup
	if markup <= 0 {
		markup = 1
	}
	currency := strings.ToLower(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "quota"
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = defaultSource
	}
	effectiveFrom := defaultEffectiveFrom
	if input.EffectiveFrom != nil && !input.EffectiveFrom.IsZero() {
		effectiveFrom = input.EffectiveFrom.UTC()
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		generatedID, err := auth.NewID("rpe")
		if err != nil {
			return RelayPricingCatalogEntry{}, fmt.Errorf("generate relay pricing entry id: %w", err)
		}
		id = generatedID
	}
	return RelayPricingCatalogEntry{
		ID:            id,
		APIType:       apiType,
		Model:         model,
		Dimension:     dimension,
		UnitCost:      input.UnitCost,
		Markup:        markup,
		Currency:      currency,
		Source:        source,
		EffectiveFrom: &effectiveFrom,
		Active:        true,
	}, nil
}

func normalizeRelayPricingAPIType(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	for apiType := relaytypes.APITypeUnknown + 1; apiType <= relaytypes.APITypeModels; apiType++ {
		if apiType.String() == value {
			return value, true
		}
	}
	return "", false
}

func normalizeRelayPricingDimension(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "prompt_tokens", "completion_tokens", "total_tokens", "image_count", "video_count", "audio_seconds", "storage_bytes", "training_tokens":
		return value, true
	default:
		return "", false
	}
}

func diffRelayPricingCatalog(active []RelayPricingCatalogEntry, proposed []RelayPricingCatalogEntry, source string, deactivateMissing bool) RelayPricingCatalogDiff {
	current := map[string]RelayPricingCatalogEntry{}
	for _, entry := range active {
		current[relayPricingCatalogKey(entry)] = entry
	}
	proposedKeys := map[string]struct{}{}
	diff := RelayPricingCatalogDiff{Entries: []RelayPricingCatalogDiffEntry{}}
	for _, entry := range proposed {
		key := relayPricingCatalogKey(entry)
		proposedKeys[key] = struct{}{}
		before, exists := current[key]
		diffEntry := RelayPricingCatalogDiffEntry{
			Key:   relayPricingCatalogDisplayKey(entry),
			After: cloneRelayPricingCatalogEntry(entry),
		}
		switch {
		case !exists:
			diffEntry.Action = "add"
			diff.Added++
		case relayPricingCatalogEntryEquivalent(before, entry):
			diffEntry.Action = "unchanged"
			diffEntry.Before = cloneRelayPricingCatalogEntry(before)
			diff.Unchanged++
		default:
			diffEntry.Action = "update"
			diffEntry.Before = cloneRelayPricingCatalogEntry(before)
			diff.Updated++
		}
		diff.Entries = append(diff.Entries, diffEntry)
	}
	if deactivateMissing {
		for _, entry := range active {
			if entry.Source != source {
				continue
			}
			key := relayPricingCatalogKey(entry)
			if _, ok := proposedKeys[key]; ok {
				continue
			}
			diff.Deactivated++
			diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
				Action: "deactivate",
				Key:    relayPricingCatalogDisplayKey(entry),
				Before: cloneRelayPricingCatalogEntry(entry),
				Reason: "missing from provider import",
			})
		}
	}
	return diff
}

func relayPricingCatalogEntryEquivalent(a, b RelayPricingCatalogEntry) bool {
	return a.APIType == b.APIType &&
		a.Model == b.Model &&
		a.Dimension == b.Dimension &&
		math.Abs(a.UnitCost-b.UnitCost) <= 0.0000000001 &&
		math.Abs(a.Markup-b.Markup) <= 0.0000000001 &&
		a.Currency == b.Currency &&
		a.Source == b.Source
}

func relayPricingCatalogKey(entry RelayPricingCatalogEntry) string {
	return entry.APIType + "\x00" + entry.Model + "\x00" + entry.Dimension
}

func relayPricingCatalogDisplayKey(entry RelayPricingCatalogEntry) string {
	model := entry.Model
	if model == "" {
		model = "*"
	}
	return entry.APIType + "/" + model + "/" + entry.Dimension
}

func cloneRelayPricingCatalogEntry(entry RelayPricingCatalogEntry) *RelayPricingCatalogEntry {
	cloned := entry
	if entry.EffectiveFrom != nil {
		effectiveFrom := *entry.EffectiveFrom
		cloned.EffectiveFrom = &effectiveFrom
	}
	return &cloned
}

func rollbackRelayPricingCatalogDiff(original RelayPricingCatalogImport, active []RelayPricingCatalogEntry) (RelayPricingCatalogDiff, []RelayPricingCatalogEntry, error) {
	current := map[string]RelayPricingCatalogEntry{}
	for _, entry := range active {
		current[relayPricingCatalogKey(entry)] = entry
	}
	diff := RelayPricingCatalogDiff{Entries: []RelayPricingCatalogDiffEntry{}}
	entries := []RelayPricingCatalogEntry{}
	for _, originalEntry := range original.Diff.Entries {
		switch originalEntry.Action {
		case "add":
			if originalEntry.After == nil {
				return diff, nil, fmt.Errorf("relay pricing catalog rollback %s missing added entry", originalEntry.Key)
			}
			key := relayPricingCatalogKey(*originalEntry.After)
			currentEntry, exists := current[key]
			if !exists {
				diff.Unchanged++
				diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
					Action: "unchanged",
					Key:    originalEntry.Key,
					Reason: "rollback target already inactive",
				})
				continue
			}
			if !relayPricingCatalogEntryEquivalent(currentEntry, *originalEntry.After) {
				return diff, nil, fmt.Errorf("%w: %s has changed after import %s", ErrRelayPricingCatalogImportConflict, originalEntry.Key, original.ID)
			}
			diff.Deactivated++
			diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
				Action: "deactivate",
				Key:    originalEntry.Key,
				Before: cloneRelayPricingCatalogEntry(currentEntry),
				Reason: "rollback added price from " + original.ID,
			})
		case "update":
			if originalEntry.Before == nil || originalEntry.After == nil {
				return diff, nil, fmt.Errorf("relay pricing catalog rollback %s missing update entries", originalEntry.Key)
			}
			key := relayPricingCatalogKey(*originalEntry.After)
			currentEntry, exists := current[key]
			if !exists {
				return diff, nil, fmt.Errorf("%w: %s is no longer active", ErrRelayPricingCatalogImportConflict, originalEntry.Key)
			}
			if relayPricingCatalogEntryEquivalent(currentEntry, *originalEntry.Before) {
				diff.Unchanged++
				diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
					Action: "unchanged",
					Key:    originalEntry.Key,
					Before: cloneRelayPricingCatalogEntry(currentEntry),
					After:  cloneRelayPricingCatalogEntry(currentEntry),
					Reason: "rollback target already restored",
				})
				continue
			}
			if !relayPricingCatalogEntryEquivalent(currentEntry, *originalEntry.After) {
				return diff, nil, fmt.Errorf("%w: %s has changed after import %s", ErrRelayPricingCatalogImportConflict, originalEntry.Key, original.ID)
			}
			restored := cloneRelayPricingCatalogEntry(*originalEntry.Before)
			restored.Active = true
			diff.Updated++
			diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
				Action: "update",
				Key:    originalEntry.Key,
				Before: cloneRelayPricingCatalogEntry(currentEntry),
				After:  restored,
				Reason: "rollback updated price from " + original.ID,
			})
			entries = append(entries, *restored)
		case "deactivate":
			if originalEntry.Before == nil {
				return diff, nil, fmt.Errorf("relay pricing catalog rollback %s missing deactivated entry", originalEntry.Key)
			}
			key := relayPricingCatalogKey(*originalEntry.Before)
			currentEntry, exists := current[key]
			if exists {
				if relayPricingCatalogEntryEquivalent(currentEntry, *originalEntry.Before) {
					diff.Unchanged++
					diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
						Action: "unchanged",
						Key:    originalEntry.Key,
						Before: cloneRelayPricingCatalogEntry(currentEntry),
						After:  cloneRelayPricingCatalogEntry(currentEntry),
						Reason: "rollback target already restored",
					})
					continue
				}
				return diff, nil, fmt.Errorf("%w: %s has changed after import %s", ErrRelayPricingCatalogImportConflict, originalEntry.Key, original.ID)
			}
			restored := cloneRelayPricingCatalogEntry(*originalEntry.Before)
			restored.Active = true
			diff.Added++
			diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
				Action: "add",
				Key:    originalEntry.Key,
				After:  restored,
				Reason: "rollback deactivated price from " + original.ID,
			})
			entries = append(entries, *restored)
		case "unchanged":
			diff.Unchanged++
			diff.Entries = append(diff.Entries, RelayPricingCatalogDiffEntry{
				Action: "unchanged",
				Key:    originalEntry.Key,
				Before: originalEntry.Before,
				After:  originalEntry.After,
				Reason: "original import left price unchanged",
			})
		default:
			return diff, nil, fmt.Errorf("unsupported relay pricing catalog rollback action: %s", originalEntry.Action)
		}
	}
	return diff, entries, nil
}
