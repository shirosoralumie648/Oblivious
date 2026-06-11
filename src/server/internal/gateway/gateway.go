package gateway

import (
	"oblivious/server/pkg/config"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type Gateway struct {
	cfg     *config.GatewayConfig
	service *Service
}

func New(cfg *config.GatewayConfig) *Gateway {
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	}

	serviceCfg := ServiceConfig{
		JWTSecret:          []byte(cfg.SessionSecret),
		AllowedOrigins:     []string{"*"},
		RedisClient:        redisClient,
		RateLimitRPM:       100,
		RateLimitTPM:       10000,
		RateLimitOrgRPM:    1000,
		RateLimitOrgTPM:    100000,
		CBThreshold:        50.0,
		CBOpenDuration:     30 * time.Second,
		HealthCheckTargets: map[ServiceTarget]string{},
		HealthCheckTimeout: 5 * time.Second,
	}

	return &Gateway{
		cfg:     cfg,
		service: NewService(serviceCfg),
	}
}

func (g *Gateway) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", gin.WrapH(g.service))
	router.GET("/healthz", gin.WrapH(g.service))
	router.NoRoute(gin.WrapH(g.service))
}
