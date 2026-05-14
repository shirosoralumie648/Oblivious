package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Server 表示一个 MCP Server 配置
type Server struct {
	ID              string    `json:"id"`
	UserID          string    `json:"userId"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	AuthToken       string    `json:"authToken,omitempty"`
	Status          string    `json:"status"` // "connected" | "disconnected" | "error"
	LastConnectedAt time.Time `json:"lastConnectedAt,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// ToolDefinition 表示 MCP 工具定义
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema"`
}

// ToolResult 表示工具执行结果
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// Client MCP 客户端
type Client struct {
	mu         sync.RWMutex
	servers    map[string]*serverConnection
	httpClient *http.Client
	store      Store
}

type serverConnection struct {
	server *Server
	status string
	tools  []ToolDefinition
}

// Store MCP Server 存储接口
type Store interface {
	CreateServer(ctx context.Context, userID string, server *Server) (*Server, error)
	GetServer(ctx context.Context, id string) (*Server, error)
	ListServers(ctx context.Context, userID string) ([]*Server, error)
	UpdateServerStatus(ctx context.Context, id string, status string) error
	DeleteServer(ctx context.Context, id string) error
}

// NewClient 创建 MCP 客户端
func NewClient(store Store) *Client {
	return &Client{
		servers:    make(map[string]*serverConnection),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		store:      store,
	}
}

// AddServer 添加 MCP Server
func (c *Client) AddServer(ctx context.Context, userID string, server *Server) (*Server, error) {
	if server.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if server.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	created, err := c.store.CreateServer(ctx, userID, server)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.servers[created.ID] = &serverConnection{
		server: created,
		status: "disconnected",
	}
	c.mu.Unlock()

	return created, nil
}

// GetServer 获取 MCP Server
func (c *Client) GetServer(ctx context.Context, id string) (*Server, error) {
	return c.store.GetServer(ctx, id)
}

// ListServers 列出用户的 MCP Server
func (c *Client) ListServers(ctx context.Context, userID string) ([]*Server, error) {
	return c.store.ListServers(ctx, userID)
}

// RemoveServer 移除 MCP Server
func (c *Client) RemoveServer(ctx context.Context, id string) error {
	c.mu.Lock()
	delete(c.servers, id)
	c.mu.Unlock()

	return c.store.DeleteServer(ctx, id)
}

// Connect 连接到 MCP Server
func (c *Client) Connect(ctx context.Context, serverID string) error {
	c.mu.RLock()
	conn, ok := c.servers[serverID]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("server not found: %s", serverID)
	}

	// 发送初始化请求
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]any{
				"name":    "oblivious",
				"version": "1.0.0",
			},
		},
	}

	resp, err := c.sendRequest(ctx, conn.server, initReq)
	if err != nil {
		c.mu.Lock()
		conn.status = "error"
		c.mu.Unlock()
		c.store.UpdateServerStatus(ctx, serverID, "error")
		return fmt.Errorf("connect failed: %w", err)
	}

	// 解析响应
	var initResp struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	json.Unmarshal(resp, &initResp)

	// 发送 initialized 通知
	initializedReq := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	c.sendRequest(ctx, conn.server, initializedReq)

	// 获取工具列表
	tools, err := c.listToolsFromServer(ctx, conn.server)
	if err != nil {
		// 非致命错误，继续连接
		tools = nil
	}

	c.mu.Lock()
	conn.status = "connected"
	conn.tools = tools
	c.mu.Unlock()

	c.store.UpdateServerStatus(ctx, serverID, "connected")

	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect(serverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, ok := c.servers[serverID]
	if !ok {
		return nil
	}

	conn.status = "disconnected"
	return nil
}

// ListTools 列出 MCP Server 的工具
func (c *Client) ListTools(serverID string) ([]ToolDefinition, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	if conn.status != "connected" {
		return nil, fmt.Errorf("server not connected")
	}

	return conn.tools, nil
}

