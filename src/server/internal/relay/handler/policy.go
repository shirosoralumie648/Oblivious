package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/types"
)

type CommercialClass string

const (
	CommercialSupportedBilled CommercialClass = "commercial_supported_billed"
	InternalAdminOnly         CommercialClass = "internal_admin_only"
	DisabledInProduction      CommercialClass = "disabled_in_production"
)

type AuthPolicy string

const (
	AuthPolicyTrustedInternalIdentity AuthPolicy = "trusted_internal_identity"
	AuthPolicyNotApplicable           AuthPolicy = "not_applicable"
)

type RateLimitPolicy string

const (
	RateLimitPolicyGlobalTokenBucket RateLimitPolicy = "global_relay_token_bucket"
	RateLimitPolicyNotApplicable     RateLimitPolicy = "not_applicable"
)

type AuditPolicy string

const (
	AuditPolicyRouteDecision AuditPolicy = "relay_route_policy_decision"
)

type BillingPolicy string

const (
	BillingPolicyUsageSettlement    BillingPolicy = "preauthorize_then_settle_usage"
	BillingPolicyEstimateSettlement BillingPolicy = "preauthorize_then_settle_estimate"
	BillingPolicyProductionDisabled BillingPolicy = "production_disabled"
	BillingPolicyNotApplicable      BillingPolicy = "not_applicable"
)

type RouteAuditResult string

const (
	RouteAuditResultAllowed  RouteAuditResult = "allowed"
	RouteAuditResultRejected RouteAuditResult = "rejected"
)

type RoutePolicy struct {
	Method            string
	Path              string
	APIType           types.APIType
	Strategy          types.HandlerStrategy
	Class             CommercialClass
	ProductionEnabled bool
	DisabledReason    string
	FutureOwner       string

	AuthPolicy             AuthPolicy
	TenantIdentityRequired bool
	RateLimitPolicy        RateLimitPolicy
	AuditPolicy            AuditPolicy
	BillingPolicy          BillingPolicy
	RequiresAuditSink      bool
}

type RouteAuditEvent struct {
	Method         string
	Path           string
	APIType        types.APIType
	Class          CommercialClass
	Result         RouteAuditResult
	UserID         string
	OrganizationID string
	RequestID      string
	ChannelID      string
	FailureReason  string
	CreatedAt      time.Time
}

type RouteAuditSink interface {
	RecordRelayRouteDecision(ctx context.Context, event RouteAuditEvent)
}

var routeObservabilityLogger = observability.NewJSONLogger(os.Stdout)
var routeObservabilityLoggerMu sync.RWMutex

func AllRoutePolicies() []RoutePolicy {
	policies := make([]RoutePolicy, len(routePolicies))
	copy(policies, routePolicies)
	return policies
}

func PolicyForRoute(method, path string) (RoutePolicy, bool) {
	for _, policy := range routePolicies {
		if policy.Method == method && policy.Path == path {
			return policy, true
		}
	}
	return RoutePolicy{}, false
}

