package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/observability"
	"oblivious/server/internal/secretbox"
)

func TestObservabilityAlertAdminRouteGetsDefaultRoutingRules(t *testing.T) {
	routingStore := observability.NewInMemoryAlertRoutingRuleStore(observability.DefaultAlertRoutingRules())
	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(observability.NewInMemoryAlertStateStore(), routingStore))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alert-routing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected alert routing route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data observability.AlertRoutingRules `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expected := observability.AlertRoutingRules{
		observability.AlertSeverityDebug:    nil,
		observability.AlertSeverityInfo:     {observability.AlertDeliveryChannelEmail},
		observability.AlertSeverityWarning:  {observability.AlertDeliveryChannelEmail, observability.AlertDeliveryChannelIM},
		observability.AlertSeverityCritical: {observability.AlertDeliveryChannelEmail, observability.AlertDeliveryChannelIM, observability.AlertDeliveryChannelSMS, observability.AlertDeliveryChannelThirdParty, observability.AlertDeliveryChannelPhone},
	}
	if !sameAlertRoutingRules(response.Data, expected) {
		t.Fatalf("expected default routing rules %+v, got %+v", expected, response.Data)
	}

	var rawEnvelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &rawEnvelope); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if string(rawEnvelope.Data["debug"]) != "[]" {
		t.Fatalf("expected debug routing channels to render as [], got %s", rawEnvelope.Data["debug"])
	}
}

func TestObservabilityAlertAdminRouteUpdatesRoutingRules(t *testing.T) {
	routingStore := observability.NewInMemoryAlertRoutingRuleStore(observability.DefaultAlertRoutingRules())
	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(observability.NewInMemoryAlertStateStore(), routingStore))
	body := bytes.NewBufferString(`{
		"rules": {
			"debug": [],
			"info": ["email"],
			"warning": ["im"],
			"critical": ["email", "im", "sms", "third_party", "phone"]
		}
	}`)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/observability/alert-routing", body)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected update route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data observability.AlertRoutingRules `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !sameAlertRoutingRules(response.Data, observability.AlertRoutingRules{
		observability.AlertSeverityDebug:    nil,
		observability.AlertSeverityInfo:     {observability.AlertDeliveryChannelEmail},
		observability.AlertSeverityWarning:  {observability.AlertDeliveryChannelIM},
		observability.AlertSeverityCritical: {observability.AlertDeliveryChannelEmail, observability.AlertDeliveryChannelIM, observability.AlertDeliveryChannelSMS, observability.AlertDeliveryChannelThirdParty, observability.AlertDeliveryChannelPhone},
	}) {
		t.Fatalf("expected updated routing rules, got %+v", response.Data)
	}

	stored, err := routingStore.GetRoutingRules(context.Background())
	if err != nil {
		t.Fatalf("get stored routing rules: %v", err)
	}
	if !sameAlertRoutingRules(stored, response.Data) {
		t.Fatalf("expected HTTP update to persist in routing store, stored=%+v response=%+v", stored, response.Data)
	}
}

