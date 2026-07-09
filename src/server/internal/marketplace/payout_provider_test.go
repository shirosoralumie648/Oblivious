package marketplace

import (
        "context"
        "encoding/json"
        "io"
        "net/http"
        "net/http/httptest"
        "testing"
)

func TestWebhookPayoutProviderDispatchesSignedRequest(t *testing.T) {
        secret := "payout-secret"
        var received webhookPayoutRequest
        server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPost {
                        t.Fatalf("expected POST, got %s", r.Method)
                }
                body, err := io.ReadAll(r.Body)
                if err != nil {
                        t.Fatalf("read request body: %v", err)
                }
                if got, want := r.Header.Get("X-Oblivious-Payout-Signature"), signWebhookPayoutBody(body, secret); got != want {
                        t.Fatalf("unexpected signature header got=%q want=%q", got, want)
                }
                if got := r.Header.Get("X-Oblivious-Payout-ID"); got != "payout_1" {
                        t.Fatalf("unexpected payout id header %q", got)
                }
                if err := json.Unmarshal(body, &received); err != nil {
                        t.Fatalf("decode request body: %v", err)
                }
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusAccepted)
                _, _ = w.Write([]byte(`{"providerPayoutID":"provider_payout_1"}`))
        }))
        defer server.Close()

        provider := NewWebhookPayoutProvider(server.URL, secret)
        result, err := provider.CreatePayout(context.Background(), MarketplacePayoutDispatchRequest{
                PayoutID:                " payout_1 ",
                PublisherOrganizationID: " publisher_org ",
                PublisherUserID:         " publisher_user ",
                Amount:                  80.125,
                Currency:                " USD ",
                SettlementIDs:           []string{" settlement_1 ", "", "settlement_2"},
        })
        if err != nil {
                t.Fatalf("CreatePayout returned error: %v", err)
        }
        if result.ProviderPayoutID != "provider_payout_1" {
                t.Fatalf("unexpected provider payout id %q", result.ProviderPayoutID)
        }
        if received.PayoutID != "payout_1" || received.PublisherOrganizationID != "publisher_org" || received.PublisherUserID != "publisher_user" {
                t.Fatalf("unexpected normalized request identity: %#v", received)
        }
        if received.Amount != 80.13 || received.Currency != "usd" {
                t.Fatalf("unexpected normalized amount/currency: %#v", received)
        }
        if len(received.SettlementIDs) != 2 || received.SettlementIDs[0] != "settlement_1" || received.SettlementIDs[1] != "settlement_2" {
                t.Fatalf("unexpected settlement ids: %#v", received.SettlementIDs)
        }
}

func TestWebhookPayoutProviderRejectsMissingProviderPayoutID(t *testing.T) {
        server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                _, _ = w.Write([]byte(`{}`))
        }))
        defer server.Close()

        provider := NewWebhookPayoutProvider(server.URL, "payout-secret")
        _, err := provider.CreatePayout(context.Background(), MarketplacePayoutDispatchRequest{
                PayoutID:                "payout_1",
                PublisherOrganizationID: "publisher_org",
                PublisherUserID:         "publisher_user",
                Amount:                  80,
                Currency:                "usd",
        })
        if err == nil {
                t.Fatal("expected error for missing provider payout id")
        }
}

func TestWebhookPayoutProviderRejectsUnsuccessfulResponse(t *testing.T) {
        server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusBadGateway)
                _, _ = w.Write([]byte(`provider unavailable`))
        }))
        defer server.Close()

        provider := NewWebhookPayoutProvider(server.URL, "payout-secret")
        _, err := provider.CreatePayout(context.Background(), MarketplacePayoutDispatchRequest{
                PayoutID:                "payout_1",
                PublisherOrganizationID: "publisher_org",
                PublisherUserID:         "publisher_user",
                Amount:                  80,
                Currency:                "usd",
        })
        if err == nil {
                t.Fatal("expected error for unsuccessful webhook response")
        }
}
