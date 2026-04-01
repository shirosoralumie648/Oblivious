package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               int
	Env                string
	CORSAllowedOrigins []string
}

func Load() (Config, error) {
	portRaw := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if portRaw == "" {
		portRaw = "8080"
	}

	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid SERVER_PORT: %q", portRaw)
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "development"
	}

	originsRaw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	origins := []string{}
	if originsRaw != "" {
		for _, part := range strings.Split(originsRaw, ",") {
			value := strings.TrimSpace(part)
			if value != "" {
				origins = append(origins, value)
			}
		}
	}

	return Config{
		Port:               port,
		Env:                env,
		CORSAllowedOrigins: origins,
	}, nil
}
