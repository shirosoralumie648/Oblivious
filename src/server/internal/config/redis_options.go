package config

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/redis/go-redis/v9"
)

// RedisClientOptions is the shared transport authority for internal Config consumers.
func RedisClientOptions(cfg Config, defaultAddr string) (*redis.Options, error) {
	addr := cfg.RedisAddr
	if addr == "" {
		addr = defaultAddr
	}
	if err := validateHostPortEndpoint(addr); err != nil {
		if cfg.RedisTLS {
			return nil, fmt.Errorf("invalid redis TLS configuration")
		}
		return nil, fmt.Errorf("invalid redis configuration")
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
	host, _, _ := net.SplitHostPort(addr)
	options.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	}
	return options, nil
}
