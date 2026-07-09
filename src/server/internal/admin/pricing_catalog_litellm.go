package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	relaytypes "oblivious/server/internal/relay/types"
)

const (
	defaultRelayPricingSyncMaxBytes = int64(10 << 20)
	maxRelayPricingSyncMaxBytes     = int64(50 << 20)
)

func (s *Service) CreateRelayPricingCatalogImportFromLiteLLMSync(ctx context.Context, actor auth.Session, request RelayPricingCatalogSyncRequest, ipAddress string) (*RelayPricingCatalogImport, error) {
	startedAt := time.Now().UTC()
	draft, err := s.buildRelayPricingLiteLLMSyncDraft(ctx, request)
	if err != nil {
		_, _ = s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:        "freshness",
			Provider:   strings.ToLower(strings.TrimSpace(request.Provider)),
			Source:     relayPricingSyncSourceName(request.Source),
			Status:     "failed",
			Error:      err.Error(),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		})
		return nil, err
	}

	notes := strings.TrimSpace(request.Notes)
	if notes == "" {
		notes = fmt.Sprintf("LiteLLM price sync from %s; skipped %d unsupported rows", draft.sourceRef, draft.skipped)
	}
	catalogImport, err := s.CreateRelayPricingCatalogImport(ctx, actor, RelayPricingCatalogImportRequest{
		Provider:          draft.provider,
		Source:            draft.source,
		SourceHash:        draft.sourceHash,
		Notes:             notes,
		DeactivateMissing: request.DeactivateMissing,
		EffectiveFrom:     request.EffectiveFrom,
		Entries:           draft.entries,
	}, ipAddress)
	if err != nil {
		_, _ = s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:          "freshness",
			Provider:     draft.provider,
			Source:       draft.source,
			SourceRef:    draft.sourceRef,
			SourceHash:   draft.sourceHash,
			Status:       "failed",
			EntryCount:   len(draft.entries),
			SkippedCount: draft.skipped,
			Error:        err.Error(),
			Metadata:     relayPricingSyncRunMetadata(map[string]any{"mode": "manual_import"}),
			StartedAt:    startedAt,
			FinishedAt:   time.Now().UTC(),
		})
		return nil, err
	}
	run, err := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
		Job:          "freshness",
		Provider:     draft.provider,
		Source:       draft.source,
		SourceRef:    draft.sourceRef,
		SourceHash:   draft.sourceHash,
		Status:       "pending_import",
		ImportID:     catalogImport.ID,
		EntryCount:   len(draft.entries),
		SkippedCount: draft.skipped,
		Metadata: relayPricingSyncRunMetadata(map[string]any{
			"mode":             "manual_import",
			"added":            catalogImport.Diff.Added,
			"updated":          catalogImport.Diff.Updated,
			"unchanged":        catalogImport.Diff.Unchanged,
			"deactivated":      catalogImport.Diff.Deactivated,
			"requiresApproval": true,
		}),
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.sync.create", "relay_pricing_catalog_import", catalogImport.ID, toJSON(map[string]any{
		"provider":       draft.provider,
		"source":         draft.source,
		"sourceRef":      draft.sourceRef,
		"sourceHash":     draft.sourceHash,
		"syncRunId":      run.ID,
		"entryCount":     len(draft.entries),
		"skippedCount":   draft.skipped,
		"pendingImport":  catalogImport.ID,
		"approvalStatus": catalogImport.Status,
	}), ipAddress)
	return catalogImport, nil
}

