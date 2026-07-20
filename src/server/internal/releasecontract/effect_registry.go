package releasecontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const EffectSurfaceSchemaV1 = "readiness-effect-surface/v1"

type EffectCoverageError struct {
	Code  string
	Field string
	Err   error
}

func (e *EffectCoverageError) Error() string {
	if e == nil {
		return "effect_coverage_error"
	}
	if e.Field == "" {
		return e.Code
	}
	return e.Code + ": field=" + e.Field
}

func (e *EffectCoverageError) Unwrap() error { return e.Err }

func IsEffectCoverageCode(err error, code string) bool {
	var coverageErr *EffectCoverageError
	return errors.As(err, &coverageErr) && coverageErr.Code == code
}

// EffectSurface is the independent expected/static inventory row. It contains
// no commitment or profile authority; profileDisposition only classifies how a
// row is expected to behave in the selected profile.
type EffectSurface struct {
	SeamID             string                       `json:"seamId"`
	DescriptorID       string                       `json:"descriptorId,omitempty"`
	EffectID           EffectID                     `json:"effectId,omitempty"`
	OwnerPackage       string                       `json:"ownerPackage"`
	OwnerSymbol        string                       `json:"ownerSymbol"`
	CapabilityID       string                       `json:"capabilityId"`
	Boundary           Boundary                     `json:"boundary"`
	ASTCall            string                       `json:"astCall"`
	RegistrationSymbol string                       `json:"registrationSymbol,omitempty"`
	GuardCall          string                       `json:"guardCall,omitempty"`
	EffectCall         string                       `json:"effectCall,omitempty"`
	Configuration      *EffectConfigurationSelector `json:"configurationSelector,omitempty"`
	EntrypointID       string                       `json:"entrypointId,omitempty"`
	ProfileDisposition Commitment                   `json:"profileDisposition"`
}

type EffectConfigurationSelector struct {
	Group string `json:"group"`
	Value string `json:"value"`
	Mode  string `json:"mode"`
}

const (
	effectSelectorOneOf    = "one_of"
	effectSelectorOptional = "optional"
)

// EffectRuntimeConfiguration is the explicit production projection used by
// coverage verification. A false optional cohort is an authored disablement,
// not an inference from missing runtime descriptors.
type EffectRuntimeConfiguration struct {
	ScheduleWorkerEnabled bool
	ChannelArchiveEnabled bool
	RelayBatchEnabled     bool
	RelayRuntimeEnabled   bool
	ChatFallbackEnabled   bool
	WebSearchProvider     string
}

type runtimeDescriptorSpec struct {
	EffectID           EffectID
	CapabilityID       CapabilityID
	Owner              string
	Boundary           Boundary
	OwnerPackage       string
	RegistrationSymbol string
	RegistrationAnchor string
	ResolverCall       string
	RegistrarCall      string
	GuardCall          string
	EffectCall         string
	Configuration      *EffectConfigurationSelector
	Disposition        Commitment
}

func descriptorSpec(effect EffectID, owner string, boundary Boundary, ownerPackage, registrationSymbol, registrationAnchor, guardCall, effectCall string, disposition Commitment, configuration *EffectConfigurationSelector) runtimeDescriptorSpec {
	resolverCall, registrarCall := structuralAuthorityCalls(registrationSymbol)
	return runtimeDescriptorSpec{
		EffectID: effect, CapabilityID: authoredEffectCapabilities[effect], Owner: owner, Boundary: boundary,
		OwnerPackage: ownerPackage, RegistrationSymbol: registrationSymbol, RegistrationAnchor: registrationAnchor,
		ResolverCall: resolverCall, RegistrarCall: registrarCall,
		GuardCall: guardCall, EffectCall: effectCall, Configuration: configuration, Disposition: disposition,
	}
}

// structuralAuthorityCalls records the exact startup authority and registrar
// receivers used by each producer family. The returned paths are joined against
// parsed identifier objects, so a same-named shadow or another receiver cannot
// satisfy a descriptor registration.
func structuralAuthorityCalls(registrationSymbol string) (string, string) {
	parts := strings.Split(registrationSymbol, ">")
	final := parts[len(parts)-1]
	switch final {
	case "newScheduledReadiness", "newChannelReadiness", "newArchiveReadiness", "newBatchPollingReadiness":
		return "authorities.CapabilityBindings.Resolve", "effects.Register"
	case "newAdminFinancialReadiness", "newCheckoutFinancialReadiness", "WithMarketplaceFinancialReadiness":
		return "financial.Authorities.CapabilityBindings.Resolve", "financial.Effects.Register"
	default:
		return "options.Authorities.CapabilityBindings.Resolve", "options.Effects.Register"
	}
}

func selector(group, value, mode string) *EffectConfigurationSelector {
	return &EffectConfigurationSelector{Group: group, Value: value, Mode: mode}
}

