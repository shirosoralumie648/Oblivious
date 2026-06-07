package http

import (
	"context"
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/mcp"
)

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
