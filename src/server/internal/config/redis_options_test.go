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

	for _, raw := range []string{"redis-secret.invalid-address", "redis-secret.invalid:invalid-port", "redis-secret.invalid:65536"} {
		t.Run("invalid TLS address is redacted", func(t *testing.T) {
			_, err := RedisClientOptions(Config{RedisAddr: raw, RedisTLS: true}, "")
			if err == nil || err.Error() != "invalid redis TLS configuration" || strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), "redis-secret") {
				t.Fatalf("invalid TLS address error = %v, want stable redacted error", err)
			}
		})
	}
}
