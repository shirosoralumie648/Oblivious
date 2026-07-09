package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestFilesUploadRequiresMappingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without mapping store")
	}))
	defer upstream.Close()

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir())
	ctx, rec := newMultipartUploadContext(t, "payload.jsonl", "hello", "assistants")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	if err := handler.HandleUpload(ctx); err != nil {
		t.Fatalf("HandleUpload error = %v", err)
	}

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_store_required") {
		t.Fatalf("expected mapping store guard, got %s", rec.Body.String())
	}
}

func TestFilesUploadReturnsLocalIDAndProviderIDAfterMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamSawFile bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/files" {
			t.Fatalf("upstream path = %s, want /v1/files", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("upstream method = %s, want POST", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Fatalf("upstream content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("upstream multipart parse error: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("upstream missing file: %v", err)
		}
		defer file.Close()
		if header.Filename != "payload.jsonl" {
			t.Fatalf("upstream filename = %q, want payload.jsonl", header.Filename)
		}
		if r.FormValue("purpose") != "assistants" {
			t.Fatalf("upstream purpose = %q, want assistants", r.FormValue("purpose"))
		}
		upstreamSawFile = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_123","object":"file","bytes":5}`))
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newMultipartUploadContext(t, "payload.jsonl", "hello", "assistants")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	if err := handler.HandleUpload(ctx); err != nil {
		t.Fatalf("HandleUpload error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !upstreamSawFile {
		t.Fatal("expected upstream to receive file")
	}
	if len(store.records) != 1 {
		t.Fatalf("mapping records = %d, want 1", len(store.records))
	}
	record := store.records[0]
	if record.LocalFileID == "" {
		t.Fatal("expected local file id")
	}
	if record.OpenAIFileID != "file_openai_123" {
		t.Fatalf("openai file id = %q, want file_openai_123", record.OpenAIFileID)
	}
	if record.LocalPath == "" || filepath.Ext(filepath.Base(record.LocalPath)) != ".jsonl" {
		t.Fatalf("expected local path with .jsonl extension, got %q", record.LocalPath)
	}
	if record.SizeBytes != 5 {
		t.Fatalf("size bytes = %d, want 5", record.SizeBytes)
	}
	if record.UserID != "user_file" || record.OrganizationID != "org_file" || record.RequestID != "req_file_upload" {
		t.Fatalf("owner evidence user=%q org=%q request=%q", record.UserID, record.OrganizationID, record.RequestID)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["id"] != record.LocalFileID {
		t.Fatalf("response id = %#v, want local file id %q", body["id"], record.LocalFileID)
	}
	if body["provider_file_id"] != "file_openai_123" {
		t.Fatalf("provider_file_id = %#v, want file_openai_123", body["provider_file_id"])
	}
}

func TestFilesUploadRoutesThroughBillingRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/files" {
			t.Fatalf("upstream path = %s, want /v1/files", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_billed","object":"file","bytes":5}`))
	}))
	defer upstream.Close()

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-files-upload",
			Name:     "Files Upload",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-files-upload",
			Enabled:  true,
		},
		ChannelID: "ch-files-upload",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter("http://direct.example.invalid", "sk-direct"), t.TempDir()).WithMappingStore(&recordingFilesMappingStore{})
	ctx, rec := newMultipartUploadContext(t, "payload.jsonl", "hello", "assistants")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	if err := handler.HandleUpload(ctx); err != nil {
		t.Fatalf("HandleUpload error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
}

func TestFilesListRequiresMappingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without mapping store")
	}))
	defer upstream.Close()

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir())
	ctx, rec := newFilesListContext()
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleList(ctx)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_store_required") {
		t.Fatalf("expected mapping store guard, got %s", rec.Body.String())
	}
}

func TestFilesListRequiresTrustedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without trusted tenant identity")
	}))
	defer upstream.Close()

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(&recordingFilesMappingStore{})
	ctx, rec := newFilesListContext()

	handler.HandleList(ctx)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_identity_required") {
		t.Fatalf("expected trusted identity guard, got %s", rec.Body.String())
	}
}

func TestFilesListRequiresListCapableMappingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without list-capable mapping store")
	}))
	defer upstream.Close()

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(&saveOnlyFilesMappingStore{})
	ctx, rec := newFilesListContext()
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleList(ctx)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_list_required") {
		t.Fatalf("expected mapping list guard, got %s", rec.Body.String())
	}
}

