package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/observability"
)

type marketplaceServiceStore struct {
	agents           map[string]*PublishedAgent
	installs         map[string]*AgentInstall
	reviews          map[string]*AgentReview
	templates        map[string]*MarketplaceTemplate
	templateInstalls map[string]*TemplateInstall
	settlementPrefs  map[string]MarketplaceSettlementPreferences
	rankingSignals   map[string]map[AgentRankingSignalEvent]int
	pendingReviews   []*PublishedAgent
	lastOwnerID      string
	lastOrgID        string
	lastListLimit    int
	lastListOffset   int
}

func newMarketplaceServiceStore() *marketplaceServiceStore {
	return &marketplaceServiceStore{
		agents: map[string]*PublishedAgent{
			"agent_approved": {
				ID:             "agent_approved",
				OrganizationID: "publisher_org",
				OwnerID:        "owner_1",
				Name:           "Approved Agent",
				Status:         "approved",
				Visibility:     "public",
			},
		},
		installs: make(map[string]*AgentInstall),
		reviews:  make(map[string]*AgentReview),
		templates: map[string]*MarketplaceTemplate{
			"tpl_1": {
				ID:             "tpl_1",
				OrganizationID: "publisher_org",
				Type:           "workflow",
				Name:           "Lead Intake",
				Description:    "Captures leads and routes follow-up work.",
				TemplateData:   json.RawMessage(`{"nodes":[{"id":"start"}]}`),
				Category:       "sales",
				Tags:           []string{"crm", "sales"},
				DownloadsCount: 7,
			},
		},
		templateInstalls: make(map[string]*TemplateInstall),
		settlementPrefs:  make(map[string]MarketplaceSettlementPreferences),
		rankingSignals:   make(map[string]map[AgentRankingSignalEvent]int),
	}
}

func marketplaceTenantKey(parts ...string) string {
	key := ""
	for _, part := range parts {
		if key != "" {
			key += ":"
		}
		key += part
	}
	return key
}

func (s *marketplaceServiceStore) CreateAgent(ctx context.Context, ownerID, organizationID string, input AgentPublishRequest) (*PublishedAgent, error) {
	s.lastOwnerID = ownerID
	s.lastOrgID = organizationID
	agent := &PublishedAgent{
		ID:             "agent_new",
		OrganizationID: organizationID,
		OwnerID:        ownerID,
		Name:           input.Name,
		Description:    input.Description,
		Tools:          input.Tools,
		Visibility:     input.Visibility,
		Status:         "pending_review",
		PricingType:    input.PricingType,
	}
	s.agents[agent.ID] = agent
	return agent, nil
}

func (s *marketplaceServiceStore) GetAgent(ctx context.Context, id string) (*PublishedAgent, error) {
	return s.agents[id], nil
}

func (s *marketplaceServiceStore) UpdateAgent(ctx context.Context, id, organizationID string, input AgentPublishRequest) (*PublishedAgent, error) {
	agent := s.agents[id]
	if agent == nil || agent.OrganizationID != organizationID {
		return nil, nil
	}
	agent.Name = input.Name
	agent.Description = input.Description
	agent.Tools = input.Tools
	agent.Visibility = input.Visibility
	agent.Status = "pending_review"
	return agent, nil
}

func (s *marketplaceServiceStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	if agent := s.agents[id]; agent != nil && agent.OrganizationID == organizationID {
		delete(s.agents, id)
	}
	return nil
}

func (s *marketplaceServiceStore) ListUserAgents(ctx context.Context, ownerID, organizationID string, limit, offset int) ([]*PublishedAgent, error) {
	s.lastOwnerID = ownerID
	s.lastOrgID = organizationID
	s.lastListLimit = limit
	s.lastListOffset = offset
	return []*PublishedAgent{{ID: "agent_owned", OrganizationID: organizationID, OwnerID: ownerID, Name: "Owned Agent"}}, nil
}

func (s *marketplaceServiceStore) ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error) {
	if s.pendingReviews != nil {
		return s.pendingReviews, nil
	}
	return []*PublishedAgent{{ID: "agent_new", Status: "pending_review"}}, nil
}

func (s *marketplaceServiceStore) ApproveAgent(ctx context.Context, id, reviewerID string) error {
	if agent := s.agents[id]; agent != nil {
		agent.Status = "approved"
	}
	return nil
}

