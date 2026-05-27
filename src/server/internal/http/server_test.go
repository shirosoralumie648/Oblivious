package http

import (
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"oblivious/server/internal/config"
)

const testDatabaseURLEnvVar = "TEST_DATABASE_URL"

func testConfig() config.Config {
	return config.Config{
		CORSAllowedOrigins:  []string{"http://localhost:5173"},
		DatabaseURL:         strings.TrimSpace(os.Getenv(testDatabaseURLEnvVar)),
		Env:                 "test",
		Port:                8080,
		SessionCookieName:   "oblivious_session",
		SessionCookieSecure: false,
		LLMTimeoutMS:        30000,
		ModelDefaultName:    "demo-reply",
	}
}

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := testConfig().DatabaseURL
	if databaseURL == "" {
		t.Skip(testDatabaseURLEnvVar + " is required for integration tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	lockIntegrationTestDatabase(t, database)

	statements := []string{
		`DROP TABLE IF EXISTS organization_invitations CASCADE`,
		`DROP TABLE IF EXISTS organization_memberships CASCADE`,
		`DROP TABLE IF EXISTS password_reset_tokens CASCADE`,
		`DROP TABLE IF EXISTS auth_rate_limits CASCADE`,
		`DROP TABLE IF EXISTS audit_logs CASCADE`,
		`DROP TABLE IF EXISTS usage_records CASCADE`,
		`DROP TABLE IF EXISTS billing_sessions CASCADE`,
		`DROP TABLE IF EXISTS topup_orders CASCADE`,
		`DROP TABLE IF EXISTS subscriptions CASCADE`,
		`DROP TABLE IF EXISTS quotas CASCADE`,
		`DROP TABLE IF EXISTS notifications CASCADE`,
		`DROP TABLE IF EXISTS agent_messages CASCADE`,
		`DROP TABLE IF EXISTS agent_conversations CASCADE`,
		`DROP TABLE IF EXISTS agents CASCADE`,
		`DROP TABLE IF EXISTS memory_chunks CASCADE`,
		`DROP TABLE IF EXISTS memory_documents CASCADE`,
		`DROP TABLE IF EXISTS mcp_servers CASCADE`,
		`DROP TABLE IF EXISTS knowledge_document_chunks CASCADE`,
		`DROP TABLE IF EXISTS knowledge_documents CASCADE`,
		`DROP TABLE IF EXISTS conversation_knowledge_bindings CASCADE`,
		`DROP TABLE IF EXISTS knowledge_bases CASCADE`,
		`DROP TABLE IF EXISTS conversation_configs CASCADE`,
		`DROP TABLE IF EXISTS user_preferences CASCADE`,
		`DROP TABLE IF EXISTS messages CASCADE`,
		`DROP TABLE IF EXISTS conversations CASCADE`,
		`DROP TABLE IF EXISTS sessions CASCADE`,
		`DROP TABLE IF EXISTS workspaces CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_login_at TIMESTAMPTZ)`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', metadata JSONB NOT NULL DEFAULT '{}', created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), archived_at TIMESTAMPTZ, CHECK (status IN ('active', 'disabled', 'archived')))`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), expires_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE auth_rate_limits (scope TEXT NOT NULL, key TEXT NOT NULL, window_start TIMESTAMPTZ NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, blocked_until TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (scope, key))`,
		`CREATE TABLE password_reset_tokens (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash TEXT NOT NULL UNIQUE, expires_at TIMESTAMPTZ NOT NULL, used_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE audit_logs (id TEXT PRIMARY KEY, actor_id TEXT NOT NULL REFERENCES users(id), actor_email TEXT NOT NULL, action TEXT NOT NULL, resource_type TEXT NOT NULL, resource_id TEXT, changes JSONB, ip_address TEXT, user_agent TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organization_memberships (id TEXT PRIMARY KEY, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, role TEXT NOT NULL, created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), removed_at TIMESTAMPTZ, CHECK (role IN ('owner', 'admin', 'member')))`,
		`CREATE UNIQUE INDEX idx_org_memberships_active_user_http_test ON organization_memberships(organization_id, user_id) WHERE removed_at IS NULL`,
		`CREATE UNIQUE INDEX idx_org_memberships_single_owner_http_test ON organization_memberships(organization_id) WHERE role = 'owner' AND removed_at IS NULL`,
		`CREATE TABLE organization_invitations (id TEXT PRIMARY KEY, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, email TEXT NOT NULL, role TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'pending', invited_by_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, accepted_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, expires_at TIMESTAMPTZ NOT NULL, accepted_at TIMESTAMPTZ, revoked_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), CHECK (role IN ('admin', 'member')), CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')))`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, role TEXT NOT NULL, content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE knowledge_bases (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, document_count INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE knowledge_documents (id TEXT PRIMARY KEY, knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE knowledge_document_chunks (id TEXT PRIMARY KEY, document_id TEXT NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, chunk_index INTEGER NOT NULL, content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (document_id, chunk_index))`,
		`CREATE TABLE conversation_knowledge_bindings (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, knowledge_base_id TEXT NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (conversation_id, knowledge_base_id))`,
		`CREATE TABLE conversation_configs (conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, model_id TEXT NOT NULL DEFAULT 'demo-reply', system_prompt_override TEXT NOT NULL DEFAULT '', temperature DOUBLE PRECISION NOT NULL DEFAULT 1, max_output_tokens INTEGER NOT NULL DEFAULT 1024, tools_enabled BOOLEAN NOT NULL DEFAULT FALSE, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE user_preferences (user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE, default_mode TEXT NOT NULL DEFAULT 'chat', model_strategy TEXT NOT NULL DEFAULT 'balanced', network_enabled_hint BOOLEAN NOT NULL DEFAULT FALSE, default_agent_model TEXT NOT NULL DEFAULT 'gpt-4o-mini', sidebar_collapsed BOOLEAN NOT NULL DEFAULT FALSE, notifications JSONB NOT NULL DEFAULT '{}', updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE notifications (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, type TEXT NOT NULL, category TEXT NOT NULL, title TEXT NOT NULL, message TEXT NOT NULL, is_read BOOLEAN NOT NULL DEFAULT FALSE, action_url TEXT, metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), read_at TIMESTAMPTZ)`,
		`CREATE TABLE usage_records (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, conversation_id TEXT REFERENCES conversations(id) ON DELETE SET NULL, model_id TEXT NOT NULL, request_count INTEGER NOT NULL, input_tokens INTEGER NOT NULL, output_tokens INTEGER NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE quotas (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, balance DECIMAL(15,6) NOT NULL DEFAULT 0, used DECIMAL(15,6) NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (organization_id))`,
		`CREATE TABLE billing_sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, channel_id TEXT, model TEXT, api_type TEXT, idempotency_key TEXT NOT NULL, pre_authorized_amt DECIMAL(15,6) NOT NULL DEFAULT 0, settled_amt DECIMAL(15,6) NOT NULL DEFAULT 0, status TEXT NOT NULL DEFAULT 'preauthorized', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), settled_at TIMESTAMPTZ)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_test_billing_sessions_unique_org_idempotency ON billing_sessions(organization_id, idempotency_key) WHERE idempotency_key <> ''`,
		`CREATE TABLE subscriptions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, package_id TEXT NOT NULL, status TEXT DEFAULT 'active', started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE topup_orders (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, amount DECIMAL(15,6) NOT NULL, money DECIMAL(10,2) NOT NULL, status TEXT DEFAULT 'pending', trade_no TEXT, paid_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agents (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, description TEXT, model TEXT DEFAULT 'gpt-4o-mini', system_prompt TEXT, tools JSONB DEFAULT '[]', config JSONB DEFAULT '{}', is_public BOOLEAN DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_conversations (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, role TEXT NOT NULL, content TEXT NOT NULL, tool_calls JSONB DEFAULT '[]', tool_call_id TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE memory_documents (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT, content TEXT NOT NULL, source_type TEXT DEFAULT 'manual', source_url TEXT, metadata JSONB DEFAULT '{}', total_chunks INTEGER DEFAULT 0, embedding_model TEXT DEFAULT 'text-embedding-3-small', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE memory_chunks (id TEXT PRIMARY KEY, document_id TEXT NOT NULL REFERENCES memory_documents(id) ON DELETE CASCADE, user_id TEXT NOT NULL, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, content TEXT NOT NULL, chunk_index INTEGER NOT NULL, embedding TEXT, metadata JSONB DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (document_id, chunk_index))`,
		`CREATE TABLE mcp_servers (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, url TEXT NOT NULL, auth_token_encrypted TEXT, status TEXT DEFAULT 'disconnected', last_connected_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare database: %v", err)
		}
	}

	return database
}

func lockIntegrationTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock integration test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock integration test database: %v", err)
		}
	})
}

func csrfTokenFromRecorder(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var response struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode csrf token response: %v", err)
	}
	if response.Data.CSRFToken == "" {
		t.Fatal("expected csrf token in session response")
	}
	return response.Data.CSRFToken
}

func csrfTokenForCookie(t *testing.T, router stdhttp.Handler, cookie *stdhttp.Cookie) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(cookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("me for csrf expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	return csrfTokenFromRecorder(t, recorder)
}

func addCSRF(request *stdhttp.Request, csrfToken string) {
	request.Header.Set(csrfHeaderName, csrfToken)
}

func registerHTTPUser(t *testing.T, router stdhttp.Handler, email string) (*stdhttp.Cookie, string, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"`+email+`","password":"StrongerPass1!"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("register %s expected 200, got %d with body %s", email, recorder.Code, recorder.Body.String())
	}
	cookie := firstCookieNamed(t, recorder, testConfig().SessionCookieName)
	csrfToken := csrfTokenFromRecorder(t, recorder)

	var response struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if response.Data.User.ID == "" {
		t.Fatal("expected registered user id")
	}
	return cookie, csrfToken, response.Data.User.ID
}

func loginHTTPUser(t *testing.T, router stdhttp.Handler, email, password string) (*stdhttp.Cookie, string, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"`+email+`","password":"`+password+`"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("login %s expected 200, got %d with body %s", email, recorder.Code, recorder.Body.String())
	}
	cookie := firstCookieNamed(t, recorder, testConfig().SessionCookieName)
	csrfToken := csrfTokenFromRecorder(t, recorder)

	var response struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return cookie, csrfToken, response.Data.User.ID
}

func firstCookieNamed(t *testing.T, recorder *httptest.ResponseRecorder, name string) *stdhttp.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %s", name)
	return nil
}

