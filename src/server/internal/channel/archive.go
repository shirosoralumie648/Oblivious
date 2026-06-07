package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultArchiveWorkerInterval = time.Hour

type MessageLogArchiveSink interface {
	ArchiveMessageLogs(ctx context.Context, object MessageLogArchiveObject) error
}

type MessageLogArchiveObject struct {
	Key       string               `json:"key"`
	Before    time.Time            `json:"before"`
	CreatedAt time.Time            `json:"created_at"`
	Logs      []*ChannelMessageLog `json:"logs"`
}

type MessageLogArchiveRequest struct {
	Before time.Time
	Limit  int
	Now    time.Time
}

func (s *Service) ArchiveExpiredMessageLogs(ctx context.Context, store MessageLogArchiveStore, sink MessageLogArchiveSink, request MessageLogArchiveRequest) (ArchiveExpiredMessageLogsResult, error) {
	if store == nil {
		return ArchiveExpiredMessageLogsResult{}, fmt.Errorf("message log archive store is required")
	}
	if sink == nil {
		return ArchiveExpiredMessageLogsResult{}, fmt.Errorf("message log archive sink is required")
	}
	before, limit := normalizeArchiveExpiredMessageLogsInput(ArchiveExpiredMessageLogsInput{Before: request.Before, Limit: request.Limit})
	result := ArchiveExpiredMessageLogsResult{Before: before}
	if before.IsZero() {
		return result, fmt.Errorf("archive cutoff is required")
	}

	logs, err := store.ListExpiredMessageLogsForArchive(ctx, ArchiveExpiredMessageLogsInput{Before: before, Limit: limit})
	if err != nil {
		return result, err
	}
	if len(logs) == 0 {
		return result, nil
	}

	now := request.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	object := MessageLogArchiveObject{
		Key:       messageLogArchiveObjectKey(now),
		Before:    before,
		CreatedAt: now,
		Logs:      cloneChannelMessageLogs(logs),
	}
	if err := sink.ArchiveMessageLogs(ctx, object); err != nil {
		return result, fmt.Errorf("archive channel message logs object: %w", err)
	}

	ids := make([]string, 0, len(logs))
	for _, log := range logs {
		if log != nil && log.ID != "" {
			ids = append(ids, log.ID)
		}
	}
	result, err = store.DeleteArchivedMessageLogs(ctx, ids)
	if err != nil {
		return result, err
	}
	result.Before = before
	result.ObjectKey = object.Key
	return result, nil
}

type FileMessageLogArchiveSink struct {
	Root string
}

func NewFileMessageLogArchiveSink(root string) *FileMessageLogArchiveSink {
	return &FileMessageLogArchiveSink{Root: strings.TrimSpace(root)}
}

func (s *FileMessageLogArchiveSink) ArchiveMessageLogs(ctx context.Context, object MessageLogArchiveObject) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(s.Root) == "" {
		return fmt.Errorf("archive root is required")
	}
	if strings.TrimSpace(object.Key) == "" {
		return fmt.Errorf("archive object key is required")
	}
	payload, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal channel message log archive object: %w", err)
	}
	path := filepath.Join(s.Root, filepath.FromSlash(object.Key))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create channel message log archive directory: %w", err)
	}
	if err := os.WriteFile(path, payload, 0600); err != nil {
		return fmt.Errorf("write channel message log archive object: %w", err)
	}
	return nil
}

type S3MessageLogArchiveSinkOptions struct {
	Endpoint   string
	Region     string
	Bucket     string
	AccessKey  string
	SecretKey  string
	HTTPClient *http.Client
	Now        func() time.Time
}

type S3MessageLogArchiveSink struct {
	endpoint   string
	region     string
	bucket     string
	accessKey  string
	secretKey  string
	httpClient *http.Client
	now        func() time.Time
}

