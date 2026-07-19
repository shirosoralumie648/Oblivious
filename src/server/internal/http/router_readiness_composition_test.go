package http

import (
	"context"
	"errors"
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

	t.Run("AST proof rejects strict composition bypasses", func(t *testing.T) {
		tests := []struct {
			name              string
			source            string
			wantViolation     string
			wantBuildRuntimes int
			wantStrictCalls   int
			wantLegacyCalls   int
		}{
			{
				name:          "keyed RouterOptions AdminService injection",
				source:        `package http; var _ = RouterOptions{AdminService: nil}`,
				wantViolation: "RouterOptions.AdminService",
			},
			{
				name:          "selector AdminService assignment",
				source:        `package http; func mutate(options RouterOptions) { options.AdminService = nil }`,
				wantViolation: "AdminService assignment",
			},
			{
				name:          "unkeyed RouterOptions literal",
				source:        `package http; var _ = RouterOptions{nil}`,
				wantViolation: "unkeyed RouterOptions",
			},
			{
				name:              "buildRuntime call inventory",
				source:            `package http; func buildRuntime() { NewReadinessRouterWithOptions(); NewRouterWithOptions() }`,
				wantBuildRuntimes: 1,
				wantStrictCalls:   1,
				wantLegacyCalls:   1,
			},
			{
				name:              "buildRuntime indirect legacy wrapper",
				source:            `package http; func buildRuntime() { compatibilityRouter() }; func compatibilityRouter() { NewRouterWithOptions() }`,
				wantBuildRuntimes: 1,
				wantLegacyCalls:   1,
			},
			{
				name:              "buildRuntime legacy function alias",
				source:            `package http; func buildRuntime() { builder := NewRouterWithOptions; builder() }`,
				wantBuildRuntimes: 1,
				wantLegacyCalls:   1,
			},
			{
				name:              "buildRuntime parenthesized legacy call",
				source:            `package http; func buildRuntime() { (NewRouterWithOptions)() }`,
				wantBuildRuntimes: 1,
				wantLegacyCalls:   1,
			},
			{
				name:              "buildRuntime method wrapper",
				source:            `package http; type compatibilityBuilder struct{}; func (compatibilityBuilder) build() { NewRouterWithOptions() }; func buildRuntime() { var builder compatibilityBuilder; builder.build() }`,
				wantBuildRuntimes: 1,
				wantLegacyCalls:   1,
			},
			{
				name:   "unrelated AdminService key is ignored",
				source: `package http; type otherOptions struct { AdminService any }; var _ = otherOptions{AdminService: nil}; var _ = RouterOptions{Guard: nil}`,
			},
			{
				name:   "unrelated typed AdminService assignment is ignored",
				source: `package http; type otherOptions struct { AdminService any }; func mutate(other otherOptions) { other.AdminService = nil }`,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				fileSet := token.NewFileSet()
				parsed, err := parser.ParseFile(fileSet, "mutation.go", test.source, 0)
				if err != nil {
					t.Fatalf("parse AST mutation: %v", err)
				}
				analysis := inspectStrictRouterSource(fileSet, parsed)
				inventory := strictRouterEntrypointInventory(analysis, "http", "buildRuntime")
				buildRuntimes := 0
				if inventory.found {
					buildRuntimes = 1
				}
				violations := strings.Join(analysis.violations, "\n")
				if test.wantViolation == "" && violations != "" {
					t.Fatalf("unexpected AST violation: %s", violations)
				}
				if test.wantViolation != "" && !strings.Contains(violations, test.wantViolation) {
					t.Fatalf("AST violations %q do not contain %q", violations, test.wantViolation)
				}
				if buildRuntimes != test.wantBuildRuntimes || inventory.strictCalls != test.wantStrictCalls || inventory.legacyCalls != test.wantLegacyCalls {
					t.Fatalf("buildRuntime inventory=(functions=%d strict=%d legacy=%d), want (%d,%d,%d)", buildRuntimes, inventory.strictCalls, inventory.legacyCalls, test.wantBuildRuntimes, test.wantStrictCalls, test.wantLegacyCalls)
				}
			})
		}
	})
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

	t.Run("late readiness denial compensates pending checkout before returning sanitized readiness", func(t *testing.T) {
		denial := &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}
		guard := &billingReadinessGuardSpy{denials: []error{nil, denial}}
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

		if recorder.Code != stdhttp.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), string(releasecontract.CodeReadinessStale)) || strings.Contains(recorder.Body.String(), "generation") {
			t.Fatalf("late denial must remain sanitized readiness 503: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		const wantReason = "marketplace checkout readiness denied before provider dispatch"
		if len(guard.calls) != 2 || settlement.createCalls != 1 || settlement.failCalls != 1 || settlement.failedOrderID != "order_1" || settlement.failedPaymentIntentID != "pi_marketplace" || settlement.failureReason != wantReason || checkout.calls != 0 || settlement.setSessionCalls != 0 {
			t.Fatalf("late denial compensation mismatch: guard=%#v create=%d fail=%d failedOrder=%q failedIntent=%q reason=%q provider=%d session=%d", guard.calls, settlement.createCalls, settlement.failCalls, settlement.failedOrderID, settlement.failedPaymentIntentID, settlement.failureReason, checkout.calls, settlement.setSessionCalls)
		}
	})

	t.Run("late readiness compensation failure is internal error and provider remains untouched", func(t *testing.T) {
		denial := &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}
		guard := &billingReadinessGuardSpy{denials: []error{nil, denial}}
		settlement := &fakeMarketplaceSettlementService{failErr: errors.New("sensitive settlement failure")}
		checkout := &fakeCheckoutCreator{}
		registry := payment.NewRegistry("stripe")
		registry.Register(payment.Provider{Name: "stripe", Configured: true, Currency: "usd"})
		handler, err := newMarketplaceHandlerWithFinancialReadiness(
			nil, nil, settlement, checkout, stripebilling.CheckoutConfig{}, registry, nil,
			newHTTPFinancialReadiness(t, guard, &billingReadinessRegistrarSpy{}),
		)
		if err != nil {
			t.Fatalf("construct strict Marketplace handler: %v", err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_paid/install", nil)
		handler.createPaidInstallCheckout(recorder, request, "buyer_1", "org_buyer", &marketplace.PublishedAgent{
			ID: "agent_paid", Name: "Paid Agent", PricingType: "one_time", PricingAmount: 25,
		}, "version_1", "stripe")

		if recorder.Code != stdhttp.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "internal_error") || strings.Contains(recorder.Body.String(), "sensitive settlement failure") {
			t.Fatalf("compensation failure must be sanitized internal_error: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		if settlement.createCalls != 1 || settlement.failCalls != 1 || checkout.calls != 0 || settlement.setSessionCalls != 0 {
			t.Fatalf("compensation failure effects mismatch: create=%d fail=%d provider=%d session=%d", settlement.createCalls, settlement.failCalls, checkout.calls, settlement.setSessionCalls)
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

type strictRouterFunctionAnalysis struct {
	calls       []string
	strictCalls int
	legacyCalls int
}

type strictRouterSourceAnalysis struct {
	violations []string
	functions  map[string]strictRouterFunctionAnalysis
}

type strictRouterEntrypointCalls struct {
	found       bool
	strictCalls int
	legacyCalls int
}

const (
	strictRouterStrictTarget = "@router:strict"
	strictRouterLegacyTarget = "@router:legacy"
	strictRouterMethodTarget = "@method:"
)

func inspectStrictRouterSource(fileSet *token.FileSet, parsed *ast.File) strictRouterSourceAnalysis {
	analysis := strictRouterSourceAnalysis{functions: map[string]strictRouterFunctionAnalysis{}}
	ast.Inspect(parsed, func(node ast.Node) bool {
		if value, ok := node.(*ast.CompositeLit); ok {
			if !isRouterOptionsType(value.Type) {
				return true
			}
			for _, element := range value.Elts {
				pair, keyed := element.(*ast.KeyValueExpr)
				if !keyed {
					analysis.violations = append(analysis.violations, "unkeyed RouterOptions literal: "+fileSet.Position(value.Pos()).String())
					break
				}
				key, ok := pair.Key.(*ast.Ident)
				if ok && key.Name == "AdminService" {
					analysis.violations = append(analysis.violations, "RouterOptions.AdminService injection: "+fileSet.Position(pair.Pos()).String())
				}
			}
		}
		return true
	})
	fileTypes := strictRouterFileVariableTypes(parsed)
	importAliases := strictRouterImportAliases(parsed)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		functionAnalysis := inspectStrictRouterFunction(fileSet, parsed.Name.Name, function, fileTypes, importAliases, &analysis.violations)
		analysis.functions[strictRouterFunctionKey(parsed.Name.Name, function)] = functionAnalysis
	}
	return analysis
}

func inspectStrictRouterFunction(
	fileSet *token.FileSet,
	packageName string,
	function *ast.FuncDecl,
	fileTypes map[string]string,
	importAliases map[string]bool,
	violations *[]string,
) strictRouterFunctionAnalysis {
	analysis := strictRouterFunctionAnalysis{}
	variableTypes := map[string]string{}
	for name, typeName := range fileTypes {
		variableTypes[name] = typeName
	}
	strictRouterRecordFieldTypes(variableTypes, function.Recv)
	strictRouterRecordFieldTypes(variableTypes, function.Type.Params)
	strictRouterRecordFieldTypes(variableTypes, function.Type.Results)
	aliases := map[string]string{}

	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.DeclStmt:
			declaration, ok := value.Decl.(*ast.GenDecl)
			if !ok || declaration.Tok != token.VAR {
				return true
			}
			for _, specification := range declaration.Specs {
				if values, ok := specification.(*ast.ValueSpec); ok {
					strictRouterRecordValueSpec(values, packageName, variableTypes, aliases, importAliases)
				}
			}
		case *ast.AssignStmt:
			for index, left := range value.Lhs {
				if selector, ok := unwrapStrictRouterExpr(left).(*ast.SelectorExpr); ok && selector.Sel.Name == "AdminService" && strictRouterExpressionType(selector.X, variableTypes) == "RouterOptions" {
					*violations = append(*violations, "AdminService assignment: "+fileSet.Position(selector.Pos()).String())
				}
				identifier, ok := unwrapStrictRouterExpr(left).(*ast.Ident)
				if !ok || index >= len(value.Rhs) {
					continue
				}
				right := value.Rhs[index]
				if typeName := strictRouterExpressionType(right, variableTypes); typeName != "" {
					variableTypes[identifier.Name] = typeName
				}
				if target := strictRouterResolveReference(right, packageName, variableTypes, aliases, importAliases); target != "" {
					aliases[identifier.Name] = target
				} else {
					delete(aliases, identifier.Name)
				}
			}
		case *ast.CallExpr:
			target := strictRouterResolveReference(value.Fun, packageName, variableTypes, aliases, importAliases)
			switch target {
			case strictRouterStrictTarget:
				analysis.strictCalls++
			case strictRouterLegacyTarget:
				analysis.legacyCalls++
			case "":
			default:
				analysis.calls = append(analysis.calls, target)
			}
		}
		return true
	})
	return analysis
}

