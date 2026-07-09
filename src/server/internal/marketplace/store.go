package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Store defines all marketplace data operations.
type Store interface {
	// Agent CRUD
	CreateAgent(ctx context.Context, ownerID, organizationID string, input AgentPublishRequest) (*PublishedAgent, error)
	GetAgent(ctx context.Context, id string) (*PublishedAgent, error)
	UpdateAgent(ctx context.Context, id, organizationID string, input AgentPublishRequest) (*PublishedAgent, error)
	DeleteAgent(ctx context.Context, id, organizationID string) error
	ListUserAgents(ctx context.Context, ownerID, organizationID string, limit, offset int) ([]*PublishedAgent, error)

	// Review queue (D-17, D-24)
	ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error)
	ListReviewQueue(ctx context.Context, status string, limit, offset int) ([]*PublishedAgent, error)
	ApproveAgent(ctx context.Context, id, reviewerID string) error
	RejectAgent(ctx context.Context, id, reviewerID, reason string) error

	// Version management (D-19)
	CreateVersion(ctx context.Context, agentID, organizationID string, version, changelog string, metadata string) (*AgentVersion, error)
	ListVersions(ctx context.Context, agentID string) ([]*AgentVersion, error)
	GetVersion(ctx context.Context, agentID, version string) (*AgentVersion, error)

	// Installs (D-20)
	InstallAgent(ctx context.Context, agentID, userID, organizationID, versionID string) (*AgentInstall, error)
	UninstallAgent(ctx context.Context, agentID, userID, organizationID string) error
	ListUserInstalls(ctx context.Context, userID, organizationID string) ([]*AgentInstall, error)
	IsInstalled(ctx context.Context, agentID, userID, organizationID string) (bool, error)
	RecordAgentRankingSignal(ctx context.Context, agentID string, event AgentRankingSignalEvent) error

	// Reviews (D-27)
	CreateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error)
	UpdateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error)
	ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*AgentReview, error)
	GetUserReview(ctx context.Context, agentID, userID, organizationID string) (*AgentReview, error)

	// Categories (D-28)
	ListCategories(ctx context.Context) ([]*Category, error)
	GetCategoryByID(ctx context.Context, id string) (*Category, error)
	GetCategoryBySlug(ctx context.Context, slug string) (*Category, error)

	// Templates
	CreateTemplate(ctx context.Context, organizationID string, input TemplateCreateRequest) (*MarketplaceTemplate, error)
	ListTemplates(ctx context.Context, filter TemplateFilter) ([]*MarketplaceTemplate, int, error)
	GetTemplate(ctx context.Context, id string) (*MarketplaceTemplate, error)
	InstallTemplate(ctx context.Context, templateID, userID, organizationID string) (*TemplateInstall, error)

	// Publisher settlement preferences
	GetPublisherSettlementPreferences(ctx context.Context, organizationID string) (*MarketplaceSettlementPreferences, error)
	UpdatePublisherSettlementPreferences(ctx context.Context, organizationID string, cycle string) (*MarketplaceSettlementPreferences, error)

	// Tags
	SetAgentTags(ctx context.Context, agentID string, tags []string) error

	// DB access for analytics queries
	GetDB() *sql.DB
}

// SQLStore implements Store using database/sql.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new SQLStore.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// GetDB returns the underlying *sql.DB for analytics queries.
func (s *SQLStore) GetDB() *sql.DB { return s.db }

