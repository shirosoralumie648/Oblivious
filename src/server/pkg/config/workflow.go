package config

import (
	"fmt"
	"os"
	"strings"
)

type WorkflowConfig struct {
	CommonConfig
	Port     string
	GRPCPort string
}

func LoadWorkflowConfig() *WorkflowConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}
	common = withServiceDatabaseURL(common, "DB_URL_WORKFLOW")

	port := strings.TrimSpace(os.Getenv("WORKFLOW_PORT"))
	if port == "" {
		port = strings.TrimSpace(os.Getenv("PORT"))
	}
	if port == "" {
		port = "8082"
	}
	grpcPort := strings.TrimSpace(os.Getenv("WORKFLOW_GRPC_PORT"))
	if grpcPort == "" {
		grpcPort = strings.TrimSpace(os.Getenv("GRPC_PORT"))
	}
	if grpcPort == "" {
		grpcPort = "50064"
	}

	return &WorkflowConfig{
		CommonConfig: common,
		Port:         port,
		GRPCPort:     grpcPort,
	}
}