func promoteHTTPUserToAdmin(t *testing.T, database *sql.DB, userID string) {
	t.Helper()

	if _, err := database.Exec(`UPDATE users SET role = 'admin' WHERE id = $1`, userID); err != nil {
		t.Fatalf("promote user to admin: %v", err)
	}
}

func createHTTPOrganization(t *testing.T, router stdhttp.Handler, cookie *stdhttp.Cookie, csrfToken, name, slug string) string {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/organizations", strings.NewReader(`{"name":"`+name+`","slug":"`+slug+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create organization expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode organization response: %v", err)
	}
	if response.Data.ID == "" {
		t.Fatal("expected organization id")
	}
	return response.Data.ID
}

func queryHTTPUserScope(t *testing.T, database *sql.DB, userID string) (string, string) {
	t.Helper()

	var workspaceID string
	var organizationID string
	if err := database.QueryRow(`SELECT id, organization_id FROM workspaces WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1`, userID).Scan(&workspaceID, &organizationID); err != nil {
		t.Fatalf("query user workspace scope: %v", err)
	}
	if workspaceID == "" || organizationID == "" {
		t.Fatalf("expected user workspace and organization, got workspace=%q organization=%q", workspaceID, organizationID)
	}
	return workspaceID, organizationID
}

func inviteHTTPMember(t *testing.T, router stdhttp.Handler, cookie *stdhttp.Cookie, csrfToken, organizationID, email string) (string, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organizations/"+organizationID+"/invitations", strings.NewReader(`{"email":"`+email+`","role":"member"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("invite member expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode invitation response: %v", err)
	}
	if response.Data.ID == "" || response.Data.Token == "" {
		t.Fatalf("expected invitation id and token, got id=%q token=%q", response.Data.ID, response.Data.Token)
	}
	return response.Data.ID, response.Data.Token
}

func acceptHTTPInvitation(t *testing.T, router stdhttp.Handler, cookie *stdhttp.Cookie, csrfToken, token string) *stdhttp.Cookie {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organization-invitations/"+token+"/accept", nil)
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)
	router.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("accept invitation expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	return firstCookieNamed(t, recorder, testConfig().SessionCookieName)
}

func TestRegisterCreatesDefaultOrganizationAndSessionScope(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, _, userID := registerHTTPUser(t, router, "tenant-default@example.com")

	var workspaceOrganizationID sql.NullString
	if err := database.QueryRow(`SELECT organization_id FROM workspaces WHERE user_id = $1`, userID).Scan(&workspaceOrganizationID); err != nil {
		t.Fatalf("query workspace organization: %v", err)
	}
	if !workspaceOrganizationID.Valid || workspaceOrganizationID.String == "" {
		t.Fatal("expected registration to assign workspace organization_id")
	}

	var sessionOrganizationID sql.NullString
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1`, userID).Scan(&sessionOrganizationID); err != nil {
		t.Fatalf("query session organization: %v", err)
	}
	if !sessionOrganizationID.Valid || sessionOrganizationID.String != workspaceOrganizationID.String {
		t.Fatalf("expected session organization %q, got %q", workspaceOrganizationID.String, sessionOrganizationID.String)
	}

	var role string
	if err := database.QueryRow(`
		SELECT role
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
	`, workspaceOrganizationID.String, userID).Scan(&role); err != nil {
		t.Fatalf("query owner membership: %v", err)
	}
	if role != "owner" {
		t.Fatalf("expected default organization owner role, got %q", role)
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("me expected 200, got %d with body %s", meRecorder.Code, meRecorder.Body.String())
	}

	var response struct {
		Data struct {
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
			Workspace struct {
				ID string `json:"id"`
			} `json:"workspace"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if response.Data.Workspace.ID == "" {
		t.Fatal("expected existing workspace payload to remain present")
	}
	if response.Data.Organization.ID != workspaceOrganizationID.String {
		t.Fatalf("expected me organization %q, got %q", workspaceOrganizationID.String, response.Data.Organization.ID)
	}
}

func TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	ownerCookie, ownerCSRF, ownerID := registerHTTPUser(t, router, "tenant-owner@example.com")
	promoteHTTPUserToAdmin(t, database, ownerID)
	organizationID := createHTTPOrganization(t, router, ownerCookie, ownerCSRF, "Tenant Scope Org", "tenant-scope-org")

	otherCookie, otherCSRF, _ := registerHTTPUser(t, router, "tenant-other@example.com")
	forbiddenRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organizations/"+organizationID+"/select", nil)
	forbiddenRequest.AddCookie(otherCookie)
	addCSRF(forbiddenRequest, otherCSRF)
	router.ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("expected non-member select to return 403, got %d with body %s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
	}

	memberCookie, memberCSRF, memberID := registerHTTPUser(t, router, "tenant-member@example.com")
	_, token := inviteHTTPMember(t, router, ownerCookie, ownerCSRF, organizationID, "tenant-member@example.com")
	memberCookie = acceptHTTPInvitation(t, router, memberCookie, memberCSRF, token)
	memberCSRF = csrfTokenForCookie(t, router, memberCookie)

	selectRecorder := httptest.NewRecorder()
	selectRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organizations/"+organizationID+"/select", nil)
	selectRequest.AddCookie(memberCookie)
	addCSRF(selectRequest, memberCSRF)
	router.ServeHTTP(selectRecorder, selectRequest)
	if selectRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected member select to return 200, got %d with body %s", selectRecorder.Code, selectRecorder.Body.String())
	}

	var sessionOrganizationID sql.NullString
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, memberID).Scan(&sessionOrganizationID); err != nil {
		t.Fatalf("query selected session organization: %v", err)
	}
	if !sessionOrganizationID.Valid || sessionOrganizationID.String != organizationID {
		t.Fatalf("expected selected session organization %q, got %q", organizationID, sessionOrganizationID.String)
	}
}

