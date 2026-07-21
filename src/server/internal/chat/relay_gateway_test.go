package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	relaytypes "oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

func TestChatRelayReadinessDispatchContract(t *testing.T) {
	contract, profile := loadChatReadinessAuthority(t)
	newRuntime := func(t *testing.T, guard releasecontract.Guard, registrar releasecontract.EffectRegistrar) RelayGatewayRuntimeOptions {
		t.Helper()
		authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
		if err != nil {
			t.Fatalf("compile runtime authorities: %v", err)
		}
		return RelayGatewayRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar}
	}

	t.Run("constructors fail closed without the startup carrier", func(t *testing.T) {
		if _, err := NewRelayGatewayWithOptions(RelayGatewayRuntimeOptions{}); !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("expected Relay gateway construction denial, got %v", err)
		}
		if _, err := NewCompositeGatewayWithOptions(nil, nil, RelayGatewayRuntimeOptions{}); !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("expected Composite gateway construction denial, got %v", err)
		}
	})

	for _, mode := range []string{"complete", "stream", "structured"} {
		mode := mode
		t.Run(mode+" guards provider transport immediately before Do", func(t *testing.T) {
			guard := &chatReadinessGuardSpy{denyAtCall: 2, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "generation"}}
			registrar := &chatEffectRegistrar{}
			networkCalls := 0
			gateway, err := NewRelayGatewayWithOptions(
				newRuntime(t, guard, registrar),
				WithRelayURL("https://secret-relay.invalid/v1"),
				WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					networkCalls++
					return nil, errors.New("network must not be called")
				})}),
			)
			if err != nil {
				t.Fatalf("construct guarded gateway: %v", err)
			}
			config := ConversationConfig{ModelID: "gpt-4o-mini"}
			switch mode {
			case "complete":
				_, err = gateway.GenerateReply(context.Background(), []Message{{Role: "user", Content: "hello"}}, config)
			case "stream":
				err = gateway.GenerateReplyStream(context.Background(), []Message{{Role: "user", Content: "hello"}}, config, func(string) error { return nil })
			case "structured":
				_, err = gateway.GenerateStructuredReply(context.Background(), []Message{{Role: "user", Content: "hello"}}, config, nil)
			}
			if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityBlocked) || networkCalls != 0 {
				t.Fatalf("transport denial leaked network call: err=%v calls=%d guard=%#v", err, networkCalls, guard.calls)
			}
			if strings.Contains(err.Error(), "secret-relay") {
				t.Fatalf("readiness error leaked relay URL: %v", err)
			}
		})
	}

	t.Run("unknown current model is denied before network", func(t *testing.T) {
		guard := &chatReadinessGuardSpy{}
		networkCalls := 0
		gateway, err := NewRelayGatewayWithOptions(
			newRuntime(t, guard, &chatEffectRegistrar{}),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				networkCalls++
				return nil, errors.New("network must not be called")
			})}),
		)
		if err != nil {
			t.Fatalf("construct guarded gateway: %v", err)
		}
		_, err = gateway.GenerateReply(context.Background(), nil, ConversationConfig{ModelID: "caller-capability"})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) || networkCalls != 0 {
			t.Fatalf("unknown model did not fail before network: err=%v calls=%d", err, networkCalls)
		}
	})

	t.Run("shared consumer skips registration but preserves dispatch guard", func(t *testing.T) {
		guard := &chatReadinessGuardSpy{denyAtCall: 1, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "generation"}}
		registrar := &chatEffectRegistrar{}
		runtimeOptions := newRuntime(t, guard, registrar)
		runtimeOptions.SkipEffectRegistration = true
		networkCalls := 0
		gateway, err := NewRelayGatewayWithOptions(runtimeOptions,
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				networkCalls++
				return nil, errors.New("network must not be called")
			})}),
		)
		if err != nil {
			t.Fatalf("construct shared guarded gateway: %v", err)
		}
		_, err = gateway.GenerateReply(context.Background(), nil, ConversationConfig{ModelID: "gpt-4o-mini"})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityBlocked) || networkCalls != 0 {
			t.Fatalf("shared consumer denial leaked network call: err=%v calls=%d", err, networkCalls)
		}
		if len(registrar.descriptors) != 0 {
			t.Fatalf("shared consumer registered descriptors: %#v", registrar.descriptors)
		}
	})

	t.Run("fallback re-authorizes after primary failure", func(t *testing.T) {
		guard := &chatReadinessGuardSpy{denyAtCall: 3, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}}
		registrar := &chatEffectRegistrar{}
		runtimeOptions := newRuntime(t, guard, registrar)
		primaryCalls := 0
		primary, err := NewRelayGatewayWithOptions(
			runtimeOptions,
			WithRelayURL("https://relay.invalid/v1"),
			WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				primaryCalls++
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"unavailable"}`))}, nil
			})}),
		)
		if err != nil {
			t.Fatalf("construct guarded primary: %v", err)
		}
		fallback := &mockReplyGenerator{reply: "unsafe fallback"}
		composite, err := NewCompositeGatewayWithOptions(primary, fallback, runtimeOptions)
		if err != nil {
			t.Fatalf("construct guarded composite: %v", err)
		}
		_, err = composite.GenerateReply(context.Background(), []Message{{Role: "user", Content: "hello"}}, ConversationConfig{ModelID: "gpt-4o-mini"})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) || primaryCalls != 1 || fallback.calls != 0 {
			t.Fatalf("fallback denial leaked dispatch: err=%v primary=%d fallback=%d guard=%#v", err, primaryCalls, fallback.calls, guard.calls)
		}
		if len(registrar.descriptors) != 2 || registrar.descriptors[0].ID == registrar.descriptors[1].ID {
			t.Fatalf("expected distinct primary/fallback descriptors, got %#v", registrar.descriptors)
		}
	})

	t.Run("cancellation cannot authorize fallback", func(t *testing.T) {
		fallback := &mockReplyGenerator{reply: "unsafe fallback"}
		composite := NewCompositeGateway(canceledChatGateway{}, fallback)
		_, err := composite.GenerateReply(context.Background(), nil, ConversationConfig{ModelID: "gpt-4o-mini"})
		if !errors.Is(err, context.Canceled) || fallback.calls != 0 {
			t.Fatalf("cancellation triggered fallback: err=%v calls=%d", err, fallback.calls)
		}
	})
}

type chatReadinessGuardCall struct {
	capabilityID string
	boundary     releasecontract.Boundary
}

type chatReadinessGuardSpy struct {
	denyAtCall int
	denial     error
	calls      []chatReadinessGuardCall
}

func (g *chatReadinessGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.calls = append(g.calls, chatReadinessGuardCall{capabilityID: capabilityID, boundary: boundary})
	if g.denyAtCall > 0 && len(g.calls) >= g.denyAtCall {
		return g.denial
	}
	return nil
}

type chatEffectRegistrar struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *chatEffectRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

func loadChatReadinessAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve chat readiness test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}

type canceledChatGateway struct{}

func (canceledChatGateway) GenerateReply(context.Context, []Message, ConversationConfig) (string, error) {
	return "", context.Canceled
}

func (canceledChatGateway) GenerateReplyStream(context.Context, []Message, ConversationConfig, func(string) error) error {
	return context.Canceled
}

func TestRelayGateway_GenerateReply(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "test-id",
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Hello, this is a test response."
				}
			}]
		}`))
	}))
	defer server.Close()

	gateway := NewRelayGateway(
		WithRelayURL(server.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}
	config := ConversationConfig{
		ModelID:     "gpt-4o-mini",
		Temperature: 1.0,
	}

	reply, err := gateway.GenerateReply(context.Background(), messages, config)
	if err != nil {
		t.Fatalf("GenerateReply failed: %v", err)
	}

	if reply != "Hello, this is a test response." {
		t.Errorf("unexpected reply: %s", reply)
	}
}

