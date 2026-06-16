package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	LocalFileID    string
	OpenAIFileID   string
	LocalPath      string
	SizeBytes      int64
	UserID         string
	OrganizationID string
	RequestID      string
	CreatedAt      time.Time
}

type FilesMappingStore interface {
	SaveFileMapping(ctx context.Context, record FileMappingRecord) error
}

type fileMappingLookupStore interface {
	GetFileMapping(ctx context.Context, localFileID, userID, organizationID string) (FileMappingRecord, error)
}

type fileMappingListStore interface {
	ListFileMappings(ctx context.Context, userID, organizationID string) ([]FileMappingRecord, error)
}

var ErrFileMappingNotFound = errors.New("relay file mapping not found")

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
	applyTrustedInternalIdentity(c)

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

	// 2. 转发到上游文件接口（直接透传 multipart）
	resp, err := h.uploadToUpstream(c, body, c.GetHeader("Content-Type"))
	if err != nil {
		c.JSON(statusCodeForFilesUploadError(err), gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return nil
	}

	upstreamBody := resp.Content
	if resp.StatusCode >= http.StatusBadRequest {
		contentType := "application/json"
		if resp.Headers != nil && strings.TrimSpace(resp.Headers.Get("Content-Type")) != "" {
			contentType = resp.Headers.Get("Content-Type")
		}
		c.Data(resp.StatusCode, contentType, upstreamBody)
		return nil
	}

	// 3. 解析 OpenAI 返回的 file_id，存入映射表
	var openAIResp map[string]any
	_ = json.Unmarshal(upstreamBody, &openAIResp)
	openAIFileID, _ := openAIResp["id"].(string)
	if strings.TrimSpace(openAIFileID) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": "upstream file response missing id"}})
		return nil
	}
	if err := h.saveFileMapping(c.Request.Context(), fileID, openAIFileID, localPath, header.Size); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_file_mapping_failed", "message": "failed to save file mapping"}})
		return nil
	}

	openAIResp["id"] = fileID
	openAIResp["provider_file_id"] = openAIFileID
	clientBody, err := json.Marshal(openAIResp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_file_response_failed", "message": "failed to prepare file response"}})
		return nil
	}

	c.Data(http.StatusOK, "application/json", clientBody)
	return nil
}

// GET /v1/files (tenant-scoped mapped passthrough)
func (h *FilesHandler) HandleList(c *gin.Context) {
	h.passthroughMappedFileList(c)
}

// GET /v1/files/:id (透传)
func (h *FilesHandler) HandleGet(c *gin.Context) {
	h.passthroughMappedFile(c, "GET", c.Param("id"), "")
}

// DELETE /v1/files/:id (透传)
func (h *FilesHandler) HandleDelete(c *gin.Context) {
	h.passthroughMappedFile(c, "DELETE", c.Param("id"), "")
}

// GET /v1/files/:id/content (透传)
func (h *FilesHandler) HandleContent(c *gin.Context) {
	h.passthroughMappedFile(c, "GET", c.Param("id"), "/content")
}

func (h *FilesHandler) uploadToUpstream(c *gin.Context, body []byte, contentType string) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router != nil {
		return router.RouteWithBilling(c.Request.Context(), types.APITypeFiles, "", "", filesIdempotencyKey(c, "upload"), fileUploadUsage(body), func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			return uploadFilesBodyToChannel(c.Request.Context(), ch, body, contentType)
		})
	}
	if h.adapter == nil {
		return nil, types.ErrNoAvailableChannel
	}
	return uploadFilesBodyWithAdapter(c.Request.Context(), h.adapter, body, contentType)
}

func uploadFilesBodyToChannel(ctx context.Context, ch *types.RouteChannel, body []byte, contentType string) (*types.ProviderResponse, error) {
	if ch == nil || ch.Channel == nil {
		return nil, types.ErrNoAvailableChannel
	}
	adapter, err := channel.AdapterForChannel(ch.Channel)
	if err != nil {
		return nil, err
	}
	return uploadFilesBodyWithAdapter(ctx, adapter, body, contentType)
}