func TestLoginResolvesDefaultOrganizationForLegacyUser(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("StrongerPass1!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO users (id, email, password_hash, role, name) VALUES ($1, $2, $3, 'user', $4)`, "user_legacy", "legacy-scope@example.com", string(passwordHash), "Legacy Scope"); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO organizations (id, slug, name, status, created_by_user_id) VALUES ($1, $2, $3, 'active', $4)`, "org_legacy", "legacy-scope", "Legacy Scope", "user_legacy"); err != nil {
		t.Fatalf("insert legacy organization: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO workspaces (id, user_id, organization_id, name) VALUES ($1, $2, $3, $4)`, "workspace_legacy", "user_legacy", "org_legacy", "Legacy Workspace"); err != nil {
		t.Fatalf("insert legacy workspace: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id) VALUES ($1, $2, $3, 'owner', $4)`, "membership_legacy", "org_legacy", "user_legacy", "user_legacy"); err != nil {
		t.Fatalf("insert legacy membership: %v", err)
	}

	cookie, _, userID := loginHTTPUser(t, router, "legacy-scope@example.com", "StrongerPass1!")
	if userID != "user_legacy" {
		t.Fatalf("expected legacy user id, got %q", userID)
	}

	var sessionOrganizationID sql.NullString
	if err := database.QueryRow(`SELECT organization_id FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID).Scan(&sessionOrganizationID); err != nil {
		t.Fatalf("query legacy login session organization: %v", err)
	}
	if !sessionOrganizationID.Valid || sessionOrganizationID.String != "org_legacy" {
		t.Fatalf("expected legacy login session organization org_legacy, got %q", sessionOrganizationID.String)
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("legacy me expected 200, got %d with body %s", meRecorder.Code, meRecorder.Body.String())
	}
	var response struct {
		Data struct {
			Organization struct {
				ID string `json:"id"`
			} `json:"organization"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode legacy me response: %v", err)
	}
	if response.Data.Organization.ID != "org_legacy" {
		t.Fatalf("expected legacy me organization org_legacy, got %q", response.Data.Organization.ID)
	}
}

func TestCrossTenantChatScopeUsesActiveOrganization(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-chat@example.com")
	workspaceID, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Chat Org", "other-chat-org")

	if _, err := database.Exec(`
		INSERT INTO conversations (id, workspace_id, organization_id, title, created_at, updated_at)
		VALUES ('conversation_other_org', $1, $2, 'Other org conversation', NOW(), NOW())
	`, workspaceID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org conversation: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO messages (id, conversation_id, organization_id, role, content, created_at)
		VALUES ('message_other_org', 'conversation_other_org', $1, 'user', 'secret from other org', NOW())
	`, otherOrganizationID); err != nil {
		t.Fatalf("insert other org message: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list conversations expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode conversation list: %v", err)
	}
	for _, conversation := range listResponse.Data {
		if conversation.ID == "conversation_other_org" {
			t.Fatalf("active organization %s must not list conversation from organization %s", activeOrganizationID, otherOrganizationID)
		}
	}

	messagesRecorder := httptest.NewRecorder()
	messagesRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/conversation_other_org/messages", nil)
	messagesRequest.AddCookie(cookie)
	router.ServeHTTP(messagesRecorder, messagesRequest)
	if messagesRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant message read to return 404, got %d with body %s", messagesRecorder.Code, messagesRecorder.Body.String())
	}

	sendRecorder := httptest.NewRecorder()
	sendRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_other_org/messages", strings.NewReader(`{"content":"mutate other tenant"}`))
	sendRequest.Header.Set("Content-Type", "application/json")
	sendRequest.AddCookie(cookie)
	addCSRF(sendRequest, csrfToken)
	router.ServeHTTP(sendRecorder, sendRequest)
	if sendRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant message write to return 404, got %d with body %s", sendRecorder.Code, sendRecorder.Body.String())
	}

	var messageCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = 'conversation_other_org'`).Scan(&messageCount); err != nil {
		t.Fatalf("count other org messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("expected denied write to leave other org message count at 1, got %d", messageCount)
	}
}

func TestCrossTenantKnowledgeScopeDeniesReadWriteAndAttach(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-knowledge@example.com")
	workspaceID, _ := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Knowledge Org", "other-knowledge-org")

	if _, err := database.Exec(`
		INSERT INTO knowledge_bases (id, workspace_id, organization_id, name, document_count, created_at, updated_at)
		VALUES ('kb_other_org', $1, $2, 'Other org knowledge', 1, NOW(), NOW())
	`, workspaceID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org knowledge base: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO knowledge_documents (id, knowledge_base_id, organization_id, title, content, created_at, updated_at)
		VALUES ('doc_other_org', 'kb_other_org', $1, 'Other org doc', 'tenant-only knowledge', NOW(), NOW())
	`, otherOrganizationID); err != nil {
		t.Fatalf("insert other org knowledge document: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO knowledge_document_chunks (id, document_id, organization_id, chunk_index, content, created_at)
		VALUES ('chunk_other_org', 'doc_other_org', $1, 0, 'tenant-only knowledge', NOW())
	`, otherOrganizationID); err != nil {
		t.Fatalf("insert other org knowledge chunk: %v", err)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/knowledge-bases/kb_other_org", nil)
	getRequest.AddCookie(cookie)
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant knowledge read to return 404, got %d with body %s", getRecorder.Code, getRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/knowledge-bases/kb_other_org", strings.NewReader(`{"name":"Mutated"}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	addCSRF(updateRequest, csrfToken)
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant knowledge update to return 404, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	deleteDocumentRecorder := httptest.NewRecorder()
	deleteDocumentRequest := httptest.NewRequest(stdhttp.MethodDelete, "/api/v1/app/knowledge-bases/kb_other_org/documents/doc_other_org", nil)
	deleteDocumentRequest.AddCookie(cookie)
	addCSRF(deleteDocumentRequest, csrfToken)
	router.ServeHTTP(deleteDocumentRecorder, deleteDocumentRequest)
	if deleteDocumentRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant document delete to return 404, got %d with body %s", deleteDocumentRecorder.Code, deleteDocumentRecorder.Body.String())
	}

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"Tenant config"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
	addCSRF(createConversationRequest, csrfToken)
	router.ServeHTTP(createConversationRecorder, createConversationRequest)
	if createConversationRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("create conversation expected 200, got %d with body %s", createConversationRecorder.Code, createConversationRecorder.Body.String())
	}
	var conversationResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createConversationRecorder.Body.Bytes(), &conversationResponse); err != nil {
		t.Fatalf("decode created conversation: %v", err)
	}

	updateConfigRecorder := httptest.NewRecorder()
	updateConfigRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/conversations/"+conversationResponse.Data.ID+"/config", strings.NewReader(`{"knowledgeBaseIds":["kb_other_org"]}`))
	updateConfigRequest.Header.Set("Content-Type", "application/json")
	updateConfigRequest.AddCookie(cookie)
	addCSRF(updateConfigRequest, csrfToken)
	router.ServeHTTP(updateConfigRecorder, updateConfigRequest)
	if updateConfigRecorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("expected cross-tenant knowledge attach to return 404, got %d with body %s", updateConfigRecorder.Code, updateConfigRecorder.Body.String())
	}

	var bindingCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM conversation_knowledge_bindings WHERE knowledge_base_id = 'kb_other_org'`).Scan(&bindingCount); err != nil {
		t.Fatalf("count cross-tenant bindings: %v", err)
	}
	if bindingCount != 0 {
		t.Fatalf("expected denied attach to create zero bindings, got %d", bindingCount)
	}

	var knowledgeName string
	if err := database.QueryRow(`SELECT name FROM knowledge_bases WHERE id = 'kb_other_org'`).Scan(&knowledgeName); err != nil {
		t.Fatalf("query knowledge name after denied update: %v", err)
	}
	if knowledgeName != "Other org knowledge" {
		t.Fatalf("expected denied update to preserve knowledge name, got %q", knowledgeName)
	}
}

func TestCrossTenantConsoleUsageUsesActiveOrganization(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, _, userID := registerHTTPUser(t, router, "cross-console@example.com")
	workspaceID, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfTokenForCookie(t, router, cookie), "Other Console Org", "other-console-org")

	if _, err := database.Exec(`
		INSERT INTO usage_records (id, user_id, workspace_id, organization_id, model_id, request_count, input_tokens, output_tokens, created_at)
		VALUES
			('usage_active_org', $1, $2, $3, 'balanced-chat', 2, 10, 20, NOW()),
			('usage_other_org', $1, $2, $4, 'quality-chat', 5, 100, 200, NOW())
	`, userID, workspaceID, activeOrganizationID, otherOrganizationID); err != nil {
		t.Fatalf("insert tenant usage records: %v", err)
	}

	usageRecorder := httptest.NewRecorder()
	usageRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/usage", nil)
	usageRequest.AddCookie(cookie)
	router.ServeHTTP(usageRecorder, usageRequest)
	if usageRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("console usage expected 200, got %d with body %s", usageRecorder.Code, usageRecorder.Body.String())
	}
	var usageResponse struct {
		Data struct {
			Requests int `json:"requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(usageRecorder.Body.Bytes(), &usageResponse); err != nil {
		t.Fatalf("decode console usage: %v", err)
	}
	if usageResponse.Data.Requests != 2 {
		t.Fatalf("expected active organization requests 2, got %d", usageResponse.Data.Requests)
	}

	billingRecorder := httptest.NewRecorder()
	billingRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/billing", nil)
	billingRequest.AddCookie(cookie)
	router.ServeHTTP(billingRecorder, billingRequest)
	if billingRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("console billing expected 200, got %d with body %s", billingRecorder.Code, billingRecorder.Body.String())
	}
	var billingResponse struct {
		Data struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			Requests     int `json:"requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(billingRecorder.Body.Bytes(), &billingResponse); err != nil {
		t.Fatalf("decode console billing: %v", err)
	}
	if billingResponse.Data.Requests != 2 || billingResponse.Data.InputTokens != 10 || billingResponse.Data.OutputTokens != 20 {
		t.Fatalf("expected active organization billing requests=2 input=10 output=20, got %+v", billingResponse.Data)
	}

	modelsRecorder := httptest.NewRecorder()
	modelsRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/models", nil)
	modelsRequest.AddCookie(cookie)
	router.ServeHTTP(modelsRecorder, modelsRequest)
	if modelsRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("console models expected 200, got %d with body %s", modelsRecorder.Code, modelsRecorder.Body.String())
	}
	var modelsResponse struct {
		Data []struct {
			ID       string `json:"id"`
			Requests int    `json:"requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(modelsRecorder.Body.Bytes(), &modelsResponse); err != nil {
		t.Fatalf("decode console models: %v", err)
	}
	if len(modelsResponse.Data) != 1 || modelsResponse.Data[0].ID != "balanced-chat" || modelsResponse.Data[0].Requests != 2 {
		t.Fatalf("expected only active organization model summary, got %+v", modelsResponse.Data)
	}
}

func TestCrossTenantAgentScopeDeniesReadWriteAndConversation(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-agent@example.com")
	_, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Agent Org", "other-agent-org")

	if _, err := database.Exec(`
		INSERT INTO agents (id, user_id, organization_id, name, description, model, system_prompt, tools, config, is_public, created_at, updated_at)
		VALUES ('agent_other_org', $1, $2, 'Other org agent', '', 'demo-reply', '', '[]', '{}', FALSE, NOW(), NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_conversations (id, agent_id, user_id, organization_id, title, created_at, updated_at)
		VALUES ('agent_conversation_other_org', 'agent_other_org', $1, $2, 'Other org conversation', NOW(), NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org agent conversation: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_messages (id, conversation_id, organization_id, role, content, tool_calls, created_at)
		VALUES ('agent_message_other_org', 'agent_conversation_other_org', $1, 'user', 'tenant-only agent message', '[]', NOW())
	`, otherOrganizationID); err != nil {
		t.Fatalf("insert other org agent message: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/agents", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list agents expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode agent list: %v", err)
	}
	for _, agent := range listResponse.Data {
		if agent.ID == "agent_other_org" {
			t.Fatalf("active organization %s must not list agent from organization %s", activeOrganizationID, otherOrganizationID)
		}
	}

	for _, requestCase := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "get agent", method: stdhttp.MethodGet, path: "/api/v1/app/agents/agent_other_org"},
		{name: "update agent", method: stdhttp.MethodPut, path: "/api/v1/app/agents/agent_other_org", body: `{"name":"Mutated"}`},
		{name: "delete agent", method: stdhttp.MethodDelete, path: "/api/v1/app/agents/agent_other_org"},
		{name: "create conversation", method: stdhttp.MethodPost, path: "/api/v1/app/agents/agent_other_org/conversations"},
		{name: "list conversations", method: stdhttp.MethodGet, path: "/api/v1/app/agents/agent_other_org/conversations"},
		{name: "get conversation", method: stdhttp.MethodGet, path: "/api/v1/app/agents/conversations/agent_conversation_other_org"},
		{name: "delete conversation", method: stdhttp.MethodDelete, path: "/api/v1/app/agents/conversations/agent_conversation_other_org"},
		{name: "send message", method: stdhttp.MethodPost, path: "/api/v1/app/agents/conversations/agent_conversation_other_org/messages", body: `{"content":"mutate other org"}`},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(requestCase.method, requestCase.path, strings.NewReader(requestCase.body))
		if requestCase.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		request.AddCookie(cookie)
		if requestCase.method != stdhttp.MethodGet {
			addCSRF(request, csrfToken)
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusNotFound {
			t.Fatalf("%s expected 404, got %d with body %s", requestCase.name, recorder.Code, recorder.Body.String())
		}
	}

	var agentName string
	if err := database.QueryRow(`SELECT name FROM agents WHERE id = 'agent_other_org'`).Scan(&agentName); err != nil {
		t.Fatalf("query other org agent: %v", err)
	}
	if agentName != "Other org agent" {
		t.Fatalf("expected denied update to preserve agent name, got %q", agentName)
	}
	var messageCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_messages WHERE conversation_id = 'agent_conversation_other_org'`).Scan(&messageCount); err != nil {
		t.Fatalf("count other org agent messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("expected denied send to leave other org agent message count at 1, got %d", messageCount)
	}
}

func TestCrossTenantMemoryScopeDeniesReadWrite(t *testing.T) {
	database := testDatabase(t)
	cfg := testConfig()
	cfg.RelayEnabled = true
	router := NewRouter(cfg, database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-memory@example.com")
	_, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Memory Org", "other-memory-org")

	if _, err := database.Exec(`
		INSERT INTO memory_documents (id, user_id, organization_id, title, content, source_type, metadata, total_chunks, embedding_model, created_at, updated_at)
		VALUES ('memory_doc_other_org', $1, $2, 'Other org memory', 'tenant-only memory', 'manual', '{}', 1, 'text-embedding-3-small', NOW(), NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org memory document: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO memory_chunks (id, document_id, user_id, organization_id, content, chunk_index, embedding, metadata, created_at)
		VALUES ('memory_chunk_other_org', 'memory_doc_other_org', $1, $2, 'tenant-only memory', 0, '[0.1,0.2]', '{}', NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org memory chunk: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/memory/documents", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list memory expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode memory list: %v", err)
	}
	for _, document := range listResponse.Data {
		if document.ID == "memory_doc_other_org" {
			t.Fatalf("active organization %s must not list memory document from organization %s", activeOrganizationID, otherOrganizationID)
		}
	}

	for _, requestCase := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "get memory", method: stdhttp.MethodGet, path: "/api/v1/app/memory/documents/memory_doc_other_org"},
		{name: "update memory", method: stdhttp.MethodPut, path: "/api/v1/app/memory/documents/memory_doc_other_org", body: `{"title":"Mutated","content":"tenant-only memory"}`},
		{name: "list chunks", method: stdhttp.MethodGet, path: "/api/v1/app/memory/documents/memory_doc_other_org/chunks"},
		{name: "delete memory", method: stdhttp.MethodDelete, path: "/api/v1/app/memory/documents/memory_doc_other_org"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(requestCase.method, requestCase.path, strings.NewReader(requestCase.body))
		if requestCase.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		request.AddCookie(cookie)
		if requestCase.method != stdhttp.MethodGet {
			addCSRF(request, csrfToken)
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusNotFound {
			t.Fatalf("%s expected 404, got %d with body %s", requestCase.name, recorder.Code, recorder.Body.String())
		}
	}

	var memoryTitle string
	if err := database.QueryRow(`SELECT title FROM memory_documents WHERE id = 'memory_doc_other_org'`).Scan(&memoryTitle); err != nil {
		t.Fatalf("query other org memory title: %v", err)
	}
	if memoryTitle != "Other org memory" {
		t.Fatalf("expected denied update to preserve memory title, got %q", memoryTitle)
	}
}

func TestCrossTenantMCPScopeDeniesReadWriteAndConnect(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-mcp@example.com")
	_, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other MCP Org", "other-mcp-org")

	if _, err := database.Exec(`
		INSERT INTO mcp_servers (id, user_id, organization_id, name, url, auth_token_encrypted, status, created_at, updated_at)
		VALUES ('mcp_other_org', $1, $2, 'Other org MCP', 'http://127.0.0.1:1', '', 'disconnected', NOW(), NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org MCP server: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/mcp-servers", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list MCP expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode MCP list: %v", err)
	}
	for _, server := range listResponse.Data {
		if server.ID == "mcp_other_org" {
			t.Fatalf("active organization %s must not list MCP server from organization %s", activeOrganizationID, otherOrganizationID)
		}
	}

	for _, requestCase := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "get MCP", method: stdhttp.MethodGet, path: "/api/v1/app/mcp-servers/mcp_other_org"},
		{name: "connect MCP", method: stdhttp.MethodPost, path: "/api/v1/app/mcp-servers/mcp_other_org/connect"},
		{name: "list MCP tools", method: stdhttp.MethodGet, path: "/api/v1/app/mcp-servers/mcp_other_org/tools"},
		{name: "delete MCP", method: stdhttp.MethodDelete, path: "/api/v1/app/mcp-servers/mcp_other_org"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(requestCase.method, requestCase.path, nil)
		request.AddCookie(cookie)
		if requestCase.method != stdhttp.MethodGet {
			addCSRF(request, csrfToken)
		}
		router.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusNotFound {
			t.Fatalf("%s expected 404, got %d with body %s", requestCase.name, recorder.Code, recorder.Body.String())
		}
	}

	var serverCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM mcp_servers WHERE id = 'mcp_other_org'`).Scan(&serverCount); err != nil {
		t.Fatalf("count other org MCP server: %v", err)
	}
	if serverCount != 1 {
		t.Fatalf("expected denied delete to preserve other org MCP server, got count %d", serverCount)
	}
}

func TestCrossTenantQuotaScopeUsesActiveOrganization(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "cross-quota@example.com")
	_, activeOrganizationID := queryHTTPUserScope(t, database, userID)
	promoteHTTPUserToAdmin(t, database, userID)
	otherOrganizationID := createHTTPOrganization(t, router, cookie, csrfToken, "Other Quota Org", "other-quota-org")

	if _, err := database.Exec(`
		INSERT INTO quotas (id, user_id, organization_id, balance, used, created_at, updated_at)
		VALUES ('quota_other_org', $1, $2, 123, 7, NOW(), NOW())
	`, userID, otherOrganizationID); err != nil {
		t.Fatalf("insert other org quota: %v", err)
	}

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/quota", nil)
	getRequest.AddCookie(cookie)
	router.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("get quota expected 200, got %d with body %s", getRecorder.Code, getRecorder.Body.String())
	}
	var getResponse struct {
		Data struct {
			OrganizationID string  `json:"organizationId"`
			Balance        float64 `json:"balance"`
			Used           float64 `json:"used"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getResponse); err != nil {
		t.Fatalf("decode quota: %v", err)
	}
	if getResponse.Data.OrganizationID != activeOrganizationID {
		t.Fatalf("expected active organization quota %s, got %s", activeOrganizationID, getResponse.Data.OrganizationID)
	}
	if getResponse.Data.Balance != 0 || getResponse.Data.Used != 0 {
		t.Fatalf("expected empty active org quota, got %+v", getResponse.Data)
	}

	topupRecorder := httptest.NewRecorder()
	topupRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/quota/topup", strings.NewReader(`{"amount":5}`))
	topupRequest.Header.Set("Content-Type", "application/json")
	topupRequest.AddCookie(cookie)
	addCSRF(topupRequest, csrfToken)
	router.ServeHTTP(topupRecorder, topupRequest)
	if topupRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("topup expected 200, got %d with body %s", topupRecorder.Code, topupRecorder.Body.String())
	}

	var activeBalance, otherBalance float64
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = $1`, activeOrganizationID).Scan(&activeBalance); err != nil {
		t.Fatalf("query active org quota: %v", err)
	}
	if err := database.QueryRow(`SELECT balance FROM quotas WHERE organization_id = $1`, otherOrganizationID).Scan(&otherBalance); err != nil {
		t.Fatalf("query other org quota: %v", err)
	}
	if activeBalance != 5 {
		t.Fatalf("expected active organization balance 5 after topup, got %.2f", activeBalance)
	}
	if otherBalance != 123 {
		t.Fatalf("expected other organization balance unchanged at 123, got %.2f", otherBalance)
	}
}

func TestHealthz(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestRegisterLoginMeLogoutFlow(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)

	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d", registerRecorder.Code)
	}
	cookie := registerRecorder.Result().Cookies()[0]
	csrfToken := csrfTokenFromRecorder(t, registerRecorder)
	if cookie.Name != testConfig().SessionCookieName {
		t.Fatalf("expected session cookie %s, got %s", testConfig().SessionCookieName, cookie.Name)
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("me expected 200, got %d", meRecorder.Code)
	}

	var meResponse struct {
		Data struct {
			Session struct {
				ID string `json:"id"`
			} `json:"session"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &meResponse); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if cookie.Value == meResponse.Data.Session.ID {
		t.Fatal("expected session cookie to be signed instead of exposing raw session id")
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	addCSRF(logoutRequest, csrfToken)
	router.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("logout expected 200, got %d", logoutRecorder.Code)
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"StrongerPass1!"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("login expected 200, got %d", loginRecorder.Code)
	}
}

func TestAuthRateLimitRejectsRepeatedFailedLogin(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"limited@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d with body %s", registerRecorder.Code, registerRecorder.Body.String())
	}

	var lastCode int
	for i := 0; i < 6; i++ {
		loginRecorder := httptest.NewRecorder()
		loginRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"limited@example.com","password":"WrongPass1!"}`))
		loginRequest.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(loginRecorder, loginRequest)
		lastCode = loginRecorder.Code
	}
	if lastCode != stdhttp.StatusTooManyRequests {
		t.Fatalf("expected repeated failed login to return 429, got %d", lastCode)
	}
}

func TestPasswordResetRoutesConfirmAndRevokeSessions(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"reset@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d with body %s", registerRecorder.Code, registerRecorder.Body.String())
	}
	oldCookie := registerRecorder.Result().Cookies()[0]

	resetRecorder := httptest.NewRecorder()
	resetRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"reset@example.com"}`))
	resetRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resetRecorder, resetRequest)
	if resetRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("password reset request expected 200, got %d with body %s", resetRecorder.Code, resetRecorder.Body.String())
	}
	var resetResponse struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resetRecorder.Body.Bytes(), &resetResponse); err != nil {
		t.Fatalf("decode password reset response: %v", err)
	}
	if resetResponse.Data.Token == "" {
		t.Fatal("expected test password reset token")
	}

	confirmRecorder := httptest.NewRecorder()
	confirmRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"`+resetResponse.Data.Token+`","password":"EvenStrongerPass2!"}`))
	confirmRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(confirmRecorder, confirmRequest)
	if confirmRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("password reset confirm expected 200, got %d with body %s", confirmRecorder.Code, confirmRecorder.Body.String())
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(oldCookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected old session to be revoked after reset, got %d with body %s", meRecorder.Code, meRecorder.Body.String())
	}
}

func TestOrganizationInvitationRevokeRejectsAcceptance(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	ownerCookie, ownerCSRF, ownerID := registerHTTPUser(t, router, "org-owner@example.com")
	promoteHTTPUserToAdmin(t, database, ownerID)
	organizationID := createHTTPOrganization(t, router, ownerCookie, ownerCSRF, "Acme", "acme")
	targetCookie, targetCSRF, _ := registerHTTPUser(t, router, "revoked-target@example.com")

	invitationID, token := inviteHTTPMember(t, router, ownerCookie, ownerCSRF, organizationID, "revoked-target@example.com")

	revokeRecorder := httptest.NewRecorder()
	revokeRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organizations/"+organizationID+"/invitations/"+invitationID+"/revoke", nil)
	revokeRequest.AddCookie(ownerCookie)
	addCSRF(revokeRequest, ownerCSRF)
	router.ServeHTTP(revokeRecorder, revokeRequest)
	if revokeRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("revoke invitation expected 200, got %d with body %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}

	var revokeResponse struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(revokeRecorder.Body.Bytes(), &revokeResponse); err != nil {
		t.Fatalf("decode revoke response: %v", err)
	}
	if revokeResponse.Data.Status != "revoked" {
		t.Fatalf("expected revoked invitation status, got %q", revokeResponse.Data.Status)
	}

	var revokeAuditCount int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM audit_logs
		WHERE resource_id = $1 AND action = 'organization.member.invitation_revoke'
	`, organizationID).Scan(&revokeAuditCount); err != nil {
		t.Fatalf("count revoke audit logs: %v", err)
	}
	if revokeAuditCount != 1 {
		t.Fatalf("expected one revoke audit log, got %d", revokeAuditCount)
	}

	acceptRecorder := httptest.NewRecorder()
	acceptRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organization-invitations/"+token+"/accept", nil)
	acceptRequest.AddCookie(targetCookie)
	addCSRF(acceptRequest, targetCSRF)
	router.ServeHTTP(acceptRecorder, acceptRequest)
	if acceptRecorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected revoked invitation accept to return 409, got %d with body %s", acceptRecorder.Code, acceptRecorder.Body.String())
	}
}