func (s *marketplaceServiceStore) RejectAgent(ctx context.Context, id, reviewerID, reason string) error {
	if agent := s.agents[id]; agent != nil {
		agent.Status = "rejected"
		agent.ReviewReason = reason
	}
	return nil
}

func (s *marketplaceServiceStore) CreateVersion(ctx context.Context, agentID, organizationID string, version, changelog string, metadata string) (*AgentVersion, error) {
	return &AgentVersion{ID: "version_1", AgentID: agentID, OrganizationID: organizationID, Version: version, Changelog: changelog}, nil
}

func (s *marketplaceServiceStore) ListVersions(ctx context.Context, agentID string) ([]*AgentVersion, error) {
	return []*AgentVersion{{ID: "version_1", AgentID: agentID, Version: "1.0.0"}}, nil
}

func (s *marketplaceServiceStore) GetVersion(ctx context.Context, agentID, version string) (*AgentVersion, error) {
	return &AgentVersion{ID: "version_1", AgentID: agentID, Version: version}, nil
}

func (s *marketplaceServiceStore) InstallAgent(ctx context.Context, agentID, userID, organizationID, versionID string) (*AgentInstall, error) {
	install := &AgentInstall{ID: "install_1", AgentID: agentID, OrganizationID: organizationID, UserID: userID}
	s.installs[marketplaceTenantKey(organizationID, agentID, userID)] = install
	return install, nil
}

func (s *marketplaceServiceStore) UninstallAgent(ctx context.Context, agentID, userID, organizationID string) error {
	delete(s.installs, marketplaceTenantKey(organizationID, agentID, userID))
	return nil
}

func (s *marketplaceServiceStore) ListUserInstalls(ctx context.Context, userID, organizationID string) ([]*AgentInstall, error) {
	var installs []*AgentInstall
	for _, install := range s.installs {
		if install.UserID == userID && install.OrganizationID == organizationID {
			installs = append(installs, install)
		}
	}
	return installs, nil
}

func (s *marketplaceServiceStore) IsInstalled(ctx context.Context, agentID, userID, organizationID string) (bool, error) {
	_, ok := s.installs[marketplaceTenantKey(organizationID, agentID, userID)]
	return ok, nil
}

func (s *marketplaceServiceStore) RecordAgentRankingSignal(ctx context.Context, agentID string, event AgentRankingSignalEvent) error {
	if s.rankingSignals[agentID] == nil {
		s.rankingSignals[agentID] = make(map[AgentRankingSignalEvent]int)
	}
	s.rankingSignals[agentID][event]++
	return nil
}

func (s *marketplaceServiceStore) CreateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error) {
	review := &AgentReview{ID: "review_1", AgentID: input.AgentID, OrganizationID: organizationID, UserID: userID, Rating: input.Rating, Body: input.Body}
	s.reviews[marketplaceTenantKey(organizationID, input.AgentID, userID)] = review
	return review, nil
}

func (s *marketplaceServiceStore) UpdateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error) {
	review := &AgentReview{ID: "review_1", AgentID: input.AgentID, OrganizationID: organizationID, UserID: userID, Rating: input.Rating, Body: input.Body}
	s.reviews[marketplaceTenantKey(organizationID, input.AgentID, userID)] = review
	return review, nil
}

func (s *marketplaceServiceStore) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*AgentReview, error) {
	return []*AgentReview{{ID: "review_1", AgentID: agentID, Rating: 5}}, nil
}

func (s *marketplaceServiceStore) GetUserReview(ctx context.Context, agentID, userID, organizationID string) (*AgentReview, error) {
	return s.reviews[marketplaceTenantKey(organizationID, agentID, userID)], nil
}

func (s *marketplaceServiceStore) ListCategories(ctx context.Context) ([]*Category, error) {
	return []*Category{{ID: "cat_1", Name: "Productivity", Slug: "productivity"}}, nil
}

func (s *marketplaceServiceStore) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	if slug == "productivity" {
		return &Category{ID: "cat_1", Name: "Productivity", Slug: "productivity"}, nil
	}
	return nil, nil
}

func (s *marketplaceServiceStore) SetAgentTags(ctx context.Context, agentID string, tags []string) error {
	return nil
}

func (s *marketplaceServiceStore) GetDB() *sql.DB {
	return nil
}

