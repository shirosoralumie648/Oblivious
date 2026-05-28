package marketplace

import (
	"context"
	"database/sql"
	"testing"
)

func TestGovernanceTakedownPreventsNewInstallsAndPreservesHistory(t *testing.T) {
	database := settlementTestDB(t)
	service := NewGovernanceService(NewSQLStore(database))
	installService := NewService(NewSQLStore(database), nil)

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementUserOrg(t, database, "admin_user", "admin_org")
	insertSettlementAgent(t, database, "agent_governed", "publisher_user", "publisher_org", "free", 0)
	if _, err := installService.InstallAgent(context.Background(), "buyer_user", "buyer_org", "agent_governed", "version_agent_governed"); err != nil {
		t.Fatalf("pre-takedown install: %v", err)
	}

	if err := service.TakedownAgent(context.Background(), GovernanceAction{
		ActorUserID:         "admin_user",
		ActorOrganizationID: "admin_org",
		AgentID:             "agent_governed",
		Reason:              "policy violation",
	}); err != nil {
		t.Fatalf("TakedownAgent returned error: %v", err)
	}

	if _, err := installService.InstallAgent(context.Background(), "buyer_user_2", "buyer_org", "agent_governed", "version_agent_governed"); err == nil {
		t.Fatal("expected takedown agent to reject new installs")
	}

	var installCount, eventCount int
	var status string
	if err := database.QueryRow(`SELECT status FROM published_agents WHERE id = 'agent_governed'`).Scan(&status); err != nil {
		t.Fatalf("query agent status: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_governed'`).Scan(&installCount); err != nil {
		t.Fatalf("count installs: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_governed' AND action = 'takedown'`).Scan(&eventCount); err != nil {
		t.Fatalf("count governance events: %v", err)
	}
	if status != "takedown" || installCount != 1 || eventCount != 1 {
		t.Fatalf("expected takedown preserving one historical install and event, got status=%s installs=%d events=%d", status, installCount, eventCount)
	}
}

func TestGovernanceAppealAndReinstateRecordEvents(t *testing.T) {
	database := settlementTestDB(t)
	service := NewGovernanceService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementUserOrg(t, database, "admin_user", "admin_org")
	insertSettlementAgent(t, database, "agent_appeal", "publisher_user", "publisher_org", "free", 0)
	if err := service.TakedownAgent(context.Background(), GovernanceAction{
		ActorUserID:         "admin_user",
		ActorOrganizationID: "admin_org",
		AgentID:             "agent_appeal",
		Reason:              "policy violation",
	}); err != nil {
		t.Fatalf("takedown: %v", err)
	}

	if err := service.AppealAgent(context.Background(), GovernanceAction{
		ActorUserID:         "publisher_user",
		ActorOrganizationID: "publisher_org",
		AgentID:             "agent_appeal",
		Reason:              "fixed the issue",
	}); err != nil {
		t.Fatalf("AppealAgent returned error: %v", err)
	}
	if err := service.ReinstateAgent(context.Background(), GovernanceAction{
		ActorUserID:         "admin_user",
		ActorOrganizationID: "admin_org",
		AgentID:             "agent_appeal",
		Reason:              "appeal accepted",
	}); err != nil {
		t.Fatalf("ReinstateAgent returned error: %v", err)
	}

	var status string
	var appealCount, reinstateCount int
	if err := database.QueryRow(`SELECT status FROM published_agents WHERE id = 'agent_appeal'`).Scan(&status); err != nil {
		t.Fatalf("query agent status: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_appeal' AND action = 'appeal'`).Scan(&appealCount); err != nil {
		t.Fatalf("count appeal events: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_appeal' AND action = 'reinstate'`).Scan(&reinstateCount); err != nil {
		t.Fatalf("count reinstate events: %v", err)
	}
	if status != "approved" || appealCount != 1 || reinstateCount != 1 {
		t.Fatalf("expected approved with appeal/reinstate events, got status=%s appeal=%d reinstate=%d", status, appealCount, reinstateCount)
	}
}

func TestGovernanceAbuseReportLifecycle(t *testing.T) {
	database := settlementTestDB(t)
	service := NewGovernanceService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "reporter_user", "reporter_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementUserOrg(t, database, "admin_user", "admin_org")
	insertSettlementAgent(t, database, "agent_abuse", "publisher_user", "publisher_org", "free", 0)

	report, err := service.ReportAbuse(context.Background(), AbuseReportRequest{
		ReporterOrganizationID: "reporter_org",
		ReporterUserID:         "reporter_user",
		AgentID:                "agent_abuse",
		Reason:                 "malware",
		Details:                "attempted credential exfiltration",
	})
	if err != nil {
		t.Fatalf("ReportAbuse returned error: %v", err)
	}
	if err := service.ResolveAbuseReport(context.Background(), AbuseResolution{
		ReportID:       report.ID,
		ReviewerUserID: "admin_user",
		Status:         "resolved",
		Resolution:     "agent removed",
	}); err != nil {
		t.Fatalf("ResolveAbuseReport returned error: %v", err)
	}

	var status, resolution string
	var reportEvents, resolveEvents int
	if err := database.QueryRow(`SELECT status, COALESCE(resolution, '') FROM marketplace_abuse_reports WHERE id = $1`, report.ID).Scan(&status, &resolution); err != nil {
		t.Fatalf("query abuse report: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_abuse' AND action = 'abuse_report'`).Scan(&reportEvents); err != nil {
		t.Fatalf("count abuse report events: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_governance_events WHERE agent_id = 'agent_abuse' AND action = 'abuse_resolve'`).Scan(&resolveEvents); err != nil {
		t.Fatalf("count abuse resolve events: %v", err)
	}
	if status != "resolved" || resolution != "agent removed" || reportEvents != 1 || resolveEvents != 1 {
		t.Fatalf("expected resolved abuse report with governance events, got status=%s resolution=%q reportEvents=%d resolveEvents=%d", status, resolution, reportEvents, resolveEvents)
	}
}

func queryAgentStatus(t *testing.T, database *sql.DB, agentID string) string {
	t.Helper()
	var status string
	if err := database.QueryRow(`SELECT status FROM published_agents WHERE id = $1`, agentID).Scan(&status); err != nil {
		t.Fatalf("query agent status: %v", err)
	}
	return status
}
