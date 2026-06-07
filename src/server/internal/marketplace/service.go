package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"oblivious/server/internal/observability"
)

// AuditLogger is the audit callback interface for logging review actions.
// The admin package's Service.CreateAuditEntry satisfies this interface.
type AuditLogger interface {
	LogAction(ctx context.Context, actorID, actorEmail, action, resourceType, resourceID, changes, ipAddress string) error
}

type AutomatedReviewer interface {
	RunAutomatedReview(ctx context.Context, agentID string) (*AutomatedReviewResult, error)
}

type ServiceOption func(*Service)

// Service provides marketplace business logic on top of Store.
type Service struct {
	store             Store
	audit             AuditLogger
	automatedReviewer AutomatedReviewer
	reviewSLAClock    func() time.Time
	reviewSLAAlert    observability.AlertSink
	reviewSLAAlerts   map[string]bool
}

// NewService creates a new marketplace Service.
func NewService(store Store, audit AuditLogger, options ...ServiceOption) *Service {
	service := &Service{store: store, audit: audit}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithAutomatedReview(reviewer AutomatedReviewer) ServiceOption {
	return func(service *Service) {
		service.automatedReviewer = reviewer
	}
}

func WithReviewSLAClock(clock func() time.Time) ServiceOption {
	return func(service *Service) {
		service.reviewSLAClock = clock
	}
}

func WithReviewSLAAlertSink(sink observability.AlertSink) ServiceOption {
	return func(service *Service) {
		service.reviewSLAAlert = sink
	}
}

type AutomatedReviewError struct {
	Result AutomatedReviewResult
}

func (e *AutomatedReviewError) Error() string {
	return "automated review rejected marketplace submission"
}

type ReviewSLAEnforcementOptions struct {
	Limit  int
	Offset int
}

type ReviewSLAEnforcementResult struct {
	Scanned int `json:"scanned"`
	Alerted int `json:"alerted"`
}

// --- Agent Publishing ---

// PublishAgent validates and creates a new published agent (D-17, D-18).
func (s *Service) PublishAgent(ctx context.Context, userID, organizationID, userEmail string, input AgentPublishRequest, ip string) (*PublishedAgent, error) {
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

	agent, err := s.store.CreateAgent(ctx, userID, organizationID, input)
	if err != nil {
		return nil, fmt.Errorf("publish agent: %w", err)
	}
	if err := s.runPublicationAutomatedReview(ctx, agent); err != nil {
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

func (s *Service) runPublicationAutomatedReview(ctx context.Context, agent *PublishedAgent) error {
	var result *AutomatedReviewResult
	if s.automatedReviewer != nil {
		review, err := s.automatedReviewer.RunAutomatedReview(ctx, agent.ID)
		if err != nil {
			return err
		}
		result = review
	} else {
		review, err := NewStaticReviewScanner().ScanAgent(ctx, *agent)
		if err != nil {
			return err
		}
		result = &review
	}
	if result == nil {
		return nil
	}
	if result.Decision == "rejected" {
		return &AutomatedReviewError{Result: *result}
	}
	return nil
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
func (s *Service) UpdateAgent(ctx context.Context, userID, organizationID, userEmail, id string, input AgentPublishRequest, ip string) (*PublishedAgent, error) {
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
	if existing.OwnerID != userID || existing.OrganizationID != organizationID {
		return nil, fmt.Errorf("update agent: only the owner can update this agent")
	}

	// Apply same validation as publish
	if input.Name != "" || input.Description != "" || input.Tools != "" {
		if err := validatePublishRequest(input); err != nil {
			return nil, fmt.Errorf("update agent: %w", err)
		}
	}

	agent, err := s.store.UpdateAgent(ctx, id, organizationID, input)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	if err := s.runPublicationAutomatedReview(ctx, agent); err != nil {
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
func (s *Service) DeleteAgent(ctx context.Context, userID, organizationID, id string) error {
	// Verify ownership
	existing, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("delete agent: agent not found")
	}
	if existing.OwnerID != userID || existing.OrganizationID != organizationID {
		return fmt.Errorf("delete agent: only the owner can delete this agent")
	}
	return s.store.DeleteAgent(ctx, id, organizationID)
}

// ListUserAgents lists all agents owned by a user.
func (s *Service) ListUserAgents(ctx context.Context, userID, organizationID string, limit, offset int) ([]*PublishedAgent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.store.ListUserAgents(ctx, userID, organizationID, limit, offset)
}

// --- Review Queue (D-17, D-24) ---

// ListPendingReviews returns the review queue ordered by submission time (oldest first).
func (s *Service) ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	agents, err := s.store.ListPendingReviews(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	s.addReviewSLAs(agents)
	return agents, nil
}

const (
	automatedReviewSLAMinutes = 5
	standardManualSLAHours    = 72
	vipManualSLAHours         = 24
	dueSoonReviewThreshold    = 4 * time.Hour
)

func (s *Service) addReviewSLAs(agents []*PublishedAgent) {
	now := time.Now().UTC()
	if s.reviewSLAClock != nil {
		now = s.reviewSLAClock().UTC()
	}
	for _, agent := range agents {
		AddReviewSLA(agent, now)
	}
}

func (s *Service) EnforceReviewSLAs(ctx context.Context, options ReviewSLAEnforcementOptions) (ReviewSLAEnforcementResult, error) {
	limit := options.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	offset := options.Offset
	if offset < 0 {
		offset = 0
	}
	agents, err := s.store.ListPendingReviews(ctx, limit, offset)
	if err != nil {
		return ReviewSLAEnforcementResult{}, err
	}
	s.addReviewSLAs(agents)

	result := ReviewSLAEnforcementResult{Scanned: len(agents)}
	if s.reviewSLAAlert == nil {
		return result, nil
	}
	if s.reviewSLAAlerts == nil {
		s.reviewSLAAlerts = make(map[string]bool)
	}
	for _, agent := range agents {
		if agent == nil || agent.ReviewSLA == nil {
			continue
		}
		event, ok := reviewSLAAlertEvent(agent)
		if !ok {
			continue
		}
		if s.reviewSLAAlerts[event.Key] {
			continue
		}
		if err := s.reviewSLAAlert.Notify(ctx, event); err != nil {
			return result, err
		}
		s.reviewSLAAlerts[event.Key] = true
		result.Alerted++
	}
	return result, nil
}

func AddReviewSLA(agent *PublishedAgent, now time.Time) {
	if agent == nil || agent.Status != "pending_review" {
		return
	}
	submittedAt := agent.CreatedAt
	if submittedAt.IsZero() {
		submittedAt = agent.UpdatedAt
	}
	if submittedAt.IsZero() {
		submittedAt = now
	}
	submittedAt = submittedAt.UTC()
	now = now.UTC()

	tier, tierSource, isVIP := reviewSLAPublisherTier(agent.PublisherReviewTier)
	manualHours := standardManualSLAHours
	if isVIP {
		manualHours = vipManualSLAHours
	}
	manualDeadline := submittedAt.Add(time.Duration(manualHours) * time.Hour)
	automatedDeadline := submittedAt.Add(time.Duration(automatedReviewSLAMinutes) * time.Minute)
	minutesUntilDeadline := int(math.Ceil(manualDeadline.Sub(now).Minutes()))

	agent.ReviewSLA = &ReviewSLA{
		SubmittedAt:               submittedAt,
		AutomatedReviewDeadlineAt: automatedDeadline,
		AutomatedReviewSlaMinutes: automatedReviewSLAMinutes,
		AutomatedReviewSlaStatus:  slaStatus(automatedDeadline, now, 0),
		ManualDeadlineAt:          manualDeadline,
		ManualSlaHours:            manualHours,
		ManualSlaStatus:           slaStatus(manualDeadline, now, dueSoonReviewThreshold),
		MinutesUntilDeadline:      minutesUntilDeadline,
		VIPPublisher:              isVIP,
		PublisherTier:             tier,
		PublisherTierSource:       tierSource,
	}
}

func reviewSLAPublisherTier(rawTier string) (tier string, source string, isVIP bool) {
	tier = strings.ToLower(strings.TrimSpace(rawTier))
	switch tier {
	case "vip", "priority", "enterprise":
		return tier, "organization_metadata", true
	case "":
		return "standard", "default", false
	default:
		return tier, "organization_metadata", false
	}
}

func slaStatus(deadline time.Time, now time.Time, dueSoonThreshold time.Duration) string {
	if !now.Before(deadline) {
		return "overdue"
	}
	if dueSoonThreshold > 0 && !now.Before(deadline.Add(-dueSoonThreshold)) {
		return "due_soon"
	}
	return "within_sla"
}

func reviewSLAAlertEvent(agent *PublishedAgent) (observability.AlertEvent, bool) {
	if agent == nil || agent.ReviewSLA == nil {
		return observability.AlertEvent{}, false
	}
	status := agent.ReviewSLA.ManualSlaStatus
	if status != "due_soon" && status != "overdue" {
		return observability.AlertEvent{}, false
	}
	severity := observability.AlertSeverityWarning
	if status == "overdue" {
		severity = observability.AlertSeverityCritical
	}
	agentName := strings.TrimSpace(agent.Name)
	if agentName == "" {
		agentName = agent.ID
	}
	return observability.AlertEvent{
		Key:        fmt.Sprintf("marketplace_review_sla:%s:manual:%s", agent.ID, status),
		Severity:   severity,
		Title:      "Marketplace review SLA " + strings.ReplaceAll(status, "_", " "),
		Message:    fmt.Sprintf("Marketplace review for %s is %s; manual deadline %s.", agentName, strings.ReplaceAll(status, "_", " "), agent.ReviewSLA.ManualDeadlineAt.Format(time.RFC3339)),
		Component:  "marketplace.review",
		OccurredAt: time.Now().UTC(),
		Fields: map[string]any{
			"agentID":                agent.ID,
			"agentName":              agentName,
			"organizationID":         agent.OrganizationID,
			"manualDeadlineAt":       agent.ReviewSLA.ManualDeadlineAt.Format(time.RFC3339),
			"manualSlaHours":         agent.ReviewSLA.ManualSlaHours,
			"manualSlaStatus":        agent.ReviewSLA.ManualSlaStatus,
			"minutesUntilDeadline":   agent.ReviewSLA.MinutesUntilDeadline,
			"publisherTier":          agent.ReviewSLA.PublisherTier,
			"publisherTierSource":    agent.ReviewSLA.PublisherTierSource,
			"vipPublisher":           agent.ReviewSLA.VIPPublisher,
			"automatedReviewStatus":  agent.ReviewSLA.AutomatedReviewSlaStatus,
			"automatedReviewMinutes": agent.ReviewSLA.AutomatedReviewSlaMinutes,
		},
	}, true
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
func (s *Service) InstallAgent(ctx context.Context, userID, organizationID, agentID, versionID string) (*AgentInstall, error) {
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

	return s.store.InstallAgent(ctx, agentID, userID, organizationID, versionID)
}

// RecordAgentRankingSignal records one aggregate ranking event for recommendation scoring.
func (s *Service) RecordAgentRankingSignal(ctx context.Context, agentID string, event AgentRankingSignalEvent) error {
	if agentID == "" {
		return fmt.Errorf("record ranking signal: agentID is required")
	}
	switch event {
	case AgentRankingSignalImpression, AgentRankingSignalClick, AgentRankingSignalInstallConversion:
	default:
		return fmt.Errorf("record ranking signal: unsupported event %q", event)
	}
	return s.store.RecordAgentRankingSignal(ctx, agentID, event)
}

// UninstallAgent removes an agent install.
func (s *Service) UninstallAgent(ctx context.Context, userID, organizationID, agentID string) error {
	return s.store.UninstallAgent(ctx, agentID, userID, organizationID)
}

// ListUserInstalls lists all agents installed by a user.
func (s *Service) ListUserInstalls(ctx context.Context, userID, organizationID string) ([]*AgentInstall, error) {
	return s.store.ListUserInstalls(ctx, userID, organizationID)
}

// --- Reviews (D-27) ---

// SubmitReview submits or updates a review. Only users who installed the agent can review.
func (s *Service) SubmitReview(ctx context.Context, userID, organizationID, userName string, input ReviewInput) (*AgentReview, error) {
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
	installed, err := s.store.IsInstalled(ctx, input.AgentID, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("submit review: %w", err)
	}
	if !installed {
		return nil, fmt.Errorf("submit review: must install the agent before reviewing")
	}

	// Check for existing review (upsert)
	existing, err := s.store.GetUserReview(ctx, input.AgentID, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("submit review: %w", err)
	}
	if existing != nil {
		return s.store.UpdateReview(ctx, userID, organizationID, input)
	}
	return s.store.CreateReview(ctx, userID, organizationID, input)
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

// --- Templates ---

func (s *Service) CreateTemplate(ctx context.Context, organizationID string, input TemplateCreateRequest) (*MarketplaceTemplate, error) {
	if err := validateTemplateCreateRequest(input); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return s.store.CreateTemplate(ctx, organizationID, input)
}

func (s *Service) ListTemplates(ctx context.Context, filter TemplateFilter) ([]*MarketplaceTemplate, int, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListTemplates(ctx, filter)
}

func (s *Service) GetTemplate(ctx context.Context, id string) (*MarketplaceTemplate, error) {
	if id == "" {
		return nil, fmt.Errorf("get template: id is required")
	}
	return s.store.GetTemplate(ctx, id)
}

func (s *Service) InstallTemplate(ctx context.Context, userID, organizationID, templateID string) (*TemplateInstall, error) {
	if templateID == "" {
		return nil, fmt.Errorf("install template: templateID is required")
	}
	template, err := s.store.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("install template: %w", err)
	}
	if template == nil {
		return nil, fmt.Errorf("install template: template not found")
	}
	install, err := s.store.InstallTemplate(ctx, templateID, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("install template: %w", err)
	}
	install.Type = template.Type
	install.Name = template.Name
	install.TemplateData = template.TemplateData
	return install, nil
}

func validateTemplateCreateRequest(input TemplateCreateRequest) error {
	switch input.Type {
	case "workflow", "bot", "plugin":
	default:
		return fmt.Errorf("type must be one of: workflow, bot, plugin")
	}
	if len(input.Name) < 3 || len(input.Name) > 200 {
		return fmt.Errorf("name must be 3-200 characters")
	}
	if len(input.Description) > 2000 {
		return fmt.Errorf("description must be 2000 characters or fewer")
	}
	if len(input.TemplateData) == 0 || !json.Valid(input.TemplateData) {
		return fmt.Errorf("templateData must be valid JSON")
	}
	return nil
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
