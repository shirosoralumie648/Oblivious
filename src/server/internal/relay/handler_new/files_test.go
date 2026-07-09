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
)

func TestFilesUploadRequiresMappingStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called without mapping store")
	}))
	t.Cleanup(upstream.Close)

	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir())
	ctx, rec := newFilesMultipartUploadContext(t, "payload.jsonl", "hello", "assistants")

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

func TestFilesUploadPreservesMultipartAndSavesMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamSawFile bool
	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
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
	t.Cleanup(upstream.Close)

	store := &recordingFilesMappingStore{}
	handler := NewFilesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"), t.TempDir()).WithMappingStore(store)
	ctx, rec := newFilesMultipartUploadContext(t, "payload.jsonl", "hello", "assistants")

	if err := handler.HandleUpload(ctx); err != nil {
		t.Fatalf("HandleUpload error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/files" {
		t.Fatalf("upstream path = %q, want /v1/files", upstreamPath)
	}
	if !upstreamSawFile {
		t.Fatal("expected upstream to receive multipart file")
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
	if record.CreatedAt.IsZero() {
		t.Fatal("expected mapping created_at timestamp")
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if response["id"] != "file_openai_123" {
		t.Fatalf("response id = %#v, want upstream file id", response["id"])
	}
}

func newFilesMultipartUploadContext(t *testing.T, filename, content, purpose string) (*gin.Context, *httptest.ResponseRecorder) {
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

type recordingFilesMappingStore struct {
	records []FileMappingRecord
}

func (s *recordingFilesMappingStore) SaveFileMapping(ctx context.Context, record FileMappingRecord) error {
	s.records = append(s.records, record)
	return nil
}

var _ = time.Time{}
