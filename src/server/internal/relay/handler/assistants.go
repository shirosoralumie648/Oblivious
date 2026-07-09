package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
)

// AssistantsHandler keeps Assistants / Threads / Runs fail-closed until their
// commercial lifecycle, billing, and governance contracts are implemented.
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
	case path == "/v1/assistants" && method == http.MethodPost:
		h.HandleCreate(c)
	case path == "/v1/assistants" && method == http.MethodGet:
		h.HandleList(c)
	case path == "/v1/assistants/"+id && method == http.MethodGet:
		h.HandleGet(c)
	case path == "/v1/assistants/"+id && method == http.MethodPost:
		h.HandleModify(c)
	case path == "/v1/assistants/"+id && method == http.MethodDelete:
		h.HandleDelete(c)
	case path == "/v1/threads" && method == http.MethodPost:
		h.HandleCreateThread(c)
	case path == "/v1/threads/"+id && method == http.MethodGet:
		h.HandleGetThread(c)
	case path == "/v1/threads/"+id+"/runs" && method == http.MethodPost:
		h.HandleCreateRun(c)
	case path == "/v1/threads/"+id+"/runs/"+c.Param("rid") && method == http.MethodGet:
		h.HandleGetRun(c)
	case path == "/v1/threads/"+id+"/runs/"+c.Param("rid")+"/submit" && method == http.MethodPost:
		h.HandleSubmitRun(c)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown assistants path"}})
	}
	return nil
}

func (h *AssistantsHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

func (h *AssistantsHandler) HandleCreate(c *gin.Context)       { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleList(c *gin.Context)         { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleGet(c *gin.Context)          { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleModify(c *gin.Context)       { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleDelete(c *gin.Context)       { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleCreateThread(c *gin.Context) { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleGetThread(c *gin.Context)    { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleCreateRun(c *gin.Context)    { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleGetRun(c *gin.Context)       { h.writeDisabled(c) }
func (h *AssistantsHandler) HandleSubmitRun(c *gin.Context)    { h.writeDisabled(c) }

func (h *AssistantsHandler) writeDisabled(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{
		"code":    "unsupported_api",
		"message": "assistants, threads, and runs are disabled until lifecycle billing and governance are implemented",
	}})
}
