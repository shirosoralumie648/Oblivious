package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

// Category classifies a tool by its origin.
type Category string

const (
	CategoryBuiltin Category = "builtin"
	CategoryCustom  Category = "custom"
	CategoryMCP     Category = "mcp"
)

// ToolMetadata holds descriptive information about a registered tool.
type ToolMetadata struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Category         Category       `json:"category"`
	InputSchema      any            `json:"inputSchema,omitempty"`
	RequiresApproval bool           `json:"requiresApproval,omitempty"`
	RiskLevel        string         `json:"riskLevel,omitempty"`
	ServerID         string         `json:"serverId,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Extra            map[string]any `json:"extra,omitempty"`
}

// ToolExecutor is a function that executes a tool call.
type ToolExecutor func(ctx context.Context, args map[string]any) (*mcp.ToolResult, error)

// registryEntry holds a tool plus its executor.
type registryEntry struct {
	metadata ToolMetadata
	executor ToolExecutor
	runtime  releasecontract.CatalogRuntimeClass
}

// Registry is a thread-safe tool registry supporting registration, discovery,
// and categorisation of builtin, custom, and MCP tools.
type Registry struct {
	mu        sync.RWMutex
	entries   map[string]registryEntry
	readiness *registryReadiness
}

// RegistryRuntimeOptions is the startup-only authority carrier for this
// independent registry API. The production Agent ToolExecutor has its own
// dispatch seam and is guarded by a later plan.
type RegistryRuntimeOptions struct {
	Guard       releasecontract.Guard
	Authorities releasecontract.RuntimeAuthorities
	Effects     releasecontract.EffectRegistrar
}

type registryReadiness struct {
	authorities releasecontract.RuntimeAuthorities
	effects     map[Category]releasecontract.CapabilityID
}

func newRegistryReadiness(options RegistryRuntimeOptions) (*registryReadiness, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "agent.tools.registry"}
	}
	effectIDs := map[Category]releasecontract.EffectID{
		CategoryBuiltin: releasecontract.EffectAgentToolBuiltin,
		CategoryCustom:  releasecontract.EffectAgentToolCustomAPIHTTP,
		CategoryMCP:     releasecontract.EffectAgentToolMCP,
	}
	capabilities := make(map[Category]releasecontract.CapabilityID, len(effectIDs))
	for category, effectID := range effectIDs {
		capabilityID, err := options.Authorities.CapabilityBindings.Resolve(effectID)
		if err != nil {
			return nil, err
		}
		capabilities[category] = capabilityID
		descriptor := releasecontract.EffectDescriptor{
			ID:           "agent.tool.registry." + string(category),
			CapabilityID: string(capabilityID),
			Boundary:     releasecontract.BoundaryOutbound,
			Owner:        "agent.tools.Registry",
		}
		if err := options.Effects.Register(descriptor); err != nil {
			return nil, err
		}
	}
	return &registryReadiness{authorities: options.Authorities, effects: capabilities}, nil
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]registryEntry),
	}
}

// NewRegistryWithOptions constructs the independent registry with immutable
// startup readiness wiring and exact once-per-category effect descriptors.
func NewRegistryWithOptions(options RegistryRuntimeOptions) (*Registry, error) {
	readiness, err := newRegistryReadiness(options)
	if err != nil {
		return nil, err
	}
	return &Registry{entries: make(map[string]registryEntry), readiness: readiness}, nil
}

// NewDefaultRegistry creates a Registry pre-loaded with all builtin tools.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	for name, tool := range mcp.BuiltinTools {
		r.RegisterBuiltin(name, tool)
	}
	return r
}

// NewDefaultRegistryWithOptions is the readiness-bound counterpart of
// NewDefaultRegistry.
func NewDefaultRegistryWithOptions(options RegistryRuntimeOptions) (*Registry, error) {
	r, err := NewRegistryWithOptions(options)
	if err != nil {
		return nil, err
	}
	for name, tool := range mcp.BuiltinTools {
		r.RegisterBuiltin(name, tool)
	}
	return r, nil
}

// RegisterBuiltin registers a builtin tool.
func (r *Registry) RegisterBuiltin(name string, tool mcp.BuiltinTool) {
	if tool == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = registryEntry{
		metadata: ToolMetadata{
			Name:        tool.Name(),
			Description: tool.Description(),
			Category:    CategoryBuiltin,
			InputSchema: tool.InputSchema(),
		},
		executor: func(ctx context.Context, args map[string]any) (*mcp.ToolResult, error) {
			return tool.Execute(ctx, args)
		},
		runtime: releasecontract.CatalogRuntimeBuiltin,
	}
}

// RegisterCustom registers a custom tool with explicit metadata.
func (r *Registry) RegisterCustom(meta ToolMetadata, executor ToolExecutor) {
	if strings.TrimSpace(meta.Name) == "" || executor == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	meta.Category = CategoryCustom
	r.entries[meta.Name] = registryEntry{
		metadata: meta,
		executor: executor,
		runtime:  releasecontract.CatalogRuntimeCustom,
	}
}

// RegisterMCP registers an MCP tool with explicit metadata.
func (r *Registry) RegisterMCP(meta ToolMetadata, executor ToolExecutor) {
	if strings.TrimSpace(meta.Name) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	meta.Category = CategoryMCP
	r.entries[meta.Name] = registryEntry{
		metadata: meta,
		executor: executor,
		runtime:  releasecontract.CatalogRuntimeMCP,
	}
}

// Register replaces or inserts a tool entry.
func (r *Registry) Register(meta ToolMetadata, executor ToolExecutor) {
	if strings.TrimSpace(meta.Name) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[meta.Name] = registryEntry{
		metadata: meta,
		executor: executor,
		runtime:  registryRuntimeForCategory(meta.Category),
	}
}

// Unregister removes a tool by name.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, name)
}

// Get returns the metadata and executor for a tool.
func (r *Registry) Get(name string) (ToolMetadata, ToolExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	if !ok {
		return ToolMetadata{}, nil, false
	}
	return entry.metadata, entry.executor, true
}

// Has reports whether a tool with the given name is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[name]
	return ok
}

// List returns all registered tool metadata sorted by name.
func (r *Registry) List() []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolMetadata, 0, len(r.entries))
	for _, entry := range r.entries {
		result = append(result, entry.metadata)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByCategory returns tools filtered by category.
func (r *Registry) ListByCategory(cat Category) []ToolMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ToolMetadata
	for _, entry := range r.entries {
		if entry.metadata.Category == cat {
			result = append(result, entry.metadata)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// ListByRiskLevel returns tools filtered by risk level.
func (r *Registry) ListByRiskLevel(level string) []ToolMetadata {
	normalized := strings.ToLower(strings.TrimSpace(level))
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ToolMetadata
	for _, entry := range r.entries {
		if strings.ToLower(strings.TrimSpace(entry.metadata.RiskLevel)) == normalized {
			result = append(result, entry.metadata)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Search finds tools whose name or description contain the query (case-insensitive).
func (r *Registry) Search(query string) []ToolMetadata {
	q := strings.ToLower(strings.TrimSpace(query))
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []ToolMetadata
	for _, entry := range r.entries {
		if strings.Contains(strings.ToLower(entry.metadata.Name), q) ||
			strings.Contains(strings.ToLower(entry.metadata.Description), q) {
			result = append(result, entry.metadata)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Execute runs a tool by name with the given arguments.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (*mcp.ToolResult, error) {
	r.mu.RLock()
	entry, ok := r.entries[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	if entry.executor == nil {
		return nil, fmt.Errorf("tool %s has no executor", name)
	}
	if r.readiness != nil {
		if err := r.readiness.authorize(ctx, entry); err != nil {
			return nil, err
		}
	}
	return entry.executor(ctx, args)
}

func registryRuntimeForCategory(category Category) releasecontract.CatalogRuntimeClass {
	switch category {
	case CategoryBuiltin:
		return releasecontract.CatalogRuntimeBuiltin
	case CategoryCustom:
		return releasecontract.CatalogRuntimeCustom
	case CategoryMCP:
		return releasecontract.CatalogRuntimeMCP
	default:
		return ""
	}
}

func (r *registryReadiness) authorize(ctx context.Context, entry registryEntry) error {
	if r == nil {
		return nil
	}
	var subject releasecontract.CatalogSubject
	switch entry.metadata.Category {
	case CategoryBuiltin:
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectTool, ID: strings.TrimSpace(entry.metadata.Name), Runtime: entry.runtime}
	case CategoryCustom:
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "custom", Runtime: releasecontract.CatalogRuntimeCustom}
	case CategoryMCP:
		subject = releasecontract.CatalogSubject{Kind: releasecontract.CatalogSubjectRuntime, ID: "mcp", Runtime: releasecontract.CatalogRuntimeMCP}
	default:
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tools.category"}
	}
	capabilityID, err := r.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, subject, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if expected := r.effects[entry.metadata.Category]; expected != capabilityID {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tools.runtimeCapability"}
	}
	return nil
}

// ToOpenAITools converts all registered tools to OpenAI function-calling format.
func (r *Registry) ToOpenAITools() []map[string]any {
	metadata := r.List()
	tools := make([]map[string]any, 0, len(metadata))
	for _, m := range metadata {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        m.Name,
				"description": m.Description,
				"parameters":  m.InputSchema,
			},
		})
	}
	return tools
}

// MarshalJSON implements json.Marshaler so the registry can be serialised
// as a list of ToolMetadata.
func (r *Registry) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.List())
}

// CountByCategory returns the number of tools in each category.
func (r *Registry) CountByCategory() map[Category]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := map[Category]int{
		CategoryBuiltin: 0,
		CategoryCustom:  0,
		CategoryMCP:     0,
	}
	for _, entry := range r.entries {
		counts[entry.metadata.Category]++
	}
	return counts
}
