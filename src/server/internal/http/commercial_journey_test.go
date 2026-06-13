package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"

	relaytypes "oblivious/server/internal/relay/types"
	stripebilling "oblivious/server/internal/stripe"
)

type commercialRelayRecorder struct {
	mu             sync.Mutex
	chatCalls      int
	embeddingCalls int
	chatOrgs       []string
	embeddingOrgs  []string
}

type commercialCheckoutCreator struct {
	database *sql.DB
	mu       sync.Mutex
	requests []stripebilling.CheckoutSessionRequest
	sessions []string
}

func (c *commercialCheckoutCreator) CreateCheckoutSession(_ context.Context, _ stripebilling.CheckoutConfig, req stripebilling.CheckoutSessionRequest) (*stripeapi.CheckoutSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.database != nil {
		var exists bool
		if err := c.database.QueryRow(`SELECT EXISTS(SELECT 1 FROM payment_intents WHERE id = $1 AND status = 'pending')`, req.PaymentIntentID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("query precreated payment intent: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("payment intent %s was not precreated", req.PaymentIntentID)
		}
	}
	sessionID := fmt.Sprintf("cs_commercial_%02d", len(c.requests)+1)
	c.requests = append(c.requests, req)
	c.sessions = append(c.sessions, sessionID)
	return &stripeapi.CheckoutSession{
		ID:  sessionID,
		URL: "https://checkout.stripe.test/" + sessionID,
	}, nil
}

func (c *commercialCheckoutCreator) requireLast(t *testing.T) (stripebilling.CheckoutSessionRequest, string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) == 0 || len(c.sessions) == 0 {
		t.Fatal("expected checkout creator to be called")
	}
	return c.requests[len(c.requests)-1], c.sessions[len(c.sessions)-1]
}