// GetPublisherSettlementPreferences reads publisher settlement preferences from organization metadata.
func (s *SQLStore) GetPublisherSettlementPreferences(ctx context.Context, organizationID string) (*MarketplaceSettlementPreferences, error) {
	var cycle sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT metadata #>> '{marketplace,settlement,cycle}'
		FROM organizations
		WHERE id = $1
	`, organizationID).Scan(&cycle); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get publisher settlement preferences: %w", err)
	}
	return &MarketplaceSettlementPreferences{Cycle: cycle.String}, nil
}

// UpdatePublisherSettlementPreferences stores the next-cycle settlement preference in organization metadata.
func (s *SQLStore) UpdatePublisherSettlementPreferences(ctx context.Context, organizationID string, cycle string) (*MarketplaceSettlementPreferences, error) {
	now := time.Now().UTC()
	var storedCycle string
	if err := s.db.QueryRowContext(ctx, `
		UPDATE organizations
		SET metadata = COALESCE(metadata, '{}'::jsonb) ||
			jsonb_build_object(
				'marketplace',
				COALESCE(metadata->'marketplace', '{}'::jsonb) ||
				jsonb_build_object(
					'settlement',
					COALESCE(metadata #> '{marketplace,settlement}', '{}'::jsonb) ||
					jsonb_build_object(
						'cycle', $2::text,
						'effectiveFrom', 'next_settlement_cycle',
						'updatedAt', $3::timestamptz
					)
				)
			),
		    updated_at = $3
		WHERE id = $1
		RETURNING metadata #>> '{marketplace,settlement,cycle}'
	`, organizationID, cycle, now).Scan(&storedCycle); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, fmt.Errorf("update publisher settlement preferences: %w", err)
	}
	return &MarketplaceSettlementPreferences{Cycle: storedCycle}, nil
}

// selectAgentColumns returns the standard SELECT column list for published_agents with JOINs.
const selectAgentColumns = `a.id, a.organization_id, a.owner_id, COALESCE(u.name, ''), a.name, a.description,
	a.icon_url, a.category_id, COALESCE(c.name, ''), a.tags, COALESCE(a.tools, '{}'::jsonb)::text,
	COALESCE(a.example_conversations, '[]'::jsonb)::text, a.system_prompt, a.visibility, a.status,
	a.review_reason, a.pricing_type, a.pricing_amount, a.install_count,
	a.rating_avg, a.rating_count, a.created_at, a.updated_at`

// scanAgent scans a single PublishedAgent from a row.Scanner.
func scanAgent(scanner interface {
	Scan(dest ...interface{}) error
}) (*PublishedAgent, error) {
	var a PublishedAgent
	var iconURL, categoryID, categoryName, systemPrompt, reviewReason sql.NullString
	var reviewedAt sql.NullTime

	if err := scanner.Scan(
		&a.ID, &a.OrganizationID, &a.OwnerID, &a.OwnerName, &a.Name, &a.Description,
		&iconURL, &categoryID, &categoryName, pq.Array(&a.Tags), &a.Tools,
		&a.ExampleConversations, &systemPrompt, &a.Visibility, &a.Status,
		&reviewReason, &a.PricingType, &a.PricingAmount, &a.InstallCount,
		&a.RatingAvg, &a.RatingCount, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}

	a.IconURL = iconURL.String
	a.CategoryID = categoryID.String
	a.CategoryName = categoryName.String
	a.SystemPrompt = systemPrompt.String
	a.ReviewReason = reviewReason.String
	if reviewedAt.Valid {
		a.UpdatedAt = reviewedAt.Time
	}

	return &a, nil
}

// scanAgents scans all rows into a slice of PublishedAgent.
func scanAgents(rows *sql.Rows) ([]*PublishedAgent, error) {
	defer rows.Close()
	var agents []*PublishedAgent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// insertVersion inserts a new agent version row.
func (s *SQLStore) insertVersion(ctx context.Context, agentID, organizationID, version, changelog, metadata string) error {
	id := uuid.New().String()
	metaVal := metadata
	if metaVal == "" {
		metaVal = "{}"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending_review')`,
		id, agentID, organizationID, version, changelog, metaVal)
	return err
}

// syncAgentTags replaces all tags for an agent in the agent_tags junction table.
func (s *SQLStore) syncAgentTags(ctx context.Context, agentID string, tags []string) error {
	if len(tags) == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM agent_tags WHERE agent_id = $1`, agentID)
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sync agent tags: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_tags WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("sync agent tags: delete: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO agent_tags (agent_id, tag) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		return fmt.Errorf("sync agent tags: prepare: %w", err)
	}
	defer stmt.Close()
	for _, tag := range tags {
		if _, err := stmt.ExecContext(ctx, agentID, tag); err != nil {
			return fmt.Errorf("sync agent tags: insert %q: %w", tag, err)
		}
	}
	return tx.Commit()
}

// --- Agent CRUD ---

// CreateAgent creates a new published agent with pending_review status.
func (s *SQLStore) CreateAgent(ctx context.Context, ownerID, organizationID string, input AgentPublishRequest) (*PublishedAgent, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	visibility := input.Visibility
	if visibility == "" {
		visibility = "private"
	}

	agent, err := scanAgent(s.db.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO published_agents (id, organization_id, owner_id, name, description, icon_url,
				category_id, tags, tools, example_conversations, system_prompt,
				visibility, status, pricing_type, pricing_amount,
				install_count, rating_avg, rating_count, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::text[], $9, $10, $11,
				$12, 'pending_review', $13, $14, 0, 0, 0, $15, $16)
			RETURNING *
		)
		SELECT `+selectAgentColumns+`
		FROM inserted a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
	`, id, organizationID, ownerID, input.Name, input.Description, nullIfEmpty(input.IconURL),
		nullIfEmpty(input.CategoryID), pq.Array(input.Tags), input.Tools,
		input.ExampleConversations, nullIfEmpty(input.SystemPrompt),
		visibility, input.PricingType, input.PricingAmount, now, now,
	))
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	// Sync tags to agent_tags junction table
	if len(input.Tags) > 0 {
		if err := s.syncAgentTags(ctx, id, input.Tags); err != nil {
			return nil, fmt.Errorf("create agent: %w", err)
		}
	}

	// Create initial version if provided
	if input.Version != "" {
		changelog := input.Changelog
		metadata := "{}"
		if err := s.insertVersion(ctx, id, organizationID, input.Version, changelog, metadata); err != nil {
			return nil, fmt.Errorf("create agent: insert version: %w", err)
		}
	}

	return agent, nil
}