func uploadFilesBodyWithAdapter(ctx context.Context, adapter types.ProviderAdapter, body []byte, contentType string) (*types.ProviderResponse, error) {
	upstreamURL, err := adapter.BuildURL("", types.APITypeFiles)
	if err != nil {
		return nil, err
	}
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamHeaders, _ := adapter.BuildHeaders(ctx, "", types.APITypeFiles)
	upstreamReq.Header = upstreamHeaders
	upstreamReq.Header.Set("Content-Type", contentType)
	upstreamReq.ContentLength = int64(len(body))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	upstreamBody, _ := io.ReadAll(resp.Body)
	return &types.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Content:    upstreamBody,
		Done:       true,
	}, nil
}

func statusCodeForFilesUploadError(err error) int {
	if err == nil {
		return http.StatusBadGateway
	}
	if errors.Is(err, types.ErrNoAvailableChannel) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}

func (h *FilesHandler) passthrough(c *gin.Context, method, path string, body []byte) {
	resp, err := h.passthroughUpstream(c, method, path, body)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, types.ErrNoAvailableChannel) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	c.Data(resp.StatusCode, resp.Headers.Get("Content-Type"), resp.Content)
}

func (h *FilesHandler) passthroughUpstream(c *gin.Context, method, path string, body []byte) (*types.ProviderResponse, error) {
	router := GetRouter()
	if router != nil {
		return router.RouteWithBilling(c.Request.Context(), types.APITypeFiles, "", "", filesIdempotencyKey(c, strings.ToLower(method)), &types.Usage{}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			return passthroughFilesPathToChannel(c.Request.Context(), ch, method, path, body)
		})
	}
	if h.adapter == nil {
		return nil, types.ErrNoAvailableChannel
	}
	return passthroughFilesPathWithAdapter(c.Request.Context(), h.adapter, method, path, body)
}

func passthroughFilesPathToChannel(ctx context.Context, ch *types.RouteChannel, method, path string, body []byte) (*types.ProviderResponse, error) {
	if ch == nil || ch.Channel == nil {
		return nil, types.ErrNoAvailableChannel
	}
	adapter, err := channel.AdapterForChannel(ch.Channel)
	if err != nil {
		return nil, err
	}
	return passthroughFilesPathWithAdapter(ctx, adapter, method, path, body)
}

func passthroughFilesPathWithAdapter(ctx context.Context, adapter types.ProviderAdapter, method, path string, body []byte) (*types.ProviderResponse, error) {
	upstreamURL, err := adapter.BuildURL("", types.APITypeFiles)
	if err != nil {
		return nil, err
	}
	upstreamURL = upstreamURL + path
	req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	headers, _ := adapter.BuildHeaders(ctx, "", types.APITypeFiles)
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyOut, _ := io.ReadAll(resp.Body)
	return &types.ProviderResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Content:    bodyOut,
		Done:       true,
	}, nil
}

func (h *FilesHandler) passthroughMappedFile(c *gin.Context, method, localFileID, suffix string) {
	if h.mappingStore == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"code": "relay_file_mapping_store_required", "message": "file passthrough requires a configured mapping store"}})
		return
	}
	applyTrustedInternalIdentity(c)

	userID, hasUserID := types.TrustedUserIDFromContext(c.Request.Context())
	organizationID, hasOrganizationID := types.TrustedOrganizationIDFromContext(c.Request.Context())
	if !hasUserID || strings.TrimSpace(userID) == "" || !hasOrganizationID || strings.TrimSpace(organizationID) == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "relay_file_mapping_identity_required", "message": "file passthrough requires trusted tenant identity"}})
		return
	}

	lookupStore, ok := h.mappingStore.(fileMappingLookupStore)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"code": "relay_file_mapping_lookup_required", "message": "file passthrough requires a mapping lookup store"}})
		return
	}

	mapping, err := lookupStore.GetFileMapping(c.Request.Context(), localFileID, userID, organizationID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "relay_file_mapping_lookup_failed"
		message := "failed to lookup file mapping"
		if errors.Is(err, ErrFileMappingNotFound) {
			status = http.StatusNotFound
			code = "relay_file_mapping_not_found"
			message = "file mapping not found"
		}
		c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
		return
	}
	if strings.TrimSpace(mapping.OpenAIFileID) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "relay_file_mapping_not_found", "message": "file mapping not found"}})
		return
	}

	h.passthrough(c, method, "/"+mapping.OpenAIFileID+suffix, nil)
}

