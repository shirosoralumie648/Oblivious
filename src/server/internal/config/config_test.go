package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

const productionSessionSecret = "0123456789abcdef0123456789abcdef"
const productionSecretEncryptionKey = "abcdef0123456789abcdef0123456789"

func TestLoadReadinessDependencyConfigContract(t *testing.T) {
	load := func(t *testing.T) (Config, error) {
		t.Helper()
		t.Setenv("SERVER_PORT", "8080")
		t.Setenv("APP_ENV", "test")
		t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
		t.Setenv("SESSION_SECRET", "test-secret")
		return Load()
	}

	t.Run("redis url is retained independently of rate limit backend", func(t *testing.T) {
		t.Setenv("RELAY_RATE_LIMIT_BACKEND", "memory")
		t.Setenv("REDIS_URL", "redis://:url-secret@redis.internal:6380/4")
		cfg, err := load(t)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RedisAddr != "redis.internal:6380" || cfg.RedisPassword != "url-secret" || cfg.RedisDB != 4 {
			t.Fatalf("redis readiness endpoint = %q/%q/%d", cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		}
	})

	t.Run("explicit redis fields override url independently", func(t *testing.T) {
		t.Setenv("RELAY_RATE_LIMIT_BACKEND", "none")
		t.Setenv("REDIS_URL", "redis://:url-secret@redis.internal:6380/4")
		t.Setenv("REDIS_ADDR", "override.internal:6379")
		t.Setenv("REDIS_PASSWORD", "override-secret")
		t.Setenv("REDIS_DB", "7")
		cfg, err := load(t)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RedisAddr != "override.internal:6379" || cfg.RedisPassword != "override-secret" || cfg.RedisDB != 7 {
			t.Fatalf("explicit redis override = %q/%q/%d", cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		}
	})

	t.Run("kafka absent is typed empty", func(t *testing.T) {
		unsetEnvForTest(t, "KAFKA_BROKERS")
		cfg, err := load(t)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.KafkaBrokers == nil || len(cfg.KafkaBrokers) != 0 {
			t.Fatalf("KafkaBrokers = %#v, want non-nil empty", cfg.KafkaBrokers)
		}
	})

	t.Run("kafka trims and returns defensive storage", func(t *testing.T) {
		t.Setenv("KAFKA_BROKERS", " kafka-1.internal:9092 ,kafka-2.internal:9093 ")
		first, err := load(t)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"kafka-1.internal:9092", "kafka-2.internal:9093"}
		if !reflect.DeepEqual(first.KafkaBrokers, want) {
			t.Fatalf("KafkaBrokers = %#v, want %#v", first.KafkaBrokers, want)
		}
		first.KafkaBrokers[0] = "mutated.invalid:1"
		second, err := load(t)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(second.KafkaBrokers, want) {
			t.Fatalf("KafkaBrokers retained caller mutation: %#v", second.KafkaBrokers)
		}
	})

	for _, raw := range []string{"", " ", ",kafka:9092", "kafka:9092,", "kafka:9092,,kafka-2:9092", "kafka-no-port", "http://kafka:9092"} {
		t.Run("rejects invalid kafka list "+raw, func(t *testing.T) {
			t.Setenv("KAFKA_BROKERS", raw)
			_, err := load(t)
			if err == nil || !strings.Contains(err.Error(), "invalid KAFKA_BROKERS") {
				t.Fatalf("error = %v, want stable KAFKA_BROKERS error", err)
			}
		})
	}

	t.Run("invalid redis url is redacted", func(t *testing.T) {
		t.Setenv("REDIS_URL", "redis://:redis-secret@%zz.invalid:6379/0")
		_, err := load(t)
		if err == nil || !strings.Contains(err.Error(), "invalid REDIS_URL") {
			t.Fatalf("error = %v, want stable REDIS_URL error", err)
		}
		for _, forbidden := range []string{"redis-secret", "%zz.invalid", "6379"} {
			if strings.Contains(err.Error(), forbidden) {
				t.Fatalf("REDIS_URL error leaked %q: %v", forbidden, err)
			}
		}
	})
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	value, present := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func setProductionRuntimeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", productionSessionSecret)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELAY_ENABLED", "true")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
	t.Setenv("CLICKHOUSE_DSN", "tcp://clickhouse:9000?database=oblivious")
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("RAG_INDEX_WORKER_ENABLED", "")
	t.Setenv("RAG_INGESTION_WORKER_ENABLED", "")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_SUCCESS_URL", "")
	t.Setenv("STRIPE_CANCEL_URL", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("ALIPAY_CHECKOUT_BASE_URL", "")
	t.Setenv("ALIPAY_WEBHOOK_SECRET", "")
	t.Setenv("WECHATPAY_CHECKOUT_BASE_URL", "")
	t.Setenv("WECHATPAY_WEBHOOK_SECRET", "")
	t.Setenv("MARKETPLACE_PAYOUT_PROVIDER", "")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_URL", "")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_ENABLED", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE_URL", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_PROVIDER", "")
}

