package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/quota"
	serverrelay "oblivious/server/internal/relay"
	relaytypes "oblivious/server/internal/relay/types"
)

func TestCombineHandlersRelayAliasesRouteToOpenAICompatiblePaths(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		aliasPath  string
		targetPath string
		body       string
	}{
		{
			name:       "chat completions",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/chat/completions?trace=1",
			targetPath: "/v1/chat/completions",
			body:       `{"model":"gpt-4o","messages":[]}`,
		},
		{
			name:       "embeddings",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/embeddings",
			targetPath: "/v1/embeddings",
			body:       `{"model":"text-embedding-3-small","input":"ping"}`,
		},
		{
			name:       "responses",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/responses",
			targetPath: "/v1/responses",
			body:       `{"model":"gpt-4o","input":"ping"}`,
		},
		{
			name:       "image generations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/generations",
			targetPath: "/v1/images/generations",
			body:       `{"model":"gpt-image-1","prompt":"ping"}`,
		},
		{
			name:       "image edits",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/edits",
			targetPath: "/v1/images/edits",
			body:       `image-edit-bytes`,
		},
		{
			name:       "image variations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/images/variations",
			targetPath: "/v1/images/variations",
			body:       `image-variation-bytes`,
		},
		{
			name:       "audio speech",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/speech",
			targetPath: "/v1/audio/speech",
			body:       `{"model":"tts-1","input":"ping","voice":"alloy"}`,
		},
		{
			name:       "audio transcriptions",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/transcriptions",
			targetPath: "/v1/audio/transcriptions",
			body:       `audio-bytes`,
		},
		{
			name:       "audio translations",
			method:     stdhttp.MethodPost,
			aliasPath:  "/api/v1/relay/audio/translations",
			targetPath: "/v1/audio/translations",
			body:       `audio-bytes`,
		},
		{
			name:       "models",
			method:     stdhttp.MethodGet,
			aliasPath:  "/api/v1/relay/models?limit=20",
			targetPath: "/v1/models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relayCalled := false
			main := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				t.Fatalf("alias route reached main handler as %s %s", r.Method, r.URL.String())
			})
			relay := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				relayCalled = true
				if r.Method != tt.method {
					t.Fatalf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.targetPath {
					t.Fatalf("relay path = %q, want %q", r.URL.Path, tt.targetPath)
				}
				if r.URL.RawQuery != httptest.NewRequest(tt.method, tt.aliasPath, nil).URL.RawQuery {
					t.Fatalf("relay query = %q, want query from %q", r.URL.RawQuery, tt.aliasPath)
				}
				if r.Header.Get("X-Relay-Alias-Test") != "preserved" {
					t.Fatalf("expected relay alias request header to be preserved")
				}
				w.WriteHeader(stdhttp.StatusAccepted)
			})
			handler := combineHandlers(main, relay)
			request := httptest.NewRequest(tt.method, tt.aliasPath, strings.NewReader(tt.body))
			request.Header.Set("X-Relay-Alias-Test", "preserved")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusAccepted {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, stdhttp.StatusAccepted, recorder.Body.String())
			}
			if !relayCalled {
				t.Fatal("expected relay handler to be called")
			}
		})
	}
}

