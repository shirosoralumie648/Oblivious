package http

import (
	"context"
	"errors"
	"testing"

	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/releasecontract"
)

type billingReadinessGuardSpy struct {
	denial error
	calls  []string
}

func (g *billingReadinessGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.calls = append(g.calls, capabilityID+":"+string(boundary))
	return g.denial
}

type billingReadinessRegistrarSpy struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *billingReadinessRegistrarSpy) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

func newHTTPFinancialReadiness(t *testing.T, guard releasecontract.Guard, registrar releasecontract.EffectRegistrar) marketplace.FinancialReadiness {
	t.Helper()
	contract, profile := loadHTTPModelReadinessAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build financial authorities: %v", err)
	}
	return marketplace.FinancialReadiness{Guard: guard, Effects: registrar, Authorities: authorities}
}

func TestBillingCheckoutReadinessContract(t *testing.T) {
	guard := &billingReadinessGuardSpy{}
	registrar := &billingReadinessRegistrarSpy{}
	readiness, err := newBillingFinancialReadiness(newHTTPFinancialReadiness(t, guard, registrar))
	if err != nil {
		t.Fatalf("new billing readiness: %v", err)
	}
	if len(registrar.descriptors) != 1 || registrar.descriptors[0].ID != "http.billing.checkout" {
		t.Fatalf("expected one stable billing descriptor, got %#v", registrar.descriptors)
	}
	if err := readiness.require(context.Background()); err != nil {
		t.Fatalf("enabled checkout should pass: %v", err)
	}
	if len(guard.calls) != 1 || guard.calls[0] == "" {
		t.Fatalf("expected one financial guard call, got %#v", guard.calls)
	}

	denial := &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}
	guard.denial = denial
	if err := readiness.require(context.Background()); !errors.Is(err, denial) {
		t.Fatalf("expected stale denial, got %v", err)
	}
	if len(guard.calls) != 2 {
		t.Fatalf("expected each provider attempt to recheck readiness, got %d calls", len(guard.calls))
	}
}

func TestMarketplaceCheckoutReadinessContract(t *testing.T) {
	denial := &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "capability"}
	guard := &billingReadinessGuardSpy{denial: denial}
	registrar := &billingReadinessRegistrarSpy{}
	service := marketplace.NewSettlementService(nil, marketplace.WithMarketplaceFinancialReadiness(newHTTPFinancialReadiness(t, guard, registrar)))
	_, err := service.CreatePaidInstallCheckout(context.Background(), marketplace.PaidInstallCheckoutRequest{
		BuyerOrganizationID: "org_1", BuyerUserID: "user_1", AgentID: "agent_1",
	})
	if !errors.Is(err, denial) {
		t.Fatalf("expected checkout denial before store/provider effects, got %v", err)
	}
	if len(guard.calls) != 1 {
		t.Fatalf("expected one pre-intent guard call, got %d", len(guard.calls))
	}
}