// GetAgent retrieves a published agent by ID with owner and category names.
func (s *SQLStore) GetAgent(ctx context.Context, id string) (*PublishedAgent, error) {
	a, err := scanAgent(s.db.QueryRowContext(ctx, `
		SELECT `+selectAgentColumns+`
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
		WHERE a.id = $1
	`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return a, nil
}

// UpdateAgent updates an existing agent. Status resets to pending_review.
func (s *SQLStore) UpdateAgent(ctx context.Context, id, organizationID string, input AgentPublishRequest) (*PublishedAgent, error) {
	now := time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		UPDATE published_agents SET
			name = COALESCE(NULLIF($1, ''), name),
			description = COALESCE(NULLIF($2, ''), description),
			icon_url = COALESCE($3, icon_url),
			category_id = COALESCE(NULLIF($4, ''), category_id),
			tools = COALESCE(NULLIF($5, ''), tools),
			example_conversations = COALESCE(NULLIF($6, ''), example_conversations),
			system_prompt = COALESCE($7, system_prompt),
			visibility = COALESCE(NULLIF($8, ''), visibility),
			pricing_type = COALESCE(NULLIF($9, ''), pricing_type),
			pricing_amount = COALESCE(NULLIF($10::decimal, 0::decimal)::decimal, pricing_amount),
			status = 'pending_review',
			review_reason = NULL,
			updated_at = $11
		WHERE id = $12 AND organization_id = $13
	`, input.Name, input.Description, nullIfEmpty(input.IconURL),
		nullIfEmpty(input.CategoryID), input.Tools, input.ExampleConversations,
		nullIfEmpty(input.SystemPrompt), input.Visibility,
		input.PricingType, input.PricingAmount, now, id, organizationID)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return nil, fmt.Errorf("update agent: agent not found")
	}

	// Sync tags
	if len(input.Tags) > 0 {
		if err := s.syncAgentTags(ctx, id, input.Tags); err != nil {
			return nil, fmt.Errorf("update agent: tags: %w", err)
		}
		// Also update the tags array column
		_, err = s.db.ExecContext(ctx, `UPDATE published_agents SET tags = $1::text[] WHERE id = $2 AND organization_id = $3`, pq.Array(input.Tags), id, organizationID)
		if err != nil {
			return nil, fmt.Errorf("update agent: tags array: %w", err)
		}
	}

	// Create new version if provided
	if input.Version != "" {
		changelog := input.Changelog
		if err := s.insertVersion(ctx, id, organizationID, input.Version, changelog, "{}"); err != nil {
			return nil, fmt.Errorf("update agent: insert version: %w", err)
		}
	}

	return s.GetAgent(ctx, id)
}

// DeleteAgent deletes a published agent only before marketplace order evidence exists.
func (s *SQLStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	var hasOrderEvidence bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM marketplace_orders
			WHERE agent_id = $1 AND publisher_organization_id = $2
		)
	`, id, organizationID).Scan(&hasOrderEvidence); err != nil {
		return fmt.Errorf("delete agent: check marketplace order audit evidence: %w", err)
	}
	if hasOrderEvidence {
		return fmt.Errorf("delete agent: marketplace order audit evidence exists; archive or takedown instead")
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM published_agents WHERE id = $1 AND organization_id = $2`, id, organizationID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

// ListUserAgents lists all agents owned by a user.
func (s *SQLStore) ListUserAgents(ctx context.Context, ownerID, organizationID string, limit, offset int) ([]*PublishedAgent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+selectAgentColumns+`
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
		WHERE a.owner_id = $1 AND a.organization_id = $2
		ORDER BY a.created_at DESC
		LIMIT $3 OFFSET $4
	`, ownerID, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user agents: %w", err)
	}
	return scanAgents(rows)
}

// --- Review Queue ---

// ListPendingReviews lists agents awaiting review, oldest first (fair queue).
func (s *SQLStore) ListPendingReviews(ctx context.Context, limit, offset int) ([]*PublishedAgent, error) {
	return s.ListReviewQueue(ctx, AgentStatusPendingReview, limit, offset)
}

// ListReviewQueue lists agents in an operator review queue status, oldest first.
func (s *SQLStore) ListReviewQueue(ctx context.Context, status string, limit, offset int) ([]*PublishedAgent, error) {
	status = normalizeReviewQueueStatus(status)
	if status == "" {
		return []*PublishedAgent{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+selectAgentColumns+`
		FROM published_agents a
		LEFT JOIN categories c ON a.category_id = c.id
		LEFT JOIN users u ON a.owner_id = u.id
		WHERE a.status = $1
		ORDER BY a.created_at ASC
		LIMIT $2 OFFSET $3
	`, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	agents, err := scanAgents(rows)
	if err != nil {
		return nil, err
	}
	if err := s.attachReviewQueueMetadata(ctx, agents); err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	if err := s.attachReviewAssignments(ctx, agents); err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	now := time.Now().UTC()
	for _, agent := range agents {
		AddReviewSLA(agent, now)
	}
	return agents, nil
}

func normalizeReviewQueueStatus(status string) string {
	status = strings.TrimSpace(status)
	switch status {
	case "", "pending":
		return AgentStatusPendingReview
	case AgentStatusPendingReview, AgentStatusAppealPending:
		return status
	default:
		return ""
	}
}

func (s *SQLStore) attachReviewQueueMetadata(ctx context.Context, agents []*PublishedAgent) error {
	if len(agents) == 0 {
		return nil
	}
	organizationIDs := make([]string, 0, len(agents))
	seen := map[string]struct{}{}
	for _, agent := range agents {
		if agent == nil || agent.OrganizationID == "" {
			continue
		}
		if _, ok := seen[agent.OrganizationID]; ok {
			continue
		}
		seen[agent.OrganizationID] = struct{}{}
		organizationIDs = append(organizationIDs, agent.OrganizationID)
	}
	if len(organizationIDs) == 0 {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, metadata
		FROM organizations
		WHERE id = ANY($1::text[])
	`, pq.Array(organizationIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	tierByOrgID := map[string]string{}
	for rows.Next() {
		var orgID string
		var metadataRaw []byte
		if err := rows.Scan(&orgID, &metadataRaw); err != nil {
			return err
		}
		tierByOrgID[orgID] = publisherReviewTierFromMetadata(metadataRaw)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, agent := range agents {
		if agent == nil || agent.PublisherReviewTier != "" {
			continue
		}
		agent.PublisherReviewTier = tierByOrgID[agent.OrganizationID]
	}
	return nil
}

func (s *SQLStore) attachReviewAssignments(ctx context.Context, agents []*PublishedAgent) error {
	if len(agents) == 0 {
		return nil
	}
	agentIDs := make([]string, 0, len(agents))
	agentByID := map[string]*PublishedAgent{}
	for _, agent := range agents {
		if agent == nil || agent.ID == "" {
			continue
		}
		agentIDs = append(agentIDs, agent.ID)
		agentByID[agent.ID] = agent
	}
	if len(agentIDs) == 0 {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT ON (agent_id) agent_id, COALESCE(actor_user_id, '')
		FROM marketplace_governance_events
		WHERE action = 'review_assign' AND agent_id = ANY($1::text[])
		ORDER BY agent_id, created_at DESC
	`, pq.Array(agentIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var agentID, reviewerUserID string
		if err := rows.Scan(&agentID, &reviewerUserID); err != nil {
			return err
		}
		if agent := agentByID[agentID]; agent != nil {
			agent.ReviewerUserID = reviewerUserID
		}
	}
	return rows.Err()
}

func publisherReviewTierFromMetadata(metadataRaw []byte) string {
	if len(metadataRaw) == 0 {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return ""
	}
	for _, key := range []string{"marketplaceReviewTier", "reviewTier", "publisherTier", "tier", "plan", "planId", "planID"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		tier := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
		switch tier {
		case "vip", "priority", "enterprise":
			return tier
		}
	}
	if value, ok := metadata["vipPublisher"].(bool); ok && value {
		return "vip"
	}
	if value, ok := metadata["isVIP"].(bool); ok && value {
		return "vip"
	}
	return ""
}

// ApproveAgent approves a pending agent.
func (s *SQLStore) ApproveAgent(ctx context.Context, id, reviewerID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE published_agents
		SET status = 'approved', reviewed_at = NOW(), review_reason = NULL
		WHERE id = $1 AND status = 'pending_review'
	`, id)
	if err != nil {
		return fmt.Errorf("approve agent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("approve agent: agent not in pending_review state")
	}

	// Also approve pending versions
	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_versions SET status = 'approved'
		WHERE agent_id = $1 AND status = 'pending_review'
	`, id)
	if err != nil {
		return fmt.Errorf("approve agent: update versions: %w", err)
	}

	return nil
}

// RejectAgent rejects a pending agent with a required reason.
func (s *SQLStore) RejectAgent(ctx context.Context, id, reviewerID, reason string) error {
	if reason == "" {
		return fmt.Errorf("reject agent: reason is required")
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE published_agents
		SET status = 'rejected', reviewed_at = NOW(), review_reason = $1
		WHERE id = $2 AND status = 'pending_review'
	`, reason, id)
	if err != nil {
		return fmt.Errorf("reject agent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("reject agent: agent not in pending_review state")
	}

	// Also reject pending versions
	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_versions SET status = 'rejected'
		WHERE agent_id = $1 AND status = 'pending_review'
	`, id)
	if err != nil {
		return fmt.Errorf("reject agent: update versions: %w", err)
	}

	return nil
}

// RequestAgentChanges asks a publisher to supplement a pending submission.
func (s *SQLStore) RequestAgentChanges(ctx context.Context, id, reviewerID, reason string) error {
	if reason == "" {
		return fmt.Errorf("request agent changes: reason is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("request agent changes: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus); err != nil {
		return fmt.Errorf("request agent changes: load agent: %w", err)
	}
	if currentStatus != AgentStatusPendingReview {
		return fmt.Errorf("request agent changes: agent not in pending_review state")
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, reviewed_at = $4, review_reason = $3, updated_at = $4
		WHERE id = $1 AND status = $5
	`, id, AgentStatusNeedsChanges, reason, now, AgentStatusPendingReview)
	if err != nil {
		return fmt.Errorf("request agent changes: update agent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request agent changes: agent not in pending_review state")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_versions SET status = $2
		WHERE agent_id = $1 AND status = $3
	`, id, AgentStatusNeedsChanges, AgentStatusPendingReview); err != nil {
		return fmt.Errorf("request agent changes: update versions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, NULLIF($2, ''), $3, 'needs_changes', $4, $5, $6, '{}', $7)
	`, uuid.New().String(), reviewerID, id, currentStatus, AgentStatusNeedsChanges, reason, now); err != nil {
		return fmt.Errorf("request agent changes: insert event: %w", err)
	}
	return tx.Commit()
}

// --- Version Management ---

// CreateVersion creates a new version for an agent.
func (s *SQLStore) CreateVersion(ctx context.Context, agentID, organizationID string, version, changelog, metadata string) (*AgentVersion, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	metaVal := metadata
	if metaVal == "" {
		metaVal = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending_review', $7)
		ON CONFLICT (agent_id, version) DO NOTHING
	`, id, agentID, organizationID, version, changelog, metaVal, now)
	if err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	// Fetch the created (or existing) version
	av, err := s.GetVersion(ctx, agentID, version)
	if err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}
	if av == nil {
		return nil, fmt.Errorf("create version: version %q already exists for agent %q", version, agentID)
	}
	return av, nil
}

// ListVersions lists all versions for an agent.
func (s *SQLStore) ListVersions(ctx context.Context, agentID string) ([]*AgentVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, organization_id, version, COALESCE(changelog, ''),
		       status, created_at
		FROM agent_versions
		WHERE agent_id = $1
		ORDER BY created_at DESC
	`, agentID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	var versions []*AgentVersion
	for rows.Next() {
		var v AgentVersion
		if err := rows.Scan(&v.ID, &v.AgentID, &v.OrganizationID, &v.Version, &v.Changelog,
			&v.Status, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("list versions: scan: %w", err)
		}
		versions = append(versions, &v)
	}
	return versions, rows.Err()
}

// GetVersion retrieves a specific version of an agent.
func (s *SQLStore) GetVersion(ctx context.Context, agentID, version string) (*AgentVersion, error) {
	var v AgentVersion
	err := s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, organization_id, version, COALESCE(changelog, ''),
		       status, created_at
		FROM agent_versions
		WHERE agent_id = $1 AND version = $2
	`, agentID, version).Scan(&v.ID, &v.AgentID, &v.OrganizationID, &v.Version, &v.Changelog,
		&v.Status, &v.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get version: %w", err)
	}
	return &v, nil
}

// --- Installs ---

// InstallAgent installs an agent for a user (idempotent, one-click).
func (s *SQLStore) InstallAgent(ctx context.Context, agentID, userID, organizationID, versionID string) (*AgentInstall, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	verID := versionID
	if verID == "" {
		// Get latest approved version
		var latestID sql.NullString
		err := s.db.QueryRowContext(ctx, `
			SELECT id FROM agent_versions
			WHERE agent_id = $1 AND status = 'approved'
			ORDER BY created_at DESC LIMIT 1
		`, agentID).Scan(&latestID)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("install agent: find version: %w", err)
		}
		if latestID.Valid {
			verID = latestID.String
		}
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_installs (id, agent_id, user_id, organization_id, version_id, installed_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		ON CONFLICT (organization_id, agent_id, user_id) DO NOTHING
	`, id, agentID, userID, organizationID, verID, now)
	if err != nil {
		return nil, fmt.Errorf("install agent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		// Only increment install count for new installs
		_, err = s.db.ExecContext(ctx, `
			UPDATE published_agents SET install_count = install_count + 1 WHERE id = $1
		`, agentID)
		if err != nil {
			return nil, fmt.Errorf("install agent: update count: %w", err)
		}
	}

	// Fetch the install record (works whether new or existing)
	var inst AgentInstall
	var instVersionID sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, organization_id, user_id, version_id, installed_at
		FROM agent_installs
		WHERE agent_id = $1 AND user_id = $2 AND organization_id = $3
	`, agentID, userID, organizationID).Scan(&inst.ID, &inst.AgentID, &inst.OrganizationID, &inst.UserID, &instVersionID, &inst.InstalledAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("install agent: install not found after insert")
		}
		return nil, fmt.Errorf("install agent: fetch: %w", err)
	}
	return &inst, nil
}

// RecordAgentRankingSignal increments aggregate recommendation counters for an agent.
func (s *SQLStore) RecordAgentRankingSignal(ctx context.Context, agentID string, event AgentRankingSignalEvent) error {
	impressionDelta := 0
	clickDelta := 0
	installDelta := 0
	switch event {
	case AgentRankingSignalImpression:
		impressionDelta = 1
	case AgentRankingSignalClick:
		clickDelta = 1
	case AgentRankingSignalInstallConversion:
		installDelta = 1
	default:
		return fmt.Errorf("record ranking signal: unsupported event %q", event)
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO marketplace_agent_ranking_signals (
			agent_id, impression_count, click_count, install_conversion_count, updated_at
		)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (agent_id) DO UPDATE SET
			impression_count = marketplace_agent_ranking_signals.impression_count + EXCLUDED.impression_count,
			click_count = marketplace_agent_ranking_signals.click_count + EXCLUDED.click_count,
			install_conversion_count = marketplace_agent_ranking_signals.install_conversion_count + EXCLUDED.install_conversion_count,
			updated_at = NOW()
	`, agentID, impressionDelta, clickDelta, installDelta); err != nil {
		return fmt.Errorf("record ranking signal: %w", err)
	}
	return nil
}

// UninstallAgent removes an agent install and decrements install count.
func (s *SQLStore) UninstallAgent(ctx context.Context, agentID, userID, organizationID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM agent_installs WHERE agent_id = $1 AND user_id = $2 AND organization_id = $3`, agentID, userID, organizationID)
	if err != nil {
		return fmt.Errorf("uninstall agent: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		_, err = s.db.ExecContext(ctx, `
			UPDATE published_agents SET install_count = GREATEST(install_count - 1, 0) WHERE id = $1
		`, agentID)
		if err != nil {
			return fmt.Errorf("uninstall agent: update count: %w", err)
		}
	}
	return nil
}

// ListUserInstalls lists all agents installed by a user.
func (s *SQLStore) ListUserInstalls(ctx context.Context, userID, organizationID string) ([]*AgentInstall, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ai.id, ai.agent_id, ai.organization_id, ai.user_id, ai.version_id, ai.installed_at
		FROM agent_installs ai
		WHERE ai.user_id = $1 AND ai.organization_id = $2
		ORDER BY ai.installed_at DESC
	`, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list user installs: %w", err)
	}
	defer rows.Close()

	var installs []*AgentInstall
	for rows.Next() {
		var inst AgentInstall
		var versionID sql.NullString
		if err := rows.Scan(&inst.ID, &inst.AgentID, &inst.OrganizationID, &inst.UserID, &versionID, &inst.InstalledAt); err != nil {
			return nil, fmt.Errorf("list user installs: scan: %w", err)
		}
		installs = append(installs, &inst)
	}
	return installs, rows.Err()
}

// IsInstalled checks if a user has installed an agent.
func (s *SQLStore) IsInstalled(ctx context.Context, agentID, userID, organizationID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM agent_installs WHERE agent_id = $1 AND user_id = $2 AND organization_id = $3)
	`, agentID, userID, organizationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is installed: %w", err)
	}
	return exists, nil
}

// --- Reviews ---

// CreateReview creates a new review with rating_avg recalculation.
func (s *SQLStore) CreateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_reviews (id, agent_id, user_id, organization_id, rating, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (organization_id, agent_id, user_id) DO NOTHING
	`, id, input.AgentID, userID, organizationID, input.Rating, input.Body, now, now)
	if err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	// Recalculate rating stats
	if err := s.recalcRating(ctx, input.AgentID); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	return s.GetUserReview(ctx, input.AgentID, userID, organizationID)
}

// UpdateReview updates an existing review with rating_avg recalculation.
func (s *SQLStore) UpdateReview(ctx context.Context, userID, organizationID string, input ReviewInput) (*AgentReview, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_reviews
		SET rating = $1, body = $2, updated_at = $3
		WHERE agent_id = $4 AND user_id = $5 AND organization_id = $6
	`, input.Rating, input.Body, now, input.AgentID, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("update review: %w", err)
	}

	// Recalculate rating stats
	if err := s.recalcRating(ctx, input.AgentID); err != nil {
		return nil, fmt.Errorf("update review: %w", err)
	}

	return s.GetUserReview(ctx, input.AgentID, userID, organizationID)
}

// recalcRating recalculates rating_avg and rating_count on published_agents.
func (s *SQLStore) recalcRating(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE published_agents SET
			rating_avg = COALESCE((SELECT AVG(rating)::decimal(3,2) FROM agent_reviews WHERE agent_id = $1), 0),
			rating_count = (SELECT COUNT(*) FROM agent_reviews WHERE agent_id = $1)
		WHERE id = $1
	`, agentID)
	if err != nil {
		return fmt.Errorf("recalc rating: %w", err)
	}
	return nil
}

