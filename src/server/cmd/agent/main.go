package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	internalagent "oblivious/server/internal/agent"
	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	agentv1 "oblivious/server/internal/grpc/agentv1"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/mcp/websearch"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/workflow"
	"oblivious/server/internal/workflow/sandbox"
	pkgagent "oblivious/server/pkg/agent"
	agentconfig "oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	if err := agentconfig.RunEntrypoint(context.Background(), releasecontract.EntrypointID("agent"), agentconfig.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: agentconfig.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, agentconfig.ResolvedEntrypointInputs) error {
		agentCfg := agentconfig.LoadAgentConfig()
		runtimeCfg, err := config.Load()
		if err != nil {
			log.Fatalf("config load failed: %v", err)
		}

		database, err := db.Open(selectAgentDatabaseURL(runtimeCfg))
		if err != nil {
			log.Fatalf("agent db open failed: %v", err)
		}
		defer database.Close()

		agentService := buildAgentRuntimeService(runtimeCfg, database)
		httpServer := newHTTPHealthServer(agentCfg.Port)
		grpcServer := grpc.NewServer()
		registerAgentGRPCService(grpcServer, agentService)

		grpcListener, err := net.Listen("tcp", ":"+agentCfg.GRPCPort)
		if err != nil {
			log.Fatalf("agent grpc listen failed: %v", err)
		}

		serverErrors := make(chan error, 2)
		go func() {
			log.Printf("Agent HTTP health listening on :%s", agentCfg.Port)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrors <- fmt.Errorf("agent http server: %w", err)
			}
		}()
		go func() {
			log.Printf("Agent gRPC service listening on :%s", agentCfg.GRPCPort)
			if err := grpcServer.Serve(grpcListener); err != nil {
				serverErrors <- fmt.Errorf("agent grpc server: %w", err)
			}
		}()

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		select {
		case err := <-serverErrors:
			log.Fatalf("agent service failed: %v", err)
		case <-ctx.Done():
		}

		log.Println("Shutting down agent service...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("agent HTTP shutdown error: %v", err)
		}
		gracefulStopGRPC(grpcServer, 10*time.Second)
		return nil
	}); err != nil {
		log.Fatalf("agent preflight failed: %v", err)
	}
}

func newHTTPHealthServer(port string) *http.Server {
	router := gin.New()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "agent"})
	})
	return &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func registerAgentGRPCService(grpcServer *grpc.Server, runtime pkgagent.AgentRuntimeService) {
	agentv1.RegisterAgentServiceServer(grpcServer, pkgagent.NewServerWithAgentService(runtime))
}

func gracefulStopGRPC(grpcServer *grpc.Server, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		grpcServer.Stop()
	}
}

func selectAgentDatabaseURL(cfg config.Config) string {
	if cfg.DBMode != "monolith" && strings.TrimSpace(cfg.DBURLAgent) != "" {
		return cfg.DBURLAgent
	}
	return cfg.DatabaseURL
}

func buildAgentRuntimeService(cfg config.Config, database *sql.DB) *internalagent.Service {
	agentService := internalagent.NewService(internalagent.NewSQLStore(database), buildAgentGateway(cfg))
	agentService.SetMCPClient(mcp.NewClient(mcp.NewSQLStore(database)))
	if cfg.RelayEnabled {
		embedder := memory.NewRelayEmbedder(agentRelayBaseURL(cfg), "text-embedding-3-small")
		agentService.SetMemory(memory.NewService(memory.NewSQLStore(database), embedder, memory.DefaultChunker(), "text-embedding-3-small"))
		agentService.SetMemoryEmbedder(embedder)
	}
	if provider := buildAgentWebSearchProvider(cfg); provider != nil {
		agentService.SetWebSearchProvider(provider)
	}
	if runner := buildAgentCustomPythonSandboxRunner(cfg); runner != nil {
		agentService.SetCustomPythonSandboxRunner(runner)
	}
	return agentService
}