func setProductionStripePaymentEnv(t *testing.T) {
	t.Helper()
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_configured")
	t.Setenv("STRIPE_SUCCESS_URL", "https://app.oblivious.ai/billing/success")
	t.Setenv("STRIPE_CANCEL_URL", "https://app.oblivious.ai/billing/cancel")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_live_configured")
}

func setProductionPayoutWebhookEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MARKETPLACE_PAYOUT_PROVIDER", "webhook")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_URL", "https://payments.oblivious.ai/marketplace/payouts")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET", "payout_webhook_secret")
}

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
	t.Setenv("CHAT_RELAY_BASE_URL", "")
	t.Setenv("AGENT_RELAY_BASE_URL", "")
	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_SUCCESS_URL", "")
	t.Setenv("STRIPE_CANCEL_URL", "")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "")
	t.Setenv("MARKETPLACE_PAYOUT_PROVIDER", "")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_URL", "")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET", "")
	t.Setenv("SCHEDULE_WORKER_ENABLED", "")
	t.Setenv("SCHEDULE_WORKER_INTERVAL_MS", "")
	t.Setenv("SCHEDULE_WORKER_CLAIM_LIMIT", "")
	t.Setenv("RELAY_RATE_LIMIT_BACKEND", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_ENABLED", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_INTERVAL_MS", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_PROVIDER", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE_URL", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_REQUIRED_MODELS", "")
	t.Setenv("RELAY_PRICING_MAINTENANCE_MAX_BYTES", "")
	t.Setenv("RELAY_PRICING_RECONCILIATION_LIMIT", "")
	t.Setenv("WORKFLOW_RELAY_BASE_URL", "")
	t.Setenv("AGENT_WEB_SEARCH_PROVIDER", "")
	t.Setenv("AGENT_WEB_SEARCH_ENDPOINT", "")
	t.Setenv("AGENT_WEB_SEARCH_API_KEY", "")
	t.Setenv("AGENT_WEB_SEARCH_RESULT_LIMIT", "")
	t.Setenv("QDRANT_URL", "")
	t.Setenv("QDRANT_API_KEY", "")
	t.Setenv("QDRANT_VECTOR_SIZE", "")
	t.Setenv("RAG_INDEX_WORKER_ENABLED", "")
	t.Setenv("RAG_INDEX_WORKER_INTERVAL_MS", "")
	t.Setenv("RAG_INDEX_WORKER_CLAIM_LIMIT", "")
	t.Setenv("RAG_INGESTION_WORKER_ENABLED", "")
	t.Setenv("RAG_INGESTION_WORKER_INTERVAL_MS", "")
	t.Setenv("RAG_INGESTION_WORKER_CLAIM_LIMIT", "")

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
	if cfg.ChatRelayBaseURL != "" || cfg.AgentRelayBaseURL != "" {
		t.Fatalf("expected relay base URL overrides to default empty, got chat=%q agent=%q", cfg.ChatRelayBaseURL, cfg.AgentRelayBaseURL)
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
	if cfg.RelayPricingMaintenanceEnabled || cfg.RelayPricingMaintenanceIntervalMS != 3600000 || cfg.RelayPricingMaintenanceSource != "litellm" || cfg.RelayPricingReconciliationLimit != 100 {
		t.Fatalf("expected relay pricing maintenance to default disabled with litellm source, got enabled=%v interval=%d source=%q limit=%d", cfg.RelayPricingMaintenanceEnabled, cfg.RelayPricingMaintenanceIntervalMS, cfg.RelayPricingMaintenanceSource, cfg.RelayPricingReconciliationLimit)
	}
	if cfg.RelayBatchPollingWorkerEnabled || cfg.RelayBatchPollingWorkerIntervalMS != 60000 || cfg.RelayBatchPollingWorkerClaimLimit != 10 {
		t.Fatalf("expected relay batch polling worker to default disabled, got enabled=%v interval=%d limit=%d", cfg.RelayBatchPollingWorkerEnabled, cfg.RelayBatchPollingWorkerIntervalMS, cfg.RelayBatchPollingWorkerClaimLimit)
	}
	if cfg.RelayBatchCommercialLifecycleEnabled {
		t.Fatal("expected relay batch commercial lifecycle to default disabled")
	}
	if cfg.RelayRealtimeCommercialLifecycleEnabled {
		t.Fatal("expected relay realtime commercial lifecycle to default disabled")
	}
	if cfg.AgentWebSearchProvider != "" || cfg.AgentWebSearchEndpoint != "" || cfg.AgentWebSearchAPIKey != "" || cfg.AgentWebSearchResultLimit != 5 {
		t.Fatalf("expected agent web search to default disabled with limit 5, got provider=%q endpoint=%q key=%q limit=%d", cfg.AgentWebSearchProvider, cfg.AgentWebSearchEndpoint, cfg.AgentWebSearchAPIKey, cfg.AgentWebSearchResultLimit)
	}
	if cfg.QdrantURL != "" || cfg.QdrantAPIKey != "" || cfg.QdrantVectorSize != 1536 {
		t.Fatalf("expected qdrant to default disabled with vector size 1536, got url=%q key=%q size=%d", cfg.QdrantURL, cfg.QdrantAPIKey, cfg.QdrantVectorSize)
	}
	if cfg.RAGIndexWorkerEnabled || cfg.RAGIndexWorkerIntervalMS != 60000 || cfg.RAGIndexWorkerClaimLimit != 10 {
		t.Fatalf("expected RAG index worker to default disabled without qdrant, got enabled=%v interval=%d limit=%d", cfg.RAGIndexWorkerEnabled, cfg.RAGIndexWorkerIntervalMS, cfg.RAGIndexWorkerClaimLimit)
	}
	if !cfg.RAGIngestionWorkerEnabled || cfg.RAGIngestionWorkerIntervalMS != 60000 || cfg.RAGIngestionWorkerClaimLimit != 10 {
		t.Fatalf("expected RAG ingestion worker to default enabled outside test env, got enabled=%v interval=%d limit=%d", cfg.RAGIngestionWorkerEnabled, cfg.RAGIngestionWorkerIntervalMS, cfg.RAGIngestionWorkerClaimLimit)
	}
}

func TestLoadAgentWebSearchConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_RELAY_BASE_URL", " http://gateway.internal:8080/v1 ")
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
	if cfg.AgentRelayBaseURL != "http://gateway.internal:8080/v1" {
		t.Fatalf("expected trimmed agent relay base URL, got %q", cfg.AgentRelayBaseURL)
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

func TestLoadChatRelayBaseURLConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("CHAT_RELAY_BASE_URL", " http://relay.internal:8080/v1 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.ChatRelayBaseURL != "http://relay.internal:8080/v1" {
		t.Fatalf("expected trimmed chat relay base URL, got %q", cfg.ChatRelayBaseURL)
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
	if !cfg.RAGIndexWorkerEnabled {
		t.Fatal("expected RAG index worker to default enabled when qdrant is configured outside test env")
	}
}

func TestLoadRAGIndexWorkerConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RAG_INDEX_WORKER_ENABLED", "true")
	t.Setenv("RAG_INDEX_WORKER_INTERVAL_MS", "250")
	t.Setenv("RAG_INDEX_WORKER_CLAIM_LIMIT", "4")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RAGIndexWorkerEnabled || cfg.RAGIndexWorkerIntervalMS != 250 || cfg.RAGIndexWorkerClaimLimit != 4 {
		t.Fatalf("unexpected RAG index worker config enabled=%v interval=%d limit=%d", cfg.RAGIndexWorkerEnabled, cfg.RAGIndexWorkerIntervalMS, cfg.RAGIndexWorkerClaimLimit)
	}
}

func TestLoadRAGIngestionWorkerConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RAG_INGESTION_WORKER_ENABLED", "true")
	t.Setenv("RAG_INGESTION_WORKER_INTERVAL_MS", "500")
	t.Setenv("RAG_INGESTION_WORKER_CLAIM_LIMIT", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RAGIngestionWorkerEnabled || cfg.RAGIngestionWorkerIntervalMS != 500 || cfg.RAGIngestionWorkerClaimLimit != 3 {
		t.Fatalf("unexpected RAG ingestion worker config enabled=%v interval=%d limit=%d", cfg.RAGIngestionWorkerEnabled, cfg.RAGIngestionWorkerIntervalMS, cfg.RAGIngestionWorkerClaimLimit)
	}
}

func TestLoadRelayPricingMaintenanceConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_PRICING_MAINTENANCE_ENABLED", "true")
	t.Setenv("RELAY_PRICING_MAINTENANCE_INTERVAL_MS", "5000")
	t.Setenv("RELAY_PRICING_MAINTENANCE_PROVIDER", " OpenAI ")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE", "litellm")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE_URL", " https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json ")
	t.Setenv("RELAY_PRICING_MAINTENANCE_REQUIRED_MODELS", "gpt-4o, text-embedding-3-small")
	t.Setenv("RELAY_PRICING_MAINTENANCE_MAX_BYTES", "1048576")
	t.Setenv("RELAY_PRICING_RECONCILIATION_LIMIT", "25")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RelayPricingMaintenanceEnabled ||
		cfg.RelayPricingMaintenanceIntervalMS != 5000 ||
		cfg.RelayPricingMaintenanceProvider != "openai" ||
		cfg.RelayPricingMaintenanceSourceURL != "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json" ||
		cfg.RelayPricingMaintenanceMaxBytes != 1048576 ||
		cfg.RelayPricingReconciliationLimit != 25 {
		t.Fatalf("unexpected relay pricing maintenance config: %+v", cfg)
	}
	if len(cfg.RelayPricingMaintenanceModels) != 2 || cfg.RelayPricingMaintenanceModels[0] != "gpt-4o" || cfg.RelayPricingMaintenanceModels[1] != "text-embedding-3-small" {
		t.Fatalf("unexpected required models: %+v", cfg.RelayPricingMaintenanceModels)
	}
}

