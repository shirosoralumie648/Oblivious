package marketplace

import (
	"context"
	"encoding/json"
	"time"
)

const (
	AgentStatusDraft         = "draft"
	AgentStatusPendingReview = "pending_review"
	AgentStatusApproved      = "approved"
	AgentStatusRejected      = "rejected"
	AgentStatusNeedsChanges  = "needs_changes"
	AgentStatusTakedown      = "takedown"
)

// PublishedAgent represents an agent published to the marketplace (D-17, D-18).
type PublishedAgent struct {
	ID                   string                  `json:"id"`
	OrganizationID       string                  `json:"organizationId"`
	OwnerID              string                  `json:"ownerID"`
	OwnerName            string                  `json:"ownerName,omitempty"`
	Name                 string                  `json:"name"`
	Description          string                  `json:"description"`
	IconURL              string                  `json:"iconURL,omitempty"`
	CategoryID           string                  `json:"categoryID,omitempty"`
	CategoryName         string                  `json:"categoryName,omitempty"`
	Tags                 []string                `json:"tags"`
	Tools                string                  `json:"tools"`                // JSON string
	ExampleConversations string                  `json:"exampleConversations"` // JSON string
	SystemPrompt         string                  `json:"systemPrompt,omitempty"`
	Visibility           string                  `json:"visibility"` // "public"|"private"|"unlisted"
	Status               string                  `json:"status"`     // "draft"|"pending_review"|"approved"|"rejected"|"needs_changes"|"takedown"
	ReviewReason         string                  `json:"reviewReason,omitempty"`
	PricingType          string                  `json:"pricingType"` // "free"|"one_time"|"subscription"
	PricingAmount        float64                 `json:"pricingAmount"`
	CurrentVersion       string                  `json:"currentVersion,omitempty"`
	InstallCount         int                     `json:"installCount"`
	RatingAvg            float64                 `json:"ratingAvg"`
	RatingCount          int                     `json:"ratingCount"`
	Recommendation       *RecommendationMetadata `json:"recommendation,omitempty"`
	CreatedAt            time.Time               `json:"createdAt"`
	UpdatedAt            time.Time               `json:"updatedAt"`
	PublisherReviewTier  string                  `json:"publisherReviewTier,omitempty"`
	ReviewSLA            *ReviewSLA              `json:"reviewSLA,omitempty"`
}