func strictRouterFileVariableTypes(parsed *ast.File) map[string]string {
	variableTypes := map[string]string{}
	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.VAR {
			continue
		}
		for _, specification := range general.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typeName := strictRouterTypeName(values.Type)
			for index, name := range values.Names {
				inferred := typeName
				if inferred == "" && index < len(values.Values) {
					inferred = strictRouterExpressionType(values.Values[index], variableTypes)
				}
				if inferred != "" {
					variableTypes[name.Name] = inferred
				}
			}
		}
	}
	return variableTypes
}

func strictRouterImportAliases(parsed *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, imported := range parsed.Imports {
		if imported.Name != nil {
			if imported.Name.Name != "_" && imported.Name.Name != "." {
				aliases[imported.Name.Name] = true
			}
			continue
		}
		path := strings.Trim(imported.Path.Value, "\"")
		if separator := strings.LastIndex(path, "/"); separator >= 0 {
			path = path[separator+1:]
		}
		aliases[path] = true
	}
	return aliases
}

func strictRouterRecordFieldTypes(variableTypes map[string]string, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		typeName := strictRouterTypeName(field.Type)
		for _, name := range field.Names {
			variableTypes[name.Name] = typeName
		}
	}
}

func strictRouterRecordValueSpec(values *ast.ValueSpec, packageName string, variableTypes map[string]string, aliases map[string]string, importAliases map[string]bool) {
	explicitType := strictRouterTypeName(values.Type)
	for index, name := range values.Names {
		if explicitType != "" {
			variableTypes[name.Name] = explicitType
		}
		if index >= len(values.Values) {
			continue
		}
		right := values.Values[index]
		if explicitType == "" {
			if typeName := strictRouterExpressionType(right, variableTypes); typeName != "" {
				variableTypes[name.Name] = typeName
			}
		}
		if target := strictRouterResolveReference(right, packageName, variableTypes, aliases, importAliases); target != "" {
			aliases[name.Name] = target
		}
	}
}

