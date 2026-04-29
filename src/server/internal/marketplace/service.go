package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
)

// AuditLogger is the audit callback interface for logging review actions.
// The admin package's Service.CreateAuditEntry satisfies this interface.
type AuditLogger interface {
	LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error
}

// Service provides marketplace business logic on top of Store.
type Service struct {
	store Store
	audit AuditLogger
}

// NewService creates a new marketplace Service.
func NewService(store Store, audit AuditLogger) *Service {
	return &Service{store: store, audit: audit}
}

// --- Agent Publishing ---

// PublishAgent validates and creates a new published agent (D-17, D-18).
func (s *Service) PublishAgent(ctx context.Context, userID, userEmail string, input AgentPublishRequest, ip string) (*PublishedAgent, error) {
	// Validate required fields
	if err := validatePublishRequest(input); err != nil {
		return nil, fmt.Errorf("publish agent: %w", err)
	}

	// Verify category exists if provided
	if input.CategoryID != "" {
		cat, err := s.store.GetCategoryBySlug(ctx, input.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("publish agent: %w", err)
		}
		if cat == nil {
			return nil, fmt.Errorf("publish agent: category %q not found", input.CategoryID)
		}
	}

	agent, err := s.store.CreateAgent(ctx, userID, input)
	if err != nil {
		return nil, fmt.Errorf("publish agent: %w", err)
	}

	// Audit
	if s.audit != nil {
		if err := s.audit.LogAction(ctx, userID, userEmail, "agent.publish", "agent", agent.ID, toJSON(input), ip); err != nil {
			// Audit failure is non-fatal
			_ = err
		}
	}

	return agent, nil
}

// validatePublishRequest validates all required fields for agent publication.
func validatePublishRequest(input AgentPublishRequest) error {
	if len(input.Name) < 3 || len(input.Name) > 100 {
		return fmt.Errorf("name must be 3-100 characters")
	}
	if len(input.Description) < 10 || len(input.Description) > 2000 {
		return fmt.Errorf("description must be 10-2000 characters")
	}
	if input.Tools == "" {
		return fmt.Errorf("tools is required")
	}
	if !json.Valid([]byte(input.Tools)) {
		return fmt.Errorf("tools must be valid JSON")
	}
	if input.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Validate pricing type
	switch input.PricingType {
	case "free":
		// Free is always valid
	case "one_time", "subscription":
		if input.PricingAmount <= 0 {
			return fmt.Errorf("pricing amount must be greater than 0 for %s", input.PricingType)
		}
	default:
		return fmt.Errorf("pricing_type must be one of: free, one_time, subscription")
	}

	// Validate visibility
	if input.Visibility != "" {
		switch input.Visibility {
		case "public", "private", "unlisted":
			// Valid
		default:
			return fmt.Errorf("visibility must be one of: public, private, unlisted")
		}
	}

	return nil
}

// GetAgent retrieves a published agent by ID.
func (s *Service) GetAgent(ctx context.Context, id string) (*PublishedAgent, error) {
	if id == "" {
		return nil, fmt.Errorf("get agent: id is required")
	}
	return s.store.GetAgent(ctx, id)
}

// UpdateAgent updates an agent (owner only). Status resets to pending_review (D-19).
func (s *Service) UpdateAgent(ctx context.Context, userID, userEmail, id string, input AgentPublishRequest, ip string) (*PublishedAgent, error) {
	if id == "" {
		return nil, fmt.Errorf("update agent: id is required")
	}

	// Verify ownership
	existing, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("update agent: agent not found")
	}
	if existing.OwnerID != userID {
		return nil, fmt.Errorf("update agent: only the owner can update this agent")
	}

	// Apply same validation as publish
	if input.Name != "" || input.Description != "" || input.Tools != "" {
		if err := validatePublishRequest(input); err != nil {
			return nil, fmt.Errorf("update agent: %w", err)
		}
	}

	agent, err := s.store.UpdateAgent(ctx, id, input)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}

	// Audit
	if s.audit != nil {
		if err := s.audit.LogAction(ctx, userID, userEmail, "agent.update", "agent", id, toJSON(input), ip); err != nil {
			_ = err
		}
	}

	return agent, nil
}

// DeleteAgent deletes an agent (owner only).
func (s *Service) DeleteAgent(ctx context.Context, userID, id string) error {
	// Verify ownership
	existing, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("delete agent: agent not found")
	}
	if existing.OwnerID != userID {
		return fmt.Errorf("delete agent: only the owner can delete this agent")
	}
	return s.store.DeleteAgent(ctx, id)
}

// ListUserAgents lists all agents owned by a user.
func (s *Service) ListUserAgents(ctx context.Context, userID string, limit, offset int) ([]*PublishedAgent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.store.ListUserAgents(ctx, userID, limit, offset)
}

// --- Review Queue (D-17, D-24) ---

// ListPendingReviews returns the review queue ordered by submission time (oldest first).
func (s *Service) ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.store.ListPendingReviews(ctx, limit, offset)
}