func (s *marketplaceServiceStore) CreateTemplate(ctx context.Context, organizationID string, input TemplateCreateRequest) (*MarketplaceTemplate, error) {
	template := &MarketplaceTemplate{
		ID:             "tpl_new",
		OrganizationID: organizationID,
		Type:           input.Type,
		Name:           input.Name,
		Description:    input.Description,
		TemplateData:   input.TemplateData,
		Category:       input.Category,
		Tags:           input.Tags,
	}
	s.templates[template.ID] = template
	return template, nil
}

func (s *marketplaceServiceStore) ListTemplates(ctx context.Context, filter TemplateFilter) ([]*MarketplaceTemplate, int, error) {
	var templates []*MarketplaceTemplate
	for _, template := range s.templates {
		if filter.Type != "" && template.Type != filter.Type {
			continue
		}
		templates = append(templates, template)
	}
	return templates, len(templates), nil
}

func (s *marketplaceServiceStore) GetTemplate(ctx context.Context, id string) (*MarketplaceTemplate, error) {
	return s.templates[id], nil
}

func (s *marketplaceServiceStore) InstallTemplate(ctx context.Context, templateID, userID, organizationID string) (*TemplateInstall, error) {
	install := &TemplateInstall{ID: "tpl_install_1", TemplateID: templateID, UserID: userID, OrganizationID: organizationID}
	s.templateInstalls[marketplaceTenantKey(organizationID, templateID, userID)] = install
	if template := s.templates[templateID]; template != nil {
		template.DownloadsCount++
	}
	return install, nil
}

func (s *marketplaceServiceStore) GetPublisherSettlementPreferences(ctx context.Context, organizationID string) (*MarketplaceSettlementPreferences, error) {
	prefs := s.settlementPrefs[organizationID]
	return &prefs, nil
}

func (s *marketplaceServiceStore) UpdatePublisherSettlementPreferences(ctx context.Context, organizationID string, cycle string) (*MarketplaceSettlementPreferences, error) {
	prefs := MarketplaceSettlementPreferences{Cycle: cycle}
	s.settlementPrefs[organizationID] = prefs
	return &prefs, nil
}

type marketplaceAuditRecorder struct {
	actions []string
}

func (a *marketplaceAuditRecorder) LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error {
	a.actions = append(a.actions, action)
	return nil
}

type marketplaceAutomatedReviewer struct {
	agentIDs []string
	results  map[string]AutomatedReviewResult
}

func (r *marketplaceAutomatedReviewer) RunAutomatedReview(ctx context.Context, agentID string) (*AutomatedReviewResult, error) {
	r.agentIDs = append(r.agentIDs, agentID)
	result := r.results[agentID]
	if result.AgentID == "" {
		result.AgentID = agentID
	}
	return &result, nil
}

type marketplaceSLACaptureAlertSink struct {
	events []observability.AlertEvent
}

func (s *marketplaceSLACaptureAlertSink) Notify(_ context.Context, event observability.AlertEvent) error {
	s.events = append(s.events, event)
	return nil
}

func validAgentPublishRequest() AgentPublishRequest {
	return AgentPublishRequest{
		Name:                 "Release Helper",
		Description:          "Helps release managers prepare release candidates.",
		CategoryID:           "productivity",
		Tools:                `{"tools":[{"name":"checklist"}]}`,
		ExampleConversations: "[]",
		Visibility:           "public",
		PricingType:          "free",
		Version:              "1.0.0",
	}
}

func TestServicePublishAgentCreatesPendingReviewAndAudit(t *testing.T) {
	store := newMarketplaceServiceStore()
	audit := &marketplaceAuditRecorder{}
	reviewer := &marketplaceAutomatedReviewer{results: map[string]AutomatedReviewResult{
		"agent_new": {Decision: "pending_manual_review"},
	}}
	service := NewService(store, audit, WithAutomatedReview(reviewer))

	agent, err := service.PublishAgent(context.Background(), "owner_1", "org_1", "owner@example.com", validAgentPublishRequest(), "127.0.0.1")
	if err != nil {
		t.Fatalf("PublishAgent returned error: %v", err)
	}
	if agent.Status != "pending_review" {
		t.Fatalf("expected pending_review status, got %q", agent.Status)
	}
	if store.lastOwnerID != "owner_1" {
		t.Fatalf("expected owner_1 to be passed to store, got %q", store.lastOwnerID)
	}
	if store.lastOrgID != "org_1" || agent.OrganizationID != "org_1" {
		t.Fatalf("expected org_1 to be passed through, store=%q agent=%q", store.lastOrgID, agent.OrganizationID)
	}
	if len(audit.actions) != 1 || audit.actions[0] != "agent.publish" {
		t.Fatalf("expected agent.publish audit action, got %v", audit.actions)
	}
	if len(reviewer.agentIDs) != 1 || reviewer.agentIDs[0] != "agent_new" {
		t.Fatalf("expected automated review for created agent, got %v", reviewer.agentIDs)
	}
}

