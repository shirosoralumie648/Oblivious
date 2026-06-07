package handler

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/relay/channel"
)

// FineTuningHandler Fine-tuning 处理（全透传）
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
	case path == "/v1/fine_tuning/jobs" && method == "POST":
		h.HandleCreateJob(c)
	case path == "/v1/fine_tuning/jobs" && method == "GET":
		h.HandleListJobs(c)
	case path == "/v1/fine_tuning/jobs/"+id && method == "GET":
		h.HandleGetJob(c)
	case path == "/v1/fine_tuning/jobs/"+id+"/cancel" && method == "POST":
		h.HandleCancelJob(c)
	case path == "/v1/fine_tuning/jobs/"+id+"/events" && method == "GET":
		h.HandleEvents(c)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown fine_tuning path"}})
	}
	return nil
}

func (h *FineTuningHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *FineTuningHandler) HandleCreateJob(c *gin.Context) { h.passthrough(c, "POST", "/v1/fine_tuning/jobs") }
func (h *FineTuningHandler) HandleListJobs(c *gin.Context) { h.passthrough(c, "GET", "/v1/fine_tuning/jobs") }
func (h *FineTuningHandler) HandleGetJob(c *gin.Context)   { h.passthrough(c, "GET", "/v1/fine_tuning/jobs/"+c.Param("id")) }
func (h *FineTuningHandler) HandleCancelJob(c *gin.Context) { h.passthrough(c, "POST", "/v1/fine_tuning/jobs/"+c.Param("id")+"/cancel") }
func (h *FineTuningHandler) HandleEvents(c *gin.Context)    { h.passthrough(c, "GET", "/v1/fine_tuning/jobs/"+c.Param("id")+"/events") }

func (h *FineTuningHandler) passthrough(c *gin.Context, method, path string) {
	upstreamURL, _ := h.adapter.BuildURL("", types.APITypeFineTuning)
	upstreamURL = upstreamURL + path
	body, _ := io.ReadAll(c.Request.Body)
	req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), "", types.APITypeFineTuning)
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", bodyOut)
}
