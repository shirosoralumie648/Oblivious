package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/types"
)

func TestProductionDisabledRoutesFailClosedBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &countingHandler{}
	threadsHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles:   filesHandler,
		types.APITypeThreads: threadsHandler,
	}, RouteRegistrationOptions{Production: true})

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler *countingHandler
	}{
		{name: "thread create", method: http.MethodPost, path: "/v1/threads", handler: threadsHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "endpoint_disabled_in_production") {
				t.Fatalf("expected endpoint_disabled_in_production response, got %s", rec.Body.String())
			}
			if tc.handler.calls != 0 {
				t.Fatalf("disabled route reached handler %d times", tc.handler.calls)
			}
		})
	}
}

func TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	batchHandler := &countingHandler{}
	realtimeHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeBatch:    batchHandler,
		types.APITypeRealtime: realtimeHandler,
	}, RouteRegistrationOptions{Production: true})

	for _, tc := range []struct {
		name    string
		method  string
		path    string
		handler *countingHandler
	}{
		{name: "realtime websocket", method: http.MethodGet, path: "/v1/realtime", handler: realtimeHandler},
		{name: "batch create", method: http.MethodPost, path: "/v1/batch", handler: batchHandler},
		{name: "batch list", method: http.MethodGet, path: "/v1/batches", handler: batchHandler},
		{name: "batch retrieve", method: http.MethodGet, path: "/v1/batches/batch_123", handler: batchHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "endpoint_disabled_in_production") {
				t.Fatalf("expected endpoint_disabled_in_production response, got %s", rec.Body.String())
			}
			if tc.handler.calls != 0 {
				t.Fatalf("disabled route reached handler %d times", tc.handler.calls)
			}
		})
	}
}

func TestProductionFilesUploadRequiresAuditSinkBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles: filesHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{}`))
	addConfiguredTrustedRelayHeaders(t, req)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_audit_sink_required") {
		t.Fatalf("expected relay_audit_sink_required response, got %s", rec.Body.String())
	}
	if filesHandler.calls != 0 {
		t.Fatalf("file upload without audit sink reached handler %d times", filesHandler.calls)
	}
}

func TestProductionFilesUploadWithAuditSinkReachesHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &identityRecordingHandler{}
	audit := &recordingRouteAuditSink{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles: filesHandler,
	}, RouteRegistrationOptions{Production: true, AuditSink: audit})

	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{}`))
	addConfiguredTrustedRelayHeaders(t, req)
	req.Header.Set(types.HeaderRequestID, "req_file_1")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if filesHandler.userID != "user_1" || filesHandler.organizationID != "org_1" || filesHandler.requestID != "req_file_1" {
		t.Fatalf("handler saw identity user=%q org=%q request=%q", filesHandler.userID, filesHandler.organizationID, filesHandler.requestID)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	event := audit.events[0]
	if event.Result != RouteAuditResultAllowed || event.APIType != types.APITypeFiles || event.UserID != "user_1" || event.OrganizationID != "org_1" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}

func TestProductionSupportedRoutesStillReachHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	addConfiguredTrustedRelayHeaders(t, req)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if chatHandler.calls != 1 {
		t.Fatalf("supported route reached handler %d times, want 1", chatHandler.calls)
	}
}

func TestDevelopmentDisabledRoutesRemainCallableForLocalCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filesHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeFiles: filesHandler,
	}, RouteRegistrationOptions{Production: false})

	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if filesHandler.calls != 1 {
		t.Fatalf("development route reached handler %d times, want 1", filesHandler.calls)
	}
}

func TestProductionSupportedRoutesRequireTrustedIdentityBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	audit := &recordingRouteAuditSink{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, AuditSink: audit})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_identity_required") {
		t.Fatalf("expected relay_identity_required response, got %s", rec.Body.String())
	}
	if chatHandler.calls != 0 {
		t.Fatalf("unauthenticated route reached handler %d times", chatHandler.calls)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if audit.events[0].Result != RouteAuditResultRejected || audit.events[0].FailureReason == "" {
		t.Fatalf("expected rejected audit event with failure reason, got %+v", audit.events[0])
	}
}

func TestProductionTrustedIdentityRejectsDefaultInternalTokenWhenSecretUnset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN", "")

	chatHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	addTrustedRelayHeaders(req)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if chatHandler.calls != 0 {
		t.Fatalf("default internal token reached handler %d times", chatHandler.calls)
	}
}