func TestServicePublishAgentRejectsAutomatedReviewFindings(t *testing.T) {
	store := newMarketplaceServiceStore()
	reviewer := &marketplaceAutomatedReviewer{results: map[string]AutomatedReviewResult{
		"agent_new": {
			Decision: "rejected",
			Findings: []ReviewFinding{{
				Type:     "prompt_injection",
				Severity: "critical",
				Field:    "system_prompt",
				Message:  "Prompt content attempts to override instructions or reveal hidden prompts.",
			}},
		},
	}}
	service := NewService(store, nil, WithAutomatedReview(reviewer))

	_, err := service.PublishAgent(context.Background(), "owner_1", "org_1", "owner@example.com", validAgentPublishRequest(), "127.0.0.1")
	if err == nil {
		t.Fatal("expected automated review rejection")
	}
	var reviewErr *AutomatedReviewError
	if !errors.As(err, &reviewErr) {
		t.Fatalf("expected AutomatedReviewError, got %T: %v", err, err)
	}
	if reviewErr.Result.Decision != "rejected" || len(reviewErr.Result.Findings) != 1 {
		t.Fatalf("expected structured rejected findings, got %+v", reviewErr.Result)
	}
	if reviewErr.Result.Findings[0].Type != "prompt_injection" {
		t.Fatalf("expected prompt_injection finding, got %+v", reviewErr.Result.Findings[0])
	}
}

func TestServicePublishAgentBlocksSensitiveAPIFindingWithDefaultScanner(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)
	req := validAgentPublishRequest()
	req.Tools = `{"tools":[{"name":"token-export","endpoint":"/oauth/tokens","scope":"admin:read"}]}`

	_, err := service.PublishAgent(context.Background(), "owner_1", "org_1", "owner@example.com", req, "127.0.0.1")
	if err == nil {
		t.Fatal("expected sensitive API finding to block publication")
	}
	var reviewErr *AutomatedReviewError
	if !errors.As(err, &reviewErr) {
		t.Fatalf("expected AutomatedReviewError, got %T: %v", err, err)
	}
	if reviewErr.Result.Decision != "rejected" {
		t.Fatalf("expected rejected decision, got %+v", reviewErr.Result)
	}
	if len(reviewErr.Result.Findings) == 0 || reviewErr.Result.Findings[0].Type != "sensitive_api" {
		t.Fatalf("expected sensitive_api finding, got %+v", reviewErr.Result.Findings)
	}
}

func TestServiceUpdateAgentRejectsAutomatedReviewFindings(t *testing.T) {
	store := newMarketplaceServiceStore()
	store.agents["agent_approved"].OwnerID = "owner_1"
	store.agents["agent_approved"].OrganizationID = "publisher_org"
	reviewer := &marketplaceAutomatedReviewer{results: map[string]AutomatedReviewResult{
		"agent_approved": {
			Decision: "rejected",
			Findings: []ReviewFinding{{
				Type:     "prompt_injection",
				Severity: "critical",
				Field:    "system_prompt",
				Message:  "Prompt content attempts to override instructions or reveal hidden prompts.",
			}},
		},
	}}
	service := NewService(store, nil, WithAutomatedReview(reviewer))

	_, err := service.UpdateAgent(context.Background(), "owner_1", "publisher_org", "owner@example.com", "agent_approved", validAgentPublishRequest(), "127.0.0.1")
	if err == nil {
		t.Fatal("expected automated review rejection")
	}
	var reviewErr *AutomatedReviewError
	if !errors.As(err, &reviewErr) {
		t.Fatalf("expected AutomatedReviewError, got %T: %v", err, err)
	}
	if len(reviewer.agentIDs) != 1 || reviewer.agentIDs[0] != "agent_approved" {
		t.Fatalf("expected automated review for updated agent, got %v", reviewer.agentIDs)
	}
	if reviewErr.Result.Decision != "rejected" || len(reviewErr.Result.Findings) != 1 {
		t.Fatalf("expected structured rejected findings, got %+v", reviewErr.Result)
	}
}