// ListReviews lists reviews for an agent with user name JOIN.
func (s *SQLStore) ListReviews(ctx context.Context, agentID string, limit, offset int) ([]*AgentReview, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.agent_id, r.user_id, COALESCE(u.name, ''), r.rating,
		       COALESCE(r.body, ''), r.created_at, r.updated_at
		FROM agent_reviews r
		LEFT JOIN users u ON r.user_id = u.id
		WHERE r.agent_id = $1
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3
	`, agentID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	var reviews []*AgentReview
	for rows.Next() {
		var r AgentReview
		if err := rows.Scan(&r.ID, &r.AgentID, &r.UserID, &r.UserName,
			&r.Rating, &r.Body, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list reviews: scan: %w", err)
		}
		reviews = append(reviews, &r)
	}
	return reviews, rows.Err()
}

// GetUserReview retrieves a user's review for an agent.
func (s *SQLStore) GetUserReview(ctx context.Context, agentID, userID, organizationID string) (*AgentReview, error) {
	var r AgentReview
	err := s.db.QueryRowContext(ctx, `
		SELECT r.id, r.agent_id, r.organization_id, r.user_id, COALESCE(u.name, ''), r.rating,
		       COALESCE(r.body, ''), r.created_at, r.updated_at
		FROM agent_reviews r
		LEFT JOIN users u ON r.user_id = u.id
		WHERE r.agent_id = $1 AND r.user_id = $2 AND r.organization_id = $3
	`, agentID, userID, organizationID).Scan(&r.ID, &r.AgentID, &r.OrganizationID, &r.UserID, &r.UserName,
		&r.Rating, &r.Body, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user review: %w", err)
	}
	return &r, nil
}

// --- Categories ---

// ListCategories lists all categories with agent counts (approved + public only).
func (s *SQLStore) ListCategories(ctx context.Context) ([]*Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.slug, c.display_order,
		       COUNT(pa.id) AS agent_count
		FROM categories c
		LEFT JOIN published_agents pa ON c.id = pa.category_id
			AND pa.status = 'approved' AND pa.visibility = 'public'
		GROUP BY c.id
		ORDER BY c.display_order ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []*Category
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.DisplayOrder,
			&cat.AgentCount); err != nil {
			return nil, fmt.Errorf("list categories: scan: %w", err)
		}
		categories = append(categories, &cat)
	}
	return categories, rows.Err()
}

// GetCategoryBySlug retrieves a category by its slug with agent count.
func (s *SQLStore) GetCategoryBySlug(ctx context.Context, slug string) (*Category, error) {
	return s.getCategory(ctx, "c.slug = $1", slug)
}

// GetCategoryByID retrieves a category by its stable ID with agent count.
func (s *SQLStore) GetCategoryByID(ctx context.Context, id string) (*Category, error) {
	return s.getCategory(ctx, "c.id = $1", id)
}

func (s *SQLStore) getCategory(ctx context.Context, predicate string, value string) (*Category, error) {
	var cat Category
	err := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.slug, c.display_order,
		       COUNT(pa.id) AS agent_count
		FROM categories c
		LEFT JOIN published_agents pa ON c.id = pa.category_id
			AND pa.status = 'approved' AND pa.visibility = 'public'
		WHERE `+predicate+`
		GROUP BY c.id
	`, value).Scan(&cat.ID, &cat.Name, &cat.Slug, &cat.DisplayOrder,
		&cat.AgentCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return &cat, nil
}