func TestOrganizationSessionSecurityOnMembershipChanges(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	ownerCookie, ownerCSRF, ownerID := registerHTTPUser(t, router, "session-owner@example.com")
	promoteHTTPUserToAdmin(t, database, ownerID)
	organizationID := createHTTPOrganization(t, router, ownerCookie, ownerCSRF, "Session Org", "session-org")
	memberCookie, memberCSRF, memberID := registerHTTPUser(t, router, "session-member@example.com")
	_, token := inviteHTTPMember(t, router, ownerCookie, ownerCSRF, organizationID, "session-member@example.com")

	acceptRecorder := httptest.NewRecorder()
	acceptRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/organization-invitations/"+token+"/accept", nil)
	acceptRequest.AddCookie(memberCookie)
	addCSRF(acceptRequest, memberCSRF)
	router.ServeHTTP(acceptRecorder, acceptRequest)
	if acceptRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("accept invitation expected 200, got %d with body %s", acceptRecorder.Code, acceptRecorder.Body.String())
	}
	rotatedCookie := firstCookieNamed(t, acceptRecorder, testConfig().SessionCookieName)

	oldMeRecorder := httptest.NewRecorder()
	oldMeRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	oldMeRequest.AddCookie(memberCookie)
	router.ServeHTTP(oldMeRecorder, oldMeRequest)
	if oldMeRecorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected old accepted session to be rotated away, got %d with body %s", oldMeRecorder.Code, oldMeRecorder.Body.String())
	}

	secondMemberCookie, _, _ := loginHTTPUser(t, router, "session-member@example.com", "StrongerPass1!")

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/organizations/"+organizationID+"/members/"+memberID, strings.NewReader(`{"role":"admin"}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(ownerCookie)
	addCSRF(updateRequest, ownerCSRF)
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update member role expected 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	for name, cookie := range map[string]*stdhttp.Cookie{
		"rotated": rotatedCookie,
		"second":  secondMemberCookie,
	} {
		meRecorder := httptest.NewRecorder()
		meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
		meRequest.AddCookie(cookie)
		router.ServeHTTP(meRecorder, meRequest)
		if meRecorder.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("expected %s member session to be revoked after role update, got %d with body %s", name, meRecorder.Code, meRecorder.Body.String())
		}
	}
}

