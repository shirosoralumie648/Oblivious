package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SERVER_PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_TIMEOUT_MS", "")
	t.Setenv("MODEL_DEFAULT_NAME", "")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_SUCCESS_URL", "")
	t.Setenv("STRIPE_CANCEL_URL", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("SCHEDULE_WORKER_ENABLED", "")
	t.Setenv("SCHEDULE_WORKER_INTERVAL_MS", "")
	t.Setenv("SCHEDULE_WORKER_CLAIM_LIMIT", "")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX", "")
	t.Setenv("AGENT_WEB_SEARCH_PROVIDER", "")
	t.Setenv("AGENT_WEB_SEARCH_ENDPOINT", "")
	t.Setenv("AGENT_WEB_SEARCH_API_KEY", "")
	t.Setenv("AGENT_WEB_SEARCH_RESULT_LIMIT", "")
	t.Setenv("QDRANT_URL", "")
	t.Setenv("QDRANT_API_KEY", "")
	t.Setenv("QDRANT_VECTOR_SIZE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Fatalf("expected default env development, got %s", cfg.Env)
	}
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("expected empty origins, got %v", cfg.CORSAllowedOrigins)
	}
	if cfg.SessionCookieName != "oblivious_session" {
		t.Fatalf("expected default cookie name, got %s", cfg.SessionCookieName)
	}
	if cfg.SessionCookieSecure {
		t.Fatal("expected secure cookie default false")
	}
	if cfg.ModelDefaultName != "demo-reply" {
		t.Fatalf("expected default model name demo-reply, got %s", cfg.ModelDefaultName)
	}
	if cfg.LLMTimeoutMS != 30000 {
		t.Fatalf("expected default llm timeout 30000, got %d", cfg.LLMTimeoutMS)
	}
	if cfg.StripeSecretKey != "" || cfg.StripeWebhookSecret != "" {
		t.Fatal("expected stripe secrets to default empty")
	}
	if !cfg.ScheduleWorkerEnabled {
		t.Fatal("expected schedule worker to default enabled outside test env")
	}
	if cfg.ScheduleWorkerIntervalMS != 60000 || cfg.ScheduleWorkerClaimLimit != 50 {
		t.Fatalf("expected default schedule worker interval/limit, got interval=%d limit=%d", cfg.ScheduleWorkerIntervalMS, cfg.ScheduleWorkerClaimLimit)
	}
	if cfg.RelayRateLimitBackend != "memory" || cfg.RedisAddr != "" || cfg.RedisPassword != "" || cfg.RedisDB != 0 || cfg.RelayRateLimitRedisKeyPrefix != "" {
		t.Fatalf("expected relay rate limit redis config to default empty, got backend=%q addr=%q db=%d prefix=%q", cfg.RelayRateLimitBackend, cfg.RedisAddr, cfg.RedisDB, cfg.RelayRateLimitRedisKeyPrefix)
	}
	if cfg.RelaySemanticCacheBackend != "memory" {
		t.Fatalf("expected relay semantic cache backend to default memory, got %q", cfg.RelaySemanticCacheBackend)
	}
	if !cfg.RelayEnabled {
		t.Fatal("expected relay to default enabled")
	}
	if cfg.AgentWebSearchProvider != "" || cfg.AgentWebSearchEndpoint != "" || cfg.AgentWebSearchAPIKey != "" || cfg.AgentWebSearchResultLimit != 5 {
		t.Fatalf("expected agent web search to default disabled with limit 5, got provider=%q endpoint=%q key=%q limit=%d", cfg.AgentWebSearchProvider, cfg.AgentWebSearchEndpoint, cfg.AgentWebSearchAPIKey, cfg.AgentWebSearchResultLimit)
	}
	if cfg.QdrantURL != "" || cfg.QdrantAPIKey != "" || cfg.QdrantVectorSize != 1536 {
		t.Fatalf("expected qdrant to default disabled with vector size 1536, got url=%q key=%q size=%d", cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantVectorSize)
	}
}

func TestLoadAgentWebSearchConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_WEB_SEARCH_PROVIDER", " TAVILY ")
	t.Setenv("AGENT_WEB_SEARCH_ENDPOINT", " https://search.example.test/query ")
	t.Setenv("AGENT_WEB_SEARCH_API_KEY", " web-search-secret ")
	t.Setenv("AGENT_WEB_SEARCH_RESULT_LIMIT", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AgentWebSearchProvider != "tavily" {
		t.Fatalf("expected normalized web search provider tavily, got %q", cfg.AgentWebSearchProvider)
	}
	if cfg.AgentWebSearchEndpoint != "https://search.example.test/query" {
		t.Fatalf("expected trimmed web search endpoint, got %q", cfg.AgentWebSearchEndpoint)
	}
	if cfg.AgentWebSearchAPIKey != "web-search-secret" {
		t.Fatalf("expected trimmed web search api key, got %q", cfg.AgentWebSearchAPIKey)
	}
	if cfg.AgentWebSearchResultLimit != 3 {
		t.Fatalf("expected web search result limit 3, got %d", cfg.AgentWebSearchResultLimit)
	}
}

func TestLoadQdrantConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("QDRANT_URL", " http://qdrant.internal:6333 ")
	t.Setenv("QDRANT_API_KEY", " qdrant-secret ")
	t.Setenv("QDRANT_VECTOR_SIZE", "3072")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.QdrantURL != "http://qdrant.internal:6333" || cfg.QdrantAPIKey != "qdrant-secret" || cfg.QdrantVectorSize != 3072 {
		t.Fatalf("unexpected qdrant config url=%q key=%q size=%d", cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantVectorSize)
	}
}

func TestLoadRAGRerankerConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RAG_RERANKER_BASE_URL", " http://reranker.internal:8081 ")
	t.Setenv("RAG_RERANKER_API_KEY", " reranker-secret ")
	t.Setenv("RAG_RERANKER_MODEL", " bge-reranker-base ")
	t.Setenv("RAG_RERANKER_TOP_K", "12")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RAGRerankerBaseURL != "http://reranker.internal:8081" || cfg.RAGRerankerAPIKey != "reranker-secret" || cfg.RAGRerankerModel != "bge-reranker-base" || cfg.RAGRerankerTopK != 12 {
		t.Fatalf("unexpected reranker config url=%q key=%q model=%q topK=%d", cfg.RAGRerankerBaseURL, cfg.RAGRerankerAPIKey, cfg.RAGRerankerModel, cfg.RAGRerankerTopK)
	}
}

func TestLoadRejectsInvalidRAGRerankerTopK(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RAG_RERANKER_TOP_K", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid RAG_RERANKER_TOP_K")
	}
}

func TestLoadRejectsInvalidQdrantVectorSize(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("QDRANT_VECTOR_SIZE", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid QDRANT_VECTOR_SIZE")
	}
}

func TestLoadRejectsInvalidAgentWebSearchResultLimit(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_WEB_SEARCH_RESULT_LIMIT", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid agent web search result limit")
	}
}

func TestLoadAllowsRelaySemanticCacheSQLBackend(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_SEMANTIC_CACHE_BACKEND", "sql")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RelaySemanticCacheBackend != "sql" {
		t.Fatalf("expected sql semantic cache backend, got %q", cfg.RelaySemanticCacheBackend)
	}
}

func TestLoadRejectsInvalidRelaySemanticCacheBackend(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_SEMANTIC_CACHE_BACKEND", "postgres")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid relay semantic cache backend")
	}
}

func TestLoadAllowsRelayRateLimitToBeExplicitlyDisabled(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "none")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RelayRateLimitBackend != "none" {
		t.Fatalf("expected explicit none rate limit backend, got %q", cfg.RelayRateLimitBackend)
	}
}

func TestLoadAllowsRelayToBeExplicitlyDisabled(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_ENABLED", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RelayEnabled {
		t.Fatal("expected RELAY_ENABLED=false to disable relay")
	}
}

func TestLoadScheduleWorkerConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("SCHEDULE_WORKER_ENABLED", "true")
	t.Setenv("SCHEDULE_WORKER_INTERVAL_MS", "250")
	t.Setenv("SCHEDULE_WORKER_CLAIM_LIMIT", "9")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ScheduleWorkerEnabled {
		t.Fatal("expected schedule worker env override to enable worker in test env")
	}
	if cfg.ScheduleWorkerIntervalMS != 250 || cfg.ScheduleWorkerClaimLimit != 9 {
		t.Fatalf("expected schedule worker overrides, got interval=%d limit=%d", cfg.ScheduleWorkerIntervalMS, cfg.ScheduleWorkerClaimLimit)
	}
}

func TestLoadRelayRedisRateLimitConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "redis")
	t.Setenv("REDIS_ADDR", "redis.internal:6380")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX", "tenant:relay:limit")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RelayRateLimitBackend != "redis" {
		t.Fatalf("expected redis rate limit backend, got %q", cfg.RelayRateLimitBackend)
	}
	if cfg.RedisAddr != "redis.internal:6380" || cfg.RedisPassword != "redis-secret" || cfg.RedisDB != 3 {
		t.Fatalf("unexpected redis config addr=%q password=%q db=%d", cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	}
	if cfg.RelayRateLimitRedisKeyPrefix != "tenant:relay:limit" {
		t.Fatalf("unexpected redis key prefix %q", cfg.RelayRateLimitRedisKeyPrefix)
	}
}

