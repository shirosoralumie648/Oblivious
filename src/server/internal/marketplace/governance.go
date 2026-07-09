package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/notification"
)

type GovernanceService struct {
	store        *SQLStore
	notification *notification.Service
	scanner      ReviewScanner
}

type GovernanceOption func(*GovernanceService)

func WithGovernanceNotifications(service *notification.Service) GovernanceOption {
	return func(s *GovernanceService) {
		s.notification = service
	}
}

func WithGovernanceScanner(scanner ReviewScanner) GovernanceOption {
	return func(s *GovernanceService) {
		s.scanner = scanner
	}
}

func NewGovernanceService(store *SQLStore, opts ...GovernanceOption) *GovernanceService {
	service := &GovernanceService{store: store, scanner: NewStaticReviewScanner()}
	for _, opt := range opts {
		opt(service)
	}
	return service
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
	if currentStatus != AgentStatusTakedown {
		return fmt.Errorf("appeal agent: agent not in takedown state")
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, review_reason = $3, updated_at = $4
		WHERE id = $1 AND status = $5
	`, action.AgentID, AgentStatusAppealPending, action.Reason, now, AgentStatusTakedown)
	if err != nil {
		return fmt.Errorf("appeal agent: update status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("appeal agent: agent not in takedown state")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, $3, $4, 'appeal', $5, $6, $7, '{}', $8)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, AgentStatusAppealPending, action.Reason, now); err != nil {
		return fmt.Errorf("appeal agent: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) ReinstateAgent(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("reinstate agent: actor, agent, and reason are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reinstate agent: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, action.AgentID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("reinstate agent: load agent: %w", err)
	}
	if currentStatus != AgentStatusAppealPending {
		return fmt.Errorf("reinstate agent: agent not in appeal_pending state")
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, review_reason = NULLIF($3, ''), updated_at = $4
		WHERE id = $1 AND status = $5
	`, action.AgentID, AgentStatusApproved, action.Reason, now, AgentStatusAppealPending)
	if err != nil {
		return fmt.Errorf("reinstate agent: update status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("reinstate agent: agent not in appeal_pending state")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'reinstate', $5, $6, $7, '{}', $8)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, AgentStatusApproved, action.Reason, now); err != nil {
		return fmt.Errorf("reinstate agent: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) RejectAppealAgent(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("reject appeal agent: actor, agent, and reason are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reject appeal agent: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, action.AgentID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("reject appeal agent: load agent: %w", err)
	}
	if currentStatus != AgentStatusAppealPending {
		return fmt.Errorf("reject appeal agent: agent not in appeal_pending state")
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, review_reason = $3, updated_at = $4
		WHERE id = $1 AND status = $5
	`, action.AgentID, AgentStatusTakedown, action.Reason, now, AgentStatusAppealPending)
	if err != nil {
		return fmt.Errorf("reject appeal agent: update status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("reject appeal agent: agent not in appeal_pending state")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'appeal_reject', $5, $6, $7, '{}', $8)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, AgentStatusTakedown, action.Reason, now); err != nil {
		return fmt.Errorf("reject appeal agent: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) AssignReview(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" {
		return fmt.Errorf("assign review: actor and agent are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("assign review: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, action.AgentID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("assign review: load agent: %w", err)
	}
	if currentStatus != AgentStatusPendingReview && currentStatus != AgentStatusAppealPending {
		return fmt.Errorf("assign review: agent not in pending review or appeal_pending state")
	}
	var existingReviewerUserID string
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(actor_user_id, '')
		FROM marketplace_governance_events
		WHERE agent_id = $1 AND action = 'review_assign'
		ORDER BY created_at DESC
		LIMIT 1
	`, action.AgentID).Scan(&existingReviewerUserID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("assign review: load reviewer assignment: %w", err)
	}
	if existingReviewerUserID != "" && existingReviewerUserID != action.ActorUserID {
		return fmt.Errorf("assign review: agent already claimed by another reviewer")
	}
	reason := strings.TrimSpace(action.Reason)
	if reason == "" {
		reason = "claimed for review"
	}
	metadata, err := json.Marshal(map[string]string{"reviewerUserId": action.ActorUserID})
	if err != nil {
		return fmt.Errorf("assign review: metadata: %w", err)
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'review_assign', $5, $6, $7, $8::jsonb, $9)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, currentStatus, reason, string(metadata), now); err != nil {
		return fmt.Errorf("assign review: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) RequestAgentChanges(ctx context.Context, action GovernanceAction) error {
	if action.ActorUserID == "" || action.AgentID == "" || action.Reason == "" {
		return fmt.Errorf("request agent changes: actor, agent, and reason are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("request agent changes: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, action.AgentID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("request agent changes: load agent: %w", err)
	}
	if currentStatus != AgentStatusPendingReview {
		return fmt.Errorf("request agent changes: agent not in pending_review state")
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE published_agents
		SET status = $2, reviewed_at = $4, review_reason = $3, updated_at = $4
		WHERE id = $1 AND status = $5
	`, action.AgentID, AgentStatusNeedsChanges, action.Reason, now, AgentStatusPendingReview); err != nil {
		return fmt.Errorf("request agent changes: update agent: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_versions SET status = $2
		WHERE agent_id = $1 AND status = $3
	`, action.AgentID, AgentStatusNeedsChanges, AgentStatusPendingReview); err != nil {
		return fmt.Errorf("request agent changes: update versions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, 'needs_changes', $5, $6, $7, '{}', $8)
	`, uuid.New().String(), action.ActorUserID, action.ActorOrganizationID, action.AgentID, currentStatus, AgentStatusNeedsChanges, action.Reason, now); err != nil {
		return fmt.Errorf("request agent changes: insert event: %w", err)
	}
	return tx.Commit()
}

func (s *GovernanceService) RunAutomatedReview(ctx context.Context, agentID string) (*AutomatedReviewResult, error) {
	if agentID == "" {
		return nil, fmt.Errorf("automated review: agent is required")
	}
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("automated review: load agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("automated review: agent not found")
	}
	if agent.Status != "pending_review" {
		return nil, fmt.Errorf("automated review: agent not in pending_review state")
	}

	scanner := s.scanner
	if scanner == nil {
		scanner = NewStaticReviewScanner()
	}
	result, err := scanner.ScanAgent(ctx, *agent)
	if err != nil {
		return nil, fmt.Errorf("automated review: scan agent: %w", err)
	}
	if result.AgentID == "" {
		result.AgentID = agent.ID
	}
	if result.Scanner == "" {
		result.Scanner = defaultReviewScannerName
	}
	if result.PolicyVersion == "" {
		result.PolicyVersion = defaultReviewPolicyVersion
	}
	if result.PolicyChecksum == "" {
		result.PolicyChecksum = staticReviewPolicyChecksum()
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}
	if result.Decision == "" {
		result.Decision = "pending_manual_review"
	}
	if result.Decision != "pending_manual_review" && result.Decision != "rejected" {
		return nil, fmt.Errorf("automated review: unsupported decision %q", result.Decision)
	}
	if err := s.persistAutomatedReview(ctx, *agent, result); err != nil {
		return nil, err
	}
	return &result, nil
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

	var publisherUserID, agentName string
	if err := tx.QueryRowContext(ctx, `SELECT owner_id, name FROM published_agents WHERE id = $1`, req.AgentID).Scan(&publisherUserID, &agentName); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("report abuse: agent not found")
		}
		return nil, fmt.Errorf("report abuse: check agent: %w", err)
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
	if err := s.notifyAbuseReportOpened(ctx, publisherUserID, agentName, req, reportID); err != nil {
		return nil, err
	}
	return s.loadAbuseReport(ctx, reportID)
}

func (s *GovernanceService) persistAutomatedReview(ctx context.Context, agent PublishedAgent, result AutomatedReviewResult) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("automated review: begin tx: %w", err)
	}
	defer tx.Rollback()

	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM published_agents WHERE id = $1 FOR UPDATE`, agent.ID).Scan(&currentStatus); err != nil {
		return fmt.Errorf("automated review: lock agent: %w", err)
	}
	if currentStatus != "pending_review" {
		return fmt.Errorf("automated review: agent not in pending_review state")
	}

	now := time.Now().UTC()
	action := "automated_review_pass"
	nextStatus := "pending_review"
	reason := "automated review passed; awaiting manual review"
	if result.Decision == "rejected" {
		action = "automated_review_reject"
		nextStatus = "rejected"
		reason = automatedReviewReason(result.Findings)
		if _, err := tx.ExecContext(ctx, `
			UPDATE published_agents
			SET status = 'rejected', review_reason = $2, reviewed_at = $3, updated_at = $3
			WHERE id = $1 AND status = 'pending_review'
		`, agent.ID, reason, now); err != nil {
			return fmt.Errorf("automated review: reject agent: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE agent_versions SET status = 'rejected'
			WHERE agent_id = $1 AND status = 'pending_review'
		`, agent.ID); err != nil {
			return fmt.Errorf("automated review: reject versions: %w", err)
		}
	}

	metadata, err := json.Marshal(map[string]any{
		"agentID":        result.AgentID,
		"decision":       result.Decision,
		"scanner":        result.Scanner,
		"policyVersion":  result.PolicyVersion,
		"policyChecksum": result.PolicyChecksum,
		"findings":       result.Findings,
		"createdAt":      result.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("automated review: encode metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_governance_events (
			id, actor_user_id, actor_organization_id, agent_id, action,
			from_status, to_status, reason, metadata, created_at
		)
		VALUES ($1, NULL, NULL, $2, $3, $4, $5, $6, $7::jsonb, $8)
	`, uuid.New().String(), agent.ID, action, currentStatus, nextStatus, reason, string(metadata), now); err != nil {
		return fmt.Errorf("automated review: insert event: %w", err)
	}
	return tx.Commit()
}

func automatedReviewReason(findings []ReviewFinding) string {
	if len(findings) == 0 {
		return "automated review rejected"
	}
	types := make([]string, 0, len(findings))
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Type == "" || seen[finding.Type] {
			continue
		}
		seen[finding.Type] = true
		types = append(types, finding.Type)
	}
	if len(types) == 0 {
		return "automated review rejected"
	}
	return "automated review rejected: " + strings.Join(types, ", ")
}

func (s *GovernanceService) notifyAbuseReportOpened(ctx context.Context, publisherUserID, agentName string, req AbuseReportRequest, reportID string) error {
	if s.notification == nil {
		return nil
	}
	_, err := s.notification.CreateEvent(ctx, notification.NotificationEvent{
		UserID:    publisherUserID,
		Type:      "warning",
		Category:  "marketplace",
		Title:     "Marketplace report received",
		Message:   fmt.Sprintf("Your published agent %q received a marketplace report for %s.", agentName, req.Reason),
		ActionURL: fmt.Sprintf("/marketplace/agents/%s/reports/%s", req.AgentID, reportID),
		Metadata: map[string]any{
			"event":                  "marketplace.abuse_report.opened",
			"reportID":               reportID,
			"agentID":                req.AgentID,
			"reason":                 req.Reason,
			"reporterOrganizationID": req.ReporterOrganizationID,
		},
	})
	if err != nil {
		return fmt.Errorf("report abuse: notify publisher: %w", err)
	}
	return nil
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

func (s *GovernanceService) ListAbuseReports(ctx context.Context, filter AbuseReportFilter) ([]*AbuseReport, error) {
	if s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("list abuse reports: store is not configured")
	}
	status := strings.TrimSpace(filter.Status)
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, reporter_organization_id, reporter_user_id, agent_id, reason,
		       details, status, resolution, reviewer_user_id, created_at, updated_at
		FROM marketplace_abuse_reports`
	args := []any{}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" WHERE status = $%d", len(args))
	}
	args = append(args, limit, offset)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := s.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list abuse reports: query: %w", err)
	}
	defer rows.Close()

	reports := []*AbuseReport{}
	for rows.Next() {
		var report AbuseReport
		var details, resolution, reviewer sql.NullString
		if err := rows.Scan(&report.ID, &report.ReporterOrganizationID, &report.ReporterUserID, &report.AgentID,
			&report.Reason, &details, &report.Status, &resolution, &reviewer, &report.CreatedAt, &report.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list abuse reports: scan: %w", err)
		}
		report.Details = details.String
		report.Resolution = resolution.String
		report.ReviewerUserID = reviewer.String
		reports = append(reports, &report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list abuse reports: rows: %w", err)
	}
	return reports, nil
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
