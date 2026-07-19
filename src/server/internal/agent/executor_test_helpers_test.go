package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

func newAuthorizedToolExecutorForTest(t *testing.T, mcpClient *mcp.Client, configure ...func(*ToolRuntimeOptions)) *ToolExecutor {
	t.Helper()
	options := authorizedToolRuntimeOptionsForTest(t, configure...)
	executor, err := NewAuthorizedToolExecutor(mcpClient, options)
	if err != nil {
		t.Fatalf("construct authorized test ToolExecutor: %v", err)
	}
	return executor
}

func newAuthorizedServiceForTest(t *testing.T, store Store, gateway chat.ChatGateway, configure ...func(*ToolRuntimeOptions)) *Service {
	t.Helper()
	service, err := NewServiceWithRuntimeOptions(store, gateway, nil, authorizedToolRuntimeOptionsForTest(t, configure...))
	if err != nil {
		t.Fatalf("construct authorized test Agent service: %v", err)
	}
	return service
}

func authorizedToolRuntimeOptionsForTest(t *testing.T, configure ...func(*ToolRuntimeOptions)) ToolRuntimeOptions {
	t.Helper()
	guard := &liveExecutorGuard{}
	guard.allow.Store(true)
	contract, profile := loadLiveExecutorAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build test runtime authorities: %v", err)
	}
	options := ToolRuntimeOptions{
		Authorities: authorities,
		Guard:       guard,
		Effects:     &liveExecutorRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)},
		HTTPClient:  http.DefaultClient,
	}
	for _, apply := range configure {
		apply(&options)
	}
	return options
}

type toolExecutorConstructorCall struct {
	file     string
	function string
	name     string
}

func TestProductionToolExecutorConstructionContract(t *testing.T) {
	calls := collectToolExecutorConstructorCalls(t, false)
	assertConstructorCallCounts(t, calls, "NewToolExecutor", map[string]int{
		"internal/agent/service.go:ExecutePlanStep": 1,
		"internal/agent/service.go:initRunner":      1,
	})
	assertConstructorCallCounts(t, calls, "NewAuthorizedToolExecutor", map[string]int{
		"internal/agent/service.go:NewServiceWithRuntimeOptions": 1,
	})
	assertConstructorCallCounts(t, calls, "NewServiceWithRuntimeOptions", map[string]int{
		"internal/http/router.go:newRouterWithOptions":   1,
		"internal/http/server.go:buildRuntimeWithRouter": 1,
	})
	assertConstructorCallCounts(t, calls, "ToolExecutorComposite", map[string]int{
		"internal/agent/executor.go:NewToolExecutor":           1,
		"internal/agent/executor.go:NewAuthorizedToolExecutor": 1,
	})
}

func TestAuthorizedToolExecutorFixtureMigrationContract(t *testing.T) {
	calls := collectToolExecutorConstructorCalls(t, true)
	assertConstructorCallCounts(t, calls, "NewToolExecutor", map[string]int{
		"internal/agent/executor_test.go:TestCompatibilityToolExecutorFailClosedContract":          5,
		"internal/agent/executor_test.go:TestToolExecutorRejectsCustomPythonInProduction":          1,
		"internal/agent/executor_test.go:TestToolExecutorOmitsCustomPythonDefinitionsInProduction": 1,
		"internal/agent/executor_test.go:TestToolExecutorRejectsCustomPythonOversizedSource":       1,
		"internal/agent/executor_test.go:TestToolExecutorRejectsCustomPythonOversizedArguments":    1,
		"internal/agent/service_test.go:TestToolDefinitionsFilterDisabledCommercialBuiltins":       1,
		"internal/agent/service_test.go:TestToolDefinitionsExposeCustomToolInputSchema":            1,
	})
	for _, call := range calls {
		if call.name == "NewToolExecutor" && call.file == "internal/http/workflow_executor_test.go" {
			t.Fatalf("HTTP workflow behavior fixture still uses deny-only compatibility constructor: %s", call.function)
		}
	}
	assertConstructorCallerCount(t, calls, "NewAuthorizedToolExecutor", "internal/agent/executor_test_helpers_test.go:newAuthorizedToolExecutorForTest", 1)
	assertConstructorCallerCount(t, calls, "NewAuthorizedToolExecutor", "internal/http/workflow_executor_test.go:authorizedWorkflowToolExecutor", 1)
	assertConstructorCallerCount(t, calls, "NewServiceWithRuntimeOptions", "internal/http/workflow_executor_test.go:authorizedAgentServiceForHTTPTest", 1)
	assertConstructorCallCounts(t, calls, "ToolExecutorComposite", map[string]int{})
}