func TestLoadRelayRedisRateLimitConfigFromRedisURL(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "redis")
	t.Setenv("REDIS_URL", "redis://:redis-secret@redis.internal:6380/3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.RedisAddr != "redis.internal:6380" || cfg.RedisPassword != "redis-secret" || cfg.RedisDB != 3 {
		t.Fatalf("unexpected redis config from url addr=%q password=%q db=%d", cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	}
}

func TestLoadObservabilityClickHouseRequestLogConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", " CLICKHOUSE ")
	t.Setenv("CLICKHOUSE_DSN", " tcp://clickhouse:9000?database=oblivious ")
	t.Setenv("CLICKHOUSE_DRIVER", " clickhouse ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ObservabilityRequestLogBackend != "clickhouse" {
		t.Fatalf("expected clickhouse request log backend, got %q", cfg.ObservabilityRequestLogBackend)
	}
	if cfg.ClickHouseDSN != "tcp://clickhouse:9000?database=oblivious" {
		t.Fatalf("expected trimmed ClickHouse DSN, got %q", cfg.ClickHouseDSN)
	}
	if cfg.ClickHouseDriver != "clickhouse" {
		t.Fatalf("expected trimmed ClickHouse driver, got %q", cfg.ClickHouseDriver)
	}
}

func TestLoadObservabilityHTTPAlertConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_ALERTS_ENABLED", "true")
	t.Setenv("ALERT_WEBHOOK_URL", " https://ops.example.test/alerts ")
	t.Setenv("ALERT_WEBHOOK_SECRET", " alert-secret ")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED", "true")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS", "1500")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ObservabilityHTTPAlertsEnabled {
		t.Fatal("expected HTTP alerts to be enabled")
	}
	if cfg.AlertWebhookURL != "https://ops.example.test/alerts" || cfg.AlertWebhookSecret != "alert-secret" {
		t.Fatalf("expected trimmed alert webhook config, got url=%q secret=%q", cfg.AlertWebhookURL, cfg.AlertWebhookSecret)
	}
	if !cfg.ObservabilityHTTPRecoveryEnabled {
		t.Fatal("expected HTTP recovery to be enabled")
	}
	if cfg.ObservabilityHTTPRecoveryCooldownMS != 1500 {
		t.Fatalf("expected recovery cooldown 1500ms, got %d", cfg.ObservabilityHTTPRecoveryCooldownMS)
	}
}

func TestLoadRejectsInvalidObservabilityHTTPRecoveryCooldown(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid HTTP recovery cooldown")
	}
}

func TestLoadRejectsClickHouseRequestLogWithoutDSN(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
	t.Setenv("CLICKHOUSE_DSN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ClickHouse DSN")
	}
}

func TestLoadRejectsInvalidRelayRateLimitBackend(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "postgres")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid relay rate limit backend")
	}
}

func TestLoadRejectsInvalidRedisDB(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "redis")
	t.Setenv("REDIS_DB", "not-an-int")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid redis db")
	}
}

func TestLoadDisablesScheduleWorkerByDefaultInTestEnv(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("SCHEDULE_WORKER_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ScheduleWorkerEnabled {
		t.Fatal("expected schedule worker to default disabled in test env")
	}
}

func TestLoadStripeConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_phase17")
	t.Setenv("STRIPE_SUCCESS_URL", "https://app.example.test/success")
	t.Setenv("STRIPE_CANCEL_URL", "https://app.example.test/cancel")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_phase17")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.StripeSecretKey != "sk_test_phase17" {
		t.Fatalf("expected stripe secret key, got %s", cfg.StripeSecretKey)
	}
	if cfg.StripeSuccessURL != "https://app.example.test/success" || cfg.StripeCancelURL != "https://app.example.test/cancel" {
		t.Fatalf("expected stripe URLs, got success=%s cancel=%s", cfg.StripeSuccessURL, cfg.StripeCancelURL)
	}
	if cfg.StripeWebhookSecret != "whsec_phase17" {
		t.Fatalf("expected stripe webhook secret, got %s", cfg.StripeWebhookSecret)
	}
}

func TestLoadRejectsInvalidLLMTimeout(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("LLM_TIMEOUT_MS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid llm timeout")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("SERVER_PORT", "abc")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "test-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing database url")
	}
}

func TestLoadRejectsMissingSessionSecret(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing session secret")
	}
}