func TestObservabilityLatencySLOProofAggregatesRuntimeDeliveryAndRecovery(t *testing.T) {
	ctx := context.Background()
	store := observability.NewInMemoryAlertStateStore()
	alertKey := "http-slo:/api/v1/app/conversations/:id/messages:latency"
	occurredAt := time.Date(2026, 6, 16, 0, 30, 0, 0, time.UTC)
	event := observability.AlertEvent{
		Key:        alertKey,
		Severity:   observability.AlertSeverityWarning,
		Title:      "HTTP latency SLO breached",
		Component:  "http",
		OccurredAt: occurredAt,
	}
	if _, err := store.RecordAlertOpen(ctx, event); err != nil {
		t.Fatalf("seed alert state: %v", err)
	}
	if err := store.RecordDeliveryAttempts(ctx, event, []observability.AlertDeliveryResult{{
		Channel:      observability.AlertDeliveryChannelEmail,
		ProviderID:   "smtp-primary",
		ProviderKind: observability.AlertProviderKindSMTP,
		Delivered:    true,
	}}); err != nil {
		t.Fatalf("seed delivery attempt: %v", err)
	}
	if _, _, err := store.RecordRecoveryAction(ctx, observability.RecoveryAction{
		ID:         "recovery_20260616_0001",
		PolicyName: "http-latency-slo",
		AlertKey:   alertKey,
		Severity:   observability.AlertSeverityWarning,
		Component:  "http",
		Type:       observability.RecoveryActionScaleOut,
		Status:     observability.RecoveryActionRecorded,
		Reason:     "audit-only latency SLO recovery plan; no infrastructure mutation executed",
		Attempt:    1,
		CreatedAt:  occurredAt.Add(time.Minute),
	}, 0); err != nil {
		t.Fatalf("seed recovery action: %v", err)
	}
	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(
		stdhttp.MethodGet,
		"/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00:00:00Z&to=2026-06-16T01:00:00Z&keyPrefix=http-slo:",
		nil,
	)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected latency SLO proof route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			LatencySLOTrigger        string `json:"latencySLOTrigger"`
			LatencySLOAlertDelivery  string `json:"latencySLOAlertDelivery"`
			LatencySLORecoveryAction string `json:"latencySLORecoveryAction"`
			Window                   string `json:"window"`
			TriggeredAlerts          int    `json:"triggeredAlerts"`
			AlertDelivery            struct {
				ConfiguredProviders int      `json:"configuredProviders"`
				DeliveredAlerts     int      `json:"deliveredAlerts"`
				FailedDeliveries    int      `json:"failedDeliveries"`
				Channels            []string `json:"channels"`
				LastDeliveryID      string   `json:"lastDeliveryId"`
			} `json:"alertDelivery"`
			RecoveryAudit struct {
				AuditRecords  int    `json:"auditRecords"`
				FailedActions int    `json:"failedActions"`
				LastRecordID  string `json:"lastRecordId"`
			} `json:"recoveryAudit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.LatencySLOTrigger != "pass" || response.Data.LatencySLOAlertDelivery != "pass" || response.Data.LatencySLORecoveryAction != "pass" {
		t.Fatalf("expected pass latency SLO proof, got %+v", response.Data)
	}
	if response.Data.Window != "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z" || response.Data.TriggeredAlerts != 1 {
		t.Fatalf("expected one alert in requested window, got %+v", response.Data)
	}
	if response.Data.AlertDelivery.ConfiguredProviders != 1 || response.Data.AlertDelivery.DeliveredAlerts != 1 || response.Data.AlertDelivery.FailedDeliveries != 0 {
		t.Fatalf("expected successful delivery proof, got %+v", response.Data.AlertDelivery)
	}
	if len(response.Data.AlertDelivery.Channels) != 1 || response.Data.AlertDelivery.Channels[0] != string(observability.AlertDeliveryChannelEmail) || response.Data.AlertDelivery.LastDeliveryID == "" {
		t.Fatalf("expected email delivery channel and id, got %+v", response.Data.AlertDelivery)
	}
	if response.Data.RecoveryAudit.AuditRecords != 1 || response.Data.RecoveryAudit.FailedActions != 0 || response.Data.RecoveryAudit.LastRecordID != "recovery_20260616_0001" {
		t.Fatalf("expected recovery audit proof, got %+v", response.Data.RecoveryAudit)
	}
}

func TestObservabilityAlertAdminRouteCreatesAndListsRedactedProviderConfig(t *testing.T) {
	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(observability.NewInMemoryAlertStateStore()))
	body := bytes.NewBufferString(`{
		"kind": "smtp",
		"name": "Primary SMTP",
		"status": "active",
		"config": {
			"smtp_host": "smtp.example.com",
			"smtp_port": "587",
			"username": "alerts@example.com",
			"password": "smtp-secret",
			"from_email": "alerts@example.com",
			"recipients": "ops@example.com,oncall@example.com"
		}
	}`)
	createRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers", body)
	createRecorder := httptest.NewRecorder()

	router.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected provider create route to return 201, got %d with body %s", createRecorder.Code, createRecorder.Body.String())
	}
	if strings.Contains(createRecorder.Body.String(), "smtp-secret") {
		t.Fatalf("expected provider create response to redact secrets, got %s", createRecorder.Body.String())
	}
	var createResponse struct {
		Data observability.AlertProviderConfigView `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResponse.Data.ID == "" {
		t.Fatal("expected created provider to include an id")
	}
	if createResponse.Data.Kind != observability.AlertProviderKindSMTP || createResponse.Data.Channel != observability.AlertDeliveryChannelEmail {
		t.Fatalf("expected smtp provider to map to email channel, got %+v", createResponse.Data)
	}
	if createResponse.Data.Config["password"] != observability.RedactedAlertProviderSecret {
		t.Fatalf("expected password to be redacted, got config %+v", createResponse.Data.Config)
	}

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alert-providers", nil)
	listRecorder := httptest.NewRecorder()

	router.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider list route to return 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	if strings.Contains(listRecorder.Body.String(), "smtp-secret") {
		t.Fatalf("expected provider list response to redact secrets, got %s", listRecorder.Body.String())
	}
	var listResponse struct {
		Data []observability.AlertProviderConfigView `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResponse.Data) != 1 || listResponse.Data[0].ID != createResponse.Data.ID {
		t.Fatalf("expected created provider to be listed, got %+v", listResponse.Data)
	}
	if listResponse.Data[0].Config["password"] != observability.RedactedAlertProviderSecret {
		t.Fatalf("expected listed provider password to be redacted, got %+v", listResponse.Data[0].Config)
	}
}

func TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted(t *testing.T) {
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", "test-observability-alert-provider-secret")
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "observability-provider-redaction@example.com")
	promoteHTTPUserToAdmin(t, database, userID)
	var gotSlackTestPath string
	slackUpstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		gotSlackTestPath = r.URL.Path
		w.WriteHeader(stdhttp.StatusOK)
	}))
	t.Cleanup(slackUpstream.Close)

	type providerSeed struct {
		kind    observability.AlertProviderKind
		name    string
		config  map[string]string
		secrets map[string]string
	}
	seeds := []providerSeed{
		{
			kind: observability.AlertProviderKindSMTP,
			name: "Primary SMTP",
			config: map[string]string{
				"smtp_host":  "smtp.example.com",
				"smtp_port":  "587",
				"username":   "alerts@example.com",
				"password":   "smtp-secret",
				"from_email": "alerts@example.com",
				"recipients": "ops@example.com",
			},
			secrets: map[string]string{"password": "smtp-secret"},
		},
		{
			kind: observability.AlertProviderKindSlackWebhook,
			name: "Slack Ops",
			config: map[string]string{
				"webhook_url": slackUpstream.URL + "/slack-secret",
			},
			secrets: map[string]string{"webhook_url": slackUpstream.URL + "/slack-secret"},
		},
		{
			kind: observability.AlertProviderKindPagerDuty,
			name: "PagerDuty Ops",
			config: map[string]string{
				"routing_key": "pd-routing-key-secret",
			},
			secrets: map[string]string{"routing_key": "pd-routing-key-secret"},
		},
		{
			kind: observability.AlertProviderKindOpsgenie,
			name: "Opsgenie Ops",
			config: map[string]string{
				"api_key":     "opsgenie-api-secret",
				"private_key": "opsgenie-private-secret",
			},
			secrets: map[string]string{
				"api_key":     "opsgenie-api-secret",
				"private_key": "opsgenie-private-secret",
			},
		},
	}

	createdIDsByName := map[string]string{}
	allSecrets := []string{}
	for _, seed := range seeds {
		payload, err := json.Marshal(map[string]any{
			"kind":   seed.kind,
			"name":   seed.name,
			"status": observability.AlertProviderStatusActive,
			"config": seed.config,
		})
		if err != nil {
			t.Fatalf("marshal provider payload: %v", err)
		}
		request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers", bytes.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		addCSRF(request, csrfToken)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		if recorder.Code != stdhttp.StatusCreated {
			t.Fatalf("create provider %s expected 201, got %d with body %s", seed.name, recorder.Code, recorder.Body.String())
		}
		assertResponseDoesNotExposeAlertProviderSecrets(t, recorder.Body.String(), seed.secrets)

		var response struct {
			Data observability.AlertProviderConfigView `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode create response for %s: %v", seed.name, err)
		}
		if response.Data.ID == "" || response.Data.Name != seed.name {
			t.Fatalf("expected created provider identity for %s, got %+v", seed.name, response.Data)
		}
		createdIDsByName[seed.name] = response.Data.ID
		for key, secret := range seed.secrets {
			allSecrets = append(allSecrets, secret)
			if response.Data.Config[key] != observability.RedactedAlertProviderSecret {
				t.Fatalf("expected create response to redact %s for %s, got %+v", key, seed.name, response.Data.Config)
			}
			assertSQLAlertProviderConfigValue(t, database, response.Data.ID, key, secret)
		}
	}

	slackID := createdIDsByName["Slack Ops"]
	testRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers/"+slackID+"/test", nil)
	testRequest.AddCookie(cookie)
	addCSRF(testRequest, csrfToken)
	testRecorder := httptest.NewRecorder()
	router.ServeHTTP(testRecorder, testRequest)
	if testRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("test provider expected 200, got %d with body %s", testRecorder.Code, testRecorder.Body.String())
	}
	if gotSlackTestPath != "/slack-secret" {
		t.Fatalf("expected SQL-backed provider test to use decrypted webhook URL path, got %q", gotSlackTestPath)
	}

	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alert-providers", nil)
	listRequest.AddCookie(cookie)
	listRecorder := httptest.NewRecorder()

	router.ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list providers expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	for _, secret := range allSecrets {
		if strings.Contains(listRecorder.Body.String(), secret) {
			t.Fatalf("expected list response to redact %q, got %s", secret, listRecorder.Body.String())
		}
	}
	if strings.Contains(listRecorder.Body.String(), "api_key_encrypted") {
		t.Fatalf("expected list response to omit encrypted secret columns, got %s", listRecorder.Body.String())
	}

	smtpID := createdIDsByName["Primary SMTP"]
	updatePayload := []byte(`{
		"kind": "smtp",
		"name": "Primary SMTP EU",
		"status": "active",
		"config": {
			"smtp_host": "smtp.eu.example.com",
			"smtp_port": "587",
			"username": "alerts-eu@example.com",
			"password": "********",
			"from_email": "alerts-eu@example.com",
			"recipients": "ops-eu@example.com"
		}
	}`)
	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/observability/alert-providers/"+smtpID, bytes.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	addCSRF(updateRequest, csrfToken)
	updateRecorder := httptest.NewRecorder()

	router.ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update provider expected 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	assertResponseDoesNotExposeAlertProviderSecrets(t, updateRecorder.Body.String(), map[string]string{"password": "smtp-secret"})
	var updateResponse struct {
		Data observability.AlertProviderConfigView `json:"data"`
	}
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updateResponse); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updateResponse.Data.Name != "Primary SMTP EU" || updateResponse.Data.Config["password"] != observability.RedactedAlertProviderSecret {
		t.Fatalf("expected update response to redact preserved password, got %+v", updateResponse.Data)
	}
	assertSQLAlertProviderConfigValue(t, database, smtpID, "password", "smtp-secret")
	assertSQLAlertProviderConfigValue(t, database, smtpID, "smtp_host", "smtp.eu.example.com")
}

