package admin

import "time"

// ChannelInfo represents a relay channel in the admin list (D-02, D-03).
type ChannelInfo struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Provider           string    `json:"provider"`
	BaseURL            string    `json:"baseURL"`
	Models             []string  `json:"models"`
	Groups             []string  `json:"groups"`
	RPM                int       `json:"rpm"`
	TPM                int       `json:"tpm"`
	Priority           int       `json:"priority"`
	EstimatedCostPer1K float64   `json:"estimatedCostPer1K"`
	CostMultiplier     float64   `json:"costMultiplier"`
	Enabled            bool      `json:"enabled"`
	Status             string    `json:"status"`  // "online"|"degraded"|"offline"
	Latency            int64     `json:"latency"` // milliseconds
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// ChannelProviderInfo describes one relay provider type supported by the
// gateway and exposed to admin channel forms.
type ChannelProviderInfo struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	Kind           string `json:"kind"`
	Status         string `json:"status"`
	DefaultBaseURL string `json:"defaultBaseURL"`
}

// ModelInventoryEntry is the admin model inventory row aggregated from relay
// channels and request-level usage records.
type ModelInventoryEntry struct {
	Model                 string                  `json:"model"`
	Providers             []string                `json:"providers"`
	Groups                []string                `json:"groups"`
	ChannelCount          int                     `json:"channelCount"`
	EnabledChannelCount   int                     `json:"enabledChannelCount"`
	DisabledChannelCount  int                     `json:"disabledChannelCount"`
	MinEstimatedCostPer1K float64                 `json:"minEstimatedCostPer1K"`
	MaxEstimatedCostPer1K float64                 `json:"maxEstimatedCostPer1K"`
	AvgCostMultiplier     float64                 `json:"avgCostMultiplier"`
	RequestCount          int                     `json:"requestCount"`
	TotalCost             float64                 `json:"totalCost"`
	TotalChannelCost      float64                 `json:"totalChannelCost"`
	Channels              []ModelInventoryChannel `json:"channels"`
}

// ModelInventoryChannel is one channel that advertises a model.
type ModelInventoryChannel struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Provider           string   `json:"provider"`
	Groups             []string `json:"groups"`
	Enabled            bool     `json:"enabled"`
	Priority           int      `json:"priority"`
	EstimatedCostPer1K float64  `json:"estimatedCostPer1K"`
	CostMultiplier     float64  `json:"costMultiplier"`
}

// ModelInventoryFilter contains filter parameters for admin model inventory.
type ModelInventoryFilter struct {
	Provider string
	Group    string
	Status   string
	Search   string
	Sort     string
	Limit    int
	Offset   int
}

// ChannelCreateRequest is the input for creating a new channel (D-02).
type ChannelCreateRequest struct {
	Name               string   `json:"name"`
	Provider           string   `json:"provider"`
	BaseURL            string   `json:"baseURL"`
	APIKey             string   `json:"apiKey"`
	Models             []string `json:"models"`
	Groups             []string `json:"groups"`
	RpmLimit           int      `json:"rpmLimit"`
	TpmLimit           int      `json:"tpmLimit"`
	Priority           int      `json:"priority"`
	EstimatedCostPer1K float64  `json:"estimatedCostPer1K"`
	CostMultiplier     float64  `json:"costMultiplier"`
}

// ChannelUpdateRequest is the input for updating an existing channel.
type ChannelUpdateRequest struct {
	Name               *string   `json:"name,omitempty"`
	BaseURL            *string   `json:"baseURL,omitempty"`
	APIKey             *string   `json:"apiKey,omitempty"`
	Models             *[]string `json:"models,omitempty"`
	Groups             *[]string `json:"groups,omitempty"`
	RpmLimit           *int      `json:"rpmLimit,omitempty"`
	TpmLimit           *int      `json:"tpmLimit,omitempty"`
	Priority           *int      `json:"priority,omitempty"`
	EstimatedCostPer1K *float64  `json:"estimatedCostPer1K,omitempty"`
	CostMultiplier     *float64  `json:"costMultiplier,omitempty"`
	Enabled            *bool     `json:"enabled,omitempty"`
}