func EnforceRoutePolicy(c *gin.Context, route Route, opts RouteRegistrationOptions) bool {
	policy, ok := PolicyForRoute(route.Method, route.Path)
	if !ok {
		recordRouteAudit(c, opts.AuditSink, RoutePolicy{
			Method:  route.Method,
			Path:    route.Path,
			APIType: route.APIType,
		}, RouteAuditResultRejected, "", "", "", "endpoint_policy_missing")
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"code":    "endpoint_policy_missing",
				"message": "relay endpoint is missing commercial policy: " + route.Method + " " + route.Path,
			},
		})
		return true
	}

	if opts.Production {
		ensureRelayRequestID(c)
	}

	if rejectIfProductionDisabled(c, route, policy, opts) {
		return true
	}

	if opts.Production && policy.RequiresAuditSink && opts.AuditSink == nil {
		requestID := ensureRelayRequestID(c)
		recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, "relay_audit_sink_required")
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"code":    "relay_audit_sink_required",
				"message": "production relay file upload requires a configured route audit sink",
			},
		})
		return true
	}

	if opts.Production && policy.Class == InternalAdminOnly {
		userID, organizationID, requestID, userGroup, ok := trustedIdentityFromHeaders(c, true)
		if !ok {
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, "relay_identity_required")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "relay_identity_required",
					"message": "production relay endpoint requires trusted internal user and organization identity",
				},
			})
			return true
		}
		ctx := c.Request.Context()
		ctx = types.WithTrustedUserID(ctx, userID)
		ctx = types.WithTrustedOrganizationID(ctx, organizationID)
		if requestID != "" {
			ctx = types.WithTrustedRequestID(ctx, requestID)
		}
		if userGroup != "" {
			ctx = types.WithTrustedUserGroup(ctx, userGroup)
		}
		if featureType := trustedFeatureTypeFromHeaders(c); featureType != "" {
			ctx = types.WithTrustedFeatureType(ctx, featureType)
		}
		c.Request = c.Request.WithContext(ctx)
		recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultAllowed, userID, organizationID, requestID, "")
		return false
	}

	if opts.Production && policy.Class == CommercialSupportedBilled {
		userID, organizationID, requestID, userGroup, ok := trustedIdentityFromHeaders(c, true)
		if ok {
			ctx := c.Request.Context()
			ctx = types.WithTrustedUserID(ctx, userID)
			ctx = types.WithTrustedOrganizationID(ctx, organizationID)
			if requestID != "" {
				ctx = types.WithTrustedRequestID(ctx, requestID)
			}
			if userGroup != "" {
				ctx = types.WithTrustedUserGroup(ctx, userGroup)
			}
			if featureType := trustedFeatureTypeFromHeaders(c); featureType != "" {
				ctx = types.WithTrustedFeatureType(ctx, featureType)
			}
			if conversationID := strings.TrimSpace(c.GetHeader(types.HeaderInternalConversation)); conversationID != "" {
				ctx = types.WithTrustedConversationID(ctx, conversationID)
			}
			c.Request = c.Request.WithContext(ctx)
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultAllowed, userID, organizationID, requestID, "")
			return false
		}

		rawToken := bearerTokenFromRequest(c.Request)
		if rawToken == "" || opts.APITokenAuthenticator == nil {
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, "relay_identity_required")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "relay_identity_required",
					"message": "production relay endpoint requires trusted internal user and organization identity",
				},
			})
			return true
		}

		model, err := modelFromRequestBody(c.Request)
		if err != nil {
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, "relay_invalid_request")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "relay_invalid_request",
					"message": "relay request body must be valid JSON",
				},
			})
			return true
		}
		identity, err := opts.APITokenAuthenticator.AuthenticateRelayAPIToken(c.Request.Context(), rawToken, model, route.APIType)
		if err != nil {
			status, code := relayAPITokenErrorResponse(err)
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, code)
			c.JSON(status, gin.H{
				"error": gin.H{
					"code":    code,
					"message": "relay API token is not authorized for this request",
				},
			})
			return true
		}
		if identity.UserID == "" || identity.OrganizationID == "" || identity.TokenID == "" {
			recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", requestID, "relay_api_token_invalid")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "relay_api_token_invalid",
					"message": "relay API token is invalid",
				},
			})
			return true
		}
		ctx := c.Request.Context()
		ctx = types.WithTrustedUserID(ctx, identity.UserID)
		ctx = types.WithTrustedOrganizationID(ctx, identity.OrganizationID)
		ctx = types.WithTrustedAPITokenID(ctx, identity.TokenID)
		if identity.UserGroup != "" {
			ctx = types.WithTrustedUserGroup(ctx, identity.UserGroup)
		}
		if requestID != "" {
			ctx = types.WithTrustedRequestID(ctx, requestID)
		}
		c.Request = c.Request.WithContext(ctx)
		recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultAllowed, identity.UserID, identity.OrganizationID, requestID, "")
	}

	return false
}

func RejectIfProductionDisabled(c *gin.Context, route Route, production bool) bool {
	policy, ok := PolicyForRoute(route.Method, route.Path)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": gin.H{
				"code":    "endpoint_policy_missing",
				"message": "relay endpoint is missing commercial policy: " + route.Method + " " + route.Path,
			},
		})
		return true
	}
	return rejectIfProductionDisabled(c, route, policy, RouteRegistrationOptions{Production: production})
}

