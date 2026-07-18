package http

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

func TestMCPCatalogMutationContract(t *testing.T) {
	guard := &mcpHandlerReadinessGuard{}
	guard.allow.Store(true)
	contract, profile := loadMCPHandlerReadinessContract(t)
	registrar := &mcpHandlerReadinessRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)}
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	store := newMCPHandlerFakeStore()
	client := mcp.NewClient(store)
	handler, err := newMCPHandlerWithOptions(client, MCPHandlerRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar})
	if err != nil {
		t.Fatalf("new MCP handler: %v", err)
	}

	for _, body := range []string{
		`{"name":"bad","url":"https://mcp.example.test","capabilityId":"caller"}`,
		`{"name":"bad","url":"https://mcp.example.test","unexpected":true}`,
	} {
		recorder := httptest.NewRecorder()
		handler.addServer(recorder, newAuthenticatedMCPRequest(stdhttp.MethodPost, "/api/v1/app/mcp-servers", body))
		if recorder.Code != stdhttp.StatusBadRequest {
			t.Fatalf("unknown MCP member status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
	}
	if len(store.servers) != 0 || guard.calls.Load() != 0 {
		t.Fatalf("rejected MCP mutation changed store/guard = %d/%d", len(store.servers), guard.calls.Load())
	}

	recorder := httptest.NewRecorder()
	handler.addServer(recorder, newAuthenticatedMCPRequest(stdhttp.MethodPost, "/api/v1/app/mcp-servers", `{"name":"valid","url":"https://mcp.example.test"}`))
	if recorder.Code != stdhttp.StatusCreated || len(store.servers) != 1 || guard.calls.Load() != 1 {
		t.Fatalf("valid MCP mutation status/store/guard = %d/%d/%d, want 201/1/1", recorder.Code, len(store.servers), guard.calls.Load())
	}
	if registrar.count() != 1 {
		t.Fatalf("MCP handler registrar count = %d, want 1", registrar.count())
	}
}

type mcpHandlerReadinessGuard struct {
	allow atomic.Bool
	calls atomic.Int32
}

func (g *mcpHandlerReadinessGuard) Require(context.Context, string, releasecontract.Boundary) error {
	g.calls.Add(1)
	if !g.allow.Load() {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "test"}
	}
	return nil
}

type mcpHandlerReadinessRegistrar struct {
	mu          sync.Mutex
	descriptors map[string]releasecontract.EffectDescriptor
}

func (r *mcpHandlerReadinessRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[descriptor.ID]; exists {
		return fmt.Errorf("duplicate descriptor %s", descriptor.ID)
	}
	r.descriptors[descriptor.ID] = descriptor
	return nil
}

func (r *mcpHandlerReadinessRegistrar) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.descriptors)
}

func loadMCPHandlerReadinessContract(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve MCP handler test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}

func TestMCPHandlerListsLocalSafeBuiltinServers(t *testing.T) {
	handler := newMCPHandler(nil)
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/mcp-local-servers", nil).
		WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
			ID:             "session_1",
			OrganizationID: "org_1",
			User:           auth.User{ID: "user_1"},
		}))
	recorder := httptest.NewRecorder()

	handler.listLocalServers(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		OK   bool                        `json:"ok"`
		Data []mcp.LocalServerDefinition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode local MCP servers response: %v", err)
	}
	if !response.OK {
		t.Fatalf("expected ok response, got %+v", response)
	}
	if len(response.Data) != 1 {
		t.Fatalf("expected one local MCP server, got %+v", response.Data)
	}
	if response.Data[0].ID != mcp.LocalBuiltinServerID || response.Data[0].ToolCount != len(mcp.ListDefaultCommercialBuiltinTools()) {
		t.Fatalf("unexpected local MCP server payload: %+v", response.Data[0])
	}
}

