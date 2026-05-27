package tenant

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type Store interface {
	ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error)
	GetOrganization(ctx context.Context, id string) (*Organization, error)
	CreateOrganization(ctx context.Context, organization *Organization) (*Organization, error)
	UpdateOrganization(ctx context.Context, id string, input OrganizationUpdate) (*Organization, error)
	ArchiveOrganization(ctx context.Context, id string) (*Organization, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListOrganizations(ctx context.Context, filter OrganizationListFilter) ([]*Organization, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.ListOrganizations(ctx, filter)
}

func (s *Service) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}
	return s.store.GetOrganization(ctx, id)
}

func (s *Service) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (*Organization, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return nil, err
	}
	slug, err := normalizeSlug(req.Slug)
	if err != nil {
		return nil, err
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}

	return s.store.CreateOrganization(ctx, &Organization{
		ID:              uuid.New().String(),
		Slug:            slug,
		Name:            name,
		Status:          StatusActive,
		Metadata:        metadata,
		CreatedByUserID: req.CreatedByUserID,
	})
}

func (s *Service) UpdateOrganization(ctx context.Context, id string, req OrganizationUpdateRequest) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}

	input := OrganizationUpdate{Metadata: req.Metadata}
	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return nil, err
		}
		input.Name = &name
	}
	if req.Status != nil {
		status, err := normalizeStatus(*req.Status)
		if err != nil {
			return nil, err
		}
		input.Status = &status
	}

	return s.store.UpdateOrganization(ctx, id, input)
}

func (s *Service) ArchiveOrganization(ctx context.Context, id string) (*Organization, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("organization id is required")
	}
	return s.store.ArchiveOrganization(ctx, id)
}

func normalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("organization name is required")
	}
	return name, nil
}

func normalizeSlug(value string) (string, error) {
	slug := strings.TrimSpace(value)
	if !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("organization slug must match %s", slugPattern.String())
	}
	return slug, nil
}

func normalizeStatus(value string) (string, error) {
	status := strings.TrimSpace(value)
	switch status {
	case StatusActive, StatusDisabled, StatusArchived:
		return status, nil
	default:
		return "", errors.New("organization status must be active, disabled, or archived")
	}
}
