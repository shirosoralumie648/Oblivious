package config

import (
	"fmt"
	"os"
	"strings"
)

type AgentConfig struct {
	CommonConfig
	Port     string
	GRPCPort string
}

func LoadAgentConfig() *AgentConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}

	port := strings.TrimSpace(os.Getenv("AGENT_PORT"))
	if port == "" {
		port = "8083"
	}
	grpcPort := strings.TrimSpace(os.Getenv("AGENT_GRPC_PORT"))
	if grpcPort == "" {
		grpcPort = strings.TrimSpace(os.Getenv("GRPC_PORT"))
	}
	if grpcPort == "" {
		grpcPort = "50063"
	}

	return &AgentConfig{
		CommonConfig: common,
		Port:         port,
		GRPCPort:     grpcPort,
	}
}
