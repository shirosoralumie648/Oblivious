package console

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/userprefs"
)

type UsageSummary struct {
	Period     string                   `json:"period"`
	Requests   int                      `json:"requests"`
	ByModel    []UsageDimensionSummary  `json:"byModel"`
	ByFeature  []UsageDimensionSummary  `json:"byFeature"`
	ByUser     []UsageDimensionSummary  `json:"byUser"`
	TimeSeries []UsageTimeSeriesSummary `json:"timeSeries"`
	Recent     []APITokenUsageItem      `json:"recent"`
}

type APITokenUsageItem struct {
	ID               string    `json:"id"`
	APITokenID       string    `json:"apiTokenId"`
	RequestID        string    `json:"requestId"`
	APIType          string    `json:"apiType"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	StatusCode       int       `json:"statusCode"`
	ErrorCode        string    `json:"errorCode,omitempty"`
	LatencyMS        int64     `json:"latencyMs"`
	Cost             float64   `json:"cost"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	TotalTokens      int       `json:"totalTokens"`
	CreatedAt        time.Time `json:"createdAt"`
}

type UsageDimensionSummary struct {
	Key          string  `json:"key"`
	RequestCount int     `json:"requestCount"`
	TotalTokens  int     `json:"totalTokens"`
	TotalCost    float64 `json:"totalCost"`
}

type UsageTimeSeriesSummary struct {
	Bucket       string  `json:"bucket"`
	RequestCount int     `json:"requestCount"`
	TotalTokens  int     `json:"totalTokens"`
	TotalCost    float64 `json:"totalCost"`
}

type ModelSummary struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Requests int    `json:"requests"`
}

type BillingSummary struct {
	Period           string                          `json:"period"`
	Requests         int                             `json:"requests"`
	InputTokens      int                             `json:"inputTokens"`
	OutputTokens     int                             `json:"outputTokens"`
	EstimatedCostUSD float64                         `json:"estimatedCostUsd"`
	BalanceUSD       float64                         `json:"balanceUsd"`
	CreditLimitUSD   float64                         `json:"creditLimitUsd"`
	CurrentSpendUSD  float64                         `json:"currentSpendUsd"`
	PaymentProviders []BillingPaymentProviderSummary `json:"paymentProviders"`
	NextInvoice      *BillingInvoiceSummary          `json:"nextInvoice,omitempty"`
}

type BillingInvoiceSummary struct {
	ID               string  `json:"id"`
	Status           string  `json:"status"`
	AmountUSD        float64 `json:"amountUsd"`
	DueAt            string  `json:"dueAt"`
	HostedInvoiceURL string  `json:"hostedInvoiceUrl,omitempty"`
	InvoicePDF       string  `json:"invoicePdf,omitempty"`
}

type BillingPaymentProviderSummary struct {
	Name string `json:"name"`
}

type AccessSummary struct {
	DefaultMode         string `json:"defaultMode"`
	ModelStrategy       string `json:"modelStrategy"`
	NetworkEnabledHint  bool   `json:"networkEnabledHint"`
	OnboardingCompleted bool   `json:"onboardingCompleted"`
	SessionExpiresAt    string `json:"sessionExpiresAt"`
	SessionID           string `json:"sessionId"`
	UserEmail           string `json:"userEmail"`
	UserID              string `json:"userId"`
	WorkspaceID         string `json:"workspaceId"`
}