func TestRelayGateway_GenerateReply_Error(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	gateway := NewRelayGateway(
		WithRelayURL(server.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)

	messages := []Message{{Role: "user", Content: "Hello"}}
	config := ConversationConfig{}

	_, err := gateway.GenerateReply(context.Background(), messages, config)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRelayGateway_GenerateReplyStream(t *testing.T) {
	// Create test server with SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send SSE chunks
		chunks := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			`data: {"choices":[{"delta":{"content":" world"}}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
		}
	}))
	defer server.Close()

	gateway := NewRelayGateway(
		WithRelayURL(server.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)

	messages := []Message{{Role: "user", Content: "Hello"}}
	config := ConversationConfig{}

	var received []string
	err := gateway.GenerateReplyStream(context.Background(), messages, config, func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	if err != nil {
		t.Fatalf("GenerateReplyStream failed: %v", err)
	}

	if len(received) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(received))
	}
	if received[0] != "Hello" || received[1] != " world" {
		t.Errorf("unexpected chunks: %v", received)
	}
}

func TestRelayGateway_GenerateReplyStreamAcceptsLargeSSEChunks(t *testing.T) {
	largeChunk := strings.Repeat("stream-token-", 7000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"` + largeChunk + `"}}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	gateway := NewRelayGateway(
		WithRelayURL(server.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)

	var received strings.Builder
	err := gateway.GenerateReplyStream(context.Background(), []Message{{Role: "user", Content: "Hello"}}, ConversationConfig{}, func(chunk string) error {
		received.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateReplyStream should accept large SSE chunks, got error: %v", err)
	}
	if received.String() != largeChunk {
		t.Fatalf("expected large chunk to round trip, got len=%d want=%d", received.Len(), len(largeChunk))
	}
}

func TestCompositeGateway_Fallback(t *testing.T) {
	// Create failing primary gateway
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failingServer.Close()

	primaryGateway := NewRelayGateway(
		WithRelayURL(failingServer.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)

	// Create fallback generator
	fallbackGenerator := &mockReplyGenerator{reply: "fallback reply"}

	composite := NewCompositeGateway(primaryGateway, fallbackGenerator)

	messages := []Message{{Role: "user", Content: "Hello"}}
	config := ConversationConfig{}

	// Should fallback to mock generator
	reply, err := composite.GenerateReply(context.Background(), messages, config)
	if err != nil {
		t.Fatalf("GenerateReply failed: %v", err)
	}

	if reply != "fallback reply" {
		t.Errorf("expected fallback reply, got: %s", reply)
	}
}

func TestCompositeGateway_NoFallbackReturnsPrimaryError(t *testing.T) {
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer failingServer.Close()

	primaryGateway := NewRelayGateway(
		WithRelayURL(failingServer.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
	)
	composite := NewCompositeGateway(primaryGateway, nil)

	_, err := composite.GenerateReply(context.Background(), []Message{{Role: "user", Content: "Hello"}}, ConversationConfig{})
	if err == nil {
		t.Fatal("expected primary Relay failure to be returned when fallback is nil")
	}
	if !strings.Contains(err.Error(), "relay returned status") {
		t.Fatalf("expected failing primary relay assertion, got %v", err)
	}
	if composite.LastError() == nil {
		t.Fatal("expected last primary error to be recorded")
	}
}

func TestRelayGateway_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	gateway := NewRelayGateway(
		WithRelayURL(server.URL+"/v1"),
		WithDefaultModel("gpt-4o-mini"),
		WithHTTPClient(&http.Client{Timeout: 100 * time.Millisecond}),
	)

	messages := []Message{{Role: "user", Content: "Hello"}}
	config := ConversationConfig{}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := gateway.GenerateReply(ctx, messages, config)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestRelayGateway_GenerateStructuredReply_WithToolCalls(t *testing.T) {
	var capturedBody map[string]any

	gateway := NewRelayGateway(
		WithRelayURL("http://relay.test/v1"),
		WithDefaultModel("gpt-4o-mini"),
		WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"id": "tool-call-response",
						"choices": [{
							"finish_reason": "tool_calls",
							"message": {
								"role": "assistant",
								"content": "",
								"tool_calls": [{
									"id": "call_weather",
									"type": "function",
									"function": {
										"name": "weather.lookup",
										"arguments": "{\"city\":\"Shanghai\"}"
									}
								}]
							}
						}],
						"usage": {
							"prompt_tokens": 12,
							"completion_tokens": 4,
							"total_tokens": 16
						}
					}`)),
				}, nil
			}),
		}),
	)

	reply, err := gateway.GenerateStructuredReply(
		context.Background(),
		[]Message{{Role: "user", Content: "What's the weather?"}},
		ConversationConfig{ModelID: "gpt-4o-mini"},
		[]map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name": "weather.lookup",
					"parameters": map[string]any{
						"type": "object",
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("GenerateStructuredReply failed: %v", err)
	}

	toolsRaw, ok := capturedBody["tools"].([]any)
	if !ok || len(toolsRaw) != 1 {
		t.Fatalf("expected request tools to be forwarded, got %#v", capturedBody["tools"])
	}

	if reply.FinishReason != "tool_calls" {
		t.Fatalf("expected finish reason tool_calls, got %q", reply.FinishReason)
	}
	if len(reply.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(reply.ToolCalls))
	}
	if reply.ToolCalls[0].Function.Name != "weather.lookup" {
		t.Fatalf("expected tool name weather.lookup, got %q", reply.ToolCalls[0].Function.Name)
	}
	if reply.ToolCalls[0].Function.Arguments != "{\"city\":\"Shanghai\"}" {
		t.Fatalf("unexpected arguments: %q", reply.ToolCalls[0].Function.Arguments)
	}
	if reply.Usage == nil || reply.Usage.TotalTokens != 16 {
		t.Fatalf("expected usage tokens to be parsed, got %+v", reply.Usage)
	}
	if !reply.Usage.RecordedByRelay {
		t.Fatalf("expected relay parsed usage to be marked authoritative, got %+v", reply.Usage)
	}
}

