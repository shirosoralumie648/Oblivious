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

func trustedFilesContext(ctx context.Context) context.Context {
	ctx = types.WithTrustedUserID(ctx, "user_file")
	ctx = types.WithTrustedOrganizationID(ctx, "org_file")
	ctx = types.WithTrustedRequestID(ctx, "req_file_upload")
	return ctx
}

type recordingFilesMappingStore struct {
	records              []FileMappingRecord
	lookup               FileMappingRecord
	lookupErr            error
	lookupLocalID        string
	lookupUserID         string
	lookupOrganizationID string
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