func TestMCPHandlerAddServerDoesNotExposeAuthToken(t *testing.T) {
	store := newMCPHandlerFakeStore()
	handler := newMCPHandler(mcp.NewClient(store))
	request := newAuthenticatedMCPRequest(stdhttp.MethodPost, "/api/v1/app/mcp-servers", `{
		"name": "Private MCP",
		"url": "https://mcp.example.test",
		"authToken": "raw-secret-token"
	}`)
	recorder := httptest.NewRecorder()

	handler.addServer(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "raw-secret-token") || strings.Contains(body, "authToken") {
		t.Fatalf("MCP add response leaked auth token: %s", body)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Status       string `json:"status"`
			HasAuthToken bool   `json:"hasAuthToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode add MCP response: %v", err)
	}
	if !response.OK || response.Data.ID == "" || response.Data.Name != "Private MCP" || response.Data.URL != "https://mcp.example.test" || response.Data.Status != "disconnected" {
		t.Fatalf("unexpected sanitized add response: %+v", response)
	}
	if !response.Data.HasAuthToken {
		t.Fatalf("add MCP response HasAuthToken = false, want true")
	}
	if len(store.servers) != 1 || store.servers[0].AuthToken != "raw-secret-token" {
		t.Fatalf("expected store to retain internal auth token for outbound MCP calls, got %+v", store.servers)
	}
}

func TestMCPHandlerListServersDoesNotExposeAuthToken(t *testing.T) {
	store := newMCPHandlerFakeStore()
	now := time.Now().UTC()
	store.servers = []*mcp.Server{{
		ID:             "mcp_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Private MCP",
		URL:            "https://mcp.example.test",
		AuthToken:      "raw-secret-token",
		Status:         "connected",
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newMCPHandler(mcp.NewClient(store))
	request := newAuthenticatedMCPRequest(stdhttp.MethodGet, "/api/v1/app/mcp-servers", "")
	recorder := httptest.NewRecorder()

	handler.listServers(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "raw-secret-token") || strings.Contains(body, "authToken") {
		t.Fatalf("MCP list response leaked auth token: %s", body)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			Status       string `json:"status"`
			HasAuthToken bool   `json:"hasAuthToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list MCP response: %v", err)
	}
	if !response.OK || len(response.Data) != 1 || response.Data[0].ID != "mcp_1" || response.Data[0].Status != "connected" {
		t.Fatalf("unexpected sanitized list response: %+v", response)
	}
	if !response.Data[0].HasAuthToken {
		t.Fatalf("list MCP response HasAuthToken = false, want true")
	}
}

func TestMCPHandlerGetServerDoesNotExposeAuthToken(t *testing.T) {
	store := newMCPHandlerFakeStore()
	now := time.Now().UTC()
	store.servers = []*mcp.Server{{
		ID:             "mcp_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Private MCP",
		URL:            "https://mcp.example.test",
		AuthToken:      "raw-secret-token",
		Status:         "connected",
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	handler := newMCPHandler(mcp.NewClient(store))
	request := newAuthenticatedMCPRequest(stdhttp.MethodGet, "/api/v1/app/mcp-servers/mcp_1", "")
	recorder := httptest.NewRecorder()

	handler.getServer(recorder, request, "mcp_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "raw-secret-token") || strings.Contains(body, "authToken") {
		t.Fatalf("MCP get response leaked auth token: %s", body)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			HasAuthToken bool `json:"hasAuthToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode get MCP response: %v", err)
	}
	if !response.Data.HasAuthToken {
		t.Fatalf("get MCP response HasAuthToken = false, want true")
	}
}

func newAuthenticatedMCPRequest(method, path, body string) *stdhttp.Request {
	reader := strings.NewReader(body)
	request := httptest.NewRequest(method, path, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		ID:             "session_1",
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
}

type mcpHandlerFakeStore struct {
	servers []*mcp.Server
}

func newMCPHandlerFakeStore() *mcpHandlerFakeStore {
	return &mcpHandlerFakeStore{}
}

func (s *mcpHandlerFakeStore) CreateServer(ctx context.Context, userID, organizationID string, server *mcp.Server) (*mcp.Server, error) {
	now := time.Now().UTC()
	created := *server
	created.ID = fmt.Sprintf("mcp_%d", len(s.servers)+1)
	created.UserID = userID
	created.OrganizationID = organizationID
	created.Status = "disconnected"
	created.CreatedAt = now
	created.UpdatedAt = now
	s.servers = append(s.servers, &created)
	return &created, nil
}

func (s *mcpHandlerFakeStore) GetServer(ctx context.Context, id, organizationID string) (*mcp.Server, error) {
	for _, server := range s.servers {
		if server.ID == id && server.OrganizationID == organizationID {
			copied := *server
			return &copied, nil
		}
	}
	return nil, nil
}

func (s *mcpHandlerFakeStore) ListServers(ctx context.Context, userID, organizationID string) ([]*mcp.Server, error) {
	var result []*mcp.Server
	for _, server := range s.servers {
		if server.UserID == userID && server.OrganizationID == organizationID {
			copied := *server
			result = append(result, &copied)
		}
	}
	return result, nil
}

func (s *mcpHandlerFakeStore) UpdateServerStatus(ctx context.Context, id, organizationID string, status string) error {
	for _, server := range s.servers {
		if server.ID == id && server.OrganizationID == organizationID {
			server.Status = status
			server.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}

func (s *mcpHandlerFakeStore) DeleteServer(ctx context.Context, id, organizationID string) error {
	for index, server := range s.servers {
		if server.ID == id && server.OrganizationID == organizationID {
			s.servers = append(s.servers[:index], s.servers[index+1:]...)
			return nil
		}
	}
	return nil
}