// RecommendationMetadata explains why a Marketplace result was recommended.
type RecommendationMetadata struct {
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// AgentRankingSignalEvent is an aggregateable marketplace recommendation signal.
type AgentRankingSignalEvent string

const (
	AgentRankingSignalImpression        AgentRankingSignalEvent = "impression"
	AgentRankingSignalClick             AgentRankingSignalEvent = "click"
	AgentRankingSignalInstallConversion AgentRankingSignalEvent = "install_conversion"
)

// ReviewSLA is computed Marketplace moderation queue metadata.
type ReviewSLA struct {
	SubmittedAt               time.Time `json:"submittedAt"`
	AutomatedReviewDeadlineAt time.Time `json:"automatedReviewDeadlineAt"`
	AutomatedReviewSlaMinutes int       `json:"automatedReviewSlaMinutes"`
	AutomatedReviewSlaStatus  string    `json:"automatedReviewSlaStatus"`
	ManualDeadlineAt          time.Time `json:"manualDeadlineAt"`
	ManualSlaHours            int       `json:"manualSlaHours"`
	ManualSlaStatus           string    `json:"manualSlaStatus"`
	MinutesUntilDeadline      int       `json:"minutesUntilDeadline"`
	VIPPublisher              bool      `json:"vipPublisher"`
	PublisherTier             string    `json:"publisherTier"`
	PublisherTierSource       string    `json:"publisherTierSource"`
}

// AgentPublishRequest is the input for publishing a new agent (D-17).
type AgentPublishRequest struct {
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	IconURL              string   `json:"iconURL"`
	CategoryID           string   `json:"categoryID"`
	Tags                 []string `json:"tags"`
	Tools                string   `json:"tools"`
	ExampleConversations string   `json:"exampleConversations"`
	SystemPrompt         string   `json:"systemPrompt"`
	Visibility           string   `json:"visibility"`
	PricingType          string   `json:"pricingType"`
	PricingAmount        float64  `json:"pricingAmount"`
	Version              string   `json:"version"`
	Changelog            string   `json:"changelog"`
}

// AgentReviewAction is the input for admin review actions (D-17).
type AgentReviewAction struct {
	AgentID string `json:"agentID"`
	Action  string `json:"action"` // "approve"|"reject"|"needs_changes"
	Reason  string `json:"reason,omitempty"`
}

// ReviewScanner performs automated Marketplace publication checks.
type ReviewScanner interface {
	ScanAgent(ctx context.Context, agent PublishedAgent) (AutomatedReviewResult, error)
}

// AutomatedReviewResult is the structured output from automated Marketplace review.
type AutomatedReviewResult struct {
	AgentID   string          `json:"agentID"`
	Decision  string          `json:"decision"` // "pending_manual_review"|"rejected"
	Scanner   string          `json:"scanner"`
	Findings  []ReviewFinding `json:"findings"`
	CreatedAt time.Time       `json:"createdAt"`
}

// ReviewFinding describes a single automated review issue.
type ReviewFinding struct {
	Type     string `json:"type"`     // "prompt_injection"|"sensitive_api"|"malicious_code"|"unsafe_tool"|"policy_violation"
	Severity string `json:"severity"` // "low"|"medium"|"high"|"critical"
	Field    string `json:"field"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

// AgentVersion represents a versioned snapshot of a published agent (D-19).
type AgentVersion struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agentID"`
	OrganizationID string    `json:"organizationId"`
	Version        string    `json:"version"`
	Changelog      string    `json:"changelog,omitempty"`
	Status         string    `json:"status"` // "pending_review"|"approved"|"rejected"|"needs_changes"
	CreatedAt      time.Time `json:"createdAt"`
}

// AgentReview represents a user review and rating of a published agent (D-27).
type AgentReview struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agentID"`
	OrganizationID string    `json:"organizationId"`
	UserID         string    `json:"userID"`
	UserName       string    `json:"userName,omitempty"`
	Rating         int       `json:"rating"`
	Body           string    `json:"body,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// AgentInstall represents a user installing a published agent (D-20).
type AgentInstall struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agentID"`
	OrganizationID string    `json:"organizationId"`
	UserID         string    `json:"userID"`
	InstalledAt    time.Time `json:"installedAt"`
}

// PaidInstallCheckoutRequest starts a payment-backed Marketplace install.
type PaidInstallCheckoutRequest struct {
	BuyerOrganizationID string
	BuyerUserID         string
	AgentID             string
	VersionID           string
	Provider            string
}

// PaidInstallCheckoutCompleted applies verified provider checkout evidence.
type PaidInstallCheckoutCompleted struct {
	EventID                   string
	OrderID                   string
	PaymentIntentID           string
	ProviderCheckoutSessionID string
	ProviderPaymentIntentID   string
}

// MarketplaceRefund applies verified refund evidence to Marketplace settlement state.
type MarketplaceRefund struct {
	EventID                 string
	ProviderRefundID        string
	PaymentIntentID         string
	ProviderPaymentIntentID string
	Amount                  float64
	Currency                string
	Reason                  string
}