func strictRouterResolveReference(expression ast.Expr, packageName string, variableTypes map[string]string, aliases map[string]string, importAliases map[string]bool) string {
	switch value := unwrapStrictRouterExpr(expression).(type) {
	case *ast.Ident:
		if target := aliases[value.Name]; target != "" {
			return target
		}
		switch value.Name {
		case "NewReadinessRouterWithOptions":
			return strictRouterStrictTarget
		case "NewRouterWithOptions":
			return strictRouterLegacyTarget
		default:
			return packageName + "." + value.Name
		}
	case *ast.SelectorExpr:
		if identifier, ok := unwrapStrictRouterExpr(value.X).(*ast.Ident); ok && importAliases[identifier.Name] {
			return ""
		}
		if receiverType := strictRouterExpressionType(value.X, variableTypes); receiverType != "" {
			return packageName + "." + receiverType + "." + value.Sel.Name
		}
		return strictRouterMethodTarget + value.Sel.Name
	default:
		return ""
	}
}

func strictRouterExpressionType(expression ast.Expr, variableTypes map[string]string) string {
	switch value := unwrapStrictRouterExpr(expression).(type) {
	case *ast.Ident:
		return variableTypes[value.Name]
	case *ast.CompositeLit:
		return strictRouterTypeName(value.Type)
	case *ast.UnaryExpr:
		return strictRouterExpressionType(value.X, variableTypes)
	case *ast.StarExpr:
		return strictRouterExpressionType(value.X, variableTypes)
	default:
		return ""
	}
}

