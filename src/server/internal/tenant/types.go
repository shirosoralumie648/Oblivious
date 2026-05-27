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

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"

	InvitationPending  = "pending"
	InvitationAccepted = "accepted"
	InvitationRevoked  = "revoked"
	InvitationExpired  = "expired"
)

type Actor struct {
	UserID    string
	Email     string
	IPAddress string
}

type Membership struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationID"`
	OrganizationSlug string     `json:"organizationSlug,omitempty"`
	OrganizationName string     `json:"organizationName,omitempty"`
	UserID           string     `json:"userID"`
	UserEmail        string     `json:"userEmail,omitempty"`
	Role             string     `json:"role"`
	CreatedByUserID  *string    `json:"createdByUserID,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	RemovedAt        *time.Time `json:"removedAt,omitempty"`
}

type Invitation struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organizationID"`
	Email            string     `json:"email"`
	Role             string     `json:"role"`
	Token            string     `json:"token,omitempty"`
	TokenHash        string     `json:"-"`
	Status           string     `json:"status"`
	InvitedByUserID  string     `json:"invitedByUserID"`
	AcceptedByUserID *string    `json:"acceptedByUserID,omitempty"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	AcceptedAt       *time.Time `json:"acceptedAt,omitempty"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role"`
}

type TransferOwnershipRequest struct {
	NewOwnerUserID string `json:"newOwnerUserID"`
}

type AuditRecord struct {
	ActorID      string
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   string
	Changes      string
	IPAddress    string
}
