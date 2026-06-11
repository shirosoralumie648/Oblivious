package config

import (
	"fmt"
	"os"
	"strings"
)

type BillingConfig struct {
	CommonConfig
	Port string
}

func LoadBillingConfig() *BillingConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}

	port := strings.TrimSpace(os.Getenv("BILLING_PORT"))
	if port == "" {
		port = "8087"
	}

	return &BillingConfig{
		CommonConfig: common,
		Port:         port,
	}
}
