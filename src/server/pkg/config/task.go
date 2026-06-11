package config

import (
	"fmt"
	"os"
	"strings"
)

type TaskConfig struct {
	CommonConfig
	Port string
}

func LoadTaskConfig() *TaskConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}

	port := strings.TrimSpace(os.Getenv("TASK_PORT"))
	if port == "" {
		port = "8084"
	}

	return &TaskConfig{
		CommonConfig: common,
		Port:         port,
	}
}
