package http

import (
	"database/sql"
	"fmt"
	"log"
	stdhttp "net/http"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/config"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/types"
)

func NewServer(cfg config.Config, database *sql.DB) *stdhttp.Server {
	// Create main router
	mainHandler := NewRouter(cfg, database)

	// If Relay is enabled, integrate it
	if cfg.RelayEnabled {
		relayStore := relay.NewRelayStore(database)
		pool := relay.NewChannelPool()

		// Load channels from database
		if err := relayStore.LoadPoolFromStore(pool); err != nil {
			log.Printf("warning: failed to load channels from database: %v", err)
		}

		// Ensure default channel for development
		ensureDefaultChannel(relayStore, pool, cfg)

		// Create Relay instance
		relayInstance, err := relay.NewRelay(&relay.Config{Pool: pool, Production: cfg.Env == "production"})
		if err != nil {
			log.Printf("warning: failed to create relay: %v", err)
		} else {
			// Wire quota.Service into the relay billing lifecycle so that
			// successful calls settle and failed calls refund correctly.
			quotaStore := quota.NewSQLStore(database)
			quotaService := quota.NewService(quotaStore)
			relayInstance.Router().SetQuotaManager(quotaService)

			// Mount Relay under /v1/*
			mainHandler = combineHandlers(mainHandler, relayInstance.Engine())
		}
	}

	return &stdhttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mainHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// ensureDefaultChannel creates a default OpenAI channel if no channels exist
func ensureDefaultChannel(store *relay.RelayStore, pool *relay.ChannelPool, cfg config.Config) {
	channels := pool.ListChannels()
	if len(channels) == 0 && cfg.OpenAIAPIKey != "" {
		defaultChannel := &types.Channel{
			ID:       uuid.New().String(),
			Name:     "Default OpenAI",
			Provider: "openai",
			BaseURL:  cfg.OpenAIBaseURL,
			APIKey:   cfg.OpenAIAPIKey,
			Models:   []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
			RPMLimit: 1000,
			TPMLimit: 100000,
			Enabled:  true,
			Strategy: "weighted",
			Priority: 0,
		}

		if err := store.CreateChannel(defaultChannel); err != nil {
			log.Printf("warning: failed to create default channel: %v", err)
		} else {
			pool.UpdateChannel(defaultChannel)
			log.Printf("created default OpenAI channel: %s", defaultChannel.ID)
		}
	}
}

// combineHandlers combines main router with relay engine
func combineHandlers(main stdhttp.Handler, relayEngine stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Route /v1/* to Relay
		if len(r.URL.Path) >= 3 && r.URL.Path[:3] == "/v1" {
			relayEngine.ServeHTTP(w, r)
			return
		}
		// Everything else goes to main router
		main.ServeHTTP(w, r)
	})
}
