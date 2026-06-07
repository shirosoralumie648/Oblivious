package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"
	"time"

	"oblivious/server/internal/mcp"
)

type mcpHandler struct {
	client *mcp.Client
}

func newMCPHandler(client *mcp.Client) mcpHandler {
	return mcpHandler{client: client}
}

// GET /api/v1/app/mcp-local-servers
func (h mcpHandler) listLocalServers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if _, ok := sessionFromContext(r); !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, mcp.NewLocalCatalog().ListServers(r.Context()))
}

// AddServerRequest 添加 MCP Server 请求
type AddServerRequest struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	AuthToken string `json:"authToken,omitempty"`
}

type mcpServerResponse struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organizationId"`
	UserID          string    `json:"userId"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	HasAuthToken    bool      `json:"hasAuthToken"`
	Status          string    `json:"status"`
	LastConnectedAt time.Time `json:"lastConnectedAt,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func toMCPServerResponse(server *mcp.Server) *mcpServerResponse {
	if server == nil {
		return nil
	}
	return &mcpServerResponse{
		ID:              server.ID,
		OrganizationID:  server.OrganizationID,
		UserID:          server.UserID,
		Name:            server.Name,
		URL:             server.URL,
		HasAuthToken:    server.HasAuthToken || server.AuthToken != "",
		Status:          server.Status,
		LastConnectedAt: server.LastConnectedAt,
		CreatedAt:       server.CreatedAt,
		UpdatedAt:       server.UpdatedAt,
	}
}

func toMCPServerResponses(servers []*mcp.Server) []*mcpServerResponse {
	responses := make([]*mcpServerResponse, 0, len(servers))
	for _, server := range servers {
		responses = append(responses, toMCPServerResponse(server))
	}
	return responses
}

// POST /api/v1/app/mcp-servers
func (h mcpHandler) addServer(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req AddServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.URL = strings.TrimSpace(req.URL)

	if req.Name == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}
	if req.URL == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "url is required")
		return
	}

	server := &mcp.Server{
		OrganizationID: session.OrganizationID,
		Name:           req.Name,
		URL:            req.URL,
		AuthToken:      req.AuthToken,
	}

	created, err := h.client.AddServer(r.Context(), session.User.ID, session.OrganizationID, server)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, toMCPServerResponse(created))
}

// GET /api/v1/app/mcp-servers
func (h mcpHandler) listServers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	servers, err := h.client.ListServers(r.Context(), session.User.ID, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, toMCPServerResponses(servers))
}

// GET /api/v1/app/mcp-servers/:id
func (h mcpHandler) getServer(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, toMCPServerResponse(server))
}

// DELETE /api/v1/app/mcp-servers/:id
func (h mcpHandler) deleteServer(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	if err := h.client.RemoveServer(r.Context(), id, session.OrganizationID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/v1/app/mcp-servers/:id/connect
func (h mcpHandler) connectServer(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	if err := h.client.Connect(r.Context(), id, session.OrganizationID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	// 返回更新后的服务器状态
	updated, _ := h.client.GetServer(r.Context(), id, session.OrganizationID)
	writeSuccess(w, stdhttp.StatusOK, toMCPServerResponse(updated))
}

// POST /api/v1/app/mcp-servers/:id/disconnect
func (h mcpHandler) disconnectServer(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	if err := h.client.Disconnect(id, session.OrganizationID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "disconnected"})
}

// GET /api/v1/app/mcp-servers/:id/tools
func (h mcpHandler) listServerTools(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	tools, err := h.client.ListTools(id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, tools)
}

// GET /api/v1/app/mcp-servers/:id/status
func (h mcpHandler) getServerStatus(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), id, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	status := h.client.GetServerStatus(id, session.OrganizationID)
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": status})
}

// POST /api/v1/app/mcp-servers/:id/execute
type executeToolRequest struct {
	ToolName string         `json:"toolName"`
	Args     map[string]any `json:"args,omitempty"`
}

func (h mcpHandler) executeTool(w stdhttp.ResponseWriter, r *stdhttp.Request, serverID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	// 验证所有权
	server, err := h.client.GetServer(r.Context(), serverID, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if server == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "server not found")
		return
	}
	if server.UserID != session.User.ID {
		writeError(w, stdhttp.StatusForbidden, "forbidden", "access denied")
		return
	}

	var req executeToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	if req.ToolName == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "toolName is required")
		return
	}

	result, err := h.client.CallTool(r.Context(), serverID, session.OrganizationID, req.ToolName, req.Args)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}
