package http

import (
	"context"
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/lib/pq"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

func TestBuildRuntimeAgentWebSearchCompositionContract(t *testing.T) {
	assertAgentWebSearchConstructionOrder(t)

	var transportCalls atomic.Int32
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		transportCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == stdhttp.MethodPost {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"web":{"results":[]}}`))
	}))
	defer upstream.Close()

	cases := []struct {
		name         string
		provider     string
		apiKey       string
		providerPair webSearchDescriptorPair
	}{
		{name: "direct Tavily", provider: "tavily", apiKey: "secret", providerPair: webSearchDescriptorPair{id: "mcp.websearch.tavily", owner: "mcp.TavilyWebSearchProvider"}},
		{name: "provider chain", provider: "brave,duckduckgo", apiKey: "secret", providerPair: webSearchDescriptorPair{id: "mcp.websearch.chain", owner: "mcp.websearch.Chain"}},
		{name: "single non-Tavily provider", provider: "brave", apiKey: "secret", providerPair: webSearchDescriptorPair{id: "mcp.websearch.provider", owner: "mcp.websearch.Provider"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			globalProvider := &countingGlobalWebSearchProvider{}
			restoreGlobal := mcp.SetWebSearchProviderForTest(globalProvider)
			t.Cleanup(restoreGlobal)
			constructionBaseline := transportCalls.Load()
			guard := &factoryReadinessGuard{}
			guard.allow.Store(true)
			contract, profile := loadFactoryReadinessContract(t)
			authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
			if err != nil {
				t.Fatalf("build runtime authorities: %v", err)
			}
			recorder := newWebSearchSubsetRecorder()
			database, err := sql.Open("postgres", "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
			if err != nil {
				t.Fatalf("open unopened database handle: %v", err)
			}
			t.Cleanup(func() { _ = database.Close() })

			cfg := testConfig()
			cfg.AgentWebSearchProvider = tc.provider
			cfg.AgentWebSearchEndpoint = upstream.URL
			cfg.AgentWebSearchAPIKey = tc.apiKey
			cfg.AgentWebSearchResultLimit = 1
			runtime, err := BuildRuntime(cfg, database, RuntimeOptions{
				Readiness: strictRouterReadinessStub{}, Guard: guard, Effects: recorder, Authorities: authorities,
			})
			if err != nil {
				t.Fatalf("BuildRuntime: %v", err)
			}
			t.Cleanup(func() { _ = runtime.Close(context.Background()) })
			if got := transportCalls.Load(); got != constructionBaseline {
				t.Fatalf("BuildRuntime transport delta = %d, want 0", got-constructionBaseline)
			}

			want := []webSearchDescriptorPair{
				{id: "agent.tool.builtin", owner: "agent.ToolExecutor.builtin"},
				{id: "mcp.websearch.builtin", owner: "mcp.WebSearchTool"},
				tc.providerPair,
			}
			if err := recorder.requireExact(want); err != nil {
				t.Fatal(err)
			}
			if got := recorder.countID("agent.tool.web_search"); got != 0 {
				t.Fatalf("agent.tool.web_search registrations = %d, want 0", got)
			}
			for _, alternative := range []webSearchDescriptorPair{
				{id: "mcp.websearch.tavily", owner: "mcp.TavilyWebSearchProvider"},
				{id: "mcp.websearch.chain", owner: "mcp.websearch.Chain"},
				{id: "mcp.websearch.provider", owner: "mcp.websearch.Provider"},
			} {
				if alternative != tc.providerPair && recorder.count(alternative) != 0 {
					t.Fatalf("non-selected provider pair %+v registered %d times", alternative, recorder.count(alternative))
				}
			}
			mutated := append([]webSearchDescriptorPair(nil), want...)
			mutated[len(mutated)-1].owner = "wrong.Owner"
			if equalWebSearchDescriptorPairs(want, mutated) {
				t.Fatal("pair comparison accepted a correct ID with the wrong owner")
			}

			executionRecorder := newWebSearchSubsetRecorder()
			provider, err := buildAgentWebSearchProviderWithOptions(cfg, mcp.WebSearchRuntimeOptions{
				Guard: guard, Authorities: authorities, Effects: executionRecorder,
			})
			if err != nil {
				t.Fatalf("build execution provider: %v", err)
			}
			executor, err := agent.NewAuthorizedToolExecutor(nil, agent.ToolRuntimeOptions{
				Guard: guard, Authorities: authorities, Effects: executionRecorder,
				HTTPClient: stdhttp.DefaultClient, WebSearchProvider: provider,
			})
			if err != nil {
				t.Fatalf("build instance-scoped executor: %v", err)
			}
			persistedAgent := &agent.Agent{OrganizationID: "org_1", Tools: []agent.Tool{{Name: "web_search", Type: "builtin", Enabled: true}}}
			guard.allow.Store(false)
			before := transportCalls.Load()
			if _, err := executor.Execute(t.Context(), persistedAgent, &agent.ToolCall{Name: "web_search", Arguments: map[string]any{"query": "denied"}}); err == nil {
				t.Fatal("denied web search unexpectedly succeeded")
			}
			if got := transportCalls.Load(); got != before {
				t.Fatalf("denied web search transport delta = %d, want 0", got-before)
			}
			guard.allow.Store(true)
			if _, err := executor.Execute(t.Context(), persistedAgent, &agent.ToolCall{Name: "web_search", Arguments: map[string]any{"query": "current"}}); err != nil {
				t.Fatalf("current web search: %v", err)
			}
			if got := transportCalls.Load(); got != before+1 {
				t.Fatalf("current web search transport delta = %d, want 1", got-before)
			}
			if got := globalProvider.calls.Load(); got != 0 {
				t.Fatalf("strict executor used package-global web search provider %d times", got)
			}
		})
	}
}

type countingGlobalWebSearchProvider struct{ calls atomic.Int32 }

func (p *countingGlobalWebSearchProvider) Search(context.Context, string) ([]mcp.WebSearchResult, error) {
	p.calls.Add(1)
	return []mcp.WebSearchResult{{Title: "unexpected global result"}}, nil
}

type webSearchDescriptorPair struct {
	id    string
	owner string
}

type webSearchSubsetRecorder struct {
	mu     sync.Mutex
	counts map[webSearchDescriptorPair]int
	ids    map[string]int
}

func newWebSearchSubsetRecorder() *webSearchSubsetRecorder {
	return &webSearchSubsetRecorder{counts: make(map[webSearchDescriptorPair]int), ids: make(map[string]int)}
}

func (r *webSearchSubsetRecorder) Register(descriptor releasecontract.EffectDescriptor) error {
	if !isWebSearchTargetID(descriptor.ID) {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[webSearchDescriptorPair{id: descriptor.ID, owner: descriptor.Owner}]++
	r.ids[descriptor.ID]++
	return nil
}

func (r *webSearchSubsetRecorder) count(pair webSearchDescriptorPair) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[pair]
}

func (r *webSearchSubsetRecorder) countID(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ids[id]
}

func (r *webSearchSubsetRecorder) requireExact(want []webSearchDescriptorPair) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual := make([]webSearchDescriptorPair, 0, len(r.counts))
	for pair, count := range r.counts {
		if count != 1 {
			return fmt.Errorf("web-search pair %+v registered %d times, want 1", pair, count)
		}
		actual = append(actual, pair)
	}
	if !equalWebSearchDescriptorPairs(actual, want) {
		return fmt.Errorf("web-search descriptor pairs = %v, want %v", sortedWebSearchPairs(actual), sortedWebSearchPairs(want))
	}
	return nil
}

func isWebSearchTargetID(id string) bool {
	switch id {
	case "agent.tool.builtin", "agent.tool.web_search", "mcp.websearch.builtin", "mcp.websearch.tavily", "mcp.websearch.chain", "mcp.websearch.provider":
		return true
	default:
		return false
	}
}

func equalWebSearchDescriptorPairs(left, right []webSearchDescriptorPair) bool {
	if len(left) != len(right) {
		return false
	}
	leftSorted := sortedWebSearchPairs(left)
	rightSorted := sortedWebSearchPairs(right)
	for index := range leftSorted {
		if leftSorted[index] != rightSorted[index] {
			return false
		}
	}
	return true
}

func sortedWebSearchPairs(pairs []webSearchDescriptorPair) []webSearchDescriptorPair {
	result := append([]webSearchDescriptorPair(nil), pairs...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].id == result[j].id {
			return result[i].owner < result[j].owner
		}
		return result[i].id < result[j].id
	})
	return result
}

func assertAgentWebSearchConstructionOrder(t *testing.T) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve runtime composition test source")
	}
	serverPath := filepath.Join(filepath.Dir(source), "server.go")
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, serverPath, nil, 0)
	if err != nil {
		t.Fatalf("parse server.go: %v", err)
	}
	var strictBody *ast.BlockStmt
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "buildRuntimeWithRouter" {
			continue
		}
		for _, statement := range function.Body.List {
			conditional, ok := statement.(*ast.IfStmt)
			identifier, isIdentifier := conditionalConditionIdentifier(conditional)
			if ok && isIdentifier && identifier == "requireReadiness" {
				strictBody = conditional.Body
				break
			}
		}
	}
	if strictBody == nil {
		t.Fatal("strict requireReadiness block not found")
	}

	var providerCall, providerField, serviceCall token.Pos
	ast.Inspect(strictBody, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			name := calledFunctionName(call.Fun)
			switch name {
			case "buildAgentWebSearchProviderWithOptions":
				providerCall = call.Pos()
			case "NewServiceWithRuntimeOptions":
				serviceCall = call.Pos()
			case "SetWebSearchProvider":
				t.Errorf("strict composition still calls SetWebSearchProvider at %s", files.Position(call.Pos()))
			case "NewWebsearchToolWithOptions", "NewRegistryWithOptions", "NewDefaultRegistryWithOptions":
				t.Errorf("strict composition pads effect coverage with %s at %s", name, files.Position(call.Pos()))
			}
		}
		pair, ok := node.(*ast.KeyValueExpr)
		if ok {
			key, keyOK := pair.Key.(*ast.Ident)
			if keyOK && key.Name == "WebSearchProvider" {
				providerField = pair.Pos()
			}
		}
		return true
	})
	if providerCall == token.NoPos || providerField == token.NoPos || serviceCall == token.NoPos {
		t.Fatalf("strict construction positions provider=%s injection=%s service=%s", files.Position(providerCall), files.Position(providerField), files.Position(serviceCall))
	}
	if !(providerCall < providerField && providerField < serviceCall) {
		t.Fatalf("strict construction order provider=%s injection=%s service=%s", files.Position(providerCall), files.Position(providerField), files.Position(serviceCall))
	}
}

func conditionalConditionIdentifier(conditional *ast.IfStmt) (string, bool) {
	if conditional == nil {
		return "", false
	}
	identifier, ok := conditional.Cond.(*ast.Ident)
	if !ok {
		return "", false
	}
	return identifier.Name, true
}

func calledFunctionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.ParenExpr:
		return calledFunctionName(value.X)
	default:
		return ""
	}
}