// runtimeDescriptorSpecs is the sole runtime descriptor allowlist and the
// sole descriptor-to-EffectID mapping. Source discovery and runtime joining
// consume the same exact owner, boundary, capability, and selection contract.
var runtimeDescriptorSpecs = map[string]runtimeDescriptorSpec{
	"worker.schedule.claim":             descriptorSpec(EffectScheduleClaim, "schedule.Worker", BoundaryWorkerClaim, "src/server/internal/schedule", "newScheduledReadiness", "", "Service.runDueTasks:readiness.requireClaim", "Service.runDueTasks:dueStore.ClaimDueScheduledTaskRuns", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.workflow.start":    descriptorSpec(EffectScheduleWorkflow, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "Service.runClaimedWorkflowTask:readiness.requireWorkflowEffect", "Service.runClaimedWorkflowTask:s.startClaimedWorkflow", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.workflow.continue": descriptorSpec(EffectScheduleWorkflow, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "Service.runClaimedWorkflowTask:readiness.requireWorkflowEffect", "Service.runClaimedWorkflowTask:s.workflowStarter.RunExecutionUntilBlocked", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.agent.start":       descriptorSpec(EffectScheduleAgent, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "Service.runClaimedAgentTask:readiness.requireAgentEffect", "Service.runClaimedAgentTask:s.agentStarter.StartRun", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),

	"worker.channel_retry.claim": descriptorSpec(EffectChannelRetryClaim, "channel.Service", BoundaryWorkerClaim, "src/server/internal/channel", "newChannelReadiness", "", "Service.ClaimDueRetryMessages:s.readiness.requireRetryClaim", "Service.ClaimDueRetryMessages:store.ClaimDueRetryMessages", CommitmentCommitted, nil),
	"channel.delivery.send":      descriptorSpec(EffectChannelDelivery, "channel.Service", BoundaryOutbound, "src/server/internal/channel", "newChannelReadiness", "", "Service.Send:s.readiness.requireDelivery", "Service.Send:deliverer.DeliverOutbound", CommitmentCommitted, nil),
	"worker.archive.claim":       descriptorSpec(EffectArchiveClaim, "channel.ArchiveWorker", BoundaryWorkerClaim, "src/server/internal/channel", "newArchiveReadiness", "", "Service.archiveExpiredMessageLogs:readiness.requireClaim", "Service.archiveExpiredMessageLogs:store.ListExpiredMessageLogsForArchive", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),
	"worker.archive.write":       descriptorSpec(EffectArchiveWrite, "channel.Service", BoundaryWorkerEffect, "src/server/internal/channel", "newArchiveReadiness", "", "Service.archiveExpiredMessageLogs:readiness.requireWrite", "Service.archiveExpiredMessageLogs:sink.ArchiveMessageLogs", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),
	"worker.archive.delete":      descriptorSpec(EffectArchiveDelete, "channel.Service", BoundaryWorkerEffect, "src/server/internal/channel", "newArchiveReadiness", "", "Service.archiveExpiredMessageLogs:readiness.requireDelete", "Service.archiveExpiredMessageLogs:store.DeleteArchivedMessageLogs", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),

	"worker.relay_batch.claim":             descriptorSpec(EffectRelayBatchClaim, "relay.BatchPollingWorker", BoundaryWorkerClaim, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.runOnce:w.readiness.requireClaim", "BatchPollingWorker.runOnce:w.store.ClaimBatchPollingJobs", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.retrieve":          descriptorSpec(EffectRelayBatchProvider, "relay.BatchPollingWorker", BoundaryOutbound, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.runOnce:w.readiness.requireProvider", "BatchPollingWorker.runOnce:w.client.RetrieveBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.complete.finalize": descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.recordBatchStatus:w.readiness.requireFinalizer", "BatchPollingWorker.recordBatchStatus:w.completionFinalizer.FinalizeCompletedBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.failure.finalize":  descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.recordBatchStatus:w.readiness.requireFinalizer", "BatchPollingWorker.recordBatchStatus:w.failureFinalizer.FinalizeFailedBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.succeeded":         descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.recordBatchStatus:w.readiness.requireTerminal", "BatchPollingWorker.recordBatchStatus:w.store.MarkBatchPollingJobSucceeded", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.dead_letter":       descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "BatchPollingWorker.recordBatchStatus:w.readiness.requireTerminal", "BatchPollingWorker.recordBatchStatus>BatchPollingWorker.recordTerminalFailedJob:w.store.MarkBatchPollingJobDeadLetter", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),

	"marketplace.settlement.intent": descriptorSpec(EffectMarketplaceSettlement, "marketplace.SettlementService", BoundaryFinancial, "src/server/internal/marketplace", "WithMarketplaceFinancialReadiness", "", "SettlementService.CreatePaidInstallCheckout:s.requireSettlement", "SettlementService.CreatePaidInstallCheckout:s.store.db.BeginTx", CommitmentCommitted, nil),
	"marketplace.payout.dispatch":   descriptorSpec(EffectMarketplacePayout, "marketplace.SettlementService", BoundaryFinancial, "src/server/internal/marketplace", "WithMarketplaceFinancialReadiness", "", "SettlementService.dispatchPersistedPayout:s.requirePayout", "SettlementService.dispatchPersistedPayout>SettlementService.dispatchPayout:s.payoutProvider.CreatePayout", CommitmentCommitted, nil),
	"relay.provider.dispatch":       descriptorSpec(EffectRelayProvider, "relay.Router", BoundaryOutbound, "src/server/internal/relay", "newRouterReadiness", "", "Router.Route:r.requireProvider", "Router.Route:fn", CommitmentCommitted, selector("relay.runtime", "enabled", effectSelectorOptional)),
	"chat.provider.dispatch":        descriptorSpec(EffectChatProvider, "chat.RelayGateway", BoundaryOutbound, "src/server/internal/chat", "NewRelayGatewayWithOptions>newRelayGatewayReadiness", "", "RelayGateway.complete:g.readiness.requireDispatch", "RelayGateway.complete:g.httpClient.Do", CommitmentCommitted, nil),
	"chat.provider.fallback":        descriptorSpec(EffectChatProvider, "chat.CompositeGateway", BoundaryOutbound, "src/server/internal/chat", "NewCompositeGatewayWithOptions>newRelayGatewayReadiness", "", "CompositeGateway.GenerateReply:g.readiness.requireDispatch", "CompositeGateway.GenerateReply:g.fallback.GenerateReply", CommitmentCommitted, selector("chat.fallback", "enabled", effectSelectorOptional)),

	"admin.channel.model.mutation": descriptorSpec(EffectHTTPMutation, "admin.channel_service", BoundaryHTTP, "src/server/internal/admin", "newModelCatalogReadiness", "", "Service.SyncChannelModels:s.modelReadiness.requireMutation", "Service.SyncChannelModels:s.store.TestChannel", CommitmentCommitted, nil),
	"http.admin.refund":            descriptorSpec(EffectAdminRefund, "http.adminHandler", BoundaryFinancial, "src/server/internal/http", "newAdminFinancialReadiness", "", "adminHandler.recordTopupRefund:h.financial.require", "adminHandler.recordTopupRefund:h.service.RecordTopupRefund", CommitmentCommitted, nil),
	"http.billing.checkout":        descriptorSpec(EffectBillingCheckout, "http.billingHandler", BoundaryFinancial, "src/server/internal/http", "newBillingFinancialReadiness>newCheckoutFinancialReadiness", "", "billingHandler.checkout:h.readiness.require", "billingHandler.checkout:checkoutCreator.CreateCheckoutSession", CommitmentCommitted, nil),
	"http.marketplace.checkout":    descriptorSpec(EffectBillingCheckout, "http.marketplaceHandler", BoundaryFinancial, "src/server/internal/http", "withMarketplaceFinancialReadiness>newCheckoutFinancialReadiness", "", "marketplaceHandler.createPaidInstallCheckout:h.readiness.require", "marketplaceHandler.createPaidInstallCheckout:checkoutCreator.CreateCheckoutSession", CommitmentCommitted, nil),
	"http.mcp.mutation":            descriptorSpec(EffectHTTPMutation, "http.mcpHandler", BoundaryHTTP, "src/server/internal/http", "newMCPMutationReadiness", "", "mcpHandler.addServer:h.readiness.authorize", "mcpHandler.addServer:h.client.AddServer", CommitmentCommitted, nil),

	"mcp.transport.dispatch": descriptorSpec(EffectMCPDispatch, "mcp.Client", BoundaryOutbound, "src/server/internal/mcp", "newClientReadiness", "", "Client.sendRequest:c.readiness.authorize", "Client.sendRequest:c.httpClient.Do", CommitmentCommitted, nil),
	"mcp.websearch.builtin":  descriptorSpec(EffectToolWebSearch, "mcp.WebSearchTool", BoundaryOutbound, "src/server/internal/mcp", "NewWebSearchTool>newWebSearchReadiness", "", "authorizedWebSearchTool.Execute:t.readiness.authorize", "authorizedWebSearchTool.Execute:t.provider.Search", CommitmentConditional, nil),
	"mcp.websearch.tavily":   descriptorSpec(EffectToolWebSearch, "mcp.TavilyWebSearchProvider", BoundaryOutbound, "src/server/internal/mcp", "newTavilyWebSearchProvider>newWebSearchReadiness", "", "tavilyWebSearchProvider.Search:p.readiness.authorize", "tavilyWebSearchProvider.Search:p.client.Do", CommitmentConditional, selector("websearch.provider", "tavily", effectSelectorOneOf)),
	"mcp.websearch.chain":    descriptorSpec(EffectToolWebSearch, "mcp.websearch.Chain", BoundaryOutbound, "src/server/internal/mcp/websearch", "NewProviderFromConfig>newSearchReadiness", "", "Chain.Search:c.readiness.authorize", "Chain.Search:provider.Search", CommitmentConditional, selector("websearch.provider", "chain", effectSelectorOneOf)),
	"mcp.websearch.provider": descriptorSpec(EffectToolWebSearch, "mcp.websearch.Provider", BoundaryOutbound, "src/server/internal/mcp/websearch", "NewProviderFromConfig>newSearchReadiness", "", "guardedProvider.Search:p.readiness.authorize", "guardedProvider.Search:p.provider.Search", CommitmentConditional, selector("websearch.provider", "provider", effectSelectorOneOf)),

	"agent.tool.builtin":         descriptorSpec(EffectAgentToolBuiltin, "agent.ToolExecutor.builtin", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolBuiltin", "ToolExecutor.executeBuiltin:e.authorizeTool", "ToolExecutor.executeBuiltin:tool.Execute", CommitmentCommitted, nil),
	"agent.tool.custom_api_http": descriptorSpec(EffectAgentToolCustomAPIHTTP, "agent.ToolExecutor.custom_api_http", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolCustomAPIHTTP", "ToolExecutor.executeCustomAPI:e.authorizeTool", "ToolExecutor.executeCustomAPI:client.Do", CommitmentConditional, nil),
	"agent.tool.local_python":    descriptorSpec(EffectAgentToolLocalPython, "agent.ToolExecutor.local_python", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolLocalPython", "ToolExecutor.executeCustomPython:e.authorizeTool", "ToolExecutor.executeCustomPython:e.pythonProcessRunner", CommitmentConditional, nil),
	"agent.tool.python_sandbox":  descriptorSpec(EffectAgentToolPythonSandbox, "agent.ToolExecutor.python_sandbox", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolPythonSandbox", "ToolExecutor.executeCustomPythonSandbox:e.authorizeTool", "ToolExecutor.executeCustomPythonSandbox:e.customPythonSandboxRunner.RunCustomPython", CommitmentExcluded, nil),
	"agent.tool.mcp":             descriptorSpec(EffectAgentToolMCP, "agent.ToolExecutor.mcp", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolMCP", "ToolExecutor.executeMCP:e.authorizeTool", "ToolExecutor.executeMCP:e.mcpClient.CallTool", CommitmentCommitted, nil),

	"agent.tool.registry.builtin": descriptorSpec(EffectAgentToolBuiltin, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryBuiltin", "Registry.Execute:r.readiness.authorize", "Registry.Execute:entry.executor", CommitmentExcluded, nil),
	"agent.tool.registry.custom":  descriptorSpec(EffectAgentToolCustomAPIHTTP, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryCustom", "Registry.Execute:r.readiness.authorize", "Registry.Execute:entry.executor", CommitmentExcluded, nil),
	"agent.tool.registry.mcp":     descriptorSpec(EffectAgentToolMCP, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryMCP", "Registry.Execute:r.readiness.authorize", "Registry.Execute:entry.executor", CommitmentExcluded, nil),
	"agent.tool.web_search":       descriptorSpec(EffectToolWebSearch, "agent.tools.WebsearchTool", BoundaryOutbound, "src/server/internal/agent/tools", "newWebsearchReadiness", "", "WebsearchTool.Execute:t.readiness.authorize", "WebsearchTool.Execute:provider.Search", CommitmentExcluded, nil),
}

type EffectSurfaceManifest struct {
	SchemaVersion string          `json:"schemaVersion"`
	Surfaces      []EffectSurface `json:"surfaces"`
}

// EffectRegistry is a construction-time evidence registry. It is deliberately
// not consulted by per-request authorization paths.
type EffectRegistry struct {
	mu          sync.RWMutex
	descriptors map[string]EffectDescriptor
}

func NewEffectRegistry(_ ...EffectSurfaceManifest) *EffectRegistry {
	return &EffectRegistry{descriptors: make(map[string]EffectDescriptor)}
}

func (r *EffectRegistry) Register(descriptor EffectDescriptor) error {
	if r == nil {
		return &EffectCoverageError{Code: "effect_registry_unavailable"}
	}
	if strings.TrimSpace(descriptor.ID) == "" || strings.TrimSpace(descriptor.CapabilityID) == "" || strings.TrimSpace(descriptor.Owner) == "" || !validBoundary(descriptor.Boundary) {
		return &EffectCoverageError{Code: "effect_registry_invalid", Field: descriptor.ID}
	}
	spec, ok := runtimeDescriptorSpecs[descriptor.ID]
	if !ok {
		return &EffectCoverageError{Code: "effect_registry_unknown", Field: descriptor.ID}
	}
	if descriptor.Owner != spec.Owner {
		return &EffectCoverageError{Code: "effect_registry_owner_mismatch", Field: descriptor.ID}
	}
	if descriptor.Boundary != spec.Boundary {
		return &EffectCoverageError{Code: "effect_registry_boundary_mismatch", Field: descriptor.ID}
	}
	if descriptor.CapabilityID != string(spec.CapabilityID) {
		return &EffectCoverageError{Code: "effect_registry_capability_mismatch", Field: descriptor.ID}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[descriptor.ID]; exists {
		return &EffectCoverageError{Code: "effect_registry_duplicate", Field: descriptor.ID}
	}
	r.descriptors[descriptor.ID] = descriptor
	return nil
}

func (r *EffectRegistry) Snapshot() []EffectDescriptor {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]EffectDescriptor, 0, len(r.descriptors))
	for _, descriptor := range r.descriptors {
		result = append(result, descriptor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (r *EffectRegistry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.descriptors)
}

func LoadEffectSurfaceManifest(path string) (EffectSurfaceManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return EffectSurfaceManifest{}, &EffectCoverageError{Code: "effect_manifest_unreadable", Field: path, Err: err}
	}
	var manifest EffectSurfaceManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return EffectSurfaceManifest{}, &EffectCoverageError{Code: "effect_manifest_invalid", Err: err}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return EffectSurfaceManifest{}, &EffectCoverageError{Code: "effect_manifest_trailing", Err: err}
	}
	if err := validateEffectSurfaceManifest(manifest); err != nil {
		return EffectSurfaceManifest{}, err
	}
	return manifest, nil
}

func validateEffectSurfaceManifest(manifest EffectSurfaceManifest) error {
	return validateEffectSurfaceManifestAgainstSpecs(manifest, runtimeDescriptorSpecs)
}

func validateEffectSurfaceManifestAgainstSpecs(manifest EffectSurfaceManifest, specs map[string]runtimeDescriptorSpec) error {
	if manifest.SchemaVersion != EffectSurfaceSchemaV1 || len(manifest.Surfaces) == 0 {
		return &EffectCoverageError{Code: "effect_manifest_empty"}
	}
	surfaces := append([]EffectSurface(nil), manifest.Surfaces...)
	sort.SliceStable(surfaces, func(i, j int) bool {
		left, right := surfaces[i].DescriptorID, surfaces[j].DescriptorID
		if left == "" {
			left = surfaces[i].SeamID
		}
		if right == "" {
			right = surfaces[j].SeamID
		}
		if left != right {
			return left < right
		}
		return surfaces[i].SeamID < surfaces[j].SeamID
	})
	seen := make(map[string]struct{}, len(manifest.Surfaces))
	descriptors := make(map[string]struct{}, len(specs))
	for _, surface := range surfaces {
		if strings.TrimSpace(surface.SeamID) == "" || strings.TrimSpace(surface.OwnerPackage) == "" || strings.TrimSpace(surface.OwnerSymbol) == "" || strings.TrimSpace(surface.CapabilityID) == "" || !validBoundary(surface.Boundary) || !validCommitment(surface.ProfileDisposition) {
			return &EffectCoverageError{Code: "effect_manifest_invalid", Field: surface.SeamID}
		}
		if _, duplicate := seen[surface.SeamID]; duplicate {
			return &EffectCoverageError{Code: "effect_manifest_duplicate", Field: surface.SeamID}
		}
		seen[surface.SeamID] = struct{}{}
		if surface.DescriptorID == "" {
			continue
		}
		if _, duplicate := descriptors[surface.DescriptorID]; duplicate {
			return &EffectCoverageError{Code: "effect_manifest_duplicate_descriptor", Field: surface.DescriptorID}
		}
		descriptors[surface.DescriptorID] = struct{}{}
		spec, ok := specs[surface.DescriptorID]
		if !ok {
			return &EffectCoverageError{Code: "effect_manifest_unknown_descriptor", Field: surface.DescriptorID}
		}
		if strings.TrimSpace(surface.RegistrationSymbol) == "" || strings.TrimSpace(surface.GuardCall) == "" || strings.TrimSpace(surface.EffectCall) == "" {
			return &EffectCoverageError{Code: "effect_manifest_structural_metadata_missing", Field: surface.DescriptorID}
		}
		if surface.Configuration != nil && (strings.TrimSpace(surface.Configuration.Group) == "" || strings.TrimSpace(surface.Configuration.Value) == "" || (surface.Configuration.Mode != effectSelectorOneOf && surface.Configuration.Mode != effectSelectorOptional)) {
			return &EffectCoverageError{Code: "effect_manifest_selector_invalid", Field: surface.DescriptorID}
		}
		if surface.SeamID != surface.DescriptorID+"@"+spec.Owner || surface.EffectID != spec.EffectID || surface.OwnerPackage != spec.OwnerPackage || surface.OwnerSymbol != spec.Owner || surface.CapabilityID != string(spec.CapabilityID) || surface.Boundary != spec.Boundary || surface.ASTCall != "structural" || surface.RegistrationSymbol != spec.RegistrationSymbol || surface.GuardCall != spec.GuardCall || surface.EffectCall != spec.EffectCall || surface.ProfileDisposition != spec.Disposition || !equalEffectSelector(surface.Configuration, spec.Configuration) {
			return &EffectCoverageError{Code: "effect_manifest_descriptor_drift", Field: surface.DescriptorID}
		}
	}
	descriptorIDs := sortedRuntimeDescriptorSpecIDs(specs)
	for _, descriptorID := range descriptorIDs {
		if _, ok := descriptors[descriptorID]; !ok {
			return &EffectCoverageError{Code: "effect_manifest_missing_descriptor", Field: descriptorID}
		}
	}
	return nil
}

func sortedRuntimeDescriptorSpecIDs(specs map[string]runtimeDescriptorSpec) []string {
	descriptorIDs := make([]string, 0, len(specs))
	for descriptorID := range specs {
		descriptorIDs = append(descriptorIDs, descriptorID)
	}
	sort.Strings(descriptorIDs)
	return descriptorIDs
}

func equalEffectSelector(left, right *EffectConfigurationSelector) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Group == right.Group && left.Value == right.Value && left.Mode == right.Mode
}

type discoveredEffect struct {
	EffectSurface
	File string `json:"file"`
	Line int    `json:"line"`
}

// DiscoverEffectSurfaces proves exact registrations and guard-before-effect
// call chains from parsed production Go syntax.
func DiscoverEffectSurfaces(repoRoot string) ([]EffectSurface, error) {
	if strings.TrimSpace(repoRoot) == "" || !filepath.IsAbs(repoRoot) {
		return nil, &EffectCoverageError{Code: "effect_discovery_repo_invalid"}
	}
	fset := token.NewFileSet()
	var discovered []discoveredEffect
	packageDirectories := make(map[string]struct{})
	err := filepath.WalkDir(filepath.Join(repoRoot, "src", "server"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == ".tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		packageDirectories[filepath.Dir(path)] = struct{}{}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isRunEntrypointCall(call) {
				return true
			}
			entrypoint := runEntrypointLiteral(call)
			if entrypoint == "" {
				return true
			}
			position := fset.Position(call.Pos())
			relative, _ := filepath.Rel(repoRoot, path)
			disposition := CommitmentExcluded
			if entrypoint == "server" {
				disposition = CommitmentCommitted
			}
			discovered = append(discovered, discoveredEffect{EffectSurface: EffectSurface{
				SeamID: "entrypoint." + entrypoint, OwnerPackage: filepath.ToSlash(filepath.Dir(relative)), OwnerSymbol: "main",
				CapabilityID: "deployment.operations", Boundary: BoundaryOperation, ASTCall: "RunEntrypoint", EntrypointID: entrypoint, ProfileDisposition: disposition,
			}, File: relative, Line: position.Line})
			return true
		})
		return nil
	})
	if err != nil {
		return nil, &EffectCoverageError{Code: "effect_discovery_failed", Err: err}
	}
	effectValues, err := effectConstantValues(repoRoot)
	if err != nil {
		return nil, &EffectCoverageError{Code: "effect_discovery_failed", Err: err}
	}
	directories := make([]string, 0, len(packageDirectories))
	for directory := range packageDirectories {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	var registrations []sourceRegistration
	for _, directory := range directories {
		packageRegistrations, err := discoverPackageRegistrations(repoRoot, directory, effectValues)
		if err != nil {
			return nil, &EffectCoverageError{Code: "effect_discovery_failed", Err: err}
		}
		registrations = append(registrations, packageRegistrations...)
	}
	for _, registration := range registrations {
		spec, known := runtimeDescriptorSpecs[registration.DescriptorID]
		if !known {
			discovered = append(discovered, discoveredEffect{EffectSurface: EffectSurface{
				SeamID: registration.DescriptorID + "@" + registration.Owner, DescriptorID: registration.DescriptorID,
				OwnerPackage: registration.OwnerPackage, OwnerSymbol: registration.Owner, CapabilityID: registration.CapabilityID,
				Boundary: registration.Boundary, ASTCall: "structural", RegistrationSymbol: registration.RegistrationSymbol,
				ProfileDisposition: CommitmentCommitted,
			}, File: registration.File, Line: registration.Line})
			continue
		}
		present, err := descriptorRegistrationStructurePresent(repoRoot, registration, spec)
		if err != nil {
			return nil, &EffectCoverageError{Code: "effect_discovery_failed", Field: registration.DescriptorID, Err: err}
		}
		if present {
			discovered = append(discovered, discoveredEffect{EffectSurface: effectSurfaceForSpec(registration.DescriptorID, spec), File: registration.File, Line: registration.Line})
		}
	}
	sort.Slice(discovered, func(i, j int) bool {
		if discovered[i].SeamID != discovered[j].SeamID {
			return discovered[i].SeamID < discovered[j].SeamID
		}
		return discovered[i].File < discovered[j].File
	})
	result := make([]EffectSurface, 0, len(discovered))
	for _, value := range discovered {
		result = append(result, value.EffectSurface)
	}
	return result, nil
}

func effectSurfaceForSpec(descriptorID string, spec runtimeDescriptorSpec) EffectSurface {
	return EffectSurface{
		SeamID: descriptorID + "@" + spec.Owner, DescriptorID: descriptorID, EffectID: spec.EffectID,
		OwnerPackage: spec.OwnerPackage, OwnerSymbol: spec.Owner, CapabilityID: string(spec.CapabilityID), Boundary: spec.Boundary,
		ASTCall: "structural", RegistrationSymbol: spec.RegistrationSymbol, GuardCall: spec.GuardCall, EffectCall: spec.EffectCall,
		Configuration: cloneEffectSelector(spec.Configuration), ProfileDisposition: spec.Disposition,
	}
}

func cloneEffectSelector(source *EffectConfigurationSelector) *EffectConfigurationSelector {
	if source == nil {
		return nil
	}
	clone := *source
	return &clone
}

func isRunEntrypointCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel != nil && selector.Sel.Name == "RunEntrypoint"
}

func runEntrypointLiteral(call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	var value string
	ast.Inspect(call.Args[1], func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING && value == "" {
			value, _ = strconv.Unquote(literal.Value)
			return false
		}
		return true
	})
	return value
}

type sourceFunction struct {
	declaration *ast.FuncDecl
	receiver    string
	receiverVar string
	rootObjects map[string]*ast.Object
	calls       []sourceCall
}

type sourceCall struct {
	name        string
	path        string
	object      *ast.Object
	localTarget string
	position    token.Pos
	node        *ast.CallExpr
	ambiguous   bool
}

type sourceRegistration struct {
	DescriptorID       string
	EffectID           EffectID
	OwnerPackage       string
	Owner              string
	CapabilityID       string
	Boundary           Boundary
	RegistrationSymbol string
	ResolverCall       string
	RegistrarCall      string
	File               string
	Line               int
}

type sourceRegistrationEnvironment struct {
	values             map[string]string
	registrationSymbol string
	file               string
	line               int
}

type sourceCapabilityBinding struct {
	reference       string
	object          *ast.Object
	effectReference string
	effectObject    *ast.Object
	errorObject     *ast.Object
	resolverPath    string
	resolverObject  *ast.Object
	position        token.Pos
}

type sourceRegistrarCall struct {
	path     string
	object   *ast.Object
	position token.Pos
}

type packageSourceFunction struct {
	declaration *ast.FuncDecl
	file        string
}

func effectConstantValues(repoRoot string) (map[string]string, error) {
	values := make(map[string]string)
	path := filepath.Join(repoRoot, "src", "server", "internal", "releasecontract", "catalog.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return values, nil
		}
		return nil, err
	}
	for _, declaration := range file.Decls {
		constants, ok := declaration.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, declarationSpec := range constants.Specs {
			valueSpec, ok := declarationSpec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, expression := range valueSpec.Values {
				if index >= len(valueSpec.Names) {
					continue
				}
				literal, ok := expression.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(literal.Value)
				if err == nil {
					values[valueSpec.Names[index].Name] = value
				}
			}
		}
	}
	return values, nil
}

func discoverPackageRegistrations(repoRoot, directory string, effectValues map[string]string) ([]sourceRegistration, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	constants := make(map[string]string)
	var files []*ast.File
	filePaths := make(map[*ast.File]string)
	var functions []packageSourceFunction
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
		filePaths[file] = path
	}
	if packageDeclaresString(files) {
		markPackageStringShadow(files)
	}
	for _, file := range files {
		collectStringConstants(file, constants)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Body != nil {
				functions = append(functions, packageSourceFunction{declaration: function, file: filePaths[file]})
			}
		}
	}
	functions = registrationAnalysisFunctions(functions)

	relativeDirectory, err := filepath.Rel(repoRoot, directory)
	if err != nil {
		return nil, err
	}
	ownerPackage := filepath.ToSlash(relativeDirectory)
	var registrations []sourceRegistration
	seen := make(map[string]struct{})
	for _, function := range functions {
		if !functionCallsRegister(function.declaration) {
			continue
		}
		resolved := resolvedCapabilityBindings(function.declaration)
		inspectReachable(function.declaration.Body, func(node ast.Node) {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isDescriptorLiteral(literal) {
				return
			}
			registrar := descriptorLiteralRegistrar(function.declaration, literal)
			if registrar == nil {
				return
			}
			position := fset.Position(literal.Pos())
			environments := []sourceRegistrationEnvironment{{
				registrationSymbol: packageFunctionSymbol(function.declaration), file: function.file, line: position.Line,
			}}
			environments = append(environments, descriptorParameterEnvironments(function, literal, functions, fset, constants, effectValues)...)
			for _, values := range descriptorRangeEnvironments(function.declaration, literal, constants, effectValues) {
				environments = append(environments, sourceRegistrationEnvironment{
					values: values, registrationSymbol: packageFunctionSymbol(function.declaration), file: function.file, line: position.Line,
				})
			}
			for _, environment := range environments {
				registration, ok := sourceRegistrationFromLiteral(repoRoot, literal, ownerPackage, environment, constants, effectValues, resolved, registrar)
				if !ok {
					continue
				}
				key := registration.DescriptorID + "\x00" + string(registration.EffectID) + "\x00" + registration.Owner + "\x00" + registration.RegistrationSymbol + "\x00" + registration.File + "\x00" + strconv.Itoa(registration.Line)
				if _, duplicate := seen[key]; duplicate {
					continue
				}
				seen[key] = struct{}{}
				registrations = append(registrations, registration)
			}
		})
	}
	sort.Slice(registrations, func(i, j int) bool {
		if registrations[i].DescriptorID != registrations[j].DescriptorID {
			return registrations[i].DescriptorID < registrations[j].DescriptorID
		}
		if registrations[i].File != registrations[j].File {
			return registrations[i].File < registrations[j].File
		}
		return registrations[i].Line < registrations[j].Line
	})
	return registrations, nil
}

// registrationAnalysisFunctions exposes a returned function body only when
// package structure proves that the returned value is invoked. This keeps the
// generic reachability walker fail-closed for ordinary uninvoked closures while
// allowing construction-time option functions to contribute registrations.
func registrationAnalysisFunctions(functions []packageSourceFunction) []packageSourceFunction {
	analysis := append([]packageSourceFunction(nil), functions...)
	context := newRegistrationAnalysisContext()
	for _, function := range functions {
		for _, literal := range invokedReturnedFunctionLiterals(function, functions, context) {
			declaration := *function.declaration
			declaration.Body = literal.Body
			analysis = append(analysis, packageSourceFunction{declaration: &declaration, file: function.file})
		}
	}
	return analysis
}

type registrationAnalysisContext struct {
	parents map[*ast.FuncDecl]map[ast.Node]ast.Node
	calls   map[*ast.FuncDecl][]*ast.CallExpr
}

func newRegistrationAnalysisContext() *registrationAnalysisContext {
	return &registrationAnalysisContext{
		parents: make(map[*ast.FuncDecl]map[ast.Node]ast.Node),
		calls:   make(map[*ast.FuncDecl][]*ast.CallExpr),
	}
}

func (context *registrationAnalysisContext) parentsFor(function *ast.FuncDecl) map[ast.Node]ast.Node {
	if parents, ok := context.parents[function]; ok {
		return parents
	}
	parents := astParentMap(function.Body)
	context.parents[function] = parents
	return parents
}

func (context *registrationAnalysisContext) callsFor(function *ast.FuncDecl) []*ast.CallExpr {
	if calls, ok := context.calls[function]; ok {
		return calls
	}
	var calls []*ast.CallExpr
	inspectReachableWithParents(function.Body, context.parentsFor(function), func(node ast.Node) {
		if call, ok := node.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
	})
	context.calls[function] = calls
	return calls
}

func invokedReturnedFunctionLiterals(producer packageSourceFunction, functions []packageSourceFunction, context *registrationAnalysisContext) []*ast.FuncLit {
	if producer.declaration == nil || producer.declaration.Body == nil {
		return nil
	}
	var literals []*ast.FuncLit
	for _, node := range producer.declaration.Body.List {
		statement, ok := node.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		for _, result := range statement.Results {
			literal, ok := unwrapParentheses(result).(*ast.FuncLit)
			if ok && returnedFunctionValueInvoked(producer, functions, context) {
				literals = append(literals, literal)
			}
		}
	}
	return literals
}

func returnedFunctionValueInvoked(producer packageSourceFunction, functions []packageSourceFunction, context *registrationAnalysisContext) bool {
	for _, caller := range functions {
		if caller.declaration == nil || caller.declaration.Body == nil {
			continue
		}
		parents := context.parentsFor(caller.declaration)
		for _, call := range context.callsFor(caller.declaration) {
			if !exactPackageFunctionCall(call, producer.declaration) {
				continue
			}
			if returnedCallDirectlyInvoked(call, parents) || returnedOptionFlowsToConsumer(caller.declaration, call, producer.declaration, functions, parents, context) {
				return true
			}
		}
	}
	return false
}

func returnedCallDirectlyInvoked(call *ast.CallExpr, parents map[ast.Node]ast.Node) bool {
	current := ast.Node(call)
	for {
		parent := parents[current]
		if parenthesized, ok := parent.(*ast.ParenExpr); ok {
			current = parenthesized
			continue
		}
		invocation, ok := parent.(*ast.CallExpr)
		return ok && invocation.Fun == current
	}
}

func returnedOptionFlowsToConsumer(caller *ast.FuncDecl, producerCall *ast.CallExpr, producer *ast.FuncDecl, functions []packageSourceFunction, parents map[ast.Node]ast.Node, context *registrationAnalysisContext) bool {
	optionType := returnedNamedType(producer)
	collection := assignedCollectionObject(producerCall, parents)
	producerStatement := containingStatement(producerCall, parents)
	if optionType == "" || collection == nil || producerStatement == nil {
		return false
	}
	for _, call := range context.callsFor(caller) {
		if call.Pos() <= producerCall.Pos() || !callPassesCollectionVariadically(call, collection) || !statementDominatesCall(producerStatement, call, parents) {
			continue
		}
		for _, consumer := range functions {
			if exactPackageFunctionCall(call, consumer.declaration) && functionInvokesVariadicOption(consumer.declaration, optionType) && optionCollectionStableBetween(caller, collection, producerCall.Pos(), call.Pos(), parents) {
				return true
			}
		}
	}
	return false
}

func returnedNamedType(function *ast.FuncDecl) string {
	if function == nil || function.Type == nil || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return ""
	}
	identifier, ok := function.Type.Results.List[0].Type.(*ast.Ident)
	if !ok {
		return ""
	}
	return identifier.Name
}

func assignedCollectionObject(call *ast.CallExpr, parents map[ast.Node]ast.Node) *ast.Object {
	current := ast.Node(call)
	for current != nil {
		parent := parents[current]
		switch value := parent.(type) {
		case *ast.ParenExpr, *ast.CompositeLit, *ast.KeyValueExpr:
			current = parent
		case *ast.AssignStmt:
			for index, expression := range value.Rhs {
				if expression == current && index < len(value.Lhs) {
					return sourceExpressionObject(value.Lhs[index])
				}
			}
			return nil
		case *ast.ValueSpec:
			for index, expression := range value.Values {
				if expression == current && index < len(value.Names) {
					return value.Names[index].Obj
				}
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func containingStatement(node ast.Node, parents map[ast.Node]ast.Node) ast.Stmt {
	for current := node; current != nil; current = parents[current] {
		if statement, ok := current.(ast.Stmt); ok {
			return statement
		}
	}
	return nil
}

func callPassesCollectionVariadically(call *ast.CallExpr, collection *ast.Object) bool {
	if call == nil || collection == nil || !call.Ellipsis.IsValid() || len(call.Args) == 0 {
		return false
	}
	return sourceExpressionObject(call.Args[len(call.Args)-1]) == collection
}

func functionInvokesVariadicOption(function *ast.FuncDecl, optionType string) bool {
	parameter := variadicParameterObject(function, optionType)
	if parameter == nil {
		return false
	}
	invoked := false
	inspectReachable(function.Body, func(node ast.Node) {
		loop, ok := node.(*ast.RangeStmt)
		if !ok || sourceExpressionObject(loop.X) != parameter {
			return
		}
		option := sourceExpressionObject(loop.Value)
		if option == nil {
			return
		}
		inspectReachableCalls(loop.Body, func(call *ast.CallExpr) {
			if sourceExpressionObject(call.Fun) == option {
				invoked = true
			}
		})
	})
	return invoked
}

func variadicParameterObject(function *ast.FuncDecl, optionType string) *ast.Object {
	if function == nil || function.Type == nil || function.Type.Params == nil || optionType == "" {
		return nil
	}
	for _, field := range function.Type.Params.List {
		ellipsis, ok := field.Type.(*ast.Ellipsis)
		if !ok {
			continue
		}
		typeName, ok := ellipsis.Elt.(*ast.Ident)
		if ok && typeName.Name == optionType && len(field.Names) == 1 {
			return field.Names[0].Obj
		}
	}
	return nil
}

func optionCollectionStableBetween(function *ast.FuncDecl, collection *ast.Object, after, before token.Pos, parents map[ast.Node]ast.Node) bool {
	stable := true
	inspectReachableWithParents(function.Body, parents, func(node ast.Node) {
		if !stable || node == nil || node.Pos() <= after || node.Pos() >= before {
			return
		}
		if write, ok := identifierWrite(node, collection); ok {
			call, appendOK := unwrapParentheses(write).(*ast.CallExpr)
			stable = appendOK && calledName(call.Fun) == "append" && len(call.Args) > 0 && sourceExpressionObject(call.Args[0]) == collection
			return
		}
		call, ok := node.(*ast.CallExpr)
		if !ok || (calledName(call.Fun) == "append" && len(call.Args) > 0 && sourceExpressionObject(call.Args[0]) == collection) {
			return
		}
		for _, argument := range call.Args {
			if sourceExpressionObject(argument) == collection {
				stable = false
				return
			}
		}
	})
	return stable
}

func packageDeclaresString(files []*ast.File) bool {
	for _, file := range files {
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Name != nil && declaration.Name.Name == "string" {
					return true
				}
			case *ast.GenDecl:
				if declaration.Tok == token.IMPORT {
					continue
				}
				for _, specification := range declaration.Specs {
					switch specification := specification.(type) {
					case *ast.TypeSpec:
						if specification.Name != nil && specification.Name.Name == "string" {
							return true
						}
					case *ast.ValueSpec:
						for _, name := range specification.Names {
							if name != nil && name.Name == "string" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

func markPackageStringShadow(files []*ast.File) {
	shadow := &ast.Object{Kind: ast.Var, Name: "string"}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if ok && identifier.Name == "string" && identifier.Obj == nil {
				identifier.Obj = shadow
			}
			return true
		})
	}
}

func collectStringConstants(file *ast.File, destination map[string]string) {
	for _, declaration := range file.Decls {
		constants, ok := declaration.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, declarationSpec := range constants.Specs {
			valueSpec, ok := declarationSpec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, expression := range valueSpec.Values {
				if index >= len(valueSpec.Names) {
					continue
				}
				if value, ok := evaluateSourceString(expression, nil, destination, nil); ok {
					destination[valueSpec.Names[index].Name] = value
				}
			}
		}
	}
}

func functionCallsRegister(function *ast.FuncDecl) bool {
	found := false
	inspectReachableCalls(function.Body, func(call *ast.CallExpr) {
		if !found && isEffectRegistrarCall(function, call) {
			found = true
		}
	})
	return found
}

func descriptorLiteralFeedsRegister(function *ast.FuncDecl, descriptor *ast.CompositeLit) bool {
	return descriptorLiteralRegistrar(function, descriptor) != nil
}

func descriptorLiteralRegistrar(function *ast.FuncDecl, descriptor *ast.CompositeLit) *sourceRegistrarCall {
	var registrar *sourceRegistrarCall
	inspectReachableCalls(function.Body, func(call *ast.CallExpr) {
		if registrar != nil || !isEffectRegistrarCall(function, call) || len(call.Args) != 1 {
			return
		}
		if registrationHasUnsafeLoopControl(function, call.Pos()) {
			return
		}
		if !astNodeContains(call.Args[0], descriptor) {
			argument, ok := call.Args[0].(*ast.Ident)
			if !ok || (!identifierAssignedDescriptor(function, argument, descriptor, call.Pos()) && !registeredRangeContainsStableDescriptor(function, argument, descriptor, call.Pos())) {
				return
			}
		}
		registrar = &sourceRegistrarCall{path: expressionReference(call.Fun), object: sourceExpressionObject(call.Fun), position: call.Pos()}
	})
	return registrar
}

func isEffectRegistrarCall(function *ast.FuncDecl, call *ast.CallExpr) bool {
	if function == nil || call == nil || len(call.Args) != 1 {
		return false
	}
	path := expressionReference(call.Fun)
	if path != "effects.Register" && path != "options.Effects.Register" && path != "financial.Effects.Register" {
		return false
	}
	root := strings.SplitN(path, ".", 2)[0]
	expected := functionRootObjects(function)[root]
	return expected != nil && sourceExpressionObject(call.Fun) == expected
}

func identifierAssignedDescriptor(function *ast.FuncDecl, identifier *ast.Ident, descriptor *ast.CompositeLit, before token.Pos) bool {
	value, _, found := latestIdentifierValue(function, identifier, before)
	return found && value != nil && astNodeContains(value, descriptor)
}

func registeredRangeContainsStableDescriptor(function *ast.FuncDecl, registeredVariable *ast.Ident, descriptor *ast.CompositeLit, registerPosition token.Pos) bool {
	if registeredVariable == nil || registeredVariable.Obj == nil {
		return false
	}
	matched := false
	inspectReachable(function.Body, func(node ast.Node) {
		if matched {
			return
		}
		rangeStatement, ok := node.(*ast.RangeStmt)
		if !ok {
			return
		}
		rangeVariable, variableOK := rangeStatement.Value.(*ast.Ident)
		if !variableOK || rangeStatement.Pos() > registerPosition || rangeVariable.Obj == nil || rangeVariable.Obj != registeredVariable.Obj {
			return
		}
		source := assignedCompositeLiteral(function, rangeStatement.X, rangeStatement.Pos())
		if source != nil && astNodeContains(source, descriptor) {
			matched = rangeRegistrationObjectStable(function, rangeVariable, registerPosition)
			return
		}
		collection, collectionOK := rangeStatement.X.(*ast.Ident)
		if collectionOK && descriptorAppendedToCollection(function, collection, descriptor, rangeStatement.Pos()) {
			matched = rangeRegistrationObjectStable(function, rangeVariable, registerPosition)
		}
	})
	return matched
}

func rangeRegistrationObjectStable(function *ast.FuncDecl, variable *ast.Ident, registerPosition token.Pos) bool {
	if function == nil || variable == nil || variable.Obj == nil {
		return false
	}
	return !hasObjectWriteBetween(function, variable.Obj, variable.Pos(), registerPosition) &&
		!hasObjectEscapeBetween(function, variable.Obj, variable.Pos(), registerPosition)
}

func descriptorAppendedToCollection(function *ast.FuncDecl, collection *ast.Ident, descriptor *ast.CompositeLit, before token.Pos) bool {
	if function == nil || collection == nil || collection.Obj == nil {
		return false
	}
	state := descriptorMembershipAfterBlock(function, function.Body, collection.Obj, descriptor, before, descriptorMembershipAbsent)
	return state == descriptorMembershipPresent
}

type descriptorMembership uint8

const (
	descriptorMembershipAbsent descriptorMembership = iota
	descriptorMembershipPresent
	descriptorMembershipAmbiguous
)

func descriptorMembershipAfterBlock(function *ast.FuncDecl, block *ast.BlockStmt, object *ast.Object, descriptor *ast.CompositeLit, before token.Pos, state descriptorMembership) descriptorMembership {
	if block == nil || object == nil {
		return state
	}
	for _, statement := range block.List {
		if statement == nil || statement.Pos() >= before {
			break
		}
		state = descriptorMembershipAfterStatement(function, statement, object, descriptor, before, state)
	}
	return state
}

func descriptorMembershipAfterStatement(function *ast.FuncDecl, statement ast.Stmt, object *ast.Object, descriptor *ast.CompositeLit, before token.Pos, state descriptorMembership) descriptorMembership {
	if statement == nil || statement.Pos() >= before {
		return state
	}
	switch value := statement.(type) {
	case *ast.AssignStmt:
		if write, ok := identifierWrite(value, object); ok {
			return descriptorMembershipAfterWrite(write, object, descriptor, state)
		}
	case *ast.DeclStmt:
		declaration, ok := value.Decl.(*ast.GenDecl)
		if !ok {
			return state
		}
		for _, spec := range declaration.Specs {
			if write, ok := identifierWrite(spec, object); ok {
				state = descriptorMembershipAfterWrite(write, object, descriptor, state)
			}
		}
	case *ast.BlockStmt:
		return descriptorMembershipAfterBlock(function, value, object, descriptor, before, state)
	case *ast.IfStmt:
		if value.Init != nil {
			state = descriptorMembershipAfterStatement(function, value.Init, object, descriptor, before, state)
		}
		if isStaticFalse(value.Cond) {
			return descriptorMembershipAfterElse(function, value.Else, object, descriptor, before, state)
		}
		if isStaticTrue(value.Cond) {
			return descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		}
		if astNodeContainsPosition(value.Body, before) {
			return descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		}
		if astNodeContainsPosition(value.Else, before) {
			return descriptorMembershipAfterElse(function, value.Else, object, descriptor, before, state)
		}
		thenState := descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		elseState := descriptorMembershipAfterElse(function, value.Else, object, descriptor, before, state)
		return mergeDescriptorMembership(thenState, elseState)
	case *ast.RangeStmt:
		if astNodeContainsPosition(value.Body, before) {
			if hasUnsafeLoopControlBefore(function, value, descriptor, value.Body, before) {
				return descriptorMembershipAmbiguous
			}
			return descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		}
		if hasUnsafeLoopControlBefore(function, value, descriptor, value.Body, descriptor.Pos()) {
			return descriptorMembershipAmbiguous
		}
		if iterations, known := knownRangeIterationCount(function, value); known {
			for range iterations {
				state = descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
			}
			return state
		}
		bodyState := descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		if hasIdentifierWriteBefore(value.Body, object, before) {
			return descriptorMembershipAmbiguous
		}
		return mergeDescriptorMembership(state, bodyState)
	case *ast.ForStmt:
		if value.Init != nil {
			state = descriptorMembershipAfterStatement(function, value.Init, object, descriptor, before, state)
		}
		if isStaticFalse(value.Cond) {
			return state
		}
		if astNodeContainsPosition(value.Body, before) {
			if hasUnsafeLoopControlBefore(function, nil, descriptor, value.Body, before) {
				return descriptorMembershipAmbiguous
			}
			return descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		}
		if hasUnsafeLoopControlBefore(function, nil, descriptor, value.Body, descriptor.Pos()) {
			return descriptorMembershipAmbiguous
		}
		iterationState := descriptorMembershipAfterBlock(function, value.Body, object, descriptor, before, state)
		if value.Post != nil {
			iterationState = descriptorMembershipAfterStatement(function, value.Post, object, descriptor, before, iterationState)
		}
		if hasIdentifierWriteBefore(value.Body, object, before) || hasIdentifierWriteBefore(value.Post, object, before) {
			return descriptorMembershipAmbiguous
		}
		return mergeDescriptorMembership(state, iterationState)
	case *ast.LabeledStmt:
		return descriptorMembershipAfterStatement(function, value.Stmt, object, descriptor, before, state)
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		if hasIdentifierWriteBefore(statement, object, before) {
			return descriptorMembershipAmbiguous
		}
	case *ast.GoStmt, *ast.DeferStmt:
		// Async/deferred mutation cannot prove collection membership.
	}
	return state
}

func descriptorMembershipAfterElse(function *ast.FuncDecl, statement ast.Stmt, object *ast.Object, descriptor *ast.CompositeLit, before token.Pos, state descriptorMembership) descriptorMembership {
	switch value := statement.(type) {
	case nil:
		return state
	case *ast.BlockStmt:
		return descriptorMembershipAfterBlock(function, value, object, descriptor, before, state)
	default:
		return descriptorMembershipAfterStatement(function, value, object, descriptor, before, state)
	}
}

func knownRangeIterationCount(function *ast.FuncDecl, statement *ast.RangeStmt) (int, bool) {
	if function == nil || statement == nil {
		return 0, false
	}
	source := assignedCompositeLiteral(function, statement.X, statement.Pos())
	if source == nil {
		return 0, false
	}
	return len(source.Elts), true
}

func registrationHasUnsafeLoopControl(function *ast.FuncDecl, before token.Pos) bool {
	if function == nil || function.Body == nil {
		return false
	}
	unsafe := false
	inspectReachable(function.Body, func(node ast.Node) {
		if unsafe {
			return
		}
		var body *ast.BlockStmt
		var rangeStatement *ast.RangeStmt
		switch loop := node.(type) {
		case *ast.RangeStmt:
			body = loop.Body
			rangeStatement = loop
		case *ast.ForStmt:
			body = loop.Body
		}
		if body != nil && astNodeContainsPosition(body, before) && hasUnsafeLoopControlBefore(function, rangeStatement, nil, body, before) {
			unsafe = true
		}
	})
	return unsafe
}

func hasUnsafeLoopControlBefore(function *ast.FuncDecl, loop *ast.RangeStmt, descriptor *ast.CompositeLit, body *ast.BlockStmt, before token.Pos) bool {
	if body == nil {
		return false
	}
	parents := astParentMap(body)
	safeContinue := make(map[*ast.BranchStmt]struct{})
	inspectReachable(body, func(node ast.Node) {
		statement, ok := node.(*ast.IfStmt)
		if !ok || statement.Pos() >= before || !isOptionalDescriptorSkipCondition(function, loop, descriptor, statement.Cond) {
			return
		}
		branch, ok := directCurrentLoopContinue(body, statement, parents)
		if ok && branch.Pos() < before {
			safeContinue[branch] = struct{}{}
		}
	})
	unsafe := false
	inspectReachable(body, func(node ast.Node) {
		if unsafe || node == nil || node.Pos() >= before {
			return
		}
		switch statement := node.(type) {
		case *ast.BranchStmt:
			switch statement.Tok {
			case token.BREAK, token.CONTINUE, token.GOTO, token.FALLTHROUGH:
				_, allowed := safeContinue[statement]
				unsafe = !allowed
			}
		case *ast.ReturnStmt:
			unsafe = !isFailureReturn(statement, parents)
		}
	})
	return unsafe
}

func directCurrentLoopContinue(loopBody *ast.BlockStmt, condition *ast.IfStmt, parents map[ast.Node]ast.Node) (*ast.BranchStmt, bool) {
	if loopBody == nil || condition == nil || condition.Body == nil || len(condition.Body.List) != 1 {
		return nil, false
	}
	branch, ok := condition.Body.List[0].(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || branch.Label != nil {
		return nil, false
	}
	for node := parents[branch]; node != nil && node != loopBody; node = parents[node] {
		switch node.(type) {
		case *ast.RangeStmt, *ast.ForStmt, *ast.FuncLit:
			return nil, false
		}
	}
	for node := ast.Node(branch); node != nil; node = parents[node] {
		if node == loopBody {
			return branch, true
		}
	}
	return nil, false
}

func isOptionalDescriptorSkipCondition(function *ast.FuncDecl, loop *ast.RangeStmt, descriptor *ast.CompositeLit, expression ast.Expr) bool {
	if function == nil || loop == nil || descriptor == nil || loop.Value == nil {
		return false
	}
	effectObject := sourceExpressionObject(loop.Value)
	if effectObject == nil {
		return false
	}
	condition, ok := unwrapParentheses(expression).(*ast.BinaryExpr)
	if !ok || condition.Op != token.LAND {
		return false
	}
	return (isSandboxEffectComparison(condition.X, effectObject) && isCapabilityUnknownCheck(condition.Y, function, loop, effectObject, descriptor)) ||
		(isSandboxEffectComparison(condition.Y, effectObject) && isCapabilityUnknownCheck(condition.X, function, loop, effectObject, descriptor))
}

func isSandboxEffectComparison(expression ast.Expr, effectObject *ast.Object) bool {
	comparison, ok := unwrapParentheses(expression).(*ast.BinaryExpr)
	if !ok || comparison.Op != token.EQL || effectObject == nil {
		return false
	}
	left := unwrapParentheses(comparison.X)
	right := unwrapParentheses(comparison.Y)
	return (expressionReference(left) == "effect.id" && sourceExpressionObject(left) == effectObject && expressionReference(right) == "releasecontract.EffectAgentToolPythonSandbox") ||
		(expressionReference(right) == "effect.id" && sourceExpressionObject(right) == effectObject && expressionReference(left) == "releasecontract.EffectAgentToolPythonSandbox")
}

func isCapabilityUnknownCheck(expression ast.Expr, function *ast.FuncDecl, loop *ast.RangeStmt, effectObject *ast.Object, descriptor *ast.CompositeLit) bool {
	call, ok := unwrapParentheses(expression).(*ast.CallExpr)
	if !ok || expressionReference(call.Fun) != "releasecontract.IsReadinessCode" || len(call.Args) != 2 ||
		expressionReference(unwrapParentheses(call.Args[1])) != "releasecontract.CodeCapabilityUnknown" {
		return false
	}
	errorObject := sourceExpressionObject(unwrapParentheses(call.Args[0]))
	capabilityObject := sourceExpressionObject(descriptorField(descriptor, "CapabilityID"))
	if errorObject == nil || capabilityObject == nil || effectObject == nil {
		return false
	}
	binding, found := latestCapabilityBindingForError(resolvedCapabilityBindings(function), errorObject, call.Pos())
	if !found || binding.effectReference != "effect.id" || binding.effectObject != effectObject || binding.object != capabilityObject {
		return false
	}
	return !hasIdentifierWriteBetween(function, errorObject, binding.position, call.Pos()) &&
		!hasIdentifierWriteBetween(function, capabilityObject, binding.position, descriptor.Pos()) &&
		!hasObjectWriteBetween(function, effectObject, binding.position, descriptor.End()) &&
		!hasObjectEscapeBetween(function, effectObject, objectAddressEscapeStart(loop, effectObject), descriptor.End())
}

func objectAddressEscapeStart(loop *ast.RangeStmt, object *ast.Object) token.Pos {
	var start token.Pos
	if loop != nil && loop.Body != nil {
		start = loop.Body.Pos()
	}
	if object != nil {
		position := object.Pos()
		if position > 0 && (start == 0 || position < start) {
			start = position
		}
	}
	return start
}

func latestCapabilityBindingForError(bindings []sourceCapabilityBinding, errorObject *ast.Object, before token.Pos) (sourceCapabilityBinding, bool) {
	var latest sourceCapabilityBinding
	found := false
	for _, binding := range bindings {
		if binding.errorObject != errorObject || binding.position >= before || (found && binding.position <= latest.position) {
			continue
		}
		latest = binding
		found = true
	}
	return latest, found
}

func hasIdentifierWriteBetween(function *ast.FuncDecl, object *ast.Object, after, before token.Pos) bool {
	if function == nil || function.Body == nil || object == nil || before <= after {
		return true
	}
	found := false
	inspectReachable(function.Body, func(node ast.Node) {
		if found || node == nil || node.Pos() <= after || node.Pos() >= before {
			return
		}
		if _, ok := identifierWrite(node, object); ok {
			found = true
		}
	})
	return found
}

func hasObjectWriteBetween(function *ast.FuncDecl, object *ast.Object, after, before token.Pos) bool {
	if function == nil || function.Body == nil || object == nil || before <= after {
		return true
	}
	found := false
	inspectReachable(function.Body, func(node ast.Node) {
		if found || node == nil || node.Pos() <= after || node.Pos() >= before {
			return
		}
		found = astNodeWritesObject(node, object)
	})
	return found
}

func astNodeWritesObject(node ast.Node, object *ast.Object) bool {
	if node == nil || object == nil {
		return false
	}
	switch statement := node.(type) {
	case *ast.AssignStmt:
		for _, expression := range statement.Lhs {
			if writtenExpressionObject(expression) == object {
				return true
			}
		}
	case *ast.IncDecStmt:
		return writtenExpressionObject(statement.X) == object
	case *ast.RangeStmt:
		return statement.Tok == token.ASSIGN && (writtenExpressionObject(statement.Key) == object || writtenExpressionObject(statement.Value) == object)
	}
	return false
}

func hasObjectEscapeBetween(function *ast.FuncDecl, object *ast.Object, after, before token.Pos) bool {
	if function == nil || function.Body == nil || object == nil || before <= after {
		return true
	}
	parents := astParentMap(function.Body)
	found := false
	inspectReachable(function.Body, func(node ast.Node) {
		if found || node == nil || node.Pos() <= after || node.Pos() >= before {
			return
		}
		switch expression := node.(type) {
		case *ast.FuncLit:
			found = functionLiteralCapturesObject(expression, object)
		case *ast.UnaryExpr:
			found = expression.Op == token.AND && sourceExpressionObject(unwrapParentheses(expression.X)) == object
		case *ast.CallExpr:
			method, ok := unwrapParentheses(expression.Fun).(*ast.SelectorExpr)
			found = ok && sourceExpressionObject(method.X) == object
		case *ast.SelectorExpr:
			found = sourceExpressionObject(expression) == object && !isSafeObjectSelectorRead(function, object, expression, parents)
		}
	})
	return found
}

func functionLiteralCapturesObject(literal *ast.FuncLit, object *ast.Object) bool {
	if literal == nil || literal.Body == nil || object == nil {
		return false
	}
	captured := false
	ast.Inspect(literal.Body, func(node ast.Node) bool {
		if captured {
			return false
		}
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Obj != nil && identifier.Obj == object {
			captured = true
			return false
		}
		return true
	})
	return captured
}

func isSafeObjectSelectorRead(function *ast.FuncDecl, object *ast.Object, selector *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	if function == nil || object == nil || selector == nil {
		return false
	}
	reference := expressionReference(selector)
	current := ast.Node(selector)
	for {
		parent := parents[current]
		if parenthesized, ok := parent.(*ast.ParenExpr); ok {
			current = parenthesized
			continue
		}
		if isSafeDescriptorSelectorRead(reference, selector, current, parents) {
			return true
		}
		switch context := parent.(type) {
		case *ast.CallExpr:
			return reference == "effect.id" && len(context.Args) == 1 && unwrapParentheses(context.Args[0]) == selector && isCapabilityResolverCall(function, context)
		case *ast.BinaryExpr:
			return reference == "effect.id" && isSandboxEffectComparison(context, object)
		case *ast.IndexExpr:
			return reference == "effect.key" && context.Index == current
		default:
			return false
		}
	}
}

func isSafeDescriptorSelectorRead(reference string, selector *ast.SelectorExpr, current ast.Node, parents map[ast.Node]ast.Node) bool {
	if selector == nil || current == nil {
		return false
	}
	if conversion, ok := parents[current].(*ast.CallExpr); ok {
		if reference != "effect.id" || len(conversion.Args) != 1 || unwrapParentheses(conversion.Args[0]) != selector || !isSafeSourceStringConversion(conversion.Fun) {
			return false
		}
		current = conversion
	}
	field, ok := parents[current].(*ast.KeyValueExpr)
	if !ok {
		return false
	}
	name, ok := field.Key.(*ast.Ident)
	if !ok {
		return false
	}
	descriptor, ok := parents[field].(*ast.CompositeLit)
	if !ok || !isDescriptorLiteral(descriptor) {
		return false
	}
	return (name.Name == "ID" && reference == "effect.id") || (name.Name == "Owner" && reference == "effect.owner")
}

func writtenExpressionObject(expression ast.Expr) *ast.Object {
	switch value := expression.(type) {
	case *ast.IndexExpr:
		return writtenExpressionObject(value.X)
	case *ast.IndexListExpr:
		return writtenExpressionObject(value.X)
	case *ast.StarExpr:
		return writtenExpressionObject(value.X)
	case *ast.ParenExpr:
		return writtenExpressionObject(value.X)
	default:
		return sourceExpressionObject(expression)
	}
}

func unwrapParentheses(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func astParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	stack := make([]ast.Node, 0, 16)
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func isFailureReturn(statement *ast.ReturnStmt, parents map[ast.Node]ast.Node) bool {
	if statement == nil || len(statement.Results) < 2 {
		return false
	}
	identifier, ok := statement.Results[len(statement.Results)-1].(*ast.Ident)
	if !ok || identifier.Obj == nil {
		return false
	}
	for node := ast.Node(statement); node != nil; node = parents[node] {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			continue
		}
		condition, ok := parents[block].(*ast.IfStmt)
		if ok && condition.Body == block && expressionRequiresNonNilObject(condition.Cond, identifier.Obj) {
			return true
		}
	}
	return false
}

func expressionRequiresNonNilObject(expression ast.Expr, object *ast.Object) bool {
	comparison, ok := unwrapParentheses(expression).(*ast.BinaryExpr)
	if !ok || comparison.Op != token.NEQ || object == nil {
		return false
	}
	return (expressionObjectIs(comparison.X, object) && isNilExpression(comparison.Y)) ||
		(expressionObjectIs(comparison.Y, object) && isNilExpression(comparison.X))
}

func expressionObjectIs(expression ast.Expr, object *ast.Object) bool {
	identifier, ok := unwrapParentheses(expression).(*ast.Ident)
	return ok && identifier.Obj != nil && identifier.Obj == object
}

func isNilExpression(expression ast.Expr) bool {
	identifier, ok := unwrapParentheses(expression).(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func descriptorMembershipAfterWrite(write ast.Expr, object *ast.Object, descriptor *ast.CompositeLit, state descriptorMembership) descriptorMembership {
	call, isAppend := write.(*ast.CallExpr)
	if !isAppend || calledName(call.Fun) != "append" || len(call.Args) < 2 {
		if write != nil && astNodeContains(write, descriptor) {
			return descriptorMembershipPresent
		}
		return descriptorMembershipAbsent
	}
	for _, argument := range call.Args[1:] {
		if astNodeContains(argument, descriptor) {
			return descriptorMembershipPresent
		}
	}
	if sourceExpressionObject(call.Args[0]) == object {
		return state
	}
	if astNodeContains(call.Args[0], descriptor) {
		return descriptorMembershipPresent
	}
	return descriptorMembershipAbsent
}

func mergeDescriptorMembership(left, right descriptorMembership) descriptorMembership {
	if left == right {
		return left
	}
	return descriptorMembershipAmbiguous
}

func astNodeContainsPosition(root ast.Node, position token.Pos) bool {
	return root != nil && position >= root.Pos() && position <= root.End()
}

func descriptorParameterEnvironments(function packageSourceFunction, descriptor *ast.CompositeLit, functions []packageSourceFunction, fset *token.FileSet, constants, effectValues map[string]string) []sourceRegistrationEnvironment {
	parameters := functionParameterNames(function.declaration)
	if len(parameters) == 0 || !descriptorUsesFunctionParameter(descriptor, parameters) {
		return nil
	}
	var environments []sourceRegistrationEnvironment
	for _, caller := range functions {
		inspectReachableCalls(caller.declaration.Body, func(call *ast.CallExpr) {
			if !exactPackageFunctionCall(call, function.declaration) {
				return
			}
			environment := make(map[string]string)
			for index, parameter := range parameters {
				if index >= len(call.Args) {
					break
				}
				if value, ok := evaluateSourceString(call.Args[index], nil, constants, effectValues); ok {
					environment[parameter] = value
				}
			}
			if len(environment) > 0 {
				position := fset.Position(call.Pos())
				environments = append(environments, sourceRegistrationEnvironment{
					values:             environment,
					registrationSymbol: packageFunctionSymbol(caller.declaration) + ">" + packageFunctionSymbol(function.declaration),
					file:               caller.file,
					line:               position.Line,
				})
			}
		})
	}
	return environments
}

func exactPackageFunctionCall(call *ast.CallExpr, callee *ast.FuncDecl) bool {
	if call == nil || callee == nil || callee.Name == nil || expressionReference(call.Fun) != callee.Name.Name {
		return false
	}
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	// parser.ParseFile gives each file its own package scope. When both
	// top-level objects are unavailable (the cross-file case), the exact
	// unqualified package path is the remaining stable edge; any available
	// object identity is still required to match.
	if identifier.Obj != nil {
		return callee.Name.Obj != nil && identifier.Obj == callee.Name.Obj
	}
	return callee.Recv == nil
}

func descriptorUsesFunctionParameter(descriptor *ast.CompositeLit, parameters []string) bool {
	parameterSet := make(map[string]struct{}, len(parameters))
	for _, parameter := range parameters {
		parameterSet[parameter] = struct{}{}
	}
	for _, field := range []string{"ID", "Owner"} {
		if _, ok := parameterSet[expressionReference(descriptorField(descriptor, field))]; ok {
			return true
		}
	}
	return false
}

func descriptorRangeEnvironments(function *ast.FuncDecl, descriptor *ast.CompositeLit, constants, effectValues map[string]string) []map[string]string {
	var environments []map[string]string
	inspectReachable(function.Body, func(node ast.Node) {
		rangeStatement, ok := node.(*ast.RangeStmt)
		if !ok || !astNodeContains(rangeStatement.Body, descriptor) {
			return
		}
		source := assignedCompositeLiteral(function, rangeStatement.X, rangeStatement.Pos())
		if source == nil {
			return
		}
		environments = append(environments, rangeCompositeEnvironments(rangeStatement, source, constants, effectValues)...)
	})
	return environments
}

func astNodeContains(root ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == target {
			found = true
			return false
		}
		return !found
	})
	return found
}

func assignedCompositeLiteral(function *ast.FuncDecl, expression ast.Expr, before token.Pos) *ast.CompositeLit {
	if literal, ok := expression.(*ast.CompositeLit); ok {
		return literal
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return nil
	}
	value, _, found := latestIdentifierValue(function, identifier, before)
	if !found {
		return nil
	}
	result, _ := value.(*ast.CompositeLit)
	return result
}

func latestIdentifierValue(function *ast.FuncDecl, identifier *ast.Ident, before token.Pos) (ast.Expr, token.Pos, bool) {
	if function == nil {
		return nil, 0, false
	}
	return latestIdentifierValueInBlock(function.Body, identifier, before)
}

func latestIdentifierValueInBlock(block *ast.BlockStmt, identifier *ast.Ident, before token.Pos) (ast.Expr, token.Pos, bool) {
	if block == nil || identifier == nil || identifier.Obj == nil {
		return nil, 0, false
	}
	state := identifierStateAfterBlock(block, identifier.Obj, before, sourceIdentifierState{})
	if state.ambiguous {
		return nil, 0, false
	}
	return state.value, state.position, state.found
}

func identifierWrite(node ast.Node, object *ast.Object) (ast.Expr, bool) {
	if object == nil {
		return nil, false
	}
	switch value := node.(type) {
	case *ast.AssignStmt:
		for index, left := range value.Lhs {
			name, ok := left.(*ast.Ident)
			if !ok || name.Obj == nil || name.Obj != object {
				continue
			}
			if index < len(value.Rhs) {
				return value.Rhs[index], true
			}
			return nil, true
		}
	case *ast.ValueSpec:
		for index, name := range value.Names {
			if name.Obj == nil || name.Obj != object {
				continue
			}
			if index < len(value.Values) {
				return value.Values[index], true
			}
			return nil, true
		}
	}
	return nil, false
}

type sourceIdentifierState struct {
	value     ast.Expr
	position  token.Pos
	found     bool
	ambiguous bool
}

func identifierStateAfterBlock(block *ast.BlockStmt, object *ast.Object, before token.Pos, state sourceIdentifierState) sourceIdentifierState {
	if block == nil || object == nil {
		return state
	}
	for _, statement := range block.List {
		if statement == nil || statement.Pos() >= before {
			break
		}
		state = identifierStateAfterStatement(statement, object, before, state)
	}
	return state
}

func identifierStateAfterStatement(statement ast.Stmt, object *ast.Object, before token.Pos, state sourceIdentifierState) sourceIdentifierState {
	if statement == nil || statement.Pos() >= before {
		return state
	}
	switch value := statement.(type) {
	case *ast.AssignStmt:
		if write, ok := identifierWrite(value, object); ok {
			return sourceIdentifierState{value: write, position: value.Pos(), found: true}
		}
	case *ast.DeclStmt:
		declaration, ok := value.Decl.(*ast.GenDecl)
		if !ok {
			return state
		}
		for _, spec := range declaration.Specs {
			if write, ok := identifierWrite(spec, object); ok {
				state = sourceIdentifierState{value: write, position: spec.Pos(), found: true}
			}
		}
	case *ast.BlockStmt:
		return identifierStateAfterBlock(value, object, before, state)
	case *ast.IfStmt:
		if value.Init != nil {
			state = identifierStateAfterStatement(value.Init, object, before, state)
		}
		if isStaticFalse(value.Cond) {
			return identifierStateAfterElse(value.Else, object, before, state)
		}
		if isStaticTrue(value.Cond) {
			return identifierStateAfterBlock(value.Body, object, before, state)
		}
		if astNodeContainsPosition(value.Body, before) {
			return identifierStateAfterBlock(value.Body, object, before, state)
		}
		if astNodeContainsPosition(value.Else, before) {
			return identifierStateAfterElse(value.Else, object, before, state)
		}
		thenState := identifierStateAfterBlock(value.Body, object, before, state)
		elseState := identifierStateAfterElse(value.Else, object, before, state)
		return mergeSourceIdentifierStates(thenState, elseState)
	case *ast.ForStmt:
		if value.Init != nil {
			state = identifierStateAfterStatement(value.Init, object, before, state)
		}
		if isStaticFalse(value.Cond) {
			return state
		}
		if astNodeContainsPosition(value.Body, before) {
			if hasUnsafeLoopControlBefore(nil, nil, nil, value.Body, before) {
				state.ambiguous = true
				return state
			}
			return identifierStateAfterBlock(value.Body, object, before, state)
		}
		iterationState := identifierStateAfterBlock(value.Body, object, before, state)
		if value.Post != nil {
			iterationState = identifierStateAfterStatement(value.Post, object, before, iterationState)
		}
		return mergeSourceIdentifierStates(state, iterationState)
	case *ast.RangeStmt:
		if astNodeContainsPosition(value.Body, before) {
			if hasUnsafeLoopControlBefore(nil, nil, nil, value.Body, before) {
				state.ambiguous = true
				return state
			}
			return identifierStateAfterBlock(value.Body, object, before, state)
		}
		bodyState := identifierStateAfterBlock(value.Body, object, before, state)
		return mergeSourceIdentifierStates(state, bodyState)
	case *ast.LabeledStmt:
		return identifierStateAfterStatement(value.Stmt, object, before, state)
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		if hasIdentifierWriteBefore(statement, object, before) {
			state.ambiguous = true
		}
	case *ast.GoStmt, *ast.DeferStmt:
		// Async/deferred writes cannot establish synchronous provenance.
	}
	return state
}

func identifierStateAfterElse(statement ast.Stmt, object *ast.Object, before token.Pos, state sourceIdentifierState) sourceIdentifierState {
	switch value := statement.(type) {
	case nil:
		return state
	case *ast.BlockStmt:
		return identifierStateAfterBlock(value, object, before, state)
	default:
		return identifierStateAfterStatement(value, object, before, state)
	}
}

func mergeSourceIdentifierStates(left, right sourceIdentifierState) sourceIdentifierState {
	if left.ambiguous || right.ambiguous || left.found != right.found || (left.found && left.value != right.value) {
		position := left.position
		if right.position > position {
			position = right.position
		}
		return sourceIdentifierState{position: position, ambiguous: true}
	}
	if right.position > left.position {
		return right
	}
	return left
}

func hasIdentifierWriteBefore(root ast.Node, object *ast.Object, before token.Pos) bool {
	found := false
	inspectReachable(root, func(node ast.Node) {
		if found || node == nil || node.Pos() >= before {
			return
		}
		if _, ok := identifierWrite(node, object); ok {
			found = true
		}
	})
	return found
}

func rangeCompositeEnvironments(statement *ast.RangeStmt, source *ast.CompositeLit, constants, effectValues map[string]string) []map[string]string {
	switch source.Type.(type) {
	case *ast.MapType:
		keyName := expressionReference(statement.Key)
		valueName := expressionReference(statement.Value)
		var environments []map[string]string
		for _, element := range source.Elts {
			row, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			environment := make(map[string]string)
			if value, ok := evaluateSourceString(row.Key, nil, constants, effectValues); ok && keyName != "" {
				environment[keyName] = value
			}
			if value, ok := evaluateSourceString(row.Value, nil, constants, effectValues); ok && valueName != "" {
				environment[valueName] = value
			}
			if len(environment) > 0 {
				environments = append(environments, environment)
			}
		}
		return environments
	case *ast.ArrayType:
		valueName := expressionReference(statement.Value)
		fieldNames := sourceStructFieldNames(source)
		if valueName == "" || len(fieldNames) == 0 {
			return nil
		}
		var environments []map[string]string
		for _, element := range source.Elts {
			row, ok := element.(*ast.CompositeLit)
			if !ok {
				continue
			}
			environment := make(map[string]string)
			for index, fieldName := range fieldNames {
				if index >= len(row.Elts) {
					break
				}
				if value, ok := evaluateSourceString(row.Elts[index], nil, constants, effectValues); ok {
					environment[valueName+"."+fieldName] = value
				}
			}
			if len(environment) > 0 {
				environments = append(environments, environment)
			}
		}
		return environments
	}
	return nil
}

func sourceStructFieldNames(source *ast.CompositeLit) []string {
	array, ok := source.Type.(*ast.ArrayType)
	if !ok {
		return nil
	}
	structure, ok := array.Elt.(*ast.StructType)
	if !ok || structure.Fields == nil {
		return nil
	}
	var names []string
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

func sourceRegistrationFromLiteral(repoRoot string, literal *ast.CompositeLit, ownerPackage string, environment sourceRegistrationEnvironment, constants, effectValues map[string]string, resolved []sourceCapabilityBinding, registrar *sourceRegistrarCall) (sourceRegistration, bool) {
	descriptorID, idOK := evaluateSourceString(descriptorField(literal, "ID"), environment.values, constants, effectValues)
	owner, ownerOK := evaluateSourceString(descriptorField(literal, "Owner"), environment.values, constants, effectValues)
	boundary, boundaryOK := sourceBoundary(descriptorField(literal, "Boundary"))
	if !idOK || !ownerOK || !boundaryOK || strings.TrimSpace(descriptorID) == "" || strings.TrimSpace(owner) == "" {
		return sourceRegistration{}, false
	}
	capabilityID, _ := evaluateSourceString(descriptorField(literal, "CapabilityID"), environment.values, constants, effectValues)
	binding := resolvedCapabilityBindingForExpression(descriptorField(literal, "CapabilityID"), literal.Pos(), resolved)
	effectID := EffectID(sourceReferenceValue(binding.effectReference, environment.values, constants, effectValues))
	if capabilityID == "" {
		capabilityID = string(authoredEffectCapabilities[effectID])
	}
	if registrar == nil || (binding.resolverPath == "" && capabilityID == "") || (binding.resolverPath != "" && !validAuthorityRegistrationPair(binding, registrar)) {
		return sourceRegistration{}, false
	}
	relative, _ := filepath.Rel(repoRoot, environment.file)
	return sourceRegistration{
		DescriptorID: descriptorID, EffectID: effectID, OwnerPackage: ownerPackage, Owner: owner, CapabilityID: capabilityID, Boundary: boundary,
		RegistrationSymbol: environment.registrationSymbol, ResolverCall: binding.resolverPath, RegistrarCall: registrar.path,
		File: filepath.ToSlash(relative), Line: environment.line,
	}, true
}

func packageFunctionSymbol(function *ast.FuncDecl) string {
	return sourceFunctionKey(functionReceiverName(function), function.Name.Name)
}

func functionReceiverName(function *ast.FuncDecl) string {
	receiver, _ := functionReceiver(function)
	return receiver
}

func evaluateSourceString(expression ast.Expr, environment, constants, effectValues map[string]string) (string, bool) {
	if expression == nil {
		return "", false
	}
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		literal, err := strconv.Unquote(value.Value)
		return literal, err == nil
	case *ast.Ident:
		return sourceLookup(value.Name, environment, constants, effectValues)
	case *ast.SelectorExpr:
		return sourceLookup(expressionReference(value), environment, constants, effectValues)
	case *ast.CallExpr:
		if len(value.Args) == 1 && isSafeSourceStringConversion(value.Fun) {
			return evaluateSourceString(value.Args[0], environment, constants, effectValues)
		}
	case *ast.BinaryExpr:
		if value.Op == token.ADD {
			left, leftOK := evaluateSourceString(value.X, environment, constants, effectValues)
			right, rightOK := evaluateSourceString(value.Y, environment, constants, effectValues)
			return left + right, leftOK && rightOK
		}
	case *ast.ParenExpr:
		return evaluateSourceString(value.X, environment, constants, effectValues)
	}
	return "", false
}

func isSafeSourceStringConversion(expression ast.Expr) bool {
	identifier, ok := unwrapParentheses(expression).(*ast.Ident)
	return ok && identifier.Name == "string" && identifier.Obj == nil
}

func sourceLookup(reference string, environment, constants, effectValues map[string]string) (string, bool) {
	if value, ok := environment[reference]; ok {
		return value, true
	}
	short := reference
	if index := strings.LastIndex(short, "."); index >= 0 {
		short = short[index+1:]
	}
	if value, ok := constants[short]; ok {
		return value, true
	}
	value, ok := effectValues[short]
	return value, ok
}

func sourceReferenceValue(reference string, environment, constants, effectValues map[string]string) string {
	if value, ok := sourceLookup(reference, environment, constants, effectValues); ok {
		return value
	}
	return ""
}

func sourceBoundary(expression ast.Expr) (Boundary, bool) {
	name := calledName(expression)
	switch name {
	case "BoundaryHTTP":
		return BoundaryHTTP, true
	case "BoundaryGRPC":
		return BoundaryGRPC, true
	case "BoundaryWorkerClaim":
		return BoundaryWorkerClaim, true
	case "BoundaryWorkerEffect":
		return BoundaryWorkerEffect, true
	case "BoundaryOutbound":
		return BoundaryOutbound, true
	case "BoundaryFinancial":
		return BoundaryFinancial, true
	case "BoundaryOperation":
		return BoundaryOperation, true
	default:
		return "", false
	}
}

func descriptorStructurePresent(repoRoot, descriptorID string, spec runtimeDescriptorSpec) (bool, error) {
	effectValues, err := effectConstantValues(repoRoot)
	if err != nil {
		return false, err
	}
	registrations, err := discoverPackageRegistrations(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(spec.OwnerPackage)), effectValues)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	for _, registration := range registrations {
		if registration.DescriptorID != descriptorID {
			continue
		}
		present, err := descriptorRegistrationStructurePresent(repoRoot, registration, spec)
		if err != nil || present {
			return present, err
		}
	}
	return false, nil
}

func descriptorRegistrationStructurePresent(repoRoot string, registration sourceRegistration, spec runtimeDescriptorSpec) (bool, error) {
	if !sourceRegistrationMatchesSpec(registration, spec) {
		return false, nil
	}
	functions, err := loadSourceFunctions(filepath.Join(repoRoot, filepath.FromSlash(spec.OwnerPackage)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return guardEffectContractPresent(functions, spec.GuardCall, spec.EffectCall), nil
}

func sourceRegistrationMatchesSpec(registration sourceRegistration, spec runtimeDescriptorSpec) bool {
	return registration.DescriptorID != "" &&
		registration.EffectID == spec.EffectID &&
		registration.OwnerPackage == spec.OwnerPackage &&
		registration.Owner == spec.Owner &&
		registration.CapabilityID == string(spec.CapabilityID) &&
		registration.Boundary == spec.Boundary &&
		registration.RegistrationSymbol == spec.RegistrationSymbol &&
		registration.ResolverCall == spec.ResolverCall &&
		registration.RegistrarCall == spec.RegistrarCall
}

func loadSourceFunctions(directory string) (map[string][]sourceFunction, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	functions := make(map[string][]sourceFunction)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			receiver, receiverVar := functionReceiver(function)
			indexed := sourceFunction{declaration: function, receiver: receiver, receiverVar: receiverVar, rootObjects: functionRootObjects(function)}
			indexed.calls = reachableSourceCalls(indexed, function.Body)
			key := sourceFunctionKey(receiver, function.Name.Name)
			functions[key] = append(functions[key], indexed)
		}
	}
	return functions, nil
}

func reachableSourceCalls(function sourceFunction, root ast.Node) []sourceCall {
	var calls []sourceCall
	block, _ := root.(*ast.BlockStmt)
	scopes := []sourceLexicalScope{{block: block}}
	if function.declaration != nil && function.declaration.Body != nil && block != function.declaration.Body {
		scopes = append(scopes, sourceLexicalScope{block: function.declaration.Body})
	}
	appendReachableSourceCalls(function, root, scopes, nil, nil, false, make(map[*ast.FuncLit]bool), &calls)
	return calls
}

type sourceLexicalScope struct {
	block  *ast.BlockStmt
	before token.Pos
}

type sourceCallableResolution struct {
	literal         *ast.FuncLit
	target          *ast.Object
	targetName      string
	scopeIndex      int
	executionScopes []sourceLexicalScope
	ambiguous       bool
}

type sourceResolutionContext struct {
	scopes           []sourceLexicalScope
	startIndex       int
	lookupBefore     token.Pos
	invocationBefore token.Pos
}

type sourceExpressionResolver struct {
	objects   map[*ast.Object]bool
	factories map[ast.Node]bool
	selectors bool
}

type sourceCarrierResolution struct {
	expression ast.Expr
	context    sourceResolutionContext
	found      bool
	ambiguous  bool
}

const maxSourceExpressionResolutionDepth = 64

type sourceDiscardedProvenance struct {
	scopes []sourceLexicalScope
}

func appendReachableSourceCalls(function sourceFunction, root ast.Node, scopes []sourceLexicalScope, discarded []sourceDiscardedProvenance, returnedSite *ast.CallExpr, completingFrame bool, visiting map[*ast.FuncLit]bool, calls *[]sourceCall) {
	parents := astParentMap(root)
	inspectReachableWithParents(root, parents, func(node ast.Node) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return
		}
		_, deferred := parents[call].(*ast.DeferStmt)
		deferredInCurrentFrame := deferred && !completingFrame && !callWithinImmediatelyInvokedLiteral(call, parents)
		site := call
		if returnedSite != nil && len(scopes) > 0 && blockDirectlyReturnsCall(scopes[0].block, call) {
			site = returnedSite
		}
		resolution := resolveImmediateSourceCallable(scopes, discarded, call.Fun, call.Pos())
		sourceCall := sourceCallFromCall(function, call, site)
		applyNamedCallableIdentity(&sourceCall, resolution)
		sourceCall.ambiguous = resolution.ambiguous && !deferredInCurrentFrame
		*calls = append(*calls, sourceCall)
		appendOpaqueNamedCallbackCalls(function, scopes, call, site, calls)
		literal := resolution.literal
		scopeIndex := resolution.scopeIndex
		if literal == nil || visiting[literal] {
			return
		}
		nextScopes := resolution.executionScopes
		if len(nextScopes) == 0 {
			nextScopes = invokedLiteralLexicalScopes(literal, scopes, scopeIndex, call.Pos())
		}
		nextDiscarded := make([]sourceDiscardedProvenance, len(discarded), len(discarded)+scopeIndex)
		copy(nextDiscarded, discarded)
		for index := range scopes[:scopeIndex] {
			ancestry := append([]sourceLexicalScope(nil), scopes[index:]...)
			if ancestry[0].before == 0 {
				ancestry[0].before = call.Pos()
			}
			nextDiscarded = append(nextDiscarded, sourceDiscardedProvenance{scopes: ancestry})
		}
		visiting[literal] = true
		appendReachableSourceCalls(function, literal.Body, nextScopes, nextDiscarded, site, true, visiting, calls)
		delete(visiting, literal)
	})
}

func invokedLiteralLexicalScopes(literal *ast.FuncLit, scopes []sourceLexicalScope, scopeIndex int, callPosition token.Pos) []sourceLexicalScope {
	next := make([]sourceLexicalScope, 1, len(scopes)-scopeIndex+1)
	next[0] = sourceLexicalScope{block: literal.Body}
	for _, scope := range scopes[scopeIndex:] {
		if scope.before == 0 {
			scope.before = callPosition
		}
		next = append(next, scope)
	}
	return next
}

func sourceCallFromCall(function sourceFunction, semantic, site *ast.CallExpr) sourceCall {
	return sourceCall{
		name: calledName(semantic.Fun), path: expressionReference(semantic.Fun), object: sourceExpressionObject(semantic.Fun),
		localTarget: localCallTarget(semantic.Fun, function.receiverVar, function.receiver), position: site.Pos(), node: site,
	}
}

func applyNamedCallableIdentity(call *sourceCall, resolution sourceCallableResolution) {
	if call == nil || resolution.target == nil || resolution.targetName == "" {
		return
	}
	call.name = resolution.targetName
	call.path = resolution.targetName
	call.object = resolution.target
	call.localTarget = ""
}

func appendOpaqueNamedCallbackCalls(function sourceFunction, scopes []sourceLexicalScope, call, site *ast.CallExpr, calls *[]sourceCall) {
	if call == nil || site == nil {
		return
	}
	for _, argument := range call.Args {
		resolution := resolveCallbackExpression(scopes, argument, call.Pos())
		if resolution.target == nil || resolution.targetName == "" {
			continue
		}
		candidate := sourceCallFromCall(function, call, site)
		applyNamedCallableIdentity(&candidate, resolution)
		*calls = append(*calls, candidate)
	}
}

func newSourceExpressionResolver(selectors bool) *sourceExpressionResolver {
	return &sourceExpressionResolver{objects: make(map[*ast.Object]bool), factories: make(map[ast.Node]bool), selectors: selectors}
}

func resolveImmediateSourceCallable(scopes []sourceLexicalScope, discarded []sourceDiscardedProvenance, expression ast.Expr, before token.Pos) sourceCallableResolution {
	if len(scopes) == 0 || expression == nil {
		return sourceCallableResolution{scopeIndex: -1}
	}
	if object := callableRootObject(expression); object != nil && !capturedObjectStableBeforeCall(discarded, object) {
		return sourceCallableResolution{scopeIndex: -1, ambiguous: true}
	}
	context := sourceResolutionContext{scopes: scopes, lookupBefore: before, invocationBefore: before}
	return newSourceExpressionResolver(false).resolveCallable(context, expression, 0)
}

func resolveCallbackExpression(scopes []sourceLexicalScope, expression ast.Expr, before token.Pos) sourceCallableResolution {
	if len(scopes) == 0 || expression == nil {
		return sourceCallableResolution{scopeIndex: -1}
	}
	context := sourceResolutionContext{scopes: scopes, lookupBefore: before, invocationBefore: before}
	return newSourceExpressionResolver(true).resolveCallable(context, expression, 0)
}

func callableRootObject(expression ast.Expr) *ast.Object {
	switch value := unwrapParentheses(expression).(type) {
	case *ast.Ident:
		return value.Obj
	case *ast.TypeAssertExpr:
		return callableRootObject(value.X)
	case *ast.IndexExpr:
		return callableRootObject(value.X)
	case *ast.IndexListExpr:
		return callableRootObject(value.X)
	}
	return nil
}

func (resolver *sourceExpressionResolver) resolveCallable(context sourceResolutionContext, expression ast.Expr, depth int) sourceCallableResolution {
	if resolver == nil || expression == nil || depth > maxSourceExpressionResolutionDepth || context.startIndex < 0 || context.startIndex >= len(context.scopes) {
		return sourceCallableResolution{scopeIndex: -1, ambiguous: expression != nil}
	}
	expression = unwrapParentheses(expression)
	switch value := expression.(type) {
	case *ast.FuncLit:
		return sourceCallableResolution{
			literal: value, scopeIndex: context.startIndex,
			executionScopes: callbackLiteralExecutionScopes(value, context.scopes, context.startIndex, context.invocationBefore),
		}
	case *ast.Ident:
		return resolver.resolveCallableIdentifier(context, value, depth+1)
	case *ast.TypeAssertExpr:
		return resolver.resolveCallable(context, value.X, depth+1)
	case *ast.IndexListExpr:
		return resolver.resolveCallable(context, value.X, depth+1)
	case *ast.IndexExpr:
		if named := resolver.resolveCallable(context, value.X, depth+1); named.target != nil {
			return named
		}
	case *ast.SelectorExpr:
		if !resolver.selectors {
			return sourceCallableResolution{scopeIndex: -1}
		}
	case *ast.CallExpr:
	default:
		return sourceCallableResolution{scopeIndex: -1}
	}
	carrier := resolver.resolveCarrier(context, expression, depth+1)
	if carrier.ambiguous {
		return sourceCallableResolution{scopeIndex: -1, ambiguous: true}
	}
	if !carrier.found || carrier.expression == expression {
		return sourceCallableResolution{scopeIndex: -1}
	}
	resolved := resolver.resolveCallable(carrier.context, carrier.expression, depth+1)
	if resolved.literal == nil && resolved.target == nil && !resolved.ambiguous {
		resolved.ambiguous = true
	}
	return resolved
}

func (resolver *sourceExpressionResolver) resolveCallableIdentifier(context sourceResolutionContext, identifier *ast.Ident, depth int) sourceCallableResolution {
	if identifier == nil || identifier.Obj == nil {
		return sourceCallableResolution{scopeIndex: -1}
	}
	object := identifier.Obj
	if object.Kind == ast.Fun {
		return sourceCallableResolution{target: object, targetName: object.Name, scopeIndex: context.startIndex}
	}
	if resolver.objects[object] {
		return sourceCallableResolution{scopeIndex: -1, ambiguous: true}
	}
	state, scopeIndex, found, ambiguous := localIdentifierState(context.scopes, object, context.startIndex, context.lookupBefore)
	if ambiguous {
		return sourceCallableResolution{scopeIndex: -1, ambiguous: true}
	}
	if !found {
		return sourceCallableResolution{scopeIndex: -1}
	}
	resolver.objects[object] = true
	defer delete(resolver.objects, object)
	next := context
	next.startIndex = scopeIndex
	next.lookupBefore = state.position
	resolved := resolver.resolveCallable(next, state.value, depth+1)
	if resolved.literal == nil && resolved.target == nil && !resolved.ambiguous && sourceObjectIsFunctionLocal(context.scopes[scopeIndex:], object) {
		resolved.ambiguous = true
	}
	return resolved
}

func localIdentifierState(scopes []sourceLexicalScope, object *ast.Object, startIndex int, firstBefore token.Pos) (sourceIdentifierState, int, bool, bool) {
	if object == nil || startIndex < 0 || startIndex >= len(scopes) {
		return sourceIdentifierState{}, -1, false, false
	}
	for index := startIndex; index < len(scopes); index++ {
		scope := scopes[index]
		cutoff := scope.before
		if index == startIndex && firstBefore != 0 {
			cutoff = firstBefore
		}
		if cutoff == 0 && scope.block != nil {
			cutoff = scope.block.End()
		}
		state := identifierStateAfterBlock(scope.block, object, cutoff, sourceIdentifierState{})
		if state.ambiguous {
			if sourceObjectIsFunctionLocal(scopes[index:], object) {
				return state, index, false, true
			}
			continue
		}
		if state.found {
			return state, index, true, false
		}
	}
	return sourceIdentifierState{}, -1, false, false
}

func sourceObjectIsFunctionLocal(scopes []sourceLexicalScope, object *ast.Object) bool {
	if sourceObjectIsLocalToScopes(scopes, object) {
		return true
	}
	if object == nil || object.Kind != ast.Var {
		return false
	}
	_, parameter := object.Decl.(*ast.Field)
	return parameter
}

func sourceObjectIsLocalToScopes(scopes []sourceLexicalScope, object *ast.Object) bool {
	if object == nil {
		return false
	}
	declaration, ok := object.Decl.(ast.Node)
	if !ok || declaration == nil {
		return false
	}
	for _, scope := range scopes {
		if scope.block != nil && astNodeContains(scope.block, declaration) {
			return true
		}
	}
	return false
}

func capturedObjectStableBeforeCall(frames []sourceDiscardedProvenance, object *ast.Object) bool {
	for _, frame := range frames {
		if len(frame.scopes) == 0 || frame.scopes[0].before == 0 || !objectProvenanceStableBefore(frame.scopes, object, make(map[*ast.FuncLit]bool), false) {
			return false
		}
	}
	return true
}

func objectProvenanceStableBefore(scopes []sourceLexicalScope, object *ast.Object, visiting map[*ast.FuncLit]bool, completingFrame bool) bool {
	if len(scopes) == 0 || scopes[0].block == nil || object == nil {
		return false
	}
	before := scopes[0].before
	if before == 0 {
		before = scopes[0].block.End()
	}
	stable := true
	parents := astParentMap(scopes[0].block)
	inspectReachableWithParents(scopes[0].block, parents, func(node ast.Node) {
		if !stable || node == nil || node.Pos() >= before {
			return
		}
		if astNodeWritesObject(node, object) || astNodeEscapesObject(node, object) {
			stable = false
			return
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return
		}
		if _, deferred := parents[call].(*ast.DeferStmt); deferred && !completingFrame && !callWithinImmediatelyInvokedLiteral(call, parents) {
			return
		}
		resolution := resolveImmediateSourceCallable(scopes, nil, call.Fun, call.Pos())
		if resolution.ambiguous {
			stable = false
			return
		}
		literal := resolution.literal
		if literal == nil {
			if opaqueCallArgumentEscapesObject(scopes, call, object) {
				stable = false
			}
			return
		}
		if visiting[literal] {
			stable = false
			return
		}
		visiting[literal] = true
		stable = objectProvenanceStableBefore(resolution.executionScopes, object, visiting, true)
		delete(visiting, literal)
	})
	return stable
}

func opaqueCallArgumentEscapesObject(scopes []sourceLexicalScope, call *ast.CallExpr, object *ast.Object) bool {
	if call == nil || object == nil {
		return false
	}
	for _, argument := range call.Args {
		resolution := resolveCallbackExpression(scopes, argument, call.Pos())
		if resolution.ambiguous {
			return true
		}
		if resolution.literal == nil {
			continue
		}
		if !objectProvenanceStableBefore(resolution.executionScopes, object, make(map[*ast.FuncLit]bool), true) {
			return true
		}
	}
	return false
}

func callbackLiteralExecutionScopes(literal *ast.FuncLit, scopes []sourceLexicalScope, scopeIndex int, before token.Pos) []sourceLexicalScope {
	next := invokedLiteralLexicalScopes(literal, scopes, scopeIndex, before)
	if scopeIndex == 0 && len(next) > 1 {
		next[1].before = before
	}
	return next
}

func (resolver *sourceExpressionResolver) resolveCarrier(context sourceResolutionContext, expression ast.Expr, depth int) sourceCarrierResolution {
	if resolver == nil || expression == nil || depth > maxSourceExpressionResolutionDepth || context.startIndex < 0 || context.startIndex >= len(context.scopes) {
		return sourceCarrierResolution{ambiguous: expression != nil}
	}
	expression = unwrapParentheses(expression)
	switch value := expression.(type) {
	case *ast.CompositeLit, *ast.FuncLit:
		return sourceCarrierResolution{expression: expression, context: context, found: true}
	case *ast.Ident:
		if value.Obj == nil || value.Obj.Kind == ast.Fun {
			return sourceCarrierResolution{expression: expression, context: context, found: value.Obj != nil}
		}
		if resolver.objects[value.Obj] {
			return sourceCarrierResolution{ambiguous: true}
		}
		state, scopeIndex, found, ambiguous := localIdentifierState(context.scopes, value.Obj, context.startIndex, context.lookupBefore)
		if ambiguous {
			if !state.ambiguous && !found {
				return sourceCarrierResolution{}
			}
			return sourceCarrierResolution{ambiguous: true}
		}
		if !found {
			return sourceCarrierResolution{}
		}
		resolver.objects[value.Obj] = true
		defer delete(resolver.objects, value.Obj)
		next := context
		next.startIndex = scopeIndex
		next.lookupBefore = state.position
		resolved := resolver.resolveCarrier(next, state.value, depth+1)
		if !resolved.found && !resolved.ambiguous && sourceObjectIsFunctionLocal(context.scopes[scopeIndex:], value.Obj) {
			resolved.ambiguous = true
		}
		return resolved
	case *ast.TypeAssertExpr:
		return resolver.resolveCarrier(context, value.X, depth+1)
	case *ast.IndexListExpr:
		return resolver.resolveCarrier(context, value.X, depth+1)
	case *ast.IndexExpr:
		container := resolver.resolveCarrier(context, value.X, depth+1)
		if container.ambiguous || !container.found {
			return container
		}
		selected, ok := indexedCallbackExpression(container.expression, value.Index)
		if !ok {
			return sourceCarrierResolution{ambiguous: expressionHasLocalRoot(context.scopes, value.X)}
		}
		return sourceCarrierResolution{expression: selected, context: container.context, found: true}
	case *ast.SelectorExpr:
		container := resolver.resolveCarrier(context, value.X, depth+1)
		if container.ambiguous || !container.found {
			return container
		}
		selected, ok := selectedCallbackExpression(container.expression, value.Sel.Name)
		if !ok {
			return sourceCarrierResolution{ambiguous: expressionHasLocalRoot(context.scopes, value.X)}
		}
		return sourceCarrierResolution{expression: selected, context: container.context, found: true}
	case *ast.CallExpr:
		factory := resolver.resolveCallable(context, value.Fun, depth+1)
		if factory.ambiguous {
			return sourceCarrierResolution{ambiguous: true}
		}
		var factoryNode ast.Node
		var body *ast.BlockStmt
		factoryScopes := factory.executionScopes
		if factory.literal != nil {
			factoryNode = factory.literal
			body = factory.literal.Body
		} else if declaration, ok := callableTargetDeclaration(factory.target); ok {
			factoryNode = declaration
			body = declaration.Body
			factoryScopes = []sourceLexicalScope{{block: body}}
		} else {
			return sourceCarrierResolution{ambiguous: expressionHasLocalRoot(context.scopes, value.Fun)}
		}
		if factoryNode == nil || resolver.factories[factoryNode] {
			return sourceCarrierResolution{ambiguous: true}
		}
		result, ok := directlyReturnedExpression(body)
		if !ok || len(factoryScopes) == 0 {
			return sourceCarrierResolution{ambiguous: true}
		}
		resolver.factories[factoryNode] = true
		defer delete(resolver.factories, factoryNode)
		resultContext := sourceResolutionContext{scopes: factoryScopes, lookupBefore: result.Pos(), invocationBefore: result.Pos()}
		return resolver.resolveCarrier(resultContext, result, depth+1)
	default:
		return sourceCarrierResolution{expression: expression, context: context, found: true}
	}
}

func callableTargetDeclaration(object *ast.Object) (*ast.FuncDecl, bool) {
	if object == nil || object.Kind != ast.Fun {
		return nil, false
	}
	declaration, ok := object.Decl.(*ast.FuncDecl)
	return declaration, ok && declaration.Body != nil
}

func expressionHasLocalRoot(scopes []sourceLexicalScope, expression ast.Expr) bool {
	object := sourceExpressionObject(unwrapParentheses(expression))
	return object != nil && sourceObjectIsFunctionLocal(scopes, object)
}

func indexedCallbackExpression(expression ast.Expr, index ast.Expr) (ast.Expr, bool) {
	composite, ok := unwrapParentheses(expression).(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	if _, mapLiteral := composite.Type.(*ast.MapType); mapLiteral {
		var selected ast.Expr
		for _, element := range composite.Elts {
			binding, ok := element.(*ast.KeyValueExpr)
			if !ok || !exactCallbackMapKey(binding.Key, index) {
				continue
			}
			if selected != nil {
				return nil, false
			}
			selected = binding.Value
		}
		return selected, selected != nil
	}
	literalIndex, ok := unwrapParentheses(index).(*ast.BasicLit)
	if !ok || literalIndex.Kind != token.INT {
		return nil, false
	}
	target, err := strconv.Atoi(literalIndex.Value)
	if err != nil || target < 0 {
		return nil, false
	}
	position := 0
	for _, element := range composite.Elts {
		value := element
		if binding, keyed := element.(*ast.KeyValueExpr); keyed {
			key, keyOK := unwrapParentheses(binding.Key).(*ast.BasicLit)
			if !keyOK || key.Kind != token.INT {
				return nil, false
			}
			position, err = strconv.Atoi(key.Value)
			if err != nil || position < 0 {
				return nil, false
			}
			value = binding.Value
		}
		if position == target {
			return value, true
		}
		position++
	}
	return nil, false
}

func exactCallbackMapKey(left, right ast.Expr) bool {
	left = unwrapParentheses(left)
	right = unwrapParentheses(right)
	switch leftValue := left.(type) {
	case *ast.BasicLit:
		rightValue, ok := right.(*ast.BasicLit)
		return ok && leftValue.Kind == rightValue.Kind && leftValue.Value == rightValue.Value
	case *ast.Ident:
		rightValue, ok := right.(*ast.Ident)
		return ok && leftValue.Obj != nil && leftValue.Obj == rightValue.Obj
	}
	return false
}

func selectedCallbackExpression(expression ast.Expr, field string) (ast.Expr, bool) {
	composite, ok := unwrapParentheses(expression).(*ast.CompositeLit)
	if !ok || field == "" {
		return nil, false
	}
	var selected ast.Expr
	for _, element := range composite.Elts {
		binding, ok := element.(*ast.KeyValueExpr)
		if !ok || expressionReference(binding.Key) != field {
			continue
		}
		if selected != nil {
			return nil, false
		}
		selected = binding.Value
	}
	return selected, selected != nil
}

func directlyReturnedExpression(block *ast.BlockStmt) (ast.Expr, bool) {
	if block == nil || len(block.List) != 1 {
		return nil, false
	}
	result, ok := block.List[0].(*ast.ReturnStmt)
	returnExpressionOK := ok && len(result.Results) == 1
	if !returnExpressionOK {
		return nil, false
	}
	return result.Results[0], true
}

func callWithinImmediatelyInvokedLiteral(call *ast.CallExpr, parents map[ast.Node]ast.Node) bool {
	for current := parents[call]; current != nil; current = parents[current] {
		if literal, ok := current.(*ast.FuncLit); ok {
			return directlyInvokedFunctionLiteral(literal, parents)
		}
	}
	return false
}

func astNodeEscapesObject(node ast.Node, object *ast.Object) bool {
	if node == nil || object == nil {
		return false
	}
	switch expression := node.(type) {
	case *ast.UnaryExpr:
		return expression.Op == token.AND && sourceExpressionObject(unwrapParentheses(expression.X)) == object
	case *ast.CallExpr:
		method, ok := unwrapParentheses(expression.Fun).(*ast.SelectorExpr)
		return ok && sourceExpressionObject(method.X) == object
	case *ast.SelectorExpr:
		return sourceExpressionObject(expression) == object
	}
	return false
}

func blockDirectlyReturnsCall(block *ast.BlockStmt, call *ast.CallExpr) bool {
	if block == nil || len(block.List) == 0 || call == nil {
		return false
	}
	for _, statement := range block.List[:len(block.List)-1] {
		if !statementBindsOnlyFunctionLiterals(statement) {
			return false
		}
	}
	result, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	return ok && len(result.Results) == 1 && unwrapParentheses(result.Results[0]) == call
}

func statementBindsOnlyFunctionLiterals(statement ast.Stmt) bool {
	assignment, ok := statement.(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) == 0 || len(assignment.Lhs) != len(assignment.Rhs) {
		return false
	}
	for index, expression := range assignment.Rhs {
		identifier, identifierOK := assignment.Lhs[index].(*ast.Ident)
		_, literalOK := unwrapParentheses(expression).(*ast.FuncLit)
		if !identifierOK || identifier.Obj == nil || !literalOK {
			return false
		}
	}
	return true
}

func functionRootObjects(function *ast.FuncDecl) map[string]*ast.Object {
	objects := make(map[string]*ast.Object)
	if function == nil {
		return objects
	}
	if function.Recv != nil {
		for _, field := range function.Recv.List {
			for _, name := range field.Names {
				objects[name.Name] = name.Obj
			}
		}
	}
	if function.Type != nil && function.Type.Params != nil {
		for _, field := range function.Type.Params.List {
			for _, name := range field.Names {
				objects[name.Name] = name.Obj
			}
		}
	}
	return objects
}

// inspectReachable excludes statically dead branches and uninvoked closures so
// dead registration, resolver, helper, and guard/effect evidence cannot satisfy
// the structural contract.
func inspectReachable(root ast.Node, visit func(ast.Node)) {
	if root == nil {
		return
	}
	parents := astParentMap(root)
	inspectReachableWithParents(root, parents, visit)
}

func inspectReachableWithParents(root ast.Node, parents map[ast.Node]ast.Node, visit func(ast.Node)) {
	if root == nil {
		return
	}
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if block, ok := node.(*ast.BlockStmt); ok {
			if statement, parentOK := parents[block].(*ast.IfStmt); parentOK && statement.Body == block && isStaticFalse(statement.Cond) {
				return false
			}
		}
		if statement, ok := parents[node].(*ast.IfStmt); ok && statement.Else == node && isStaticTrue(statement.Cond) {
			return false
		}
		visit(node)
		if literal, ok := node.(*ast.FuncLit); ok && !directlyInvokedFunctionLiteral(literal, parents) {
			return false
		}
		return true
	})
}

func directlyInvokedFunctionLiteral(literal *ast.FuncLit, parents map[ast.Node]ast.Node) bool {
	current := ast.Node(literal)
	for {
		parent := parents[current]
		if parenthesized, ok := parent.(*ast.ParenExpr); ok {
			current = parenthesized
			continue
		}
		call, ok := parent.(*ast.CallExpr)
		return ok && call.Fun == current
	}
}

func inspectReachableCalls(root ast.Node, visit func(*ast.CallExpr)) {
	inspectReachable(root, func(node ast.Node) {
		if call, ok := node.(*ast.CallExpr); ok {
			visit(call)
		}
	})
}

func isStaticFalse(expression ast.Expr) bool {
	literal, ok := expression.(*ast.Ident)
	return ok && literal.Name == "false"
}

func isStaticTrue(expression ast.Expr) bool {
	literal, ok := expression.(*ast.Ident)
	return ok && literal.Name == "true"
}

func functionReceiver(function *ast.FuncDecl) (string, string) {
	if function == nil || function.Recv == nil || len(function.Recv.List) != 1 {
		return "", ""
	}
	field := function.Recv.List[0]
	receiver := receiverTypeName(field.Type)
	variable := ""
	if len(field.Names) == 1 {
		variable = field.Names[0].Name
	}
	return receiver, variable
}

func receiverTypeName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverTypeName(value.X)
	case *ast.IndexExpr:
		return receiverTypeName(value.X)
	case *ast.IndexListExpr:
		return receiverTypeName(value.X)
	case *ast.SelectorExpr:
		if value.Sel != nil {
			return value.Sel.Name
		}
	}
	return ""
}

func sourceFunctionKey(receiver, name string) string {
	if receiver == "" {
		return name
	}
	return receiver + "." + name
}

func localCallTarget(expression ast.Expr, receiverVar, receiver string) string {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || receiverVar == "" || receiver == "" {
		return ""
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok || identifier.Name != receiverVar {
		return ""
	}
	return sourceFunctionKey(receiver, selector.Sel.Name)
}

func calledName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		if value.Sel != nil {
			return value.Sel.Name
		}
	case *ast.IndexExpr:
		return calledName(value.X)
	case *ast.IndexListExpr:
		return calledName(value.X)
	}
	return ""
}

func descriptorField(literal *ast.CompositeLit, name string) ast.Expr {
	for _, element := range literal.Elts {
		keyed, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		identifier, ok := keyed.Key.(*ast.Ident)
		if ok && identifier.Name == name {
			return keyed.Value
		}
	}
	return nil
}

func isDescriptorLiteral(literal *ast.CompositeLit) bool {
	if literal == nil {
		return false
	}
	if calledName(literal.Type) == "EffectDescriptor" {
		return true
	}
	return descriptorField(literal, "ID") != nil && descriptorField(literal, "CapabilityID") != nil && descriptorField(literal, "Boundary") != nil && descriptorField(literal, "Owner") != nil
}

func expressionReference(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := expressionReference(value.X)
		if prefix != "" && value.Sel != nil {
			return prefix + "." + value.Sel.Name
		}
	case *ast.CallExpr:
		if len(value.Args) == 1 && isSafeSourceStringConversion(value.Fun) {
			return expressionReference(value.Args[0])
		}
	case *ast.ParenExpr:
		return expressionReference(value.X)
	}
	return ""
}

func resolvedCapabilityBindings(function *ast.FuncDecl) []sourceCapabilityBinding {
	var bindings []sourceCapabilityBinding
	if function == nil || function.Body == nil {
		return bindings
	}
	inspectReachable(function.Body, func(node ast.Node) {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) == 0 || len(assignment.Rhs) != 1 {
			return
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok || !isCapabilityResolverCall(function, call) || len(call.Args) != 1 {
			return
		}
		bindingExpression := assignment.Lhs[0]
		binding := expressionReference(bindingExpression)
		effect := expressionReference(call.Args[0])
		if binding != "" && effect != "" {
			bindings = append(bindings, sourceCapabilityBinding{
				reference: binding, object: sourceExpressionObject(bindingExpression), effectReference: effect,
				resolverPath: expressionReference(call.Fun), resolverObject: sourceExpressionObject(call.Fun), effectObject: sourceExpressionObject(call.Args[0]), errorObject: sourceExpressionObject(assignment.Lhs[len(assignment.Lhs)-1]), position: assignment.Pos(),
			})
		}
	})
	return bindings
}

func isCapabilityResolverCall(function *ast.FuncDecl, call *ast.CallExpr) bool {
	if function == nil || call == nil || len(call.Args) != 1 {
		return false
	}
	path := expressionReference(call.Fun)
	if path != "authorities.CapabilityBindings.Resolve" && path != "options.Authorities.CapabilityBindings.Resolve" && path != "financial.Authorities.CapabilityBindings.Resolve" {
		return false
	}
	root := strings.SplitN(path, ".", 2)[0]
	expected := functionRootObjects(function)[root]
	return expected != nil && sourceExpressionObject(call.Fun) == expected
}

func resolvedCapabilityBindingForExpression(expression ast.Expr, before token.Pos, bindings []sourceCapabilityBinding) sourceCapabilityBinding {
	reference := expressionReference(expression)
	object := sourceExpressionObject(expression)
	var matched sourceCapabilityBinding
	for _, binding := range bindings {
		if binding.reference != reference || binding.position >= before || binding.position <= matched.position {
			continue
		}
		if object != nil && binding.object != nil && object != binding.object {
			continue
		}
		matched = binding
	}
	return matched
}

func resolvedCapabilityEffectReference(expression ast.Expr, before token.Pos, bindings []sourceCapabilityBinding) string {
	return resolvedCapabilityBindingForExpression(expression, before, bindings).effectReference
}

func validAuthorityRegistrationPair(binding sourceCapabilityBinding, registrar *sourceRegistrarCall) bool {
	if registrar == nil || binding.resolverObject == nil || registrar.object == nil {
		return false
	}
	pairs := map[string]string{
		"authorities.CapabilityBindings.Resolve":           "effects.Register",
		"options.Authorities.CapabilityBindings.Resolve":   "options.Effects.Register",
		"financial.Authorities.CapabilityBindings.Resolve": "financial.Effects.Register",
	}
	if pairs[binding.resolverPath] != registrar.path {
		return false
	}
	if strings.HasPrefix(binding.resolverPath, "options.") || strings.HasPrefix(binding.resolverPath, "financial.") {
		return binding.resolverObject == registrar.object
	}
	return true
}

func sourceExpressionObject(expression ast.Expr) *ast.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Obj
	case *ast.SelectorExpr:
		return sourceExpressionObject(value.X)
	case *ast.CallExpr:
		if len(value.Args) == 1 && isSafeSourceStringConversion(value.Fun) {
			return sourceExpressionObject(value.Args[0])
		}
	case *ast.ParenExpr:
		return sourceExpressionObject(value.X)
	}
	return nil
}