// ApproveAgent approves a pending agent (D-17).
func (s *Service) ApproveAgent(ctx context.Context, reviewerID, reviewerEmail, agentID, ip string) (*PublishedAgent, error) {
	if agentID == "" {
		return nil, fmt.Errorf("approve agent: agentID is required")
	}

	if err := s.store.ApproveAgent(ctx, agentID, reviewerID); err != nil {
		return nil, fmt.Errorf("approve agent: %w", err)
	}

	// Audit
	if s.audit != nil {
		if err := s.audit.LogAction(ctx, reviewerID, reviewerEmail, "agent.approve", "agent", agentID, "", ip); err != nil {
			_ = err
		}
	}

	return s.store.GetAgent(ctx, agentID)
}

// RejectAgent rejects a pending agent with a required reason (D-17).
func (s *Service) RejectAgent(ctx context.Context, reviewerID, reviewerEmail, agentID, reason, ip string) (*PublishedAgent, error) {
	if agentID == "" {
		return nil, fmt.Errorf("reject agent: agentID is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("reject agent: reason is required")
	}

	if err := s.store.RejectAgent(ctx, agentID, reviewerID, reason); err != nil {
		return nil, fmt.Errorf("reject agent: %w", err)
	}

	// Audit
	if s.audit != nil {
		if err := s.audit.LogAction(ctx, reviewerID, reviewerEmail, "agent.reject", "agent", agentID, reason, ip); err != nil {
			_ = err
		}
	}

	return s.store.GetAgent(ctx, agentID)
}

// --- Installs (D-20) ---

// InstallAgent installs an approved, public agent for a user (idempotent, one-click).
func (s *Service) InstallAgent(ctx context.Context, userID, agentID, versionID string) (*AgentInstall, error) {
	if agentID == "" {
		return nil, fmt.Errorf("install agent: agentID is required")
	}

	// Verify agent is approved and public
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("install agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("install agent: agent not found")
	}
	if agent.Status != "approved" {
		return nil, fmt.Errorf("install agent: only approved agents can be installed")
	}
	if agent.Visibility != "public" {
		return nil, fmt.Errorf("install agent: only public agents can be installed")
	}

	return s.store.InstallAgent(ctx, agentID, userID, versionID)
}

// UninstallAgent removes an agent install.
func (s *Service) UninstallAgent(ctx context.Context, userID, agentID string) error {
	return s.store.UninstallAgent(ctx, agentID, userID)
}

// ListUserInstalls lists all agents installed by a user.
func (s *Service) ListUserInstalls(ctx context.Context, userID string) ([]*AgentInstall, error) {
	return s.store.ListUserInstalls(ctx, userID)
}

// --- Reviews (D-27) ---

// SubmitReview submits or updates a review. Only users who installed the agent can review.
func (s *Service) SubmitReview(ctx context.Context, userID, userName string, input ReviewInput) (*AgentReview, error) {
	if input.AgentID == "" {
		return nil, fmt.Errorf("submit review: agentID is required")
	}
	if input.Rating < 1 || input.Rating > 5 {
		return nil, fmt.Errorf("submit review: rating must be 1-5")
	}

	// Verify agent exists and is approved
	agent, err := s.store.GetAgent(ctx, input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("submit review: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("submit review: agent not found")
	}
	if agent.Status != "approved" {
		return nil, fmt.Errorf("submit review: can only review approved agents")
	}

	// Verify user has installed this agent
	installed, err := s.store.IsInstalled(ctx, input.AgentID, userID)
	if err != nil {
		return nil, fmt.Errorf("submit review: %w", err)
	}
	if !installed {
		return nil, fmt.Errorf("submit review: must install the agent before reviewing")
	}

	// Check for existing review (upsert)
	existing, err := s.store.GetUserReview(ctx, input.AgentID, userID)
	if err != nil {
		return nil, fmt.Errorf("submit review: %w", err)
	}
	if existing != nil {
		return s.store.UpdateReview(ctx, userID, input)
	}
	return s.store.CreateReview(ctx, userID, input)
}

// ListReviews lists reviews for an agent.
func (s *Service) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*AgentReview, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.store.ListReviews(ctx, agentID, limit, offset)
}

// --- Categories (D-28) ---

// ListCategories lists all categories with agent counts.
func (s *Service) ListCategories(ctx context.Context) ([]*Category, error) {
	return s.store.ListCategories(ctx)
}

// GetCategory retrieves a category by slug.
func (s *Service) GetCategory(ctx context.Context, slug string) (*Category, error) {
	return s.store.GetCategoryBySlug(ctx, slug)
}

// --- Versions (D-19) ---

// GetAgentVersion retrieves a specific agent version.
func (s *Service) GetAgentVersion(ctx context.Context, agentID, version string) (*AgentVersion, error) {
	return s.store.GetVersion(ctx, agentID, version)
}

// ListAgentVersions lists all versions for an agent.
func (s *Service) ListAgentVersions(ctx context.Context, agentID string) ([]*AgentVersion, error) {
	return s.store.ListVersions(ctx, agentID)
}

// toJSON marshals a value to a JSON string for audit logging.
func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