func TestProductionSupportedRoutesAttachTrustedIdentityAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &identityRecordingHandler{}
	audit := &recordingRouteAuditSink{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, AuditSink: audit})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	addConfiguredTrustedRelayHeaders(t, req)
	req.Header.Set(types.HeaderRequestID, "req_123")
	req.Header.Set(types.HeaderInternalUserGroup, "vip")
	req.Header.Set(types.HeaderInternalConversation, "conversation_1")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if chatHandler.userID != "user_1" || chatHandler.organizationID != "org_1" || chatHandler.requestID != "req_123" {
		t.Fatalf("handler saw identity user=%q org=%q request=%q", chatHandler.userID, chatHandler.organizationID, chatHandler.requestID)
	}
	if chatHandler.userGroup != "vip" {
		t.Fatalf("handler saw user group %q, want vip", chatHandler.userGroup)
	}
	if chatHandler.conversationID != "conversation_1" {
		t.Fatalf("handler saw conversation id %q, want conversation_1", chatHandler.conversationID)
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	event := audit.events[0]
	if event.Result != RouteAuditResultAllowed || event.UserID != "user_1" || event.OrganizationID != "org_1" || event.RequestID != "req_123" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}

func TestProductionSupportedRoutesAcceptRelayAPIToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &identityRecordingHandler{}
	authenticator := &fakeRelayAPITokenAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_1",
			UserID:         "user_token",
			OrganizationID: "org_token",
			UserGroup:      "vip",
		},
	}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, APITokenAuthenticator: authenticator})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}`))
	req.Header.Set("Authorization", "Bearer obv_test")
	req.Header.Set(types.HeaderRequestID, "req_token")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if authenticator.rawToken != "obv_test" || authenticator.model != "gpt-4o" || authenticator.apiType != types.APITypeChat {
		t.Fatalf("authenticator saw token=%q model=%q apiType=%s", authenticator.rawToken, authenticator.model, authenticator.apiType.String())
	}
	if chatHandler.userID != "user_token" || chatHandler.organizationID != "org_token" || chatHandler.apiTokenID != "tok_1" || chatHandler.requestID != "req_token" {
		t.Fatalf("handler saw identity user=%q org=%q token=%q request=%q", chatHandler.userID, chatHandler.organizationID, chatHandler.apiTokenID, chatHandler.requestID)
	}
	if chatHandler.userGroup != "vip" {
		t.Fatalf("handler saw API token user group %q, want vip", chatHandler.userGroup)
	}
}

func TestProductionAPITokenRequestGetsGeneratedRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &identityRecordingHandler{}
	authenticator := &fakeRelayAPITokenAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_1",
			UserID:         "user_token",
			OrganizationID: "org_token",
		},
	}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true, APITokenAuthenticator: authenticator})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"ping"}]}`))
	req.Header.Set("Authorization", "Bearer obv_test")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if chatHandler.requestID == "" {
		t.Fatal("expected generated request id in handler context")
	}
	if rec.Header().Get(types.HeaderRequestID) != chatHandler.requestID {
		t.Fatalf("response request id = %q, handler request id = %q", rec.Header().Get(types.HeaderRequestID), chatHandler.requestID)
	}
}

func TestProductionSupportedRoutesRejectAPITokenModelDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{
		Production:            true,
		APITokenAuthenticator: &fakeRelayAPITokenAuthenticator{err: types.ErrRelayAPITokenModelDenied},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Authorization", "Bearer obv_model_denied")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_model_not_allowed") {
		t.Fatalf("expected relay_model_not_allowed response, got %s", rec.Body.String())
	}
	if chatHandler.calls != 0 {
		t.Fatalf("denied token reached handler %d times", chatHandler.calls)
	}
}

func TestRoutePolicyObservabilityRecordsAllowedAndRejectedDecisions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chatHandler := &countingHandler{}
	filesHandler := &countingHandler{}
	batchHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat:  chatHandler,
		types.APITypeFiles: filesHandler,
		types.APITypeBatch: batchHandler,
	}, RouteRegistrationOptions{Production: true})

	rejectedBefore := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(CommercialSupportedBilled),
		types.APITypeChat.String(),
		string(RouteAuditResultRejected),
		"relay_identity_required",
	))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	rejectedAfter := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(CommercialSupportedBilled),
		types.APITypeChat.String(),
		string(RouteAuditResultRejected),
		"relay_identity_required",
	))
	if rejectedAfter != rejectedBefore+1 {
		t.Fatalf("expected rejected route decision metric increment, before=%v after=%v", rejectedBefore, rejectedAfter)
	}

	allowedBefore := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(CommercialSupportedBilled),
		types.APITypeChat.String(),
		string(RouteAuditResultAllowed),
		"none",
	))
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))
	addConfiguredTrustedRelayHeaders(t, req)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	allowedAfter := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(CommercialSupportedBilled),
		types.APITypeChat.String(),
		string(RouteAuditResultAllowed),
		"none",
	))
	if allowedAfter != allowedBefore+1 {
		t.Fatalf("expected allowed route decision metric increment, before=%v after=%v", allowedBefore, allowedAfter)
	}

	disabledBefore := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(DisabledInProduction),
		types.APITypeBatch.String(),
		string(RouteAuditResultRejected),
		"endpoint_disabled_in_production",
	))
	req = httptest.NewRequest(http.MethodGet, "/v1/batches", nil)
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	disabledAfter := testutil.ToFloat64(metrics.RelayRouteDecisionsTotal.WithLabelValues(
		string(DisabledInProduction),
		types.APITypeBatch.String(),
		string(RouteAuditResultRejected),
		"endpoint_disabled_in_production",
	))
	if disabledAfter != disabledBefore+1 {
		t.Fatalf("expected disabled route decision metric increment, before=%v after=%v", disabledBefore, disabledAfter)
	}
}

