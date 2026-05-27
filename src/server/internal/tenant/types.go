package tenant

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusArchived = "archived"
)

type Organization struct {
	ID              string         `json:"id"`
	Slug            string         `json:"slug"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Metadata        map[string]any `json:"metadata"`
	CreatedByUserID *string        `json:"createdByUserID,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	ArchivedAt      *time.Time     `json:"archivedAt,omitempty"`
}

type CreateOrganizationRequest struct {
	Slug            string         `json:"slug"`
	Name            string         `json:"name"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedByUserID *string        `json:"-"`
}

type OrganizationUpdateRequest struct {
	Name     *string        `json:"name,omitempty"`
	Status   *string        `json:"status,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type OrganizationListFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

type OrganizationUpdate struct {
	Name     *string
	Status   *string
	Metadata map[string]any
}