func TestLoadRelayPricingMaintenanceRequiresHTTPSURLWhenEnabled(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_PRICING_MAINTENANCE_ENABLED", "true")
	t.Setenv("RELAY_PRICING_MAINTENANCE_PROVIDER", "openai")
	t.Setenv("RELAY_PRICING_MAINTENANCE_SOURCE_URL", "http://prices.example.test/model_prices.json")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "RELAY_PRICING_MAINTENANCE_SOURCE_URL must be an https URL") {
		t.Fatalf("expected HTTPS source URL error, got %v", err)
	}
}

func TestLoadWorkflowSystemLimitConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("WORKFLOW_SYSTEM_MAX_CONCURRENT", "75")
	t.Setenv("WORKFLOW_GLOBAL_MAX_EXECUTIONS_PER_MINUTE", "120")
	t.Setenv("WORKFLOW_RELAY_BASE_URL", " http://relay.internal:8080/v1 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.WorkflowSystemMaxConcurrent != 75 || cfg.WorkflowGlobalMaxExecutionsPerMinute != 120 {
		t.Fatalf("unexpected workflow system limits: concurrent=%d perMinute=%d", cfg.WorkflowSystemMaxConcurrent, cfg.WorkflowGlobalMaxExecutionsPerMinute)
	}
	if cfg.WorkflowRelayBaseURL != "http://relay.internal:8080/v1" {
		t.Fatalf("unexpected workflow relay base URL: %q", cfg.WorkflowRelayBaseURL)
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

func TestLoadRelayBatchPollingWorkerConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_BATCH_POLLING_WORKER_ENABLED", "true")
	t.Setenv("RELAY_BATCH_POLLING_WORKER_INTERVAL_MS", "750")
	t.Setenv("RELAY_BATCH_POLLING_WORKER_CLAIM_LIMIT", "6")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RelayBatchPollingWorkerEnabled || cfg.RelayBatchPollingWorkerIntervalMS != 750 || cfg.RelayBatchPollingWorkerClaimLimit != 6 {
		t.Fatalf("unexpected relay batch polling worker config enabled=%v interval=%d limit=%d", cfg.RelayBatchPollingWorkerEnabled, cfg.RelayBatchPollingWorkerIntervalMS, cfg.RelayBatchPollingWorkerClaimLimit)
	}
}