// RelayPricingSettings contains the gateway multiplier settings used by relay
// billing. It mirrors new-api's model/group ratio concepts with explicit JSON
// maps so admins can tune pricing without redeploying.
type RelayPricingSettings struct {
	ModelMultipliers map[string]float64 `json:"modelMultipliers"`
	GroupMultipliers map[string]float64 `json:"groupMultipliers"`
}

// ChannelTestResult holds the result of a channel connectivity test (D-06).
type ChannelTestResult struct {
	Success      bool                 `json:"success"`
	Latency      int64                `json:"latency"`
	Provider     string               `json:"provider,omitempty"`
	Models       []string             `json:"models,omitempty"`
	Balance      *ChannelBalance      `json:"balance,omitempty"`
	BalanceError string               `json:"balanceError,omitempty"`
	Health       *ChannelHealthDetail `json:"health,omitempty"`
	Error        string               `json:"error,omitempty"`
}

// ChannelModelSyncResult is returned when an admin probes a channel and writes
// the discovered upstream model list back to the channel configuration.
type ChannelModelSyncResult struct {
	Channel    *ChannelInfo       `json:"channel"`
	TestResult *ChannelTestResult `json:"testResult"`
}

// ChannelModelUpdatePreview describes the delta between the configured channel
// model list and the currently reported upstream model list.
type ChannelModelUpdatePreview struct {
	ID             string             `json:"id"`
	CurrentModels  []string           `json:"currentModels"`
	UpstreamModels []string           `json:"upstreamModels"`
	Added          []string           `json:"added"`
	Removed        []string           `json:"removed"`
	Unchanged      []string           `json:"unchanged"`
	TestResult     *ChannelTestResult `json:"testResult"`
}

// ChannelModelUpdateApplyRequest controls how detected upstream model changes
// are applied to a channel. Mode may be "merge" or "replace"; empty means
// merge.
type ChannelModelUpdateApplyRequest struct {
	Mode string `json:"mode,omitempty"`
}

// ChannelModelUpdateApplyResult returns the diff plus the persisted channel.
type ChannelModelUpdateApplyResult struct {
	Channel       *ChannelInfo               `json:"channel"`
	Preview       *ChannelModelUpdatePreview `json:"preview"`
	Mode          string                     `json:"mode"`
	AppliedModels []string                   `json:"appliedModels"`
}

// ChannelDiagnosticsUpdate persists the latest operational probe state for a
// relay channel so the admin surface can show known health without relying only
// on transient probe responses.
type ChannelDiagnosticsUpdate struct {
	Status       string               `json:"status"`
	Latency      int64                `json:"latency"`
	Balance      *ChannelBalance      `json:"balance,omitempty"`
	BalanceError string               `json:"balanceError,omitempty"`
	Health       *ChannelHealthDetail `json:"health,omitempty"`
	Error        string               `json:"error,omitempty"`
	CheckedAt    time.Time            `json:"checkedAt"`
}

// ChannelBalanceRefreshResult is returned when an admin explicitly refreshes
// and stores the provider-reported account balance for a channel.
type ChannelBalanceRefreshResult struct {
	ID            string               `json:"id"`
	Status        string               `json:"status"`
	Balance       *ChannelBalance      `json:"balance,omitempty"`
	BalanceError  string               `json:"balanceError,omitempty"`
	ChannelHealth *ChannelHealthDetail `json:"channelHealth,omitempty"`
	TestResult    *ChannelTestResult   `json:"testResult"`
	CheckedAt     time.Time            `json:"checkedAt"`
}

type ChannelBalance struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Source   string  `json:"source,omitempty"`
}

type ChannelHealthDetail struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}

