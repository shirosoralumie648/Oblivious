package releasecontract_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"testing"

	serverhttp "oblivious/server/internal/http"
	"oblivious/server/internal/releasecontract"
)

func TestCatalogAuthorityContract(t *testing.T) {
	contract, profile := loadCatalogTestAuthority(t)
	guard := &catalogGuardSpy{}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile runtime authorities: %v", err)
	}

	t.Run("known subject resolves current server binding then guards", func(t *testing.T) {
		capabilityID, err := authorities.CatalogAuthorizer.ResolveAndRequire(context.Background(), releasecontract.CatalogSubject{
			Kind: releasecontract.CatalogSubjectModel, ID: "gpt-4o", Runtime: releasecontract.CatalogRuntimeServerModel,
		}, releasecontract.BoundaryOutbound)
		if err != nil {
			t.Fatalf("resolve known catalog subject: %v", err)
		}
		if capabilityID != "relay.provider_inference" {
			t.Fatalf("resolved capability = %q", capabilityID)
		}
		calls := guard.snapshot()
		if len(calls) != 1 || calls[0].capabilityID != "relay.provider_inference" || calls[0].boundary != releasecontract.BoundaryOutbound {
			t.Fatalf("guard calls = %#v", calls)
		}
	})

	t.Run("unknown and runtime-class mismatch fail before guard", func(t *testing.T) {
		before := len(guard.snapshot())
		for _, subject := range []releasecontract.CatalogSubject{
			{Kind: releasecontract.CatalogSubjectModel, ID: "deleted-model", Runtime: releasecontract.CatalogRuntimeServerModel},
			{Kind: releasecontract.CatalogSubjectModel, ID: "gpt-4o", Runtime: releasecontract.CatalogRuntimeCustom},
			{Kind: releasecontract.CatalogSubjectTool, ID: "calculator", Runtime: releasecontract.CatalogRuntimeNetwork},
			{Kind: releasecontract.CatalogSubjectRuntime, ID: "mcp", Runtime: releasecontract.CatalogRuntimeCustom},
		} {
			_, err := authorities.CatalogAuthorizer.ResolveAndRequire(context.Background(), subject, releasecontract.BoundaryOutbound)
			assertCatalogCode(t, err, releasecontract.CodeCapabilityUnknown)
		}
		if got := len(guard.snapshot()); got != before {
			t.Fatalf("resolution failures called guard %d times", got-before)
		}
	})

	t.Run("deleted binding remains unknown and ambiguous startup fails", func(t *testing.T) {
		deletedContract, deletedProfile := removeCatalogBinding(contract, profile, "model.gpt-4o")
		deleted, err := releasecontract.NewRuntimeAuthorities(deletedContract, deletedProfile, &catalogGuardSpy{})
		if err != nil {
			t.Fatalf("compile authority after authored deletion: %v", err)
		}
		_, err = deleted.CatalogAuthorizer.ResolveAndRequire(context.Background(), releasecontract.CatalogSubject{
			Kind: releasecontract.CatalogSubjectModel, ID: "gpt-4o", Runtime: releasecontract.CatalogRuntimeServerModel,
		}, releasecontract.BoundaryOutbound)
		assertCatalogCode(t, err, releasecontract.CodeCapabilityUnknown)

		ambiguousContract := contract
		ambiguousProfile := profile
		duplicate := releasecontract.CatalogBinding{
			ID: "model.gpt-4o-duplicate", SubjectKind: releasecontract.CatalogSubjectModel,
			SubjectID: "gpt-4o", RuntimeClass: releasecontract.CatalogRuntimeServerModel,
			CapabilityID: "relay.provider_inference",
		}
		ambiguousContract.CatalogBindings = append(append([]releasecontract.CatalogBinding(nil), contract.CatalogBindings...), duplicate)
		sort.Slice(ambiguousContract.CatalogBindings, func(i, j int) bool {
			return ambiguousContract.CatalogBindings[i].ID < ambiguousContract.CatalogBindings[j].ID
		})
		ambiguousProfile.CatalogBindingIDs = append(append([]string(nil), profile.CatalogBindingIDs...), duplicate.ID)
		sort.Strings(ambiguousProfile.CatalogBindingIDs)
		replaceCatalogProfile(&ambiguousContract, ambiguousProfile)
		_, err = releasecontract.NewRuntimeAuthorities(ambiguousContract, ambiguousProfile, &catalogGuardSpy{})
		assertCatalogCode(t, err, releasecontract.CodeCapabilityUnknown)
	})

	t.Run("caller and persisted capability values have no authority input", func(t *testing.T) {
		if _, ok := reflect.TypeOf(releasecontract.CatalogSubject{}).FieldByName("CapabilityID"); ok {
			t.Fatal("CatalogSubject accepts caller-supplied capability authority")
		}
		stalePersistedCapability := "admin.governance"
		_ = stalePersistedCapability
		capabilityID, err := authorities.CatalogAuthorizer.ResolveAndRequire(context.Background(), releasecontract.CatalogSubject{
			Kind: releasecontract.CatalogSubjectModel, ID: "gpt-4o-mini", Runtime: releasecontract.CatalogRuntimeServerModel,
		}, releasecontract.BoundaryOutbound)
		if err != nil || capabilityID != "relay.provider_inference" {
			t.Fatalf("current re-resolution = %q, %v", capabilityID, err)
		}
	})

	t.Run("compiled catalog is immutable after source mutation", func(t *testing.T) {
		for index := range contract.CatalogBindings {
			if contract.CatalogBindings[index].ID == "model.gpt-4o-mini" {
				contract.CatalogBindings[index].CapabilityID = "admin.governance"
			}
		}
		capabilityID, err := authorities.CatalogAuthorizer.ResolveAndRequire(context.Background(), releasecontract.CatalogSubject{
			Kind: releasecontract.CatalogSubjectModel, ID: "gpt-4o-mini", Runtime: releasecontract.CatalogRuntimeServerModel,
		}, releasecontract.BoundaryOutbound)
		if err != nil || capabilityID != "relay.provider_inference" {
			t.Fatalf("source mutation changed compiled catalog: %q, %v", capabilityID, err)
		}
	})
}

