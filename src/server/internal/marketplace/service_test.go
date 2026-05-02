package marketplace

import (
	"context"
	"database/sql"
	"testing"
)

type marketplaceServiceStore struct {
	agents         map[string]*PublishedAgent
	installs       map[string]*AgentInstall
	reviews        map[string]*AgentReview
	lastOwnerID    string
	lastListLimit  int
	lastListOffset int
}

func newMarketplaceServiceStore() *marketplaceServiceStore {
	return &marketplaceServiceStore{
		agents: map[string]*PublishedAgent{
			"agent_approved": {
				ID:         "agent_approved",
				OwnerID:    "owner_1",
				Name:       "Approved Agent",
				Status:     "approved",
				Visibility: "public",
			},
		},
		installs: make(map[string]*AgentInstall),
		reviews:  make(map[string]*AgentReview),
	}
}

func (s *marketplaceServiceStore) CreateAgent(ctx context.Context, ownerID string, input AgentPublishRequest) (*PublishedAgent, error) {
	s.lastOwnerID = ownerID
	agent := &PublishedAgent{
		ID:          "agent_new",
		OwnerID:     ownerID,
		Name:        input.Name,
		Description: input.Description,
		Tools:       input.Tools,
		Visibility:  input.Visibility,
		Status:      "pending_review",
		PricingType: input.PricingType,
	}
	s.agents[agent.ID] = agent
	return agent, nil
}

func (s *marketplaceServiceStore) GetAgent(ctx context.Context, id string) (*PublishedAgent, error) {
	return s.agents[id], nil
}

func (s *marketplaceServiceStore) UpdateAgent(ctx context.Context, id string, input AgentPublishRequest) (*PublishedAgent, error) {
	agent := s.agents[id]
	if agent == nil {
		return nil, nil
	}
	agent.Name = input.Name
	agent.Description = input.Description
	agent.Tools = input.Tools
	agent.Visibility = input.Visibility
	agent.Status = "pending_review"
	return agent, nil
}

func (s *marketplaceServiceStore) DeleteAgent(ctx context.Context, id string) error {
	delete(s.agents, id)
	return nil
}

func (s *marketplaceServiceStore) ListUserAgents(ctx context.Context, ownerID string, limit, offset int) ([]*PublishedAgent, error) {
	s.lastOwnerID = ownerID
	s.lastListLimit = limit
	s.lastListOffset = offset
	return []*PublishedAgent{{ID: "agent_owned", OwnerID: ownerID, Name: "Owned Agent"}}, nil
}

func (s *marketplaceServiceStore) ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error) {
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

func (s *marketplaceServiceStore) CreateVersion(ctx context.Context, agentID string, version, changelog string, metadata string) (*AgentVersion, error) {
	return &AgentVersion{ID: "version_1", AgentID: agentID, Version: version, Changelog: changelog}, nil
}

func (s *marketplaceServiceStore) ListVersions(ctx context.Context, agentID string) ([]*AgentVersion, error) {
	return []*AgentVersion{{ID: "version_1", AgentID: agentID, Version: "1.0.0"}}, nil
}

func (s *marketplaceServiceStore) GetVersion(ctx context.Context, agentID, version string) (*AgentVersion, error) {
	return &AgentVersion{ID: "version_1", AgentID: agentID, Version: version}, nil
}

func (s *marketplaceServiceStore) InstallAgent(ctx context.Context, agentID, userID, versionID string) (*AgentInstall, error) {
	install := &AgentInstall{ID: "install_1", AgentID: agentID, UserID: userID}
	s.installs[agentID+":"+userID] = install
	return install, nil
}

func (s *marketplaceServiceStore) UninstallAgent(ctx context.Context, agentID, userID string) error {
	delete(s.installs, agentID+":"+userID)
	return nil
}

func (s *marketplaceServiceStore) ListUserInstalls(ctx context.Context, userID string) ([]*AgentInstall, error) {
	var installs []*AgentInstall
	for _, install := range s.installs {
		if install.UserID == userID {
			installs = append(installs, install)
		}
	}
	return installs, nil
}

func (s *marketplaceServiceStore) IsInstalled(ctx context.Context, agentID, userID string) (bool, error) {
	_, ok := s.installs[agentID+":"+userID]
	return ok, nil
}

func (s *marketplaceServiceStore) CreateReview(ctx context.Context, userID string, input ReviewInput) (*AgentReview, error) {
	review := &AgentReview{ID: "review_1", AgentID: input.AgentID, UserID: userID, Rating: input.Rating, Body: input.Body}
	s.reviews[input.AgentID+":"+userID] = review
	return review, nil
}

func (s *marketplaceServiceStore) UpdateReview(ctx context.Context, userID string, input ReviewInput) (*AgentReview, error) {
	review := &AgentReview{ID: "review_1", AgentID: input.AgentID, UserID: userID, Rating: input.Rating, Body: input.Body}
	s.reviews[input.AgentID+":"+userID] = review
	return review, nil
}

func (s *marketplaceServiceStore) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*AgentReview, error) {
	return []*AgentReview{{ID: "review_1", AgentID: agentID, Rating: 5}}, nil
}

