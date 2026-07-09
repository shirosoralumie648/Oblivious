package config

import (
	"fmt"
	"os"
	"strings"
)

type RAGConfig struct {
	CommonConfig
	Port string
}

func LoadRAGConfig() *RAGConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}
	common = withServiceDatabaseURL(common, "DB_URL_RAG")

	port := strings.TrimSpace(os.Getenv("RAG_PORT"))
	if port == "" {
		port = "8080"
	}

	return &RAGConfig{
		CommonConfig: common,
		Port:         port,
	}
}
