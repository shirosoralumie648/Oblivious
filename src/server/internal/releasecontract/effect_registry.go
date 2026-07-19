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

type runtimeDescriptorSpec struct {
	EffectID           EffectID
	CapabilityID       CapabilityID
	Owner              string
	Boundary           Boundary
	OwnerPackage       string
	RegistrationSymbol string
	RegistrationAnchor string
	GuardCall          string
	EffectCall         string
	Configuration      *EffectConfigurationSelector
	Disposition        Commitment
}

func descriptorSpec(effect EffectID, owner string, boundary Boundary, ownerPackage, registrationSymbol, registrationAnchor, guardCall, effectCall string, disposition Commitment, configuration *EffectConfigurationSelector) runtimeDescriptorSpec {
	return runtimeDescriptorSpec{
		EffectID: effect, CapabilityID: authoredEffectCapabilities[effect], Owner: owner, Boundary: boundary,
		OwnerPackage: ownerPackage, RegistrationSymbol: registrationSymbol, RegistrationAnchor: registrationAnchor,
		GuardCall: guardCall, EffectCall: effectCall, Configuration: configuration, Disposition: disposition,
	}
}

func selector(group, value, mode string) *EffectConfigurationSelector {
	return &EffectConfigurationSelector{Group: group, Value: value, Mode: mode}
}