func TestLoadRelayBatchCommercialLifecycleConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RelayBatchCommercialLifecycleEnabled {
		t.Fatal("expected relay batch commercial lifecycle to be enabled")
	}
}

func TestLoadRelayRealtimeCommercialLifecycleConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("RELAY_REALTIME_COMMERCIAL_LIFECYCLE_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.RelayRealtimeCommercialLifecycleEnabled {
		t.Fatal("expected relay realtime commercial lifecycle to be enabled")
	}
}

func TestLoadRejectsProductionBatchCommercialLifecycleWithoutPollingWorker(t *testing.T) {
	setProductionRuntimeEnv(t)
	setProductionStripePaymentEnv(t)
	setProductionPayoutWebhookEnv(t)
	t.Setenv("RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED", "true")
	t.Setenv("RELAY_BATCH_POLLING_WORKER_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production batch commercial lifecycle without polling worker")
	}
	if !strings.Contains(err.Error(), "RELAY_BATCH_POLLING_WORKER_ENABLED=true is required when RELAY_BATCH_COMMERCIAL_LIFECYCLE_ENABLED=true in production") {
		t.Fatalf("expected batch polling worker prerequisite error, got %v", err)
	}
}

func TestLoadRejectsInvalidRAGIndexWorkerConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		val  string
	}{
		{name: "interval", key: "RAG_INDEX_WORKER_INTERVAL_MS", val: "0"},
		{name: "claim limit", key: "RAG_INDEX_WORKER_CLAIM_LIMIT", val: "0"},
		{name: "ingestion interval", key: "RAG_INGESTION_WORKER_INTERVAL_MS", val: "0"},
		{name: "ingestion claim limit", key: "RAG_INGESTION_WORKER_CLAIM_LIMIT", val: "0"},
		{name: "batch polling interval", key: "RELAY_BATCH_POLLING_WORKER_INTERVAL_MS", val: "0"},
		{name: "batch polling claim limit", key: "RELAY_BATCH_POLLING_WORKER_CLAIM_LIMIT", val: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "8080")
			t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
			t.Setenv("SESSION_SECRET", "test-secret")
			t.Setenv(tc.key, tc.val)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for invalid %s", tc.key)
			}
		})
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
	t.Setenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS", "2500")

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
	if cfg.ObservabilityHTTPLatencySLOThresholdMS != 2500 {
		t.Fatalf("expected latency SLO threshold 2500ms, got %d", cfg.ObservabilityHTTPLatencySLOThresholdMS)
	}
}