func TestObservabilityAlertAdminRouteUpdatesProviderConfigWithoutReplacingRedactedSecret(t *testing.T) {
	providerStore := observability.NewInMemoryAlertProviderConfigStore()
	router := newObservabilityAlertRouter(
		passAdminMiddleware{},
		newObservabilityAlertHandlerWithStores(observability.NewInMemoryAlertStateStore(), nil, providerStore),
	)
	created, err := providerStore.SaveAlertProviderConfig(context.Background(), observability.AlertProviderConfig{
		ID:     "alert_provider_1",
		Kind:   observability.AlertProviderKindSMTP,
		Name:   "Primary SMTP",
		Status: observability.AlertProviderStatusActive,
		Config: map[string]string{
			"smtp_host":  "smtp.example.com",
			"smtp_port":  "587",
			"username":   "alerts@example.com",
			"password":   "smtp-secret",
			"from_email": "alerts@example.com",
			"recipients": "ops@example.com,oncall@example.com",
		},
	})
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	body := bytes.NewBufferString(`{
		"kind": "smtp",
		"name": "Primary SMTP EU",
		"status": "active",
		"config": {
			"smtp_host": "smtp.eu.example.com",
			"smtp_port": "587",
			"username": "alerts-eu@example.com",
			"password": "********",
			"from_email": "alerts-eu@example.com",
			"recipients": "ops-eu@example.com"
		}
	}`)
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/observability/alert-providers/"+created.ID, body)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider update route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "smtp-secret") {
		t.Fatalf("expected provider update response to redact secrets, got %s", recorder.Body.String())
	}
	var response struct {
		Data observability.AlertProviderConfigView `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if response.Data.Name != "Primary SMTP EU" || response.Data.Config["password"] != observability.RedactedAlertProviderSecret {
		t.Fatalf("expected updated redacted provider response, got %+v", response.Data)
	}

	stored, ok, err := providerStore.GetAlertProviderConfig(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get stored provider: %v", err)
	}
	if !ok {
		t.Fatal("expected provider to remain stored")
	}
	if stored.Config["password"] != "smtp-secret" {
		t.Fatalf("expected redacted update to preserve stored password, got %+v", stored.Config)
	}
	if stored.Config["smtp_host"] != "smtp.eu.example.com" {
		t.Fatalf("expected update to replace non-secret fields, got %+v", stored.Config)
	}
}

func assertResponseDoesNotExposeAlertProviderSecrets(t *testing.T, body string, secrets map[string]string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(body, secret) {
			t.Fatalf("expected alert provider response to redact %q, got %s", secret, body)
		}
	}
	if strings.Contains(body, "api_key_encrypted") {
		t.Fatalf("expected alert provider response to omit encrypted secret columns, got %s", body)
	}
}

func assertSQLAlertProviderConfigValue(t *testing.T, database interface {
	QueryRow(query string, args ...any) *sql.Row
}, providerID, key, want string) {
	t.Helper()

	var got string
	if err := database.QueryRow(`SELECT config ->> $2 FROM observability_alert_provider_configs WHERE id = $1`, providerID, key).Scan(&got); err != nil {
		t.Fatalf("query stored alert provider config %s.%s: %v", providerID, key, err)
	}
	if got != want {
		if observability.IsAlertProviderSecretConfigKey(key) {
			if got == "" {
				t.Fatalf("expected stored alert provider config %s.%s to be non-empty", providerID, key)
			}
			if got == want || strings.Contains(got, want) {
				t.Fatalf("stored alert provider config %s.%s contains plaintext %q: %q", providerID, key, want, got)
			}
			if !secretbox.IsProtected(got) {
				t.Fatalf("expected stored alert provider config %s.%s to be protected, got %q", providerID, key, got)
			}
			opened, err := secretbox.Open(secretbox.DomainObservabilityAlertProviderConfigKey, got)
			if err != nil {
				t.Fatalf("open stored alert provider config %s.%s: %v", providerID, key, err)
			}
			if opened != want {
				t.Fatalf("opened alert provider config %s.%s=%q, want %q", providerID, key, opened, want)
			}
			return
		}
		t.Fatalf("expected stored alert provider config %s.%s=%q, got %q", providerID, key, want, got)
	}
	if observability.IsAlertProviderSecretConfigKey(key) {
		t.Fatalf("expected stored alert provider config %s.%s to be protected, got plaintext %q", providerID, key, got)
	}
}

func TestObservabilityAlertAdminRouteTestsProviderConfig(t *testing.T) {
	providerStore := observability.NewInMemoryAlertProviderConfigStore()
	router := newObservabilityAlertRouter(
		passAdminMiddleware{},
		newObservabilityAlertHandlerWithStores(observability.NewInMemoryAlertStateStore(), nil, providerStore),
	)
	var postedPayload map[string]any
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			t.Errorf("expected provider test POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&postedPayload); err != nil {
			t.Errorf("decode provider test payload: %v", err)
		}
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer upstream.Close()
	created, err := providerStore.SaveAlertProviderConfig(context.Background(), observability.AlertProviderConfig{
		ID:     "alert_provider_slack",
		Kind:   observability.AlertProviderKindSlackWebhook,
		Name:   "Slack Ops",
		Status: observability.AlertProviderStatusActive,
		Config: map[string]string{
			"webhook_url": upstream.URL,
		},
	})
	if err != nil {
		t.Fatalf("seed provider: %v", err)
	}
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers/"+created.ID+"/test", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected provider test route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data observability.AlertProviderTestResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode test response: %v", err)
	}
	if !response.Data.OK || response.Data.ProviderID != created.ID || response.Data.Kind != observability.AlertProviderKindSlackWebhook {
		t.Fatalf("expected successful provider test result, got %+v", response.Data)
	}
	text, _ := postedPayload["text"].(string)
	if text == "" || !strings.Contains(text, "Slack Ops") {
		t.Fatalf("expected provider test to post a Slack alert payload, got %+v", postedPayload)
	}
}

func TestObservabilityAlertHandlerUsesInjectedRoutingStore(t *testing.T) {
	routingStore := &recordingAlertRoutingRuleStore{
		rules: observability.AlertRoutingRules{
			observability.AlertSeverityDebug:    {},
			observability.AlertSeverityInfo:     {observability.AlertDeliveryChannelInApp},
			observability.AlertSeverityWarning:  {observability.AlertDeliveryChannelIM},
			observability.AlertSeverityCritical: {observability.AlertDeliveryChannelSMS},
		},
	}
	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(observability.NewInMemoryAlertStateStore(), routingStore))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alert-routing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected injected alert routing route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if routingStore.getCalls != 1 {
		t.Fatalf("expected injected routing store to be called once, got %d", routingStore.getCalls)
	}
	var response struct {
		Data observability.AlertRoutingRules `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !sameAlertRoutingRules(response.Data, routingStore.rules) {
		t.Fatalf("expected injected routing rules %+v, got %+v", routingStore.rules, response.Data)
	}
}

