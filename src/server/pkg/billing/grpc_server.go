package billing

import (
	"context"
	"fmt"
	"log"

	billingpb "oblivious/server/internal/grpc/billingv1"
)

type Store interface {
	RecordUsage(ctx context.Context, orgID, userID, resourceType string, amount int64, metadata map[string]string) error
	GetQuota(ctx context.Context, orgID, userID, resourceType string) (total, used int64, err error)
}

type Server struct {
	billingpb.UnimplementedBillingServiceServer
	store  Store
	logger *log.Logger
}

func NewServer(store Store, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		store:  store,
		logger: logger,
	}
}

func (s *Server) RecordUsage(ctx context.Context, req *billingpb.RecordUsageRequest) (*billingpb.RecordUsageResponse, error) {
	if req.OrganizationId == "" {
		return &billingpb.RecordUsageResponse{Success: false, Message: "organization_id is required"}, nil
	}
	if req.ResourceType == "" {
		return &billingpb.RecordUsageResponse{Success: false, Message: "resource_type is required"}, nil
	}
	if req.Amount <= 0 {
		return &billingpb.RecordUsageResponse{Success: false, Message: "amount must be positive"}, nil
	}

	if err := s.store.RecordUsage(ctx, req.OrganizationId, req.UserId, req.ResourceType, req.Amount, req.Metadata); err != nil {
		s.logger.Printf("failed to record usage: %v", err)
		return &billingpb.RecordUsageResponse{Success: false, Message: fmt.Sprintf("failed to record usage: %v", err)}, nil
	}

	return &billingpb.RecordUsageResponse{Success: true, Message: "usage recorded"}, nil
}

func (s *Server) GetQuota(ctx context.Context, req *billingpb.GetQuotaRequest) (*billingpb.GetQuotaResponse, error) {
	if req.OrganizationId == "" {
		return &billingpb.GetQuotaResponse{}, fmt.Errorf("organization_id is required")
	}
	if req.ResourceType == "" {
		return &billingpb.GetQuotaResponse{}, fmt.Errorf("resource_type is required")
	}

	total, used, err := s.store.GetQuota(ctx, req.OrganizationId, req.UserId, req.ResourceType)
	if err != nil {
		s.logger.Printf("failed to get quota: %v", err)
		return &billingpb.GetQuotaResponse{}, fmt.Errorf("failed to get quota: %v", err)
	}

	remaining := total - used
	if remaining < 0 {
		remaining = 0
	}

	return &billingpb.GetQuotaResponse{
		Total:     total,
		Used:      used,
		Remaining: remaining,
	}, nil
}