// CallTool 调用 MCP 工具
func (c *Client) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*ToolResult, error) {
	c.mu.RLock()
	conn, ok := c.servers[serverID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}

	resp, err := c.sendRequest(ctx, conn.server, req)
	if err != nil {
		return nil, fmt.Errorf("call tool failed: %w", err)
	}

	// 解析响应
	var toolResp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(resp, &toolResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if toolResp.Error != nil {
		return &ToolResult{
			Content: toolResp.Error.Message,
			IsError: true,
		}, nil
	}

	var content strings.Builder
	for _, c := range toolResp.Result.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)
		}
	}

	return &ToolResult{
		Content: content.String(),
		IsError: toolResp.Result.IsError,
	}, nil
}

// listToolsFromServer 从服务器获取工具列表
func (c *Client) listToolsFromServer(ctx context.Context, server *Server) ([]ToolDefinition, error) {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  "tools/list",
	}

	resp, err := c.sendRequest(ctx, server, req)
	if err != nil {
		return nil, err
	}

	var toolsResp struct {
		Result struct {
			Tools []ToolDefinition `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &toolsResp); err != nil {
		return nil, err
	}

	return toolsResp.Result.Tools, nil
}

// sendRequest 发送 JSON-RPC 请求
func (c *Client) sendRequest(ctx context.Context, server *Server, req any) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if server.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+server.AuthToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http error: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetServerStatus 获取服务器状态
func (c *Client) GetServerStatus(serverID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if conn, ok := c.servers[serverID]; ok {
		return conn.status
	}
	return "unknown"
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// CreateServer 创建 Server
func (s *SQLStore) CreateServer(ctx context.Context, userID string, server *Server) (*Server, error) {
	id := fmt.Sprintf("mcp_%d", time.Now().UnixNano())
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mcp_servers (id, user_id, name, url, auth_token_encrypted, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, userID, server.Name, server.URL, server.AuthToken, "disconnected", now, now)
	if err != nil {
		return nil, fmt.Errorf("insert server: %w", err)
	}

	return &Server{
		ID:        id,
		UserID:    userID,
		Name:      server.Name,
		URL:       server.URL,
		AuthToken: server.AuthToken,
		Status:    "disconnected",
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetServer 获取 Server
func (s *SQLStore) GetServer(ctx context.Context, id string) (*Server, error) {
	var server Server
	var authToken sql.NullString
	var lastConnected sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, url, auth_token_encrypted, status, last_connected_at, created_at, updated_at
		FROM mcp_servers WHERE id = $1
	`, id).Scan(&server.ID, &server.UserID, &server.Name, &server.URL, &authToken, &server.Status, &lastConnected, &server.CreatedAt, &server.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get server: %w", err)
	}

	server.AuthToken = authToken.String
	if lastConnected.Valid {
		server.LastConnectedAt = lastConnected.Time
	}

	return &server, nil
}

// ListServers 列出用户的 Server
func (s *SQLStore) ListServers(ctx context.Context, userID string) ([]*Server, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, url, auth_token_encrypted, status, last_connected_at, created_at, updated_at
		FROM mcp_servers WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var servers []*Server
	for rows.Next() {
		var server Server
		var authToken sql.NullString
		var lastConnected sql.NullTime

		err := rows.Scan(&server.ID, &server.UserID, &server.Name, &server.URL, &authToken, &server.Status, &lastConnected, &server.CreatedAt, &server.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan server: %w", err)
		}

		server.AuthToken = authToken.String
		if lastConnected.Valid {
			server.LastConnectedAt = lastConnected.Time
		}
		servers = append(servers, &server)
	}

	return servers, rows.Err()
}

// UpdateServerStatus 更新服务器状态
func (s *SQLStore) UpdateServerStatus(ctx context.Context, id string, status string) error {
	now := time.Now()
	var lastConnected interface{}
	if status == "connected" {
		lastConnected = now
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE mcp_servers SET status = $2, last_connected_at = $3, updated_at = $4 WHERE id = $1
	`, id, status, lastConnected, now)
	return err
}

// DeleteServer 删除 Server
func (s *SQLStore) DeleteServer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1`, id)
	return err
}
