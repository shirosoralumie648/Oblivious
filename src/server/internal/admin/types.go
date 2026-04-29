package admin

import "time"

// ChannelInfo represents a relay channel in the admin list (D-02, D-03).
type ChannelInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	BaseURL   string    `json:"baseURL"`
	Models    []string  `json:"models"`
	RPM       int       `json:"rpm"`
	TPM       int       `json:"tpm"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	Status    string    `json:"status"`  // "online"|"degraded"|"offline"
	Latency   int64     `json:"latency"` // milliseconds
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ChannelCreateRequest is the input for creating a new channel (D-02).
type ChannelCreateRequest struct {
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	BaseURL  string   `json:"baseURL"`
	APIKey   string   `json:"apiKey"`
	Models   []string `json:"models"`
	RpmLimit int      `json:"rpmLimit"`
	TpmLimit int      `json:"tpmLimit"`
	Priority int      `json:"priority"`
}

// ChannelUpdateRequest is the input for updating an existing channel.
type ChannelUpdateRequest struct {
	Name     *string   `json:"name,omitempty"`
	BaseURL  *string   `json:"baseURL,omitempty"`
	APIKey   *string   `json:"apiKey,omitempty"`
	Models   *[]string `json:"models,omitempty"`
	RpmLimit *int      `json:"rpmLimit,omitempty"`
	TpmLimit *int      `json:"tpmLimit,omitempty"`
	Priority *int      `json:"priority,omitempty"`
	Enabled  *bool     `json:"enabled,omitempty"`
}

// ChannelTestResult holds the result of a channel connectivity test (D-06).
type ChannelTestResult struct {
	Success bool   `json:"success"`
	Latency int64  `json:"latency"`
	Error   string `json:"error,omitempty"`
}

// RouteInfo represents a model route configuration (D-04).
type RouteInfo struct {
	ID        string        `json:"id"`
	Model     string        `json:"model"`
	Strategy  string        `json:"strategy"`
	Channels  []RouteChannel `json:"channels"`
	CreatedAt time.Time     `json:"createdAt"`
}

// RouteChannel is a channel within a route with weight and priority.
type RouteChannel struct {
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName"`
	Weight      int    `json:"weight"`
	Priority    int    `json:"priority"`
	Enabled     bool   `json:"enabled"`
}

// RouteCreateRequest is the input for creating a new model route.
type RouteCreateRequest struct {
	Model    string            `json:"model"`
	Strategy string            `json:"strategy"`
	Channels []RouteChannelInput `json:"channels"`
}

// RouteChannelInput is the channel weight input for route creation/update.
type RouteChannelInput struct {
	ChannelID string `json:"channelID"`
	Weight    int    `json:"weight"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
}

// RouteUpdateRequest is the input for updating a model route.
type RouteUpdateRequest struct {
	Model    *string            `json:"model,omitempty"`
	Strategy *string            `json:"strategy,omitempty"`
	Channels *[]RouteChannelInput `json:"channels,omitempty"`
}

// PlanInfo represents a package/plan for admin management (D-10, D-11).
type PlanInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	QuotaAmount  float64   `json:"quotaAmount"`
	TokenQuota   int       `json:"tokenQuota"`
	Price        float64   `json:"price"`
	ModelAccess  []string  `json:"modelAccess"`
	AgentLimit   int       `json:"agentLimit"`
	DurationDays *int      `json:"durationDays,omitempty"`
	IsActive     bool      `json:"isActive"`
	IsPublic     bool      `json:"isPublic"`
	SortOrder    int       `json:"sortOrder"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// PlanCreateRequest is the input for creating a new plan.
type PlanCreateRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	QuotaAmount float64  `json:"quotaAmount"`
	TokenQuota  int      `json:"tokenQuota"`
	Price       float64  `json:"price"`
	ModelAccess []string `json:"modelAccess"`
	AgentLimit  int      `json:"agentLimit"`
	DurationDays *int    `json:"durationDays,omitempty"`
	IsPublic    bool     `json:"isPublic"`
	SortOrder   int      `json:"sortOrder"`
}

// PlanUpdateRequest is the input for updating an existing plan.
type PlanUpdateRequest struct {
	Name        *string   `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	QuotaAmount *float64  `json:"quotaAmount,omitempty"`
	TokenQuota  *int      `json:"tokenQuota,omitempty"`
	Price       *float64  `json:"price,omitempty"`
	ModelAccess *[]string `json:"modelAccess,omitempty"`
	AgentLimit  *int      `json:"agentLimit,omitempty"`
	IsActive    *bool     `json:"isActive,omitempty"`
	IsPublic    *bool     `json:"isPublic,omitempty"`
}

// UserDetail is the full user view for admin management (D-12, D-13).
type UserDetail struct {
	ID          string          `json:"id"`
	Email       string          `json:"email"`
	Name        string          `json:"name"`
	Role        string          `json:"role"`   // "admin"|"moderator"|"user"
	PlanID      *string         `json:"planID,omitempty"`
	PlanName    *string         `json:"planName,omitempty"`
	Status      string          `json:"status"` // "active"|"disabled"
	CreatedAt   time.Time       `json:"createdAt"`
	LastLoginAt *time.Time      `json:"lastLoginAt,omitempty"`
	UsageStats  *UserUsageStats `json:"usageStats,omitempty"`
}

// UserUsageStats holds aggregated usage statistics for a user.
type UserUsageStats struct {
	TotalTokens   int     `json:"totalTokens"`
	TotalAPICalls int     `json:"totalAPICalls"`
	TotalCost     float64 `json:"totalCost"`
}

// UserUpdateRequest is the input for updating a user's admin attributes.
type UserUpdateRequest struct {
	Role   *string `json:"role,omitempty"`
	PlanID *string `json:"planID,omitempty"`
	Status *string `json:"status,omitempty"`
}

// UserListFilter contains filter parameters for listing users.
type UserListFilter struct {
	Search string
	Role   string
	PlanID string
	Status string
	Sort   string
	Limit  int
	Offset int
}

// AuditEntry represents an audit log record (D-08).
type AuditEntry struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actorID"`
	ActorEmail   string    `json:"actorEmail"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceID,omitempty"`
	Changes      string    `json:"changes,omitempty"` // JSON string
	IPAddress    string    `json:"ipAddress,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// AuditFilter contains filter parameters for listing audit entries.
type AuditFilter struct {
	ActorID      string
	Action       string
	ResourceType string
	DateFrom     string
	DateTo       string
	Limit        int
	Offset       int
}

// BatchRequest is the input for batch operations (D-08).
type BatchRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"` // "enable"|"disable"
}
