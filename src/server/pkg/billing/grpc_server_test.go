package billing

import (
	"context"
	"errors"
	"log"
	"testing"

	billingpb "oblivious/server/internal/grpc/billingv1"
)

type mockStore struct {
	recordUsageErr error
	getQuotaResult struct {
		total int64
		used  int64
		err   error
	}
}

func (m *mockStore) RecordUsage(ctx context.Context, orgID, userID, resourceType string, amount int64, metadata map[string]string) error {
	return m.recordUsageErr
}

func (m *mockStore) GetQuota(ctx context.Context, orgID, userID, resourceType string) (total, used int64, err error) {
	return m.getQuotaResult.total, m.getQuotaResult.used, m.getQuotaResult.err
}

func TestServer_RecordUsage(t *testing.T) {
	tests := []struct {
		name       string
		req        *billingpb.RecordUsageRequest
		storeErr   error
		wantErr    bool
		wantMsg    string
		wantOK     bool
	}{
		{
			name:    "empty organization_id",
			req:     &billingpb.RecordUsageRequest{OrganizationId: "", ResourceType: "api_call", Amount: 1},
			wantErr: false,
			wantMsg: "organization_id is required",
			wantOK:  false,
		},
		{
			name:    "empty resource_type",
			req:     &billingpb.RecordUsageRequest{OrganizationId: "org1", ResourceType: "", Amount: 1},
			wantErr: false,
			wantMsg: "resource_type is required",
			wantOK:  false,
		},
		{
			name:    "zero amount",
			req:     &billingpb.RecordUsageRequest{OrganizationId: "org1", ResourceType: "api_call", Amount: 0},
			wantErr: false,
			wantMsg: "amount must be positive",
			wantOK:  false,
		},
		{
			name:    "negative amount",
			req:     &billingpb.RecordUsageRequest{OrganizationId: "org1", ResourceType: "api_call", Amount: -5},
			wantErr: false,
			wantMsg: "amount must be positive",
			wantOK:  false,
		},
		{
			name:     "store error",
			req:      &billingpb.RecordUsageRequest{OrganizationId: "org1", ResourceType: "api_call", Amount: 10},
			storeErr: errors.New("db error"),
			wantErr:  false,
			wantOK:   false,
		},
		{
			name:    "valid request",
			req:     &billingpb.RecordUsageRequest{OrganizationId: "org1", UserId: "user1", ResourceType: "api_call", Amount: 10, Metadata: map[string]string{"key": "value"}},
			wantErr: false,
			wantMsg: "usage recorded",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockStore{recordUsageErr: tt.storeErr}
			s := NewServer(store, nil)

			resp, err := s.RecordUsage(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp.Success != tt.wantOK {
				t.Errorf("RecordUsage() success = %v, want %v", resp.Success, tt.wantOK)
			}
			if tt.wantMsg != "" && resp.Message != tt.wantMsg {
				t.Errorf("RecordUsage() message = %v, want %v", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestServer_GetQuota(t *testing.T) {
	tests := []struct {
		name        string
		req         *billingpb.GetQuotaRequest
		storeTotal  int64
		storeUsed   int64
		storeErr    error
		wantErr     bool
		wantTotal   int64
		wantUsed    int64
		wantRemain  int64
	}{
		{
			name:    "empty organization_id",
			req:     &billingpb.GetQuotaRequest{OrganizationId: "", ResourceType: "api_call"},
			wantErr: true,
		},
		{
			name:    "empty resource_type",
			req:     &billingpb.GetQuotaRequest{OrganizationId: "org1", ResourceType: ""},
			wantErr: true,
		},
		{
			name:       "store error",
			req:        &billingpb.GetQuotaRequest{OrganizationId: "org1", ResourceType: "api_call"},
			storeErr:   errors.New("db error"),
			wantErr:    true,
		},
		{
			name:       "valid request",
			req:        &billingpb.GetQuotaRequest{OrganizationId: "org1", UserId: "user1", ResourceType: "api_call"},
			storeTotal: 1000,
			storeUsed:  300,
			wantErr:    false,
			wantTotal:  1000,
			wantUsed:   300,
			wantRemain: 700,
		},
		{
			name:       "over quota",
			req:        &billingpb.GetQuotaRequest{OrganizationId: "org1", ResourceType: "api_call"},
			storeTotal: 1000,
			storeUsed:  1200,
			wantErr:    false,
			wantTotal:  1000,
			wantUsed:   1200,
			wantRemain: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockStore{}
			store.getQuotaResult.total = tt.storeTotal
			store.getQuotaResult.used = tt.storeUsed
			store.getQuotaResult.err = tt.storeErr
			s := NewServer(store, nil)

			resp, err := s.GetQuota(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetQuota() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if resp.Total != tt.wantTotal {
					t.Errorf("GetQuota() total = %v, want %v", resp.Total, tt.wantTotal)
				}
				if resp.Used != tt.wantUsed {
					t.Errorf("GetQuota() used = %v, want %v", resp.Used, tt.wantUsed)
				}
				if resp.Remaining != tt.wantRemain {
					t.Errorf("GetQuota() remaining = %v, want %v", resp.Remaining, tt.wantRemain)
				}
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	s := NewServer(nil, nil)
	if s.logger == nil {
		t.Error("NewServer() should set default logger")
	}

	customLogger := log.Default()
	s = NewServer(nil, customLogger)
	if s.logger != customLogger {
		t.Error("NewServer() should use provided logger")
	}
}