func TestSensitiveOrganizationActionsAreRateLimited(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	adminCookie, adminCSRF, adminID := registerHTTPUser(t, router, "rate-admin@example.com")
	promoteHTTPUserToAdmin(t, database, adminID)

	var lastCode int
	for i := 0; i < 6; i++ {
		recorder := httptest.NewRecorder()
		body := strings.NewReader(`{"name":"Rate Org ` + string(rune('A'+i)) + `","slug":"rate-org-` + string(rune('a'+i)) + `"}`)
		request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/admin/organizations", body)
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(adminCookie)
		addCSRF(request, adminCSRF)
		router.ServeHTTP(recorder, request)
		lastCode = recorder.Code
	}
	if lastCode != stdhttp.StatusTooManyRequests {
		t.Fatalf("expected repeated sensitive organization writes to return 429, got %d", lastCode)
	}
}

func TestConversationAndMessageFlow(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"chat@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]
	csrfToken := csrfTokenFromRecorder(t, registerRecorder)

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"First chat"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
	addCSRF(createConversationRequest, csrfToken)
	router.ServeHTTP(createConversationRecorder, createConversationRequest)
	if createConversationRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("create conversation expected 200, got %d", createConversationRecorder.Code)
	}

	var createdConversation Envelope
	if err := json.Unmarshal(createConversationRecorder.Body.Bytes(), &createdConversation); err != nil {
		t.Fatalf("decode conversation response: %v", err)
	}
	conversationData, ok := createdConversation.Data.(map[string]any)
	if !ok {
		var typed struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(createConversationRecorder.Body.Bytes(), &typed); err != nil {
			t.Fatalf("decode typed conversation response: %v", err)
		}
		conversationData = map[string]any{"id": typed.Data.ID}
	}
	conversationID := conversationData["id"].(string)

	sendRecorder := httptest.NewRecorder()
	sendRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/"+conversationID+"/messages", strings.NewReader(`{"content":"hello"}`))
	sendRequest.Header.Set("Content-Type", "application/json")
	sendRequest.AddCookie(cookie)
	addCSRF(sendRequest, csrfToken)
	router.ServeHTTP(sendRecorder, sendRequest)
	if sendRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("send message expected 200, got %d", sendRecorder.Code)
	}

	messagesRecorder := httptest.NewRecorder()
	messagesRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/"+conversationID+"/messages", nil)
	messagesRequest.AddCookie(cookie)
	router.ServeHTTP(messagesRecorder, messagesRequest)
	if messagesRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list messages expected 200, got %d", messagesRecorder.Code)
	}
}

