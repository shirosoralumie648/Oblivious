package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
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
		{"tasks", stdhttp.MethodGet, "/api/v1/app/tasks"},
		{"organizations", stdhttp.MethodGet, "/api/v1/app/organizations"},
		{"organization members", stdhttp.MethodGet, "/api/v1/app/organizations/org_1/members"},
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

func TestRouteSurfaceRegistersCanonicalKnowledgeRoutesThroughRegistrar(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	if !strings.Contains(string(source), "registerKnowledgeRoutes(mux, authMiddleware, knowledgeHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical app knowledge routes through registerKnowledgeRoutes")
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
		{"update document", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"delete document", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"list document versions", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/versions"},
		{"list chunks", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks"},
		{"update chunk", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"retrieve", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve"},
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
		{"update document", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"delete document", stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1"},
		{"list document versions", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/versions"},
		{"list chunks", stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks"},
		{"update chunk", stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1"},
		{"split chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/split"},
		{"merge chunk", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/doc_1/chunks/chunk_1/merge"},
		{"retrieve", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/retrieve"},
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

func TestRouteSurfaceRegistersConsoleInvoiceRoute(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	routesSource, err := os.ReadFile("routes_console.go")
	if err != nil {
		t.Fatalf("read routes_console.go: %v", err)
	}

	if !strings.Contains(string(routerSource), "registerConsoleRoutes(mux, authMiddleware, consoleHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRoutes")
	}
	if !strings.Contains(string(routesSource), `mux.Handle("/api/v1/console/invoices"`) {
		t.Fatal("expected NewRouterWithOptions to expose /api/v1/console/invoices")
	}
}

func TestRouteSurfaceRegistersConsoleAPITokenRoutes(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	routesSource, err := os.ReadFile("routes_console.go")
	if err != nil {
		t.Fatalf("read routes_console.go: %v", err)
	}
	if !strings.Contains(string(routerSource), "registerConsoleRoutes(mux, authMiddleware, consoleHandler)") {
		t.Fatal("expected NewRouterWithOptions to register canonical console routes through registerConsoleRoutes")
	}
	text := string(routesSource)

	for _, expected := range []string{
		`mux.Handle("/api/v1/console/api-tokens"`,
		`mux.Handle("/api/v1/console/api-tokens/"`,
		"consoleHandler.createAPIToken",
		"consoleHandler.listAPITokenUsage",
		"consoleHandler.revokeAPIToken",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected NewRouterWithOptions to expose Console API token route surface containing %q", expected)
		}
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

func TestRouteSurfaceChatMutationsRejectCookieWithoutCSRFWithoutDatabase(t *testing.T) {
	session := routeSurfaceUserSession()
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{AuthStore: stubAuthStore{session: session}})
	cookie := routeSurfaceSignedSessionCookie(t, session)

	tests := []routeSurfaceCase{
		{"create conversation", stdhttp.MethodPost, "/api/v1/app/conversations"},
		{"send message", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages"},
		{"update conversation config", stdhttp.MethodPut, "/api/v1/app/conversations/conversation_1/config"},
		{"convert conversation to task", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/convert-to-task"},
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
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	text := string(source)
	if !strings.Contains(text, `mux.Handle("/metrics", workflowMetricsHandler(workflowService))`) {
		t.Fatal("expected /metrics to use workflowMetricsHandler")
	}
	if !strings.Contains(text, "RefreshExecutionHealthMetrics") {
		t.Fatal("expected workflowMetricsHandler to refresh workflow execution health metrics before scrape")
	}
}

func TestRouteSurfaceWiresConfiguredWorkflowSystemLimits(t *testing.T) {
	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	serviceSource, err := os.ReadFile("workflow_service.go")
	if err != nil {
		t.Fatalf("read workflow_service.go: %v", err)
	}

	if !strings.Contains(string(routerSource), "workflowService = newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected default router workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
	}
	if !strings.Contains(string(serverSource), "workflowService := newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())") {
		t.Fatal("expected server workflow service to use configured workflow limits, failure notifications, and alert delivery sink")
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
		"/api/v1/admin/channels",
		"/api/v1/admin/routes",
		"/api/v1/admin/plans",
		"/api/v1/admin/users",
		"/api/v1/admin/organizations",
		"/api/v1/admin/audit-logs",
		"/api/v1/admin/reviews",
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
		{"billing topup refund", stdhttp.MethodPost, "/api/v1/admin/billing/topups/topup_1/refund"},
		{"billing payout paid", stdhttp.MethodPost, "/api/v1/admin/billing/payouts/payout_1/paid"},
		{"core stats", stdhttp.MethodGet, "/api/v1/admin/stats"},
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
		{"channel provider catalog", stdhttp.MethodGet, "/api/v1/admin/channel-providers"},
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
		{"marketplace abuse reports", stdhttp.MethodGet, "/api/v1/admin/marketplace/abuse-reports"},
		{"marketplace abuse resolve", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/resolve"},
		{"marketplace abuse dismiss", stdhttp.MethodPost, "/api/v1/admin/marketplace/abuse-reports/report_1/dismiss"},
		{"marketplace review sla enforce", stdhttp.MethodPost, "/api/v1/admin/reviews/sla/enforce"},
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
