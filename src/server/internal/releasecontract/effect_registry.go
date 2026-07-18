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
	SeamID             string     `json:"seamId"`
	OwnerPackage       string     `json:"ownerPackage"`
	OwnerSymbol        string     `json:"ownerSymbol"`
	CapabilityID       string     `json:"capabilityId"`
	Boundary           Boundary   `json:"boundary"`
	ASTCall            string     `json:"astCall"`
	EntrypointID       string     `json:"entrypointId,omitempty"`
	ProfileDisposition Commitment `json:"profileDisposition"`
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
	if !isKnownCapability(descriptor.CapabilityID) {
		return &EffectCoverageError{Code: "effect_registry_unknown_capability", Field: descriptor.CapabilityID}
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
	if manifest.SchemaVersion != EffectSurfaceSchemaV1 || len(manifest.Surfaces) == 0 {
		return &EffectCoverageError{Code: "effect_manifest_empty"}
	}
	seen := make(map[string]struct{}, len(manifest.Surfaces))
	for _, surface := range manifest.Surfaces {
		if strings.TrimSpace(surface.SeamID) == "" || strings.TrimSpace(surface.OwnerPackage) == "" || strings.TrimSpace(surface.OwnerSymbol) == "" || strings.TrimSpace(surface.CapabilityID) == "" || !validBoundary(surface.Boundary) || !validCommitment(surface.ProfileDisposition) {
			return &EffectCoverageError{Code: "effect_manifest_invalid", Field: surface.SeamID}
		}
		if _, duplicate := seen[surface.SeamID]; duplicate {
			return &EffectCoverageError{Code: "effect_manifest_duplicate", Field: surface.SeamID}
		}
		seen[surface.SeamID] = struct{}{}
	}
	return nil
}

type discoveredEffect struct {
	EffectSurface
	File string `json:"file"`
	Line int    `json:"line"`
}

// DiscoverEffectSurfaces parses production Go source and extracts each
// EffectDescriptor composite literal. No environment or executable identity
// participates in discovery.
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
			literal, ok := node.(*ast.CompositeLit)
			if !ok || (!isEffectDescriptorType(literal.Type) && !looksLikeEffectDescriptor(literal)) {
				return true
			}
			values := literalEffectFields(literal)
			id, _ := values["ID"].(string)
			capability, _ := values["CapabilityID"].(string)
			boundary, _ := values["Boundary"].(string)
			owner, _ := values["Owner"].(string)
			if id == "" || owner == "" {
				return true
			}
			if capability == "" {
				capability = knownCapabilityForEffect(id)
			}
			boundary = normalizeBoundary(boundary)
			if boundary == "" || capability == "" {
				return true
			}
			position := fset.Position(literal.Pos())
			relative, _ := filepath.Rel(repoRoot, path)
			ownerPackage := filepath.ToSlash(filepath.Dir(relative))
			discovered = append(discovered, discoveredEffect{EffectSurface: EffectSurface{
				SeamID: id + "@" + owner, OwnerPackage: ownerPackage, OwnerSymbol: owner,
				CapabilityID: capability, Boundary: Boundary(boundary), ASTCall: "EffectDescriptor",
			}, File: relative, Line: position.Line})
			return true
		})
		return nil
	})
	if err != nil {
		return nil, &EffectCoverageError{Code: "effect_discovery_failed", Err: err}
	}
	appendExecutorEffectSurfaces(repoRoot, &discovered)
	sort.Slice(discovered, func(i, j int) bool {
		if discovered[i].SeamID != discovered[j].SeamID {
			return discovered[i].SeamID < discovered[j].SeamID
		}
		return discovered[i].File+strconv.Itoa(discovered[i].Line) < discovered[j].File+strconv.Itoa(discovered[j].Line)
	})
	result := make([]EffectSurface, 0, len(discovered))
	for _, value := range discovered {
		result = append(result, value.EffectSurface)
	}
	return result, nil
}

