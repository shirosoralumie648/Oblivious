package marketplace

import "time"

// PublishedAgent represents an agent published to the marketplace (D-17, D-18).
type PublishedAgent struct {
	ID                   string    `json:"id"`
	OwnerID              string    `json:"ownerID"`
	OwnerName            string    `json:"ownerName,omitempty"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	IconURL              string    `json:"iconURL,omitempty"`
	CategoryID           string    `json:"categoryID,omitempty"`
	CategoryName         string    `json:"categoryName,omitempty"`
	Tags                 []string  `json:"tags"`
	Tools                string    `json:"tools"`                // JSON string
	ExampleConversations string   `json:"exampleConversations"`  // JSON string
	SystemPrompt         string    `json:"systemPrompt,omitempty"`
	Visibility           string    `json:"visibility"`           // "public"|"private"|"unlisted"
	Status               string    `json:"status"`               // "draft"|"pending_review"|"approved"|"rejected"
	ReviewReason         string    `json:"reviewReason,omitempty"`
	PricingType          string    `json:"pricingType"`          // "free"|"one_time"|"subscription"
	PricingAmount        float64   `json:"pricingAmount"`
	CurrentVersion       string    `json:"currentVersion,omitempty"`
	InstallCount         int       `json:"installCount"`
	RatingAvg            float64   `json:"ratingAvg"`
	RatingCount          int       `json:"ratingCount"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// AgentPublishRequest is the input for publishing a new agent (D-17).
type AgentPublishRequest struct {
	Name                 string  `json:"name"`
	Description          string  `json:"description"`
	IconURL              string  `json:"iconURL"`
	CategoryID           string  `json:"categoryID"`
	Tags                 []string `json:"tags"`
	Tools                string  `json:"tools"`
	ExampleConversations string  `json:"exampleConversations"`
	SystemPrompt         string  `json:"systemPrompt"`
	Visibility           string  `json:"visibility"`
	PricingType          string  `json:"pricingType"`
	PricingAmount        float64 `json:"pricingAmount"`
	Version              string  `json:"version"`
	Changelog            string  `json:"changelog"`
}

// AgentReviewAction is the input for admin review actions (D-17).
type AgentReviewAction struct {
	AgentID string `json:"agentID"`
	Action  string `json:"action"` // "approve"|"reject"
	Reason  string `json:"reason,omitempty"`
}

// AgentVersion represents a versioned snapshot of a published agent (D-19).
type AgentVersion struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentID"`
	Version   string    `json:"version"`
	Changelog string    `json:"changelog,omitempty"`
	Status    string    `json:"status"` // "pending_review"|"approved"|"rejected"
	CreatedAt time.Time `json:"createdAt"`
}

// AgentReview represents a user review and rating of a published agent (D-27).
type AgentReview struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentID"`
	UserID    string    `json:"userID"`
	UserName  string    `json:"userName,omitempty"`
	Rating    int       `json:"rating"`
	Body      string    `json:"body,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AgentInstall represents a user installing a published agent (D-20).
type AgentInstall struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agentID"`
	UserID      string    `json:"userID"`
	InstalledAt time.Time `json:"installedAt"`
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