func TestRuntimeAuthoritiesContract(t *testing.T) {
	contract, profile := loadCatalogTestAuthority(t)
	guard := &catalogGuardSpy{}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile runtime authorities: %v", err)
	}
	if !authorities.Valid() {
		t.Fatal("compiled runtime authorities are not valid")
	}

	applicable := []struct {
		effect     releasecontract.EffectID
		capability releasecontract.CapabilityID
	}{
		{releasecontract.EffectScheduleClaim, "task.scheduled_execution"},
		{releasecontract.EffectScheduleWorkflow, "workflow.graph_execution"},
		{releasecontract.EffectScheduleAgent, "agent.run"},
		{releasecontract.EffectChannelRetryClaim, "channel.delivery"},
		{releasecontract.EffectChannelDelivery, "channel.delivery"},
		{releasecontract.EffectRelayBatchClaim, "relay.provider_inference"},
		{releasecontract.EffectRelayBatchProvider, "relay.provider_inference"},
		{releasecontract.EffectArchiveClaim, "observability.audit"},
		{releasecontract.EffectArchiveWrite, "observability.audit"},
		{releasecontract.EffectArchiveDelete, "observability.audit"},
		{releasecontract.EffectRelayProvider, "relay.provider_inference"},
		{releasecontract.EffectChatProvider, "relay.provider_inference"},
		{releasecontract.EffectMCPDispatch, "mcp.tool_execution"},
		{releasecontract.EffectToolBuiltin, "mcp.tool_execution"},
		{releasecontract.EffectToolWebSearch, "mcp.network_execution"},
		{releasecontract.EffectHTTPMutation, "gateway.request_admission"},
		{releasecontract.EffectBillingCheckout, "billing.payment_lifecycle"},
		{releasecontract.EffectAdminRefund, "billing.payment_lifecycle"},
		{releasecontract.EffectMarketplacePayout, "marketplace.commerce"},
		{releasecontract.EffectMarketplaceSettlement, "marketplace.commerce"},
		{releasecontract.EffectAgentToolBuiltin, "mcp.tool_execution"},
		{releasecontract.EffectAgentToolCustomAPIHTTP, "mcp.custom_execution"},
		{releasecontract.EffectAgentToolLocalPython, "mcp.custom_execution"},
		{releasecontract.EffectAgentToolMCP, "mcp.tool_execution"},
	}
	for _, tt := range applicable {
		capabilityID, err := authorities.CapabilityBindings.Resolve(tt.effect)
		if err != nil || capabilityID != tt.capability {
			t.Errorf("resolve %s = %q, %v; want %q", tt.effect, capabilityID, err, tt.capability)
		}
	}
	for _, effect := range []releasecontract.EffectID{releasecontract.EffectAgentToolPythonSandbox, "caller.effect", ""} {
		_, err := authorities.CapabilityBindings.Resolve(effect)
		assertCatalogCode(t, err, releasecontract.CodeCapabilityUnknown)
	}

	copyOfBindings := authorities.CapabilityBindings
	if capabilityID, err := copyOfBindings.Resolve(releasecontract.EffectRelayProvider); err != nil || capabilityID != "relay.provider_inference" {
		t.Fatalf("copied immutable bindings = %q, %v", capabilityID, err)
	}

	t.Run("missing authored mapping and guard fail before construction", func(t *testing.T) {
		missing := contract
		missing.Capabilities = append([]releasecontract.Capability(nil), contract.Capabilities...)
		for index, capability := range missing.Capabilities {
			if capability.ID == "agent.run" {
				missing.Capabilities = append(missing.Capabilities[:index], missing.Capabilities[index+1:]...)
				break
			}
		}
		_, err := releasecontract.NewRuntimeAuthorities(missing, profile, &catalogGuardSpy{})
		assertCatalogCode(t, err, releasecontract.CodeCapabilityUnknown)
		_, err = releasecontract.NewRuntimeAuthorities(contract, profile, nil)
		assertCatalogCode(t, err, releasecontract.CodeReadinessUnavailable)
	})

	t.Run("router options carry one complete startup authority value", func(t *testing.T) {
		options := serverhttp.RouterOptions{
			Readiness:   catalogManagerStub{},
			Guard:       guard,
			Effects:     catalogEffectRegistrar{},
			Authorities: authorities,
		}
		if err := options.ValidateReadinessAuthorities(); err != nil {
			t.Fatalf("validate complete router readiness options: %v", err)
		}
		if err := (serverhttp.RouterOptions{}).ValidateReadinessAuthorities(); err == nil {
			t.Fatal("zero router readiness options passed validation")
		}
		options.Authorities = releasecontract.RuntimeAuthorities{}
		if err := options.ValidateReadinessAuthorities(); err == nil {
			t.Fatal("missing runtime authority passed validation")
		}
	})
}