func appendExecutorEffectSurfaces(repoRoot string, discovered *[]discoveredEffect) {
	executorPath := filepath.Join(repoRoot, "src", "server", "internal", "agent", "executor.go")
	content, err := os.ReadFile(executorPath)
	if err == nil {
		for _, item := range []struct {
			marker string
			id     string
			owner  string
			cap    string
		}{
			{"\"builtin\"", "agent.tool.builtin", "agent.ToolExecutor.builtin", "mcp.tool_execution"},
			{"\"custom_api_http\"", "agent.tool.custom_api_http", "agent.ToolExecutor.custom_api_http", "mcp.custom_execution"},
			{"\"custom_python_process\"", "agent.tool.local_python", "agent.ToolExecutor.local_python", "mcp.custom_execution"},
			{"\"custom_python_sandbox\"", "agent.tool.python_sandbox", "agent.ToolExecutor.python_sandbox", "sandbox.code_execution"},
			{"\"mcp\"", "agent.tool.mcp", "agent.ToolExecutor.mcp", "mcp.tool_execution"},
		} {
			if strings.Contains(string(content), item.marker) {
				*discovered = append(*discovered, discoveredEffect{EffectSurface: EffectSurface{
					SeamID: item.id + "@" + item.owner, OwnerPackage: "src/server/internal/agent", OwnerSymbol: item.owner,
					CapabilityID: item.cap, Boundary: BoundaryOutbound, ASTCall: "ToolExecutor",
				}})
			}
		}
	}
	registryPath := filepath.Join(repoRoot, "src", "server", "internal", "agent", "tools", "registry.go")
	content, err = os.ReadFile(registryPath)
	if err == nil {
		for _, item := range []struct {
			marker string
			id     string
			cap    string
		}{
			{"CategoryBuiltin", "agent.tool.registry.builtin", "mcp.tool_execution"},
			{"CategoryCustom", "agent.tool.registry.custom", "mcp.custom_execution"},
			{"CategoryMCP", "agent.tool.registry.mcp", "mcp.tool_execution"},
		} {
			if strings.Contains(string(content), item.marker) {
				*discovered = append(*discovered, discoveredEffect{EffectSurface: EffectSurface{
					SeamID: item.id + "@agent.tools.Registry", OwnerPackage: "src/server/internal/agent/tools", OwnerSymbol: "agent.tools.Registry",
					CapabilityID: item.cap, Boundary: BoundaryOutbound, ASTCall: "Registry",
				}})
			}
		}
	}
}

func isEffectDescriptorType(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "EffectDescriptor"
	case *ast.SelectorExpr:
		return value.Sel != nil && value.Sel.Name == "EffectDescriptor"
	default:
		return false
	}
}

func looksLikeEffectDescriptor(literal *ast.CompositeLit) bool {
	seen := map[string]bool{}
	for _, element := range literal.Elts {
		if keyValue, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := keyValue.Key.(*ast.Ident); ok {
				seen[key.Name] = true
			}
		}
	}
	return seen["ID"] && seen["CapabilityID"] && seen["Boundary"] && seen["Owner"]
}

func literalEffectFields(literal *ast.CompositeLit) map[string]any {
	result := make(map[string]any)
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if stringLiteral, ok := keyValue.Value.(*ast.BasicLit); ok && stringLiteral.Kind == token.STRING {
			if value, err := strconv.Unquote(stringLiteral.Value); err == nil {
				result[key.Name] = value
			}
		}
		if selector, ok := keyValue.Value.(*ast.SelectorExpr); ok {
			result[key.Name] = selector.Sel.Name
		}
	}
	return result
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
	if err := joinRuntime(options.manifest.Surfaces, options.profile, runtime); err != nil {
		return err
	}
	if len(runtime) == 0 {
		return &EffectCoverageError{Code: "effect_coverage_zero_runtime"}
	}
	if options.authorities.Valid() {
		for _, descriptor := range runtime {
			effectID := effectIDForDescriptor(descriptor)
			capability, err := options.authorities.CapabilityBindings.Resolve(effectID)
			if err != nil {
				return &EffectCoverageError{Code: "effect_coverage_unknown_effect", Field: descriptor.ID}
			}
			if string(capability) != descriptor.CapabilityID {
				return &EffectCoverageError{Code: "effect_coverage_capability_mismatch", Field: descriptor.ID}
			}
		}
	}
	return nil
}

