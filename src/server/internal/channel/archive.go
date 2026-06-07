package channel

import (
	"context"
	"encoding/json"
	"fmt"
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
