package config

import (
	"fmt"
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
	ModelBaseURL        string
	ModelAPIKey         string
	ModelDefaultName    string
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
	modelBaseURL := strings.TrimSpace(os.Getenv("MODEL_BASE_URL"))
	modelAPIKey := strings.TrimSpace(os.Getenv("MODEL_API_KEY"))
	modelDefaultName := strings.TrimSpace(os.Getenv("MODEL_DEFAULT_NAME"))
	if modelDefaultName == "" {
		modelDefaultName = "demo-reply"
	}

	return Config{
		Port:                port,
		Env:                 env,
		CORSAllowedOrigins:  origins,
		DatabaseURL:         databaseURL,
		SessionCookieName:   sessionCookieName,
		SessionCookieSecure: sessionCookieSecure,
		SessionSecret:       sessionSecret,
		LLMBaseURL:          llmBaseURL,
		LLMAPIKey:           llmAPIKey,
		LLMTimeoutMS:        llmTimeoutMS,
		ModelBaseURL:        modelBaseURL,
		ModelAPIKey:         modelAPIKey,
		ModelDefaultName:    modelDefaultName,
	}, nil
}
