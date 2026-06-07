package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                int
	Env                 string
	CORSAllowedOrigins  []string
	DatabaseURL         string
	SessionCookieName   string
	SessionCookieSecure bool
	SessionSecret       string
	LLMBaseURL          string
	LLMAPIKey           string
	LLMTimeoutMS        int
	ModelDefaultName    string

	// Agent web search configuration. Provider stays disabled unless a supported
	// provider, endpoint, and API key are all configured.
	AgentWebSearchProvider    string
	AgentWebSearchEndpoint    string
	AgentWebSearchAPIKey      string
	AgentWebSearchResultLimit int

	// Qdrant vector database configuration. Disabled unless QDRANT_URL is set.
	QdrantURL        string
	QdrantAPIKey     string
	QdrantVectorSize int

	// RAG reranker configuration. Disabled unless RAG_RERANKER_BASE_URL is set.
	RAGRerankerBaseURL string
	RAGRerankerAPIKey  string
	RAGRerankerModel   string
	RAGRerankerTopK    int

	// Relay configuration
	RelayEnabled                 bool
	RelayDefaultModel            string
	RelayRateLimitBackend        string
	RelayRateLimitRedisKeyPrefix string
	RelaySemanticCacheBackend    string
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int

	// Default channel configuration (for development)
	OpenAIAPIKey  string
	OpenAIBaseURL string

	// Stripe billing configuration
	StripeSecretKey     string
	StripeSuccessURL    string
	StripeCancelURL     string
	StripeWebhookSecret string

	// Scheduled task worker configuration
	ScheduleWorkerEnabled    bool
	ScheduleWorkerIntervalMS int
	ScheduleWorkerClaimLimit int

	// Observability request log configuration. ClickHouse is disabled unless
	// explicitly selected because local/dev environments usually do not run it.
	ObservabilityRequestLogBackend string
	ClickHouseDSN                  string
	ClickHouseDriver               string
	ObservabilityHTTPAlertsEnabled bool
	AlertWebhookURL                string
	AlertWebhookSecret             string

	// HTTP recovery currently records restart actions for 5xx alert audits. It
	// does not execute infrastructure mutations.
	ObservabilityHTTPRecoveryEnabled    bool
	ObservabilityHTTPRecoveryCooldownMS int
}