func (h *FilesHandler) passthroughMappedFileList(c *gin.Context) {
	if h.mappingStore == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"code": "relay_file_mapping_store_required", "message": "file list requires a configured mapping store"}})
		return
	}
	applyTrustedInternalIdentity(c)

	userID, hasUserID := types.TrustedUserIDFromContext(c.Request.Context())
	organizationID, hasOrganizationID := types.TrustedOrganizationIDFromContext(c.Request.Context())
	if !hasUserID || strings.TrimSpace(userID) == "" || !hasOrganizationID || strings.TrimSpace(organizationID) == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "relay_file_mapping_identity_required", "message": "file list requires trusted tenant identity"}})
		return
	}

	listStore, ok := h.mappingStore.(fileMappingListStore)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": gin.H{"code": "relay_file_mapping_list_required", "message": "file list requires a mapping list store"}})
		return
	}

	mappings, err := listStore.ListFileMappings(c.Request.Context(), userID, organizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "relay_file_mapping_list_failed", "message": "failed to list file mappings"}})
		return
	}
	if len(mappings) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"object":   "list",
			"data":     []any{},
			"has_more": false,
			"first_id": nil,
			"last_id":  nil,
		})
		return
	}

	resp, err := h.passthroughUpstream(c, http.MethodGet, "", nil)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, types.ErrNoAvailableChannel) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
		return
	}
	contentType := resp.Headers.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.Data(resp.StatusCode, contentType, resp.Content)
		return
	}

	rewritten, err := rewriteTenantFileList(resp.Content, mappings)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "relay_file_list_response_failed", "message": "failed to prepare tenant file list response"}})
		return
	}
	c.Data(http.StatusOK, "application/json", rewritten)
}

func rewriteTenantFileList(body []byte, mappings []FileMappingRecord) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	data, ok := payload["data"].([]any)
	if !ok {
		return nil, errors.New("upstream file list response missing data")
	}

	byProviderID := make(map[string]FileMappingRecord, len(mappings))
	for _, mapping := range mappings {
		openAIFileID := strings.TrimSpace(mapping.OpenAIFileID)
		if openAIFileID == "" || strings.TrimSpace(mapping.LocalFileID) == "" {
			continue
		}
		byProviderID[openAIFileID] = mapping
	}

	filtered := make([]any, 0, len(data))
	localIDs := make([]string, 0, len(data))
	for _, item := range data {
		file, ok := item.(map[string]any)
		if !ok {
			continue
		}
		providerFileID, _ := file["id"].(string)
		mapping, ok := byProviderID[providerFileID]
		if !ok {
			continue
		}
		file["provider_file_id"] = providerFileID
		file["id"] = mapping.LocalFileID
		filtered = append(filtered, file)
		localIDs = append(localIDs, mapping.LocalFileID)
	}

	payload["data"] = filtered
	payload["has_more"] = false
	if len(localIDs) == 0 {
		payload["first_id"] = nil
		payload["last_id"] = nil
	} else {
		payload["first_id"] = localIDs[0]
		payload["last_id"] = localIDs[len(localIDs)-1]
	}

	return json.Marshal(payload)
}

// saveFileMapping 将本地 fileID 和 OpenAI fileID 的映射存入 DB
func (h *FilesHandler) saveFileMapping(ctx context.Context, localID, openaiID, path string, size int64) error {
	userID, _ := types.TrustedUserIDFromContext(ctx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	requestID, _ := types.TrustedRequestIDFromContext(ctx)
	return h.mappingStore.SaveFileMapping(ctx, FileMappingRecord{
		LocalFileID:    localID,
		OpenAIFileID:   openaiID,
		LocalPath:      path,
		SizeBytes:      size,
		UserID:         userID,
		OrganizationID: organizationID,
		RequestID:      requestID,
		CreatedAt:      time.Now().UTC(),
	})
}

func fileUploadUsage(body []byte) *types.Usage {
	return &types.Usage{StorageBytes: int64(len(body))}
}

func filesIdempotencyKey(c *gin.Context, operation string) string {
	if c != nil {
		if key := strings.TrimSpace(c.GetHeader("Idempotency-Key")); key != "" {
			return key
		}
	}
	return "files_" + strings.TrimSpace(operation) + "_" + uuid.NewString()
}