type CreateAPITokenRequest struct {
	Name               string     `json:"name"`
	UserGroup          string     `json:"userGroup,omitempty"`
	ModelLimitsEnabled bool       `json:"modelLimitsEnabled"`
	ModelLimits        []string   `json:"modelLimits"`
	QuotaLimit         *float64   `json:"quotaLimit,omitempty"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
}

type Store interface {
	GetBillingSummary(ctx context.Context, organizationID string) (BillingSummary, error)
	ListBillingInvoices(ctx context.Context, organizationID string) ([]BillingInvoiceSummary, error)
	GetModelSummaries(ctx context.Context, organizationID string) ([]ModelSummary, error)
	GetUsageSummary(ctx context.Context, organizationID, userID string) (UsageSummary, error)
}

type APITokenStore interface {
	CreateRelayAPIToken(ctx context.Context, input relay.CreateRelayAPITokenInput) (relay.CreatedRelayAPIToken, error)
	ListRelayAPITokens(ctx context.Context, organizationID, userID string) ([]relay.RelayAPITokenListItem, error)
	ListRelayAPITokenUsage(ctx context.Context, organizationID, userID, tokenID string) ([]relay.RelayAPITokenUsageItem, error)
	RevokeRelayAPIToken(ctx context.Context, organizationID, userID, tokenID string) error
}

type Service struct {
	apiTokenStore    APITokenStore
	paymentProviders []BillingPaymentProviderSummary
	store            Store
}

type ServiceOption func(*Service)

func WithBillingPaymentProviders(providers []BillingPaymentProviderSummary) ServiceOption {
	return func(service *Service) {
		service.paymentProviders = cloneBillingPaymentProviders(providers)
	}
}

func NewService(store Store, options ...ServiceOption) *Service {
	service := &Service{store: store}
	for _, option := range options {
		option(service)
	}
	return service
}

func NewServiceWithAPITokens(store Store, apiTokenStore APITokenStore, options ...ServiceOption) *Service {
	service := NewService(store, options...)
	service.apiTokenStore = apiTokenStore
	return service
}

func (s *Service) GetUsage(ctx context.Context, session auth.Session) (UsageSummary, error) {
	return s.store.GetUsageSummary(ctx, session.OrganizationID, session.User.ID)
}

func (s *Service) GetModels(ctx context.Context, session auth.Session) ([]ModelSummary, error) {
	return s.store.GetModelSummaries(ctx, session.OrganizationID)
}

func (s *Service) GetBilling(ctx context.Context, session auth.Session) (BillingSummary, error) {
	summary, err := s.store.GetBillingSummary(ctx, session.OrganizationID)
	if err != nil {
		return BillingSummary{}, err
	}
	summary.PaymentProviders = cloneBillingPaymentProviders(s.paymentProviders)
	return summary, nil
}

func (s *Service) ListBillingInvoices(ctx context.Context, session auth.Session) ([]BillingInvoiceSummary, error) {
	return s.store.ListBillingInvoices(ctx, session.OrganizationID)
}

func (s *Service) GetAccess(session auth.Session, preferences userprefs.Preferences) AccessSummary {
	return AccessSummary{
		DefaultMode:         preferences.DefaultMode,
		ModelStrategy:       preferences.ModelStrategy,
		NetworkEnabledHint:  preferences.NetworkEnabledHint,
		OnboardingCompleted: preferences.OnboardingCompleted,
		SessionExpiresAt:    session.ExpiresAt.UTC().Format(time.RFC3339),
		SessionID:           session.ID,
		UserEmail:           session.User.Email,
		UserID:              session.User.ID,
		WorkspaceID:         session.WorkspaceID,
	}
}

func (s *Service) CreateAPIToken(ctx context.Context, session auth.Session, input CreateAPITokenRequest) (relay.CreatedRelayAPIToken, error) {
	if s.apiTokenStore == nil {
		return relay.CreatedRelayAPIToken{}, errors.New("api token store is not configured")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return relay.CreatedRelayAPIToken{}, errors.New("api token name is required")
	}
	if input.QuotaLimit != nil && *input.QuotaLimit <= 0 {
		return relay.CreatedRelayAPIToken{}, errors.New("api token quota limit must be greater than zero")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC()) {
		return relay.CreatedRelayAPIToken{}, errors.New("api token expiration must be in the future")
	}
	return s.apiTokenStore.CreateRelayAPIToken(ctx, relay.CreateRelayAPITokenInput{
		UserID:             session.User.ID,
		OrganizationID:     session.OrganizationID,
		UserGroup:          strings.TrimSpace(input.UserGroup),
		Name:               name,
		ModelLimitsEnabled: input.ModelLimitsEnabled,
		ModelLimits:        normalizeModelLimits(input.ModelLimits),
		QuotaLimit:         input.QuotaLimit,
		ExpiresAt:          input.ExpiresAt,
	})
}

func (s *Service) ListAPITokens(ctx context.Context, session auth.Session) ([]relay.RelayAPITokenListItem, error) {
	if s.apiTokenStore == nil {
		return nil, errors.New("api token store is not configured")
	}
	return s.apiTokenStore.ListRelayAPITokens(ctx, session.OrganizationID, session.User.ID)
}

func (s *Service) ListAPITokenUsage(ctx context.Context, session auth.Session, tokenID string) ([]APITokenUsageItem, error) {
	if s.apiTokenStore == nil {
		return nil, errors.New("api token store is not configured")
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, sql.ErrNoRows
	}
	tokens, err := s.apiTokenStore.ListRelayAPITokens(ctx, session.OrganizationID, session.User.ID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, token := range tokens {
		if token.ID == tokenID {
			found = true
			break
		}
	}
	if !found {
		return nil, sql.ErrNoRows
	}
	usage, err := s.apiTokenStore.ListRelayAPITokenUsage(ctx, session.OrganizationID, session.User.ID, tokenID)
	if err != nil {
		return nil, err
	}
	return userVisibleAPITokenUsageItems(usage), nil
}

func (s *Service) RevokeAPIToken(ctx context.Context, session auth.Session, tokenID string) error {
	if s.apiTokenStore == nil {
		return errors.New("api token store is not configured")
	}
	if strings.TrimSpace(tokenID) == "" {
		return sql.ErrNoRows
	}
	return s.apiTokenStore.RevokeRelayAPIToken(ctx, session.OrganizationID, session.User.ID, tokenID)
}

func normalizeModelLimits(modelLimits []string) []string {
	normalized := make([]string, 0, len(modelLimits))
	seen := make(map[string]bool)
	for _, model := range modelLimits {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		normalized = append(normalized, model)
	}
	return normalized
}

func cloneBillingPaymentProviders(providers []BillingPaymentProviderSummary) []BillingPaymentProviderSummary {
	cloned := make([]BillingPaymentProviderSummary, 0, len(providers))
	for _, provider := range providers {
		name := strings.ToLower(strings.TrimSpace(provider.Name))
		if name == "" {
			continue
		}
		cloned = append(cloned, BillingPaymentProviderSummary{Name: name})
	}
	return cloned
}

func userVisibleAPITokenUsageItems(items []relay.RelayAPITokenUsageItem) []APITokenUsageItem {
	visible := make([]APITokenUsageItem, 0, len(items))
	for _, item := range items {
		visible = append(visible, APITokenUsageItem{
			ID:               item.ID,
			APITokenID:       item.APITokenID,
			RequestID:        item.RequestID,
			APIType:          item.APIType,
			Model:            item.Model,
			Status:           item.Status,
			StatusCode:       item.StatusCode,
			ErrorCode:        item.ErrorCode,
			LatencyMS:        item.LatencyMS,
			Cost:             item.Cost,
			PromptTokens:     item.PromptTokens,
			CompletionTokens: item.CompletionTokens,
			TotalTokens:      item.TotalTokens,
			CreatedAt:        item.CreatedAt,
		})
	}
	return visible
}

type SQLStore struct {
	db *sql.DB
}

func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}