func TestRelayGateway_GenerateStructuredReply_PlainText(t *testing.T) {
	gateway := NewRelayGateway(
		WithRelayURL("http://relay.test/v1"),
		WithDefaultModel("gpt-4o-mini"),
		WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"id": "plain-text-reply",
						"choices": [{
							"finish_reason": "stop",
							"message": {
								"role": "assistant",
								"content": "This is a plain text response."
							}
						}],
						"usage": {
							"prompt_tokens": 5,
							"completion_tokens": 3,
							"total_tokens": 8
						}
					}`)),
				}, nil
			}),
		}),
	)

	reply, err := gateway.GenerateStructuredReply(
		context.Background(),
		[]Message{{Role: "user", Content: "Hello"}},
		ConversationConfig{ModelID: "gpt-4o-mini"},
		nil,
	)
	if err != nil {
		t.Fatalf("GenerateStructuredReply failed: %v", err)
	}
	if reply.Content != "This is a plain text response." {
		t.Fatalf("expected plain text content, got %q", reply.Content)
	}
	if reply.FinishReason != "stop" {
		t.Fatalf("expected finish_reason stop, got %q", reply.FinishReason)
	}
	if len(reply.ToolCalls) != 0 {
		t.Fatalf("expected 0 tool calls, got %d", len(reply.ToolCalls))
	}
	if reply.Usage == nil || reply.Usage.TotalTokens != 8 {
		t.Fatalf("expected usage tokens to be parsed, got %+v", reply.Usage)
	}
	if !reply.Usage.RecordedByRelay {
		t.Fatalf("expected relay parsed usage to be marked authoritative, got %+v", reply.Usage)
	}
}

func TestRelayGateway_ForwardsInternalRelayHeaders(t *testing.T) {
	var gotUserID string
	var gotWorkspaceID string
	var gotOrganizationID string
	var gotRequestID string
	var gotInternalAuth string
	var gotUserGroup string
	var gotConversationID string
	var gotFeatureType string
	var gotCanonicalFeatureType string

	gateway := NewRelayGateway(
		WithRelayURL("http://relay.test/v1"),
		WithDefaultModel("gpt-4o-mini"),
		WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotUserID = r.Header.Get("X-Oblivious-Internal-User-ID")
				gotWorkspaceID = r.Header.Get("X-Oblivious-Internal-Workspace-ID")
				gotOrganizationID = r.Header.Get("X-Oblivious-Internal-Organization-ID")
				gotRequestID = r.Header.Get("X-Request-ID")
				gotInternalAuth = r.Header.Get("X-Oblivious-Internal-Auth")
				gotUserGroup = r.Header.Get("X-Oblivious-Internal-User-Group")
				gotConversationID = r.Header.Get("X-Oblivious-Internal-Conversation-ID")
				gotFeatureType = r.Header.Get("X-Oblivious-Internal-Feature-Type")
				gotCanonicalFeatureType = r.Header.Get(relaytypes.HeaderInternalFeatureType)

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{
						"id": "reply",
						"choices": [{
							"message": {
								"role": "assistant",
								"content": "ok"
							}
						}]
					}`)),
				}, nil
			}),
		}),
	)

	_, err := gateway.GenerateReply(
		WithRelayRequestMetadata(context.Background(), RelayRequestMetadata{
			UserID:         "user_1",
			WorkspaceID:    "workspace_1",
			OrganizationID: "org_1",
			RequestID:      "req_123",
			UserGroup:      "vip",
			FeatureType:    "workflow",
		}),
		[]Message{{Role: "user", Content: "hello"}},
		ConversationConfig{ConversationID: "conversation_1", ModelID: "gpt-4o-mini"},
	)
	if err != nil {
		t.Fatalf("GenerateReply failed: %v", err)
	}

	if gotUserID != "user_1" {
		t.Fatalf("expected user header user_1, got %q", gotUserID)
	}
	if gotWorkspaceID != "workspace_1" {
		t.Fatalf("expected workspace header workspace_1, got %q", gotWorkspaceID)
	}
	if gotOrganizationID != "org_1" {
		t.Fatalf("expected organization header org_1, got %q", gotOrganizationID)
	}
	if gotRequestID != "req_123" {
		t.Fatalf("expected request id req_123, got %q", gotRequestID)
	}
	if gotInternalAuth == "" {
		t.Fatalf("expected internal auth header to be set")
	}
	if gotUserGroup != "vip" {
		t.Fatalf("expected user group header vip, got %q", gotUserGroup)
	}
	if gotConversationID != "conversation_1" {
		t.Fatalf("expected conversation header conversation_1, got %q", gotConversationID)
	}
	if gotFeatureType != "workflow" {
		t.Fatalf("expected feature type header workflow, got %q", gotFeatureType)
	}
	if gotCanonicalFeatureType != "workflow" {
		t.Fatalf("expected canonical feature type header workflow, got %q", gotCanonicalFeatureType)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// mockReplyGenerator for testing
type mockReplyGenerator struct {
	reply string
	err   error
	calls int
}

func (m *mockReplyGenerator) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	return m.reply, nil
}