func strictRouterTypeName(expression ast.Expr) string {
	if expression == nil {
		return ""
	}
	switch value := unwrapStrictRouterExpr(expression).(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.StarExpr:
		return strictRouterTypeName(value.X)
	default:
		return ""
	}
}

func unwrapStrictRouterExpr(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func strictRouterFunctionKey(packageName string, function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return packageName + "." + function.Name.Name
	}
	return packageName + "." + strictRouterTypeName(function.Recv.List[0].Type) + "." + function.Name.Name
}

func mergeStrictRouterSourceAnalysis(target *strictRouterSourceAnalysis, source strictRouterSourceAnalysis) {
	target.violations = append(target.violations, source.violations...)
	if target.functions == nil {
		target.functions = map[string]strictRouterFunctionAnalysis{}
	}
	for name, function := range source.functions {
		target.functions[name] = function
	}
}

func strictRouterEntrypointInventory(analysis strictRouterSourceAnalysis, packageName, root string) strictRouterEntrypointCalls {
	result := strictRouterEntrypointCalls{}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(key string) {
		if visited[key] {
			return
		}
		function, ok := analysis.functions[key]
		if !ok {
			return
		}
		visited[key] = true
		result.strictCalls += function.strictCalls
		result.legacyCalls += function.legacyCalls
		for _, callee := range function.calls {
			if strings.HasPrefix(callee, strictRouterMethodTarget) {
				methodName := strings.TrimPrefix(callee, strictRouterMethodTarget)
				for candidate := range analysis.functions {
					if strings.HasPrefix(candidate, packageName+".") && strings.HasSuffix(candidate, "."+methodName) {
						visit(candidate)
					}
				}
				continue
			}
			visit(callee)
		}
	}
	rootKey := packageName + "." + root
	if _, ok := analysis.functions[rootKey]; ok {
		result.found = true
		visit(rootKey)
	}
	return result
}