func effectIDForDescriptor(descriptor EffectDescriptor) EffectID {
	switch descriptor.ID {
	case "worker.schedule.claim":
		return EffectScheduleClaim
	case "worker.schedule.workflow.start", "worker.schedule.workflow.continue":
		return EffectScheduleWorkflow
	case "worker.schedule.agent.start":
		return EffectScheduleAgent
	case "worker.channel_retry.claim":
		return EffectChannelRetryClaim
	case "channel.delivery.send":
		return EffectChannelDelivery
	case "worker.relay_batch.claim":
		return EffectRelayBatchClaim
	case "worker.relay_batch.retrieve":
		return EffectRelayBatchProvider
	case "worker.relay_batch.complete.finalize", "worker.relay_batch.failure.finalize", "worker.relay_batch.succeeded", "worker.relay_batch.dead_letter":
		return EffectRelayBatchFinalize
	case "worker.archive.claim":
		return EffectArchiveClaim
	case "worker.archive.write":
		return EffectArchiveWrite
	case "worker.archive.delete":
		return EffectArchiveDelete
	case "relay.provider.dispatch":
		return EffectRelayProvider
	case "mcp.transport.dispatch":
		return EffectMCPDispatch
	case "http.mcp.mutation":
		return EffectHTTPMutation
	case "http.admin.refund":
		return EffectAdminRefund
	case "marketplace.payout.dispatch":
		return EffectMarketplacePayout
	case "marketplace.settlement.intent":
		return EffectMarketplaceSettlement
	case "agent.tool.builtin":
		return EffectAgentToolBuiltin
	case "agent.tool.custom_api_http":
		return EffectAgentToolCustomAPIHTTP
	case "agent.tool.local_python":
		return EffectAgentToolLocalPython
	case "agent.tool.python_sandbox":
		return EffectAgentToolPythonSandbox
	case "agent.tool.mcp":
		return EffectAgentToolMCP
	}
	switch {
	case strings.Contains(descriptor.Owner, "Chat"), strings.Contains(descriptor.Owner, "chat"):
		return EffectChatProvider
	case strings.Contains(descriptor.Owner, "WebSearch"), strings.Contains(descriptor.Owner, "websearch"), strings.Contains(descriptor.Owner, "Websearch"):
		return EffectToolWebSearch
	case strings.Contains(descriptor.Owner, "Billing"), strings.Contains(descriptor.Owner, "billing"):
		return EffectBillingCheckout
	case strings.Contains(descriptor.Owner, "Registry"):
		switch descriptor.CapabilityID {
		case "mcp.custom_execution":
			return EffectAgentToolCustomAPIHTTP
		default:
			return EffectAgentToolMCP
		}
	}
	return EffectID(descriptor.ID)
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
		if surface.ASTCall == "RunEntrypoint" || surface.ASTCall == "runtime" {
			continue
		}
		expectedByKey[surface.SeamID] = surface
		discoveredSurface, ok := discoveredByKey[surface.SeamID]
		if !ok {
			return &EffectCoverageError{Code: "effect_coverage_missing_static", Field: surface.SeamID}
		}
		if discoveredSurface.CapabilityID != surface.CapabilityID || discoveredSurface.Boundary != surface.Boundary || discoveredSurface.OwnerSymbol != surface.OwnerSymbol {
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
	seen := make(map[string]struct{}, len(runtime))
	matchedExpected := make(map[string]struct{}, len(expected))
	for _, descriptor := range runtime {
		if _, duplicate := seen[descriptor.ID]; duplicate {
			return &EffectCoverageError{Code: "effect_coverage_duplicate_runtime", Field: descriptor.ID}
		}
		seen[descriptor.ID] = struct{}{}
		if !validBoundary(descriptor.Boundary) || !isKnownCapability(descriptor.CapabilityID) {
			return &EffectCoverageError{Code: "effect_coverage_unclassified", Field: descriptor.ID}
		}
		matched := false
		for _, surface := range expected {
			if strings.HasSuffix(surface.SeamID, "@"+descriptor.Owner) && strings.HasPrefix(surface.SeamID, descriptor.ID+"@") {
				matched = true
				matchedExpected[surface.SeamID] = struct{}{}
				if surface.ProfileDisposition == CommitmentExcluded && (profile.ID == "monolith" || profile.ID == "") {
					return &EffectCoverageError{Code: "effect_coverage_profile_drift", Field: descriptor.ID}
				}
				break
			}
		}
		if !matched {
			candidates := make([]EffectSurface, 0, 1)
			for _, surface := range expected {
				if (strings.HasSuffix(surface.SeamID, "@"+descriptor.Owner) || surface.SeamID == descriptor.Owner) && surface.ASTCall == "runtime" {
					if !containsKey(matchedExpected, surface.SeamID) {
						candidates = append(candidates, surface)
					}
				}
			}
			if len(candidates) == 1 {
				surface := candidates[0]
				matched = true
				matchedExpected[surface.SeamID] = struct{}{}
				if surface.ProfileDisposition == CommitmentExcluded && (profile.ID == "monolith" || profile.ID == "") {
					return &EffectCoverageError{Code: "effect_coverage_profile_drift", Field: descriptor.ID}
				}
			}
		}
		if !matched {
			for _, surface := range expected {
				if strings.HasSuffix(surface.SeamID, "@"+descriptor.Owner) && strings.Contains(surface.SeamID, descriptor.ID) {
					matched = true
					matchedExpected[surface.SeamID] = struct{}{}
					break
				}
			}
		}
		if !matched {
			return &EffectCoverageError{Code: "effect_coverage_extra_runtime", Field: descriptor.ID}
		}
	}
	for _, surface := range expected {
		if surface.ProfileDisposition == CommitmentExcluded && (profile.ID == "monolith" || profile.ID == "") {
			continue
		}
		if strings.HasPrefix(surface.ASTCall, "runtime") && !containsKey(matchedExpected, surface.SeamID) {
			return &EffectCoverageError{Code: "effect_coverage_missing_runtime", Field: surface.SeamID}
		}
	}
	return nil
}

func containsKey(values map[string]struct{}, key string) bool {
	_, ok := values[key]
	return ok
}

func validBoundary(boundary Boundary) bool {
	switch boundary {
	case BoundaryHTTP, BoundaryGRPC, BoundaryWorkerClaim, BoundaryWorkerEffect, BoundaryOutbound, BoundaryFinancial, BoundaryOperation:
		return true
	default:
		return false
	}
}

func normalizeBoundary(value string) string {
	switch value {
	case "BoundaryHTTP":
		return string(BoundaryHTTP)
	case "BoundaryGRPC":
		return string(BoundaryGRPC)
	case "BoundaryWorkerClaim":
		return string(BoundaryWorkerClaim)
	case "BoundaryWorkerEffect":
		return string(BoundaryWorkerEffect)
	case "BoundaryOutbound":
		return string(BoundaryOutbound)
	case "BoundaryFinancial":
		return string(BoundaryFinancial)
	case "BoundaryOperation":
		return string(BoundaryOperation)
	default:
		return value
	}
}

func isKnownCapability(capability string) bool {
	for _, known := range []string{"admin.governance", "agent.run", "agent.tool_execution", "billing.ledger_lifecycle", "billing.payment_lifecycle", "channel.delivery", "chat.conversation_use", "chat.export", "deployment.operations", "gateway.request_admission", "identity.account_session", "identity.organization_membership", "knowledge.ingestion", "knowledge.retrieval", "marketplace.commerce", "marketplace.governance", "mcp.custom_execution", "mcp.network_execution", "mcp.tool_execution", "observability.audit", "observability.slo", "relay.provider_inference", "relay.stream_cancel", "relay.usage_settlement", "release.contract_reporting", "sandbox.code_execution", "task.scheduled_execution", "workflow.graph_execution", "workflow.replay"} {
		if capability == known {
			return true
		}
	}
	return false
}

func knownCapabilityForEffect(id string) string {
	for effect, capability := range authoredEffectCapabilities {
		if string(effect) == id {
			return string(capability)
		}
	}
	switch {
	case id == "http.admin.refund" || id == "http.billing.checkout":
		return "billing.payment_lifecycle"
	case strings.HasPrefix(id, "worker.schedule.workflow"):
		return "workflow.graph_execution"
	case strings.HasPrefix(id, "worker.schedule.agent"):
		return "agent.run"
	case strings.HasPrefix(id, "worker.schedule"):
		return "task.scheduled_execution"
	case strings.HasPrefix(id, "worker.channel"), strings.HasPrefix(id, "channel.delivery"):
		return "channel.delivery"
	case strings.HasPrefix(id, "worker.relay_batch"), strings.HasPrefix(id, "relay.provider"):
		return "relay.provider_inference"
	case strings.HasPrefix(id, "worker.archive"):
		return "observability.audit"
	case strings.HasPrefix(id, "marketplace."):
		return "marketplace.commerce"
	case strings.HasPrefix(id, "http."):
		return "gateway.request_admission"
	default:
		return ""
	}
}

var _ EffectRegistrar = (*EffectRegistry)(nil)