func (s *marketplaceServiceStore) GetUserReview(ctx context.Context, agentID, userID string) (*AgentReview, error) {
	return s.reviews[agentID+":"+userID], nil
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

type marketplaceAuditRecorder struct {
	actions []string
}

func (a *marketplaceAuditRecorder) LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error {
	a.actions = append(a.actions, action)
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
	service := NewService(store, audit)

	agent, err := service.PublishAgent(context.Background(), "owner_1", "owner@example.com", validAgentPublishRequest(), "127.0.0.1")
	if err != nil {
		t.Fatalf("PublishAgent returned error: %v", err)
	}
	if agent.Status != "pending_review" {
		t.Fatalf("expected pending_review status, got %q", agent.Status)
	}
	if store.lastOwnerID != "owner_1" {
		t.Fatalf("expected owner_1 to be passed to store, got %q", store.lastOwnerID)
	}
	if len(audit.actions) != 1 || audit.actions[0] != "agent.publish" {
		t.Fatalf("expected agent.publish audit action, got %v", audit.actions)
	}
}

func TestServiceInstallAgentCreatesUserInstall(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)

	install, err := service.InstallAgent(context.Background(), "user_1", "agent_approved", "version_1")
	if err != nil {
		t.Fatalf("InstallAgent returned error: %v", err)
	}
	if install.AgentID != "agent_approved" || install.UserID != "user_1" {
		t.Fatalf("unexpected install: %#v", install)
	}
}

func TestServiceSubmitReviewCreatesAndUpdatesInstalledUserReview(t *testing.T) {
	store := newMarketplaceServiceStore()
	service := NewService(store, nil)
	if _, err := service.InstallAgent(context.Background(), "user_1", "agent_approved", "version_1"); err != nil {
		t.Fatalf("InstallAgent returned error: %v", err)
	}

	review, err := service.SubmitReview(context.Background(), "user_1", "Reviewer", ReviewInput{
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

	updated, err := service.SubmitReview(context.Background(), "user_1", "Reviewer", ReviewInput{
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

	agents, err := service.ListUserAgents(context.Background(), "owner_1", 999, 7)
	if err != nil {
		t.Fatalf("ListUserAgents returned error: %v", err)
	}
	if len(agents) != 1 || agents[0].OwnerID != "owner_1" {
		t.Fatalf("unexpected my-agents result: %#v", agents)
	}
	if store.lastListLimit != 20 || store.lastListOffset != 7 {
		t.Fatalf("expected clamped limit=20 offset=7, got limit=%d offset=%d", store.lastListLimit, store.lastListOffset)
	}
}