func (s *Service) RunRelayPricingCatalogFreshnessSync(ctx context.Context, actor auth.Session, request RelayPricingCatalogSyncRequest, ipAddress string) (*RelayPricingCatalogSyncRun, error) {
	startedAt := time.Now().UTC()
	draft, err := s.buildRelayPricingLiteLLMSyncDraft(ctx, request)
	if err != nil {
		run, runErr := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:        "freshness",
			Provider:   strings.ToLower(strings.TrimSpace(request.Provider)),
			Source:     relayPricingSyncSourceName(request.Source),
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

	store, err := s.relayPricingCatalogStore()
	if err != nil {
		return nil, err
	}
	active, err := store.ListActiveRelayPricingCatalogEntries(ctx)
	if err != nil {
		run, runErr := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:          "freshness",
			Provider:     draft.provider,
			Source:       draft.source,
			SourceRef:    draft.sourceRef,
			SourceHash:   draft.sourceHash,
			Status:       "failed",
			EntryCount:   len(draft.entries),
			SkippedCount: draft.skipped,
			Error:        err.Error(),
			StartedAt:    startedAt,
			FinishedAt:   time.Now().UTC(),
		})
		if runErr != nil {
			return nil, runErr
		}
		return run, err
	}
	diff := diffRelayPricingCatalog(active, draft.entries, draft.source, request.DeactivateMissing)
	metadata := relayPricingSyncRunMetadata(map[string]any{
		"mode":             "scheduled_freshness",
		"added":            diff.Added,
		"updated":          diff.Updated,
		"unchanged":        diff.Unchanged,
		"deactivated":      diff.Deactivated,
		"requiresApproval": false,
	})
	if relayPricingCatalogDiffMaterialChangeCount(diff) == 0 {
		run, err := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:          "freshness",
			Provider:     draft.provider,
			Source:       draft.source,
			SourceRef:    draft.sourceRef,
			SourceHash:   draft.sourceHash,
			Status:       "unchanged",
			EntryCount:   len(draft.entries),
			SkippedCount: draft.skipped,
			Metadata:     metadata,
			StartedAt:    startedAt,
			FinishedAt:   time.Now().UTC(),
		})
		if err != nil {
			return nil, err
		}
		_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.sync.freshness_unchanged", "relay_pricing_sync_run", run.ID, toJSON(run), ipAddress)
		return run, nil
	}

	catalogImport, err := s.CreateRelayPricingCatalogImport(ctx, actor, RelayPricingCatalogImportRequest{
		Provider:          draft.provider,
		Source:            draft.source,
		SourceHash:        draft.sourceHash,
		Notes:             fmt.Sprintf("Scheduled LiteLLM price freshness sync from %s; skipped %d unsupported rows", draft.sourceRef, draft.skipped),
		DeactivateMissing: request.DeactivateMissing,
		EffectiveFrom:     request.EffectiveFrom,
		Entries:           draft.entries,
	}, ipAddress)
	if err != nil {
		run, runErr := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
			Job:          "freshness",
			Provider:     draft.provider,
			Source:       draft.source,
			SourceRef:    draft.sourceRef,
			SourceHash:   draft.sourceHash,
			Status:       "failed",
			EntryCount:   len(draft.entries),
			SkippedCount: draft.skipped,
			Error:        err.Error(),
			Metadata:     metadata,
			StartedAt:    startedAt,
			FinishedAt:   time.Now().UTC(),
		})
		if runErr != nil {
			return nil, runErr
		}
		return run, err
	}
	metadata = relayPricingSyncRunMetadata(map[string]any{
		"mode":             "scheduled_freshness",
		"added":            catalogImport.Diff.Added,
		"updated":          catalogImport.Diff.Updated,
		"unchanged":        catalogImport.Diff.Unchanged,
		"deactivated":      catalogImport.Diff.Deactivated,
		"requiresApproval": true,
	})
	run, err := s.recordRelayPricingCatalogSyncRun(ctx, RelayPricingCatalogSyncRun{
		Job:          "freshness",
		Provider:     draft.provider,
		Source:       draft.source,
		SourceRef:    draft.sourceRef,
		SourceHash:   draft.sourceHash,
		Status:       "pending_import",
		ImportID:     catalogImport.ID,
		EntryCount:   len(draft.entries),
		SkippedCount: draft.skipped,
		Metadata:     metadata,
		StartedAt:    startedAt,
		FinishedAt:   time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	_ = s.LogAction(ctx, actor.User.ID, actor.User.Email, "pricing.relay_catalog.sync.freshness_pending_import", "relay_pricing_sync_run", run.ID, toJSON(map[string]any{
		"syncRun":       run,
		"pendingImport": catalogImport.ID,
	}), ipAddress)
	return run, nil
}

type relayPricingLiteLLMSyncDraft struct {
	provider   string
	source     string
	sourceRef  string
	sourceHash string
	entries    []RelayPricingCatalogEntry
	skipped    int
}

func (s *Service) buildRelayPricingLiteLLMSyncDraft(ctx context.Context, request RelayPricingCatalogSyncRequest) (relayPricingLiteLLMSyncDraft, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		return relayPricingLiteLLMSyncDraft{}, fmt.Errorf("provider is required")
	}
	source := relayPricingSyncSourceName(request.Source)
	body, sourceRef, err := s.relayPricingSyncSourceBody(ctx, request)
	if err != nil {
		return relayPricingLiteLLMSyncDraft{}, err
	}
	sourceHash := "sha256:" + sha256Hex(body)
	entries, skipped, err := parseLiteLLMPricingCatalog(body, liteLLMParseOptions{
		Provider:       provider,
		Source:         source,
		EffectiveFrom:  request.EffectiveFrom,
		RequiredModels: request.RequiredModels,
	})
	if err != nil {
		return relayPricingLiteLLMSyncDraft{}, err
	}
	return relayPricingLiteLLMSyncDraft{
		provider:   provider,
		source:     source,
		sourceRef:  sourceRef,
		sourceHash: sourceHash,
		entries:    entries,
		skipped:    skipped,
	}, nil
}

