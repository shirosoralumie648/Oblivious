package http

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/payment"
	"oblivious/server/internal/releasecontract"
	stripebilling "oblivious/server/internal/stripe"
)

func TestStrictRouterReadinessCompositionContract(t *testing.T) {
	assertStrictRouterProductionSource(t)
}

func TestStrictRouterAdminReadinessCompositionContract(t *testing.T) {
	t.Run("missing startup carrier fails before route construction", func(t *testing.T) {
		guard := &billingReadinessGuardSpy{}
		registrar := &billingReadinessRegistrarSpy{}
		valid := newStrictRouterOptions(t, guard, registrar)
		tests := []struct {
			name    string
			options RouterOptions
		}{
			{name: "guard", options: func() RouterOptions { value := valid; value.Guard = nil; return value }()},
			{name: "authorities", options: func() RouterOptions {
				value := valid
				value.Authorities = releasecontract.RuntimeAuthorities{}
				return value
			}()},
			{name: "effects", options: func() RouterOptions { value := valid; value.Effects = nil; return value }()},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handler, err := NewReadinessRouterWithOptions(testConfig(), nil, test.options)
				if err == nil || handler != nil {
					t.Fatalf("missing %s must fail strict construction: handler=%v err=%v", test.name, handler, err)
				}
			})
		}
	})

	t.Run("caller supplied AdminService is rejected before dependencies", func(t *testing.T) {
		store := &strictRouterAdminStoreSpy{}
		registrar := &billingReadinessRegistrarSpy{}
		options := newStrictRouterOptions(t, &billingReadinessGuardSpy{}, registrar)
		options.AdminService = admin.NewService(store)

		handler, err := NewReadinessRouterWithOptions(testConfig(), nil, options)
		if err == nil || !strings.Contains(err.Error(), "caller-supplied AdminService") || handler != nil {
			t.Fatalf("strict injected AdminService must fail with a stable error: handler=%v err=%v", handler, err)
		}
		if store.calls != 0 || len(registrar.descriptors) != 0 {
			t.Fatalf("strict rejection reached injected service or route dependencies: store=%d descriptors=%d", store.calls, len(registrar.descriptors))
		}
	})

	t.Run("production Admin service registers model mutation exactly once", func(t *testing.T) {
		registrar := &billingReadinessRegistrarSpy{}
		handler, err := NewReadinessRouterWithOptions(testConfig(), nil, newStrictRouterOptions(t, &billingReadinessGuardSpy{}, registrar))
		if err != nil || handler == nil {
			t.Fatalf("strict router construction failed: handler=%v err=%v", handler, err)
		}
		registrations := 0
		for _, descriptor := range registrar.descriptors {
			if descriptor.ID == "admin.channel.model.mutation" {
				registrations++
			}
		}
		if registrations != 1 {
			t.Fatalf("admin model mutation registrations=%d, want 1: %#v", registrations, registrar.descriptors)
		}
	})

	t.Run("production sources cannot inject AdminService", assertStrictRouterProductionSource)
}

