package marketplace

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type GovernanceService struct {
	store *SQLStore
}

func NewGovernanceService(store *SQLStore) *GovernanceService {
	return &GovernanceService{store: store}
}

func (s *GovernanceService) TakedownAgent(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("takedown agent: actor, agent, and reason are required")
	}
	return s.updateAgentGovernanceStatus(ctx, action, "takedown", "takedown")
}

func (s *GovernanceService) AppealAgent(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.ActorOrganizationID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("appeal agent: actor organization, agent, and reason are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("appeal agent: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus, ownerOrganizationID string
	if err := tx.QueryRowContext(ctx, `
		SELECT status, organization_id FROM published_agents WHERE id = $1 FOR UPDATE
	`, action.AgentID).Scan(&currentStatus, &ownerOrganizationID); err != nil {
		return fmt.Errorf("appeal agent: load agent: %w", err)
	}
	if ownerOrganizationID != action.ActorOrganizationID {
		return fmt.Errorf("appeal agent: only the publisher organization can appeal")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, 'appeal', $5, $5, $6, '{}', $7)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, action.Reason, time.Now().UTC()); err != nil {
		return fmt.Errorf("appeal agent: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) ReinstateAgent(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("reinstate agent: actor, agent, and reason are required")
	}
	return s.updateAgentGovernanceStatus(ctx, action, "reinstate", "approved")
}

func (s *GovernanceService) ReportAbuse(ctx context.Context, req AbuseReportRequest) (*AbuseReport, error) {
	if req.ReporterOrganizationID == "" || req.ReporterUserID == "" || req.AgentID == "" || req.Reason == "" {
		return nil, fmt.Errorf("report abuse: reporter, agent, and reason are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("report abuse: begin tx: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM published_agents WHERE id = $1)`, req.AgentID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("report abuse: check agent: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("report abuse: agent not found")
	}

	now := time.Now().UTC()
	reportID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_abuse_reports (
			id, reporter_organization_id, reporter_user_id, agent_id, reason,
			details, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), 'open', $7, $7)
	`, reportID, req.ReporterOrganizationID, req.ReporterUserID, req.AgentID, req.Reason, req.Details, now); err != nil {
		return nil, fmt.Errorf("report abuse: insert report: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, 'abuse_report', NULL, 'open', $5, '{}', $6)
	`, uuid.New().String(), req.ReporterUserID, req.ReporterOrganizationID, req.AgentID, req.Reason, now); err != nil {
		return nil, fmt.Errorf("report abuse: insert event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("report abuse: commit: %w", err)
	}
	return s.loadAbuseReport(ctx, reportID)
}

func (s *GovernanceService) ResolveAbuseReport(ctx context.Context, resolution AbuseResolution) error {
	if resolution.ReportID == "" || resolution.ReviewerUserID == "" || resolution.Resolution == "" {
		return fmt.Errorf("resolve abuse report: report, reviewer, and resolution are required")
	}
	if resolution.Status != "resolved" && resolution.Status != "dismissed" {
		return fmt.Errorf("resolve abuse report: status must be resolved or dismissed")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("resolve abuse report: begin tx: %w", err)
	}
	defer tx.Rollback()

	var agentID, currentStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT agent_id, status FROM marketplace_abuse_reports WHERE id = $1 FOR UPDATE
	`, resolution.ReportID).Scan(&agentID, &currentStatus); err != nil {
		return fmt.Errorf("resolve abuse report: load report: %w", err)
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_abuse_reports
		SET status = $2, resolution = $3, reviewer_user_id = $4,
		    updated_at = $5, resolved_at = $5
		WHERE id = $1
	`, resolution.ReportID, resolution.Status, resolution.Resolution, resolution.ReviewerUserID, now); err != nil {
		return fmt.Errorf("resolve abuse report: update report: %w", err)
	}
	action := "abuse_resolve"
	if resolution.Status == "dismissed" {
		action = "abuse_dismiss"
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, agent_id, action, from_status, to_status,
			reason, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '{}', $8)
	`, uuid.New().String(), resolution.ReviewerUserID, agentID, action, currentStatus, resolution.Status, resolution.Resolution, now); err != nil {
		return fmt.Errorf("resolve abuse report: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) updateAgentGovernanceStatus(ctx context.Context, action GovernanceAction, eventAction string, nextStatus string) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s agent: begin tx: %w", eventAction, err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, action.AgentID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("%s agent: load agent: %w", eventAction, err)
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, review_reason = NULLIF($3, ''), updated_at = $4
		WHERE id = $1
	`, action.AgentID, nextStatus, action.Reason, now); err != nil {
		return fmt.Errorf("%s agent: update status: %w", eventAction, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, '{}', $9)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, eventAction, currentStatus, nextStatus, action.Reason, now); err != nil {
		return fmt.Errorf("%s agent: insert event: %w", eventAction, err)
	}
	return tx.Commit()
}

func (s *GovernanceService) loadAbuseReport(ctx context.Context, reportID string) (*AbuseReport, error) {
	var report AbuseReport
	var details, resolution, reviewer sql.NullString
	if err := s.store.db.QueryRowContext(ctx, `
		SELECT id, reporter_organization_id, reporter_user_id, agent_id, reason,
		       details, status, resolution, reviewer_user_id, created_at, updated_at
		FROM marketplace_abuse_reports WHERE id = $1
	`, reportID).Scan(&report.ID, &report.ReporterOrganizationID, &report.ReporterUserID, &report.AgentID,
		&report.Reason, &details, &report.Status, &resolution, &reviewer, &report.CreatedAt, &report.UpdatedAt); err != nil {
		return nil, fmt.Errorf("load abuse report: %w", err)
	}
	report.Details = details.String
	report.Resolution = resolution.String
	report.ReviewerUserID = reviewer.String
	return &report, nil
}
