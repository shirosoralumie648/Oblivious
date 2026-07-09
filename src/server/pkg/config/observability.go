package config

import (
	"fmt"
	"os"
	"strings"
)

type ObservabilityConfig struct {
	CommonConfig
	Port string
}

func LoadObservabilityConfig() *ObservabilityConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}
	common = withServiceDatabaseURL(common, "DB_URL_OBSERVABILITY")

	port := strings.TrimSpace(os.Getenv("OBSERVABILITY_PORT"))
	if port == "" {
		port = "8090"
	}

	return &ObservabilityConfig{
		CommonConfig: common,
		Port:         port,
	}
}
