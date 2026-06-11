package config

import (
	"fmt"
	"os"
	"strings"
)

type GatewayConfig struct {
	CommonConfig
	Port string
}

func LoadGatewayConfig() *GatewayConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}

	port := strings.TrimSpace(os.Getenv("GATEWAY_PORT"))
	if port == "" {
		port = "8080"
	}

	return &GatewayConfig{
		CommonConfig: common,
		Port:         port,
	}
}