// runtimeDescriptorSpecs is the sole runtime descriptor allowlist and the
// sole descriptor-to-EffectID mapping. Source discovery and runtime joining
// consume the same exact owner, boundary, capability, and selection contract.
var runtimeDescriptorSpecs = map[string]runtimeDescriptorSpec{
	"worker.schedule.claim":             descriptorSpec(EffectScheduleClaim, "schedule.Worker", BoundaryWorkerClaim, "src/server/internal/schedule", "newScheduledReadiness", "", "runDueTasks:requireClaim", "runDueTasks:ClaimDueScheduledTaskRuns", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.workflow.start":    descriptorSpec(EffectScheduleWorkflow, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "runClaimedWorkflowTask:requireWorkflowEffect", "runClaimedWorkflowTask:startClaimedWorkflow", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.workflow.continue": descriptorSpec(EffectScheduleWorkflow, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "runClaimedWorkflowTask:requireWorkflowEffect", "runClaimedWorkflowTask:RunExecutionUntilBlocked", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),
	"worker.schedule.agent.start":       descriptorSpec(EffectScheduleAgent, "schedule.Service", BoundaryWorkerEffect, "src/server/internal/schedule", "newScheduledReadiness", "", "runClaimedAgentTask:requireAgentEffect", "runClaimedAgentTask:StartRun", CommitmentCommitted, selector("schedule.worker", "enabled", effectSelectorOptional)),

	"worker.channel_retry.claim": descriptorSpec(EffectChannelRetryClaim, "channel.Service", BoundaryWorkerClaim, "src/server/internal/channel", "newChannelReadiness", "", "ClaimDueRetryMessages:requireRetryClaim", "ClaimDueRetryMessages:ClaimDueRetryMessages", CommitmentCommitted, nil),
	"channel.delivery.send":      descriptorSpec(EffectChannelDelivery, "channel.Service", BoundaryOutbound, "src/server/internal/channel", "newChannelReadiness", "", "Send:requireDelivery", "Send:DeliverOutbound", CommitmentCommitted, nil),
	"worker.archive.claim":       descriptorSpec(EffectArchiveClaim, "channel.ArchiveWorker", BoundaryWorkerClaim, "src/server/internal/channel", "newArchiveReadiness", "", "archiveExpiredMessageLogs:requireClaim", "archiveExpiredMessageLogs:ListExpiredMessageLogsForArchive", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),
	"worker.archive.write":       descriptorSpec(EffectArchiveWrite, "channel.Service", BoundaryWorkerEffect, "src/server/internal/channel", "newArchiveReadiness", "", "archiveExpiredMessageLogs:requireWrite", "archiveExpiredMessageLogs:ArchiveMessageLogs", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),
	"worker.archive.delete":      descriptorSpec(EffectArchiveDelete, "channel.Service", BoundaryWorkerEffect, "src/server/internal/channel", "newArchiveReadiness", "", "archiveExpiredMessageLogs:requireDelete", "archiveExpiredMessageLogs:DeleteArchivedMessageLogs", CommitmentCommitted, selector("channel.archive", "enabled", effectSelectorOptional)),

	"worker.relay_batch.claim":             descriptorSpec(EffectRelayBatchClaim, "relay.BatchPollingWorker", BoundaryWorkerClaim, "src/server/internal/relay", "newBatchPollingReadiness", "", "runOnce:requireClaim", "runOnce:ClaimBatchPollingJobs", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.retrieve":          descriptorSpec(EffectRelayBatchProvider, "relay.BatchPollingWorker", BoundaryOutbound, "src/server/internal/relay", "newBatchPollingReadiness", "", "runOnce:requireProvider", "runOnce:RetrieveBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.complete.finalize": descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "recordBatchStatus:requireFinalizer", "recordBatchStatus:FinalizeCompletedBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.failure.finalize":  descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "recordBatchStatus:requireFinalizer", "recordBatchStatus:FinalizeFailedBatch", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.succeeded":         descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "recordBatchStatus:requireTerminal", "recordBatchStatus:MarkBatchPollingJobSucceeded", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),
	"worker.relay_batch.dead_letter":       descriptorSpec(EffectRelayBatchFinalize, "relay.BatchPollingWorker", BoundaryWorkerEffect, "src/server/internal/relay", "newBatchPollingReadiness", "", "recordBatchStatus:requireTerminal", "recordBatchStatus>recordTerminalFailedJob:MarkBatchPollingJobDeadLetter", CommitmentCommitted, selector("relay.batch", "enabled", effectSelectorOptional)),

	"marketplace.settlement.intent": descriptorSpec(EffectMarketplaceSettlement, "marketplace.SettlementService", BoundaryFinancial, "src/server/internal/marketplace", "WithMarketplaceFinancialReadiness", "", "CreatePaidInstallCheckout:requireSettlement", "CreatePaidInstallCheckout:BeginTx", CommitmentCommitted, nil),
	"marketplace.payout.dispatch":   descriptorSpec(EffectMarketplacePayout, "marketplace.SettlementService", BoundaryFinancial, "src/server/internal/marketplace", "WithMarketplaceFinancialReadiness", "", "dispatchPersistedPayout:requirePayout", "dispatchPersistedPayout>dispatchPayout:CreatePayout", CommitmentCommitted, nil),
	"relay.provider.dispatch":       descriptorSpec(EffectRelayProvider, "relay.Router", BoundaryOutbound, "src/server/internal/relay", "newRouterReadiness", "", "Route:requireProvider", "Route:fn", CommitmentCommitted, selector("relay.runtime", "enabled", effectSelectorOptional)),
	"chat.provider.dispatch":        descriptorSpec(EffectChatProvider, "chat.RelayGateway", BoundaryOutbound, "src/server/internal/chat", "NewRelayGatewayWithOptions>newRelayGatewayReadiness", "", "complete:requireDispatch", "complete:Do", CommitmentCommitted, nil),
	"chat.provider.fallback":        descriptorSpec(EffectChatProvider, "chat.CompositeGateway", BoundaryOutbound, "src/server/internal/chat", "NewCompositeGatewayWithOptions>newRelayGatewayReadiness", "", "GenerateReply:requireDispatch", "GenerateReply:GenerateReply", CommitmentCommitted, selector("chat.fallback", "enabled", effectSelectorOptional)),

	"admin.channel.model.mutation": descriptorSpec(EffectHTTPMutation, "admin.channel_service", BoundaryHTTP, "src/server/internal/admin", "newModelCatalogReadiness", "", "SyncChannelModels:requireMutation", "SyncChannelModels:TestChannel", CommitmentCommitted, nil),
	"http.admin.refund":            descriptorSpec(EffectAdminRefund, "http.adminHandler", BoundaryFinancial, "src/server/internal/http", "newAdminFinancialReadiness", "", "recordTopupRefund:require", "recordTopupRefund:RecordTopupRefund", CommitmentCommitted, nil),
	"http.billing.checkout":        descriptorSpec(EffectBillingCheckout, "http.billingHandler", BoundaryFinancial, "src/server/internal/http", "newBillingFinancialReadiness>newCheckoutFinancialReadiness", "", "checkout:require", "checkout:CreateCheckoutSession", CommitmentCommitted, nil),
	"http.marketplace.checkout":    descriptorSpec(EffectBillingCheckout, "http.marketplaceHandler", BoundaryFinancial, "src/server/internal/http", "withMarketplaceFinancialReadiness>newCheckoutFinancialReadiness", "", "createPaidInstallCheckout:require", "createPaidInstallCheckout:CreateCheckoutSession", CommitmentCommitted, nil),
	"http.mcp.mutation":            descriptorSpec(EffectHTTPMutation, "http.mcpHandler", BoundaryHTTP, "src/server/internal/http", "newMCPMutationReadiness", "", "addServer:authorize", "addServer:AddServer", CommitmentCommitted, nil),

	"mcp.transport.dispatch": descriptorSpec(EffectMCPDispatch, "mcp.Client", BoundaryOutbound, "src/server/internal/mcp", "newClientReadiness", "", "sendRequest:authorize", "sendRequest:Do", CommitmentCommitted, nil),
	"mcp.websearch.builtin":  descriptorSpec(EffectToolWebSearch, "mcp.WebSearchTool", BoundaryOutbound, "src/server/internal/mcp", "NewWebSearchTool>newWebSearchReadiness", "", "Execute:authorize", "Execute:Search", CommitmentConditional, nil),
	"mcp.websearch.tavily":   descriptorSpec(EffectToolWebSearch, "mcp.TavilyWebSearchProvider", BoundaryOutbound, "src/server/internal/mcp", "newTavilyWebSearchProvider>newWebSearchReadiness", "", "Search:authorize", "Search:Do", CommitmentConditional, selector("websearch.provider", "tavily", effectSelectorOneOf)),
	"mcp.websearch.chain":    descriptorSpec(EffectToolWebSearch, "mcp.websearch.Chain", BoundaryOutbound, "src/server/internal/mcp/websearch", "NewProviderFromConfig>newSearchReadiness", "", "Search:authorize", "Search:Search", CommitmentConditional, selector("websearch.provider", "chain", effectSelectorOneOf)),
	"mcp.websearch.provider": descriptorSpec(EffectToolWebSearch, "mcp.websearch.Provider", BoundaryOutbound, "src/server/internal/mcp/websearch", "NewProviderFromConfig>newSearchReadiness", "", "Search:authorize", "Search:Search", CommitmentConditional, selector("websearch.provider", "provider", effectSelectorOneOf)),

	"agent.tool.builtin":         descriptorSpec(EffectAgentToolBuiltin, "agent.ToolExecutor.builtin", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolBuiltin", "executeBuiltin:authorizeTool", "executeBuiltin:Execute", CommitmentCommitted, nil),
	"agent.tool.custom_api_http": descriptorSpec(EffectAgentToolCustomAPIHTTP, "agent.ToolExecutor.custom_api_http", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolCustomAPIHTTP", "executeCustomAPI:authorizeTool", "executeCustomAPI:Do", CommitmentConditional, nil),
	"agent.tool.local_python":    descriptorSpec(EffectAgentToolLocalPython, "agent.ToolExecutor.local_python", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolLocalPython", "executeCustomPython:authorizeTool", "executeCustomPython:pythonProcessRunner", CommitmentConditional, nil),
	"agent.tool.python_sandbox":  descriptorSpec(EffectAgentToolPythonSandbox, "agent.ToolExecutor.python_sandbox", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolPythonSandbox", "executeCustomPythonSandbox:authorizeTool", "executeCustomPythonSandbox:RunCustomPython", CommitmentExcluded, nil),
	"agent.tool.mcp":             descriptorSpec(EffectAgentToolMCP, "agent.ToolExecutor.mcp", BoundaryOutbound, "src/server/internal/agent", "NewAuthorizedToolExecutor", "EffectAgentToolMCP", "executeMCP:authorizeTool", "executeMCP:CallTool", CommitmentCommitted, nil),

	"agent.tool.registry.builtin": descriptorSpec(EffectAgentToolBuiltin, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryBuiltin", "Execute:authorize", "Execute:executor", CommitmentExcluded, nil),
	"agent.tool.registry.custom":  descriptorSpec(EffectAgentToolCustomAPIHTTP, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryCustom", "Execute:authorize", "Execute:executor", CommitmentExcluded, nil),
	"agent.tool.registry.mcp":     descriptorSpec(EffectAgentToolMCP, "agent.tools.Registry", BoundaryOutbound, "src/server/internal/agent/tools", "newRegistryReadiness", "CategoryMCP", "Execute:authorize", "Execute:executor", CommitmentExcluded, nil),
	"agent.tool.web_search":       descriptorSpec(EffectToolWebSearch, "agent.tools.WebsearchTool", BoundaryOutbound, "src/server/internal/agent/tools", "newWebsearchReadiness", "", "Execute:authorize", "Execute:Search", CommitmentExcluded, nil),
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
	seen := make(map[string]struct{}, len(manifest.Surfaces))
	descriptors := make(map[string]struct{}, len(specs))
	for _, surface := range manifest.Surfaces {
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
	for descriptorID := range specs {
		if _, ok := descriptors[descriptorID]; !ok {
			return &EffectCoverageError{Code: "effect_manifest_missing_descriptor", Field: descriptorID}
		}
	}
	return nil
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
	descriptorIDs := make([]string, 0, len(runtimeDescriptorSpecs))
	for descriptorID := range runtimeDescriptorSpecs {
		descriptorIDs = append(descriptorIDs, descriptorID)
	}
	sort.Strings(descriptorIDs)
	for _, descriptorID := range descriptorIDs {
		spec := runtimeDescriptorSpecs[descriptorID]
		present, err := descriptorStructurePresent(repoRoot, descriptorID, spec)
		if err != nil {
			return nil, &EffectCoverageError{Code: "effect_discovery_failed", Field: descriptorID, Err: err}
		}
		if present {
			discovered = append(discovered, discoveredEffect{EffectSurface: effectSurfaceForSpec(descriptorID, spec), File: spec.OwnerPackage})
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
	calls       []sourceCall
}

type sourceCall struct {
	name     string
	position token.Pos
}

func descriptorStructurePresent(repoRoot, descriptorID string, spec runtimeDescriptorSpec) (bool, error) {
	functions, err := loadSourceFunctions(filepath.Join(repoRoot, filepath.FromSlash(spec.OwnerPackage)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	anchor := spec.RegistrationAnchor
	if anchor == "" {
		anchor = descriptorID
	}
	effectSymbol, err := effectConstantSymbol(repoRoot, spec.EffectID)
	if err != nil {
		return false, err
	}
	if effectSymbol == "" || !registrationContractPresent(functions, spec.RegistrationSymbol, descriptorID, anchor, effectSymbol, spec.Owner, spec.Boundary) {
		return false, nil
	}
	return guardEffectContractPresent(functions, spec.GuardCall, spec.EffectCall), nil
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
			indexed := sourceFunction{declaration: function}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if ok {
					indexed.calls = append(indexed.calls, sourceCall{name: calledName(call.Fun), position: call.Pos()})
				}
				return true
			})
			functions[function.Name.Name] = append(functions[function.Name.Name], indexed)
		}
	}
	return functions, nil
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

func registrationContractPresent(functions map[string][]sourceFunction, contract, descriptorID, anchor, effectSymbol, owner string, boundary Boundary) bool {
	path := strings.Split(contract, ">")
	if len(path) == 0 {
		return false
	}
	for _, first := range functions[path[0]] {
		next := ""
		if len(path) > 1 {
			next = path[1]
		}
		if !functionHasRegistrationIdentity(first.declaration, descriptorID, anchor, owner, next) {
			continue
		}
		current := first
		valid := true
		for index := 1; index < len(path); index++ {
			if !hasCallAfter(current, path[index], 0) || len(functions[path[index]]) == 0 {
				valid = false
				break
			}
			current = functions[path[index]][0]
		}
		if valid && hasCallAfter(current, "Register", 0) && functionHasBoundary(current.declaration, boundary) && (functionHasEffectBinding(first.declaration, effectSymbol) || functionHasEffectBinding(current.declaration, effectSymbol)) {
			return true
		}
	}
	return false
}

func functionHasRegistrationIdentity(function *ast.FuncDecl, descriptorID, anchor, owner, next string) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if found {
			return false
		}
		if next != "" {
			call, ok := node.(*ast.CallExpr)
			if !ok || calledName(call.Fun) != next {
				return true
			}
			found = syntaxListContainsAnchor(call.Args, descriptorID) && syntaxListContainsAnchor(call.Args, owner)
			return !found
		}
		literal, ok := node.(*ast.CompositeLit)
		if ok && syntaxContainsAnchor(literal, anchor) && syntaxContainsAnchor(literal, owner) {
			found = true
		}
		return !found
	})
	if found {
		return true
	}
	return next == "" && functionHasCompositeAnchor(function, anchor) && functionHasCompositeAnchor(function, owner)
}

func functionHasCompositeAnchor(function *ast.FuncDecl, anchor string) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if found {
			return false
		}
		literal, ok := node.(*ast.CompositeLit)
		found = ok && syntaxContainsAnchor(literal, anchor)
		return !found
	})
	return found
}

func syntaxListContainsAnchor(nodes []ast.Expr, anchor string) bool {
	for _, node := range nodes {
		if syntaxContainsAnchor(node, anchor) {
			return true
		}
	}
	return false
}

func functionHasEffectBinding(function *ast.FuncDecl, effectSymbol string) bool {
	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		if found {
			return false
		}
		if call, ok := node.(*ast.CallExpr); ok && calledName(call.Fun) == "Resolve" && syntaxListContainsAnchor(call.Args, effectSymbol) {
			found = true
			return false
		}
		if literal, ok := node.(*ast.CompositeLit); ok && syntaxContainsAnchor(literal, effectSymbol) {
			found = true
			return false
		}
		return true
	})
	return found
}

func functionHasBoundary(function *ast.FuncDecl, boundary Boundary) bool {
	identifier := boundarySourceIdentifier(boundary)
	if identifier == "" {
		return false
	}
	return functionHasCompositeAnchor(function, identifier)
}

func boundarySourceIdentifier(boundary Boundary) string {
	switch boundary {
	case BoundaryHTTP:
		return "BoundaryHTTP"
	case BoundaryGRPC:
		return "BoundaryGRPC"
	case BoundaryWorkerClaim:
		return "BoundaryWorkerClaim"
	case BoundaryWorkerEffect:
		return "BoundaryWorkerEffect"
	case BoundaryOutbound:
		return "BoundaryOutbound"
	case BoundaryFinancial:
		return "BoundaryFinancial"
	case BoundaryOperation:
		return "BoundaryOperation"
	default:
		return ""
	}
}

func effectConstantSymbol(repoRoot string, effect EffectID) (string, error) {
	path := filepath.Join(repoRoot, "src", "server", "internal", "releasecontract", "catalog.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
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
				literal, ok := expression.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING || index >= len(valueSpec.Names) {
					continue
				}
				value, err := strconv.Unquote(literal.Value)
				if err == nil && EffectID(value) == effect {
					return valueSpec.Names[index].Name, nil
				}
			}
		}
	}
	return "", nil
}

