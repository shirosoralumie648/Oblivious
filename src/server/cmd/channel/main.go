package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/channel"
	"oblivious/server/internal/config"
	"oblivious/server/internal/releasecontract"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("channel"), config.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, config.ResolvedEntrypointInputs) error {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		dbURL := cfg.DatabaseURL
		if cfg.DBMode != "monolith" && cfg.DBURLChannel != "" {
			dbURL = cfg.DBURLChannel
		}
		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		store := channel.NewSQLStore(db)
		service := channel.NewChannelService(&storeAdapter{store})
		router := gin.Default()
		registerRoutes(router, service)
		srv := &http.Server{Addr: ":8080", Handler: router}
		go func() {
			log.Println("Channel service listening on :8080")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("listen: %s\n", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down channel service...")
		_ = srv.Shutdown(context.Background())
		return nil
	}); err != nil {
		log.Fatalf("channel preflight failed: %v", err)
	}
}

func registerRoutes(r *gin.Engine, service *channel.ChannelService) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/events/bus", func(c *gin.Context) {
		bus := service.EventBus()
		if bus == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event bus not available"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"message": "event bus available for subscription",
		})
	})
}

type storeAdapter struct {
	*channel.SQLStore
}

func (s *storeAdapter) Create(ctx context.Context, input channel.CreateChannelInput) (*channel.ChannelConfig, error) {
	return s.CreateConfig(ctx, &channel.ChannelConfig{
		OrganizationID: input.OrganizationID,
		Type:           input.Type,
		Name:           input.Name,
		Config:         input.Config,
	})
}

func (s *storeAdapter) Get(ctx context.Context, organizationID, id string) (*channel.ChannelConfig, error) {
	return s.GetConfig(ctx, organizationID, id)
}

func (s *storeAdapter) GetByID(ctx context.Context, id string) (*channel.ChannelConfig, error) {
	return s.GetConfigByID(ctx, id)
}

func (s *storeAdapter) List(ctx context.Context, input channel.ListChannelsInput) ([]*channel.ChannelConfig, error) {
	return s.ListConfigs(ctx, input.OrganizationID)
}

func (s *storeAdapter) Update(ctx context.Context, organizationID, id string, input channel.UpdateChannelInput) (*channel.ChannelConfig, error) {
	update := channel.ConfigUpdate{
		Name:   input.Name,
		Config: input.Config,
	}
	if input.Type != nil {
		update.Type = *input.Type
	}
	if input.Status != nil {
		update.Status = *input.Status
	}
	return s.UpdateConfig(ctx, organizationID, id, update)
}

func (s *storeAdapter) Delete(ctx context.Context, organizationID, id string) error {
	_, err := s.UpdateConfigStatus(ctx, organizationID, id, channel.ChannelStatusDisabled)
	return err
}