func rejectIfProductionDisabled(c *gin.Context, route Route, policy RoutePolicy, opts RouteRegistrationOptions) bool {
	production := opts.Production
	if !production {
		return false
	}

	if policy.ProductionEnabled && policy.Class != DisabledInProduction {
		return false
	}

	message := "relay endpoint is disabled in production: " + route.Method + " " + route.Path
	if policy.DisabledReason != "" {
		message += " (" + policy.DisabledReason + ")"
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"code":    "endpoint_disabled_in_production",
			"message": message,
		},
	})
	recordRouteAudit(c, opts.AuditSink, policy, RouteAuditResultRejected, "", "", c.GetHeader(types.HeaderRequestID), "endpoint_disabled_in_production")
	return true
}

func trustedIdentityFromHeaders(c *gin.Context, requireConfiguredSecret bool) (string, string, string, string, bool) {
	expectedToken := os.Getenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN")
	if expectedToken == "" {
		if requireConfiguredSecret {
			return "", "", strings.TrimSpace(c.GetHeader(types.HeaderRequestID)), "", false
		}
		expectedToken = types.SharedInternalToken
	}

	internalAuth := strings.TrimSpace(c.GetHeader(types.HeaderInternalAuth))
	userID := trustedHeader(c, types.HeaderInternalUserID, "X-Oblivious-Internal-User-ID")
	organizationID := trustedHeader(c, types.HeaderInternalOrganization, "X-Oblivious-Internal-Organization-ID")
	requestID := strings.TrimSpace(c.GetHeader(types.HeaderRequestID))
	userGroup := trustedHeader(c, types.HeaderInternalUserGroup, "X-Oblivious-Internal-User-Group")

	if internalAuth != expectedToken || userID == "" || organizationID == "" {
		return "", "", requestID, "", false
	}
	return userID, organizationID, requestID, userGroup, true
}

func trustedFeatureTypeFromHeaders(c *gin.Context) string {
	return trustedHeader(c, types.HeaderInternalFeatureType, "X-Oblivious-Internal-Feature-Type")
}

func trustedHeader(c *gin.Context, names ...string) string {
	for _, name := range names {
		value := strings.TrimSpace(c.GetHeader(name))
		if value != "" {
			return value
		}
	}
	return ""
}

func ensureRelayRequestID(c *gin.Context) string {
	requestID := strings.TrimSpace(c.GetHeader(types.HeaderRequestID))
	if requestID == "" {
		requestID = "req_" + uuid.NewString()
		c.Request.Header.Set(types.HeaderRequestID, requestID)
	}
	c.Header(types.HeaderRequestID, requestID)
	return requestID
}

func bearerTokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	prefix := "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(auth[len(prefix):])
}

func modelFromRequestBody(r *http.Request) (string, error) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodDelete {
		return "", nil
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "json") {
		return "", nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return payload.Model, nil
}

func relayAPITokenErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, types.ErrRelayAPITokenModelDenied):
		return http.StatusForbidden, "relay_model_not_allowed"
	case errors.Is(err, types.ErrRelayAPITokenQuotaExceeded):
		return http.StatusPaymentRequired, "relay_api_token_quota_exceeded"
	case errors.Is(err, types.ErrRelayAPITokenExpired):
		return http.StatusUnauthorized, "relay_api_token_expired"
	case errors.Is(err, types.ErrRelayAPITokenRevoked):
		return http.StatusUnauthorized, "relay_api_token_revoked"
	default:
		return http.StatusUnauthorized, "relay_api_token_invalid"
	}
}