func TestConsoleUsageReflectsRecordedChatRequests(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"usage@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]
	csrfToken := csrfTokenFromRecorder(t, registerRecorder)

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"Usage chat"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
	addCSRF(createConversationRequest, csrfToken)
	router.ServeHTTP(createConversationRecorder, createConversationRequest)

	var createdConversation struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createConversationRecorder.Body.Bytes(), &createdConversation); err != nil {
		t.Fatalf("decode conversation response: %v", err)
	}

	sendRecorder := httptest.NewRecorder()
	sendRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/"+createdConversation.Data.ID+"/messages", strings.NewReader(`{"content":"track this request"}`))
	sendRequest.Header.Set("Content-Type", "application/json")
	sendRequest.AddCookie(cookie)
	addCSRF(sendRequest, csrfToken)
	router.ServeHTTP(sendRecorder, sendRequest)
	if sendRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("send message expected 200, got %d", sendRecorder.Code)
	}

	var usageCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM usage_records`).Scan(&usageCount); err != nil {
		t.Fatalf("count usage records: %v", err)
	}
	if usageCount != 1 {
		t.Fatalf("expected 1 usage record, got %d", usageCount)
	}

	usageRecorder := httptest.NewRecorder()
	usageRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/console/usage", nil)
	usageRequest.AddCookie(cookie)
	router.ServeHTTP(usageRecorder, usageRequest)
	if usageRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("console usage expected 200, got %d with body %s", usageRecorder.Code, usageRecorder.Body.String())
	}

	var usageResponse struct {
		Data struct {
			Period   string `json:"period"`
			Requests int    `json:"requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(usageRecorder.Body.Bytes(), &usageResponse); err != nil {
		t.Fatalf("decode usage response: %v", err)
	}
	if usageResponse.Data.Period != "7d" {
		t.Fatalf("expected period 7d, got %q", usageResponse.Data.Period)
	}
	if usageResponse.Data.Requests != 1 {
		t.Fatalf("expected requests 1, got %d", usageResponse.Data.Requests)
	}
}

