package config

import (
	"fmt"
	"os"
	"strings"
)

type AdminConfig struct {
	CommonConfig
	Port string
}

func LoadAdminConfig() *AdminConfig {
	common, err := LoadCommon()
	if err != nil {
		panic(fmt.Sprintf("Failed to load common config: %v", err))
	}
	common = withServiceDatabaseURL(common, "DB_URL_ADMIN")

	port := strings.TrimSpace(os.Getenv("ADMIN_PORT"))
	if port == "" {
		port = "8080"
	}

	return &AdminConfig{
		CommonConfig: common,
		Port:         port,
	}
}
