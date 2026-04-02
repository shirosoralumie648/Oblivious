package http

import (
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	"oblivious/server/internal/config"
)

func testConfig() config.Config {
	return config.Config{
		DatabaseURL:         "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable",
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

	database, err := sql.Open("postgres", testConfig().DatabaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	statements := []string{
		`DROP TABLE IF EXISTS conversation_configs`,
		`DROP TABLE IF EXISTS user_preferences`,
		`DROP TABLE IF EXISTS messages`,
		`DROP TABLE IF EXISTS conversations`,
		`DROP TABLE IF EXISTS sessions`,
		`DROP TABLE IF EXISTS workspaces`,
		`DROP TABLE IF EXISTS users`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), expires_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, title TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE, role TEXT NOT NULL, content TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE conversation_configs (conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE, model_id TEXT NOT NULL DEFAULT 'demo-reply', system_prompt_override TEXT NOT NULL DEFAULT '', temperature DOUBLE PRECISION NOT NULL DEFAULT 1, max_output_tokens INTEGER NOT NULL DEFAULT 1024, tools_enabled BOOLEAN NOT NULL DEFAULT FALSE, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE user_preferences (user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE, default_mode TEXT NOT NULL DEFAULT 'chat', model_strategy TEXT NOT NULL DEFAULT 'balanced', network_enabled_hint BOOLEAN NOT NULL DEFAULT FALSE, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare database: %v", err)
		}
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
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
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)

	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d", registerRecorder.Code)
	}
	cookie := registerRecorder.Result().Cookies()[0]
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

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	router.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("logout expected 200, got %d", logoutRecorder.Code)
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"secret"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("login expected 200, got %d", loginRecorder.Code)
	}
}

func TestConversationAndMessageFlow(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"chat@example.com","password":"secret"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"First chat"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
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

func TestRegisterStoresHashedPassword(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"hash@example.com","password":"secret"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)

	if registerRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("register expected 200, got %d", registerRecorder.Code)
	}

	var storedPassword string
	if err := database.QueryRow(`SELECT password_hash FROM users WHERE email = $1`, "hash@example.com").Scan(&storedPassword); err != nil {
		t.Fatalf("query password hash: %v", err)
	}
	if storedPassword == "secret" {
		t.Fatalf("expected stored password hash to differ from raw password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte("secret")); err != nil {
		t.Fatalf("expected stored hash to match password: %v", err)
	}
}

func TestLoginAcceptsRawPasswordAgainstStoredHash(t *testing.T) {
	database := testDatabase(t)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
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
	loginRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"hashed@example.com","password":"secret"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("login expected 200, got %d with body %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestMeReturnsExpandedSessionPayload(t *testing.T) {
	router := NewRouter(testConfig(), testDatabase(t))

	registerRecorder := httptest.NewRecorder()
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"state@example.com","password":"secret"}`))
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
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"prefs@example.com","password":"secret"}`))
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
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"updateprefs@example.com","password":"secret"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/me/preferences", strings.NewReader(`{"onboardingCompleted":true,"defaultMode":"solo","modelStrategy":"high_quality","networkEnabledHint":true}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
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
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"models@example.com","password":"secret"}`))
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
	registerRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"config@example.com","password":"secret"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(registerRecorder, registerRequest)
	cookie := registerRecorder.Result().Cookies()[0]

	createConversationRecorder := httptest.NewRecorder()
	createConversationRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations", strings.NewReader(`{"title":"Config chat"}`))
	createConversationRequest.Header.Set("Content-Type", "application/json")
	createConversationRequest.AddCookie(cookie)
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

	updateConfigRecorder := httptest.NewRecorder()
	updateConfigRequest := httptest.NewRequest(stdhttp.MethodPut, "/api/v1/app/conversations/"+created.Data.ID+"/config", strings.NewReader(`{"modelId":"quality-chat","systemPromptOverride":"Be concise","temperature":0.7,"maxOutputTokens":512,"toolsEnabled":true}`))
	updateConfigRequest.Header.Set("Content-Type", "application/json")
	updateConfigRequest.AddCookie(cookie)
	router.ServeHTTP(updateConfigRecorder, updateConfigRequest)
	if updateConfigRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("update config expected 200, got %d with body %s", updateConfigRecorder.Code, updateConfigRecorder.Body.String())
	}

	var updated struct {
		Data struct {
			ModelID              string  `json:"modelId"`
			SystemPromptOverride string  `json:"systemPromptOverride"`
			Temperature          float64 `json:"temperature"`
			MaxOutputTokens      int     `json:"maxOutputTokens"`
			ToolsEnabled         bool    `json:"toolsEnabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(updateConfigRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated config: %v", err)
	}
	if updated.Data.ModelID != "quality-chat" || updated.Data.SystemPromptOverride != "Be concise" || updated.Data.Temperature != 0.7 || updated.Data.MaxOutputTokens != 512 || !updated.Data.ToolsEnabled {
		t.Fatalf("unexpected updated config: %+v", updated.Data)
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
