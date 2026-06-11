package admin

import (
	"oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
)

type Admin struct {
	cfg     *config.AdminConfig
	service *Service
}

func New(cfg *config.AdminConfig) *Admin {
	return &Admin{
		cfg: cfg,
	}
}

func (a *Admin) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