func TestObservabilityAlertAdminRouteRejectsInvalidRoutingRule(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "invalid severity", body: `{"rules":{"notice":["email"]}}`},
		{name: "invalid channel", body: `{"rules":{"warning":["webhook"]}}`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			routingStore := observability.NewInMemoryAlertRoutingRuleStore(observability.DefaultAlertRoutingRules())
			router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(observability.NewInMemoryAlertStateStore(), routingStore))
			request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/observability/alert-routing", bytes.NewBufferString(tt.body))
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected invalid routing rule to return 400, got %d with body %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestObservabilityAlertAdminRouteAcknowledgesAlertState(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	openedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	if _, err := store.RecordAlertOpen(context.Background(), observability.AlertEvent{
		Key:        "workflow-failure-rate",
		Severity:   observability.AlertSeverityCritical,
		Title:      "Workflow execution failure rate > 10%",
		Component:  "workflow",
		OccurredAt: openedAt,
	}); err != nil {
		t.Fatalf("seed alert state: %v", err)
	}

	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/observability/alerts/workflow-failure-rate/acknowledge", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected acknowledge route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data observability.AlertState `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != observability.AlertStatusAcknowledged {
		t.Fatalf("expected acknowledged alert state, got %+v", response.Data)
	}
	if response.Data.Key != "workflow-failure-rate" || response.Data.Component != "workflow" {
		t.Fatalf("expected alert identity to round trip, got %+v", response.Data)
	}
}