// MarketplaceOrder records paid install order state.
type MarketplaceOrder struct {
	ID                        string    `json:"id"`
	BuyerOrganizationID       string    `json:"buyerOrganizationId"`
	BuyerUserID               string    `json:"buyerUserId"`
	PublisherOrganizationID   string    `json:"publisherOrganizationId"`
	PublisherUserID           string    `json:"publisherUserId"`
	AgentID                   string    `json:"agentId"`
	VersionID                 string    `json:"versionId,omitempty"`
	Provider                  string    `json:"provider,omitempty"`
	PaymentIntentID           string    `json:"paymentIntentId"`
	ProviderCheckoutSessionID string    `json:"providerCheckoutSessionId,omitempty"`
	ProviderPaymentIntentID   string    `json:"providerPaymentIntentId,omitempty"`
	InstallID                 string    `json:"installId,omitempty"`
	GrossAmount               float64   `json:"grossAmount"`
	PlatformFeeAmount         float64   `json:"platformFeeAmount"`
	PublisherNetAmount        float64   `json:"publisherNetAmount"`
	RefundedAmount            float64   `json:"refundedAmount"`
	Currency                  string    `json:"currency"`
	Status                    string    `json:"status"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// MarketplaceSettlement records publisher revenue and refund impact.
type MarketplaceSettlement struct {
	ID                      string    `json:"id"`
	OrderID                 string    `json:"orderId"`
	PublisherOrganizationID string    `json:"publisherOrganizationId"`
	PublisherUserID         string    `json:"publisherUserId"`
	AgentID                 string    `json:"agentId"`
	GrossAmount             float64   `json:"grossAmount"`
	PlatformFeeAmount       float64   `json:"platformFeeAmount"`
	PublisherNetAmount      float64   `json:"publisherNetAmount"`
	RefundedAmount          float64   `json:"refundedAmount"`
	PayoutID                string    `json:"payoutId,omitempty"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

// MarketplacePayout records provider-neutral local payout state.
type MarketplacePayout struct {
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

// GovernanceAction captures actor intent for Marketplace moderation changes.
type GovernanceAction struct {
	ActorUserID         string
	ActorOrganizationID string
	AgentID             string
	Reason              string
}

// AbuseReportRequest records a user abuse report for an agent.
type AbuseReportRequest struct {
	ReporterOrganizationID string
	ReporterUserID         string
	AgentID                string
	Reason                 string
	Details                string
}

// AbuseResolution resolves or dismisses an abuse report.
type AbuseResolution struct {
	ReportID       string
	ReviewerUserID string
	Status         string
	Resolution     string
}

// AbuseReportFilter scopes the admin Marketplace abuse review queue.
type AbuseReportFilter struct {
	Status string
	Limit  int
	Offset int
}

// AbuseReport is the persisted Marketplace abuse report.
type AbuseReport struct {
	ID                     string    `json:"id"`
	ReporterOrganizationID string    `json:"reporterOrganizationId"`
	ReporterUserID         string    `json:"reporterUserId"`
	AgentID                string    `json:"agentId"`
	Reason                 string    `json:"reason"`
	Details                string    `json:"details,omitempty"`
	Status                 string    `json:"status"`
	Resolution             string    `json:"resolution,omitempty"`
	ReviewerUserID         string    `json:"reviewerUserId,omitempty"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// ReviewInput is the input for a user submitting a review.
type ReviewInput struct {
	AgentID string `json:"agentID"`
	Rating  int    `json:"rating"`
	Body    string `json:"body,omitempty"`
}

// Category represents a marketplace category (D-28).
type Category struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	DisplayOrder int    `json:"displayOrder"`
	AgentCount   int    `json:"agentCount,omitempty"`
}

// MarketplaceTemplate represents a reusable bot, workflow, or plugin template.
type MarketplaceTemplate struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	TemplateData   json.RawMessage `json:"templateData"`
	Category       string          `json:"category,omitempty"`
	Tags           []string        `json:"tags"`
	DownloadsCount int             `json:"downloadsCount"`
	RatingAvg      float64         `json:"ratingAvg,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt,omitempty"`
}

// TemplateCreateRequest is the input for publishing a reusable template.
type TemplateCreateRequest struct {
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	TemplateData json.RawMessage `json:"templateData"`
	Category     string          `json:"category"`
	Tags         []string        `json:"tags"`
}

// TemplateFilter contains public template browsing filters.
type TemplateFilter struct {
	Query    string
	Type     string
	Category string
	Tags     []string
	Limit    int
	Offset   int
}

// TemplateInstall is the lightweight clone response returned when a template is used.
type TemplateInstall struct {
	ID             string          `json:"id"`
	TemplateID     string          `json:"templateID"`
	OrganizationID string          `json:"organizationId"`
	UserID         string          `json:"userID"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	TemplateData   json.RawMessage `json:"templateData"`
	InstalledAt    time.Time       `json:"installedAt"`
}

// MarketplaceSearchFilter contains filter parameters for searching the marketplace (D-30).
type MarketplaceSearchFilter struct {
	Query        string
	CategorySlug string
	Tags         []string
	MinRating    int
	MaxRating    int
	PricingType  string
	Sort         string
	Limit        int
	Offset       int
}
