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

// AssistantsHandler Assistants / Threads / Runs 处理（全透传）
type AssistantsHandler struct {
	adapter *channel.OpenAIAdapter
}

func NewAssistantsHandler(a *channel.OpenAIAdapter) *AssistantsHandler {
	return &AssistantsHandler{adapter: a}
}

func (h *AssistantsHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	method := c.Request.Method
	id := c.Param("id")

	switch {
	case path == "/v1/assistants" && method == "POST":
		h.HandleCreate(c)
	case path == "/v1/assistants" && method == "GET":
		h.HandleList(c)
	case path == "/v1/assistants/"+id && method == "GET":
		h.HandleGet(c)
	case path == "/v1/assistants/"+id && method == "POST":
		h.HandleModify(c)
	case path == "/v1/assistants/"+id && method == "DELETE":
		h.HandleDelete(c)
	case path == "/v1/threads" && method == "POST":
		h.HandleCreateThread(c)
	case path == "/v1/threads/"+id && method == "GET":
		h.HandleGetThread(c)
	case path == "/v1/threads/"+id+"/runs" && method == "POST":
		h.HandleCreateRun(c)
	case path == "/v1/threads/"+id+"/runs/"+c.Param("rid") && method == "GET":
		h.HandleGetRun(c)
	case path == "/v1/threads/"+id+"/runs/"+c.Param("rid")+"/submit" && method == "POST":
		h.HandleSubmitRun(c)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown assistants path"}})
	}
	return nil
}

func (h *AssistantsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *AssistantsHandler) HandleCreate(c *gin.Context)   { h.passthrough(c, "POST", "/v1/assistants") }
func (h *AssistantsHandler) HandleList(c *gin.Context)   { h.passthrough(c, "GET", "/v1/assistants") }
func (h *AssistantsHandler) HandleGet(c *gin.Context)     { h.passthrough(c, "GET", "/v1/assistants/"+c.Param("id")) }
func (h *AssistantsHandler) HandleModify(c *gin.Context)  { h.passthrough(c, "POST", "/v1/assistants/"+c.Param("id")) }
func (h *AssistantsHandler) HandleDelete(c *gin.Context)  { h.passthrough(c, "DELETE", "/v1/assistants/"+c.Param("id")) }
func (h *AssistantsHandler) HandleCreateThread(c *gin.Context) { h.passthrough(c, "POST", "/v1/threads") }
func (h *AssistantsHandler) HandleGetThread(c *gin.Context) { h.passthrough(c, "GET", "/v1/threads/"+c.Param("id")) }
func (h *AssistantsHandler) HandleCreateRun(c *gin.Context) { h.passthrough(c, "POST", "/v1/threads/"+c.Param("id")+"/runs") }
func (h *AssistantsHandler) HandleGetRun(c *gin.Context) {
	h.passthrough(c, "GET", "/v1/threads/"+c.Param("id")+"/runs/"+c.Param("rid"))
}
func (h *AssistantsHandler) HandleSubmitRun(c *gin.Context) {
	h.passthrough(c, "POST", "/v1/threads/"+c.Param("id")+"/runs/"+c.Param("rid")+"/submit")
}

func (h *AssistantsHandler) passthrough(c *gin.Context, method, path string) {
	upstreamURL, _ := h.adapter.BuildURL("", types.APITypeAssistants)
	upstreamURL = upstreamURL + path
	body, _ := io.ReadAll(c.Request.Body)
	req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), "", types.APITypeAssistants)
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