func relayPricingSyncSourceName(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "litellm"
	}
	return source
}

func (s *Service) recordRelayPricingCatalogSyncRun(ctx context.Context, run RelayPricingCatalogSyncRun) (*RelayPricingCatalogSyncRun, error) {
	store, err := s.relayPricingCatalogSyncRunStore()
	if err != nil {
		return nil, err
	}
	if run.ID == "" {
		id, idErr := auth.NewID("rpcs")
		if idErr != nil {
			return nil, fmt.Errorf("generate relay pricing sync run id: %w", idErr)
		}
		run.ID = id
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now().UTC()
	}
	if len(run.Metadata) == 0 {
		run.Metadata = json.RawMessage(`{}`)
	}
	return store.CreateRelayPricingCatalogSyncRun(ctx, run)
}

func relayPricingSyncRunMetadata(value map[string]any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

func relayPricingCatalogDiffMaterialChangeCount(diff RelayPricingCatalogDiff) int {
	return diff.Added + diff.Updated + diff.Deactivated
}

func (s *Service) relayPricingSyncSourceBody(ctx context.Context, request RelayPricingCatalogSyncRequest) ([]byte, string, error) {
	if len(request.SourceJSON) > 0 {
		body := []byte(request.SourceJSON)
		if !json.Valid(body) {
			return nil, "", fmt.Errorf("sourceJson must be valid JSON")
		}
		return body, "inline-json", nil
	}
	sourceURL := strings.TrimSpace(request.SourceURL)
	if sourceURL == "" {
		return nil, "", fmt.Errorf("sourceUrl or sourceJson is required")
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return nil, "", fmt.Errorf("parse sourceUrl: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, "", fmt.Errorf("sourceUrl must use https")
	}
	if parsed.Host == "" {
		return nil, "", fmt.Errorf("sourceUrl host is required")
	}

	maxBytes := request.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultRelayPricingSyncMaxBytes
	}
	if maxBytes > maxRelayPricingSyncMaxBytes {
		maxBytes = maxRelayPricingSyncMaxBytes
	}
	client := s.relayPricingSyncHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create price sync request: %w", err)
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, "", fmt.Errorf("fetch price sync source: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("fetch price sync source returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read price sync source: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("price sync source exceeds %d bytes", maxBytes)
	}
	if !json.Valid(body) {
		return nil, "", fmt.Errorf("price sync source must be valid JSON")
	}
	return body, sourceURL, nil
}

type liteLLMParseOptions struct {
	Provider       string
	Source         string
	EffectiveFrom  *time.Time
	RequiredModels []string
}

type liteLLMPriceRow struct {
	LiteLLMProvider     string   `json:"litellm_provider"`
	Mode                string   `json:"mode"`
	InputCostPerToken   *float64 `json:"input_cost_per_token"`
	OutputCostPerToken  *float64 `json:"output_cost_per_token"`
	OutputCostPerImage  *float64 `json:"output_cost_per_image"`
	InputCostPerImage   *float64 `json:"input_cost_per_image"`
	CostPerImage        *float64 `json:"cost_per_image"`
	SupportedOpenAIArgs []string `json:"supported_openai_params"`
}

func parseLiteLLMPricingCatalog(body []byte, options liteLLMParseOptions) ([]RelayPricingCatalogEntry, int, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, 0, fmt.Errorf("parse LiteLLM pricing JSON: %w", err)
	}
	provider := strings.ToLower(strings.TrimSpace(options.Provider))
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "litellm"
	}
	required := map[string]struct{}{}
	for _, model := range options.RequiredModels {
		model = strings.TrimSpace(model)
		if model != "" {
			required[model] = struct{}{}
		}
	}

	models := make([]string, 0, len(raw))
	for model := range raw {
		models = append(models, model)
	}
	sort.Strings(models)

	entries := []RelayPricingCatalogEntry{}
	coveredModels := map[string]struct{}{}
	skipped := 0
	for _, model := range models {
		modelName := strings.TrimSpace(model)
		if modelName == "" {
			skipped++
			continue
		}
		var row liteLLMPriceRow
		if err := json.Unmarshal(raw[model], &row); err != nil {
			return nil, skipped, fmt.Errorf("parse LiteLLM pricing row %s: %w", model, err)
		}
		rowProvider := strings.ToLower(strings.TrimSpace(row.LiteLLMProvider))
		if provider != "" && rowProvider != provider {
			skipped++
			continue
		}
		apiType, ok := liteLLMModeToAPIType(row.Mode)
		if !ok {
			skipped++
			continue
		}
		before := len(entries)
		switch apiType {
		case relaytypes.APITypeChat, relaytypes.APITypeCompletions:
			appendTokenEntry := func(cost *float64, dimension string) {
				if cost != nil && *cost > 0 {
					entries = append(entries, liteLLMEntry(apiType, modelName, dimension, *cost, source, options.EffectiveFrom))
				}
			}
			appendTokenEntry(row.InputCostPerToken, "prompt_tokens")
			appendTokenEntry(row.OutputCostPerToken, "completion_tokens")
		case relaytypes.APITypeEmbeddings:
			if row.InputCostPerToken != nil && *row.InputCostPerToken > 0 {
				entries = append(entries, liteLLMEntry(apiType, modelName, "prompt_tokens", *row.InputCostPerToken, source, options.EffectiveFrom))
			}
		case relaytypes.APITypeImageGen:
			if cost := firstPositiveFloat(row.OutputCostPerImage, row.InputCostPerImage, row.CostPerImage); cost > 0 {
				entries = append(entries, liteLLMEntry(apiType, modelName, "image_count", cost, source, options.EffectiveFrom))
			}
		}
		if len(entries) == before {
			skipped++
			continue
		}
		coveredModels[modelName] = struct{}{}
	}
	if len(entries) == 0 {
		return nil, skipped, fmt.Errorf("LiteLLM pricing source produced no valid entries for provider %s", provider)
	}
	missing := []string{}
	for model := range required {
		if _, ok := coveredModels[model]; !ok {
			missing = append(missing, model)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return nil, skipped, fmt.Errorf("LiteLLM pricing source missing required models: %s", strings.Join(missing, ", "))
	}
	return entries, skipped, nil
}

func liteLLMModeToAPIType(mode string) (relaytypes.APIType, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "chat":
		return relaytypes.APITypeChat, true
	case "completion":
		return relaytypes.APITypeCompletions, true
	case "embedding":
		return relaytypes.APITypeEmbeddings, true
	case "image_generation":
		return relaytypes.APITypeImageGen, true
	default:
		return relaytypes.APITypeUnknown, false
	}
}

func liteLLMEntry(apiType relaytypes.APIType, model, dimension string, unitCost float64, source string, effectiveFrom *time.Time) RelayPricingCatalogEntry {
	return RelayPricingCatalogEntry{
		APIType:       apiType.String(),
		Model:         model,
		Dimension:     dimension,
		UnitCost:      unitCost,
		Markup:        1,
		Currency:      "quota",
		Source:        source,
		EffectiveFrom: effectiveFrom,
		Active:        true,
	}
}

func firstPositiveFloat(values ...*float64) float64 {
	for _, value := range values {
		if value != nil && *value > 0 {
			return *value
		}
	}
	return 0
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