func TestRoutePolicyObservabilityWritesStructuredDecisionEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer
	restoreLogger := setObservabilityLoggerForTest(observability.NewJSONLogger(&output))
	defer restoreLogger()

	chatHandler := &countingHandler{}
	engine := gin.New()
	RegisterRoutesWithOptions(engine, map[types.APIType]types.Handler{
		types.APITypeChat: chatHandler,
	}, RouteRegistrationOptions{Production: true})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?api_key=sk-secret", strings.NewReader(`{"prompt":"secret"}`))
	addConfiguredTrustedRelayHeaders(t, req)
	req.Header.Set(types.HeaderRequestID, "req_obs_1")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("expected structured relay JSON log, got error: %v\n%s", err, output.String())
	}
	if record["event"] != "relay.route_decision" {
		t.Fatalf("expected relay.route_decision event, got %#v", record["event"])
	}
	if record["request_id"] != "req_obs_1" {
		t.Fatalf("expected request id req_obs_1, got %#v", record["request_id"])
	}
	if record["organization_id"] != "org_1" || record["user_id"] != "user_1" {
		t.Fatalf("expected trusted identity in relay event, got %#v", record)
	}
	if record["relay_route_class"] != string(CommercialSupportedBilled) {
		t.Fatalf("expected relay route class, got %#v", record["relay_route_class"])
	}
	if record["relay_api_type"] != types.APITypeChat.String() {
		t.Fatalf("expected relay api type, got %#v", record["relay_api_type"])
	}
	if record["billing_policy"] != string(BillingPolicyUsageSettlement) {
		t.Fatalf("expected billing policy, got %#v", record["billing_policy"])
	}
	forbiddenText := output.String()
	for _, forbidden := range []string{"sk-secret", "secret", "api_key", "prompt"} {
		if strings.Contains(forbiddenText, forbidden) {
			t.Fatalf("relay event leaked forbidden text %q: %s", forbidden, forbiddenText)
		}
	}
}

type countingHandler struct {
	calls int
}

func (h *countingHandler) Handle(c *gin.Context) error {
	h.calls++
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
	return nil
}

func (h *countingHandler) HandleStream(c *gin.Context) error {
	return errors.New("stream should not be called in these tests")
}

type identityRecordingHandler struct {
	calls          int
	userID         string
	organizationID string
	requestID      string
	apiTokenID     string
	userGroup      string
	conversationID string
}

func (h *identityRecordingHandler) Handle(c *gin.Context) error {
	h.calls++
	h.userID, _ = types.TrustedUserIDFromContext(c.Request.Context())
	h.organizationID, _ = types.TrustedOrganizationIDFromContext(c.Request.Context())
	h.requestID, _ = types.TrustedRequestIDFromContext(c.Request.Context())
	h.apiTokenID, _ = types.TrustedAPITokenIDFromContext(c.Request.Context())
	h.userGroup, _ = types.TrustedUserGroupFromContext(c.Request.Context())
	h.conversationID, _ = types.TrustedConversationIDFromContext(c.Request.Context())
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
	return nil
}

func (h *identityRecordingHandler) HandleStream(c *gin.Context) error {
	return errors.New("stream should not be called in these tests")
}

type recordingRouteAuditSink struct {
	events []RouteAuditEvent
}

func (s *recordingRouteAuditSink) RecordRelayRouteDecision(ctx context.Context, event RouteAuditEvent) {
	s.events = append(s.events, event)
}

func addTrustedRelayHeaders(req *http.Request) {
	req.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	req.Header.Set(types.HeaderInternalUserID, "user_1")
	req.Header.Set(types.HeaderInternalOrganization, "org_1")
}

func addConfiguredTrustedRelayHeaders(t *testing.T, req *http.Request) {
	t.Helper()
	t.Setenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN", "test-internal-token")
	req.Header.Set(types.HeaderInternalAuth, "test-internal-token")
	req.Header.Set(types.HeaderInternalUserID, "user_1")
	req.Header.Set(types.HeaderInternalOrganization, "org_1")
}

type fakeRelayAPITokenAuthenticator struct {
	identity types.RelayAPITokenIdentity
	err      error
	rawToken string
	model    string
	apiType  types.APIType
}

func (a *fakeRelayAPITokenAuthenticator) AuthenticateRelayAPIToken(ctx context.Context, rawToken, model string, apiType types.APIType) (types.RelayAPITokenIdentity, error) {
	a.rawToken = rawToken
	a.model = model
	a.apiType = apiType
	if a.err != nil {
		return types.RelayAPITokenIdentity{}, a.err
	}
	return a.identity, nil
}