func TestRelayChatRequestLogJoinsUsageLedgerByRequestID(t *testing.T) {
	t.Setenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN", "test-internal-token")

	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl_test","choices":[{"message":{"role":"assistant","content":"pong"}}],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20}}`))
	}))
	defer upstream.Close()

	pool := serverrelay.NewChannelPool()
	pool.AddChannel(&relaytypes.Channel{
		ID:             "ch_request_log_join",
		OrganizationID: "org_join",
		Name:           "Request Log Join",
		Provider:       "openai",
		BaseURL:        upstream.URL,
		APIKey:         "sk-test",
		Models:         []string{"gpt-4o-mini"},
		CBThreshold:    5,
		Enabled:        true,
	}, 100)
	pricing := serverrelay.NewPricingStore()
	pricing.SetPrice("gpt-4o-mini", relaytypes.APITypeChat, relaytypes.DimPromptTokens, 0.01)
	pricing.SetPrice("gpt-4o-mini", relaytypes.APITypeChat, relaytypes.DimCompletionTokens, 0.01)
	pricing.SetPrice("gpt-4o-mini", relaytypes.APITypeChat, relaytypes.DimTotalTokens, 0.01)
	relayInstance, err := serverrelay.NewRelay(&serverrelay.Config{
		Pool:         pool,
		Production:   true,
		PricingStore: pricing,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	usageLogger := &requestLogJoinUsageLogger{}
	quotaManager := &requestLogJoinQuotaManager{}
	tokenQuotaManager := &requestLogJoinAPITokenQuotaManager{}
	relayInstance.Router().SetUsageLogger(usageLogger)
	relayInstance.Router().SetQuotaManager(quotaManager)
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuotaManager)

	sink := &captureMiddlewareRequestLogSink{}
	restoreSink := setRequestLogSinkForTest(sink)
	defer restoreSink()

	handler := withRequestID(withLogging(relayInstance.Engine()))
	requestBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`
	request := httptest.NewRequest(stdhttp.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(relaytypes.HeaderInternalAuth, "test-internal-token")
	request.Header.Set(relaytypes.HeaderInternalUserID, "user_join")
	request.Header.Set(relaytypes.HeaderInternalOrganization, "org_join")
	request.Header.Set(relaytypes.HeaderInternalFeatureType, "chat")
	request.Header.Set(relaytypes.HeaderRequestID, "req_join_live")
	request.Header.Set("Idempotency-Key", "idem_join_live")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected relay success, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected one usage ledger record, got %+v", usageLogger.records)
	}
	usage := usageLogger.records[0]
	if usage.RequestID != "req_join_live" ||
		usage.OrganizationID != "org_join" ||
		usage.UserID != "user_join" ||
		usage.ChannelID != "ch_request_log_join" ||
		usage.Provider != "openai" ||
		usage.TotalTokens != 20 ||
		usage.Cost != 0.2 ||
		usage.Status != serverrelay.RelayUsageStatusSuccess {
		t.Fatalf("unexpected usage ledger record: %+v", usage)
	}
	if quotaManager.settleCalls != 1 || tokenQuotaManager.settleCalls != 0 {
		t.Fatalf("expected org quota settlement only, quota=%+v token=%+v", quotaManager, tokenQuotaManager)
	}
	if len(sink.rows) != 1 {
		t.Fatalf("expected one HTTP request log row, got %+v", sink.rows)
	}
	row := sink.rows[0]
	if row.RequestID != usage.RequestID ||
		row.OrganizationID != "00000000-0000-0000-0000-000000000000" ||
		row.UserID != "00000000-0000-0000-0000-000000000000" ||
		row.Service != "relay" ||
		row.Endpoint != "/v1/chat/completions" ||
		row.Model != "gpt-4o-mini" ||
		row.RequestTokens != uint32(usage.PromptTokens) ||
		row.ResponseTokens != uint32(usage.CompletionTokens) ||
		row.CostUSD != usage.Cost {
		t.Fatalf("request log row does not join usage ledger: row=%+v usage=%+v", row, usage)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		t.Fatalf("request log metadata should be JSON: %v\n%s", err, row.Metadata)
	}
	for key, want := range map[string]any{
		"provider":              "openai",
		"channel_id":            "ch_request_log_join",
		"billing_session_id":    "bill_join",
		"relay_usage_status":    "success",
		"relay_organization_id": "org_join",
		"relay_user_id":         "user_join",
		"preauthorized_amount":  quotaManager.preconsumeAmount,
		"total_tokens":          float64(20),
	} {
		if metadata[key] != want {
			t.Fatalf("metadata[%s] = %#v, want %#v; metadata=%+v", key, metadata[key], want, metadata)
		}
	}
	if _, ok := metadata["token_preauthorized_amount"]; ok {
		t.Fatalf("internal trusted Relay request should not record API token preauthorization metadata: %+v", metadata)
	}
}

func TestCombineHandlersRelayAliasesReachProductionRelayPolicy(t *testing.T) {
	relayInstance, err := serverrelay.NewRelay(&serverrelay.Config{
		Production:   true,
		PricingStore: serverrelay.NewPricingStoreWithDefaults(),
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	handler := combineHandlers(stdhttp.NotFoundHandler(), relayInstance.Engine())

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "chat completions", method: stdhttp.MethodPost, path: "/api/v1/relay/chat/completions", body: `{"model":"gpt-4o","messages":[]}`},
		{name: "embeddings", method: stdhttp.MethodPost, path: "/api/v1/relay/embeddings", body: `{"model":"text-embedding-3-small","input":"ping"}`},
		{name: "responses", method: stdhttp.MethodPost, path: "/api/v1/relay/responses", body: `{"model":"gpt-4o","input":"ping"}`},
		{name: "image generations", method: stdhttp.MethodPost, path: "/api/v1/relay/images/generations", body: `{"model":"gpt-image-1","prompt":"ping"}`},
		{name: "image edits", method: stdhttp.MethodPost, path: "/api/v1/relay/images/edits", body: `image-edit-bytes`},
		{name: "image variations", method: stdhttp.MethodPost, path: "/api/v1/relay/images/variations", body: `image-variation-bytes`},
		{name: "audio speech", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/speech", body: `{"model":"tts-1","input":"ping","voice":"alloy"}`},
		{name: "audio transcriptions", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/transcriptions", body: `audio-bytes`},
		{name: "audio translations", method: stdhttp.MethodPost, path: "/api/v1/relay/audio/translations", body: `audio-bytes`},
		{name: "models", method: stdhttp.MethodGet, path: "/api/v1/relay/models"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if strings.HasPrefix(strings.TrimSpace(tt.body), "{") {
				request.Header.Set("Content-Type", "application/json")
			}

			handler.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("status = %d, want %d from relay policy; body=%s", recorder.Code, stdhttp.StatusUnauthorized, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "relay_identity_required") {
				t.Fatalf("expected relay policy response, got %s", recorder.Body.String())
			}
		})
	}
}

type requestLogJoinUsageLogger struct {
	records []serverrelay.RelayUsageLogRecord
}

func (l *requestLogJoinUsageLogger) RecordRelayUsage(_ context.Context, record serverrelay.RelayUsageLogRecord) error {
	l.records = append(l.records, record)
	return nil
}

func (l *requestLogJoinUsageLogger) ReplaceRelayUsage(_ context.Context, record serverrelay.RelayUsageLogRecord) error {
	for index := range l.records {
		if l.records[index].RequestID == record.RequestID {
			l.records[index] = record
			return nil
		}
	}
	l.records = append(l.records, record)
	return nil
}

type requestLogJoinQuotaManager struct {
	preconsumeAmount float64
	settleCalls      int
	settledAmount    float64
}

func (m *requestLogJoinQuotaManager) PreConsume(_ context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error) {
	m.preconsumeAmount = amount
	return &quota.BillingSession{
		ID:               "bill_join",
		UserID:           userID,
		OrganizationID:   organizationID,
		ChannelID:        channelID,
		Model:            model,
		APIType:          apiType,
		IdempotencyKey:   idempotencyKey,
		PreAuthorizedAmt: amount,
		Status:           "preauthorized",
		CreatedAt:        time.Now().UTC(),
	}, nil
}

func (m *requestLogJoinQuotaManager) Settle(_ context.Context, _ string, _ string, actualAmount float64) error {
	m.settleCalls++
	m.settledAmount = actualAmount
	return nil
}

func (m *requestLogJoinQuotaManager) Refund(context.Context, string, string) error {
	return nil
}

type requestLogJoinAPITokenQuotaManager struct {
	preauthorizedAmount float64
	settleCalls         int
}

func (m *requestLogJoinAPITokenQuotaManager) PreAuthorizeRelayAPITokenQuota(_ context.Context, _ string, amount float64) error {
	m.preauthorizedAmount = amount
	return nil
}

func (m *requestLogJoinAPITokenQuotaManager) SettleRelayAPITokenQuota(context.Context, string, float64, float64) error {
	m.settleCalls++
	return nil
}

func (m *requestLogJoinAPITokenQuotaManager) RefundRelayAPITokenQuota(context.Context, string, float64) error {
	return nil
}

func (m *requestLogJoinAPITokenQuotaManager) RefundRelayAPITokenQuotaOnce(context.Context, string, float64, string) error {
	return nil
}

func TestCombineHandlersDoesNotBroadenRelayAliasesToUnsupportedSurfaces(t *testing.T) {
	tests := []struct {
		method string
		path   string
	}{
		{method: stdhttp.MethodPost, path: "/api/v1/relay/files"},
		{method: stdhttp.MethodPost, path: "/api/v1/relay/fine_tuning/jobs"},
		{method: stdhttp.MethodPost, path: "/api/v1/relay/assistants"},
		{method: stdhttp.MethodGet, path: "/api/v1/relay/realtime"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			mainCalled := false
			main := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				mainCalled = true
				w.WriteHeader(stdhttp.StatusNotFound)
			})
			relay := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				t.Fatalf("unsupported alias was routed to relay as %s %s", r.Method, r.URL.String())
			})
			handler := combineHandlers(main, relay)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			handler.ServeHTTP(recorder, request)

			if !mainCalled {
				t.Fatal("expected unsupported alias to remain with main handler")
			}
			if recorder.Code != stdhttp.StatusNotFound {
				t.Fatalf("status = %d, want %d", recorder.Code, stdhttp.StatusNotFound)
			}
		})
	}
}