func NewS3MessageLogArchiveSink(options S3MessageLogArchiveSinkOptions) *S3MessageLogArchiveSink {
	region := strings.TrimSpace(options.Region)
	if region == "" {
		region = "us-east-1"
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &S3MessageLogArchiveSink{
		endpoint:   strings.TrimRight(strings.TrimSpace(options.Endpoint), "/"),
		region:     region,
		bucket:     strings.Trim(strings.TrimSpace(options.Bucket), "/"),
		accessKey:  strings.TrimSpace(options.AccessKey),
		secretKey:  strings.TrimSpace(options.SecretKey),
		httpClient: client,
		now:        now,
	}
}

func (s *S3MessageLogArchiveSink) ArchiveMessageLogs(ctx context.Context, object MessageLogArchiveObject) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.httpClient == nil {
		return fmt.Errorf("s3 archive sink is required")
	}
	if s.endpoint == "" {
		return fmt.Errorf("s3 archive endpoint is required")
	}
	if s.bucket == "" {
		return fmt.Errorf("s3 archive bucket is required")
	}
	if s.accessKey == "" || s.secretKey == "" {
		return fmt.Errorf("s3 archive credentials are required")
	}
	if strings.TrimSpace(object.Key) == "" {
		return fmt.Errorf("archive object key is required")
	}
	payload, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal channel message log archive object: %w", err)
	}
	endpointURL, err := s.archiveObjectURL(object.Key)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpointURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build s3 archive request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	s.signPutObjectRequest(request, payload)

	response, err := s.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("put channel message log archive object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("put channel message log archive object returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s *S3MessageLogArchiveSink) archiveObjectURL(key string) (string, error) {
	parsed, err := url.Parse(s.endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid s3 archive endpoint: %q", s.endpoint)
	}
	prefix := strings.TrimRight(parsed.EscapedPath(), "/")
	parsed.Path = prefix + "/" + archivePathEscape(s.bucket) + "/" + archiveKeyEscape(key)
	parsed.RawPath = ""
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func (s *S3MessageLogArchiveSink) signPutObjectRequest(request *http.Request, payload []byte) {
	now := s.now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	payloadHash := sha256Hex(payload)
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)

	scope := date + "/" + s.region + "/s3/aws4_request"
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "content-type:" + request.Header.Get("Content-Type") + "\n" +
		"host:" + request.URL.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	canonicalRequest := strings.Join([]string{
		request.Method,
		request.URL.EscapedPath(),
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	signature := hex.EncodeToString(hmacSHA256(s3SigningKey(s.secretKey, date, s.region), stringToSign))
	request.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey,
		scope,
		signedHeaders,
		signature,
	))
}

func archiveKeyEscape(key string) string {
	parts := strings.Split(strings.TrimLeft(strings.TrimSpace(key), "/"), "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		escaped = append(escaped, archivePathEscape(part))
	}
	return strings.Join(escaped, "/")
}

func archivePathEscape(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "+", "%20")
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func s3SigningKey(secret, date, region string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), date)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, "s3")
	return hmacSHA256(serviceKey, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

type ArchiveWorkerConfig struct {
	Interval  time.Duration
	Retention time.Duration
	Limit     int
	Now       func() time.Time
	Ticks     <-chan time.Time
	OnError   func(error)
}

type ArchiveWorker struct {
	service   *Service
	store     MessageLogArchiveStore
	sink      MessageLogArchiveSink
	interval  time.Duration
	retention time.Duration
	limit     int
	now       func() time.Time
	ticks     <-chan time.Time
	onError   func(error)
}

func NewArchiveWorker(service *Service, store MessageLogArchiveStore, sink MessageLogArchiveSink, config ArchiveWorkerConfig) *ArchiveWorker {
	if service == nil {
		service = NewService(nil)
	}
	interval := config.Interval
	if interval <= 0 {
		interval = defaultArchiveWorkerInterval
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	retention := config.Retention
	if retention <= 0 {
		retention = defaultMessageLogRetention
	}
	return &ArchiveWorker{
		service:   service,
		store:     store,
		sink:      sink,
		interval:  interval,
		retention: retention,
		limit:     config.Limit,
		now:       now,
		ticks:     config.Ticks,
		onError:   config.OnError,
	}
}

func (w *ArchiveWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil || w.store == nil || w.sink == nil {
		return
	}
	ticks := w.ticks
	var ticker *time.Ticker
	if ticks == nil {
		ticker = time.NewTicker(w.interval)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			w.runOnce(ctx)
		}
	}
}

func (w *ArchiveWorker) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	now := w.now().UTC()
	_, err := w.service.ArchiveExpiredMessageLogs(ctx, w.store, w.sink, MessageLogArchiveRequest{
		Before: now.Add(-w.retention),
		Limit:  w.limit,
		Now:    now,
	})
	if err != nil && w.onError != nil {
		w.onError(err)
	}
}

func messageLogArchiveObjectKey(now time.Time) string {
	timestamp := now.UTC().Format("20060102T150405Z")
	return "channel-message-logs/" + timestamp + "-" + generateID("batch") + ".json"
}

func cloneChannelMessageLogs(logs []*ChannelMessageLog) []*ChannelMessageLog {
	cloned := make([]*ChannelMessageLog, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		item := *log
		item.RawMessage = append(json.RawMessage(nil), log.RawMessage...)
		if log.NextRetryAt != nil {
			nextRetryAt := log.NextRetryAt.UTC()
			item.NextRetryAt = &nextRetryAt
		}
		item.TransformedMessage.Content = append([]ContentPart(nil), log.TransformedMessage.Content...)
		if log.TransformedMessage.Metadata != nil {
			item.TransformedMessage.Metadata = cloneMap(log.TransformedMessage.Metadata)
		}
		cloned = append(cloned, &item)
	}
	return cloned
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