func buildAgentCustomPythonSandboxRunner(cfg config.Config) internalagent.CustomPythonSandboxRunner {
	if !cfg.WorkflowSandboxEnabled {
		return nil
	}
	var allowedLanguages []string
	for _, language := range strings.Split(cfg.WorkflowSandboxAllowedLanguages, ",") {
		language = strings.TrimSpace(language)
		if language != "" {
			allowedLanguages = append(allowedLanguages, language)
		}
	}
	return agentCustomPythonSandboxRunner{runner: sandbox.NewDockerSandboxRunner(sandbox.Config{
		Enabled:          true,
		AllowedLanguages: allowedLanguages,
		MemoryMB:         cfg.WorkflowSandboxMemoryMB,
		CPUs:             float64(cfg.WorkflowSandboxCPUs),
		DefaultTimeoutMS: cfg.WorkflowSandboxDefaultTimeoutMS,
		MaxTimeoutMS:     cfg.WorkflowSandboxMaxTimeoutMS,
	})}
}

type agentCustomPythonSandboxRunner struct {
	runner workflow.CodeRunner
}

func (r agentCustomPythonSandboxRunner) RunCustomPython(ctx context.Context, req internalagent.CustomPythonSandboxRequest) (*internalagent.CustomPythonSandboxResult, error) {
	result, err := r.runner.RunWorkflowCode(ctx, workflow.WorkflowCodeRequest{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		AgentID:        req.AgentID,
		RunID:          req.RunID,
		ToolRunID:      req.ToolRunID,
		ToolCallID:     req.ToolCallID,
		ToolName:       req.ToolName,
		RequestID:      req.RequestID,
		Language:       "python",
		Code:           req.Code,
		Inputs:         req.Inputs,
		TimeoutMS:      req.TimeoutMS,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &internalagent.CustomPythonSandboxResult{
		Stdout:   stringFromWorkflowOutput(result.Output, "stdout"),
		Stderr:   stringFromWorkflowOutput(result.Output, "stderr"),
		ExitCode: intFromWorkflowOutput(result.Output, "exitCode"),
		Logs:     result.Logs,
		Raw:      result.Raw,
	}, nil
}

func stringFromWorkflowOutput(output map[string]any, key string) string {
	if value, ok := output[key].(string); ok {
		return value
	}
	return ""
}

func intFromWorkflowOutput(output map[string]any, key string) int {
	switch value := output[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func buildAgentGateway(cfg config.Config) chat.ChatGateway {
	if !cfg.RelayEnabled {
		if cfg.Env == "production" {
			return chat.NewLocalGateway(nil)
		}
		localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
		return chat.NewLocalGateway(localGenerator)
	}
	relayGateway := chat.NewRelayGateway(
		chat.WithRelayURL(agentRelayBaseURL(cfg)),
		chat.WithDefaultModel(cfg.RelayDefaultModel),
	)
	if cfg.Env != "production" {
		localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
		return chat.NewCompositeGateway(relayGateway, localGenerator)
	}
	return relayGateway
}

func agentRelayBaseURL(cfg config.Config) string {
	if baseURL := strings.TrimSpace(cfg.AgentRelayBaseURL); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	return "http://localhost:8080/v1"
}

func buildAgentWebSearchProvider(cfg config.Config) mcp.WebSearchProvider {
	providerName := strings.ToLower(strings.TrimSpace(cfg.AgentWebSearchProvider))
	if providerName == "" {
		return nil
	}
	if providerName == "tavily" {
		return mcp.NewTavilyWebSearchProvider(mcp.TavilyWebSearchProviderConfig{
			Endpoint:    cfg.AgentWebSearchEndpoint,
			APIKey:      cfg.AgentWebSearchAPIKey,
			ResultLimit: cfg.AgentWebSearchResultLimit,
		})
	}
	provider, err := websearch.NewProviderFromConfig(websearch.Config{
		Provider:    providerName,
		APIKey:      cfg.AgentWebSearchAPIKey,
		Endpoint:    cfg.AgentWebSearchEndpoint,
		GoogleCSEID: cfg.AgentWebSearchGoogleCSEID,
	})
	if err != nil {
		log.Printf("warning: agent web search provider %q not configured: %v", providerName, err)
		return nil
	}
	return provider
}