// ChannelHealth is the lightweight health view for admin channel diagnostics.
type ChannelHealth struct {
	ID           string               `json:"id"`
	Status       string               `json:"status"`
	Latency      int64                `json:"latency"`
	Models       []string             `json:"models,omitempty"`
	Balance      *ChannelBalance      `json:"balance,omitempty"`
	BalanceError string               `json:"balanceError,omitempty"`
	Health       *ChannelHealthDetail `json:"health,omitempty"`
	Error        string               `json:"error,omitempty"`
	CheckedAt    time.Time            `json:"checkedAt"`
}

// ChannelRuntimeStats exposes in-memory relay routing counters for admin
// diagnostics. These values reset with the relay process.
type ChannelRuntimeStats struct {
	ChannelID                 string     `json:"channelID"`
	RPMCurrent                int        `json:"rpmCurrent"`
	TPMCurrent                int        `json:"tpmCurrent"`
	TotalRequests             int64      `json:"totalRequests"`
	SuccessCount              int64      `json:"successCount"`
	FailureCount              int64      `json:"failureCount"`
	AvgLatencyMS              float64    `json:"avgLatencyMs"`
	RateLimitedUntil          *time.Time `json:"rateLimitedUntil,omitempty"`
	AffinityConversationCount int        `json:"affinityConversationCount"`
}

// RouteInfo represents a model route configuration (D-04).
type RouteInfo struct {
	ID        string         `json:"id"`
	Model     string         `json:"model"`
	Strategy  string         `json:"strategy"`
	Channels  []RouteChannel `json:"channels"`
	CreatedAt time.Time      `json:"createdAt"`
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
	Model    string              `json:"model"`
	Strategy string              `json:"strategy"`
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
	Model    *string              `json:"model,omitempty"`
	Strategy *string              `json:"strategy,omitempty"`
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
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	QuotaAmount  float64  `json:"quotaAmount"`
	TokenQuota   int      `json:"tokenQuota"`
	Price        float64  `json:"price"`
	ModelAccess  []string `json:"modelAccess"`
	AgentLimit   int      `json:"agentLimit"`
	DurationDays *int     `json:"durationDays,omitempty"`
	IsPublic     bool     `json:"isPublic"`
	SortOrder    int      `json:"sortOrder"`
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
	Role        string          `json:"role"` // "admin"|"moderator"|"user"
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
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId,omitempty"`
	ActorID        string    `json:"actorID"`
	ActorEmail     string    `json:"actorEmail"`
	Action         string    `json:"action"`
	ResourceType   string    `json:"resourceType"`
	ResourceID     string    `json:"resourceID,omitempty"`
	Changes        string    `json:"changes,omitempty"` // JSON string
	IPAddress      string    `json:"ipAddress,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// AuditFilter contains filter parameters for listing audit entries.
type AuditFilter struct {
	ActorID        string
	Action         string
	OrganizationID string
	ResourceType   string
	ResourceID     string
	DateFrom       string
	DateTo         string
	Limit          int
	Offset         int
}

// APITokenEntry is the admin-wide Relay API token inspection row.
type APITokenEntry struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	UserID             string     `json:"userId"`
	UserEmail          string     `json:"userEmail"`
	Name               string     `json:"name"`
	TokenPrefix        string     `json:"tokenPrefix"`
	Status             string     `json:"status"`
	UserGroup          string     `json:"userGroup,omitempty"`
	ModelLimitsEnabled bool       `json:"modelLimitsEnabled"`
	ModelLimits        []string   `json:"modelLimits"`
	QuotaLimit         *float64   `json:"quotaLimit,omitempty"`
	UsedQuota          float64    `json:"usedQuota"`
	RequestCount       int        `json:"requestCount"`
	TotalCost          float64    `json:"totalCost"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt         *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
}

// APITokenFilter contains filter parameters for admin Relay API token inspection.
type APITokenFilter struct {
	OrganizationID string
	UserID         string
	Status         string
	UserGroup      string
	Search         string
	Model          string
	Limit          int
	Offset         int
}

