package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func loadRedisConfig() (addr, password string, db int, err error) {
	if redisURLRaw := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURLRaw != "" {
		parsed, parseErr := url.Parse(redisURLRaw)
		if parseErr != nil {
			return "", "", 0, fmt.Errorf("invalid REDIS_URL: %q", redisURLRaw)
		}
		if parsed.Scheme != "redis" && parsed.Scheme != "rediss" {
			return "", "", 0, fmt.Errorf("invalid REDIS_URL scheme: %q", parsed.Scheme)
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return "", "", 0, fmt.Errorf("invalid REDIS_URL: host is required")
		}
		addr = parsed.Host
		if pwd, ok := parsed.User.Password(); ok {
			password = pwd
		}
		dbRaw := strings.Trim(strings.TrimSpace(parsed.Path), "/")
		if dbRaw != "" {
			db, err = strconv.Atoi(dbRaw)
			if err != nil || db < 0 {
				return "", "", 0, fmt.Errorf("invalid REDIS_URL database: %q", dbRaw)
			}
		}
	}

	if redisAddrRaw, ok := os.LookupEnv("REDIS_ADDR"); ok {
		addr = strings.TrimSpace(redisAddrRaw)
	}
	if redisPasswordRaw, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		password = strings.TrimSpace(redisPasswordRaw)
	}
	if redisDBRaw := strings.TrimSpace(os.Getenv("REDIS_DB")); redisDBRaw != "" {
		db, err = strconv.Atoi(redisDBRaw)
		if err != nil || db < 0 {
			return "", "", 0, fmt.Errorf("invalid REDIS_DB: %q", redisDBRaw)
		}
	}

	return addr, password, db, nil
}
