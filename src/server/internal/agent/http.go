package agent

import (
	"oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
)

type HTTPService struct {
	cfg     *config.AgentConfig
	service *Service
}

func New(cfg *config.AgentConfig) *HTTPService {
	return &HTTPService{
		cfg:     cfg,
		service: NewService(nil, nil),
	}
}

func (h *HTTPService) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "agent"})
	})
}