func TestLoadObservabilityHTTPRecoveryAuditConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_ENABLED", "true")
	t.Setenv("OBSERVABILITY_HTTP_RECOVERY_AUDIT_COOLDOWN_MS", "1750")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ObservabilityHTTPRecoveryEnabled {
		t.Fatal("expected HTTP recovery audit to be enabled")
	}
	if cfg.ObservabilityHTTPRecoveryCooldownMS != 1750 {
		t.Fatalf("expected recovery audit cooldown 1750ms, got %d", cfg.ObservabilityHTTPRecoveryCooldownMS)
	}
}

func TestLoadChannelMessageLogArchiveConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_ROOT", " /var/lib/oblivious/channel-archives ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_INTERVAL_MS", "300000")
	t.Setenv("CHANNEL_MESSAGE_LOG_RETENTION_HOURS", "168")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_LIMIT", "75")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ChannelMessageLogArchiveEnabled {
		t.Fatal("expected channel message log archive to be enabled when root is configured")
	}
	if cfg.ChannelMessageLogArchiveRoot != "/var/lib/oblivious/channel-archives" {
		t.Fatalf("expected trimmed channel archive root, got %q", cfg.ChannelMessageLogArchiveRoot)
	}
	if cfg.ChannelMessageLogArchiveIntervalMS != 300000 || cfg.ChannelMessageLogRetentionHours != 168 || cfg.ChannelMessageLogArchiveLimit != 75 {
		t.Fatalf("unexpected channel archive config interval=%d retention=%d limit=%d", cfg.ChannelMessageLogArchiveIntervalMS, cfg.ChannelMessageLogRetentionHours, cfg.ChannelMessageLogArchiveLimit)
	}
}

func TestLoadChannelMessageLogS3ArchiveConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND", " MinIO ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ENDPOINT", " http://minio.internal:9000 ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_REGION", " us-east-1 ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_BUCKET", " channel-archives ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ACCESS_KEY", " minio-access ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_SECRET_KEY", " minio-secret ")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_INTERVAL_MS", "300000")
	t.Setenv("CHANNEL_MESSAGE_LOG_RETENTION_HOURS", "168")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_LIMIT", "75")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !cfg.ChannelMessageLogArchiveEnabled {
		t.Fatal("expected channel message log archive to be enabled for s3 backend")
	}
	if cfg.ChannelMessageLogArchiveBackend != "s3" {
		t.Fatalf("expected minio alias to normalize to s3 backend, got %q", cfg.ChannelMessageLogArchiveBackend)
	}
	if cfg.ChannelMessageLogArchiveS3Endpoint != "http://minio.internal:9000" ||
		cfg.ChannelMessageLogArchiveS3Region != "us-east-1" ||
		cfg.ChannelMessageLogArchiveS3Bucket != "channel-archives" ||
		cfg.ChannelMessageLogArchiveS3AccessKey != "minio-access" ||
		cfg.ChannelMessageLogArchiveS3SecretKey != "minio-secret" {
		t.Fatalf("unexpected s3 archive config endpoint=%q region=%q bucket=%q access=%q secret=%q",
			cfg.ChannelMessageLogArchiveS3Endpoint,
			cfg.ChannelMessageLogArchiveS3Region,
			cfg.ChannelMessageLogArchiveS3Bucket,
			cfg.ChannelMessageLogArchiveS3AccessKey,
			cfg.ChannelMessageLogArchiveS3SecretKey)
	}
	if cfg.ChannelMessageLogArchiveIntervalMS != 300000 || cfg.ChannelMessageLogRetentionHours != 168 || cfg.ChannelMessageLogArchiveLimit != 75 {
		t.Fatalf("unexpected channel archive config interval=%d retention=%d limit=%d", cfg.ChannelMessageLogArchiveIntervalMS, cfg.ChannelMessageLogRetentionHours, cfg.ChannelMessageLogArchiveLimit)
	}
}

