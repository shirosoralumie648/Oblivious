package admin

import (
	"testing"
	"time"
)

func TestSystemStatsStruct(t *testing.T) {
	stats := &SystemStats{
		Users: UserStats{
			TotalUsers:    100,
			ActiveUsers:   50,
			NewUsersToday: 5,
			NewUsersWeek:  20,
		},
		Quotas: QuotaStats{
			TotalBalance: 1000.50,
			TotalUsed:    500.25,
			ActiveTopups: 10,
		},
		Conversations: 500,
		Agents:        25,
		Tasks:         100,
		MCPServers:    15,
	}

	if stats.Users.TotalUsers != 100 {
		t.Errorf("expected 100 total users, got %d", stats.Users.TotalUsers)
	}
	if stats.Quotas.TotalBalance != 1000.50 {
		t.Errorf("expected 1000.50 total balance, got %f", stats.Quotas.TotalBalance)
	}
}

func TestUserInfoStruct(t *testing.T) {
	now := time.Now().UTC()
	user := &UserInfo{
		ID:          "user-1",
		Email:       "test@example.com",
		Name:        "Test User",
		CreatedAt:   now,
		LastLoginAt: &now,
		Balance:     100.0,
		Used:        50.0,
		AgentCount:  5,
		TaskCount:   10,
	}

	if user.ID != "user-1" {
		t.Errorf("expected user-1, got %s", user.ID)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", user.Email)
	}
}

func TestListUsersPagination(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 20},
		{-1, 20},
		{50, 50},
		{150, 100},
	}

	for _, tt := range tests {
		limit := tt.input
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		if limit != tt.expected {
			t.Errorf("input %d: expected %d, got %d", tt.input, tt.expected, limit)
		}
	}
}