func Load() (Config, error) {
	portRaw := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if portRaw == "" {
		portRaw = "8080"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %q", portRaw)
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "development"
	}

	originsRaw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	origins := []string{}
	if originsRaw != "" {
		for _, part := range strings.Split(originsRaw, ",") {
			value := strings.TrimSpace(part)
			if value != "" {
				origins = append(origins, value)
			}
		}
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	sessionSecret := strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	if sessionSecret == "" {
		return Config{}, fmt.Errorf("SESSION_SECRET is required")
	}

	sessionCookieName := strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
	if sessionCookieName == "" {
		sessionCookieName = "oblivious_session"
	}

	sessionCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE")), "true")
	llmBaseURL := strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	llmAPIKey := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	llmTimeoutMS := 30000
	llmTimeoutRaw := strings.TrimSpace(os.Getenv("LLM_TIMEOUT_MS"))
	if llmTimeoutRaw != "" {
		parsedTimeout, parseErr := strconv.Atoi(llmTimeoutRaw)
		if parseErr != nil || parsedTimeout < 1 {
			return Config{}, fmt.Errorf("invalid LLM_TIMEOUT_MS: %q", llmTimeoutRaw)
		}
		llmTimeoutMS = parsedTimeout
	}
	modelDefaultName := strings.TrimSpace(os.Getenv("MODEL_DEFAULT_NAME"))
	if modelDefaultName == "" {
		modelDefaultName = "demo-reply"
	}
	agentWebSearchProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_PROVIDER")))
	agentWebSearchEndpoint := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_ENDPOINT"))
	agentWebSearchAPIKey := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_API_KEY"))
	agentWebSearchResultLimit := 5
	agentWebSearchResultLimitRaw := strings.TrimSpace(os.Getenv("AGENT_WEB_SEARCH_RESULT_LIMIT"))
	if agentWebSearchResultLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(agentWebSearchResultLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid AGENT_WEB_SEARCH_RESULT_LIMIT: %q", agentWebSearchResultLimitRaw)
		}
		agentWebSearchResultLimit = parsedLimit
	}
	qdrantURL := strings.TrimSpace(os.Getenv("QDRANT_URL"))
	qdrantAPIKey := strings.TrimSpace(os.Getenv("QDRANT_API_KEY"))
	qdrantVectorSize := 1536
	qdrantVectorSizeRaw := strings.TrimSpace(os.Getenv("QDRANT_VECTOR_SIZE"))
	if qdrantVectorSizeRaw != "" {
		parsedSize, parseErr := strconv.Atoi(qdrantVectorSizeRaw)
		if parseErr != nil || parsedSize < 1 {
			return Config{}, fmt.Errorf("invalid QDRANT_VECTOR_SIZE: %q", qdrantVectorSizeRaw)
		}
		qdrantVectorSize = parsedSize
	}
	ragRerankerBaseURL := strings.TrimSpace(os.Getenv("RAG_RERANKER_BASE_URL"))
	ragRerankerAPIKey := strings.TrimSpace(os.Getenv("RAG_RERANKER_API_KEY"))
	ragRerankerModel := strings.TrimSpace(os.Getenv("RAG_RERANKER_MODEL"))
	if ragRerankerModel == "" {
		ragRerankerModel = "bge-reranker-large"
	}
	ragRerankerTopK := 5
	ragRerankerTopKRaw := strings.TrimSpace(os.Getenv("RAG_RERANKER_TOP_K"))
	if ragRerankerTopKRaw != "" {
		parsedTopK, parseErr := strconv.Atoi(ragRerankerTopKRaw)
		if parseErr != nil || parsedTopK < 1 {
			return Config{}, fmt.Errorf("invalid RAG_RERANKER_TOP_K: %q", ragRerankerTopKRaw)
		}
		ragRerankerTopK = parsedTopK
	}

	// Relay configuration
	relayEnabled := true
	if relayEnabledRaw := strings.TrimSpace(os.Getenv("RELAY_ENABLED")); relayEnabledRaw != "" {
		relayEnabled = strings.EqualFold(relayEnabledRaw, "true")
	}

	relayDefaultModel := strings.TrimSpace(os.Getenv("RELAY_DEFAULT_MODEL"))
	if relayDefaultModel == "" {
		relayDefaultModel = "gpt-4o-mini"
	}
	relaySemanticCacheBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_SEMANTIC_CACHE_BACKEND")))
	switch relaySemanticCacheBackend {
	case "":
		relaySemanticCacheBackend = "memory"
	case "none", "memory", "sql":
	default:
		return Config{}, fmt.Errorf("invalid RELAY_SEMANTIC_CACHE_BACKEND: %q", relaySemanticCacheBackend)
	}
	relayRateLimitBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_BACKEND")))
	switch relayRateLimitBackend {
	case "":
		relayRateLimitBackend = "memory"
	case "none", "memory", "redis":
	default:
		return Config{}, fmt.Errorf("invalid RELAY_RATE_LIMIT_BACKEND: %q", relayRateLimitBackend)
	}
	redisAddr := ""
	redisPassword := ""
	redisDB := 0
	if redisURLRaw := strings.TrimSpace(os.Getenv("REDIS_URL")); relayRateLimitBackend == "redis" && redisURLRaw != "" {
		parsedRedisURL, parseErr := parseRedisURL(redisURLRaw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		redisAddr = parsedRedisURL.addr
		redisPassword = parsedRedisURL.password
		redisDB = parsedRedisURL.db
	}
	if redisAddrRaw, ok := os.LookupEnv("REDIS_ADDR"); ok {
		redisAddr = strings.TrimSpace(redisAddrRaw)
	}
	if redisPasswordRaw, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		redisPassword = strings.TrimSpace(redisPasswordRaw)
	}
	if redisDBRaw := strings.TrimSpace(os.Getenv("REDIS_DB")); redisDBRaw != "" {
		parsedDB, parseErr := parseRedisDB(redisDBRaw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		redisDB = parsedDB
	}
	relayRateLimitRedisKeyPrefix := strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX"))

	// Default channel configuration (for development)
	openaiAPIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	openaiBaseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if openaiBaseURL == "" {
		openaiBaseURL = "https://api.openai.com"
	}

	stripeSecretKey := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
	stripeSuccessURL := strings.TrimSpace(os.Getenv("STRIPE_SUCCESS_URL"))
	stripeCancelURL := strings.TrimSpace(os.Getenv("STRIPE_CANCEL_URL"))
	stripeWebhookSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))

	scheduleWorkerEnabled := !strings.EqualFold(env, "test")
	if raw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_ENABLED")); raw != "" {
		scheduleWorkerEnabled = strings.EqualFold(raw, "true")
	}
	scheduleWorkerIntervalMS := 60000
	scheduleWorkerIntervalRaw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_INTERVAL_MS"))
	if scheduleWorkerIntervalRaw != "" {
		parsedInterval, parseErr := strconv.Atoi(scheduleWorkerIntervalRaw)
		if parseErr != nil || parsedInterval < 1 {
			return Config{}, fmt.Errorf("invalid SCHEDULE_WORKER_INTERVAL_MS: %q", scheduleWorkerIntervalRaw)
		}
		scheduleWorkerIntervalMS = parsedInterval
	}
	scheduleWorkerClaimLimit := 50
	scheduleWorkerClaimLimitRaw := strings.TrimSpace(os.Getenv("SCHEDULE_WORKER_CLAIM_LIMIT"))
	if scheduleWorkerClaimLimitRaw != "" {
		parsedLimit, parseErr := strconv.Atoi(scheduleWorkerClaimLimitRaw)
		if parseErr != nil || parsedLimit < 1 {
			return Config{}, fmt.Errorf("invalid SCHEDULE_WORKER_CLAIM_LIMIT: %q", scheduleWorkerClaimLimitRaw)
		}
		scheduleWorkerClaimLimit = parsedLimit
	}
	observabilityRequestLogBackend := strings.ToLower(strings.TrimSpace(os.Getenv("OBSERVABILITY_REQUEST_LOG_BACKEND")))
	switch observabilityRequestLogBackend {
	case "":
		observabilityRequestLogBackend = "none"
	case "none", "clickhouse":
	default:
		return Config{}, fmt.Errorf("invalid OBSERVABILITY_REQUEST_LOG_BACKEND: %q", observabilityRequestLogBackend)
	}
	clickHouseDSN := strings.TrimSpace(os.Getenv("CLICKHOUSE_DSN"))
	if observabilityRequestLogBackend == "clickhouse" && clickHouseDSN == "" {
		return Config{}, fmt.Errorf("CLICKHOUSE_DSN is required when OBSERVABILITY_REQUEST_LOG_BACKEND=clickhouse")
	}
	clickHouseDriver := strings.TrimSpace(os.Getenv("CLICKHOUSE_DRIVER"))
	if clickHouseDriver == "" {
		clickHouseDriver = "clickhouse"
	}
	observabilityHTTPAlertsEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_ALERTS_ENABLED")), "true")
	alertWebhookURL := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL"))
	alertWebhookSecret := strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_SECRET"))
	observabilityHTTPRecoveryEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_ENABLED")), "true")
	observabilityHTTPRecoveryCooldownMS := 300000
	observabilityHTTPRecoveryCooldownRaw := strings.TrimSpace(os.Getenv("OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS"))
	if observabilityHTTPRecoveryCooldownRaw != "" {
		parsedCooldown, parseErr := strconv.Atoi(observabilityHTTPRecoveryCooldownRaw)
		if parseErr != nil || parsedCooldown < 1 {
			return Config{}, fmt.Errorf("invalid OBSERVABILITY_HTTP_RECOVERY_COOLDOWN_MS: %q", observabilityHTTPRecoveryCooldownRaw)
		}
		observabilityHTTPRecoveryCooldownMS = parsedCooldown
	}

	return Config{
		Port:                         port,
		Env:                          env,
		CORSAllowedOrigins:           origins,
		DatabaseURL:                  databaseURL,
		SessionCookieName:            sessionCookieName,
		SessionCookieSecure:          sessionCookieSecure,
		SessionSecret:                sessionSecret,
		LLMBaseURL:                   llmBaseURL,
		LLMAPIKey:                    llmAPIKey,
		LLMTimeoutMS:                 llmTimeoutMS,
		ModelDefaultName:             modelDefaultName,
		AgentWebSearchProvider:       agentWebSearchProvider,
		AgentWebSearchEndpoint:       agentWebSearchEndpoint,
		AgentWebSearchAPIKey:         agentWebSearchAPIKey,
		AgentWebSearchResultLimit:    agentWebSearchResultLimit,
		QdrantURL:                    qdrantURL,
		QdrantAPIKey:                 qdrantAPIKey,
		QdrantVectorSize:             qdrantVectorSize,
		RAGRerankerBaseURL:           ragRerankerBaseURL,
		RAGRerankerAPIKey:            ragRerankerAPIKey,
		RAGRerankerModel:             ragRerankerModel,
		RAGRerankerTopK:              ragRerankerTopK,
		RelayEnabled:                 relayEnabled,
		RelayDefaultModel:            relayDefaultModel,
		RelayRateLimitBackend:        relayRateLimitBackend,
		RelayRateLimitRedisKeyPrefix: relayRateLimitRedisKeyPrefix,
		RelaySemanticCacheBackend:    relaySemanticCacheBackend,
		RedisAddr:                    redisAddr,
		RedisPassword:                redisPassword,
		RedisDB:                      redisDB,
		OpenAIAPIKey:                 openaiAPIKey,
		OpenAIBaseURL:                openaiBaseURL,
		StripeSecretKey:              stripeSecretKey,
		StripeSuccessURL:             stripeSuccessURL,
		StripeCancelURL:              stripeCancelURL,
		StripeWebhookSecret:          stripeWebhookSecret,

		ScheduleWorkerEnabled:    scheduleWorkerEnabled,
		ScheduleWorkerIntervalMS: scheduleWorkerIntervalMS,
		ScheduleWorkerClaimLimit: scheduleWorkerClaimLimit,

		ObservabilityRequestLogBackend: observabilityRequestLogBackend,
		ClickHouseDSN:                  clickHouseDSN,
		ClickHouseDriver:               clickHouseDriver,
		ObservabilityHTTPAlertsEnabled: observabilityHTTPAlertsEnabled,
		AlertWebhookURL:                alertWebhookURL,
		AlertWebhookSecret:             alertWebhookSecret,

		ObservabilityHTTPRecoveryEnabled:    observabilityHTTPRecoveryEnabled,
		ObservabilityHTTPRecoveryCooldownMS: observabilityHTTPRecoveryCooldownMS,
	}, nil
}

type redisURLConfig struct {
	addr     string
	password string
	db       int
}

func parseRedisURL(raw string) (redisURLConfig, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL: %q", raw)
	}
	if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL scheme: %q", parsed.Scheme)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL: host is required")
	}
	cfg := redisURLConfig{addr: parsed.Host}
	if password, ok := parsed.User.Password(); ok {
		cfg.password = password
	}
	dbRaw := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if dbRaw == "" {
		return cfg, nil
	}
	parsedDB, parseErr := parseRedisDB(dbRaw)
	if parseErr != nil {
		return redisURLConfig{}, fmt.Errorf("invalid REDIS_URL database: %q", dbRaw)
	}
	cfg.db = parsedDB
	return cfg, nil
}

func parseRedisDB(raw string) (int, error) {
	parsedDB, err := strconv.Atoi(raw)
	if err != nil || parsedDB < 0 {
		return 0, fmt.Errorf("invalid REDIS_DB: %q", raw)
	}
	return parsedDB, nil
}
