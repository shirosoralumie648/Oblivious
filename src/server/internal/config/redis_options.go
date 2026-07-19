package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisClientOptions is the shared transport authority for internal Config consumers.
func RedisClientOptions(cfg Config, defaultAddr string) (*redis.Options, error) {
	addr := strings.TrimSpace(cfg.RedisAddr)
	if addr == "" {
		addr = strings.TrimSpace(defaultAddr)
	}
	options := &redis.Options{
		Addr:     addr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		Protocol: 2,
	}
	if !cfg.RedisTLS {
		return options, nil
	}
	host, portRaw, err := net.SplitHostPort(addr)
	port, portErr := strconv.Atoi(portRaw)
	if err != nil || strings.TrimSpace(host) == "" || portErr != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid redis TLS configuration")
	}
	options.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	}
	return options, nil
}