func TestFilesListFiltersTenantMappingsAndRewritesIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if r.Method != http.MethodGet {
			t.Fatalf("upstream method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object":"list",
			"data":[
				{"id":"file_openai_owned","object":"file","filename":"owned.jsonl","bytes":5},
				{"id":"file_openai_other","object":"file","filename":"other.jsonl","bytes":9}
			],
			"first_id":"file_openai_owned",
			"last_id":"file_openai_other",
			"has_more":true
		}`))
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{
		list: []FileMappingRecord{{
			LocalFileID:    "file_local_owned",
			OpenAIFileID:   "file_openai_owned",
			UserID:         "user_file",
			OrganizationID: "org_file",
		}},
	}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesListContext()
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleList(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/files" {
		t.Fatalf("upstream path = %q, want /v1/files", upstreamPath)
	}
	if store.listUserID != "user_file" || store.listOrganizationID != "org_file" {
		t.Fatalf("list evidence user=%q org=%q", store.listUserID, store.listOrganizationID)
	}

	var body struct {
		Data []struct {
			ID             string `json:"id"`
			ProviderFileID string `json:"provider_file_id"`
			Filename       string `json:"filename"`
		} `json:"data"`
		FirstID string `json:"first_id"`
		LastID  string `json:"last_id"`
		HasMore bool   `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response: %v\n%s", err, rec.Body.String())
	}
	if len(body.Data) != 1 {
		t.Fatalf("data len = %d, want 1; body=%s", len(body.Data), rec.Body.String())
	}
	if body.Data[0].ID != "file_local_owned" ||
		body.Data[0].ProviderFileID != "file_openai_owned" ||
		body.Data[0].Filename != "owned.jsonl" {
		t.Fatalf("unexpected rewritten file entry: %+v", body.Data[0])
	}
	if body.FirstID != "file_local_owned" || body.LastID != "file_local_owned" || body.HasMore {
		t.Fatalf("unexpected pagination metadata: first=%q last=%q has_more=%v", body.FirstID, body.LastID, body.HasMore)
	}
	if strings.Contains(rec.Body.String(), "file_openai_other") || strings.Contains(rec.Body.String(), "other.jsonl") {
		t.Fatalf("response leaked unmapped upstream file: %s", rec.Body.String())
	}
}

func TestFilesListWithNoTenantMappingsDoesNotCallUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called when tenant has no file mappings")
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesListContext()
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleList(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data    []any `json:"data"`
		HasMore bool  `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(body.Data) != 0 || body.HasMore {
		t.Fatalf("empty tenant list response mismatch: %+v", body)
	}
}

func TestFilesListRoutesMappedPassthroughThroughBillingRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"file_openai_123","object":"file"}],"has_more":false}`))
	}))
	defer upstream.Close()

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-files-list",
			Name:     "Files List",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-files-list",
			Enabled:  true,
		},
		ChannelID: "ch-files-list",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	store := &recordingFilesMappingStore{
		list: []FileMappingRecord{{
			LocalFileID:    "file_local_123",
			OpenAIFileID:   "file_openai_123",
			UserID:         "user_file",
			OrganizationID: "org_file",
		}},
	}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter("http://direct.example.invalid", "sk-direct"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesListContext()
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleList(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
	if upstreamPath != "/v1/files" {
		t.Fatalf("upstream path = %q, want /v1/files", upstreamPath)
	}
}

func TestFilesGetLooksUpTenantMappingBeforePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if r.Method != http.MethodGet {
			t.Fatalf("upstream method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_123","object":"file"}`))
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{
		lookup: FileMappingRecord{
			LocalFileID:    "file_local_123",
			OpenAIFileID:   "file_openai_123",
			UserID:         "user_file",
			OrganizationID: "org_file",
		},
	}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesIDContext(http.MethodGet, "/v1/files/file_local_123", "file_local_123")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleGet(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/files/file_openai_123" {
		t.Fatalf("upstream path = %q, want /v1/files/file_openai_123", upstreamPath)
	}
	if store.lookupLocalID != "file_local_123" ||
		store.lookupUserID != "user_file" ||
		store.lookupOrganizationID != "org_file" {
		t.Fatalf("lookup evidence local=%q user=%q org=%q", store.lookupLocalID, store.lookupUserID, store.lookupOrganizationID)
	}
}

func TestFilesGetRoutesMappedPassthroughThroughBillingRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_123","object":"file"}`))
	}))
	defer upstream.Close()

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-files-get",
			Name:     "Files Get",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-files-get",
			Enabled:  true,
		},
		ChannelID: "ch-files-get",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	store := &recordingFilesMappingStore{
		lookup: FileMappingRecord{
			LocalFileID:    "file_local_123",
			OpenAIFileID:   "file_openai_123",
			UserID:         "user_file",
			OrganizationID: "org_file",
		},
	}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter("http://direct.example.invalid", "sk-direct"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesIDContext(http.MethodGet, "/v1/files/file_local_123", "file_local_123")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleGet(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
	if upstreamPath != "/v1/files/file_openai_123" {
		t.Fatalf("upstream path = %q, want /v1/files/file_openai_123", upstreamPath)
	}
}

func TestFilesGetFailsClosedWithoutTenantMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without tenant mapping")
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{lookupErr: ErrFileMappingNotFound}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesIDContext(http.MethodGet, "/v1/files/file_other", "file_other")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleGet(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_not_found") {
		t.Fatalf("expected mapping not found guard, got %s", rec.Body.String())
	}
}

func TestFilesGetRequiresMappingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without mapping store")
	}))
	defer upstream.Close()

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir())
	ctx, rec := newFilesIDContext(http.MethodGet, "/v1/files/file_local_123", "file_local_123")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleGet(ctx)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "relay_file_mapping_store_required") {
		t.Fatalf("expected mapping store guard, got %s", rec.Body.String())
	}
}

func TestFilesDeleteTombstonesTenantMappingAfterUpstreamSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if r.Method != http.MethodDelete {
			t.Fatalf("upstream method = %s, want DELETE", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_123","object":"file","deleted":true}`))
	}))
	defer upstream.Close()

	store := &recordingFilesMappingStore{
		lookup: FileMappingRecord{
			LocalFileID:    "file_local_123",
			OpenAIFileID:   "file_openai_123",
			UserID:         "user_file",
			OrganizationID: "org_file",
		},
	}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesIDContext(http.MethodDelete, "/v1/files/file_local_123", "file_local_123")
	ctx.Request = ctx.Request.WithContext(trustedFilesContext(ctx.Request.Context()))

	handler.HandleDelete(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/files/file_openai_123" {
		t.Fatalf("upstream path = %q, want /v1/files/file_openai_123", upstreamPath)
	}
	if store.tombstoneLocalID != "file_local_123" ||
		store.tombstoneUserID != "user_file" ||
		store.tombstoneOrganizationID != "org_file" {
		t.Fatalf("tombstone evidence local=%q user=%q org=%q", store.tombstoneLocalID, store.tombstoneUserID, store.tombstoneOrganizationID)
	}
	if store.tombstoneDeletedAt.IsZero() {
		t.Fatal("expected tombstone deleted_at timestamp")
	}
}

func newMultipartUploadContext(t *testing.T, filename, content, purpose string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile error = %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file error = %v", err)
	}
	if purpose != "" {
		if err := writer.WriteField("purpose", purpose); err != nil {
			t.Fatalf("write purpose error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart close error = %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/files", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Request = req
	return ctx, rec
}

func newFilesIDContext(method, path, id string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: id}}
	ctx.Request = httptest.NewRequest(method, path, nil)
	return ctx, rec
}

func newFilesListContext() (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	return ctx, rec
}

func trustedFilesContext(ctx context.Context) context.Context {
	ctx = types.WithTrustedUserID(ctx, "user_file")
	ctx = types.WithTrustedOrganizationID(ctx, "org_file")
	ctx = types.WithTrustedRequestID(ctx, "req_file_upload")
	return ctx
}

type recordingFilesMappingStore struct {
	records                 []FileMappingRecord
	lookup                  FileMappingRecord
	lookupErr               error
	lookupLocalID           string
	lookupUserID            string
	lookupOrganizationID    string
	list                    []FileMappingRecord
	listErr                 error
	listUserID              string
	listOrganizationID      string
	tombstoneLocalID        string
	tombstoneUserID         string
	tombstoneOrganizationID string
	tombstoneDeletedAt      time.Time
}

func (s *recordingFilesMappingStore) SaveFileMapping(ctx context.Context, record FileMappingRecord) error {
	s.records = append(s.records, record)
	return nil
}

func (s *recordingFilesMappingStore) GetFileMapping(ctx context.Context, localFileID, userID, organizationID string) (FileMappingRecord, error) {
	s.lookupLocalID = localFileID
	s.lookupUserID = userID
	s.lookupOrganizationID = organizationID
	if s.lookupErr != nil {
		return FileMappingRecord{}, s.lookupErr
	}
	if s.lookup.LocalFileID == "" {
		return FileMappingRecord{}, ErrFileMappingNotFound
	}
	if s.lookup.LocalFileID != localFileID ||
		s.lookup.UserID != userID ||
		s.lookup.OrganizationID != organizationID {
		return FileMappingRecord{}, ErrFileMappingNotFound
	}
	return s.lookup, nil
}

func (s *recordingFilesMappingStore) ListFileMappings(ctx context.Context, userID, organizationID string) ([]FileMappingRecord, error) {
	s.listUserID = userID
	s.listOrganizationID = organizationID
	if s.listErr != nil {
		return nil, s.listErr
	}
	records := make([]FileMappingRecord, 0, len(s.list))
	for _, record := range s.list {
		if record.UserID == userID && record.OrganizationID == organizationID {
			records = append(records, record)
		}
	}
	return records, nil
}

func (s *recordingFilesMappingStore) TombstoneFileMapping(ctx context.Context, localFileID, userID, organizationID string, deletedAt time.Time) error {
	s.tombstoneLocalID = localFileID
	s.tombstoneUserID = userID
	s.tombstoneOrganizationID = organizationID
	s.tombstoneDeletedAt = deletedAt
	return nil
}

type saveOnlyFilesMappingStore struct{}

func (s *saveOnlyFilesMappingStore) SaveFileMapping(ctx context.Context, record FileMappingRecord) error {
	return nil
}