func TestRegisterStoresHashedPassword(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"hash@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)

	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d", registerRecorder.Code)
	}

	var storedPassword string
	if err := database.QueryRow(`SELECT password_hash FROM users WHERE email = $1`, "hash@example.com").Scan(&storedPassword); err != nil {
		t.Fatalf("query password hash: %v", err)
	}
	if storedPassword == "StrongerPass1!" {
		t.Fatalf("expected stored password hash to differ from raw password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte("StrongerPass1!")); err != nil {
		t.Fatalf("expected stored hash to match password: %v", err)
	}
}

func TestLoginAcceptsRawPasswordAgainstStoredHash(t *testing.T) {
	database := testDatabase(t)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("StrongerPass1!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, "user_hashed", "hashed@example.com", string(passwordHash)); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, $3)`, "workspace_hashed", "user_hashed", "Default Workspace"); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	router := NewRouter(testConfig(), database)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"hashed@example.com","password":"StrongerPass1!"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("login expected 200, got %d with body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestMeReturnsExpandedSessionPayload(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"state@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("me expected 200, got %d", meRecorder.Code)
	}

	var response struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
			Workspace struct {
				ID string `json:"id"`
			} `json:"workspace"`
			Session struct {
				ExpiresAt string `json:"expiresAt"`
				ID        string `json:"id"`
			} `json:"session"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if response.Data.User.ID == "" {
		t.Fatalf("expected user.id in me response")
	}
	if response.Data.Workspace.ID == "" {
		t.Fatalf("expected workspace.id in me response")
	}
	if response.Data.Session.ID == "" {
		t.Fatalf("expected session.id in me response")
	}
	if response.Data.Session.ExpiresAt == "" {
		t.Fatalf("expected session.expiresAt in me response")
	}
}

func TestGetPreferencesReturnsUserInitializationState(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"prefs@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	preferencesRecorder := httptest.NewRecorder()
	preferencesRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/me/preferences", nil)
	preferencesRequest.AddCookie(cookie)
	router.ServeHTTP(preferencesRecorder, preferencesRequest)
	if preferencesRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("preferences expected 200, got %d with body %s", preferencesRecorder.Code, preferencesRecorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(preferencesRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode preferences response: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data payload, got %T", payload["data"])
	}
	if data["defaultMode"] == nil || data["modelStrategy"] == nil || data["networkEnabledHint"] == nil {
		t.Fatalf("expected defaultMode, modelStrategy, and networkEnabledHint in preferences data")
	}
}

func TestUpdatePreferencesPersistsOnboardingState(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"updateprefs@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]
	csrfToken := csrfTokenFromRecorder(t, registerRecorder)

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/me/preferences", strings.NewReader(`{"onboardingCompleted":true,"defaultMode":"solo","modelStrategy":"high_quality","networkEnabledHint":true}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	addCSRF(updateRequest, csrfToken)
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update preferences expected 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("me expected 200, got %d", meRecorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	data := payload["data"].(map[string]any)
	if data["onboardingCompleted"] != true {
		t.Fatalf("expected onboardingCompleted true, got %v", data["onboardingCompleted"])
	}
	preferences := data["preferences"].(map[string]any)
	if preferences["defaultMode"] != "solo" {
		t.Fatalf("expected defaultMode solo, got %v", preferences["defaultMode"])
	}
}

func TestListModelsReturnsAvailableOptions(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"models@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	modelsRecorder := httptest.NewRecorder()
	modelsRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/models", nil)
	modelsRequest.AddCookie(cookie)
	router.ServeHTTP(modelsRecorder, modelsRequest)
	if modelsRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list models expected 200, got %d with body %s", modelsRecorder.Code, modelsRecorder.Body.String())
	}
}

func TestConversationConfigFlow(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"config@example.com","password":"StrongerPass1!"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]
	csrfToken := csrfTokenFromRecorder(t, registerRecorder)

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"Config chat"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
	addCSRF(createConversationRequest, csrfToken)
	router.ServeHTTP(createConversationRecorder, createConversationRequest)

	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createConversationRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created conversation: %v", err)
	}

	getConfigRecorder := httptest.NewRecorder()
	getConfigRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/"+created.Data.ID+"/config", nil)
	getConfigRequest.AddCookie(cookie)
	router.ServeHTTP(getConfigRecorder, getConfigRequest)
	if getConfigRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("get config expected 200, got %d with body %s", getConfigRecorder.Code, getConfigRecorder.Body.String())
	}

	createKnowledgeBaseRecorder := httptest.NewRecorder()
	createKnowledgeBaseRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/knowledge-bases", strings.NewReader(`{"name":"Reference Docs"}`))
	createKnowledgeBaseRequest.Header.Set("Content-Type", "application/json")
	createKnowledgeBaseRequest.AddCookie(cookie)
	addCSRF(createKnowledgeBaseRequest, csrfToken)
	router.ServeHTTP(createKnowledgeBaseRecorder, createKnowledgeBaseRequest)
	if createKnowledgeBaseRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("create knowledge base expected 200, got %d with body %s", createKnowledgeBaseRecorder.Code, createKnowledgeBaseRecorder.Body.String())
	}

	var knowledgeBase struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createKnowledgeBaseRecorder.Body.Bytes(), &knowledgeBase); err != nil {
		t.Fatalf("decode created knowledge base: %v", err)
	}

	updateConfigRecorder := httptest.NewRecorder()
	updateConfigRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/conversations/"+created.Data.ID+"/config", strings.NewReader(`{"modelId":"quality-chat","systemPromptOverride":"Be concise","temperature":0.7,"maxOutputTokens":512,"toolsEnabled":true,"knowledgeBaseIds":["`+knowledgeBase.Data.ID+`"]}`))
	updateConfigRequest.Header.Set("Content-Type", "application/json")
	updateConfigRequest.AddCookie(cookie)
	addCSRF(updateConfigRequest, csrfToken)
	router.ServeHTTP(updateConfigRecorder, updateConfigRequest)
	if updateConfigRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update config expected 200, got %d with body %s", updateConfigRecorder.Code, updateConfigRecorder.Body.String())
	}

	var updated struct {
		Data struct {
			ModelID              string   `json:"modelId"`
			SystemPromptOverride string   `json:"systemPromptOverride"`
			Temperature          float64  `json:"temperature"`
			MaxOutputTokens      int      `json:"maxOutputTokens"`
			ToolsEnabled         bool     `json:"toolsEnabled"`
			KnowledgeBaseIDs     []string `json:"knowledgeBaseIds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(updateConfigRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated config: %v", err)
	}
	if updated.Data.ModelID != "quality-chat" || updated.Data.SystemPromptOverride != "Be concise" || updated.Data.Temperature != 0.7 || updated.Data.MaxOutputTokens != 512 || !updated.Data.ToolsEnabled {
		t.Fatalf("unexpected updated config: %+v", updated.Data)
	}
	if len(updated.Data.KnowledgeBaseIDs) != 1 || updated.Data.KnowledgeBaseIDs[0] != knowledgeBase.Data.ID {
		t.Fatalf("expected knowledge bindings [%s], got %+v", knowledgeBase.Data.ID, updated.Data.KnowledgeBaseIDs)
	}
}

func TestMeRequiresSession(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}