type catalogGuardCall struct {
	capabilityID string
	boundary     releasecontract.Boundary
}

type catalogGuardSpy struct {
	mu    sync.Mutex
	calls []catalogGuardCall
}

func (g *catalogGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, catalogGuardCall{capabilityID: capabilityID, boundary: boundary})
	return nil
}

func (g *catalogGuardSpy) snapshot() []catalogGuardCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]catalogGuardCall(nil), g.calls...)
}

type catalogManagerStub struct{}

func (catalogManagerStub) Bootstrap(context.Context) error { return nil }
func (catalogManagerStub) StartRefresh(context.Context)    {}
func (catalogManagerStub) Require(string) error            { return nil }
func (catalogManagerStub) Evaluate() releasecontract.Evaluation {
	return releasecontract.Evaluation{Generation: 1}
}
func (catalogManagerStub) ExportAudit(string) error { return nil }

type catalogEffectRegistrar struct{}

func (catalogEffectRegistrar) Register(releasecontract.EffectDescriptor) error { return nil }

func loadCatalogTestAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve catalog test source")
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

func removeCatalogBinding(contract releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile, bindingID string) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	bindings := make([]releasecontract.CatalogBinding, 0, len(contract.CatalogBindings))
	for _, binding := range contract.CatalogBindings {
		if binding.ID != bindingID {
			bindings = append(bindings, binding)
		}
	}
	contract.CatalogBindings = bindings
	ids := make([]string, 0, len(profile.CatalogBindingIDs))
	for _, id := range profile.CatalogBindingIDs {
		if id != bindingID {
			ids = append(ids, id)
		}
	}
	profile.CatalogBindingIDs = ids
	replaceCatalogProfile(&contract, profile)
	return contract, profile
}

func replaceCatalogProfile(contract *releasecontract.AuthoredContractV1, profile releasecontract.DeploymentProfile) {
	for index := range contract.Profiles {
		if contract.Profiles[index].ID == profile.ID {
			contract.Profiles[index] = profile
		}
	}
}

func assertCatalogCode(t *testing.T, err error, code releasecontract.ReadinessCode) {
	t.Helper()
	var readinessErr *releasecontract.ReadinessError
	if !errors.As(err, &readinessErr) || readinessErr.Code != code {
		t.Fatalf("error = %T %v, want code %q", err, err, code)
	}
}