func strictRouterDirectCallers(analysis strictRouterSourceAnalysis, packageName, callee string) []string {
	callers := []string{}
	prefix := packageName + "."
	target := prefix + callee
	for qualifiedName, function := range analysis.functions {
		if !strings.HasPrefix(qualifiedName, prefix) {
			continue
		}
		for _, called := range function.calls {
			if called == target {
				callers = append(callers, strings.TrimPrefix(qualifiedName, prefix))
				break
			}
		}
	}
	return callers
}

func isRouterOptionsType(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "RouterOptions"
	case *ast.SelectorExpr:
		return value.Sel.Name == "RouterOptions"
	default:
		return false
	}
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
	productionAnalysis := strictRouterSourceAnalysis{functions: map[string]strictRouterFunctionAnalysis{}}
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
			analysis := inspectStrictRouterSource(fileSet, parsed)
			mergeStrictRouterSourceAnalysis(&productionAnalysis, analysis)
			return nil
		})
		if err != nil {
			t.Fatalf("inspect production source %s: %v", root, err)
		}
	}
	for _, violation := range productionAnalysis.violations {
		t.Error(violation)
	}
	strictInventory := strictRouterEntrypointInventory(productionAnalysis, "http", "buildRuntime")
	if !strictInventory.found || strictInventory.strictCalls != 1 || strictInventory.legacyCalls != 0 {
		t.Errorf("production buildRuntime inventory=(found=%t strict=%d legacy=%d), want (true,1,0)", strictInventory.found, strictInventory.strictCalls, strictInventory.legacyCalls)
	}
	publicInventory := strictRouterEntrypointInventory(productionAnalysis, "http", "BuildRuntime")
	if !publicInventory.found || publicInventory.strictCalls != 1 || publicInventory.legacyCalls != 0 {
		t.Errorf("exported BuildRuntime inventory=(found=%t strict=%d legacy=%d), want (true,1,0)", publicInventory.found, publicInventory.strictCalls, publicInventory.legacyCalls)
	}
	compatibilityInventory := strictRouterEntrypointInventory(productionAnalysis, "http", "buildCompatibilityRuntime")
	if !compatibilityInventory.found || compatibilityInventory.strictCalls != 0 || compatibilityInventory.legacyCalls != 1 {
		t.Errorf("compatibility runtime inventory=(found=%t strict=%d legacy=%d), want (true,0,1)", compatibilityInventory.found, compatibilityInventory.strictCalls, compatibilityInventory.legacyCalls)
	}
	compatibilityCallers := strictRouterDirectCallers(productionAnalysis, "http", "buildCompatibilityRuntime")
	if len(compatibilityCallers) != 1 || compatibilityCallers[0] != "NewServer" {
		t.Errorf("buildCompatibilityRuntime callers=%#v, want []string{\"NewServer\"}", compatibilityCallers)
	}
}
