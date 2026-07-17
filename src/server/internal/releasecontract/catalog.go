package releasecontract

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type CatalogSubject struct {
	Kind    CatalogSubjectKind
	ID      string
	Runtime CatalogRuntimeClass
}

type CapabilityID string

type CatalogResolver interface {
	Resolve(CatalogSubject) (CapabilityID, error)
}

type catalogKey struct {
	digest  string
	kind    CatalogSubjectKind
	id      string
	runtime CatalogRuntimeClass
}

type compiledCatalogResolver struct {
	digest   string
	bindings map[catalogKey]CapabilityID
}

func (r compiledCatalogResolver) Resolve(subject CatalogSubject) (CapabilityID, error) {
	if strings.TrimSpace(subject.ID) == "" || !validCatalogSubjectKind(subject.Kind) || !validCatalogRuntimeClass(subject.Runtime) {
		return "", readinessError(CodeCapabilityUnknown, "catalogSubject", nil)
	}
	capabilityID, ok := r.bindings[catalogKey{digest: r.digest, kind: subject.Kind, id: subject.ID, runtime: subject.Runtime}]
	if !ok {
		return "", readinessError(CodeCapabilityUnknown, "catalogSubject", nil)
	}
	return capabilityID, nil
}

type CatalogAuthorizer struct {
	resolver CatalogResolver
	guard    Guard
}

func (a CatalogAuthorizer) ResolveAndRequire(ctx context.Context, subject CatalogSubject, boundary Boundary) (CapabilityID, error) {
	if a.resolver == nil || a.guard == nil {
		return "", readinessError(CodeReadinessUnavailable, "catalogAuthorizer", nil)
	}
	capabilityID, err := a.resolver.Resolve(subject)
	if err != nil {
		return "", err
	}
	if err := a.guard.Require(ctx, string(capabilityID), boundary); err != nil {
		return "", err
	}
	return capabilityID, nil
}

type EffectID string

const (
	EffectScheduleClaim          EffectID = "worker.schedule.claim"
	EffectScheduleWorkflow       EffectID = "worker.schedule.workflow"
	EffectScheduleAgent          EffectID = "worker.schedule.agent"
	EffectChannelRetryClaim      EffectID = "worker.channel_retry.claim"
	EffectChannelDelivery        EffectID = "worker.channel_retry.delivery"
	EffectRelayBatchClaim        EffectID = "worker.relay_batch.claim"
	EffectRelayBatchProvider     EffectID = "worker.relay_batch.provider"
	EffectArchiveClaim           EffectID = "worker.archive.claim"
	EffectArchiveWrite           EffectID = "worker.archive.write"
	EffectArchiveDelete          EffectID = "worker.archive.delete"
	EffectRelayProvider          EffectID = "relay.provider.dispatch"
	EffectChatProvider           EffectID = "chat.provider.dispatch"
	EffectMCPDispatch            EffectID = "mcp.dispatch"
	EffectToolBuiltin            EffectID = "tool.builtin.dispatch"
	EffectToolWebSearch          EffectID = "tool.web_search.dispatch"
	EffectHTTPMutation           EffectID = "http.mutation"
	EffectBillingCheckout        EffectID = "billing.checkout"
	EffectAdminRefund            EffectID = "admin.refund"
	EffectMarketplacePayout      EffectID = "marketplace.payout"
	EffectMarketplaceSettlement  EffectID = "marketplace.settlement"
	EffectAgentToolBuiltin       EffectID = "agent.tool.builtin"
	EffectAgentToolCustomAPIHTTP EffectID = "agent.tool.custom_api_http"
	EffectAgentToolLocalPython   EffectID = "agent.tool.local_python"
	EffectAgentToolPythonSandbox EffectID = "agent.tool.python_sandbox"
	EffectAgentToolMCP           EffectID = "agent.tool.mcp"
)

var authoredEffectCapabilities = map[EffectID]CapabilityID{
	EffectScheduleClaim:          "task.scheduled_execution",
	EffectScheduleWorkflow:       "workflow.graph_execution",
	EffectScheduleAgent:          "agent.run",
	EffectChannelRetryClaim:      "channel.delivery",
	EffectChannelDelivery:        "channel.delivery",
	EffectRelayBatchClaim:        "relay.provider_inference",
	EffectRelayBatchProvider:     "relay.provider_inference",
	EffectArchiveClaim:           "observability.audit",
	EffectArchiveWrite:           "observability.audit",
	EffectArchiveDelete:          "observability.audit",
	EffectRelayProvider:          "relay.provider_inference",
	EffectChatProvider:           "relay.provider_inference",
	EffectMCPDispatch:            "mcp.tool_execution",
	EffectToolBuiltin:            "mcp.tool_execution",
	EffectToolWebSearch:          "mcp.network_execution",
	EffectHTTPMutation:           "gateway.request_admission",
	EffectBillingCheckout:        "billing.payment_lifecycle",
	EffectAdminRefund:            "billing.payment_lifecycle",
	EffectMarketplacePayout:      "marketplace.commerce",
	EffectMarketplaceSettlement:  "marketplace.commerce",
	EffectAgentToolBuiltin:       "mcp.tool_execution",
	EffectAgentToolCustomAPIHTTP: "mcp.custom_execution",
	EffectAgentToolLocalPython:   "mcp.custom_execution",
	EffectAgentToolPythonSandbox: "sandbox.code_execution",
	EffectAgentToolMCP:           "mcp.tool_execution",
}