// UsageLogEntry is a request-level Relay usage record for admin inspection.
type UsageLogEntry struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organizationId,omitempty"`
	UserID           string    `json:"userId"`
	APITokenID       string    `json:"apiTokenId,omitempty"`
	RequestID        string    `json:"requestId,omitempty"`
	APIType          string    `json:"apiType,omitempty"`
	FeatureType      string    `json:"featureType,omitempty"`
	QuotaMode        string    `json:"quotaMode,omitempty"`
	Model            string    `json:"model"`
	ChannelID        string    `json:"channelId,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	Status           string    `json:"status,omitempty"`
	StatusCode       int       `json:"statusCode,omitempty"`
	ErrorCode        string    `json:"errorCode,omitempty"`
	LatencyMS        int       `json:"latencyMs,omitempty"`
	Cost             float64   `json:"cost"`
	ChannelCost      float64   `json:"channelCost"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	CreatedAt        time.Time `json:"createdAt"`
}

// UsageLogFilter contains filter parameters for admin Relay usage inspection.
type UsageLogFilter struct {
	OrganizationID string
	UserID         string
	APITokenID     string
	RequestID      string
	APIType        string
	FeatureType    string
	QuotaMode      string
	Model          string
	ChannelID      string
	Provider       string
	Status         string
	Limit          int
	Offset         int
}

// UsageAnalyticsFilter contains filter parameters for aggregated usage analytics.
type UsageAnalyticsFilter struct {
	OrganizationID string
	UserID         string
	APIType        string
	FeatureType    string
	QuotaMode      string
	Model          string
	ChannelID      string
	Provider       string
	Status         string
	Granularity    string
	From           time.Time
	To             time.Time
	Limit          int
}

// UsageAnalyticsBucket is one grouped usage aggregate. Dimension is one of
// model, feature, user, or time.
type UsageAnalyticsBucket struct {
	Dimension    string    `json:"dimension"`
	Key          string    `json:"key"`
	Primary      string    `json:"primary,omitempty"`
	Secondary    string    `json:"secondary,omitempty"`
	RequestCount int       `json:"requestCount"`
	TotalTokens  int       `json:"totalTokens"`
	TotalCost    float64   `json:"totalCost"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
}

type UsageAnalytics struct {
	ByModel         []UsageAnalyticsBucket `json:"byModel"`
	ByFeature       []UsageAnalyticsBucket `json:"byFeature"`
	ByUser          []UsageAnalyticsBucket `json:"byUser"`
	ByTime          []UsageAnalyticsBucket `json:"byTime"`
	CrossDimensions []UsageAnalyticsBucket `json:"crossDimensions"`
}

// BatchRequest is the input for batch operations (D-08).
type BatchRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"` // "enable"|"disable"
}

// BillingInspectionFilter contains shared filters for read-only Admin billing inspection.
type BillingInspectionFilter struct {
	OrganizationID string
	UserID         string
	Status         string
	Kind           string
	Provider       string
	Limit          int
	Offset         int
}

// BillingAmountSummary holds count and amount totals for a billing surface.
type BillingAmountSummary struct {
	Count               int     `json:"count"`
	TotalAmount         float64 `json:"totalAmount,omitempty"`
	PreAuthorizedAmount float64 `json:"preAuthorizedAmount,omitempty"`
	SettledAmount       float64 `json:"settledAmount,omitempty"`
	RefundedAmount      float64 `json:"refundedAmount,omitempty"`
	PaidAmount          float64 `json:"paidAmount,omitempty"`
	AmountDue           float64 `json:"amountDue,omitempty"`
	AmountPaid          float64 `json:"amountPaid,omitempty"`
	GrossAmount         float64 `json:"grossAmount,omitempty"`
	PlatformFeeAmount   float64 `json:"platformFeeAmount,omitempty"`
	PublisherNetAmount  float64 `json:"publisherNetAmount,omitempty"`
	FailedCount         int     `json:"failedCount,omitempty"`
	ActiveCount         int     `json:"activeCount,omitempty"`
}

