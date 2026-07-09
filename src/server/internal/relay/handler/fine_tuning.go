package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
)

// FineTuningHandler keeps fine-tuning fail-closed until job lifecycle,
// training-token billing, and audit contracts are implemented.
type FineTuningHandler struct {
	adapter *channel.OpenAIAdapter
}

func NewFineTuningHandler(a *channel.OpenAIAdapter) *FineTuningHandler {
	return &FineTuningHandler{adapter: a}
}

func (h *FineTuningHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	method := c.Request.Method
	id := c.Param("id")

	switch {
	case path == "/v1/fine_tuning/jobs" && method == http.MethodPost:
		h.HandleCreateJob(c)
	case path == "/v1/fine_tuning/jobs" && method == http.MethodGet:
		h.HandleListJobs(c)
	case path == "/v1/fine_tuning/jobs/"+id && method == http.MethodGet:
		h.HandleGetJob(c)
	case path == "/v1/fine_tuning/jobs/"+id+"/cancel" && method == http.MethodPost:
		h.HandleCancelJob(c)
	case path == "/v1/fine_tuning/jobs/"+id+"/events" && method == http.MethodGet:
		h.HandleEvents(c)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown fine_tuning path"}})
	}
	return nil
}

func (h *FineTuningHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *FineTuningHandler) HandleCreateJob(c *gin.Context) { h.writeDisabled(c) }
func (h *FineTuningHandler) HandleListJobs(c *gin.Context)  { h.writeDisabled(c) }
func (h *FineTuningHandler) HandleGetJob(c *gin.Context)    { h.writeDisabled(c) }
func (h *FineTuningHandler) HandleCancelJob(c *gin.Context) { h.writeDisabled(c) }
func (h *FineTuningHandler) HandleEvents(c *gin.Context)    { h.writeDisabled(c) }

func (h *FineTuningHandler) writeDisabled(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{
		"code":    "unsupported_api",
		"message": "fine-tuning is disabled until job lifecycle, billing, and audit are implemented",
	}})
}
