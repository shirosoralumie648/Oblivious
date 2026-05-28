package marketplace

import (
	"context"
	"fmt"
)

// PublisherStats holds aggregate statistics for a publisher's agents (D-23).
type PublisherStats struct {
	TotalAgents             int          `json:"totalAgents"`
	TotalInstalls           int          `json:"totalInstalls"`
	ActiveUsers             int          `json:"activeUsers"`
	TotalAPICalls           int          `json:"totalAPICalls"`
	GrossRevenue            float64      `json:"grossRevenue"`
	PlatformFees            float64      `json:"platformFees"`
	NetRevenue              float64      `json:"netRevenue"`
	RefundedAmount          float64      `json:"refundedAmount"`
	PendingSettlementAmount float64      `json:"pendingSettlementAmount"`
	AvailableAmount         float64      `json:"availableAmount"`
	PayoutPendingAmount     float64      `json:"payoutPendingAmount"`
	PaidOutAmount           float64      `json:"paidOutAmount"`
	PerAgentStats           []AgentStats `json:"perAgentStats"`
}

// AgentStats holds per-agent statistics for a publisher (D-23).
type AgentStats struct {
	AgentID      string `json:"agentID"`
	AgentName    string `json:"agentName"`
	InstallCount int    `json:"installCount"`
	ActiveUsers  int    `json:"activeUsers"`
	APICallCount int    `json:"apiCallCount"`
}

// GetPublisherStats returns aggregate analytics for all agents owned by a publisher (D-23).
func (s *Service) GetPublisherStats(ctx context.Context, ownerID, organizationID string) (*PublisherStats, error) {
	db := s.store.GetDB()
	stats := &PublisherStats{}

	// Total agents
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM published_agents WHERE owner_id = $1 AND organization_id = $2
	`, ownerID, organizationID).Scan(&stats.TotalAgents)
	if err != nil {
		return nil, fmt.Errorf("get publisher stats: total agents: %w", err)
	}

	// Total installs
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(install_count), 0) FROM published_agents WHERE owner_id = $1 AND organization_id = $2
	`, ownerID, organizationID).Scan(&stats.TotalInstalls)
	if err != nil {
		return nil, fmt.Errorf("get publisher stats: total installs: %w", err)
	}

	// Active users (distinct users who installed any agent by this publisher)
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT ai.user_id)
		FROM agent_installs ai
		JOIN published_agents pa ON ai.agent_id = pa.id
		WHERE pa.owner_id = $1 AND pa.organization_id = $2
	`, ownerID, organizationID).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("get publisher stats: active users: %w", err)
	}

	// Total API calls: requires schema extension (usage_records.agent_id column).
	// For now, default to 0. See deferred items.
	stats.TotalAPICalls = 0

	if err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(gross_amount), 0),
			COALESCE(SUM(platform_fee_amount), 0),
			COALESCE(SUM(publisher_net_amount), 0),
			COALESCE(SUM(refunded_amount), 0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN publisher_net_amount - refunded_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'available' THEN publisher_net_amount - refunded_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'payout_pending' THEN publisher_net_amount - refunded_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paid_out' THEN publisher_net_amount - refunded_amount ELSE 0 END), 0)
		FROM marketplace_settlements
		WHERE publisher_user_id = $1 AND publisher_organization_id = $2
	`, ownerID, organizationID).Scan(&stats.GrossRevenue, &stats.PlatformFees, &stats.NetRevenue,
		&stats.RefundedAmount, &stats.PendingSettlementAmount, &stats.AvailableAmount,
		&stats.PayoutPendingAmount, &stats.PaidOutAmount); err != nil {
		return nil, fmt.Errorf("get publisher stats: settlement amounts: %w", err)
	}

	// Per-agent stats
	rows, err := db.QueryContext(ctx, `
		SELECT pa.id, pa.name, pa.install_count,
		       (SELECT COUNT(DISTINCT ai2.user_id)
		        FROM agent_installs ai2
		        WHERE ai2.agent_id = pa.id)
		FROM published_agents pa
		WHERE pa.owner_id = $1 AND pa.organization_id = $2
		ORDER BY pa.install_count DESC
	`, ownerID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("get publisher stats: per agent: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var as AgentStats
		if err := rows.Scan(&as.AgentID, &as.AgentName, &as.InstallCount, &as.ActiveUsers); err != nil {
			return nil, fmt.Errorf("get publisher stats: scan agent: %w", err)
		}
		// APICallCount: requires schema extension. Default 0.
		as.APICallCount = 0
		stats.PerAgentStats = append(stats.PerAgentStats, as)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get publisher stats: rows: %w", err)
	}

	// Aggregate total API calls
	for _, as := range stats.PerAgentStats {
		stats.TotalAPICalls += as.APICallCount
	}

	return stats, nil
}

// GetAgentStats returns analytics for a single agent (owner-gated at handler layer).
func (s *Service) GetAgentStats(ctx context.Context, agentID, organizationID string) (*AgentStats, error) {
	db := s.store.GetDB()
	var as AgentStats

	err := db.QueryRowContext(ctx, `
		SELECT pa.id, pa.name, pa.install_count,
		       (SELECT COUNT(DISTINCT ai.user_id)
		        FROM agent_installs ai
		        WHERE ai.agent_id = pa.id)
		FROM published_agents pa
		WHERE pa.id = $1 AND pa.organization_id = $2
	`, agentID, organizationID).Scan(&as.AgentID, &as.AgentName, &as.InstallCount, &as.ActiveUsers)
	if err != nil {
		return nil, fmt.Errorf("get agent stats: %w", err)
	}

	// APICallCount: requires schema extension (usage_records.agent_id column).
	// For now, default to 0.
	as.APICallCount = 0

	return &as, nil
}
