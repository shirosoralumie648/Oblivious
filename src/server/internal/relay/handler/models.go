package handler

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
)

type ModelsHandler struct {
	pool *types.ChannelPoolInterface
}

func NewModelsHandler(p *types.ChannelPoolInterface) *ModelsHandler {
	return &ModelsHandler{pool: p}
}

func (h *ModelsHandler) Handle(c *gin.Context) error {
	models := h.models()
	data := make([]gin.H, 0, len(models))
	created := time.Now().Unix()
	for _, model := range models {
		data = append(data, gin.H{
			"id":       model,
			"object":   "model",
			"created":  created,
			"owned_by": "oblivious",
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
	return nil
}

func (h *ModelsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *ModelsHandler) models() []string {
	if h == nil || h.pool == nil || *h.pool == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, ch := range (*h.pool).ListChannels() {
		if ch == nil || !ch.Enabled {
			continue
		}
		for _, model := range ch.Models {
			if model == "" {
				continue
			}
			seen[model] = struct{}{}
		}
	}
	models := make([]string, 0, len(seen))
	for model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}