func syntaxContainsAnchor(root ast.Node, anchor string) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if found {
			return false
		}
		switch value := node.(type) {
		case *ast.BasicLit:
			if value.Kind == token.STRING {
				literal, err := strconv.Unquote(value.Value)
				found = err == nil && literal == anchor
			}
		case *ast.Ident:
			found = value.Name == anchor
		case *ast.SelectorExpr:
			found = value.Sel != nil && value.Sel.Name == anchor
		}
		return !found
	})
	return found
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
		guardPosition := firstCallPosition(root, guardCall)
		if guardPosition == 0 {
			continue
		}
		if len(effectPath) == 1 {
			if firstCallPositionAfter(root, effectCall, guardPosition) != 0 {
				return true
			}
			continue
		}
		if firstCallPositionAfter(root, effectPath[1], guardPosition) == 0 {
			continue
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
		if call.name == name && call.position > after {
			return call.position
		}
	}
	return 0
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
	if err := joinRuntime(options.manifest.Surfaces, options.profile, runtime); err != nil {
		return err
	}
	for _, descriptor := range runtime {
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
}

func (o EffectCoverageOptions) options() effectCoverageOptions {
	return effectCoverageOptions{repoRoot: o.RepoRoot, manifestPath: o.ManifestPath, manifest: o.Manifest, expected: o.Expected, runtime: o.Runtime, registry: o.Registry, contract: o.Contract, profile: o.Profile, authorities: o.Authorities}
}