func TestServiceCreatesAndInstallsMarketplaceTemplate(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)

	created, err := service.CreateTemplate(context.Background(), "org_1", TemplateCreateRequest{
		Type:         "agent",
		Name:         "Launch Checklist",
		Description:  "Reusable launch agent for GTM handoffs.",
		TemplateData: json.RawMessage(`{"steps":["brief","review"]}`),
		Category:     "operations",
		Tags:         []string{"launch"},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if created.ID == "" || created.OrganizationID != "org_1" || created.Type != "agent" {
		t.Fatalf("unexpected created template: %+v", created)
	}

	install, err := service.InstallTemplate(context.Background(), "user_1", "buyer_org", "tpl_1")
	if err != nil {
		t.Fatalf("install template: %v", err)
	}
	if install.TemplateID != "tpl_1" || string(install.TemplateData) != `{"nodes":[{"id":"start"}]}` {
		t.Fatalf("expected installed template data to be returned, got %+v", install)
	}
	if store.templates["tpl_1"].DownloadsCount != 8 {
		t.Fatalf("expected download count increment, got %d", store.templates["tpl_1"].DownloadsCount)
	}
}

func TestServiceRejectsInvalidMarketplaceTemplate(t *testing.T) {
	service := NewService(newMarketplaceServiceStore(), nil)

	_, err := service.CreateTemplate(context.Background(), "org_1", TemplateCreateRequest{
		Type:         "bot",
		Name:         "Bad",
		TemplateData: json.RawMessage(`{"nodes":[]}`),
	})
	if err == nil || !strings.Contains(err.Error(), "agent, workflow, plugin") {
		t.Fatalf("expected legacy bot template type to be rejected against public contract, got %v", err)
	}
}

func TestServiceInstallAgentCreatesUserInstall(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)

	install, err := service.InstallAgent(context.Background(), "user_1", "org_1", "agent_approved", "version_1")
	if err != nil {
		t.Fatalf("InstallAgent returned error: %v", err)
	}
	if install.AgentID != "agent_approved" || install.UserID != "user_1" || install.OrganizationID != "org_1" {
		t.Fatalf("unexpected install: %#v", install)
	}
}

func TestServiceRecordAgentRankingSignalValidatesAndDelegatesEvents(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)
	ctx := context.Background()

	for _, event := range []AgentRankingSignalEvent{
		AgentRankingSignalImpression,
		AgentRankingSignalClick,
		AgentRankingSignalInstallConversion,
		AgentRankingSignalImpression,
	} {
		if err := service.RecordAgentRankingSignal(ctx, "agent_approved", event); err != nil {
			t.Fatalf("RecordAgentRankingSignal(%s) returned error: %v", event, err)
		}
	}

	signals := store.rankingSignals["agent_approved"]
	if signals[AgentRankingSignalImpression] != 2 ||
		signals[AgentRankingSignalClick] != 1 ||
		signals[AgentRankingSignalInstallConversion] != 1 {
		t.Fatalf("unexpected ranking signal counters: %+v", signals)
	}
	if err := service.RecordAgentRankingSignal(ctx, "", AgentRankingSignalImpression); err == nil {
		t.Fatal("expected blank agent ID to be rejected")
	}
	if err := service.RecordAgentRankingSignal(ctx, "agent_approved", AgentRankingSignalEvent("share")); err == nil {
		t.Fatal("expected unsupported ranking signal event to be rejected")
	}
}

func TestServiceSubmitReviewCreatesAndUpdatesInstalledUserReview(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)
	if _, err := service.InstallAgent(context.Background(), "user_1", "org_1", "agent_approved", "version_1"); err != nil {
		t.Fatalf("InstallAgent returned error: %v", err)
	}

	review, err := service.SubmitReview(context.Background(), "user_1", "org_1", "Reviewer", ReviewInput{
		AgentID: "agent_approved",
		Rating:  5,
		Body:    "Excellent release assistant.",
	})
	if err != nil {
		t.Fatalf("SubmitReview create returned error: %v", err)
	}
	if review.Rating != 5 {
		t.Fatalf("expected initial rating 5, got %d", review.Rating)
	}

	updated, err := service.SubmitReview(context.Background(), "user_1", "org_1", "Reviewer", ReviewInput{
		AgentID: "agent_approved",
		Rating:  4,
		Body:    "Still useful after another pass.",
	})
	if err != nil {
		t.Fatalf("SubmitReview update returned error: %v", err)
	}
	if updated.Rating != 4 {
		t.Fatalf("expected updated rating 4, got %d", updated.Rating)
	}
}