func TestStrictRouterMarketplaceReadinessCompositionContract(t *testing.T) {
	t.Run("strict construction registers marketplace checkout exactly once", func(t *testing.T) {
		registrar := &billingReadinessRegistrarSpy{}
		handler, err := NewReadinessRouterWithOptions(testConfig(), nil, newStrictRouterOptions(t, &billingReadinessGuardSpy{}, registrar))
		if err != nil || handler == nil {
			t.Fatalf("strict router construction failed: handler=%v err=%v", handler, err)
		}
		registrations := 0
		for _, descriptor := range registrar.descriptors {
			if descriptor.ID == "http.marketplace.checkout" {
				registrations++
			}
		}
		if registrations != 1 {
			t.Fatalf("marketplace checkout registrations=%d, want 1: %#v", registrations, registrar.descriptors)
		}
	})

	t.Run("strict Marketplace constructor is explicit", func(t *testing.T) {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("resolve Marketplace composition test source")
		}
		contents, err := os.ReadFile(filepath.Join(filepath.Dir(source), "marketplace_handler.go"))
		if err != nil {
			t.Fatalf("read Marketplace handler source: %v", err)
		}
		if !strings.Contains(string(contents), "func newMarketplaceHandlerWithFinancialReadiness") {
			t.Fatal("strict Marketplace readiness constructor is missing")
		}
		_, err = newMarketplaceHandlerWithFinancialReadiness(
			nil, nil, &fakeMarketplaceSettlementService{}, &fakeCheckoutCreator{}, stripebilling.CheckoutConfig{},
			payment.NewRegistry("stripe"), nil, marketplace.FinancialReadiness{},
		)
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("missing financial readiness must fail strict construction, got %v", err)
		}
	})

	t.Run("denial precedes local order and provider effects", func(t *testing.T) {
		denial := &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}
		guard := &billingReadinessGuardSpy{denial: denial}
		registrar := &billingReadinessRegistrarSpy{}
		settlement := &fakeMarketplaceSettlementService{}
		checkout := &fakeCheckoutCreator{}
		registry := payment.NewRegistry("stripe")
		registry.Register(payment.Provider{Name: "stripe", Configured: true, Currency: "usd"})
		handler, err := newMarketplaceHandlerWithFinancialReadiness(
			nil, nil, settlement, checkout, stripebilling.CheckoutConfig{}, registry, nil,
			newHTTPFinancialReadiness(t, guard, registrar),
		)
		if err != nil {
			t.Fatalf("construct strict Marketplace handler: %v", err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_paid/install", nil)
		handler.createPaidInstallCheckout(recorder, request, "buyer_1", "org_buyer", &marketplace.PublishedAgent{
			ID: "agent_paid", Name: "Paid Agent", PricingType: "one_time", PricingAmount: 25,
		}, "version_1", "stripe")

		if recorder.Code != stdhttp.StatusServiceUnavailable || len(guard.calls) != 1 {
			t.Fatalf("expected one current readiness denial, status=%d calls=%#v body=%s", recorder.Code, guard.calls, recorder.Body.String())
		}
		if settlement.createCalls != 0 || settlement.setSessionCalls != 0 || settlement.failCalls != 0 || checkout.request.PaymentIntentID != "" {
			t.Fatalf("denial reached checkout effects: create=%d providerIntent=%q session=%d failure=%d", settlement.createCalls, checkout.request.PaymentIntentID, settlement.setSessionCalls, settlement.failCalls)
		}
	})
}

func newStrictRouterOptions(t *testing.T, guard releasecontract.Guard, registrar releasecontract.EffectRegistrar) RouterOptions {
	t.Helper()
	contract, profile := loadHTTPModelReadinessAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile strict router authorities: %v", err)
	}
	return RouterOptions{
		Readiness:   strictRouterReadinessStub{},
		Guard:       guard,
		Effects:     registrar,
		Authorities: authorities,
	}
}

type strictRouterReadinessStub struct{}

func (strictRouterReadinessStub) Bootstrap(context.Context) error { return nil }
func (strictRouterReadinessStub) StartRefresh(context.Context)    {}
func (strictRouterReadinessStub) Require(string) error            { return nil }
func (strictRouterReadinessStub) Evaluate() releasecontract.Evaluation {
	return releasecontract.Evaluation{}
}
func (strictRouterReadinessStub) ExportAudit(string) error { return nil }

type strictRouterAdminStoreSpy struct {
	admin.Store
	calls int
}

func (s *strictRouterAdminStoreSpy) GetChannel(context.Context, string, string) (*admin.ChannelInfo, error) {
	s.calls++
	return nil, nil
}

func (s *strictRouterAdminStoreSpy) TestChannel(context.Context, string, string) (*admin.ChannelTestResult, error) {
	s.calls++
	return nil, nil
}

func assertStrictRouterProductionSource(t *testing.T) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve strict router composition test source")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	paths := []string{filepath.Join(serverRoot, "internal/http"), filepath.Join(serverRoot, "cmd/server")}
	fileSet := token.NewFileSet()
	buildRuntimeUsesStrictEntrypoint := false
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				if pair, ok := node.(*ast.KeyValueExpr); ok {
					if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "AdminService" {
						t.Errorf("production source injects RouterOptions.AdminService: %s", fileSet.Position(pair.Pos()))
					}
				}
				return true
			})
			if filepath.Base(path) == "server.go" {
				for _, declaration := range parsed.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if !ok || function.Name.Name != "buildRuntime" {
						continue
					}
					ast.Inspect(function.Body, func(node ast.Node) bool {
						call, ok := node.(*ast.CallExpr)
						if !ok {
							return true
						}
						if identifier, ok := call.Fun.(*ast.Ident); ok && identifier.Name == "NewReadinessRouterWithOptions" {
							buildRuntimeUsesStrictEntrypoint = true
						}
						return true
					})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("inspect production source %s: %v", root, err)
		}
	}
	if !buildRuntimeUsesStrictEntrypoint {
		t.Fatal("buildRuntime does not reach NewReadinessRouterWithOptions")
	}
}