func TestLoadRejectsInvalidChannelMessageLogArchiveConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		val  string
	}{
		{name: "interval", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_INTERVAL_MS", val: "0"},
		{name: "retention", key: "CHANNEL_MESSAGE_LOG_RETENTION_HOURS", val: "0"},
		{name: "limit", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_LIMIT", val: "-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "8080")
			t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
			t.Setenv("SESSION_SECRET", "test-secret")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_ROOT", "/tmp/channel-archives")
			t.Setenv(tc.key, tc.val)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for invalid %s", tc.key)
			}
		})
	}
}

func TestLoadRejectsInvalidChannelMessageLogArchiveBackend(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND", "gcs")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid channel message log archive backend")
	}
}

func TestLoadRejectsIncompleteChannelMessageLogS3ArchiveConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
	}{
		{name: "endpoint", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ENDPOINT"},
		{name: "bucket", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_S3_BUCKET"},
		{name: "access key", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ACCESS_KEY"},
		{name: "secret key", key: "CHANNEL_MESSAGE_LOG_ARCHIVE_S3_SECRET_KEY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "8080")
			t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
			t.Setenv("SESSION_SECRET", "test-secret")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_BACKEND", "s3")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ENDPOINT", "http://minio.internal:9000")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_BUCKET", "channel-archives")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_ACCESS_KEY", "minio-access")
			t.Setenv("CHANNEL_MESSAGE_LOG_ARCHIVE_S3_SECRET_KEY", "minio-secret")
			t.Setenv(tc.key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for missing %s", tc.key)
			}
		})
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

func TestLoadRejectsInvalidObservabilityHTTPLatencySLOThreshold(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid HTTP latency SLO threshold")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_HTTP_LATENCY_SLO_THRESHOLD_MS") {
		t.Fatalf("expected latency SLO threshold error, got %v", err)
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

func TestLoadRejectsProductionRelayWithoutRequestLogSink(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", productionSessionSecret)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELAY_ENABLED", "true")
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "none")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production relay without request log sink")
	}
	if !strings.Contains(err.Error(), "OBSERVABILITY_REQUEST_LOG_BACKEND must not be none") {
		t.Fatalf("expected request log sink error, got %v", err)
	}
}

func TestLoadRejectsProductionWithoutRelay(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", productionSessionSecret)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "production")
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("RELAY_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production with relay disabled")
	}
	if !strings.Contains(err.Error(), "RELAY_ENABLED=false is not allowed") {
		t.Fatalf("expected production relay-disabled error, got %v", err)
	}
}

func TestLoadRejectsProductionWithoutQdrantURL(t *testing.T) {
	setProductionRuntimeEnv(t)
	t.Setenv("QDRANT_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production without QDRANT_URL")
	}
	if !strings.Contains(err.Error(), "QDRANT_URL is required when APP_ENV=production for Knowledge/RAG routes") {
		t.Fatalf("expected production Qdrant prerequisite error, got %v", err)
	}
}

func TestLoadRejectsProductionQdrantWithoutRAGIndexWorker(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", productionSessionSecret)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELAY_ENABLED", "true")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
	t.Setenv("CLICKHOUSE_DSN", "tcp://clickhouse:9000?database=oblivious")
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("RAG_INDEX_WORKER_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production qdrant without RAG index worker")
	}
	if !strings.Contains(err.Error(), "RAG_INDEX_WORKER_ENABLED=false is not allowed") {
		t.Fatalf("expected production RAG index worker error, got %v", err)
	}
}

func TestLoadRejectsProductionWithoutRAGIngestionWorker(t *testing.T) {
	setProductionRuntimeEnv(t)
	t.Setenv("RAG_INGESTION_WORKER_ENABLED", "false")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production without RAG ingestion worker")
	}
	if !strings.Contains(err.Error(), "RAG_INGESTION_WORKER_ENABLED=false is not allowed when APP_ENV=production") {
		t.Fatalf("expected production RAG ingestion worker error, got %v", err)
	}
}

func TestLoadRejectsProductionWeakSessionSecret(t *testing.T) {
	for _, tc := range []struct {
		name        string
		secret      string
		wantMessage string
	}{
		{name: "default value", secret: "change-me", wantMessage: "SESSION_SECRET must not use a default value"},
		{name: "short value", secret: "short-production-secret", wantMessage: "SESSION_SECRET must be at least 32 characters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "8080")
			t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
			t.Setenv("SESSION_SECRET", tc.secret)
			t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
			t.Setenv("SESSION_COOKIE_SECURE", "true")
			t.Setenv("APP_ENV", "production")
			t.Setenv("RELAY_ENABLED", "true")
			t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
			t.Setenv("CLICKHOUSE_DSN", "tcp://clickhouse:9000?database=oblivious")

			_, err := Load()
			if err == nil {
				t.Fatal("expected error for weak production session secret")
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q error, got %v", tc.wantMessage, err)
			}
		})
	}
}