func TestServiceListUserAgentsClampsMyAgentsPagination(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)

	agents, err := service.ListUserAgents(context.Background(), "owner_1", "org_1", 999, 7)
	if err != nil {
		t.Fatalf("ListUserAgents returned error: %v", err)
	}
	if len(agents) != 1 || agents[0].OwnerID != "owner_1" {
		t.Fatalf("unexpected my-agents result: %#v", agents)
	}
	if store.lastListLimit != 20 || store.lastListOffset != 7 {
		t.Fatalf("expected clamped limit=20 offset=7, got limit=%d offset=%d", store.lastListLimit, store.lastListOffset)
	}
	if store.lastOrgID != "org_1" || agents[0].OrganizationID != "org_1" {
		t.Fatalf("expected org_1 to be used for my-agents, store=%q agent=%q", store.lastOrgID, agents[0].OrganizationID)
	}
}

func TestServiceSettlementPreferencesDefaultToMonthly(t *testing.T) {
	service := NewService(newMarketplaceServiceStore(), nil)

	prefs, err := service.GetPublisherSettlementPreferences(context.Background(), "publisher_org")
	if err != nil {
		t.Fatalf("GetPublisherSettlementPreferences returned error: %v", err)
	}
	if prefs.Cycle != "monthly" || prefs.PayoutBusinessDays != 5 || prefs.ProcessingFeePercent != 1 {
		t.Fatalf("expected monthly default with 5 business days and 1%% fee, got %+v", prefs)
	}
	if prefs.MinimumPayoutAmount != 100 || prefs.EffectiveFrom != "next_settlement_cycle" {
		t.Fatalf("expected minimum payout and next-cycle effect metadata, got %+v", prefs)
	}
}

func TestServiceUpdatesSettlementPreferencesForNextCycle(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)

	prefs, err := service.UpdatePublisherSettlementPreferences(context.Background(), "publisher_org", "quarterly")
	if err != nil {
		t.Fatalf("UpdatePublisherSettlementPreferences returned error: %v", err)
	}
	if prefs.Cycle != "quarterly" || prefs.PayoutBusinessDays != 5 || prefs.ProcessingFeePercent != 0.5 {
		t.Fatalf("expected quarterly payout preferences, got %+v", prefs)
	}
	stored := store.settlementPrefs["publisher_org"]
	if stored.Cycle != "quarterly" {
		t.Fatalf("expected store to persist quarterly cycle, got %+v", stored)
	}
}

func TestServiceRejectsInvalidSettlementCycle(t *testing.T) {
	service := NewService(newMarketplaceServiceStore(), nil)

	_, err := service.UpdatePublisherSettlementPreferences(context.Background(), "publisher_org", "daily")
	if err == nil {
		t.Fatal("expected invalid settlement cycle to be rejected")
	}
}

