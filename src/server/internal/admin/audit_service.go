package admin

import (
	"context"
	"fmt"
)

// ListAuditEntries returns paginated audit entries with filters.
func (s *Service) ListAuditEntries(ctx context.Context, filter AuditFilter) ([]*AuditEntry, int, error) {
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.store.ListAuditEntries(ctx, filter)
}

// CreateAuditEntry creates a new audit entry.
func (s *Service) CreateAuditEntry(ctx context.Context, entry *AuditEntry) error {
	if entry == nil {
		return fmt.Errorf("audit entry is nil")
	}
	return s.store.CreateAuditEntry(ctx, entry)
}