func recordRouteAudit(c *gin.Context, sink RouteAuditSink, policy RoutePolicy, result RouteAuditResult, userID, organizationID, requestID, failureReason string) {
	if requestID == "" {
		requestID = c.GetHeader(types.HeaderRequestID)
	}
	ctx, span := observability.StartSpan(
		c.Request.Context(),
		"relay.route_policy",
		observability.String("relay.route_class", string(policy.Class)),
		observability.String("relay.api_type", policy.APIType.String()),
		observability.String("relay.result", string(result)),
	)
	defer span.End()

	metrics.RecordRelayRouteDecision(string(policy.Class), policy.APIType.String(), string(result), failureReason)
	currentObservabilityLogger().Log(ctx, observability.Event{
		Component:       "relay",
		Event:           "relay.route_decision",
		RequestID:       requestID,
		OrganizationID:  organizationID,
		UserID:          userID,
		Method:          policy.Method,
		Route:           policy.Path,
		RelayRouteClass: string(policy.Class),
		RelayAPIType:    policy.APIType.String(),
		BillingPolicy:   string(policy.BillingPolicy),
		FailureReason:   failureReason,
	})

	if sink == nil {
		return
	}
	sink.RecordRelayRouteDecision(ctx, RouteAuditEvent{
		Method:         policy.Method,
		Path:           policy.Path,
		APIType:        policy.APIType,
		Class:          policy.Class,
		Result:         result,
		UserID:         userID,
		OrganizationID: organizationID,
		RequestID:      requestID,
		FailureReason:  failureReason,
		CreatedAt:      time.Now().UTC(),
	})
}

func currentObservabilityLogger() *observability.Logger {
	routeObservabilityLoggerMu.RLock()
	defer routeObservabilityLoggerMu.RUnlock()
	return routeObservabilityLogger
}

func setObservabilityLoggerForTest(logger *observability.Logger) func() {
	routeObservabilityLoggerMu.Lock()
	previous := routeObservabilityLogger
	routeObservabilityLogger = logger
	routeObservabilityLoggerMu.Unlock()

	return func() {
		routeObservabilityLoggerMu.Lock()
		routeObservabilityLogger = previous
		routeObservabilityLoggerMu.Unlock()
	}
}