func TestLoadRejectsProductionWeakSecretEncryptionKey(t *testing.T) {
	for _, tc := range []struct {
		name        string
		key         string
		wantMessage string
	}{
		{name: "missing", key: "", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY is required"},
		{name: "default value", key: "change-me", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must not use a default value"},
		{name: "short value", key: "short-secretbox-key", wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must be at least 32 characters"},
		{name: "same as session", key: productionSessionSecret, wantMessage: "OBLIVIOUS_SECRET_ENCRYPTION_KEY must be distinct from SESSION_SECRET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SERVER_PORT", "8080")
			t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
			t.Setenv("SESSION_SECRET", productionSessionSecret)
			t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", tc.key)
			t.Setenv("SESSION_COOKIE_SECURE", "true")
			t.Setenv("APP_ENV", "production")
			t.Setenv("RELAY_ENABLED", "true")
			t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
			t.Setenv("CLICKHOUSE_DSN", "tcp://clickhouse:9000?database=oblivious")

			_, err := Load()
			if err == nil {
				t.Fatal("expected error for weak production secret encryption key")
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q error, got %v", tc.wantMessage, err)
			}
		})
	}
}

func TestLoadRejectsProductionInsecureSessionCookie(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", productionSessionSecret)
	t.Setenv("OBLIVIOUS_SECRET_ENCRYPTION_KEY", productionSecretEncryptionKey)
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("APP_ENV", "production")
	t.Setenv("RELAY_ENABLED", "true")
	t.Setenv("OBSERVABILITY_REQUEST_LOG_BACKEND", "clickhouse")
	t.Setenv("CLICKHOUSE_DSN", "tcp://clickhouse:9000?database=oblivious")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for insecure production session cookie")
	}
	if !strings.Contains(err.Error(), "SESSION_COOKIE_SECURE=false is not allowed") {
		t.Fatalf("expected secure cookie error, got %v", err)
	}
}

func TestLoadRejectsProductionWithoutConfiguredPaymentProvider(t *testing.T) {
	setProductionRuntimeEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production without a configured payment provider")
	}
	if !strings.Contains(err.Error(), "at least one payment provider must be fully configured") {
		t.Fatalf("expected production payment provider error, got %v", err)
	}
}

func TestLoadRejectsProductionPartialStripeConfig(t *testing.T) {
	setProductionRuntimeEnv(t)
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_partial")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for partial production Stripe config")
	}
	if !strings.Contains(err.Error(), "STRIPE_SECRET_KEY, STRIPE_SUCCESS_URL, STRIPE_CANCEL_URL, and STRIPE_WEBHOOK_SECRET are required together") {
		t.Fatalf("expected partial Stripe config error, got %v", err)
	}
}

func TestLoadRejectsProductionPartialDomesticPaymentConfig(t *testing.T) {
	setProductionRuntimeEnv(t)
	t.Setenv("ALIPAY_CHECKOUT_BASE_URL", "https://checkout.alipay.example.test/pay")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for partial production Alipay config")
	}
	if !strings.Contains(err.Error(), "ALIPAY_CHECKOUT_BASE_URL and ALIPAY_WEBHOOK_SECRET are required together") {
		t.Fatalf("expected partial Alipay config error, got %v", err)
	}
}

func TestLoadAcceptsProductionWithStripePaymentConfig(t *testing.T) {
	setProductionRuntimeEnv(t)
	setProductionStripePaymentEnv(t)
	setProductionPayoutWebhookEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected production Stripe config to load, got %v", err)
	}
	if cfg.StripeSecretKey != "sk_live_configured" || cfg.StripeWebhookSecret != "whsec_live_configured" {
		t.Fatalf("expected production Stripe config to be retained, got key=%q webhook=%q", cfg.StripeSecretKey, cfg.StripeWebhookSecret)
	}
}

func TestLoadAcceptsProductionWithDomesticPaymentConfig(t *testing.T) {
	setProductionRuntimeEnv(t)
	t.Setenv("ALIPAY_CHECKOUT_BASE_URL", "https://checkout.alipay.example.test/pay")
	t.Setenv("ALIPAY_WEBHOOK_SECRET", "alipay_live_webhook_secret")
	setProductionPayoutWebhookEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected production domestic payment config to load, got %v", err)
	}
	if cfg.AlipayCheckoutBaseURL != "https://checkout.alipay.example.test/pay" || cfg.AlipayWebhookSecret != "alipay_live_webhook_secret" {
		t.Fatalf("expected production Alipay config to be retained, got url=%q secret=%q", cfg.AlipayCheckoutBaseURL, cfg.AlipayWebhookSecret)
	}
}