func TestObservabilityAlertAdminRouteListsAlertStates(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	openedAt := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	for _, event := range []observability.AlertEvent{
		{
			Key:        "workflow-failure-rate",
			Severity:   observability.AlertSeverityCritical,
			Title:      "Workflow execution failure rate > 10%",
			Component:  "workflow",
			OccurredAt: openedAt,
		},
		{
			Key:        "relay-backlog",
			Severity:   observability.AlertSeverityWarning,
			Title:      "Relay backlog",
			Component:  "relay",
			OccurredAt: openedAt.Add(3 * time.Minute),
		},
	} {
		if _, err := store.RecordAlertOpen(context.Background(), event); err != nil {
			t.Fatalf("seed alert state %s: %v", event.Key, err)
		}
	}

	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alerts?status=open", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected list route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data []observability.AlertState `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("expected two alert states, got %+v", response.Data)
	}
	if response.Data[0].Key != "relay-backlog" || response.Data[1].Key != "workflow-failure-rate" {
		t.Fatalf("expected alerts ordered by last occurrence desc, got %+v", response.Data)
	}
}

func TestObservabilityAlertAdminRouteListsAlertStatesWithCombinedFilters(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	openedAt := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	for _, event := range []observability.AlertEvent{
		{
			Key:        "relay-backlog",
			Severity:   observability.AlertSeverityCritical,
			Title:      "Relay backlog",
			Component:  "relay",
			OccurredAt: openedAt,
		},
		{
			Key:        "relay-latency",
			Severity:   observability.AlertSeverityWarning,
			Title:      "Relay latency",
			Component:  "relay",
			OccurredAt: openedAt.Add(1 * time.Minute),
		},
		{
			Key:        "workflow-failure-rate",
			Severity:   observability.AlertSeverityCritical,
			Title:      "Workflow execution failure rate > 10%",
			Component:  "workflow",
			OccurredAt: openedAt.Add(2 * time.Minute),
		},
	} {
		if _, err := store.RecordAlertOpen(context.Background(), event); err != nil {
			t.Fatalf("seed alert state %s: %v", event.Key, err)
		}
	}
	if _, err := store.AcknowledgeAlert(context.Background(), "relay-backlog", openedAt.Add(3*time.Minute)); err != nil {
		t.Fatalf("acknowledge relay backlog: %v", err)
	}

	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alerts?status=acknowledged&severity=critical&component=relay", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected filtered list route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data []observability.AlertState `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected one filtered alert state, got %+v", response.Data)
	}
	if response.Data[0].Key != "relay-backlog" || response.Data[0].Status != observability.AlertStatusAcknowledged {
		t.Fatalf("expected acknowledged critical relay alert, got %+v", response.Data[0])
	}
}