type CapabilityBindings struct {
	bindings map[EffectID]CapabilityID
}

func (b CapabilityBindings) Resolve(effect EffectID) (CapabilityID, error) {
	capabilityID, ok := b.bindings[effect]
	if !ok || strings.TrimSpace(string(effect)) == "" {
		return "", readinessError(CodeCapabilityUnknown, "effectId", nil)
	}
	return capabilityID, nil
}

type RuntimeAuthorities struct {
	CatalogAuthorizer  CatalogAuthorizer
	CapabilityBindings CapabilityBindings
	initialized        bool
}

func (a RuntimeAuthorities) Valid() bool {
	return a.initialized && a.CatalogAuthorizer.resolver != nil && a.CatalogAuthorizer.guard != nil && len(a.CapabilityBindings.bindings) > 0
}

func NewRuntimeAuthorities(contract AuthoredContractV1, profile DeploymentProfile, guard Guard) (RuntimeAuthorities, error) {
	if guard == nil {
		return RuntimeAuthorities{}, readinessError(CodeReadinessUnavailable, "guard", nil)
	}
	if err := validateReadinessProfile(contract, profile); err != nil {
		return RuntimeAuthorities{}, err
	}
	contractDigest, err := Digest(contract)
	if err != nil {
		return RuntimeAuthorities{}, readinessError(CodeBuildIdentityMismatch, "contractDigest", err)
	}
	capabilityPolicy, _, err := applicableCapabilityPolicy(contract, profile)
	if err != nil {
		return RuntimeAuthorities{}, err
	}
	capabilityIDs := make(map[string]struct{}, len(capabilityPolicy))
	for capabilityID := range capabilityPolicy {
		capabilityIDs[capabilityID] = struct{}{}
	}

	bindingByID := make(map[string]CatalogBinding, len(contract.CatalogBindings))
	for _, binding := range contract.CatalogBindings {
		if _, duplicate := bindingByID[binding.ID]; duplicate {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "catalogBindings.id", nil)
		}
		bindingByID[binding.ID] = binding
	}
	catalogBindings := make(map[catalogKey]CapabilityID, len(profile.CatalogBindingIDs))
	for _, bindingID := range profile.CatalogBindingIDs {
		binding, ok := bindingByID[bindingID]
		if !ok || !validCatalogSubjectKind(binding.SubjectKind) || !validCatalogRuntimeClass(binding.RuntimeClass) || strings.TrimSpace(binding.SubjectID) == "" {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "profile.catalogBindingIds", nil)
		}
		policy, ok := capabilityPolicy[binding.CapabilityID]
		if !ok {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "catalogBindings.capabilityId", nil)
		}
		key := catalogKey{digest: contractDigest, kind: binding.SubjectKind, id: binding.SubjectID, runtime: binding.RuntimeClass}
		if _, duplicate := catalogBindings[key]; duplicate {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "catalogBindings.ambiguous", nil)
		}
		if policy.commitment != CommitmentExcluded {
			catalogBindings[key] = CapabilityID(binding.CapabilityID)
		}
	}
	if len(catalogBindings) == 0 {
		return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "catalogBindings", nil)
	}

	effectBindings := make(map[EffectID]CapabilityID, len(authoredEffectCapabilities))
	effects := make([]EffectID, 0, len(authoredEffectCapabilities))
	for effect := range authoredEffectCapabilities {
		effects = append(effects, effect)
	}
	sort.Slice(effects, func(i, j int) bool { return effects[i] < effects[j] })
	for _, effect := range effects {
		capabilityID := authoredEffectCapabilities[effect]
		policy, ok := capabilityPolicy[string(capabilityID)]
		if !ok {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, fmt.Sprintf("effect.%s", effect), nil)
		}
		if _, authored := capabilityIDs[string(capabilityID)]; !authored {
			return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, fmt.Sprintf("effect.%s", effect), nil)
		}
		if policy.commitment != CommitmentExcluded {
			effectBindings[effect] = capabilityID
		}
	}
	if len(effectBindings) == 0 {
		return RuntimeAuthorities{}, readinessError(CodeCapabilityUnknown, "effectBindings", nil)
	}

	return RuntimeAuthorities{
		CatalogAuthorizer: CatalogAuthorizer{
			resolver: compiledCatalogResolver{digest: contractDigest, bindings: cloneCatalogBindings(catalogBindings)},
			guard:    guard,
		},
		CapabilityBindings: CapabilityBindings{bindings: cloneEffectBindings(effectBindings)},
		initialized:        true,
	}, nil
}

func cloneCatalogBindings(source map[catalogKey]CapabilityID) map[catalogKey]CapabilityID {
	result := make(map[catalogKey]CapabilityID, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneEffectBindings(source map[EffectID]CapabilityID) map[EffectID]CapabilityID {
	result := make(map[EffectID]CapabilityID, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
