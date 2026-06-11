package billing

import (
	"oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
)

type HTTPService struct {
	cfg *config.BillingConfig
}

func New(cfg *config.BillingConfig) *HTTPService {
	return &HTTPService{cfg: cfg}
}

func (h *HTTPService) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "billing"})
	})
}