var routePolicies = []RoutePolicy{
	supportedUsage("POST", "/v1/chat/completions", types.APITypeChat, types.StrategyNative),
	supportedUsage("POST", "/v1/responses", types.APITypeResponses, types.StrategyNative),
	internalAdmin("GET", "/v1/models", types.APITypeModels, types.StrategyNative),
	disabled("GET", "/v1/realtime", types.APITypeRealtime, types.StrategyNative, "realtime settlement and client-abort billing are not defined", "Phase 15"),
	supportedUsage("POST", "/v1/embeddings", types.APITypeEmbeddings, types.StrategyNative),
	supportedEstimate("POST", "/v1/images/generations", types.APITypeImageGen, types.StrategyNative),
	supportedEstimate("POST", "/v1/images/edits", types.APITypeImageEdit, types.StrategyNative),
	supportedEstimate("POST", "/v1/images/variations", types.APITypeImageVar, types.StrategyNative),
	disabled("POST", "/v1/videos", types.APITypeVideos, types.StrategyNative, "video billing and provider behavior are not verified", "Phase 15"),
	supportedEstimate("POST", "/v1/audio/speech", types.APITypeAudioSpeech, types.StrategyNative),
	supportedEstimate("POST", "/v1/audio/transcriptions", types.APITypeAudioSTT, types.StrategyNative),
	supportedEstimate("POST", "/v1/audio/translations", types.APITypeAudioTranslate, types.StrategyNative),
	supportedEstimate("POST", "/v1/moderations", types.APITypeModeration, types.StrategyNative),
	supportedUsage("POST", "/v1/completions", types.APITypeCompletions, types.StrategyNative),
	disabled("POST", "/v1/batch", types.APITypeBatch, types.StrategyNative, "async batch settlement and audit are not defined", "Phase 15"),
	disabled("GET", "/v1/batches", types.APITypeBatch, types.StrategyPassthrough, "batch passthrough lacks commercial audit and settlement", "Phase 15"),
	disabled("GET", "/v1/batches/:id", types.APITypeBatch, types.StrategyPassthrough, "batch passthrough lacks commercial audit and settlement", "Phase 15"),
	supportedFileUpload("POST", "/v1/files", types.APITypeFiles, types.StrategyFileProxy),
	supportedEstimate("GET", "/v1/files", types.APITypeFiles, types.StrategyPassthrough),
	supportedEstimate("GET", "/v1/files/:id", types.APITypeFiles, types.StrategyPassthrough),
	supportedEstimate("DELETE", "/v1/files/:id", types.APITypeFiles, types.StrategyPassthrough),
	supportedEstimate("GET", "/v1/files/:id/content", types.APITypeFiles, types.StrategyPassthrough),
	disabled("POST", "/v1/fine_tuning/jobs", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and training-token billing are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs/:id", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning job lifecycle and audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/fine_tuning/jobs/:id/cancel", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning cancel billing and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/fine_tuning/jobs/:id/events", types.APITypeFineTuning, types.StrategyPassthrough, "fine-tuning event streaming audit is not implemented", "Phase 15"),
	disabled("POST", "/v1/assistants", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("GET", "/v1/assistants", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("GET", "/v1/assistants/:id", types.APITypeAssistants, types.StrategyPassthrough, "assistants lifecycle billing and governance are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads", types.APITypeThreads, types.StrategyPassthrough, "threads lifecycle billing and audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/threads/:id", types.APITypeThreads, types.StrategyPassthrough, "threads lifecycle billing and audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads/:id/runs", types.APITypeRuns, types.StrategyPassthrough, "runs lifecycle billing and tool-call audit are not implemented", "Phase 15"),
	disabled("GET", "/v1/threads/:id/runs/:rid", types.APITypeRuns, types.StrategyPassthrough, "runs lifecycle billing and tool-call audit are not implemented", "Phase 15"),
	disabled("POST", "/v1/threads/:id/runs/:rid/submit", types.APITypeRuns, types.StrategyPassthrough, "run submit-tool-output billing and audit are not implemented", "Phase 15"),
}

func supportedUsage(method, path string, apiType types.APIType, strategy types.HandlerStrategy) RoutePolicy {
	return supported(method, path, apiType, strategy, BillingPolicyUsageSettlement)
}

func supportedEstimate(method, path string, apiType types.APIType, strategy types.HandlerStrategy) RoutePolicy {
	return supported(method, path, apiType, strategy, BillingPolicyEstimateSettlement)
}

func supportedFileUpload(method, path string, apiType types.APIType, strategy types.HandlerStrategy) RoutePolicy {
	policy := supported(method, path, apiType, strategy, BillingPolicyEstimateSettlement)
	policy.RequiresAuditSink = true
	return policy
}

func internalAdmin(method, path string, apiType types.APIType, strategy types.HandlerStrategy) RoutePolicy {
	return RoutePolicy{
		Method:            method,
		Path:              path,
		APIType:           apiType,
		Strategy:          strategy,
		Class:             InternalAdminOnly,
		ProductionEnabled: true,
		AuthPolicy:        AuthPolicyTrustedInternalIdentity,
		RateLimitPolicy:   RateLimitPolicyNotApplicable,
		AuditPolicy:       AuditPolicyRouteDecision,
		BillingPolicy:     BillingPolicyNotApplicable,
	}
}

func supported(method, path string, apiType types.APIType, strategy types.HandlerStrategy, billingPolicy BillingPolicy) RoutePolicy {
	return RoutePolicy{
		Method:                 method,
		Path:                   path,
		APIType:                apiType,
		Strategy:               strategy,
		Class:                  CommercialSupportedBilled,
		ProductionEnabled:      true,
		AuthPolicy:             AuthPolicyTrustedInternalIdentity,
		TenantIdentityRequired: true,
		RateLimitPolicy:        RateLimitPolicyGlobalTokenBucket,
		AuditPolicy:            AuditPolicyRouteDecision,
		BillingPolicy:          billingPolicy,
	}
}

func disabled(method, path string, apiType types.APIType, strategy types.HandlerStrategy, reason, futureOwner string) RoutePolicy {
	return RoutePolicy{
		Method:                 method,
		Path:                   path,
		APIType:                apiType,
		Strategy:               strategy,
		Class:                  DisabledInProduction,
		ProductionEnabled:      false,
		DisabledReason:         reason,
		FutureOwner:            futureOwner,
		AuthPolicy:             AuthPolicyNotApplicable,
		TenantIdentityRequired: false,
		RateLimitPolicy:        RateLimitPolicyNotApplicable,
		AuditPolicy:            AuditPolicyRouteDecision,
		BillingPolicy:          BillingPolicyProductionDisabled,
	}
}