func functionParameterNames(function *ast.FuncDecl) []string {
	if function == nil || function.Type == nil || function.Type.Params == nil {
		return nil
	}
	var names []string
	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
}

func guardEffectContractPresent(functions map[string][]sourceFunction, guardContract, effectContract string) bool {
	guardPath, guardCall, ok := parseCallContract(guardContract)
	if !ok || len(guardPath) != 1 {
		return false
	}
	effectPath, effectCall, ok := parseCallContract(effectContract)
	if !ok || len(effectPath) == 0 || effectPath[0] != guardPath[0] {
		return false
	}
	for _, root := range functions[guardPath[0]] {
		rootTarget := effectCall
		if len(effectPath) > 1 {
			rootTarget = effectPath[1]
		}
		if !guardDominatesTarget(root, guardCall, rootTarget) {
			continue
		}
		if len(effectPath) == 1 {
			return true
		}
		valid := true
		for index := 1; index < len(effectPath); index++ {
			candidates := functions[effectPath[index]]
			if len(candidates) == 0 {
				valid = false
				break
			}
			if index == len(effectPath)-1 {
				matched := false
				for _, candidate := range candidates {
					if firstCallPosition(candidate, effectCall) != 0 {
						matched = true
						break
					}
				}
				valid = matched
				break
			}
			if !hasCallAfter(candidates[0], effectPath[index+1], 0) {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}

func guardDominatesTarget(function sourceFunction, guardName, targetName string) bool {
	if function.declaration == nil || function.declaration.Body == nil {
		return false
	}
	for _, call := range function.calls {
		if call.ambiguous {
			return false
		}
	}
	parents := astParentMap(function.declaration.Body)
	var guards, targets []sourceCall
	for _, call := range function.calls {
		if sourceCallMatches(function, call, guardName) {
			guards = append(guards, call)
		}
		if sourceCallMatches(function, call, targetName) {
			targets = append(targets, call)
		}
	}
	if len(guards) == 0 || len(targets) == 0 {
		return false
	}
	var gates []ast.Stmt
	for _, guard := range guards {
		check, ok := checkedGuardStatement(guard.node, parents)
		if !ok {
			continue
		}
		if blockContainsSourceCall(function, check.Body, targetName) {
			return false
		}
		if !blockAlwaysExits(check.Body) {
			continue
		}
		gates = append(gates, check)
		if optional := optionalGuardBoundary(check, guard, parents); optional != nil {
			gates = append(gates, optional)
		}
	}
	if len(gates) == 0 {
		return false
	}
	for _, target := range targets {
		dominated := false
		for _, gate := range gates {
			if statementDominatesCall(gate, target.node, parents) {
				dominated = true
				break
			}
		}
		if !dominated {
			return false
		}
	}
	return true
}

func checkedGuardStatement(call *ast.CallExpr, parents map[ast.Node]ast.Node) (*ast.IfStmt, bool) {
	if call == nil {
		return nil, false
	}
	current := ast.Node(call)
	for {
		parenthesized, ok := parents[current].(*ast.ParenExpr)
		if !ok {
			break
		}
		current = parenthesized
	}
	assignment, ok := parents[current].(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) == 0 || len(assignment.Rhs) != 1 || assignment.Rhs[0] != current {
		return nil, false
	}
	check, ok := parents[assignment].(*ast.IfStmt)
	if !ok || check.Init != assignment {
		return nil, false
	}
	errorObject := sourceExpressionObject(assignment.Lhs[len(assignment.Lhs)-1])
	if errorObject == nil || !expressionRequiresNonNilObject(check.Cond, errorObject) {
		return nil, false
	}
	return check, true
}

func blockContainsSourceCall(function sourceFunction, block *ast.BlockStmt, name string) bool {
	for _, candidate := range reachableSourceCalls(function, block) {
		if sourceCallMatches(function, candidate, name) {
			return true
		}
	}
	return false
}

func blockAlwaysExits(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.List {
		if statementAlwaysExits(statement) {
			return true
		}
	}
	return false
}

func statementAlwaysExits(statement ast.Stmt) bool {
	switch statement := statement.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return statement.Tok == token.BREAK || statement.Tok == token.CONTINUE
	case *ast.BlockStmt:
		return blockAlwaysExits(statement)
	case *ast.LabeledStmt:
		return statementAlwaysExits(statement.Stmt)
	case *ast.IfStmt:
		if isStaticTrue(statement.Cond) {
			return blockAlwaysExits(statement.Body)
		}
		if isStaticFalse(statement.Cond) {
			return elseAlwaysExits(statement.Else)
		}
		return blockAlwaysExits(statement.Body) && elseAlwaysExits(statement.Else)
	}
	return false
}

func elseAlwaysExits(statement ast.Stmt) bool {
	if statement == nil {
		return false
	}
	return statementAlwaysExits(statement)
}

func optionalGuardBoundary(check *ast.IfStmt, guard sourceCall, parents map[ast.Node]ast.Node) *ast.IfStmt {
	block, ok := parents[check].(*ast.BlockStmt)
	if !ok {
		return nil
	}
	wrapper, ok := parents[block].(*ast.IfStmt)
	if !ok || wrapper.Body != block || wrapper.Else != nil {
		return nil
	}
	receiver, _, found := strings.Cut(guard.path, ".")
	if !found {
		return nil
	}
	if index := strings.LastIndex(guard.path, "."); index > 0 {
		receiver = guard.path[:index]
	}
	if !expressionRequiresNonNilReference(wrapper.Cond, receiver) {
		return nil
	}
	return wrapper
}

func expressionRequiresNonNilReference(expression ast.Expr, reference string) bool {
	comparison, ok := unwrapParentheses(expression).(*ast.BinaryExpr)
	if !ok || comparison.Op != token.NEQ || reference == "" {
		return false
	}
	left := unwrapParentheses(comparison.X)
	right := unwrapParentheses(comparison.Y)
	return (expressionReference(left) == reference && isNilExpression(right)) ||
		(expressionReference(right) == reference && isNilExpression(left))
}

func statementDominatesCall(gate ast.Stmt, call *ast.CallExpr, parents map[ast.Node]ast.Node) bool {
	if gate == nil || call == nil {
		return false
	}
	container, statements := statementContainer(gate, parents)
	if container == nil {
		return false
	}
	current := ast.Node(call)
	for current != nil && parents[current] != container {
		current = parents[current]
	}
	target, ok := current.(ast.Stmt)
	if !ok {
		return false
	}
	gateIndex, targetIndex := -1, -1
	for index, statement := range statements {
		if statement == gate {
			gateIndex = index
		}
		if statement == target {
			targetIndex = index
		}
	}
	return gateIndex >= 0 && targetIndex > gateIndex
}

func statementContainer(statement ast.Stmt, parents map[ast.Node]ast.Node) (ast.Node, []ast.Stmt) {
	switch container := parents[statement].(type) {
	case *ast.BlockStmt:
		return container, container.List
	case *ast.CaseClause:
		return container, container.Body
	case *ast.CommClause:
		return container, container.Body
	default:
		return nil, nil
	}
}

func parseCallContract(contract string) ([]string, string, bool) {
	parts := strings.SplitN(contract, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, "", false
	}
	path := strings.Split(parts[0], ">")
	for _, function := range path {
		if strings.TrimSpace(function) == "" {
			return nil, "", false
		}
	}
	return path, parts[1], true
}

func firstCallPosition(function sourceFunction, name string) token.Pos {
	return firstCallPositionAfter(function, name, 0)
}

func firstCallPositionAfter(function sourceFunction, name string, after token.Pos) token.Pos {
	for _, call := range function.calls {
		if sourceCallMatches(function, call, name) && call.position > after {
			return call.position
		}
	}
	return 0
}

func sourceCallMatches(function sourceFunction, call sourceCall, name string) bool {
	if strings.Contains(name, ".") {
		return exactSourceCallMatches(function, call, name) || call.localTarget == name
	}
	return call.name == name
}

func exactSourceCallMatches(function sourceFunction, call sourceCall, contract string) bool {
	if call.path != contract || call.object == nil {
		return false
	}
	root, _, _ := strings.Cut(contract, ".")
	expected, scoped := function.rootObjects[root]
	return !scoped || (expected != nil && call.object == expected)
}

func hasCallAfter(function sourceFunction, name string, after token.Pos) bool {
	return firstCallPositionAfter(function, name, after) != 0
}

func VerifyEffectCoverage(args ...any) error {
	options := effectCoverageOptions{}
	for _, arg := range args {
		switch value := arg.(type) {
		case string:
			if strings.HasSuffix(value, ".json") {
				options.manifestPath = value
			} else if options.repoRoot == "" {
				options.repoRoot = value
			}
		case EffectSurfaceManifest:
			options.manifest = value
		case []EffectSurface:
			options.expected = value
		case []EffectDescriptor:
			options.runtime = value
		case *EffectRegistry:
			options.registry = value
		case AuthoredContractV1:
			options.contract = value
		case DeploymentProfile:
			options.profile = value
		case RuntimeAuthorities:
			options.authorities = value
		case EffectCoverageOptions:
			options = value.options()
		case *EffectCoverageOptions:
			if value != nil {
				options = value.options()
			}
		}
	}
	if options.manifest.SchemaVersion == "" && options.manifestPath != "" {
		manifest, err := LoadEffectSurfaceManifest(options.manifestPath)
		if err != nil {
			return err
		}
		options.manifest = manifest
	}
	if options.manifest.SchemaVersion == "" && len(options.expected) > 0 {
		options.manifest = EffectSurfaceManifest{SchemaVersion: EffectSurfaceSchemaV1, Surfaces: options.expected}
	}
	if err := validateEffectSurfaceManifest(options.manifest); err != nil {
		return err
	}
	if err := validateEffectCoverageAuthorities(options.contract, options.profile, options.authorities); err != nil {
		return err
	}
	if options.selection == nil {
		return &EffectCoverageError{Code: "effect_coverage_configuration_required"}
	}
	if options.repoRoot != "" {
		static, err := DiscoverEffectSurfaces(options.repoRoot)
		if err != nil {
			return err
		}
		if err := joinStatic(options.manifest.Surfaces, static); err != nil {
			return err
		}
	}
	runtime := options.runtime
	if options.registry != nil {
		runtime = options.registry.Snapshot()
	}
	if len(runtime) == 0 {
		return &EffectCoverageError{Code: "effect_coverage_zero_runtime"}
	}
	if err := joinRuntime(options.manifest.Surfaces, options.profile, runtime, options.selection); err != nil {
		return err
	}
	runtimeForValidation := append([]EffectDescriptor(nil), runtime...)
	sort.Slice(runtimeForValidation, func(i, j int) bool { return runtimeForValidation[i].ID < runtimeForValidation[j].ID })
	for _, descriptor := range runtimeForValidation {
		effectID := effectIDForDescriptor(descriptor)
		if effectID == "" {
			return &EffectCoverageError{Code: "effect_coverage_unknown_effect", Field: descriptor.ID}
		}
		capability, err := options.authorities.CapabilityBindings.Resolve(effectID)
		if err != nil {
			return &EffectCoverageError{Code: "effect_coverage_unknown_effect", Field: descriptor.ID}
		}
		if string(capability) != descriptor.CapabilityID {
			return &EffectCoverageError{Code: "effect_coverage_capability_mismatch", Field: descriptor.ID}
		}
	}
	return nil
}

func validateEffectCoverageAuthorities(contract AuthoredContractV1, profile DeploymentProfile, authorities RuntimeAuthorities) error {
	if strings.TrimSpace(contract.SchemaVersion) == "" || len(contract.Capabilities) == 0 || len(contract.Profiles) == 0 {
		return &EffectCoverageError{Code: "effect_coverage_contract_required"}
	}
	if err := validateReadinessProfile(contract, profile); err != nil {
		return &EffectCoverageError{Code: "effect_coverage_profile_invalid", Field: profile.ID, Err: err}
	}
	if !authorities.Valid() {
		return &EffectCoverageError{Code: "effect_coverage_authorities_required"}
	}
	digest, err := Digest(contract)
	if err != nil {
		return &EffectCoverageError{Code: "effect_coverage_contract_invalid", Err: err}
	}
	resolver, ok := authorities.CatalogAuthorizer.resolver.(compiledCatalogResolver)
	if !ok || resolver.digest != digest {
		return &EffectCoverageError{Code: "effect_coverage_authorities_mismatch"}
	}
	return nil
}

func effectIDForDescriptor(descriptor EffectDescriptor) EffectID {
	if spec, ok := runtimeDescriptorSpecs[descriptor.ID]; ok {
		return spec.EffectID
	}
	return ""
}

type EffectCoverageOptions struct {
	RepoRoot     string
	Manifest     EffectSurfaceManifest
	ManifestPath string
	Expected     []EffectSurface
	Runtime      []EffectDescriptor
	Registry     *EffectRegistry
	Contract     AuthoredContractV1
	Profile      DeploymentProfile
	Authorities  RuntimeAuthorities
	Selection    *EffectRuntimeConfiguration
}

type effectCoverageOptions struct {
	repoRoot     string
	manifestPath string
	manifest     EffectSurfaceManifest
	expected     []EffectSurface
	runtime      []EffectDescriptor
	registry     *EffectRegistry
	contract     AuthoredContractV1
	profile      DeploymentProfile
	authorities  RuntimeAuthorities
	selection    *EffectRuntimeConfiguration
}

func (o EffectCoverageOptions) options() effectCoverageOptions {
	return effectCoverageOptions{repoRoot: o.RepoRoot, manifestPath: o.ManifestPath, manifest: o.Manifest, expected: o.Expected, runtime: o.Runtime, registry: o.Registry, contract: o.Contract, profile: o.Profile, authorities: o.Authorities, selection: o.Selection}
}

func joinStatic(expected, discovered []EffectSurface) error {
	expectedRows := append([]EffectSurface(nil), expected...)
	discoveredRows := append([]EffectSurface(nil), discovered...)
	sort.SliceStable(expectedRows, func(i, j int) bool { return expectedRows[i].SeamID < expectedRows[j].SeamID })
	sort.SliceStable(discoveredRows, func(i, j int) bool { return discoveredRows[i].SeamID < discoveredRows[j].SeamID })
	discoveredByKey := make(map[string]EffectSurface, len(discovered))
	for _, surface := range discoveredRows {
		key := surface.SeamID
		if _, duplicate := discoveredByKey[key]; duplicate {
			return &EffectCoverageError{Code: "effect_coverage_duplicate_static", Field: key}
		}
		discoveredByKey[key] = surface
	}
	expectedByKey := make(map[string]EffectSurface, len(expected))
	for _, surface := range expectedRows {
		if surface.ASTCall == "classification" {
			continue
		}
		expectedByKey[surface.SeamID] = surface
		discoveredSurface, ok := discoveredByKey[surface.SeamID]
		if !ok {
			return &EffectCoverageError{Code: "effect_coverage_missing_static", Field: surface.SeamID}
		}
		if discoveredSurface.DescriptorID != surface.DescriptorID || discoveredSurface.EffectID != surface.EffectID || discoveredSurface.OwnerPackage != surface.OwnerPackage || discoveredSurface.CapabilityID != surface.CapabilityID || discoveredSurface.Boundary != surface.Boundary || discoveredSurface.OwnerSymbol != surface.OwnerSymbol || discoveredSurface.RegistrationSymbol != surface.RegistrationSymbol || discoveredSurface.GuardCall != surface.GuardCall || discoveredSurface.EffectCall != surface.EffectCall || discoveredSurface.ProfileDisposition != surface.ProfileDisposition || !equalEffectSelector(discoveredSurface.Configuration, surface.Configuration) {
			return &EffectCoverageError{Code: "effect_coverage_static_drift", Field: surface.SeamID}
		}
	}
	discoveredKeys := make([]string, 0, len(discoveredByKey))
	for key := range discoveredByKey {
		discoveredKeys = append(discoveredKeys, key)
	}
	sort.Strings(discoveredKeys)
	for _, key := range discoveredKeys {
		if _, ok := expectedByKey[key]; !ok {
			return &EffectCoverageError{Code: "effect_coverage_extra_static", Field: key}
		}
	}
	return nil
}

func joinRuntime(expected []EffectSurface, profile DeploymentProfile, runtime []EffectDescriptor, selection *EffectRuntimeConfiguration) error {
	_ = profile
	if selection == nil {
		return &EffectCoverageError{Code: "effect_coverage_configuration_required"}
	}
	provider := strings.TrimSpace(selection.WebSearchProvider)
	if provider != "tavily" && provider != "chain" && provider != "provider" {
		return &EffectCoverageError{Code: "effect_coverage_selection_drift", Field: "websearch.provider"}
	}
	expectedByDescriptor := make(map[string]EffectSurface, len(runtimeDescriptorSpecs))
	for _, surface := range expected {
		if surface.DescriptorID != "" {
			expectedByDescriptor[surface.DescriptorID] = surface
		}
	}
	seen := make(map[string]struct{}, len(runtime))
	runtimeRows := append([]EffectDescriptor(nil), runtime...)
	sort.SliceStable(runtimeRows, func(i, j int) bool { return runtimeRows[i].ID < runtimeRows[j].ID })
	for _, descriptor := range runtimeRows {
		if _, duplicate := seen[descriptor.ID]; duplicate {
			return &EffectCoverageError{Code: "effect_coverage_duplicate_runtime", Field: descriptor.ID}
		}
		seen[descriptor.ID] = struct{}{}
		spec, ok := runtimeDescriptorSpecs[descriptor.ID]
		if !ok {
			return &EffectCoverageError{Code: "effect_coverage_extra_runtime", Field: descriptor.ID}
		}
		if _, ok := expectedByDescriptor[descriptor.ID]; !ok {
			return &EffectCoverageError{Code: "effect_coverage_extra_runtime", Field: descriptor.ID}
		}
		if spec.Disposition == CommitmentExcluded {
			return &EffectCoverageError{Code: "effect_coverage_profile_drift", Field: descriptor.ID}
		}
		if descriptor.Owner != spec.Owner || descriptor.Boundary != spec.Boundary || descriptor.CapabilityID != string(spec.CapabilityID) {
			return &EffectCoverageError{Code: "effect_coverage_runtime_drift", Field: descriptor.ID}
		}
	}
	selectionDrift := make(map[string]struct{})
	for _, descriptorID := range sortedRuntimeDescriptorSpecIDs(runtimeDescriptorSpecs) {
		spec := runtimeDescriptorSpecs[descriptorID]
		if spec.Disposition == CommitmentExcluded {
			continue
		}
		selected, group := effectDescriptorSelected(spec, selection)
		_, present := seen[descriptorID]
		if spec.Configuration == nil {
			if !present {
				return &EffectCoverageError{Code: "effect_coverage_missing_runtime", Field: descriptorID}
			}
			continue
		}
		if selected != present {
			selectionDrift[group] = struct{}{}
		}
	}
	if len(selectionDrift) > 0 {
		groups := make([]string, 0, len(selectionDrift))
		for group := range selectionDrift {
			groups = append(groups, group)
		}
		sort.Strings(groups)
		return &EffectCoverageError{Code: "effect_coverage_selection_drift", Field: groups[0]}
	}
	return nil
}

func effectDescriptorSelected(spec runtimeDescriptorSpec, selection *EffectRuntimeConfiguration) (bool, string) {
	if spec.Configuration == nil {
		return true, ""
	}
	group := spec.Configuration.Group
	switch spec.Configuration.Mode {
	case effectSelectorOneOf:
		return spec.Configuration.Value == strings.TrimSpace(selection.WebSearchProvider), group
	case effectSelectorOptional:
		enabled := map[string]bool{
			"schedule.worker": selection.ScheduleWorkerEnabled,
			"channel.archive": selection.ChannelArchiveEnabled,
			"relay.batch":     selection.RelayBatchEnabled,
			"relay.runtime":   selection.RelayRuntimeEnabled,
			"chat.fallback":   selection.ChatFallbackEnabled,
		}
		selected, known := enabled[group]
		return known && selected, group
	default:
		return false, group
	}
}

func validBoundary(boundary Boundary) bool {
	switch boundary {
	case BoundaryHTTP, BoundaryGRPC, BoundaryWorkerClaim, BoundaryWorkerEffect, BoundaryOutbound, BoundaryFinancial, BoundaryOperation:
		return true
	default:
		return false
	}
}

var _ EffectRegistrar = (*EffectRegistry)(nil)