func TestObservabilityAlertAdminRouteListsDeliveryAttempts(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	at := time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC)
	if err := store.RecordDeliveryAttempts(context.Background(), observability.AlertEvent{
		Key:        "relay-backlog",
		Severity:   observability.AlertSeverityWarning,
		Title:      "Relay backlog",
		Component:  "relay",
		OccurredAt: at,
	}, []observability.AlertDeliveryResult{
		{Channel: observability.AlertDeliveryChannelEmail, Delivered: true},
		{
			Channel:      observability.AlertDeliveryChannelIM,
			ProviderID:   "alert_provider_slack_ops",
			ProviderKind: observability.AlertProviderKindSlackWebhook,
			Err:          context.DeadlineExceeded,
		},
	}); err != nil {
		t.Fatalf("seed delivery attempts: %v", err)
	}

	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/alerts/relay-backlog/deliveries", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected delivery route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []observability.AlertDeliveryAttempt `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("expected two delivery attempts, got %+v", response.Data)
	}
	if response.Data[0].AlertKey != "relay-backlog" || response.Data[0].Channel != observability.AlertDeliveryChannelEmail || !response.Data[0].Delivered {
		t.Fatalf("expected successful email attempt first, got %+v", response.Data[0])
	}
	if response.Data[1].Channel != observability.AlertDeliveryChannelIM || response.Data[1].Error == "" {
		t.Fatalf("expected failed IM attempt second, got %+v", response.Data[1])
	}
	if response.Data[1].ProviderID != "alert_provider_slack_ops" || response.Data[1].ProviderKind != observability.AlertProviderKindSlackWebhook {
		t.Fatalf("expected provider metadata on failed IM attempt, got %+v", response.Data[1])
	}
	var rawResponse struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &rawResponse); err != nil {
		t.Fatalf("decode raw delivery response: %v", err)
	}
	if rawResponse.Data[1]["providerId"] != "alert_provider_slack_ops" || rawResponse.Data[1]["providerKind"] != string(observability.AlertProviderKindSlackWebhook) {
		t.Fatalf("expected lower-camel provider metadata in response, got %+v", rawResponse.Data[1])
	}
	if _, ok := rawResponse.Data[1]["ProviderID"]; ok {
		t.Fatalf("expected response not to expose Go field name ProviderID, got %+v", rawResponse.Data[1])
	}
}