func TestServiceListPendingReviewsAddsReviewSLAForStandardAndVIPPublishers(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	standardSubmittedAt := now.Add(-70 * time.Hour)
	vipSubmittedAt := now.Add(-25 * time.Hour)
	store := newMarketplaceServiceStore()
	store.pendingReviews = []*PublishedAgent{
		{
			ID:             "agent_standard",
			OrganizationID: "org_standard",
			Status:         "pending_review",
			CreatedAt:      standardSubmittedAt,
			UpdatedAt:      standardSubmittedAt,
		},
		{
			ID:                  "agent_vip",
			OrganizationID:      "org_vip",
			Status:              "pending_review",
			CreatedAt:           vipSubmittedAt,
			UpdatedAt:           vipSubmittedAt,
			PublisherReviewTier: "vip",
		},
	}
	service := NewService(store, nil, WithReviewSLAClock(func() time.Time { return now }))

	reviews, err := service.ListPendingReviews(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("ListPendingReviews returned error: %v", err)
	}
	if len(reviews) != 2 {
		t.Fatalf("expected 2 pending reviews, got %d", len(reviews))
	}

	standardSLA := reviews[0].ReviewSLA
	if standardSLA == nil {
		t.Fatal("expected standard pending review to include reviewSLA")
	}
	if standardSLA.ManualSlaHours != 72 || !standardSLA.ManualDeadlineAt.Equal(standardSubmittedAt.Add(72*time.Hour)) {
		t.Fatalf("expected 72h standard manual SLA, got %+v", standardSLA)
	}
	if standardSLA.ManualSlaStatus != "due_soon" || standardSLA.MinutesUntilDeadline != 120 || standardSLA.VIPPublisher {
		t.Fatalf("expected standard review due soon with 120 minutes left, got %+v", standardSLA)
	}
	if standardSLA.AutomatedReviewSlaMinutes != 5 || !standardSLA.AutomatedReviewDeadlineAt.Equal(standardSubmittedAt.Add(5*time.Minute)) {
		t.Fatalf("expected 5m automated review SLA metadata, got %+v", standardSLA)
	}

	vipSLA := reviews[1].ReviewSLA
	if vipSLA == nil {
		t.Fatal("expected VIP pending review to include reviewSLA")
	}
	if vipSLA.ManualSlaHours != 24 || !vipSLA.VIPPublisher || vipSLA.PublisherTier != "vip" {
		t.Fatalf("expected VIP review to use 24h SLA with VIP input, got %+v", vipSLA)
	}
	if vipSLA.ManualSlaStatus != "overdue" || vipSLA.MinutesUntilDeadline != -60 {
		t.Fatalf("expected VIP review to be 60 minutes overdue, got %+v", vipSLA)
	}
}

func TestServiceEnforceReviewSLAsAlertsDueSoonAndOverdueOnce(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	store := newMarketplaceServiceStore()
	store.pendingReviews = []*PublishedAgent{
		{
			ID:             "agent_due_soon",
			Name:           "Due Soon Review",
			OrganizationID: "org_standard",
			Status:         AgentStatusPendingReview,
			CreatedAt:      now.Add(-70 * time.Hour),
			UpdatedAt:      now.Add(-70 * time.Hour),
		},
		{
			ID:                  "agent_vip_overdue",
			Name:                "VIP Overdue Review",
			OrganizationID:      "org_vip",
			Status:              AgentStatusPendingReview,
			CreatedAt:           now.Add(-25 * time.Hour),
			UpdatedAt:           now.Add(-25 * time.Hour),
			PublisherReviewTier: "vip",
		},
	}
	alerts := &marketplaceSLACaptureAlertSink{}
	service := NewService(store, nil, WithReviewSLAClock(func() time.Time { return now }), WithReviewSLAAlertSink(alerts))

	result, err := service.EnforceReviewSLAs(context.Background(), ReviewSLAEnforcementOptions{Limit: 20})
	if err != nil {
		t.Fatalf("EnforceReviewSLAs returned error: %v", err)
	}
	if result.Scanned != 2 || result.Alerted != 2 {
		t.Fatalf("expected two scanned and alerted reviews, got %+v", result)
	}
	if len(alerts.events) != 2 {
		t.Fatalf("expected two alert events, got %+v", alerts.events)
	}
	if alerts.events[0].Key != "marketplace_review_sla:agent_due_soon:manual:due_soon" ||
		alerts.events[0].Severity != observability.AlertSeverityWarning ||
		alerts.events[0].Fields["manualSlaStatus"] != "due_soon" ||
		alerts.events[0].Fields["minutesUntilDeadline"] != 120 {
		t.Fatalf("unexpected due-soon SLA alert: %+v", alerts.events[0])
	}
	if alerts.events[1].Key != "marketplace_review_sla:agent_vip_overdue:manual:overdue" ||
		alerts.events[1].Severity != observability.AlertSeverityCritical ||
		alerts.events[1].Fields["manualSlaHours"] != 24 ||
		alerts.events[1].Fields["publisherTier"] != "vip" {
		t.Fatalf("unexpected overdue VIP SLA alert: %+v", alerts.events[1])
	}

	secondResult, err := service.EnforceReviewSLAs(context.Background(), ReviewSLAEnforcementOptions{Limit: 20})
	if err != nil {
		t.Fatalf("second EnforceReviewSLAs returned error: %v", err)
	}
	if secondResult.Scanned != 2 || secondResult.Alerted != 0 {
		t.Fatalf("expected second enforcement to scan but not duplicate alerts, got %+v", secondResult)
	}
	if len(alerts.events) != 2 {
		t.Fatalf("expected no duplicate alert events, got %+v", alerts.events)
	}
}
