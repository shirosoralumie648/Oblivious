package config

import (
	"fmt"
	"os"
	"strings"
)

type TaskConfig struct {
	CommonConfig
	Port     string
	GRPCPort string
}

func LoadTaskConfig() *TaskConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}
	common = withServiceDatabaseURL(common, "DB_URL_TASK")

	port := strings.TrimSpace(os.Getenv("TASK_PORT"))
	if port == "" {
		port = "8084"
	}
	grpcPort := strings.TrimSpace(os.Getenv("TASK_GRPC_PORT"))
	if grpcPort == "" {
		grpcPort = strings.TrimSpace(os.Getenv("GRPC_PORT"))
	}
	if grpcPort == "" {
		grpcPort = "50065"
	}

	return &TaskConfig{
		CommonConfig: common,
		Port:         port,
		GRPCPort:     grpcPort,
	}
}
