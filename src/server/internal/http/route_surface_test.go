package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/schedule"
)

const routeSurfaceStrongPassword = "StrongerPass1!"

func TestRouteSurfaceRequiresSessionForAppRoutes(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"agents", stdhttp.MethodGet, "/api/v1/app/agents"},
		{"agent memories", stdhttp.MethodGet, "/api/v1/agent/memories"},
		{"memory documents", stdhttp.MethodGet, "/api/v1/app/memory/documents"},
		{"mcp servers", stdhttp.MethodGet, "/api/v1/app/mcp-servers"},
		{"notifications", stdhttp.MethodGet, "/api/v1/app/notifications"},
		{"quota", stdhttp.MethodGet, "/api/v1/app/quota"},
		{"console usage", stdhttp.MethodGet, "/api/v1/console/usage"},
		{"preferences", stdhttp.MethodGet, "/api/v1/app/me/preferences"},
		{"knowledge bases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases"},
		{"knowledge document upload", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"knowledge ingestion jobs", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/ingestion-jobs"},
		{"tasks", stdhttp.MethodGet, "/api/v1/app/tasks"},
		{"organizations", stdhttp.MethodGet, "/api/v1/app/organizations"},
		{"organization members", stdhttp.MethodGet, "/api/v1/app/organizations/org_1/members"},
		{"gateway proxy", stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions"},
		{"websocket", stdhttp.MethodGet, "/api/v1/ws"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWebSocketRequiresSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/ws", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected websocket route to require session with 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceWebSocketRejectsUnsupportedMethodsWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/ws", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusMethodNotAllowed {
		t.Fatalf("expected websocket route to reject POST with 405, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceRegistersCanonicalKnowledgeRoutesThroughRegistrar(t *testing.T) {
	called := routeSurfaceCalledRegistrars(t, "router.go")

	if !called["registerKnowledgeRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register canonical app knowledge routes through registerKnowledgeRouteSurfaces")
	}
	if !called["registerKnowledgeAliasRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register Knowledge compatibility alias routes through registerKnowledgeAliasRouteSurfaces")
	}
}

func TestRouteSurfaceRegistersCanonicalChatRoutesThroughRegistrar(t *testing.T) {
	called := routeSurfaceCalledRegistrars(t, "router.go")

	if !called["registerChatRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register canonical app Chat routes through registerChatRouteSurfaces")
	}
	if !called["registerConversationAliasRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register Chat conversation alias routes through registerConversationAliasRouteSurfaces")
	}
}

func TestRouteSurfaceStandaloneChatRouterRegistersChatOnlyRoutes(t *testing.T) {
	router := NewChatRouter(testConfig(), nil)

	chatRoutes := []routeSurfaceCase{
		{"models", stdhttp.MethodGet, "/api/v1/app/models"},
		{"list app conversations", stdhttp.MethodGet, "/api/v1/app/conversations"},
		{"list alias conversations", stdhttp.MethodGet, "/api/v1/conversations"},
	}
	for _, tt := range chatRoutes {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected standalone chat route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}

	healthRecorder := httptest.NewRecorder()
	router.ServeHTTP(healthRecorder, httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil))
	if healthRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected standalone chat health route to return 200, got %d with body %s", healthRecorder.Code, healthRecorder.Body.String())
	}

	adminRecorder := httptest.NewRecorder()
	router.ServeHTTP(adminRecorder, httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/stats", nil))
	if adminRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected standalone chat router not to expose admin routes, got %d with body %s", adminRecorder.Code, adminRecorder.Body.String())
	}
}

func TestRouteSurfaceKnowledgeRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list knowledge bases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases"},
		{"create knowledge base", stdhttp.MethodPost, "/api/v1/app/knowledge-bases"},
		{"get knowledge base", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1"},
		{"update knowledge base", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1"},
		{"delete knowledge base", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1"},
		{"list documents", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents"},
		{"create document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents"},
		{"upload document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"list ingestion jobs", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/ingestion-jobs"},
		{"update document", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"delete document", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"list document versions", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/versions"},
		{"list chunks", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks"},
		{"update chunk", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"retrieve", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve"},
		{"retrieve debug", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve/debug"},
		{"list retrieval test cases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases"},
		{"create retrieval test case", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases"},
		{"run retrieval test cases", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Knowledge route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKnowledgeRoutesDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"list knowledge bases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases"},
		{"create knowledge base", stdhttp.MethodPost, "/api/v1/app/knowledge-bases"},
		{"get knowledge base", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1"},
		{"update knowledge base", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1"},
		{"delete knowledge base", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1"},
		{"list documents", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents"},
		{"create document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents"},
		{"upload document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"list ingestion jobs", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/ingestion-jobs"},
		{"update document", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"delete document", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"list document versions", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/versions"},
		{"list chunks", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks"},
		{"update chunk", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"retrieve", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve"},
		{"retrieve debug", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve/debug"},
		{"list retrieval test cases", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases"},
		{"create retrieval test case", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases"},
		{"run retrieval test cases", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Docs","title":"Plan","content":"hello","query":"hello","expectedResult":{"documentId":"doc_1","documentTitle":"Plan","snippet":"hello"},"splitAt":1,"direction":"next"}`))
			request.Header.Set("Content-Type", "application/json")
			if !isSafeMethod(tt.method) {
				request.Header.Set(csrfHeaderName, csrfToken)
			}
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Knowledge route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKnowledgeAliasRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	for _, tt := range routeSurfaceKnowledgeAliasCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Knowledge alias route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKnowledgeAliasRoutesDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	for _, tt := range routeSurfaceKnowledgeAliasCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Docs","title":"Plan","content":"hello","query":"hello","expectedResult":{"documentId":"doc_1","documentTitle":"Plan","snippet":"hello"},"splitAt":1,"direction":"next"}`))
			request.Header.Set("Content-Type", "application/json")
			if !isSafeMethod(tt.method) {
				request.Header.Set(csrfHeaderName, csrfToken)
			}
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Knowledge alias route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			case stdhttp.StatusNotFound:
				if strings.Contains(recorder.Body.String(), "route not found") {
					t.Fatalf("expected registered Knowledge alias route to dispatch for %s %s, got route 404 with body %s", tt.method, tt.path, recorder.Body.String())
				}
			}
		})
	}
}

func routeSurfaceKnowledgeAliasCases() []routeSurfaceCase {
	return []routeSurfaceCase{
		{"list alias knowledge bases", stdhttp.MethodGet, "/api/v1/knowledge-bases"},
		{"create alias knowledge base", stdhttp.MethodPost, "/api/v1/knowledge-bases"},
		{"get alias knowledge base", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1"},
		{"update alias knowledge base", stdhttp.MethodPut, "/api/v1/knowledge-bases/kb_1"},
		{"delete alias knowledge base", stdhttp.MethodDelete, "/api/v1/knowledge-bases/kb_1"},
		{"list alias documents", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1/documents"},
		{"create alias document", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/documents"},
		{"upload alias document", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/documents/upload"},
		{"list alias ingestion jobs", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1/documents/ingestion-jobs"},
		{"update alias document", stdhttp.MethodPut, "/api/v1/knowledge-bases/kb_1/documents/doc_1"},
		{"delete alias document", stdhttp.MethodDelete, "/api/v1/knowledge-bases/kb_1/documents/doc_1"},
		{"root delete alias document", stdhttp.MethodDelete, "/api/v1/documents/doc_1"},
		{"list alias document versions", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1/documents/doc_1/versions"},
		{"list alias chunks", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1/documents/doc_1/chunks"},
		{"update alias chunk", stdhttp.MethodPut, "/api/v1/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split alias chunk", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge alias chunk", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"alias retrieve", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/retrieve"},
		{"alias retrieve debug", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/retrieve/debug"},
		{"list alias retrieval test cases", stdhttp.MethodGet, "/api/v1/knowledge-bases/kb_1/retrieval-test-cases"},
		{"create alias retrieval test case", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/retrieval-test-cases"},
		{"run alias retrieval test cases", stdhttp.MethodPost, "/api/v1/knowledge-bases/kb_1/retrieval-test-cases/run"},
	}
}

func routeSurfaceKnowledgeAliasMutationCases() []routeSurfaceCase {
	mutations := make([]routeSurfaceCase, 0)
	for _, tt := range routeSurfaceKnowledgeAliasCases() {
		if !isSafeMethod(tt.method) {
			mutations = append(mutations, tt)
		}
	}
	return mutations
}

func TestRouteSurfaceRegistersConsoleInvoiceRoute(t *testing.T) {
	if !routeSurfaceCalledRegistrars(t, "router.go")["registerConsoleRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRouteSurfaces")
	}
	if !routeSurfaceOperationPresent(consoleRouteSurfaceOperations(), stdhttp.MethodGet, "/api/v1/console/invoices", "listConsoleBillingInvoices") {
		t.Fatal("expected typed Console route surface to expose /api/v1/console/invoices")
	}
}

func TestRouteSurfaceRegistersConsoleAPITokenRoutes(t *testing.T) {
	if !routeSurfaceCalledRegistrars(t, "router.go")["registerConsoleRouteSurfaces"] {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRouteSurfaces")
	}
	for _, expected := range []struct {
		method      string
		path        string
		operationID string
	}{
		{stdhttp.MethodGet, "/api/v1/console/api-tokens", "listConsoleAPITokens"},
		{stdhttp.MethodPost, "/api/v1/console/api-tokens", "createConsoleAPIToken"},
		{stdhttp.MethodGet, "/api/v1/console/api-tokens/{tokenId}/usage", "listConsoleAPITokenUsage"},
		{stdhttp.MethodDelete, "/api/v1/console/api-tokens/{tokenId}", "revokeConsoleAPIToken"},
	} {
		if !routeSurfaceOperationPresent(consoleRouteSurfaceOperations(), expected.method, expected.path, expected.operationID) {
			t.Fatalf("expected typed Console API token route %s %s (%s)", expected.method, expected.path, expected.operationID)
		}
	}
}

func routeSurfaceOperationPresent(operations []OperationContractMetadataV1, method, path, operationID string) bool {
	for _, operation := range operations {
		if operation.Method == method && operation.NormalizedPath == path && operation.OperationID == operationID {
			return true
		}
	}
	return false
}

func TestRouteSurfaceRegistersAdminAPITokenRoutes(t *testing.T) {
	source := routeSurfaceReadSourceFile(t, "router.go")
	text := string(source)

	for _, expected := range []string{
		`mux.Handle("/api/v1/admin/api-tokens"`,
		`mux.Handle("/api/v1/admin/api-tokens/"`,
		"adminHandler.listAPITokens",
		"adminHandler.revokeAPIToken",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected NewRouterWithOptions to expose Admin API token route surface containing %q", expected)
		}
	}
}

func TestRouteSurfaceConsoleReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"get usage", stdhttp.MethodGet, "/api/v1/console/usage"},
		{"get access", stdhttp.MethodGet, "/api/v1/console/access"},
		{"get models", stdhttp.MethodGet, "/api/v1/console/models"},
		{"get billing", stdhttp.MethodGet, "/api/v1/console/billing"},
		{"list invoices", stdhttp.MethodGet, "/api/v1/console/invoices"},
		{"list api tokens", stdhttp.MethodGet, "/api/v1/console/api-tokens"},
		{"list api token usage", stdhttp.MethodGet, "/api/v1/console/api-tokens/tok_1/usage"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Console read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceRequiresSessionForConsoleAPITokenRoutesWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []struct {
		method string
		path   string
	}{
		{stdhttp.MethodGet, "/api/v1/console/api-tokens"},
		{stdhttp.MethodPost, "/api/v1/console/api-tokens"},
		{stdhttp.MethodGet, "/api/v1/console/api-tokens/tok_1/usage"},
		{stdhttp.MethodDelete, "/api/v1/console/api-tokens/tok_1"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceConsoleAPITokenMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create api token", stdhttp.MethodPost, "/api/v1/console/api-tokens"},
		{"revoke api token", stdhttp.MethodDelete, "/api/v1/console/api-tokens/tok_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceTaskMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create task", stdhttp.MethodPost, "/api/v1/app/tasks"},
		{"start task", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/start"},
		{"approve task", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/approve"},
		{"pause task", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/pause"},
		{"resume task", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/resume"},
		{"cancel task", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/cancel"},
		{"update task budget", stdhttp.MethodPost, "/api/v1/app/tasks/task_1/budget"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceTaskReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list tasks", stdhttp.MethodGet, "/api/v1/app/tasks"},
		{"get task", stdhttp.MethodGet, "/api/v1/app/tasks/task_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Task read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceNotificationMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create notification", stdhttp.MethodPost, "/api/v1/app/notifications"},
		{"mark all read", stdhttp.MethodPost, "/api/v1/app/notifications/mark-all-read"},
		{"mark read", stdhttp.MethodPatch, "/api/v1/app/notifications/notification_1"},
		{"delete notification", stdhttp.MethodDelete, "/api/v1/app/notifications/notification_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfacePreferencesMutationRejectsCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/me/preferences", strings.NewReader(`{"defaultMode":"solo"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403 for PUT /api/v1/app/me/preferences, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfacePreferencesAndNotificationReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"get preferences", stdhttp.MethodGet, "/api/v1/app/me/preferences"},
		{"list notifications", stdhttp.MethodGet, "/api/v1/app/notifications"},
		{"get unread notification count", stdhttp.MethodGet, "/api/v1/app/notifications/unread-count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Preferences/Notification read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatPersonaRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list personas", stdhttp.MethodGet, "/api/v1/app/personas"},
		{"create persona", stdhttp.MethodPost, "/api/v1/app/personas"},
		{"get persona", stdhttp.MethodGet, "/api/v1/app/personas/persona_1"},
		{"update persona", stdhttp.MethodPut, "/api/v1/app/personas/persona_1"},
		{"delete persona", stdhttp.MethodDelete, "/api/v1/app/personas/persona_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Helpful Assistant"}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Chat persona route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatPrivateReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"models", stdhttp.MethodGet, "/api/v1/app/models"},
		{"list personas", stdhttp.MethodGet, "/api/v1/app/personas"},
		{"get persona", stdhttp.MethodGet, "/api/v1/app/personas/persona_1"},
		{"list conversations", stdhttp.MethodGet, "/api/v1/app/conversations"},
		{"get conversation", stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1"},
		{"list messages", stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/messages"},
		{"get conversation config", stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/config"},
		{"export conversation", stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/export.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Chat private read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatConversationAliasRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list alias conversations", stdhttp.MethodGet, "/api/v1/conversations"},
		{"get alias conversation", stdhttp.MethodGet, "/api/v1/conversations/conversation_1"},
		{"list alias messages", stdhttp.MethodGet, "/api/v1/conversations/conversation_1/messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Chat alias route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatConversationAliasMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create alias conversation", stdhttp.MethodPost, "/api/v1/conversations"},
		{"update alias conversation", stdhttp.MethodPut, "/api/v1/conversations/conversation_1"},
		{"delete alias conversation", stdhttp.MethodDelete, "/api/v1/conversations/conversation_1"},
		{"fork alias conversation", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/fork"},
		{"send alias message", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/messages"},
		{"update alias message", stdhttp.MethodPut, "/api/v1/conversations/conversation_1/messages/message_1"},
		{"delete alias message", stdhttp.MethodDelete, "/api/v1/conversations/conversation_1/messages/message_1"},
		{"bookmark alias message", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/messages/message_1/bookmark"},
		{"create alias message share", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/messages/message_1/share"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"content":"hello","bookmarked":true}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for Chat alias %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatConversationAliasMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create alias conversation", stdhttp.MethodPost, "/api/v1/conversations"},
		{"update alias conversation", stdhttp.MethodPut, "/api/v1/conversations/conversation_1"},
		{"fork alias conversation", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/fork"},
		{"send alias message", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/messages"},
		{"update alias message", stdhttp.MethodPut, "/api/v1/conversations/conversation_1/messages/message_1"},
		{"create alias message share", stdhttp.MethodPost, "/api/v1/conversations/conversation_1/messages/message_1/share"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected Chat alias route to pass auth/csrf and dispatch to request validation for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
				t.Fatalf("expected invalid_request response from Chat alias handler, got %s", recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create conversation", stdhttp.MethodPost, "/api/v1/app/conversations"},
		{"update conversation", stdhttp.MethodPut, "/api/v1/app/conversations/conversation_1"},
		{"delete conversation", stdhttp.MethodDelete, "/api/v1/app/conversations/conversation_1"},
		{"fork conversation", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/fork"},
		{"send message", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages"},
		{"stream message", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages/stream"},
		{"update message", stdhttp.MethodPut, "/api/v1/app/conversations/conversation_1/messages/message_1"},
		{"delete message", stdhttp.MethodDelete, "/api/v1/app/conversations/conversation_1/messages/message_1"},
		{"bookmark message", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages/message_1/bookmark"},
		{"update conversation config", stdhttp.MethodPut, "/api/v1/app/conversations/conversation_1/config"},
		{"convert conversation to task", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/convert-to-task"},
		{"create conversation share", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/share"},
		{"create message share", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages/message_1/share"},
		{"create persona", stdhttp.MethodPost, "/api/v1/app/personas"},
		{"update persona", stdhttp.MethodPut, "/api/v1/app/personas/persona_1"},
		{"delete persona", stdhttp.MethodDelete, "/api/v1/app/personas/persona_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"content":"hello","modelId":"quality-chat"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatPersonasDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	store := &chatFakeStore{}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	mux := stdhttp.NewServeMux()
	authMiddleware := newAuthMiddleware(testConfig(), auth.NewService(stubAuthStore{session: session}))
	registerChatRoutes(mux, authMiddleware, handler)
	router := authMiddleware.securityGuard(mux)
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"list personas", stdhttp.MethodGet, "/api/v1/app/personas"},
		{"create persona", stdhttp.MethodPost, "/api/v1/app/personas"},
		{"get persona", stdhttp.MethodGet, "/api/v1/app/personas/persona_1"},
		{"update persona", stdhttp.MethodPut, "/api/v1/app/personas/persona_1"},
		{"delete persona", stdhttp.MethodDelete, "/api/v1/app/personas/persona_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.lastPersonaID = ""
			store.lastWorkspaceID = ""
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Helpful Assistant","role":"Tutor"}`))
			request.Header.Set("Content-Type", "application/json")
			if !isSafeMethod(tt.method) {
				request.Header.Set(csrfHeaderName, csrfToken)
			}
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected Chat persona route to dispatch with 200 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
			if store.lastWorkspaceID != session.OrganizationID {
				t.Fatalf("expected persona route to use active organization scope %s, got %s", session.OrganizationID, store.lastWorkspaceID)
			}
			if strings.Contains(tt.path, "/persona_1") && store.lastPersonaID != "persona_1" {
				t.Fatalf("expected persona id persona_1, got %s", store.lastPersonaID)
			}
			if !strings.Contains(recorder.Body.String(), `"ok":true`) {
				t.Fatalf("expected success envelope, got %s", recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceChatPublicShareReadsDispatchWithoutSession(t *testing.T) {
	handler := newChatHandler(chat.NewService(&chatFakeStore{}, noopReplyGenerator{}, "demo-reply", nil))
	mux := stdhttp.NewServeMux()
	authMiddleware := newAuthMiddleware(testConfig(), auth.NewService(stubAuthStore{}))
	registerChatRoutes(mux, authMiddleware, handler)

	tests := []routeSurfaceCase{
		{"message share", stdhttp.MethodGet, "/api/v1/app/message-shares/msgshare_1"},
		{"conversation share", stdhttp.MethodGet, "/api/v1/app/conversation-shares/convshare_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			mux.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected public share route to dispatch without session for %s, got %d with body %s", tt.path, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"id":"`) {
				t.Fatalf("expected share response payload, got %s", recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKnowledgeMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create knowledge base", stdhttp.MethodPost, "/api/v1/app/knowledge-bases"},
		{"update knowledge base", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1"},
		{"delete knowledge base", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1"},
		{"create document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents"},
		{"upload document", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"update document", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"delete document", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"update chunk", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"retrieve", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve"},
		{"create retrieval test case", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases"},
		{"run retrieval test cases", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieval-test-cases/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Docs","title":"Plan","content":"hello","query":"hello","expectedResult":{"documentId":"doc_1","documentTitle":"Plan","snippet":"hello"},"splitAt":1,"direction":"next"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceKnowledgeAliasMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	for _, tt := range routeSurfaceKnowledgeAliasMutationCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Docs","title":"Plan","content":"hello","query":"hello","expectedResult":{"documentId":"doc_1","documentTitle":"Plan","snippet":"hello"},"splitAt":1,"direction":"next"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for Knowledge alias %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkspaceAgentReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list agents", stdhttp.MethodGet, "/api/v1/app/agents"},
		{"get agent", stdhttp.MethodGet, "/api/v1/app/agents/agent_1"},
		{"list agent tools", stdhttp.MethodGet, "/api/v1/app/agents/agent_1/tools"},
		{"list agent conversations", stdhttp.MethodGet, "/api/v1/app/agents/agent_1/conversations"},
		{"get agent conversation", stdhttp.MethodGet, "/api/v1/app/agents/conversations/conversation_1"},
		{"list agent messages", stdhttp.MethodGet, "/api/v1/app/agents/conversations/conversation_1/messages"},
		{"list agent runs", stdhttp.MethodGet, "/api/v1/app/agents/conversations/conversation_1/runs"},
		{"get agent run", stdhttp.MethodGet, "/api/v1/app/agents/runs/run_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered workspace Agent read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkspaceAgentMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create agent", stdhttp.MethodPost, "/api/v1/app/agents"},
		{"update agent", stdhttp.MethodPut, "/api/v1/app/agents/agent_1"},
		{"delete agent", stdhttp.MethodDelete, "/api/v1/app/agents/agent_1"},
		{"create conversation", stdhttp.MethodPost, "/api/v1/app/agents/agent_1/conversations"},
		{"delete conversation", stdhttp.MethodDelete, "/api/v1/app/agents/conversations/conversation_1"},
		{"send message", stdhttp.MethodPost, "/api/v1/app/agents/conversations/conversation_1/messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Agent","content":"hello"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkspaceAgentMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create agent", stdhttp.MethodPost, "/api/v1/app/agents"},
		{"update agent", stdhttp.MethodPut, "/api/v1/app/agents/agent_1"},
		{"delete agent", stdhttp.MethodDelete, "/api/v1/app/agents/agent_1"},
		{"create conversation", stdhttp.MethodPost, "/api/v1/app/agents/agent_1/conversations"},
		{"delete conversation", stdhttp.MethodDelete, "/api/v1/app/agents/conversations/conversation_1"},
		{"send message", stdhttp.MethodPost, "/api/v1/app/agents/conversations/conversation_1/messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Agent","content":"hello"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered workspace Agent route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMemoryReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list memory documents", stdhttp.MethodGet, "/api/v1/app/memory/documents"},
		{"get memory document", stdhttp.MethodGet, "/api/v1/app/memory/documents/document_1"},
		{"list memory document chunks", stdhttp.MethodGet, "/api/v1/app/memory/documents/document_1/chunks"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Memory read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMemoryMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create memory document", stdhttp.MethodPost, "/api/v1/app/memory/documents"},
		{"update memory document", stdhttp.MethodPut, "/api/v1/app/memory/documents/document_1"},
		{"delete memory document", stdhttp.MethodDelete, "/api/v1/app/memory/documents/document_1"},
		{"search memory documents", stdhttp.MethodPost, "/api/v1/app/memory/search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"title":"Plan","content":"hello","query":"hello"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAgentMemoryMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create agent memory", stdhttp.MethodPost, "/api/v1/agent/memories"},
		{"import agent memories", stdhttp.MethodPost, "/api/v1/agent/memories/import"},
		{"update agent memory", stdhttp.MethodPatch, "/api/v1/agent/memories/memory_1"},
		{"delete agent memory", stdhttp.MethodDelete, "/api/v1/agent/memories/memory_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"content":"hello","memories":[{"content":"hello"}]}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAgentReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list agent tools", stdhttp.MethodGet, "/api/v1/agent/tools?agentId=agent_1"},
		{"get agent run", stdhttp.MethodGet, "/api/v1/agent/runs/run_1"},
		{"list agent memories", stdhttp.MethodGet, "/api/v1/agent/memories"},
		{"export agent memories", stdhttp.MethodGet, "/api/v1/agent/memories/export"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Agent read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMemoryMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create memory document", stdhttp.MethodPost, "/api/v1/app/memory/documents"},
		{"update memory document", stdhttp.MethodPut, "/api/v1/app/memory/documents/document_1"},
		{"delete memory document", stdhttp.MethodDelete, "/api/v1/app/memory/documents/document_1"},
		{"search memory documents", stdhttp.MethodPost, "/api/v1/app/memory/search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"title":"Plan","content":"hello","query":"hello"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Memory route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAgentMemoryMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create agent memory", stdhttp.MethodPost, "/api/v1/agent/memories"},
		{"import agent memories", stdhttp.MethodPost, "/api/v1/agent/memories/import"},
		{"update agent memory", stdhttp.MethodPatch, "/api/v1/agent/memories/memory_1"},
		{"delete agent memory", stdhttp.MethodDelete, "/api/v1/agent/memories/memory_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"content":"hello","memories":[{"content":"hello"}]}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Agent memory route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceRefreshesWorkflowHealthBeforeMetricsScrape(t *testing.T) {
	source := routeSurfaceReadSourceFile(t, "router.go")

	text := string(source)
	if !strings.Contains(text, `mux.Handle("/metrics", workflowMetricsHandler(workflowService))`) {
		t.Fatal("expected /metrics to use workflowMetricsHandler")
	}
	if !strings.Contains(text, "RefreshExecutionHealthMetrics") {
		t.Fatal("expected workflowMetricsHandler to refresh workflow execution health metrics before scrape")
	}
}

func TestRouteSurfaceWiresConfiguredWorkflowSystemLimits(t *testing.T) {
	routerSource := routeSurfaceReadSourceFile(t, "router.go")
	serverSource := routeSurfaceReadSourceFile(t, "server.go")
	serviceSource := routeSurfaceReadSourceFile(t, "workflow_service.go")

	if !strings.Contains(string(routerSource), "workflowService = newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected default router workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
	}
	if !strings.Contains(string(serverSource), "workflowService := newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected server workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
	}
	if !strings.Contains(string(routerSource), "registerWorkflowAgentExecutor(workflowService, agentService)") {
		t.Fatal("expected default router workflow service to register the Agent node executor after Agent service construction")
	}
	if !strings.Contains(string(serverSource), "registerWorkflowAgentExecutor(workflowService, agentService)") {
		t.Fatal("expected server workflow service to register the Agent node executor after Agent service construction")
	}
	serviceText := string(serviceSource)
	if !strings.Contains(serviceText, "workflow.WithSystemWorkflowLimits") ||
		!strings.Contains(serviceText, "cfg.WorkflowSystemMaxConcurrent") ||
		!strings.Contains(serviceText, "cfg.WorkflowGlobalMaxExecutionsPerMinute") ||
		!strings.Contains(serviceText, "workflow.WithFailurePauseNotificationSink") ||
		!strings.Contains(serviceText, "workflowFailurePauseAlertEvent") {
		t.Fatal("expected workflow service helper to pass configured system workflow limits, failure notification sink, and alert event adapter")
	}
}

func TestRouteSurfaceAdminRoutesRequireAdmin(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	cookie := routeSurfaceRegisterUser(t, router, "route-user@example.com")

	adminRoutes := []string{
		"/api/v1/admin/stats",
		"/api/v1/admin/models",
		"/api/v1/admin/channels",
		"/api/v1/admin/routes",
		"/api/v1/admin/plans",
		"/api/v1/admin/users",
		"/api/v1/admin/organizations",
		"/api/v1/admin/audit-logs",
		"/api/v1/admin/reviews",
		"/api/v1/admin/usage-logs",
		"/api/v1/admin/usage-analytics",
		"/api/v1/admin/billing/reconciliation/relay-usage-prices",
		"/api/v1/admin/billing/reconciliation/usage-request-logs",
		"/api/v1/admin/release-evidence/rag-indexing",
		"/api/v1/admin/release-evidence/relay-realtime",
		"/api/v1/admin/release-evidence/relay-batch",
		"/api/v1/admin/release-evidence/marketplace-payout",
		"/api/v1/admin/release-evidence/marketplace-governance",
		"/api/v1/admin/release-evidence/provider-runtime-config",
		"/api/v1/admin/release-evidence/microservice-database",
		"/api/v1/admin/pricing/relay-catalog/imports",
		"/api/v1/admin/pricing/relay-catalog/sync",
		"/api/v1/admin/pricing/relay-catalog/sync-runs",
	}

	for _, path := range adminRoutes {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodGet, path, nil)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected 403 for normal user on %s, got %d with body %s", path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSubRoutesRequireAdminWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	for _, tt := range routeSurfaceAdminSubRouteCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected 401 for anonymous %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSubRoutesRejectNonAdminWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	for _, tt := range routeSurfaceAdminSubRouteCases() {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected 403 for non-admin %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSettingsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"relay pricing settings", stdhttp.MethodPut, "/api/v1/admin/settings/relay-pricing"},
		{"usage limit settings", stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminAPITokenRevokeRejectsAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/api-tokens/tok_1/revoke", nil)
	request.AddCookie(cookie)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403 for admin API token revoke, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceAdminBillingPayoutMutationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create due payouts", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/create-due"},
		{"mark payout paid", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/paid"},
		{"mark payout failed", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"providerPayoutID":"provider_1","reason":"operator check"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminSettingsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	session.OrganizationID = "org_settings_route"
	store := &fakeAdminStore{
		relayPricingSettings: admin.RelayPricingSettings{
			GroupMultipliers: map[string]float64{"standard": 1},
			ModelMultipliers: map[string]float64{"gpt-4o-mini": 0.5},
		},
	}
	quotaService := &fakeAdminQuotaSettingsService{
		settings: []quota.UsageLimitSettings{{
			OrganizationID:        "org_settings_route",
			QuotaMode:             "organization",
			MaxConcurrentRequests: 4,
			WindowSeconds:         60,
			MaxTokensPerWindow:    1000,
			MaxTokensPerRequest:   250,
		}},
	}
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AdminQuotaSettingsService: quotaService,
		AdminService:              admin.NewService(store),
		AuthStore:                 stubAuthStore{session: session},
	})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	getPricingRecorder := httptest.NewRecorder()
	getPricingRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/settings/relay-pricing", nil)
	getPricingRequest.AddCookie(cookie)
	router.ServeHTTP(getPricingRecorder, getPricingRequest)
	if getPricingRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected relay pricing settings GET to dispatch, got %d with body %s", getPricingRecorder.Code, getPricingRecorder.Body.String())
	}
	if !strings.Contains(getPricingRecorder.Body.String(), `"gpt-4o-mini":0.5`) {
		t.Fatalf("expected relay pricing settings response, got %s", getPricingRecorder.Body.String())
	}

	updatePricingRecorder := httptest.NewRecorder()
	updatePricingRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/settings/relay-pricing", strings.NewReader(`{
		"modelMultipliers": {"gpt-4o": 1.25},
		"groupMultipliers": {"vip": 0.75}
	}`))
	updatePricingRequest.Header.Set("Content-Type", "application/json")
	updatePricingRequest.Header.Set(csrfHeaderName, csrfToken)
	updatePricingRequest.AddCookie(cookie)
	router.ServeHTTP(updatePricingRecorder, updatePricingRequest)
	if updatePricingRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected relay pricing settings PUT to dispatch, got %d with body %s", updatePricingRecorder.Code, updatePricingRecorder.Body.String())
	}
	if store.relayPricingSettings.ModelMultipliers["gpt-4o"] != 1.25 || store.relayPricingSettings.GroupMultipliers["vip"] != 0.75 {
		t.Fatalf("expected relay pricing settings to be saved through router, got %+v", store.relayPricingSettings)
	}

	getUsageRecorder := httptest.NewRecorder()
	getUsageRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/settings/usage-limits", nil)
	getUsageRequest.AddCookie(cookie)
	router.ServeHTTP(getUsageRecorder, getUsageRequest)
	if getUsageRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage limits GET to dispatch, got %d with body %s", getUsageRecorder.Code, getUsageRecorder.Body.String())
	}
	if quotaService.listOrganizationID != "org_settings_route" ||
		!strings.Contains(getUsageRecorder.Body.String(), `"usageLimits"`) ||
		!strings.Contains(getUsageRecorder.Body.String(), `"maxTokensPerRequest":250`) {
		t.Fatalf("expected usage limits response scoped to session org, listOrg=%q body=%s", quotaService.listOrganizationID, getUsageRecorder.Body.String())
	}

	updateUsageRecorder := httptest.NewRecorder()
	updateUsageRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits", strings.NewReader(`{
		"organizationId": "org_untrusted",
		"userId": "user_settings_route",
		"quotaMode": "user",
		"maxConcurrentRequests": 2,
		"windowSeconds": 30,
		"maxTokensPerWindow": 500,
		"maxTokensPerRequest": 100
	}`))
	updateUsageRequest.Header.Set("Content-Type", "application/json")
	updateUsageRequest.Header.Set(csrfHeaderName, csrfToken)
	updateUsageRequest.AddCookie(cookie)
	router.ServeHTTP(updateUsageRecorder, updateUsageRequest)
	if updateUsageRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected usage limits PUT to dispatch, got %d with body %s", updateUsageRecorder.Code, updateUsageRecorder.Body.String())
	}
	if quotaService.saved.OrganizationID != "org_settings_route" ||
		quotaService.saved.UserID != "user_settings_route" ||
		quotaService.saved.QuotaMode != "user" ||
		quotaService.saved.MaxTokensPerRequest != 100 {
		t.Fatalf("expected usage limit save to preserve user fields and force session organization, got %+v", quotaService.saved)
	}
}

func TestRouteSurfaceKeepsAuthRoutesPublic(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"public-auth@example.com","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public register to return 200, got %d with body %s", registerRecorder.Code, registerRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"public-auth@example.com","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public login to return 200, got %d with body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestRouteSurfaceAuthPasswordResetPublicRoutesWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"request password reset", stdhttp.MethodPost, "/api/v1/auth/password-reset/request"},
		{"confirm password reset", stdhttp.MethodPost, "/api/v1/auth/password-reset/confirm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{`))
			request.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusBadRequest {
				t.Fatalf("expected public password reset route to dispatch to JSON validation with 400 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAuthMeRequiresSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected auth me to require a session with 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceRejectsCookieMutationWithoutCSRF(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	cookie, _ := routeSurfaceRegisterUserWithCSRF(t, router, "csrf-user@example.com")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(cookie)
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfacePublishingChannelReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list channels", stdhttp.MethodGet, "/api/v1/channels"},
		{"get channel", stdhttp.MethodGet, "/api/v1/channels/channel_1"},
		{"list channel messages", stdhttp.MethodGet, "/api/v1/channels/channel_1/messages"},
		{"list failed channel messages", stdhttp.MethodGet, "/api/v1/channels/channel_1/failed-messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected publishing channel read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfacePublishingChannelMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create channel", stdhttp.MethodPost, "/api/v1/channels"},
		{"update channel", stdhttp.MethodPut, "/api/v1/channels/channel_1"},
		{"delete channel", stdhttp.MethodDelete, "/api/v1/channels/channel_1"},
		{"update channel status", stdhttp.MethodPatch, "/api/v1/channels/channel_1/status"},
		{"test channel", stdhttp.MethodPost, "/api/v1/channels/channel_1/test"},
		{"send channel message", stdhttp.MethodPost, "/api/v1/channels/channel_1/send"},
		{"retry failed messages", stdhttp.MethodPost, "/api/v1/channels/channel_1/retry-failed-messages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminObservabilityMutationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"update alert routing", stdhttp.MethodPut, "/api/v1/admin/observability/alert-routing"},
		{"create alert provider", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers"},
		{"update alert provider", stdhttp.MethodPut, "/api/v1/admin/observability/alert-providers/provider_1"},
		{"test alert provider", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers/provider_1/test"},
		{"acknowledge alert", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/acknowledge"},
		{"resolve alert", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/resolve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminMarketplaceGovernanceMutationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"takedown agent", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/takedown"},
		{"reinstate agent", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/reinstate"},
		{"reject agent appeal", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/reject-appeal"},
		{"resolve abuse report", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/resolve"},
		{"dismiss abuse report", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/dismiss"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminMarketplaceReviewMutationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"enforce review sla", stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce"},
		{"approve review", stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/approve"},
		{"claim review", stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/claim"},
		{"reject review", stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/reject"},
		{"request review changes", stdhttp.MethodPost, "/api/v1/admin/reviews/agent_1/needs-changes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAdminOrganizationMutationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create organization", stdhttp.MethodPost, "/api/v1/admin/organizations"},
		{"update organization", stdhttp.MethodPut, "/api/v1/admin/organizations/org_1"},
		{"archive organization", stdhttp.MethodPost, "/api/v1/admin/organizations/org_1/archive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Acme","slug":"acme"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMCPReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list local mcp servers", stdhttp.MethodGet, "/api/v1/app/mcp-local-servers"},
		{"list mcp servers", stdhttp.MethodGet, "/api/v1/app/mcp-servers"},
		{"get mcp server", stdhttp.MethodGet, "/api/v1/app/mcp-servers/mcp_1"},
		{"list mcp tools", stdhttp.MethodGet, "/api/v1/app/mcp-servers/mcp_1/tools"},
		{"get mcp status", stdhttp.MethodGet, "/api/v1/app/mcp-servers/mcp_1/status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered MCP read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMCPMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"add mcp server", stdhttp.MethodPost, "/api/v1/app/mcp-servers"},
		{"delete mcp server", stdhttp.MethodDelete, "/api/v1/app/mcp-servers/mcp_1"},
		{"connect mcp server", stdhttp.MethodPost, "/api/v1/app/mcp-servers/mcp_1/connect"},
		{"disconnect mcp server", stdhttp.MethodPost, "/api/v1/app/mcp-servers/mcp_1/disconnect"},
		{"execute mcp tool", stdhttp.MethodPost, "/api/v1/app/mcp-servers/mcp_1/execute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceMarketplaceUserMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"publish agent", stdhttp.MethodPost, "/api/v1/marketplace/agents"},
		{"update agent", stdhttp.MethodPut, "/api/v1/marketplace/agents/agent_1"},
		{"delete agent", stdhttp.MethodDelete, "/api/v1/marketplace/agents/agent_1"},
		{"install agent", stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_1/install"},
		{"uninstall agent by detail", stdhttp.MethodDelete, "/api/v1/marketplace/agents/agent_1/install"},
		{"uninstall agent by installs", stdhttp.MethodDelete, "/api/v1/marketplace/installs/agent_1"},
		{"submit review", stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_1/reviews"},
		{"appeal agent", stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_1/appeal"},
		{"report abuse", stdhttp.MethodPost, "/api/v1/marketplace/agents/agent_1/abuse-reports"},
		{"update settlement preferences", stdhttp.MethodPut, "/api/v1/marketplace/publisher/settlement-preferences"},
		{"create template", stdhttp.MethodPost, "/api/v1/marketplace/templates"},
		{"install template", stdhttp.MethodPost, "/api/v1/marketplace/templates/template_1/install"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceAgentRunMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create run", stdhttp.MethodPost, "/api/v1/agent/runs"},
		{"approve tool", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-tool"},
		{"reject tool", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/reject-tool"},
		{"retry tool", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-tool"},
		{"continue budget", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-budget"},
		{"adjust plan", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/adjust-plan"},
		{"continue plan", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/continue-plan"},
		{"approve plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/approve-plan-step"},
		{"execute plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/execute-plan-step"},
		{"skip plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/skip-plan-step"},
		{"retry plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/retry-plan-step"},
		{"update plan step", stdhttp.MethodPatch, "/api/v1/agent/runs/run_1/update-plan-step"},
		{"create plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/create-plan-step"},
		{"move plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/move-plan-step"},
		{"delete plan step", stdhttp.MethodPost, "/api/v1/agent/runs/run_1/delete-plan-step"},
		{"workspace approve tool", stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/tool_run_1/approve"},
		{"workspace reject tool", stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/tool_run_1/reject"},
		{"workspace retry tool", stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/tool_run_1/retry"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceBillingCheckoutRejectsCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/billing/checkout", strings.NewReader(`{"kind":"topup","amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403 for POST /api/v1/billing/checkout, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceQuotaTopupRejectsCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/quota/topup", strings.NewReader(`{"amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected missing csrf to be rejected with 403 for POST /api/v1/app/quota/topup, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceQuotaReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"get quota", stdhttp.MethodGet, "/api/v1/app/quota"},
		{"list packages", stdhttp.MethodGet, "/api/v1/app/packages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Quota read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceQuotaTopupDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/quota/topup", strings.NewReader(`{"amount":25}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(csrfHeaderName, csrfToken)
	request.AddCookie(cookie)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusPaymentRequired {
		t.Fatalf("expected quota top-up compatibility route to dispatch and return 402, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRouteSurfaceTenantOrganizationReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list organizations", stdhttp.MethodGet, "/api/v1/app/organizations"},
		{"list organization members", stdhttp.MethodGet, "/api/v1/app/organizations/org_1/members"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Tenant organization read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceTenantOrganizationMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"select organization", stdhttp.MethodPost, "/api/v1/app/organizations/org_1/select"},
		{"update member role", stdhttp.MethodPut, "/api/v1/app/organizations/org_1/members/user_1"},
		{"remove member", stdhttp.MethodDelete, "/api/v1/app/organizations/org_1/members/user_1"},
		{"invite member", stdhttp.MethodPost, "/api/v1/app/organizations/org_1/invitations"},
		{"revoke invitation", stdhttp.MethodPost, "/api/v1/app/organizations/org_1/invitations/invitation_1/revoke"},
		{"transfer ownership", stdhttp.MethodPost, "/api/v1/app/organizations/org_1/ownership-transfer"},
		{"accept invitation", stdhttp.MethodPost, "/api/v1/app/organization-invitations/token_1/accept"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkflowReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{})

	tests := []routeSurfaceCase{
		{"list workflows", stdhttp.MethodGet, "/api/v1/workflows"},
		{"get workflow", stdhttp.MethodGet, "/api/v1/workflows/workflow_1"},
		{"list workflow versions", stdhttp.MethodGet, "/api/v1/workflows/workflow_1/versions"},
		{"list workflow executions", stdhttp.MethodGet, "/api/v1/workflows/workflow_1/executions"},
		{"get workflow execution", stdhttp.MethodGet, "/api/v1/workflows/workflow_1/executions/wexec_1"},
		{"get workflow execution debug snapshot", stdhttp.MethodGet, "/api/v1/workflows/workflow_1/executions/wexec_1/debug-snapshot"},
		{"get workflow execution state replay", stdhttp.MethodGet, "/api/v1/workflows/workflow_1/executions/wexec_1/state-replay"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Workflow read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkflowExecutionControlMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"resource check", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resource-check"},
		{"failure decision", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/decision"},
		{"pause execution", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/pause"},
		{"resume execution", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resume"},
		{"cancel execution", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/cancel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkflowManagementMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create workflow", stdhttp.MethodPost, "/api/v1/workflows"},
		{"semantic matches", stdhttp.MethodPost, "/api/v1/workflows/semantic-matches"},
		{"conversation matches", stdhttp.MethodPost, "/api/v1/workflows/conversation-matches"},
		{"debug retention prune", stdhttp.MethodPost, "/api/v1/workflows/debug-retention/prune"},
		{"update workflow", stdhttp.MethodPut, "/api/v1/workflows/workflow_1"},
		{"delete workflow", stdhttp.MethodDelete, "/api/v1/workflows/workflow_1"},
		{"execute workflow", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/execute"},
		{"session webhook", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/webhook"},
		{"create branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches"},
		{"publish branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/branch_1/publish"},
		{"merge branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/branch_1/merge"},
		{"rollback workflow", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/rollback"},
		{"test node", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/test-node"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Workflow","definition":{"nodes":[{"id":"start"}]},"message":"hello","conversationId":"conversation_1","version":1,"nodeId":"start"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceWorkflowManagementMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create workflow", stdhttp.MethodPost, "/api/v1/workflows"},
		{"semantic matches", stdhttp.MethodPost, "/api/v1/workflows/semantic-matches"},
		{"conversation matches", stdhttp.MethodPost, "/api/v1/workflows/conversation-matches"},
		{"debug retention prune", stdhttp.MethodPost, "/api/v1/workflows/debug-retention/prune"},
		{"update workflow", stdhttp.MethodPut, "/api/v1/workflows/workflow_1"},
		{"delete workflow", stdhttp.MethodDelete, "/api/v1/workflows/workflow_1"},
		{"execute workflow", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/execute"},
		{"session webhook", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/webhook"},
		{"create branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches"},
		{"publish branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/branch_1/publish"},
		{"merge branch", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/branch_1/merge"},
		{"rollback workflow", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/rollback"},
		{"test node", stdhttp.MethodPost, "/api/v1/workflows/workflow_1/test-node"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Workflow","definition":{"nodes":[{"id":"start"}]},"message":"hello","conversationId":"conversation_1","version":1,"nodeId":"start"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Workflow route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceScheduledTaskMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	scheduleService := schedule.NewService(&scheduleRouteFakeStore{})
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore:       stubAuthStore{session: session},
		ScheduleService: scheduleService,
	})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create scheduled task", stdhttp.MethodPost, "/api/v1/scheduled-tasks"},
		{"update scheduled task status", stdhttp.MethodPatch, "/api/v1/scheduled-tasks/sched_1/status"},
		{"run scheduled task now", stdhttp.MethodPost, "/api/v1/scheduled-tasks/sched_1/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Daily workflow","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 9 * * *","enabled":true}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceScheduledTaskReadRoutesRequireSessionWithoutDatabase(t *testing.T) {
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		ScheduleService: schedule.NewService(&scheduleRouteFakeStore{}),
	})

	tests := []routeSurfaceCase{
		{"list scheduled tasks", stdhttp.MethodGet, "/api/v1/scheduled-tasks"},
		{"list scheduled task runs", stdhttp.MethodGet, "/api/v1/scheduled-tasks/sched_1/runs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected registered Scheduled Task read route to require session with 401 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceScheduledTaskMutationsDispatchWithCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	store := &scheduleRouteFakeStore{
		gotTask: schedule.ScheduledTask{
			ID:             "sched_1",
			OrganizationID: session.OrganizationID,
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        false,
		},
		recordedRun: schedule.ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  session.OrganizationID,
			ScheduledTaskID: "sched_1",
			Status:          schedule.RunStatusQueued,
		},
	}
	scheduleService := schedule.NewService(store)
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore:       stubAuthStore{session: session},
		ScheduleService: scheduleService,
	})
	cookie := routeSurfaceSignedSessionCookie(t, session)
	csrfToken := routeSurfaceCSRFToken(session)

	tests := []routeSurfaceCase{
		{"create scheduled task", stdhttp.MethodPost, "/api/v1/scheduled-tasks"},
		{"update scheduled task status", stdhttp.MethodPatch, "/api/v1/scheduled-tasks/sched_1/status"},
		{"run scheduled task now", stdhttp.MethodPost, "/api/v1/scheduled-tasks/sched_1/run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"name":"Daily workflow","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 9 * * *","enabled":true}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(csrfHeaderName, csrfToken)
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			switch recorder.Code {
			case stdhttp.StatusUnauthorized, stdhttp.StatusForbidden, stdhttp.StatusNotFound, stdhttp.StatusMethodNotAllowed:
				t.Fatalf("expected registered Scheduled Task route to pass auth/csrf and dispatch for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}

	if store.createdInput.OrganizationID != session.OrganizationID || store.createdInput.Name != "Daily workflow" {
		t.Fatalf("expected create route to dispatch with session org and normalized name, got %+v", store.createdInput)
	}
	if store.updateEnabledTaskID != "sched_1" || !store.updateEnabledInput.Enabled {
		t.Fatalf("expected status route to dispatch update for sched_1, task=%q input=%+v", store.updateEnabledTaskID, store.updateEnabledInput)
	}
	if store.recordedRunInput.ScheduledTaskID != "sched_1" || store.recordedRunInput.Status != schedule.RunStatusRunning {
		t.Fatalf("expected run-now route to dispatch running run for sched_1, got %+v", store.recordedRunInput)
	}
}

func routeSurfaceRegisterUser(t *testing.T, router stdhttp.Handler, email string) *stdhttp.Cookie {
	t.Helper()

	cookie, _ := routeSurfaceRegisterUserWithCSRF(t, router, email)
	return cookie
}

func routeSurfaceRegisterUserWithCSRF(t *testing.T, router stdhttp.Handler, email string) (*stdhttp.Cookie, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"`+routeSurfaceStrongPassword+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("register test user: expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	var response struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &response)
	return cookies[0], response.Data.CSRFToken
}

type routeSurfaceCase struct {
	name   string
	method string
	path   string
}

type routeSurfaceManifest struct {
	Scope        PublicOperationScopeV1        `json:"scope"`
	Operations   []OperationContractMetadataV1 `json:"operations"`
	RouteSamples []routeSurfaceManifestSample  `json:"routeSamples"`
	Routes       []routeSurfaceManifestRoute   `json:"-"`
}

type routeSurfaceManifestRoute struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	SamplePath string `json:"samplePath"`
	Security   string `json:"security"`
}

type routeSurfaceManifestSample struct {
	Method         string `json:"method"`
	NormalizedPath string `json:"normalizedPath"`
	SamplePath     string `json:"samplePath"`
}

func loadRouteSurfaceManifest(t *testing.T) routeSurfaceManifest {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(routeSurfaceRepoRoot(t), "docs", "api", "route-surface-manifest.json"))
	if err != nil {
		t.Fatalf("read route surface manifest: %v", err)
	}

	var manifest routeSurfaceManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("parse route surface manifest: %v", err)
	}
	if manifest.Scope.SchemaVersion != "public-operation-scope/v1" || len(manifest.Operations) == 0 || len(manifest.RouteSamples) == 0 {
		t.Fatal("expected route surface manifest v2 scope, operations, and route samples")
	}

	samples := make(map[string]string, len(manifest.RouteSamples))
	for _, sample := range manifest.RouteSamples {
		key := routeSurfaceKey(sample.Method, sample.NormalizedPath)
		if _, exists := samples[key]; exists {
			t.Fatalf("duplicate route surface sample %s", key)
		}
		samples[key] = sample.SamplePath
	}
	for _, operation := range manifest.Operations {
		key := routeSurfaceKey(operation.Method, operation.NormalizedPath)
		samplePath, ok := samples[key]
		if !ok || samplePath == "" {
			t.Fatalf("route surface operation %s has no sample path", key)
		}
		manifest.Routes = append(manifest.Routes, routeSurfaceManifestRoute{
			Method: operation.Method, Path: operation.NormalizedPath, SamplePath: samplePath, Security: operation.Security,
		})
	}
	sort.Slice(manifest.Routes, func(i, j int) bool {
		return routeSurfaceKey(manifest.Routes[i].Method, manifest.Routes[i].Path) < routeSurfaceKey(manifest.Routes[j].Method, manifest.Routes[j].Path)
	})
	return manifest
}

func TestRouteSurfaceRuntimeDescriptorParityContract(t *testing.T) {
	manifest := loadRouteSurfaceManifest(t)
	expected := routeSurfaceRouterOwnedManifestOperations(t, manifest)
	adminSession := routeSurfaceAdminSession()
	var productionRegistrar *RouteSurfaceRegistrar
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore: stubAuthStore{session: adminSession},
		RouteSurfaceRegistrarFactory: func(mux *stdhttp.ServeMux, policies RouteSurfacePolicies) (*RouteSurfaceRegistrar, error) {
			registrar, err := NewRouteSurfaceRegistrar(mux, policies)
			productionRegistrar = registrar
			return registrar, err
		},
	})
	if productionRegistrar == nil {
		t.Fatal("expected production router to construct the route surface registrar")
	}
	if err := productionRegistrar.validateSnapshot(); err != nil {
		t.Fatalf("validate production route surface snapshot: %v", err)
	}
	allActual := productionRegistrar.Snapshot()
	expectedIDs := make(map[string]struct{}, len(expected))
	for _, operation := range expected {
		expectedIDs[operation.OperationID] = struct{}{}
	}
	actual := make([]RouteSurfaceDescriptor, 0, len(expected))
	for _, descriptor := range allActual {
		if _, ok := expectedIDs[descriptor.OperationID]; ok {
			actual = append(actual, descriptor)
		}
	}
	if len(actual) != len(expected) || len(actual) == 0 {
		t.Fatalf("expected nonempty router-owned descriptor parity, expected=%d actual=%d", len(expected), len(actual))
	}
	if len(productionRegistrar.mountedPatterns()) != len(allActual) {
		t.Fatalf("expected one exact mount per descriptor, mounts=%d descriptors=%d", len(productionRegistrar.mountedPatterns()), len(allActual))
	}
	observation, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, actual)
	if err != nil {
		t.Fatalf("compare production route surface snapshot: %v", err)
	}
	if observation.ParityResult != "pass" || observation.OperationCount != 3 || observation.MountedCount != 3 || observation.DescriptorCount != 3 || observation.CoreDigest == "" || observation.CoreDigest != observation.RuntimeDigest {
		t.Fatalf("unexpected production route surface observation: %+v", observation)
	}

	t.Run("exact production dispatch and auth", func(t *testing.T) {
		cookie := routeSurfaceSignedSessionCookie(t, adminSession)
		for _, operation := range expected {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(operation.Method, operation.NormalizedPath, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != stdhttp.StatusUnauthorized {
				t.Fatalf("expected anonymous %s %s to be rejected with 401, got %d", operation.Method, operation.NormalizedPath, recorder.Code)
			}

			recorder = httptest.NewRecorder()
			request = httptest.NewRequest(operation.Method, operation.NormalizedPath, nil)
			request.AddCookie(cookie)
			router.ServeHTTP(recorder, request)
			if recorder.Code == stdhttp.StatusUnauthorized || recorder.Code == stdhttp.StatusForbidden || routeSurfaceLooksUnregistered(recorder.Code, recorder.Body.String()) {
				t.Fatalf("expected authenticated %s %s to dispatch, got %d with body %s", operation.Method, operation.NormalizedPath, recorder.Code, recorder.Body.String())
			}
		}
	})

	t.Run("same source registration composes declared policies", func(t *testing.T) {
		mux := stdhttp.NewServeMux()
		var calls []string
		middleware := func(id string) RouteSurfaceMiddleware {
			return func(next stdhttp.Handler) stdhttp.Handler {
				return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					calls = append(calls, id)
					next.ServeHTTP(w, r)
				})
			}
		}
		policies := RouteSurfacePolicies{
			Auth:       map[RouteSurfaceAuth]RouteSurfaceMiddleware{RouteSurfaceAuthSession: middleware("auth")},
			Middleware: map[string]RouteSurfaceMiddleware{"audit": middleware("audit")},
			CSRF:       middleware("csrf"),
			Guard: func(effectID, capabilityID string, next stdhttp.Handler) stdhttp.Handler {
				return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					calls = append(calls, "guard:"+effectID+":"+capabilityID)
					next.ServeHTTP(w, r)
				})
			},
			AllowedCapabilities: map[string]struct{}{"test.contract": {}},
		}
		registrar, err := NewRouteSurfaceRegistrar(mux, policies)
		if err != nil {
			t.Fatalf("construct registrar: %v", err)
		}
		registration := RouteSurfaceRegistration{
			Method: stdhttp.MethodPost, Path: "/api/v1/contract/{contractId}", OperationID: "mutateContract",
			MiddlewareIDs: []string{"audit"}, Auth: RouteSurfaceAuthSession, CSRF: true,
			GuardEffectID: "contract.write", CapabilityID: "test.contract",
			Request:          MediaSchemaIdentityV1{MediaType: "application/json", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:request"}},
			SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "204", SchemaIdentity: routeSurfaceNoneSchema()}},
			Handler: stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
				calls = append(calls, "handler")
				w.WriteHeader(stdhttp.StatusNoContent)
			}),
		}
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register route: %v", err)
		}
		if err := registrar.validateSnapshot(); err != nil {
			t.Fatalf("validate route snapshot: %v", err)
		}
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/contract/contract_1", bytes.NewBufferString(`{}`)))
		if recorder.Code != stdhttp.StatusNoContent {
			t.Fatalf("expected exact subroute handler dispatch, got %d", recorder.Code)
		}
		wantCalls := "auth,csrf,audit,guard:contract.write:test.contract,handler"
		if strings.Join(calls, ",") != wantCalls {
			t.Fatalf("unexpected policy composition order: got %q want %q", strings.Join(calls, ","), wantCalls)
		}
		if err := registrar.Register(registration); routeSurfaceErrorCode(err) != "route_surface_duplicate" {
			t.Fatalf("expected duplicate registration rejection, got %v", err)
		}
		registration.Path = "/api/v1/contract/other"
		registration.CapabilityID = "test.unknown"
		if err := registrar.Register(registration); routeSurfaceErrorCode(err) != "route_surface_capability_unknown" {
			t.Fatalf("expected unknown capability rejection, got %v", err)
		}
	})

	t.Run("mount descriptor and inventory failures are closed", func(t *testing.T) {
		empty, err := NewRouteSurfaceRegistrar(stdhttp.NewServeMux(), RouteSurfacePolicies{AllowedCapabilities: map[string]struct{}{"test.contract": {}}})
		if err != nil {
			t.Fatalf("construct empty registrar: %v", err)
		}
		if code := routeSurfaceErrorCode(empty.validateSnapshot()); code != "route_surface_inventory_empty" {
			t.Fatalf("expected empty inventory failure, got %q", code)
		}
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, nil); routeSurfaceErrorCode(err) != "route_surface_inventory_empty" {
			t.Fatalf("expected empty comparison failure, got %v", err)
		}

		key := routeSurfaceKey(actual[0].Method, actual[0].Path)
		pattern := productionRegistrar.mounts[key]
		delete(productionRegistrar.mounts, key)
		if code := routeSurfaceErrorCode(productionRegistrar.validateSnapshot()); code != "route_surface_mount_descriptor_mismatch" {
			t.Fatalf("expected descriptor without mount failure, got %q", code)
		}
		productionRegistrar.mounts[key] = pattern
		productionRegistrar.mounts["GET /api/v1/extra"] = "GET /api/v1/extra"
		if code := routeSurfaceErrorCode(productionRegistrar.validateSnapshot()); code != "route_surface_mount_descriptor_mismatch" {
			t.Fatalf("expected mount without descriptor failure, got %q", code)
		}
		delete(productionRegistrar.mounts, "GET /api/v1/extra")
	})

	t.Run("excluded operation and metadata drift fail", func(t *testing.T) {
		excluded := cloneRouteSurfaceDescriptor(actual[0])
		excluded.Method = stdhttp.MethodGet
		excluded.Path = "/healthz"
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, append(actual, excluded)); routeSurfaceErrorCode(err) != "route_surface_excluded_registered" {
			t.Fatalf("expected excluded runtime registration rejection, got %v", err)
		}
		drift := append([]RouteSurfaceDescriptor(nil), actual...)
		drift[0] = cloneRouteSurfaceDescriptor(drift[0])
		drift[0].CapabilityID = "test.wrong"
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, drift); routeSurfaceErrorCode(err) != "route_surface_parity_failed" {
			t.Fatalf("expected descriptor drift failure, got %v", err)
		}
	})

	t.Run("snapshot is defensive and browser fields are impossible", func(t *testing.T) {
		first := productionRegistrar.Snapshot()
		first[0].MiddlewareIDs = append(first[0].MiddlewareIDs, "mutated")
		first[0].SuccessResponses[0].Status = "299"
		second := productionRegistrar.Snapshot()
		if reflect.DeepEqual(first, second) || second[0].SuccessResponses[0].Status == "299" {
			t.Fatal("expected registrar snapshot to be immutable through defensive copies")
		}
		typ := reflect.TypeOf(RouteSurfaceDescriptor{})
		for index := 0; index < typ.NumField(); index++ {
			name := strings.ToLower(typ.Field(index).Name)
			if strings.Contains(name, "encoder") || strings.Contains(name, "decoder") || strings.Contains(name, "event") {
				t.Fatalf("browser-owned field entered runtime descriptor: %s", typ.Field(index).Name)
			}
		}
		decoder := json.NewDecoder(bytes.NewBufferString(`{"method":"GET","responseDecoder":"browser"}`))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&RouteSurfaceDescriptor{}); err == nil {
			t.Fatal("expected copied browser field to fail strict descriptor decoding")
		}
	})

	t.Run("base media comparison accepts runtime charset only", func(t *testing.T) {
		expectedMarkdown := OperationContractMetadataV1{
			Method: stdhttp.MethodGet, NormalizedPath: "/api/v1/markdown", OperationID: "exportMarkdown", Security: "public", CapabilityID: "test.contract",
			Request:          MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
			SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "200", MediaType: "text/markdown", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:markdown"}}},
		}
		actualMarkdown := RouteSurfaceDescriptor{
			Method: expectedMarkdown.Method, Path: expectedMarkdown.NormalizedPath, OperationID: expectedMarkdown.OperationID,
			Auth: RouteSurfaceAuthPublic, Security: expectedMarkdown.Security, CapabilityID: expectedMarkdown.CapabilityID, Request: expectedMarkdown.Request,
			SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "200", MediaType: "text/markdown; charset=utf-8", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:markdown"}}},
		}
		scope := PublicOperationScopeV1{SchemaVersion: "public-operation-scope/v1", Dispositions: []PublicOperationDispositionV1{{Method: stdhttp.MethodGet, NormalizedPath: "/api/v1/markdown", Disposition: "included"}}}
		if observation, err := CompareRouteSurfaceSnapshot(scope, []OperationContractMetadataV1{expectedMarkdown}, []RouteSurfaceDescriptor{actualMarkdown}); err != nil || observation.CoreDigest != observation.RuntimeDigest {
			t.Fatalf("expected base markdown media parity, observation=%+v err=%v", observation, err)
		}
		actualMarkdown.SuccessResponses[0].MediaType = "text/plain; charset=utf-8"
		if _, err := CompareRouteSurfaceSnapshot(scope, []OperationContractMetadataV1{expectedMarkdown}, []RouteSurfaceDescriptor{actualMarkdown}); routeSurfaceErrorCode(err) != "route_surface_parity_failed" {
			t.Fatalf("expected wrong base media failure, got %v", err)
		}
	})

	t.Run("production router uses explicit registrar entrypoint", func(t *testing.T) {
		if count := routeSurfaceASTSelectorCallCount(t, "router.go", "Register"); count < len(expected) {
			t.Fatalf("expected at least %d explicit registrar calls, got %d", len(expected), count)
		}
	})
}

func routeSurfaceRouterOwnedManifestOperations(t *testing.T, manifest routeSurfaceManifest) []OperationContractMetadataV1 {
	t.Helper()
	wanted := make(map[string]struct{}, len(routerOwnedRouteSurfaceOperations()))
	for _, operation := range routerOwnedRouteSurfaceOperations() {
		wanted[routeSurfaceKey(operation.Method, operation.NormalizedPath)] = struct{}{}
	}
	result := make([]OperationContractMetadataV1, 0, len(wanted))
	for _, operation := range manifest.Operations {
		if _, ok := wanted[routeSurfaceKey(operation.Method, operation.NormalizedPath)]; ok {
			result = append(result, operation)
		}
	}
	if len(result) != len(wanted) {
		t.Fatalf("expected %d router-owned manifest operations, got %d", len(wanted), len(result))
	}
	return result
}

func routeSurfaceASTSelectorCallCount(t *testing.T, sourceFile, selectorName string) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, routeSurfaceSourceFile(t, sourceFile), nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", sourceFile, err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == selectorName {
			count++
		}
		return true
	})
	return count
}

func TestRouteSurfaceSnapshotContract(t *testing.T) {
	registrar, mux, scope, expected, registrations, handlerDispatchCount := routeSurfaceSnapshotFixture(t)
	registrationCount := len(registrations)
	snapshot := registrar.Snapshot()
	descriptorCount := len(snapshot)
	caseCount := 0
	run := func(name string, test func(*testing.T)) {
		caseCount++
		t.Run(name, test)
	}

	run("baseline snapshot is nonempty deterministic and independently comparable", func(t *testing.T) {
		if registrationCount != 2 || descriptorCount != registrationCount {
			t.Fatalf("unexpected snapshot evidence counts: registrations=%d descriptors=%d", registrationCount, descriptorCount)
		}
		if err := registrar.validateSnapshot(); err != nil {
			t.Fatalf("validate baseline snapshot: %v", err)
		}
		observation, err := CompareRouteSurfaceSnapshot(scope, expected, snapshot)
		if err != nil {
			t.Fatalf("compare baseline snapshot: %v", err)
		}
		if observation.ParityResult != "pass" || observation.OperationCount != 2 || observation.MountedCount != 2 || observation.DescriptorCount != 2 || observation.MediaProbeCount == 0 {
			t.Fatalf("unexpected baseline observation: %+v", observation)
		}
		second := registrar.Snapshot()
		if !reflect.DeepEqual(snapshot, second) {
			t.Fatalf("snapshot order is not deterministic: first=%#v second=%#v", snapshot, second)
		}
	})

	run("exact handlers dispatch and wrong methods do not", func(t *testing.T) {
		for _, request := range []*stdhttp.Request{
			httptest.NewRequest(stdhttp.MethodGet, "/api/v1/snapshot/items/item_1", nil),
			httptest.NewRequest(stdhttp.MethodPost, "/api/v1/snapshot/items", bytes.NewBufferString(`{}`)),
		} {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != stdhttp.StatusNoContent {
				t.Fatalf("expected exact fixture dispatch for %s %s, got %d", request.Method, request.URL.Path, recorder.Code)
			}
		}
		before := *handlerDispatchCount
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/snapshot/items/item_1", nil))
		if recorder.Code != stdhttp.StatusMethodNotAllowed || *handlerDispatchCount != before {
			t.Fatalf("wrong method reached handler: status=%d before=%d after=%d", recorder.Code, before, *handlerDispatchCount)
		}
	})

	run("omitted extra and duplicate descriptors fail", func(t *testing.T) {
		if _, err := CompareRouteSurfaceSnapshot(scope, expected, snapshot[1:]); routeSurfaceErrorCode(err) != "route_surface_parity_failed" {
			t.Fatalf("expected omitted descriptor parity failure, got %v", err)
		}
		extra := cloneRouteSurfaceDescriptor(snapshot[0])
		extra.Path = "/api/v1/snapshot/extra"
		if _, err := CompareRouteSurfaceSnapshot(scope, expected, append(append([]RouteSurfaceDescriptor(nil), snapshot...), extra)); routeSurfaceErrorCode(err) != "route_surface_runtime_unknown" {
			t.Fatalf("expected extra descriptor failure, got %v", err)
		}
		duplicate := append(append([]RouteSurfaceDescriptor(nil), snapshot...), cloneRouteSurfaceDescriptor(snapshot[0]))
		if _, err := CompareRouteSurfaceSnapshot(scope, expected, duplicate); routeSurfaceErrorCode(err) != "route_surface_duplicate" {
			t.Fatalf("expected duplicate descriptor failure, got %v", err)
		}
	})

	run("descriptor and mount splits make snapshot empty", func(t *testing.T) {
		key := routeSurfaceKey(snapshot[0].Method, snapshot[0].Path)
		pattern := registrar.mounts[key]
		delete(registrar.mounts, key)
		if split := registrar.Snapshot(); len(split) != 0 {
			t.Fatalf("descriptor without mount produced %d snapshot rows", len(split))
		}
		registrar.mounts[key] = pattern

		registrar.mounts["GET /api/v1/snapshot/handler-only"] = "GET /api/v1/snapshot/handler-only"
		if split := registrar.Snapshot(); len(split) != 0 {
			t.Fatalf("mount without descriptor produced %d snapshot rows", len(split))
		}
		delete(registrar.mounts, "GET /api/v1/snapshot/handler-only")

		registrar.mounts[key] = "POST " + snapshot[0].Path
		if split := registrar.Snapshot(); len(split) != 0 {
			t.Fatalf("wrong exact mount pattern produced %d snapshot rows", len(split))
		}
		registrar.mounts[key] = pattern
		if err := registrar.validateSnapshot(); err != nil {
			t.Fatalf("restore fixture registrar: %v", err)
		}
	})

	run("registration policy mutations fail construction", func(t *testing.T) {
		if err := registrar.Register(registrations[0]); routeSurfaceErrorCode(err) != "route_surface_duplicate" {
			t.Fatalf("expected duplicate registration error, got %v", err)
		}
		mutations := []struct {
			name string
			code string
			edit func(*RouteSurfaceRegistration, *RouteSurfacePolicies)
		}{
			{"missing handler", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Handler = nil }},
			{"unknown middleware", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.MiddlewareIDs = []string{"unknown"} }},
			{"missing auth", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, policies *RouteSurfacePolicies) {
				delete(policies.Auth, RouteSurfaceAuthSession)
			}},
			{"csrf on safe method", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Method = stdhttp.MethodGet }},
			{"missing guard", "route_surface_policy_missing", func(_ *RouteSurfaceRegistration, policies *RouteSurfacePolicies) { policies.Guard = nil }},
			{"unknown capability", "route_surface_capability_unknown", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.CapabilityID = "snapshot.unknown" }},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := registrations[1]
				candidate.Path = "/api/v1/snapshot/mutation/" + strings.ReplaceAll(mutation.name, " ", "-")
				policies := routeSurfaceSnapshotPolicies(handlerDispatchCount)
				mutation.edit(&candidate, &policies)
				candidateRegistrar, err := NewRouteSurfaceRegistrar(stdhttp.NewServeMux(), policies)
				if err != nil {
					t.Fatalf("construct mutation registrar: %v", err)
				}
				if err := candidateRegistrar.Register(candidate); routeSurfaceErrorCode(err) != mutation.code {
					t.Fatalf("mutation %q error = %v, want %s", mutation.name, err, mutation.code)
				}
			})
		}
	})

	run("auth csrf capability request and response mutations fail parity", func(t *testing.T) {
		mutations := []struct {
			name string
			edit func([]RouteSurfaceDescriptor)
		}{
			{"operation", func(value []RouteSurfaceDescriptor) { value[0].OperationID = "wrongOperation" }},
			{"auth", func(value []RouteSurfaceDescriptor) {
				value[1].Auth = RouteSurfaceAuthPublic
				value[1].Security = "public"
			}},
			{"csrf", func(value []RouteSurfaceDescriptor) { value[1].CSRF = false; value[1].Security = "cookie" }},
			{"capability", func(value []RouteSurfaceDescriptor) { value[0].CapabilityID = "snapshot.wrong" }},
			{"request media", func(value []RouteSurfaceDescriptor) { value[1].Request.MediaType = "text/plain" }},
			{"request schema", func(value []RouteSurfaceDescriptor) { value[1].Request.SchemaIdentity.Value = "sha256:wrong" }},
			{"response status", func(value []RouteSurfaceDescriptor) { value[0].SuccessResponses[0].Status = "201" }},
			{"response media", func(value []RouteSurfaceDescriptor) { value[0].SuccessResponses[0].MediaType = "text/plain" }},
			{"response schema", func(value []RouteSurfaceDescriptor) {
				value[0].SuccessResponses[0].SchemaIdentity = SchemaIdentityV1{Kind: "inline", Value: "sha256:wrong"}
			}},
		}
		for _, mutation := range mutations {
			t.Run(mutation.name, func(t *testing.T) {
				candidate := routeSurfaceCloneDescriptors(snapshot)
				mutation.edit(candidate)
				if _, err := CompareRouteSurfaceSnapshot(scope, expected, candidate); routeSurfaceErrorCode(err) != "route_surface_parity_failed" {
					t.Fatalf("mutation %q error = %v, want route_surface_parity_failed", mutation.name, err)
				}
			})
		}
	})

	run("markdown base media accepts charset and rejects wrong base", func(t *testing.T) {
		operation := OperationContractMetadataV1{
			Method: stdhttp.MethodGet, NormalizedPath: "/api/v1/snapshot/export", OperationID: "exportSnapshot",
			Security: "public", CapabilityID: "snapshot.read", Request: MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
			SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "200", MediaType: "text/markdown", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:markdown"}}},
		}
		descriptor := RouteSurfaceDescriptor{
			Method: operation.Method, Path: operation.NormalizedPath, OperationID: operation.OperationID,
			Auth: RouteSurfaceAuthPublic, Security: operation.Security, CapabilityID: operation.CapabilityID, Request: operation.Request,
			SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "200", MediaType: "text/markdown; charset=utf-8", SchemaIdentity: operation.SuccessResponses[0].SchemaIdentity}},
		}
		markdownScope := PublicOperationScopeV1{SchemaVersion: "public-operation-scope/v1", Dispositions: []PublicOperationDispositionV1{{Method: operation.Method, NormalizedPath: operation.NormalizedPath, Disposition: "included"}}}
		if _, err := CompareRouteSurfaceSnapshot(markdownScope, []OperationContractMetadataV1{operation}, []RouteSurfaceDescriptor{descriptor}); err != nil {
			t.Fatalf("compatible markdown media failed: %v", err)
		}
		descriptor.SuccessResponses[0].MediaType = "application/json"
		if _, err := CompareRouteSurfaceSnapshot(markdownScope, []OperationContractMetadataV1{operation}, []RouteSurfaceDescriptor{descriptor}); routeSurfaceErrorCode(err) != "route_surface_parity_failed" {
			t.Fatalf("wrong markdown base media error = %v", err)
		}
	})

	run("excluded registration and zero inventory fail", func(t *testing.T) {
		excluded := cloneRouteSurfaceDescriptor(snapshot[0])
		excluded.Method = stdhttp.MethodGet
		excluded.Path = "/healthz"
		excludedScope := scope
		excludedScope.Dispositions = append(append([]PublicOperationDispositionV1(nil), scope.Dispositions...), PublicOperationDispositionV1{Method: stdhttp.MethodGet, NormalizedPath: "/healthz", Disposition: "excluded"})
		if _, err := CompareRouteSurfaceSnapshot(excludedScope, expected, append(routeSurfaceCloneDescriptors(snapshot), excluded)); routeSurfaceErrorCode(err) != "route_surface_excluded_registered" {
			t.Fatalf("excluded registration error = %v", err)
		}
		if _, err := CompareRouteSurfaceSnapshot(scope, nil, nil); routeSurfaceErrorCode(err) != "route_surface_inventory_empty" {
			t.Fatalf("zero inventory error = %v", err)
		}
	})

	run("browser fields and direct ServeMux bypass are independently rejected", func(t *testing.T) {
		decoder := json.NewDecoder(bytes.NewBufferString(`{"method":"GET","normalizedPath":"/api/v1/snapshot/items/{itemId}","responseDecoder":"browser"}`))
		decoder.DisallowUnknownFields()
		var descriptor RouteSurfaceDescriptor
		if err := decoder.Decode(&descriptor); err == nil {
			t.Fatal("strict runtime descriptor accepted browser responseDecoder")
		}
		directSource := `package fixture
import "net/http"
func register(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/snapshot/bypass", handler)
}`
		if count := routeSurfaceDirectHandleCount(t, directSource); count != 1 {
			t.Fatalf("direct ServeMux bypass fixture count = %d, want 1", count)
		}
		registrarSource := `package fixture
func register(registrar interface{ Register(any) error }, registration any) error {
	return registrar.Register(registration)
}`
		if count := routeSurfaceDirectHandleCount(t, registrarSource); count != 0 {
			t.Fatalf("registrar fixture was misclassified as %d direct Handle calls", count)
		}
	})

	if caseCount == 0 || registrationCount == 0 || *handlerDispatchCount == 0 || descriptorCount == 0 {
		t.Fatalf("snapshot suite discovery is vacuous: cases=%d registrations=%d handlerDispatches=%d descriptors=%d", caseCount, registrationCount, *handlerDispatchCount, descriptorCount)
	}
}

func routeSurfaceSnapshotFixture(t *testing.T) (*RouteSurfaceRegistrar, *stdhttp.ServeMux, PublicOperationScopeV1, []OperationContractMetadataV1, []RouteSurfaceRegistration, *int) {
	t.Helper()
	handlerDispatchCount := 0
	policies := routeSurfaceSnapshotPolicies(&handlerDispatchCount)
	mux := stdhttp.NewServeMux()
	registrar, err := NewRouteSurfaceRegistrar(mux, policies)
	if err != nil {
		t.Fatalf("construct snapshot fixture registrar: %v", err)
	}
	read := OperationContractMetadataV1{
		Method: stdhttp.MethodGet, NormalizedPath: "/api/v1/snapshot/items/{itemId}", OperationID: "getSnapshotItem",
		Security: "public", CapabilityID: "snapshot.read", Request: MediaSchemaIdentityV1{SchemaIdentity: routeSurfaceNoneSchema()},
		SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "204", SchemaIdentity: routeSurfaceNoneSchema()}},
	}
	write := OperationContractMetadataV1{
		Method: stdhttp.MethodPost, NormalizedPath: "/api/v1/snapshot/items", OperationID: "createSnapshotItem",
		Security: "cookie+csrf", CSRF: true, CapabilityID: "snapshot.write",
		Request:          MediaSchemaIdentityV1{MediaType: "application/json", SchemaIdentity: SchemaIdentityV1{Kind: "inline", Value: "sha256:request"}},
		SuccessResponses: []StatusMediaSchemaIdentityV1{{Status: "204", SchemaIdentity: routeSurfaceNoneSchema()}},
	}
	handler := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		handlerDispatchCount++
		w.WriteHeader(stdhttp.StatusNoContent)
	})
	registrations := []RouteSurfaceRegistration{
		routeSurfaceRegistrationFromOperation(read, RouteSurfaceAuthPublic, nil, "", handler),
		routeSurfaceRegistrationFromOperation(write, RouteSurfaceAuthSession, []string{"audit"}, "snapshot.write", handler),
	}
	for _, registration := range registrations {
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register snapshot fixture %s: %v", registration.OperationID, err)
		}
	}
	scope := PublicOperationScopeV1{
		SchemaVersion: "public-operation-scope/v1", MandatoryPrefixes: []string{"/api/"},
		Dispositions: []PublicOperationDispositionV1{
			{Method: read.Method, NormalizedPath: read.NormalizedPath, Disposition: "included"},
			{Method: write.Method, NormalizedPath: write.NormalizedPath, Disposition: "included"},
		},
	}
	return registrar, mux, scope, []OperationContractMetadataV1{read, write}, registrations, &handlerDispatchCount
}

func routeSurfaceSnapshotPolicies(handlerDispatchCount *int) RouteSurfacePolicies {
	pass := func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			next.ServeHTTP(w, r)
		})
	}
	return RouteSurfacePolicies{
		Auth: map[RouteSurfaceAuth]RouteSurfaceMiddleware{RouteSurfaceAuthSession: pass},
		Middleware: map[string]RouteSurfaceMiddleware{
			"audit": func(next stdhttp.Handler) stdhttp.Handler {
				return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
					_ = handlerDispatchCount
					next.ServeHTTP(w, r)
				})
			},
		},
		CSRF: pass,
		Guard: func(_, _ string, next stdhttp.Handler) stdhttp.Handler {
			return pass(next)
		},
		AllowedCapabilities: map[string]struct{}{"snapshot.read": {}, "snapshot.write": {}},
	}
}

func routeSurfaceCloneDescriptors(input []RouteSurfaceDescriptor) []RouteSurfaceDescriptor {
	result := make([]RouteSurfaceDescriptor, len(input))
	for index, descriptor := range input {
		result[index] = cloneRouteSurfaceDescriptor(descriptor)
	}
	return result
}

func routeSurfaceDirectHandleCount(t *testing.T, source string) int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, 0)
	if err != nil {
		t.Fatalf("parse direct Handle fixture: %v", err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && (selector.Sel.Name == "Handle" || selector.Sel.Name == "HandleFunc") {
			count++
		}
		return true
	})
	return count
}

func routeSurfaceRepoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(routeSurfaceHTTPSourceDir(t), "..", "..", "..", ".."))
}

func routeSurfaceHTTPSourceDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve route surface source directory")
	}
	return filepath.Dir(file)
}

func routeSurfaceSourceFile(t *testing.T, sourceFile string) string {
	t.Helper()
	if filepath.IsAbs(sourceFile) {
		return sourceFile
	}
	return filepath.Join(routeSurfaceHTTPSourceDir(t), sourceFile)
}

func routeSurfaceReadSourceFile(t *testing.T, sourceFile string) []byte {
	t.Helper()
	resolved := routeSurfaceSourceFile(t, sourceFile)
	content, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("read %s: %v", sourceFile, err)
	}
	return content
}

type routeSurfaceRuntimeRegistration struct {
	Path   string
	Source string
}

func loadRouteSurfaceRuntimeRegistrations(t *testing.T) []routeSurfaceRuntimeRegistration {
	t.Helper()

	sourceFiles := routeSurfaceRuntimeSourceFiles(t)
	registrationsBySource := map[string]routeSurfaceRuntimeRegistration{}
	for _, sourceFile := range sourceFiles {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, sourceFile, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", sourceFile, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			path, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.HasPrefix(path, "/api/") {
				return true
			}

			position := fset.Position(lit.Pos())
			source := fmt.Sprintf("%s:%d", filepath.Base(sourceFile), position.Line)
			registrationsBySource[source] = routeSurfaceRuntimeRegistration{Path: path, Source: source}
			return true
		})
	}

	registrations := make([]routeSurfaceRuntimeRegistration, 0, len(registrationsBySource))
	for _, registration := range registrationsBySource {
		registrations = append(registrations, registration)
	}
	sort.Slice(registrations, func(i, j int) bool {
		if registrations[i].Path == registrations[j].Path {
			return registrations[i].Source < registrations[j].Source
		}
		return registrations[i].Path < registrations[j].Path
	})
	if len(registrations) == 0 {
		t.Fatal("expected runtime route registrations")
	}
	return registrations
}

func routeSurfaceRuntimeSourceFiles(t *testing.T) []string {
	t.Helper()

	calledRegistrars := routeSurfaceCalledRegistrars(t, "router.go")
	sourceDir := routeSurfaceHTTPSourceDir(t)
	files := []string{filepath.Join(sourceDir, "router.go")}
	registrarFiles, err := filepath.Glob(filepath.Join(sourceDir, "routes_*.go"))
	if err != nil {
		t.Fatalf("glob route files: %v", err)
	}
	for _, sourceFile := range registrarFiles {
		if strings.HasSuffix(sourceFile, "_test.go") {
			continue
		}
		for registrar := range routeSurfaceDeclaredRegistrars(t, sourceFile) {
			if calledRegistrars[registrar] {
				files = append(files, sourceFile)
				break
			}
		}
	}
	sort.Strings(files)
	return files
}

func routeSurfaceCalledRegistrars(t *testing.T, sourceFile string) map[string]bool {
	t.Helper()

	sourceFile = routeSurfaceSourceFile(t, sourceFile)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourceFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", sourceFile, err)
	}
	called := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		if strings.HasPrefix(ident.Name, "register") && (strings.HasSuffix(ident.Name, "Routes") || strings.HasSuffix(ident.Name, "RouteSurfaces")) {
			called[ident.Name] = true
		}
		return true
	})
	return called
}

func routeSurfaceDeclaredRegistrars(t *testing.T, sourceFile string) map[string]bool {
	t.Helper()

	sourceFile = routeSurfaceSourceFile(t, sourceFile)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourceFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", sourceFile, err)
	}
	declared := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, "register") && strings.HasSuffix(fn.Name.Name, "Routes") {
			declared[fn.Name.Name] = true
		}
	}
	return declared
}

func routeSurfaceManifestCoversRuntimePath(runtimePath string, manifestPaths []string) bool {
	for _, manifestPath := range manifestPaths {
		if runtimePath == manifestPath {
			return true
		}
		if strings.HasSuffix(runtimePath, "/") && strings.HasPrefix(manifestPath, runtimePath) {
			return true
		}
	}
	return false
}

func routeSurfaceManifestHandler(session auth.Session) stdhttp.Handler {
	return combineHandlers(
		NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}}),
		stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
			writeError(w, stdhttp.StatusUnauthorized, "relay_identity_required", "relay identity required")
		}),
	)
}

func routeSurfaceManifestRequest(route routeSurfaceManifestRoute) *stdhttp.Request {
	body := strings.NewReader(`{}`)
	if route.Method == stdhttp.MethodGet || route.Method == stdhttp.MethodDelete {
		body = strings.NewReader("")
	}
	request := httptest.NewRequest(route.Method, route.SamplePath, body)
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestRouteSurfaceRuntimeAPIRoutesAreDocumentedInManifestWithoutDatabase(t *testing.T) {
	manifest := loadRouteSurfaceManifest(t)
	manifestPaths := make([]string, 0, len(manifest.Routes))
	for _, route := range manifest.Routes {
		manifestPaths = append(manifestPaths, route.Path)
	}
	sort.Strings(manifestPaths)

	var missing []string
	for _, registration := range loadRouteSurfaceRuntimeRegistrations(t) {
		if !routeSurfaceManifestCoversRuntimePath(registration.Path, manifestPaths) {
			missing = append(missing, fmt.Sprintf("%s registered at %s", registration.Path, registration.Source))
		}
	}
	if len(missing) > 0 {
		t.Fatalf("runtime API routes are missing route-surface manifest coverage:\n- %s", strings.Join(missing, "\n- "))
	}
}

func TestRouteSurfaceDeclaredRouteRegistrarsAreMountedWithoutDatabase(t *testing.T) {
	calledRegistrars := routeSurfaceCalledRegistrars(t, "router.go")
	registrarFiles, err := filepath.Glob("routes_*.go")
	if err != nil {
		t.Fatalf("glob route files: %v", err)
	}

	var missing []string
	for _, sourceFile := range registrarFiles {
		if strings.HasSuffix(sourceFile, "_test.go") {
			continue
		}
		for registrar := range routeSurfaceDeclaredRegistrars(t, sourceFile) {
			if calledRegistrars[registrar] {
				continue
			}
			explicitRegistrar := strings.TrimSuffix(registrar, "Routes") + "RouteSurfaces"
			if calledRegistrars[explicitRegistrar] {
				continue
			}
			missing = append(missing, fmt.Sprintf("%s or %s declared in %s", registrar, explicitRegistrar, sourceFile))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("declared route registrars must be mounted by NewRouterWithOptions:\n- %s", strings.Join(missing, "\n- "))
	}
}

func TestRouteSurfaceManifestRoutesAreRegisteredWithoutDatabase(t *testing.T) {
	manifest := loadRouteSurfaceManifest(t)

	userSession := routeSurfaceUserSession()
	adminSession := routeSurfaceAdminSession()
	router := routeSurfaceManifestHandler(userSession)
	userCookie := routeSurfaceSignedSessionCookie(t, userSession)
	adminCookie := routeSurfaceSignedSessionCookie(t, adminSession)
	userCSRF := routeSurfaceCSRFToken(userSession)
	adminCSRF := routeSurfaceCSRFToken(adminSession)

	for _, route := range manifest.Routes {
		route := route
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := routeSurfaceManifestRequest(route)

			switch route.Security {
			case "bearer":
				request.Header.Set("Authorization", "Bearer route_surface_token")
			case "cookie":
				if strings.HasPrefix(route.Path, "/api/v1/admin/") {
					request.AddCookie(adminCookie)
				} else {
					request.AddCookie(userCookie)
				}
			case "cookie+csrf":
				if strings.HasPrefix(route.Path, "/api/v1/admin/") {
					request.AddCookie(adminCookie)
					request.Header.Set("X-CSRF-Token", adminCSRF)
				} else {
					request.AddCookie(userCookie)
					request.Header.Set("X-CSRF-Token", userCSRF)
				}
			case "public":
			default:
				t.Fatalf("unsupported route manifest security %q for %s %s", route.Security, route.Method, route.Path)
			}

			router.ServeHTTP(recorder, request)

			if routeSurfaceLooksUnregistered(recorder.Code, recorder.Body.String()) {
				t.Fatalf("manifest route %s %s sample %s was not registered: got %d with body %s", route.Method, route.Path, route.SamplePath, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRouteSurfaceManifestSecurityGuardsWithoutDatabase(t *testing.T) {
	manifest := loadRouteSurfaceManifest(t)

	userSession := routeSurfaceUserSession()
	adminSession := routeSurfaceAdminSession()
	anonymousRouter := routeSurfaceManifestHandler(auth.Session{})
	userRouter := routeSurfaceManifestHandler(userSession)
	adminRouter := routeSurfaceManifestHandler(adminSession)
	userCookie := routeSurfaceSignedSessionCookie(t, userSession)
	adminCookie := routeSurfaceSignedSessionCookie(t, adminSession)
	userCSRF := routeSurfaceCSRFToken(userSession)
	checkedAnonymous := 0
	checkedCSRF := 0
	checkedAdmin := 0
	checkedBearer := 0

	for _, route := range manifest.Routes {
		route := route

		switch route.Security {
		case "cookie", "cookie+csrf":
			t.Run("anonymous "+route.Method+" "+route.Path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				anonymousRouter.ServeHTTP(recorder, routeSurfaceManifestRequest(route))

				if recorder.Code != stdhttp.StatusUnauthorized {
					t.Fatalf("expected anonymous protected route to return 401 for %s %s, got %d with body %s", route.Method, route.Path, recorder.Code, recorder.Body.String())
				}
			})
			checkedAnonymous++
		case "bearer":
			t.Run("missing bearer "+route.Method+" "+route.Path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				userRouter.ServeHTTP(recorder, routeSurfaceManifestRequest(route))

				if recorder.Code != stdhttp.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "relay_identity_required") {
					t.Fatalf("expected missing bearer route to return Relay 401 for %s %s, got %d with body %s", route.Method, route.Path, recorder.Code, recorder.Body.String())
				}
			})
			checkedBearer++
		case "public":
		default:
			t.Fatalf("unsupported route manifest security %q for %s %s", route.Security, route.Method, route.Path)
		}

		if route.Security == "cookie+csrf" {
			t.Run("missing csrf "+route.Method+" "+route.Path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := routeSurfaceManifestRequest(route)
				if strings.HasPrefix(route.Path, "/api/v1/admin/") {
					request.AddCookie(adminCookie)
					adminRouter.ServeHTTP(recorder, request)
				} else {
					request.AddCookie(userCookie)
					userRouter.ServeHTTP(recorder, request)
				}

				if recorder.Code != stdhttp.StatusForbidden {
					t.Fatalf("expected signed cookie without CSRF to return 403 for %s %s, got %d with body %s", route.Method, route.Path, recorder.Code, recorder.Body.String())
				}
			})
			checkedCSRF++
		}

		if strings.HasPrefix(route.Path, "/api/v1/admin/") && (route.Security == "cookie" || route.Security == "cookie+csrf") {
			t.Run("non admin "+route.Method+" "+route.Path, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				request := routeSurfaceManifestRequest(route)
				request.AddCookie(userCookie)
				if route.Security == "cookie+csrf" {
					request.Header.Set(csrfHeaderName, userCSRF)
				}

				userRouter.ServeHTTP(recorder, request)

				if recorder.Code != stdhttp.StatusForbidden {
					t.Fatalf("expected non-admin session to return 403 for %s %s, got %d with body %s", route.Method, route.Path, recorder.Code, recorder.Body.String())
				}
			})
			checkedAdmin++
		}
	}

	if checkedAnonymous == 0 || checkedCSRF == 0 || checkedAdmin == 0 || checkedBearer == 0 {
		t.Fatalf("security guard test did not cover all guard classes: anonymous=%d csrf=%d admin=%d bearer=%d", checkedAnonymous, checkedCSRF, checkedAdmin, checkedBearer)
	}
}

func TestRouteSurfaceManifestAdminRoutesDispatchWithAdminSessionWithoutDatabase(t *testing.T) {
	manifest := loadRouteSurfaceManifest(t)

	adminSession := routeSurfaceAdminSession()
	router := routeSurfaceManifestHandler(adminSession)
	adminCookie := routeSurfaceSignedSessionCookie(t, adminSession)
	adminCSRF := routeSurfaceCSRFToken(adminSession)
	checkedAdminRoutes := 0

	for _, route := range manifest.Routes {
		route := route
		if !strings.HasPrefix(route.Path, "/api/v1/admin/") || (route.Security != "cookie" && route.Security != "cookie+csrf") {
			continue
		}

		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := routeSurfaceManifestRequest(route)
			request.AddCookie(adminCookie)
			if route.Security == "cookie+csrf" {
				request.Header.Set(csrfHeaderName, adminCSRF)
			}

			router.ServeHTTP(recorder, request)

			if recorder.Code == stdhttp.StatusUnauthorized || recorder.Code == stdhttp.StatusForbidden || routeSurfaceLooksUnregistered(recorder.Code, recorder.Body.String()) {
				t.Fatalf("expected admin manifest route to pass auth/csrf and dispatch for %s %s sample %s, got %d with body %s", route.Method, route.Path, route.SamplePath, recorder.Code, recorder.Body.String())
			}
		})
		checkedAdminRoutes++
	}

	if checkedAdminRoutes == 0 {
		t.Fatal("expected manifest admin dispatch test to cover admin routes")
	}
}

func routeSurfaceLooksUnregistered(status int, body string) bool {
	if status == stdhttp.StatusMethodNotAllowed {
		return true
	}
	if status != stdhttp.StatusNotFound {
		return false
	}
	trimmed := strings.TrimSpace(body)
	return trimmed == "404 page not found" || strings.Contains(trimmed, `"message":"route not found"`)
}

func routeSurfaceAdminSubRouteCases() []routeSurfaceCase {
	return []routeSurfaceCase{
		{"billing summary", stdhttp.MethodGet, "/api/v1/admin/billing/summary"},
		{"billing sessions", stdhttp.MethodGet, "/api/v1/admin/billing/sessions"},
		{"billing payment intents", stdhttp.MethodGet, "/api/v1/admin/billing/payment-intents"},
		{"billing webhook events", stdhttp.MethodGet, "/api/v1/admin/billing/webhook-events"},
		{"billing subscriptions", stdhttp.MethodGet, "/api/v1/admin/billing/subscriptions"},
		{"billing topups", stdhttp.MethodGet, "/api/v1/admin/billing/topups"},
		{"billing invoices", stdhttp.MethodGet, "/api/v1/admin/billing/invoices"},
		{"billing refunds", stdhttp.MethodGet, "/api/v1/admin/billing/refunds"},
		{"billing settlements", stdhttp.MethodGet, "/api/v1/admin/billing/settlements"},
		{"billing payouts", stdhttp.MethodGet, "/api/v1/admin/billing/payouts"},
		{"billing relay usage price reconciliation", stdhttp.MethodGet, "/api/v1/admin/billing/reconciliation/relay-usage-prices"},
		{"billing usage request log coverage", stdhttp.MethodGet, "/api/v1/admin/billing/reconciliation/usage-request-logs"},
		{"release evidence rag indexing", stdhttp.MethodGet, "/api/v1/admin/release-evidence/rag-indexing"},
		{"release evidence relay realtime", stdhttp.MethodGet, "/api/v1/admin/release-evidence/relay-realtime"},
		{"release evidence relay batch", stdhttp.MethodGet, "/api/v1/admin/release-evidence/relay-batch"},
		{"release evidence marketplace payout", stdhttp.MethodGet, "/api/v1/admin/release-evidence/marketplace-payout"},
		{"release evidence marketplace governance", stdhttp.MethodGet, "/api/v1/admin/release-evidence/marketplace-governance"},
		{"release evidence provider runtime config", stdhttp.MethodGet, "/api/v1/admin/release-evidence/provider-runtime-config"},
		{"release evidence microservice database", stdhttp.MethodGet, "/api/v1/admin/release-evidence/microservice-database"},
		{"billing topup refund", stdhttp.MethodPost, "/api/v1/admin/billing/topups/topup_1/refund"},
		{"billing payout create due", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/create-due"},
		{"billing payout paid", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/paid"},
		{"billing payout failed", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/failed"},
		{"pricing relay catalog imports list", stdhttp.MethodGet, "/api/v1/admin/pricing/relay-catalog/imports"},
		{"pricing relay catalog imports create", stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports"},
		{"pricing relay catalog sync", stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/sync"},
		{"pricing relay catalog sync runs list", stdhttp.MethodGet, "/api/v1/admin/pricing/relay-catalog/sync-runs"},
		{"pricing relay catalog import approve", stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/rpci_1/approve"},
		{"pricing relay catalog import reject", stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/rpci_1/reject"},
		{"pricing relay catalog import rollback", stdhttp.MethodPost, "/api/v1/admin/pricing/relay-catalog/imports/rpci_1/rollback"},
		{"settings relay pricing get", stdhttp.MethodGet, "/api/v1/admin/settings/relay-pricing"},
		{"settings relay pricing update", stdhttp.MethodPut, "/api/v1/admin/settings/relay-pricing"},
		{"settings usage limits get", stdhttp.MethodGet, "/api/v1/admin/settings/usage-limits"},
		{"settings usage limits update", stdhttp.MethodPut, "/api/v1/admin/settings/usage-limits"},
		{"core usage logs", stdhttp.MethodGet, "/api/v1/admin/usage-logs"},
		{"core usage analytics", stdhttp.MethodGet, "/api/v1/admin/usage-analytics"},
		{"core stats", stdhttp.MethodGet, "/api/v1/admin/stats"},
		{"core model inventory", stdhttp.MethodGet, "/api/v1/admin/models"},
		{"core api tokens list", stdhttp.MethodGet, "/api/v1/admin/api-tokens"},
		{"core api token revoke", stdhttp.MethodPost, "/api/v1/admin/api-tokens/tok_1/revoke"},
		{"core routes list", stdhttp.MethodGet, "/api/v1/admin/routes"},
		{"core routes create", stdhttp.MethodPost, "/api/v1/admin/routes"},
		{"core route get", stdhttp.MethodGet, "/api/v1/admin/routes/route_1"},
		{"core route update", stdhttp.MethodPut, "/api/v1/admin/routes/route_1"},
		{"core route delete", stdhttp.MethodDelete, "/api/v1/admin/routes/route_1"},
		{"core plans list", stdhttp.MethodGet, "/api/v1/admin/plans"},
		{"core plans create", stdhttp.MethodPost, "/api/v1/admin/plans"},
		{"core plan get", stdhttp.MethodGet, "/api/v1/admin/plans/plan_1"},
		{"core plan update", stdhttp.MethodPut, "/api/v1/admin/plans/plan_1"},
		{"core plan deactivate", stdhttp.MethodDelete, "/api/v1/admin/plans/plan_1"},
		{"core users list", stdhttp.MethodGet, "/api/v1/admin/users"},
		{"core user get", stdhttp.MethodGet, "/api/v1/admin/users/user_1"},
		{"core user update", stdhttp.MethodPut, "/api/v1/admin/users/user_1"},
		{"core user quota update", stdhttp.MethodPatch, "/api/v1/admin/users/user_1"},
		{"core user delete", stdhttp.MethodDelete, "/api/v1/admin/users/user_1"},
		{"core user disable", stdhttp.MethodPost, "/api/v1/admin/users/user_1/disable"},
		{"core user enable", stdhttp.MethodPost, "/api/v1/admin/users/user_1/enable"},
		{"core audit logs", stdhttp.MethodGet, "/api/v1/admin/audit-logs"},
		{"organization list", stdhttp.MethodGet, "/api/v1/admin/organizations"},
		{"organization detail", stdhttp.MethodGet, "/api/v1/admin/organizations/org_1"},
		{"organization members", stdhttp.MethodGet, "/api/v1/admin/organizations/org_1/members"},
		{"channel provider catalog", stdhttp.MethodGet, "/api/v1/admin/channel-providers"},
		{"channel list", stdhttp.MethodGet, "/api/v1/admin/channels"},
		{"channel runtime stats", stdhttp.MethodGet, "/api/v1/admin/channels/stats"},
		{"channel detail", stdhttp.MethodGet, "/api/v1/admin/channels/channel_1"},
		{"channel health", stdhttp.MethodGet, "/api/v1/admin/channels/channel_1/health"},
		{"channel sync models", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/sync-models"},
		{"channel model update detect", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/model-updates/detect"},
		{"channel model update apply", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/model-updates/apply"},
		{"channel refresh balance", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/refresh-balance"},
		{"observability alert routing get", stdhttp.MethodGet, "/api/v1/admin/observability/alert-routing"},
		{"observability alert routing update", stdhttp.MethodPut, "/api/v1/admin/observability/alert-routing"},
		{"observability providers list", stdhttp.MethodGet, "/api/v1/admin/observability/alert-providers"},
		{"observability providers create", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers"},
		{"observability provider update", stdhttp.MethodPut, "/api/v1/admin/observability/alert-providers/provider_1"},
		{"observability provider test", stdhttp.MethodPost, "/api/v1/admin/observability/alert-providers/provider_1/test"},
		{"observability alerts list", stdhttp.MethodGet, "/api/v1/admin/observability/alerts"},
		{"observability alert get", stdhttp.MethodGet, "/api/v1/admin/observability/alerts/relay-backlog"},
		{"observability alert acknowledge", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/acknowledge"},
		{"observability alert resolve", stdhttp.MethodPost, "/api/v1/admin/observability/alerts/relay-backlog/resolve"},
		{"observability alert deliveries", stdhttp.MethodGet, "/api/v1/admin/observability/alerts/relay-backlog/deliveries"},
		{"observability recovery actions", stdhttp.MethodGet, "/api/v1/admin/observability/recovery-actions"},
		{"marketplace takedown", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/takedown"},
		{"marketplace reinstate", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/reinstate"},
		{"marketplace reject appeal", stdhttp.MethodPost, "/api/v1/admin/marketplace/agents/agent_1/reject-appeal"},
		{"marketplace abuse reports", stdhttp.MethodGet, "/api/v1/admin/marketplace/abuse-reports"},
		{"marketplace abuse resolve", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/resolve"},
		{"marketplace abuse dismiss", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/dismiss"},
		{"marketplace review list", stdhttp.MethodGet, "/api/v1/admin/reviews"},
		{"marketplace review sla enforce", stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce"},
	}
}

func TestRouteSurfaceAdminChannelOperationsRejectAdminCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceAdminSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"sync models", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/sync-models"},
		{"detect model updates", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/model-updates/detect"},
		{"apply model updates", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/model-updates/apply"},
		{"refresh balance", stdhttp.MethodPost, "/api/v1/admin/channels/channel_1/refresh-balance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{"mode":"merge"}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)

			router.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusForbidden {
				t.Fatalf("expected missing csrf to be rejected with 403 for %s %s, got %d with body %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func routeSurfaceUserSession() auth.Session {
	return auth.Session{
		ID:             "session_route_surface_user",
		OrganizationID: "org_route_surface",
		WorkspaceID:    "workspace_route_surface",
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
		User: auth.User{
			ID:    "user_route_surface",
			Email: "route-surface-user@example.com",
			Name:  "Route Surface User",
			Role:  "user",
		},
	}
}

func routeSurfaceAdminSession() auth.Session {
	session := routeSurfaceUserSession()
	session.ID = "session_route_surface_admin"
	session.User.ID = "admin_route_surface"
	session.User.Email = "route-surface-admin@example.com"
	session.User.Name = "Route Surface Admin"
	session.User.Role = "admin"
	return session
}

func routeSurfaceSignedSessionCookie(t *testing.T, session auth.Session) *stdhttp.Cookie {
	t.Helper()

	middleware := newAuthMiddleware(testConfig(), auth.NewService(stubAuthStore{session: session}))
	recorder := httptest.NewRecorder()
	middleware.setSessionCookie(recorder, session)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one signed session cookie, got %d", len(cookies))
	}
	return cookies[0]
}

func routeSurfaceCSRFToken(session auth.Session) string {
	middleware := newAuthMiddleware(testConfig(), auth.NewService(stubAuthStore{session: session}))
	return middleware.csrfToken(session.ID)
}