func TestCommercialHTTPJourney(t *testing.T) {
	database := testDatabase(t)
	applyCommercialJourneyMigrations(t, database)

	const commercialJourneyModel = "commercial-journey-model"
	relayPort, relayRecorder := startCommercialJourneyRelay(t)
	checkoutCreator := &commercialCheckoutCreator{database: database}
	cfg := testConfig()
	cfg.Port = relayPort
	cfg.RelayEnabled = true
	cfg.RelayDefaultModel = commercialJourneyModel
	cfg.ModelDefaultName = commercialJourneyModel
	cfg.StripeWebhookSecret = "whsec_commercial_journey"
	cfg.StripeSuccessURL = "https://app.oblivious.test/commercial/success"
	cfg.StripeCancelURL = "https://app.oblivious.test/commercial/cancel"
	router := NewRouterWithOptions(cfg, database, RouterOptions{CheckoutCreator: checkoutCreator})

	cookie, csrfToken, userID := registerHTTPUser(t, router, "commercial-journey@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)

	t.Run("tenant and provider configuration", func(t *testing.T) {
		body := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/app/organizations", "", cookie, "", stdhttp.StatusOK)
		if !strings.Contains(string(body), organizationID) || !strings.Contains(string(body), "owner") {
			t.Fatalf("expected app organization list to expose owner membership for %s, body=%s", organizationID, body)
		}

		channelID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/admin/channels", `{
			"name":"Commercial Relay Channel",
			"provider":"openai",
			"baseURL":"https://relay-provider.invalid/v1",
			"apiKey":"sk-commercial-placeholder",
			"models":["`+commercialJourneyModel+`"],
			"rpmLimit":120,
			"tpmLimit":120000,
			"priority":10
		}`, stdhttp.StatusCreated)

		routeID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/admin/routes", `{
			"model":"`+commercialJourneyModel+`",
			"strategy":"weighted",
			"channels":[{"channelID":"`+channelID+`","weight":100,"priority":10,"enabled":true}]
		}`, stdhttp.StatusCreated)

		body = commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/admin/routes/"+routeID, "", cookie, "", stdhttp.StatusOK)
		if !strings.Contains(string(body), channelID) || !strings.Contains(string(body), commercialJourneyModel) {
			t.Fatalf("expected admin route inspection to expose channel/model config, body=%s", body)
		}
	})

	planID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/admin/plans", `{
		"name":"Commercial Completion Pro",
		"description":"Commercial journey subscription plan",
		"quotaAmount":100,
		"tokenQuota":1000000,
		"price":29,
		"modelAccess":["`+commercialJourneyModel+`"],
		"agentLimit":10,
		"durationDays":30,
		"isPublic":true,
		"sortOrder":1
	}`, stdhttp.StatusCreated)

	t.Run("subscription and topup lifecycle", func(t *testing.T) {
		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/billing/checkout", `{"packageId":"`+planID+`","kind":"subscription"}`, cookie, csrfToken, stdhttp.StatusCreated)
		subscriptionCheckout, subscriptionSessionID := checkoutCreator.requireLast(t)
		if subscriptionCheckout.OrganizationID != organizationID || subscriptionCheckout.UserID != userID || subscriptionCheckout.PlanID != planID || subscriptionCheckout.CheckoutKind != "subscription" {
			t.Fatalf("subscription checkout did not preserve tenant metadata: %+v", subscriptionCheckout)
		}
		commercialSendSignedWebhook(t, router, cfg.StripeWebhookSecret, signedCommercialCheckoutCompletedPayload(
			cfg.StripeWebhookSecret,
			"evt_commercial_subscription",
			subscriptionSessionID,
			"pi_provider_commercial_subscription",
			organizationID,
			userID,
			subscriptionCheckout.PaymentIntentID,
			planID,
			"subscription",
			2900,
		))

		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/billing/checkout", `{"kind":"topup","amount":25}`, cookie, csrfToken, stdhttp.StatusCreated)
		topupCheckout, topupSessionID := checkoutCreator.requireLast(t)
		if topupCheckout.OrganizationID != organizationID || topupCheckout.UserID != userID || topupCheckout.CheckoutKind != "topup" || topupCheckout.PlanPrice != 25 {
			t.Fatalf("topup checkout did not preserve tenant metadata: %+v", topupCheckout)
		}
		commercialSendSignedWebhook(t, router, cfg.StripeWebhookSecret, signedCommercialCheckoutCompletedPayload(
			cfg.StripeWebhookSecret,
			"evt_commercial_topup",
			topupSessionID,
			"pi_provider_commercial_topup",
			organizationID,
			userID,
			topupCheckout.PaymentIntentID,
			"",
			"topup",
			2500,
		))

		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/app/quota/topup", `{"amount":5}`, cookie, csrfToken, stdhttp.StatusPaymentRequired)
		commercialRequireCount(t, database, "subscription", `SELECT COUNT(*) FROM subscriptions WHERE organization_id = $1 AND user_id = $2 AND package_id = $3 AND status = 'active'`, organizationID, userID, planID)
		commercialRequireCount(t, database, "paid topup", `SELECT COUNT(*) FROM topup_orders WHERE organization_id = $1 AND user_id = $2 AND amount = 25 AND status = 'paid'`, organizationID, userID)
	})

	conversationID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/app/conversations", `{"title":"Commercial Relay Journey"}`, stdhttp.StatusOK)
	t.Run("Chat through Relay", func(t *testing.T) {
		body := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/app/conversations/"+conversationID+"/messages", `{"content":"Use Relay for this commercial journey."}`, cookie, csrfToken, stdhttp.StatusOK)
		if !strings.Contains(string(body), "Commercial Relay assistant response") {
			t.Fatalf("expected chat response to come from the fake Relay, body=%s", body)
		}
		relayRecorder.requireChatOrganization(t, organizationID)
		commercialRequireCount(t, database, "chat usage record", `SELECT COUNT(*) FROM usage_records WHERE organization_id = $1 AND user_id = $2 AND conversation_id = $3`, organizationID, userID, conversationID)
		if _, err := database.Exec(`
			INSERT INTO billing_sessions (id, user_id, organization_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt, status, created_at, settled_at)
			VALUES ('bs_commercial_relay_journey', $1, $2, 'ch_commercial_relay', $3, 'chat.completions', 'commercial-journey-chat', 1, 1, 'settled', NOW(), NOW())
		`, userID, organizationID, commercialJourneyModel); err != nil {
			t.Fatalf("insert commercial relay billing session: %v", err)
		}
	})

	t.Run("Knowledge RAG through Relay embeddings", func(t *testing.T) {
		knowledgeBaseID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/app/knowledge-bases", `{"name":"Commercial RAG Sources"}`, stdhttp.StatusOK)
		documentID := commercialPostDataID(t, router, cookie, csrfToken, "/api/v1/app/knowledge-bases/"+knowledgeBaseID+"/documents", `{
			"title":"Deployment Runbook",
			"content":"Commercial deployment rollback restore runbook with source citations and Relay embeddings."
		}`, stdhttp.StatusOK)
		body := commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/app/knowledge-bases/"+knowledgeBaseID+"/retrieve", `{"query":"deployment rollback restore"}`, cookie, csrfToken, stdhttp.StatusOK)
		if !strings.Contains(string(body), "embedding_rag") || !strings.Contains(string(body), "Deployment Runbook") || !strings.Contains(string(body), "chunkIndex") {
			t.Fatalf("expected embedding_rag retrieval with source citation, body=%s", body)
		}
		relayRecorder.requireEmbeddingOrganization(t, organizationID)
		commercialRequireCount(t, database, "knowledge document organization", `SELECT COUNT(*) FROM knowledge_documents WHERE id = $1 AND organization_id = $2`, documentID, organizationID)
	})

	t.Run("durable Agent run approval and retry", func(t *testing.T) {
		run, pendingToolRun, failedToolRun := prepareHTTPAgentWorkflowState(t, database, userID, organizationID)
		body := commercialDoJSON(t, router, stdhttp.MethodGet, "/api/v1/app/agents/runs/"+run.ID, "", cookie, "", stdhttp.StatusOK)
		if !strings.Contains(string(body), run.ID) || !strings.Contains(string(body), pendingToolRun.ID) || !strings.Contains(string(body), failedToolRun.ID) {
			t.Fatalf("expected durable run detail with tool runs, body=%s", body)
		}
		body = commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+pendingToolRun.ID+"/approve", `{"reason":"commercial operator approved"}`, cookie, csrfToken, stdhttp.StatusOK)
		if !strings.Contains(string(body), "approved") {
			t.Fatalf("expected approved tool run, body=%s", body)
		}
		_, _, retryToolRun := prepareHTTPAgentWorkflowState(t, database, userID, organizationID)
		body = commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+retryToolRun.ID+"/retry", "", cookie, csrfToken, stdhttp.StatusOK)
		if !strings.Contains(string(body), `"attemptCount":2`) || !strings.Contains(string(body), "completed") {
			t.Fatalf("expected retry attempt evidence, body=%s", body)
		}
	})

	t.Run("Marketplace publish review paid install settlement refund and governance", func(t *testing.T) {
		publisherCookie, publisherCSRF, publisherUserID := registerHTTPUser(t, router, "commercial-publisher@example.com")
		_, publisherOrganizationID := queryHTTPUserScope(t, database, publisherUserID)
		agentID := commercialPostDataID(t, router, publisherCookie, publisherCSRF, "/api/v1/marketplace/agents", `{
			"name":"Commercial Journey Agent",
			"description":"Commercial journey paid agent with settlement boundaries.",
			"tags":["commercial"],
			"categoryID":"cat_productivity",
			"tools":"[{\"name\":\"datetime\",\"type\":\"builtin\"}]",
			"exampleConversations":"[]",
			"systemPrompt":"Help commercial operators.",
			"visibility":"public",
			"pricingType":"one_time",
			"pricingAmount":50,
			"version":"1.0.0",
			"changelog":"Initial commercial version"
		}`, stdhttp.StatusCreated)
		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/admin/reviews/"+agentID+"/approve", "", cookie, csrfToken, stdhttp.StatusOK)
		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/marketplace/agents/"+agentID+"/install", `{"provider":"stripe"}`, cookie, csrfToken, stdhttp.StatusCreated)
		marketplaceCheckout, marketplaceSessionID := checkoutCreator.requireLast(t)
		if marketplaceCheckout.CheckoutKind != "marketplace_install" || marketplaceCheckout.AgentID != agentID || marketplaceCheckout.OrganizationID != organizationID || marketplaceCheckout.PublisherOrganizationID != publisherOrganizationID {
			t.Fatalf("marketplace checkout did not preserve buyer/publisher metadata: %+v", marketplaceCheckout)
		}

		commercialSendSignedWebhook(t, router, cfg.StripeWebhookSecret, signedHTTPMarketplaceCheckoutCompletedPayload(cfg.StripeWebhookSecret, "evt_commercial_marketplace_install", map[string]string{
			"organization_id":           organizationID,
			"user_id":                   userID,
			"payment_intent_id":         marketplaceCheckout.PaymentIntentID,
			"checkout_kind":             "marketplace_install",
			"marketplace_order_id":      marketplaceCheckout.MarketplaceOrderID,
			"agent_id":                  marketplaceCheckout.AgentID,
			"version_id":                marketplaceCheckout.VersionID,
			"publisher_user_id":         publisherUserID,
			"publisher_organization_id": publisherOrganizationID,
		}, map[string]string{
			"id":             marketplaceSessionID,
			"payment_intent": "pi_commercial_marketplace",
			"amount_total":   "5000",
			"currency":       "usd",
		}))
		commercialSendSignedWebhook(t, router, cfg.StripeWebhookSecret, signedHTTPMarketplaceRefundPayload(cfg.StripeWebhookSecret, "evt_commercial_marketplace_refund", map[string]string{
			"organization_id":   organizationID,
			"user_id":           userID,
			"payment_intent_id": marketplaceCheckout.PaymentIntentID,
			"checkout_kind":     "marketplace_install",
		}, map[string]string{
			"id":             "re_commercial_marketplace",
			"payment_intent": "pi_commercial_marketplace",
			"charge":         "ch_commercial_marketplace",
			"amount":         "1000",
			"currency":       "usd",
			"status":         "succeeded",
			"reason":         "requested_by_customer",
		}))
		if _, err := database.Exec(`
			INSERT INTO marketplace_payouts (id, publisher_organization_id, publisher_user_id, amount, currency, provider, provider_payout_id, status, metadata, created_at, updated_at)
			VALUES ('payout_commercial_journey', $1, $2, 40, 'usd', 'local', 'po_commercial_journey', 'pending', '{}', NOW(), NOW())
		`, publisherOrganizationID, publisherUserID); err != nil {
			t.Fatalf("insert commercial payout evidence: %v", err)
		}
		if _, err := database.Exec(`UPDATE marketplace_settlements SET payout_id = 'payout_commercial_journey', status = 'payout_pending' WHERE order_id = $1`, marketplaceCheckout.MarketplaceOrderID); err != nil {
			t.Fatalf("link commercial payout evidence: %v", err)
		}
		commercialDoJSON(t, router, stdhttp.MethodPost, "/api/v1/marketplace/agents/"+agentID+"/abuse-reports", `{"reason":"policy_review","details":"commercial governance evidence"}`, cookie, csrfToken, stdhttp.StatusCreated)

		commercialRequireCount(t, database, "marketplace paid install", `SELECT COUNT(*) FROM agent_installs WHERE agent_id = $1 AND organization_id = $2`, agentID, organizationID)
		commercialRequireCount(t, database, "marketplace settlement", `SELECT COUNT(*) FROM marketplace_settlements WHERE order_id = $1 AND publisher_organization_id = $2 AND refunded_amount = 10`, marketplaceCheckout.MarketplaceOrderID, publisherOrganizationID)
		commercialRequireCount(t, database, "marketplace governance report", `SELECT COUNT(*) FROM marketplace_abuse_reports WHERE agent_id = $1 AND reporter_organization_id = $2`, agentID, organizationID)
	})

	t.Run("Admin billing inspection", func(t *testing.T) {
		surfaces := map[string]string{
			"/api/v1/admin/billing/summary?organizationID=" + organizationID:         "billingSessions",
			"/api/v1/admin/billing/sessions?organizationID=" + organizationID:        "bs_commercial_relay_journey",
			"/api/v1/admin/billing/payment-intents?organizationID=" + organizationID: "paymentIntents",
			"/api/v1/admin/billing/webhook-events?organizationID=" + organizationID:  "webhookEvents",
			"/api/v1/admin/billing/subscriptions?organizationID=" + organizationID:   "subscriptions",
			"/api/v1/admin/billing/topups?organizationID=" + organizationID:          "topups",
			"/api/v1/admin/billing/refunds?organizationID=" + organizationID:         "refunds",
			"/api/v1/admin/billing/settlements?limit=20":                             "settlements",
			"/api/v1/admin/billing/payouts?limit=20":                                 "payouts",
		}
		for path, expected := range surfaces {
			body := commercialDoJSON(t, router, stdhttp.MethodGet, path, "", cookie, "", stdhttp.StatusOK)
			if !strings.Contains(string(body), expected) {
				t.Fatalf("expected admin billing surface %s to contain %q, body=%s", path, expected, body)
			}
		}
	})

	assertCommercialJourneyTenantEvidence(t, database, organizationID, userID, conversationID)
}

func applyCommercialJourneyMigrations(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, path := range []string{
		"../../migrations/0013_channels.sql",
		"../../migrations/0035_channel_groups.sql",
		"../../migrations/0039_channel_diagnostics.sql",
		"../../migrations/0077_channel_default_weight.sql",
		"../../migrations/0021_plan_extensions.sql",
		"../../migrations/0032_knowledge_rag_index.sql",
		"../../migrations/0042_workflows.sql",
	} {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read commercial journey migration %s: %v", path, err)
		}
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatalf("apply commercial journey migration %s: %v", path, err)
		}
	}
	if _, err := database.Exec(`TRUNCATE model_channel_weights, model_routes, channels CASCADE`); err != nil {
		t.Fatalf("reset commercial journey Relay config tables: %v", err)
	}
}

func startCommercialJourneyRelay(t *testing.T) (int, *commercialRelayRecorder) {
	t.Helper()
	recorder := &commercialRelayRecorder{}
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		recorder.mu.Lock()
		recorder.chatCalls++
		recorder.chatOrgs = append(recorder.chatOrgs, commercialRelayOrganizationHeader(r))
		recorder.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl_commercial_journey",
			"object":"chat.completion",
			"created":1770000000,
			"model":"gpt-4o-mini",
			"choices":[{"index":0,"message":{"role":"assistant","content":"Commercial Relay assistant response"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13}
		}`))
	})
	mux.HandleFunc("/v1/embeddings", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		var request struct {
			Input []string `json:"input"`
			Model string   `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			stdhttp.Error(w, "bad embedding request", stdhttp.StatusBadRequest)
			return
		}
		recorder.mu.Lock()
		recorder.embeddingCalls++
		recorder.embeddingOrgs = append(recorder.embeddingOrgs, commercialRelayOrganizationHeader(r))
		recorder.mu.Unlock()

		var builder strings.Builder
		builder.WriteString(`{"object":"list","model":"text-embedding-3-small","data":[`)
		for i := range request.Input {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(`{"object":"embedding","index":`)
			builder.WriteString(strconv.Itoa(i))
			builder.WriteString(`,"embedding":`)
			builder.WriteString(commercialEmbeddingJSON())
			builder.WriteByte('}')
		}
		builder.WriteString(`],"usage":{"prompt_tokens":1,"total_tokens":1}}`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(builder.String()))
	})

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("start commercial Relay listener: %v", err)
	}
	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("parse commercial Relay address %q: %v", server.Listener.Addr().String(), err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse commercial Relay port %q: %v", portText, err)
	}
	return port, recorder
}

func commercialRelayOrganizationHeader(r *stdhttp.Request) string {
	if organizationID := strings.TrimSpace(r.Header.Get(relaytypes.HeaderInternalOrganization)); organizationID != "" {
		return organizationID
	}
	return strings.TrimSpace(r.Header.Get("X-Oblivious-Internal-Organization-ID"))
}

func commercialEmbeddingJSON() string {
	values := make([]string, 1536)
	for i := range values {
		switch i {
		case 0:
			values[i] = "1"
		case 1:
			values[i] = "0.5"
		case 2:
			values[i] = "0.25"
		default:
			values[i] = "0"
		}
	}
	return "[" + strings.Join(values, ",") + "]"
}

func (r *commercialRelayRecorder) requireChatOrganization(t *testing.T, organizationID string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.chatCalls == 0 {
		t.Fatal("expected Chat to call Relay")
	}
	for _, seen := range r.chatOrgs {
		if seen == organizationID {
			return
		}
	}
	t.Fatalf("expected Relay chat call for organization %s, got %v", organizationID, r.chatOrgs)
}

func (r *commercialRelayRecorder) requireEmbeddingOrganization(t *testing.T, organizationID string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.embeddingCalls == 0 {
		t.Fatal("expected Knowledge to call Relay embeddings")
	}
	for _, seen := range r.embeddingOrgs {
		if seen == organizationID {
			return
		}
	}
	t.Fatalf("expected Relay embedding call for organization %s, got %v", organizationID, r.embeddingOrgs)
}

func commercialDoJSON(t *testing.T, router stdhttp.Handler, method, path, body string, cookie *stdhttp.Cookie, csrfToken string, expectedStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrfToken != "" {
		addCSRF(request, csrfToken)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != expectedStatus {
		t.Fatalf("%s %s expected %d, got %d with body %s", method, path, expectedStatus, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.Bytes()
}

func commercialPostDataID(t *testing.T, router stdhttp.Handler, cookie *stdhttp.Cookie, csrfToken, path, body string, expectedStatus int) string {
	t.Helper()
	responseBody := commercialDoJSON(t, router, stdhttp.MethodPost, path, body, cookie, csrfToken, expectedStatus)
	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		t.Fatalf("decode %s response: %v", path, err)
	}
	if response.Data.ID == "" {
		t.Fatalf("expected %s response data.id, body=%s", path, responseBody)
	}
	return response.Data.ID
}

func commercialSendSignedWebhook(t *testing.T, router stdhttp.Handler, secret string, payload *webhook.SignedPayload) {
	t.Helper()
	_ = secret
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/billing/stripe/webhook", strings.NewReader(string(payload.Payload)))
	request.Header.Set("Stripe-Signature", payload.Header)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("signed webhook expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func signedCommercialCheckoutCompletedPayload(secret, eventID, sessionID, providerPaymentIntentID, organizationID, userID, paymentIntentID, planID, checkoutKind string, amountTotal int) *webhook.SignedPayload {
	subscriptionID := ""
	customerID := ""
	if checkoutKind == "subscription" {
		subscriptionID = "sub_" + eventID
		customerID = "cus_" + eventID
	}
	payload := []byte(`{
		"id": "` + eventID + `",
		"object": "event",
		"api_version": "` + stripeapi.APIVersion + `",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "` + sessionID + `",
				"object": "checkout.session",
				"payment_intent": "` + providerPaymentIntentID + `",
				"subscription": "` + subscriptionID + `",
				"customer": "` + customerID + `",
				"amount_total": ` + strconv.Itoa(amountTotal) + `,
				"currency": "usd",
				"metadata": {
					"organization_id": "` + organizationID + `",
					"user_id": "` + userID + `",
					"payment_intent_id": "` + paymentIntentID + `",
					"plan_id": "` + planID + `",
					"checkout_kind": "` + checkoutKind + `"
				}
			}
		}
	}`)
	return webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
}

func commercialRequireCount(t *testing.T, database *sql.DB, label, query string, args ...any) {
	t.Helper()
	var count int
	if err := database.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("query %s count: %v", label, err)
	}
	if count == 0 {
		t.Fatalf("expected at least one %s row", label)
	}
}

func assertCommercialJourneyTenantEvidence(t *testing.T, database *sql.DB, organizationID, userID, conversationID string) {
	t.Helper()
	checks := []struct {
		label string
		query string
		args  []any
	}{
		{"session", `SELECT COUNT(*) FROM sessions WHERE organization_id = $1 AND user_id = $2`, []any{organizationID, userID}},
		{"workspace", `SELECT COUNT(*) FROM workspaces WHERE organization_id = $1 AND user_id = $2`, []any{organizationID, userID}},
		{"conversation", `SELECT COUNT(*) FROM conversations WHERE organization_id = $1 AND id = $2`, []any{organizationID, conversationID}},
		{"message", `SELECT COUNT(*) FROM messages WHERE organization_id = $1 AND conversation_id = $2`, []any{organizationID, conversationID}},
		{"knowledge base", `SELECT COUNT(*) FROM knowledge_bases WHERE organization_id = $1`, []any{organizationID}},
		{"knowledge chunks", `SELECT COUNT(*) FROM knowledge_document_chunks WHERE organization_id = $1 AND embedding IS NOT NULL`, []any{organizationID}},
		{"usage", `SELECT COUNT(*) FROM usage_records WHERE organization_id = $1 AND user_id = $2`, []any{organizationID, userID}},
		{"payment intents", `SELECT COUNT(*) FROM payment_intents WHERE organization_id = $1 AND user_id = $2`, []any{organizationID, userID}},
		{"billing sessions", `SELECT COUNT(*) FROM billing_sessions WHERE organization_id = $1 AND user_id = $2`, []any{organizationID, userID}},
		{"marketplace buyer orders", `SELECT COUNT(*) FROM marketplace_orders WHERE buyer_organization_id = $1 AND buyer_user_id = $2`, []any{organizationID, userID}},
	}
	for _, check := range checks {
		commercialRequireCount(t, database, check.label, check.query, check.args...)
	}
}
