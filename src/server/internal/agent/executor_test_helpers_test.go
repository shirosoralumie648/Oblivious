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
}

func collectToolExecutorConstructorCalls(t *testing.T, tests bool) []toolExecutorConstructorCall {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve constructor contract source")
	}
	serverRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
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
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					name := calledFunctionName(call.Fun)
					if name == "NewToolExecutor" && len(call.Args) != 1 {
						return true
					}
					if name == "NewToolExecutor" || name == "NewAuthorizedToolExecutor" || name == "NewServiceWithRuntimeOptions" {
						calls = append(calls, toolExecutorConstructorCall{file: filepath.ToSlash(rel), function: function.Name.Name, name: name})
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

func calledFunctionName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	default:
		return ""
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