func joinStatic(expected, discovered []EffectSurface) error {
	discoveredByKey := make(map[string]EffectSurface, len(discovered))
	for _, surface := range discovered {
		key := surface.SeamID
		if _, duplicate := discoveredByKey[key]; duplicate {
			return &EffectCoverageError{Code: "effect_coverage_duplicate_static", Field: key}
		}
		discoveredByKey[key] = surface
	}
	expectedByKey := make(map[string]EffectSurface, len(expected))
	for _, surface := range expected {
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
	for key := range discoveredByKey {
		if _, ok := expectedByKey[key]; !ok {
			return &EffectCoverageError{Code: "effect_coverage_extra_static", Field: key}
		}
	}
	return nil
}

func joinRuntime(expected []EffectSurface, profile DeploymentProfile, runtime []EffectDescriptor) error {
	_ = profile
	expectedByDescriptor := make(map[string]EffectSurface, len(runtimeDescriptorSpecs))
	for _, surface := range expected {
		if surface.DescriptorID != "" {
			expectedByDescriptor[surface.DescriptorID] = surface
		}
	}
	seen := make(map[string]struct{}, len(runtime))
	for _, descriptor := range runtime {
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
	selectorGroups := make(map[string][]string)
	selectorModes := make(map[string]string)
	for descriptorID, spec := range runtimeDescriptorSpecs {
		if spec.Disposition == CommitmentExcluded {
			continue
		}
		if spec.Configuration == nil {
			if _, ok := seen[descriptorID]; !ok {
				return &EffectCoverageError{Code: "effect_coverage_missing_runtime", Field: descriptorID}
			}
			continue
		}
		group := spec.Configuration.Group + "\x00" + spec.Configuration.Value
		selectorGroups[group] = append(selectorGroups[group], descriptorID)
		selectorModes[spec.Configuration.Group] = spec.Configuration.Mode
	}
	for groupValue, descriptors := range selectorGroups {
		parts := strings.SplitN(groupValue, "\x00", 2)
		mode := selectorModes[parts[0]]
		present := 0
		for _, descriptorID := range descriptors {
			if _, ok := seen[descriptorID]; ok {
				present++
			}
		}
		if mode == effectSelectorOptional && present != 0 && present != len(descriptors) {
			return &EffectCoverageError{Code: "effect_coverage_selection_drift", Field: parts[0]}
		}
	}
	oneOfSelections := make(map[string]int)
	for descriptorID, spec := range runtimeDescriptorSpecs {
		if spec.Configuration != nil && spec.Configuration.Mode == effectSelectorOneOf {
			if _, ok := seen[descriptorID]; ok {
				oneOfSelections[spec.Configuration.Group]++
			}
		}
	}
	for group, mode := range selectorModes {
		if mode == effectSelectorOneOf && oneOfSelections[group] != 1 {
			return &EffectCoverageError{Code: "effect_coverage_selection_drift", Field: group}
		}
	}
	return nil
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