func TestObservabilityAlertAdminRouteListsRecoveryActions(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	createdAt := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
	if _, _, err := store.RecordRecoveryAction(context.Background(), observability.RecoveryAction{
		ID:         "restart-relay:relay-backlog:1",
		PolicyName: "restart-relay",
		AlertKey:   "relay-backlog",
		Severity:   observability.AlertSeverityCritical,
		Component:  "relay",
		Type:       observability.RecoveryActionRestart,
		Status:     observability.RecoveryActionRecorded,
		Reason:     "Relay backlog",
		CreatedAt:  createdAt,
	}, 0); err != nil {
		t.Fatalf("seed relay recovery action: %v", err)
	}
	if _, _, err := store.RecordRecoveryAction(context.Background(), observability.RecoveryAction{
		ID:         "scale-workflow:workflow-failure-rate:1",
		PolicyName: "scale-workflow",
		AlertKey:   "workflow-failure-rate",
		Severity:   observability.AlertSeverityWarning,
		Component:  "workflow",
		Type:       observability.RecoveryActionScaleOut,
		Status:     observability.RecoveryActionRecorded,
		Reason:     "Workflow failures",
		CreatedAt:  createdAt.Add(2 * time.Minute),
	}, 0); err != nil {
		t.Fatalf("seed workflow recovery action: %v", err)
	}

	router := newObservabilityAlertRouter(passAdminMiddleware{}, newObservabilityAlertHandler(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/observability/recovery-actions?alertKey=relay-backlog", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected recovery action route to return 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []observability.RecoveryAction `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected one filtered recovery action, got %+v", response.Data)
	}
	action := response.Data[0]
	if action.AlertKey != "relay-backlog" || action.PolicyName != "restart-relay" || action.Type != observability.RecoveryActionRestart {
		t.Fatalf("expected relay restart recovery action, got %+v", action)
	}
}

type passAdminMiddleware struct{}

func (passAdminMiddleware) requireAdmin(next stdhttp.Handler) stdhttp.Handler {
	return next
}

type recordingAlertRoutingRuleStore struct {
	getCalls    int
	updateCalls int
	rules       observability.AlertRoutingRules
}

func (s *recordingAlertRoutingRuleStore) GetRoutingRules(context.Context) (observability.AlertRoutingRules, error) {
	s.getCalls++
	return s.rules, nil
}

func (s *recordingAlertRoutingRuleStore) UpdateRoutingRules(_ context.Context, rules observability.AlertRoutingRules) (observability.AlertRoutingRules, error) {
	s.updateCalls++
	s.rules = rules
	return rules, nil
}

func sameAlertRoutingRules(a, b observability.AlertRoutingRules) bool {
	if len(a) != len(b) {
		return false
	}
	for severity, channels := range a {
		if !sameHTTPDeliveryChannels(channels, b[severity]) {
			return false
		}
	}
	return true
}

func sameHTTPDeliveryChannels(a, b []observability.AlertDeliveryChannel) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
