package rag

import (
	"oblivious/server/pkg/config"

	"github.com/gin-gonic/gin"
)

type RAG struct {
	cfg *config.RAGConfig
}

func New(cfg *config.RAGConfig) *RAG {
	return &RAG{cfg: cfg}
}

func (r *RAG) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
