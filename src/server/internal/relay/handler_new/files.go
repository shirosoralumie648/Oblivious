package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

// FilesHandler 文件代理处理（upload + 透传）
type FilesHandler struct {
	pool         *types.ChannelPoolInterface
	adapter      *channel.OpenAIAdapter
	storagePath  string
	mappingStore FilesMappingStore
}

func NewFilesHandler(p *types.ChannelPoolInterface, a *channel.OpenAIAdapter, storagePath string) *FilesHandler {
	return &FilesHandler{pool: p, adapter: a, storagePath: storagePath}
}

func (h *FilesHandler) WithMappingStore(store FilesMappingStore) *FilesHandler {
	h.mappingStore = store
	return h
}

type FileMappingRecord struct {
	LocalFileID  string
	OpenAIFileID string
	LocalPath    string
	SizeBytes    int64
	CreatedAt    time.Time
}

type FilesMappingStore interface {
	SaveFileMapping(ctx context.Context, record FileMappingRecord) error
}

func (h *FilesHandler) Handle(c *gin.Context) error {
	path := c.Request.URL.Path
	switch {
	case path == "/v1/files" && c.Request.Method == "POST":
		return h.HandleUpload(c)
	case path == "/v1/files" && c.Request.Method == "GET":
		h.HandleList(c)
		return nil
	case path == "/v1/files/"+c.Param("id") && c.Request.Method == "GET":
		h.HandleGet(c)
		return nil
	case path == "/v1/files/"+c.Param("id") && c.Request.Method == "DELETE":
		h.HandleDelete(c)
		return nil
	case path == "/v1/files/"+c.Param("id")+"/content" && c.Request.Method == "GET":
		h.HandleContent(c)
		return nil
	}
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "unknown files path"}})
	return nil
}

func (h *FilesHandler) HandleStream(c *gin.Context) error {
	return h.Handle(c)
}

// POST /v1/files (文件代理：用户上传 -> 本地存储 -> 转发 OpenAI)
func (h *FilesHandler) HandleUpload(c *gin.Context) error {
	if h.mappingStore == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"code": "relay_file_mapping_store_required", "message": "file upload requires a configured mapping store"}})
		return nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "failed to read upload body"}})
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "no file provided"}})
		return nil
	}
	defer file.Close()

	// 1. 保存到本地 S3 兼容路径
	fileID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	localPath := filepath.Join(h.storagePath, "files", fileID+ext)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "failed to create storage directory"}})
		return nil
	}

	dst, err := os.Create(localPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "failed to save file"}})
		return nil
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "failed to save file"}})
		return nil
	}

	// 2. 转发到 OpenAI（简化：直接透传 multipart）
	upstreamURL, _ := h.adapter.BuildURL("", types.APITypeFiles)
	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}
	upstreamHeaders, _ := h.adapter.BuildHeaders(c.Request.Context(), "", types.APITypeFiles)
	upstreamReq.Header = upstreamHeaders
	upstreamReq.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	upstreamReq.ContentLength = int64(len(body))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		c.Data(resp.StatusCode, "application/json", responseBody)
		return nil
	}

	// 3. 解析 OpenAI 返回的 file_id，存入映射表
	var openAIResp map[string]any
	json.Unmarshal(responseBody, &openAIResp)
	openAIFileID, _ := openAIResp["id"].(string)
	if err := h.saveFileMapping(c.Request.Context(), fileID, openAIFileID, localPath, header.Size); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_file_mapping_failed", "message": "failed to save file mapping"}})
		return nil
	}

	c.Data(http.StatusOK, "application/json", responseBody)
	return nil
}

// GET /v1/files (透传)
func (h *FilesHandler) HandleList(c *gin.Context) {
	h.passthrough(c, "GET", "/v1/files", nil)
}

// GET /v1/files/:id (透传)
func (h *FilesHandler) HandleGet(c *gin.Context) {
	h.passthrough(c, "GET", "/v1/files/"+c.Param("id"), nil)
}

// DELETE /v1/files/:id (透传)
func (h *FilesHandler) HandleDelete(c *gin.Context) {
	h.passthrough(c, "DELETE", "/v1/files/"+c.Param("id"), nil)
}

// GET /v1/files/:id/content (透传)
func (h *FilesHandler) HandleContent(c *gin.Context) {
	h.passthrough(c, "GET", "/v1/files/"+c.Param("id")+"/content", nil)
}

func (h *FilesHandler) passthrough(c *gin.Context, method, path string, body []byte) {
	upstreamURL, _ := h.adapter.BuildURL("", types.APITypeFiles)
	upstreamURL = upstreamURL + path
	req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	headers, _ := h.adapter.BuildHeaders(c.Request.Context(), "", types.APITypeFiles)
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
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), bodyOut)
}

func (h *FilesHandler) saveFileMapping(ctx context.Context, localID, openaiID, path string, size int64) error {
	return h.mappingStore.SaveFileMapping(ctx, FileMappingRecord{
		LocalFileID:  localID,
		OpenAIFileID: openaiID,
		LocalPath:    path,
		SizeBytes:    size,
		CreatedAt:    time.Now().UTC(),
	})
}
