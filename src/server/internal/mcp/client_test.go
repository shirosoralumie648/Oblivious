package mcp

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientRehydratesPersistedServerOnConnectListToolsAndCallTool(t *testing.T) {
	ctx := context.Background()
	const (
		serverID       = "mcp_persisted"
		organizationID = "org_1"
	)

	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode json-rpc request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"test-mcp","version":"1.0.0"}}}`)
		case "notifications/initialized":
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":{}}`)
		case "tools/list":
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"echo","description":"Echo text","inputSchema":{"type":"object"}}]}}`)
		case "tools/call":
			fmt.Fprint(w, `{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"called persisted echo"}],"isError":false}}`)
		default:
			t.Fatalf("unexpected json-rpc method %q", req.Method)
		}
	}))
	t.Cleanup(mcpServer.Close)

	store := newMemoryMCPStore(&Server{
		ID:             serverID,
		OrganizationID: organizationID,
		UserID:         "user_1",
		Name:           "Persisted",
		URL:            mcpServer.URL,
		Status:         "disconnected",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})
	client := NewClient(store)

	if got := len(client.servers); got != 0 {
		t.Fatalf("new client in-memory server map len = %d, want 0", got)
	}

	if err := client.Connect(ctx, serverID, organizationID); err != nil {
		t.Fatalf("Connect persisted server returned error: %v", err)
	}

	tools, err := client.ListTools(serverID, organizationID)
	if err != nil {
		t.Fatalf("ListTools persisted server returned error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("ListTools returned %+v, want echo tool", tools)
	}

	result, err := client.CallTool(ctx, serverID, organizationID, "echo", map[string]any{"text": "hi"})
	if err != nil {
		t.Fatalf("CallTool persisted server returned error: %v", err)
	}
	if result == nil || result.Content != "called persisted echo" || result.IsError {
		t.Fatalf("CallTool result = %+v, want non-error echoed content", result)
	}
	if got := store.getServerCalls(serverID, organizationID); got == 0 {
		t.Fatalf("store GetServer calls = %d, want rehydrate lookup", got)
	}
}

func TestSQLStoreCreateServerStoresProtectedAuthTokenAndGetServerHydratesUsableToken(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN_ENCRYPTION_KEY", "test-mcp-token-encryption-key")
	ctx := context.Background()
	capture := &mcpSQLCapture{}
	db := openMCPTestDB(t, "mcp_auth_token_create", capture)
	store := NewSQLStore(db)

	created, err := store.CreateServer(ctx, "user_1", "org_1", &Server{
		Name:      "private mcp",
		URL:       "https://mcp.example.test",
		AuthToken: "plain-secret-token",
	})
	if err != nil {
		t.Fatalf("CreateServer returned error: %v", err)
	}

	persistedToken := capture.insertedAuthToken()
	if persistedToken == "plain-secret-token" {
		t.Fatalf("auth_token_encrypted persisted plaintext token")
	}
	if persistedToken == "" {
		t.Fatalf("auth_token_encrypted persisted empty token")
	}
	if !strings.HasPrefix(persistedToken, mcpAuthTokenGCMCodecPrefix) {
		t.Fatalf("auth_token_encrypted = %q, want GCM protected token prefix", persistedToken)
	}
	if strings.Contains(persistedToken, "plain-secret-token") {
		t.Fatalf("auth_token_encrypted embedded plaintext token: %q", persistedToken)
	}
	encodedPayload := strings.TrimPrefix(persistedToken, mcpAuthTokenGCMCodecPrefix)
	if decodedPayload, err := base64.RawURLEncoding.DecodeString(encodedPayload); err == nil && string(decodedPayload) == "plain-secret-token" {
		t.Fatalf("auth_token_encrypted payload is reversible plaintext base64")
	}

	got, err := store.GetServer(ctx, created.ID, "org_1")
	if err != nil {
		t.Fatalf("GetServer returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("GetServer returned nil")
	}
	if got.AuthToken != "plain-secret-token" {
		t.Fatalf("GetServer AuthToken = %q, want usable plaintext token", got.AuthToken)
	}
}

func TestSQLStoreListServersDoesNotHydrateAuthToken(t *testing.T) {
	ctx := context.Background()
	capture := &mcpSQLCapture{
		listRows: [][]driver.Value{{
			"mcp_1",
			"org_1",
			"user_1",
			"private mcp",
			"https://mcp.example.test",
			"plain-secret-token",
			"disconnected",
			nil,
			time.Now(),
			time.Now(),
		}},
	}
	db := openMCPTestDB(t, "mcp_auth_token_list", capture)
	store := NewSQLStore(db)

	servers, err := store.ListServers(ctx, "user_1", "org_1")
	if err != nil {
		t.Fatalf("ListServers returned error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("ListServers returned %d servers, want 1", len(servers))
	}
	if servers[0].AuthToken != "" {
		t.Fatalf("ListServers AuthToken = %q, want empty", servers[0].AuthToken)
	}
}

type memoryMCPStore struct {
	mu             sync.Mutex
	servers        map[string]*Server
	getServerCount map[string]int
}

func newMemoryMCPStore(servers ...*Server) *memoryMCPStore {
	store := &memoryMCPStore{
		servers:        make(map[string]*Server),
		getServerCount: make(map[string]int),
	}
	for _, server := range servers {
		store.servers[serverKey(server.ID, server.OrganizationID)] = cloneServer(server)
	}
	return store
}

func (s *memoryMCPStore) CreateServer(ctx context.Context, userID, organizationID string, server *Server) (*Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created := cloneServer(server)
	created.UserID = userID
	created.OrganizationID = organizationID
	if created.ID == "" {
		created.ID = fmt.Sprintf("mcp_%d", time.Now().UnixNano())
	}
	s.servers[serverKey(created.ID, created.OrganizationID)] = cloneServer(created)
	return created, nil
}

func (s *memoryMCPStore) GetServer(ctx context.Context, id, organizationID string) (*Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := serverKey(id, organizationID)
	s.getServerCount[key]++
	return cloneServer(s.servers[key]), nil
}

func (s *memoryMCPStore) ListServers(ctx context.Context, userID, organizationID string) ([]*Server, error) {
	return nil, nil
}

func (s *memoryMCPStore) UpdateServerStatus(ctx context.Context, id, organizationID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	server := s.servers[serverKey(id, organizationID)]
	if server == nil {
		return nil
	}
	server.Status = status
	server.UpdatedAt = time.Now()
	return nil
}

func (s *memoryMCPStore) DeleteServer(ctx context.Context, id, organizationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.servers, serverKey(id, organizationID))
	return nil
}

func (s *memoryMCPStore) getServerCalls(id, organizationID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getServerCount[serverKey(id, organizationID)]
}

func serverKey(id, organizationID string) string {
	return id + "\x00" + organizationID
}

func cloneServer(server *Server) *Server {
	if server == nil {
		return nil
	}
	cloned := *server
	return &cloned
}

var mcpTestDrivers sync.Map

func openMCPTestDB(t *testing.T, name string, capture *mcpSQLCapture) *sql.DB {
	t.Helper()

	driverName := name + "_" + fmt.Sprint(time.Now().UnixNano())
	mcpTestDrivers.Store(driverName, capture)
	sql.Register(driverName, mcpTestDriver{name: driverName})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		mcpTestDrivers.Delete(driverName)
	})
	return db
}

type mcpSQLCapture struct {
	mu       sync.Mutex
	inserted []driver.NamedValue
	listRows [][]driver.Value
}

func (c *mcpSQLCapture) insertedAuthToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inserted) < 6 {
		return ""
	}
	token, _ := c.inserted[5].Value.(string)
	return token
}

type mcpTestDriver struct {
	name string
}

func (d mcpTestDriver) Open(_ string) (driver.Conn, error) {
	capture, _ := mcpTestDrivers.Load(d.name)
	return mcpTestConn{capture: capture.(*mcpSQLCapture)}, nil
}

type mcpTestConn struct {
	capture *mcpSQLCapture
}

func (c mcpTestConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c mcpTestConn) Close() error {
	return nil
}

func (c mcpTestConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c mcpTestConn) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	c.capture.mu.Lock()
	c.capture.inserted = append([]driver.NamedValue(nil), args...)
	c.capture.mu.Unlock()
	return driver.RowsAffected(1), nil
}

func (c mcpTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.capture.mu.Lock()
	defer c.capture.mu.Unlock()

	rows := append([][]driver.Value(nil), c.capture.listRows...)
	if strings.Contains(query, "WHERE id = $1") && len(c.capture.inserted) >= 9 {
		rows = [][]driver.Value{{
			c.capture.inserted[0].Value,
			c.capture.inserted[2].Value,
			c.capture.inserted[1].Value,
			c.capture.inserted[3].Value,
			c.capture.inserted[4].Value,
			c.capture.inserted[5].Value,
			c.capture.inserted[6].Value,
			nil,
			c.capture.inserted[7].Value,
			c.capture.inserted[8].Value,
		}}
	}
	return &mcpTestRows{
		columns: []string{"id", "organization_id", "user_id", "name", "url", "auth_token_encrypted", "status", "last_connected_at", "created_at", "updated_at"},
		rows:    rows,
	}, nil
}

func (c mcpTestConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type mcpTestRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *mcpTestRows) Columns() []string {
	return r.columns
}

func (r *mcpTestRows) Close() error {
	return nil
}

func (r *mcpTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