// --- Templates ---

func (s *SQLStore) CreateTemplate(ctx context.Context, organizationID string, input TemplateCreateRequest) (*MarketplaceTemplate, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	template, err := scanTemplate(s.db.QueryRowContext(ctx, `
		INSERT INTO marketplace_templates (id, organization_id, type, name, description, template_data, category, tags, downloads_count, rating_avg, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8::text[], 0, NULL, $9, $10)
		RETURNING id, organization_id, type, name, COALESCE(description, ''), template_data, COALESCE(category, ''), tags,
			downloads_count, COALESCE(rating_avg, 0), created_at, updated_at
	`, id, organizationID, input.Type, input.Name, nullIfEmpty(input.Description), string(input.TemplateData), nullIfEmpty(input.Category), pq.Array(input.Tags), now, now))
	if err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return template, nil
}

func (s *SQLStore) ListTemplates(ctx context.Context, filter TemplateFilter) ([]*MarketplaceTemplate, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, type, name, COALESCE(description, ''), template_data, COALESCE(category, ''), tags,
			downloads_count, COALESCE(rating_avg, 0), created_at, updated_at,
			COUNT(*) OVER() AS total
		FROM marketplace_templates
		WHERE ($1 = '' OR type = $1)
			AND ($2 = '' OR category = $2)
			AND ($3 = '' OR name ILIKE '%' || $3 || '%' OR COALESCE(description, '') ILIKE '%' || $3 || '%')
			AND (COALESCE(cardinality($4::text[]), 0) = 0 OR tags && $4::text[])
		ORDER BY downloads_count DESC, created_at DESC
		LIMIT $5 OFFSET $6
	`, filter.Type, filter.Category, filter.Query, pq.Array(filter.Tags), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []*MarketplaceTemplate
	total := 0
	for rows.Next() {
		template, rowTotal, err := scanTemplateWithTotal(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("list templates: scan: %w", err)
		}
		templates = append(templates, template)
		total = rowTotal
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

func (s *SQLStore) GetTemplate(ctx context.Context, id string) (*MarketplaceTemplate, error) {
	template, err := scanTemplate(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, type, name, COALESCE(description, ''), template_data, COALESCE(category, ''), tags,
			downloads_count, COALESCE(rating_avg, 0), created_at, updated_at
		FROM marketplace_templates
		WHERE id = $1
	`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get template: %w", err)
	}
	return template, nil
}