func TestToolExecutorConstructorInventoryMutationContract(t *testing.T) {
	serverRoot := t.TempDir()
	agentDir := filepath.Join(serverRoot, "internal", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("create mutation package: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(serverRoot, "internal", "http"), 0o755); err != nil {
		t.Fatalf("create mutation HTTP package: %v", err)
	}
	source := `package agent
func direct() { NewToolExecutor(nil) }
func parenthesized() { (NewToolExecutor)(nil) }
func compatibilityAlias() { ctor := NewToolExecutor; ctor(nil) }
func parenthesizedAlias() { ctor := (NewToolExecutor); (ctor)(nil) }
func authorizedAlias() { ctor := NewAuthorizedToolExecutor; ctor(nil, ToolRuntimeOptions{}) }
func serviceAlias() { ctor := NewServiceWithRuntimeOptions; ctor(nil, nil, nil, ToolRuntimeOptions{}) }
func composites() { _ = ToolExecutor{}; _ = &ToolExecutor{} }
`
	if err := os.WriteFile(filepath.Join(agentDir, "mutation.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write mutation source: %v", err)
	}

	calls := collectToolExecutorConstructorCallsAtRoot(t, serverRoot, false)
	assertConstructorCallCounts(t, calls, "NewToolExecutor", map[string]int{
		"internal/agent/mutation.go:direct":             1,
		"internal/agent/mutation.go:parenthesized":      1,
		"internal/agent/mutation.go:compatibilityAlias": 1,
		"internal/agent/mutation.go:parenthesizedAlias": 1,
	})
	assertConstructorCallCounts(t, calls, "NewAuthorizedToolExecutor", map[string]int{
		"internal/agent/mutation.go:authorizedAlias": 1,
	})
	assertConstructorCallCounts(t, calls, "NewServiceWithRuntimeOptions", map[string]int{
		"internal/agent/mutation.go:serviceAlias": 1,
	})
	assertConstructorCallCounts(t, calls, "ToolExecutorComposite", map[string]int{
		"internal/agent/mutation.go:composites": 2,
	})
}

func collectToolExecutorConstructorCalls(t *testing.T, tests bool) []toolExecutorConstructorCall {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve constructor contract source")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	return collectToolExecutorConstructorCallsAtRoot(t, serverRoot, tests)
}

func collectToolExecutorConstructorCallsAtRoot(t *testing.T, serverRoot string, tests bool) []toolExecutorConstructorCall {
	t.Helper()
	fset := token.NewFileSet()
	var calls []toolExecutorConstructorCall
	for _, dir := range []string{"internal/agent", "internal/http"} {
		err := filepath.WalkDir(filepath.Join(serverRoot, dir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") != tests {
				return nil
			}
			parsed, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(serverRoot, path)
			if err != nil {
				return err
			}
			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Body == nil {
					continue
				}
				aliases := collectToolExecutorConstructorAliases(function.Body)
				ast.Inspect(function.Body, func(node ast.Node) bool {
					switch value := node.(type) {
					case *ast.CallExpr:
						name := calledFunctionName(value.Fun, aliases)
						if name == "NewToolExecutor" && len(value.Args) != 1 {
							return true
						}
						if isToolExecutorConstructor(name) {
							calls = append(calls, toolExecutorConstructorCall{file: filepath.ToSlash(rel), function: function.Name.Name, name: name})
						}
					case *ast.CompositeLit:
						if isToolExecutorType(value.Type) {
							calls = append(calls, toolExecutorConstructorCall{file: filepath.ToSlash(rel), function: function.Name.Name, name: "ToolExecutorComposite"})
						}
					}
					return true
				})
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk production constructor calls: %v", err)
		}
	}
	return calls
}

func collectToolExecutorConstructorAliases(body *ast.BlockStmt) map[string]string {
	aliases := make(map[string]string)
	type aliasAssignment struct {
		name  string
		value ast.Expr
	}
	var assignments []aliasAssignment
	ast.Inspect(body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index := 0; index < len(value.Lhs) && index < len(value.Rhs); index++ {
				name, ok := value.Lhs[index].(*ast.Ident)
				if ok {
					assignments = append(assignments, aliasAssignment{name: name.Name, value: value.Rhs[index]})
				}
			}
		case *ast.ValueSpec:
			for index := 0; index < len(value.Names) && index < len(value.Values); index++ {
				assignments = append(assignments, aliasAssignment{name: value.Names[index].Name, value: value.Values[index]})
			}
		}
		return true
	})
	for _, assignment := range assignments {
		name := calledFunctionName(assignment.value, aliases)
		if isToolExecutorConstructor(name) {
			aliases[assignment.name] = name
		}
	}
	return aliases
}

func calledFunctionName(expression ast.Expr, aliases map[string]string) string {
	expression = unwrapParenExpr(expression)
	switch value := expression.(type) {
	case *ast.Ident:
		if name, ok := aliases[value.Name]; ok {
			return name
		}
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
	}
}

func unwrapParenExpr(expression ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			return expression
		}
		expression = parenthesized.X
	}
}

func isToolExecutorConstructor(name string) bool {
	return name == "NewToolExecutor" || name == "NewAuthorizedToolExecutor" || name == "NewServiceWithRuntimeOptions"
}

func isToolExecutorType(expression ast.Expr) bool {
	expression = unwrapParenExpr(expression)
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "ToolExecutor"
	case *ast.SelectorExpr:
		return value.Sel.Name == "ToolExecutor"
	default:
		return false
	}
}

func assertConstructorCallCounts(t *testing.T, calls []toolExecutorConstructorCall, constructor string, want map[string]int) {
	t.Helper()
	got := make(map[string]int)
	for _, call := range calls {
		if call.name == constructor {
			got[call.file+":"+call.function]++
		}
	}
	if equalCallCounts(got, want) {
		return
	}
	t.Fatalf("%s call inventory = %v, want %v", constructor, sortedCallCounts(got), sortedCallCounts(want))
}

func assertConstructorCallerCount(t *testing.T, calls []toolExecutorConstructorCall, constructor, caller string, want int) {
	t.Helper()
	got := 0
	for _, call := range calls {
		if call.name == constructor && call.file+":"+call.function == caller {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s calls from %s = %d, want %d", constructor, caller, got, want)
	}
}

func equalCallCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func sortedCallCounts(counts map[string]int) []string {
	result := make([]string, 0, len(counts))
	for key, value := range counts {
		result = append(result, key+"="+strconv.Itoa(value))
	}
	sort.Strings(result)
	return result
}