func TestLoadRejectsProductionDomesticPaymentConfigWithInsecureCheckoutURL(t *testing.T) {
	for _, tc := range []struct {
		name      string
		urlEnv    string
		secretEnv string
		message   string
	}{
		{
			name:      "alipay",
			urlEnv:    "ALIPAY_CHECKOUT_BASE_URL",
			secretEnv: "ALIPAY_WEBHOOK_SECRET",
			message:   "ALIPAY_CHECKOUT_BASE_URL must use https",
		},
		{
			name:      "wechatpay",
			urlEnv:    "WECHATPAY_CHECKOUT_BASE_URL",
			secretEnv: "WECHATPAY_WEBHOOK_SECRET",
			message:   "WECHATPAY_CHECKOUT_BASE_URL must use https",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setProductionRuntimeEnv(t)
			setProductionPayoutWebhookEnv(t)
			t.Setenv(tc.urlEnv, "http://checkout."+tc.name+".example.test/pay")
			t.Setenv(tc.secretEnv, tc.name+"_live_webhook_secret")

			_, err := Load()
			if err == nil {
				t.Fatal("expected error for insecure production domestic checkout URL")
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("expected insecure checkout URL error %q, got %v", tc.message, err)
			}
		})
	}
}

func TestLoadRejectsProductionWithoutMarketplacePayoutProvider(t *testing.T) {
	setProductionRuntimeEnv(t)
	setProductionStripePaymentEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for production without marketplace payout provider")
	}
	if !strings.Contains(err.Error(), "MARKETPLACE_PAYOUT_PROVIDER=local is not allowed") {
		t.Fatalf("expected production marketplace payout provider error, got %v", err)
	}
}

func TestLoadRejectsMarketplacePayoutWebhookWithoutSecret(t *testing.T) {
	setProductionRuntimeEnv(t)
	setProductionStripePaymentEnv(t)
	t.Setenv("MARKETPLACE_PAYOUT_PROVIDER", "webhook")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_URL", "https://payments.oblivious.ai/marketplace/payouts")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for incomplete marketplace payout webhook config")
	}
	if !strings.Contains(err.Error(), "MARKETPLACE_PAYOUT_WEBHOOK_URL and MARKETPLACE_PAYOUT_WEBHOOK_SECRET are required together") {
		t.Fatalf("expected incomplete marketplace payout webhook error, got %v", err)
	}
}

func TestLoadRejectsInvalidMarketplacePayoutWebhookURL(t *testing.T) {
	setProductionRuntimeEnv(t)
	setProductionStripePaymentEnv(t)
	t.Setenv("MARKETPLACE_PAYOUT_PROVIDER", "webhook")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_URL", "ftp://payments.oblivious.ai/marketplace/payouts")
	t.Setenv("MARKETPLACE_PAYOUT_WEBHOOK_SECRET", "payout_webhook_secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid marketplace payout webhook URL")
	}
	if !strings.Contains(err.Error(), "MARKETPLACE_PAYOUT_WEBHOOK_URL must use http or https") {
		t.Fatalf("expected invalid marketplace payout webhook URL error, got %v", err)
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

func TestLoadDomesticPaymentCheckoutConfig(t *testing.T) {
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("ALIPAY_CHECKOUT_BASE_URL", " https://checkout.alipay.example.test/pay ")
	t.Setenv("ALIPAY_WEBHOOK_SECRET", " alipay_webhook_secret ")
	t.Setenv("WECHATPAY_CHECKOUT_BASE_URL", " https://checkout.wechatpay.example.test/pay ")
	t.Setenv("WECHATPAY_WEBHOOK_SECRET", " wechatpay_webhook_secret ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.AlipayCheckoutBaseURL != "https://checkout.alipay.example.test/pay" {
		t.Fatalf("expected alipay checkout base URL, got %q", cfg.AlipayCheckoutBaseURL)
	}
	if cfg.AlipayWebhookSecret != "alipay_webhook_secret" {
		t.Fatalf("expected alipay webhook secret, got %q", cfg.AlipayWebhookSecret)
	}
	if cfg.WeChatPayCheckoutBaseURL != "https://checkout.wechatpay.example.test/pay" {
		t.Fatalf("expected wechatpay checkout base URL, got %q", cfg.WeChatPayCheckoutBaseURL)
	}
	if cfg.WeChatPayWebhookSecret != "wechatpay_webhook_secret" {
		t.Fatalf("expected wechatpay webhook secret, got %q", cfg.WeChatPayWebhookSecret)
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