// BillingInspectionSummary aggregates all v06 money-movement inspection surfaces.
type BillingInspectionSummary struct {
	BillingSessions BillingAmountSummary `json:"billingSessions"`
	PaymentIntents  BillingAmountSummary `json:"paymentIntents"`
	WebhookEvents   BillingAmountSummary `json:"webhookEvents"`
	Subscriptions   BillingAmountSummary `json:"subscriptions"`
	Topups          BillingAmountSummary `json:"topups"`
	Invoices        BillingAmountSummary `json:"invoices"`
	Refunds         BillingAmountSummary `json:"refunds"`
	Settlements     BillingAmountSummary `json:"settlements"`
	Payouts         BillingAmountSummary `json:"payouts"`
}

// BillingSessionInspection is a read-only Relay billing session row.
type BillingSessionInspection struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organizationId"`
	UserID              string     `json:"userId"`
	ChannelID           string     `json:"channelId,omitempty"`
	Model               string     `json:"model,omitempty"`
	APIType             string     `json:"apiType,omitempty"`
	IdempotencyKey      string     `json:"idempotencyKey"`
	PreAuthorizedAmount float64    `json:"preAuthorizedAmount"`
	SettledAmount       float64    `json:"settledAmount"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"createdAt"`
	SettledAt           *time.Time `json:"settledAt,omitempty"`
}