func (s *SQLStore) InstallTemplate(ctx context.Context, templateID, userID, organizationID string) (*TemplateInstall, error) {
	template, err := s.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("install template: %w", err)
	}
	if template == nil {
		return nil, fmt.Errorf("install template: template not found")
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE marketplace_templates SET downloads_count = downloads_count + 1 WHERE id = $1`, templateID); err != nil {
		return nil, fmt.Errorf("install template: update count: %w", err)
	}
	return &TemplateInstall{
		ID:             uuid.New().String(),
		TemplateID:     template.ID,
		OrganizationID: organizationID,
		UserID:         userID,
		Type:           template.Type,
		Name:           template.Name,
		TemplateData:   template.TemplateData,
		InstalledAt:    time.Now().UTC(),
	}, nil
}

func scanTemplate(scanner interface {
	Scan(dest ...interface{}) error
}) (*MarketplaceTemplate, error) {
	var template MarketplaceTemplate
	var data []byte
	if err := scanner.Scan(
		&template.ID,
		&template.OrganizationID,
		&template.Type,
		&template.Name,
		&template.Description,
		&data,
		&template.Category,
		pq.Array(&template.Tags),
		&template.DownloadsCount,
		&template.RatingAvg,
		&template.CreatedAt,
		&template.UpdatedAt,
	); err != nil {
		return nil, err
	}
	template.TemplateData = data
	return &template, nil
}

func scanTemplateWithTotal(scanner interface {
	Scan(dest ...interface{}) error
}) (*MarketplaceTemplate, int, error) {
	var template MarketplaceTemplate
	var data []byte
	var total int
	if err := scanner.Scan(
		&template.ID,
		&template.OrganizationID,
		&template.Type,
		&template.Name,
		&template.Description,
		&data,
		&template.Category,
		pq.Array(&template.Tags),
		&template.DownloadsCount,
		&template.RatingAvg,
		&template.CreatedAt,
		&template.UpdatedAt,
		&total,
	); err != nil {
		return nil, 0, err
	}
	template.TemplateData = data
	return &template, total, nil
}

// --- Tags ---

// SetAgentTags replaces all tags for an agent.
func (s *SQLStore) SetAgentTags(ctx context.Context, agentID string, tags []string) error {
	// Update the tags array column
	_, err := s.db.ExecContext(ctx, `UPDATE published_agents SET tags = $1::text[] WHERE id = $2`, pq.Array(tags), agentID)
	if err != nil {
		return fmt.Errorf("set agent tags: update array: %w", err)
	}
	// Sync junction table
	if err := s.syncAgentTags(ctx, agentID, tags); err != nil {
		return fmt.Errorf("set agent tags: %w", err)
	}
	return nil
}

// nullIfEmpty returns nil interface for empty strings (for COALESCE / nullable columns).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Ensure SQLStore implements Store at compile time.
var _ Store = (*SQLStore)(nil)
