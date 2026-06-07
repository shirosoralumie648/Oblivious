package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

func TestProviderResponseFromHTTPAttachesUsage(t *testing.T) {
	body := []byte(`{"id":"chatcmpl_1","usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`)

	resp := providerResponseFromHTTP(http.StatusOK, body)

	if resp.Usage == nil {
		t.Fatal("expected usage to be parsed")
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 8 || resp.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}

func TestProviderResponseFromHTTPLeavesMissingUsageNil(t *testing.T) {
	resp := providerResponseFromHTTP(http.StatusOK, []byte(`{"id":"img_1"}`))

	if resp.Usage != nil {
		t.Fatalf("expected nil usage when provider body has no usage, got %+v", resp.Usage)
	}
}

func TestExecuteProviderAdapterRequestPreservesRetryAfterHeader(t *testing.T) {
	adapter := &stubProviderAdapter{
		response: &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"45"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
		},
	}

	resp, err := executeProviderAdapterRequest(context.Background(), adapter, &types.ProviderRequest{Model: "gpt-4o", APIType: types.APITypeChat})
	if err != nil {
		t.Fatalf("executeProviderAdapterRequest returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected provider response")
	}
	if got := resp.Headers.Get("Retry-After"); got != "45" {
		t.Fatalf("Retry-After header = %q, want 45", got)
	}
}

func TestApplyTrustedInternalIdentityPreservesExistingAPITokenIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := types.WithTrustedUserID(req.Context(), "route_user")
	ctx = types.WithTrustedOrganizationID(ctx, "route_org")
	ctx = types.WithTrustedAPITokenID(ctx, "api_token_route")
	req = req.WithContext(ctx)
	req.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	req.Header.Set(types.HeaderInternalUserID, "header_user")
	req.Header.Set(types.HeaderInternalOrganization, "header_org")
	c.Request = req

	applyTrustedInternalIdentity(c)

	gotCtx := c.Request.Context()
	apiTokenID, _ := types.TrustedAPITokenIDFromContext(gotCtx)
	userID, _ := types.TrustedUserIDFromContext(gotCtx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(gotCtx)
	if apiTokenID != "api_token_route" || userID != "route_user" || organizationID != "route_org" {
		t.Fatalf("trusted route identity was overwritten: apiToken=%q user=%q org=%q", apiTokenID, userID, organizationID)
	}
}

func TestApplyTrustedInternalIdentityPreservesExistingRouteIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := types.WithTrustedUserID(req.Context(), "route_user")
	ctx = types.WithTrustedOrganizationID(ctx, "route_org")
	req = req.WithContext(ctx)
	req.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	req.Header.Set(types.HeaderInternalUserID, "header_user")
	req.Header.Set(types.HeaderInternalOrganization, "header_org")
	c.Request = req

	applyTrustedInternalIdentity(c)

	gotCtx := c.Request.Context()
	userID, _ := types.TrustedUserIDFromContext(gotCtx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(gotCtx)
	if userID != "route_user" || organizationID != "route_org" {
		t.Fatalf("trusted route identity was overwritten: user=%q org=%q", userID, organizationID)
	}
}

type stubProviderAdapter struct {
	response *http.Response
	err      error
}

func (a *stubProviderAdapter) Name() string { return "stub" }

func (a *stubProviderAdapter) Provider() string { return "stub" }

func (a *stubProviderAdapter) Capabilities() types.Capabilities { return types.Capabilities{} }

func (a *stubProviderAdapter) BuildURL(string, types.APIType) (string, error) {
	return "http://stub", nil
}

func (a *stubProviderAdapter) BuildHeaders(context.Context, string, types.APIType) (http.Header, error) {
	return http.Header{}, nil
}

func (a *stubProviderAdapter) ConvertRequest(req *types.ProviderRequest) (*types.ProviderRequest, error) {
	return req, nil
}

func (a *stubProviderAdapter) ConvertResponse(resp []byte, _ bool) (*types.ProviderResponse, error) {
	return &types.ProviderResponse{StatusCode: http.StatusOK, Content: resp, Done: true}, nil
}

func (a *stubProviderAdapter) DoRequest(context.Context, *types.ProviderRequest) (*http.Response, error) {
	return a.response, a.err
}

func (a *stubProviderAdapter) HealthCheck(context.Context) error { return nil }

func (a *stubProviderAdapter) MapError(statusCode int, _ []byte) *types.ProviderError {
	return &types.ProviderError{Code: "rate_limited", Message: "rate limited", StatusCode: statusCode, Retryable: true}
}

func (a *stubProviderAdapter) EstimateUsage(*types.ProviderRequest) *types.Usage { return nil }
