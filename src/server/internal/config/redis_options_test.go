package config

import (
	"crypto/tls"
	"strings"
	"testing"
)

func TestRedisClientOptionsContract(t *testing.T) {
	t.Run("plain config preserves fields and default address", func(t *testing.T) {
		options, err := RedisClientOptions(Config{
			RedisPassword: "redis-secret",
			RedisDB:       4,
		}, "localhost:6379")
		if err != nil {
			t.Fatal(err)
		}
		if options.Addr != "localhost:6379" || options.Password != "redis-secret" || options.DB != 4 || options.Protocol != 2 {
			t.Fatalf("plain Redis options = addr=%q password=%q db=%d protocol=%d", options.Addr, options.Password, options.DB, options.Protocol)
		}
		if options.TLSConfig != nil {
			t.Fatal("plain Redis options unexpectedly enabled TLS")
		}
	})

	t.Run("TLS config verifies the endpoint host", func(t *testing.T) {
		options, err := RedisClientOptions(Config{
			RedisAddr:     "redis.internal:6380",
			RedisPassword: "redis-secret",
			RedisDB:       7,
			RedisTLS:      true,
		}, "")
		if err != nil {
			t.Fatal(err)
		}
		if options.TLSConfig == nil || options.TLSConfig.MinVersion < tls.VersionTLS12 {
			t.Fatalf("TLS options = %#v, want TLS 1.2 minimum", options.TLSConfig)
		}
		if options.TLSConfig.ServerName != "redis.internal" || options.TLSConfig.InsecureSkipVerify {
			t.Fatalf("TLS verification server_name=%q insecure=%t", options.TLSConfig.ServerName, options.TLSConfig.InsecureSkipVerify)
		}
	})

	malformed := []struct {
		name string
		raw  string
	}{
		{name: "leading whitespace", raw: " redis-secret.internal:6379"},
		{name: "control byte", raw: "redis\x01-secret.internal:6379"},
		{name: "path", raw: "redis-secret.internal/path:6379"},
		{name: "query", raw: "redis-secret.internal?db=1:6379"},
		{name: "userinfo", raw: "user@redis-secret.internal:6379"},
		{name: "blank DNS label", raw: "redis-secret..internal:6379"},
		{name: "leading hyphen", raw: "-redis-secret.internal:6379"},
		{name: "missing port", raw: "redis-secret.invalid-address"},
		{name: "nonnumeric port", raw: "redis-secret.invalid:invalid-port"},
		{name: "zero port", raw: "redis-secret.invalid:0"},
		{name: "overflow port", raw: "redis-secret.invalid:65536"},
	}
	for _, tlsEnabled := range []bool{false, true} {
		mode := "plain"
		wantError := "invalid redis configuration"
		if tlsEnabled {
			mode = "TLS"
			wantError = "invalid redis TLS configuration"
		}
		for _, test := range malformed {
			t.Run(mode+" rejects "+test.name, func(t *testing.T) {
				_, err := RedisClientOptions(Config{RedisAddr: test.raw, RedisTLS: tlsEnabled}, "")
				if err == nil || err.Error() != wantError || strings.Contains(err.Error(), test.raw) || strings.Contains(err.Error(), "redis-secret") {
					t.Fatalf("invalid %s address error = %v, want stable redacted error", mode, err)
				}
			})
		}
	}
}