// PaymentIntentInspection is a read-only commercial checkout intent row.
type PaymentIntentInspection struct {
	ID                        string    `json:"id"`
	Provider                  string    `json:"provider"`
	ProviderCheckoutSessionID string    `json:"providerCheckoutSessionId,omitempty"`
	ProviderPaymentIntentID   string    `json:"providerPaymentIntentId,omitempty"`
	OrganizationID            string    `json:"organizationId"`
	UserID                    string    `json:"userId"`
	PackageID                 string    `json:"packageId,omitempty"`
	Kind                      string    `json:"kind"`
	Amount                    float64   `json:"amount"`
	Currency                  string    `json:"currency"`
	Status                    string    `json:"status"`
	RefundedAmount            float64   `json:"refundedAmount"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// WebhookEventInspection is a read-only provider webhook ledger row.
type WebhookEventInspection struct {
	ID              string     `json:"id"`
	Provider        string     `json:"provider"`
	EventID         string     `json:"eventId"`
	EventType       string     `json:"eventType"`
	Status          string     `json:"status"`
	OrganizationID  string     `json:"organizationId,omitempty"`
	UserID          string     `json:"userId,omitempty"`
	PaymentIntentID string     `json:"paymentIntentId,omitempty"`
	Error           string     `json:"error,omitempty"`
	ReceivedAt      time.Time  `json:"receivedAt"`
	ProcessedAt     *time.Time `json:"processedAt,omitempty"`
}

// SubscriptionInspection is a read-only subscription lifecycle row.
type SubscriptionInspection struct {
	ID                        string     `json:"id"`
	OrganizationID            string     `json:"organizationId"`
	UserID                    string     `json:"userId"`
	PackageID                 string     `json:"packageId"`
	Status                    string     `json:"status"`
	ProviderSubscriptionID    string     `json:"providerSubscriptionId,omitempty"`
	ProviderCustomerID        string     `json:"providerCustomerId,omitempty"`
	ProviderCheckoutSessionID string     `json:"providerCheckoutSessionId,omitempty"`
	ProviderLatestInvoiceID   string     `json:"providerLatestInvoiceId,omitempty"`
	CurrentPeriodStart        time.Time  `json:"currentPeriodStart"`
	CurrentPeriodEnd          *time.Time `json:"currentPeriodEnd,omitempty"`
	CancelAtPeriodEnd         bool       `json:"cancelAtPeriodEnd"`
	FailedPaymentAt           *time.Time `json:"failedPaymentAt,omitempty"`
	CreatedAt                 time.Time  `json:"createdAt"`
	UpdatedAt                 time.Time  `json:"updatedAt"`
}

// TopupInspection is a read-only top-up order row.
type TopupInspection struct {
	ID                        string     `json:"id"`
	OrganizationID            string     `json:"organizationId"`
	UserID                    string     `json:"userId"`
	PaymentIntentID           string     `json:"paymentIntentId,omitempty"`
	ProviderCheckoutSessionID string     `json:"providerCheckoutSessionId,omitempty"`
	Amount                    float64    `json:"amount"`
	Money                     float64    `json:"money"`
	Status                    string     `json:"status"`
	TradeNo                   string     `json:"tradeNo,omitempty"`
	RefundedAmount            float64    `json:"refundedAmount"`
	PaidAt                    *time.Time `json:"paidAt,omitempty"`
	CreatedAt                 time.Time  `json:"createdAt"`
}

// InvoiceInspection is a read-only billing invoice row.
type InvoiceInspection struct {
	ID                      string     `json:"id"`
	Provider                string     `json:"provider"`
	ProviderInvoiceID       string     `json:"providerInvoiceId"`
	ProviderSubscriptionID  string     `json:"providerSubscriptionId,omitempty"`
	ProviderPaymentIntentID string     `json:"providerPaymentIntentId,omitempty"`
	OrganizationID          string     `json:"organizationId"`
	UserID                  string     `json:"userId"`
	SubscriptionID          string     `json:"subscriptionId,omitempty"`
	PaymentIntentID         string     `json:"paymentIntentId,omitempty"`
	Status                  string     `json:"status"`
	AmountDue               float64    `json:"amountDue"`
	AmountPaid              float64    `json:"amountPaid"`
	Currency                string     `json:"currency"`
	HostedInvoiceURL        string     `json:"hostedInvoiceUrl,omitempty"`
	InvoicePDF              string     `json:"invoicePdf,omitempty"`
	PeriodStart             *time.Time `json:"periodStart,omitempty"`
	PeriodEnd               *time.Time `json:"periodEnd,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// RefundInspection is a read-only refund row.
type RefundInspection struct {
	ID                      string    `json:"id"`
	Provider                string    `json:"provider"`
	ProviderRefundID        string    `json:"providerRefundId"`
	ProviderChargeID        string    `json:"providerChargeId,omitempty"`
	ProviderPaymentIntentID string    `json:"providerPaymentIntentId,omitempty"`
	OrganizationID          string    `json:"organizationId"`
	UserID                  string    `json:"userId"`
	PaymentIntentID         string    `json:"paymentIntentId,omitempty"`
	TopupOrderID            string    `json:"topupOrderId,omitempty"`
	Amount                  float64   `json:"amount"`
	Currency                string    `json:"currency"`
	Status                  string    `json:"status"`
	Reason                  string    `json:"reason,omitempty"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

// MarketplaceSettlementInspection is a read-only Marketplace settlement row.
type MarketplaceSettlementInspection struct {
	ID                      string     `json:"id"`
	OrderID                 string     `json:"orderId"`
	PublisherOrganizationID string     `json:"publisherOrganizationId"`
	PublisherUserID         string     `json:"publisherUserId"`
	AgentID                 string     `json:"agentId"`
	GrossAmount             float64    `json:"grossAmount"`
	PlatformFeeAmount       float64    `json:"platformFeeAmount"`
	PublisherNetAmount      float64    `json:"publisherNetAmount"`
	RefundedAmount          float64    `json:"refundedAmount"`
	PayoutID                string     `json:"payoutId,omitempty"`
	Status                  string     `json:"status"`
	HoldUntil               *time.Time `json:"holdUntil,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// MarketplacePayoutInspection is a read-only Marketplace payout-state row.
type MarketplacePayoutInspection struct {
	ID                      string    `json:"id"`
	PublisherOrganizationID string    `json:"publisherOrganizationId"`
	PublisherUserID         string    `json:"publisherUserId"`
	Amount                  float64   `json:"amount"`
	Currency                string    `json:"currency"`
	Provider                string    `json:"provider"`
	ProviderPayoutID        string    `json:"providerPayoutId,omitempty"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}
