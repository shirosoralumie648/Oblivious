package channel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeMessageLogArchiveStore struct {
	listInput ArchiveExpiredMessageLogsInput
	listed    []*ChannelMessageLog
	listErr   error

	deleteIDs []string
	deleteErr error
}

func (f *fakeMessageLogArchiveStore) ListExpiredMessageLogsForArchive(ctx context.Context, input ArchiveExpiredMessageLogsInput) ([]*ChannelMessageLog, error) {
	f.listInput = input
	return f.listed, f.listErr
}

func (f *fakeMessageLogArchiveStore) DeleteArchivedMessageLogs(ctx context.Context, ids []string) (ArchiveExpiredMessageLogsResult, error) {
	f.deleteIDs = append([]string(nil), ids...)
	if f.deleteErr != nil {
		return ArchiveExpiredMessageLogsResult{}, f.deleteErr
	}
	return ArchiveExpiredMessageLogsResult{ArchivedIDs: append([]string(nil), ids...), Count: len(ids)}, nil
}

type fakeMessageLogArchiveSink struct {
	objects []MessageLogArchiveObject
	err     error
}

func (f *fakeMessageLogArchiveSink) ArchiveMessageLogs(ctx context.Context, object MessageLogArchiveObject) error {
	f.objects = append(f.objects, object)
	return f.err
}

func TestServiceArchiveExpiredMessageLogsWritesObjectBeforeDeleting(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	before := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	logs := []*ChannelMessageLog{
		{
			ID:               "msg_old_recorded",
			ChannelID:        "channel_1",
			Direction:        DirectionOutbound,
			RawMessage:       json.RawMessage(`{"text":"old"}`),
			TransformSuccess: true,
			Status:           MessageStatusRecorded,
			CreatedAt:        before.Add(-time.Hour),
		},
		{
			ID:                 "msg_old_failure",
			ChannelID:          "channel_1",
			Direction:          DirectionOutbound,
			RawMessage:         json.RawMessage(`{"text":"failed"}`),
			TransformSuccess:   false,
			TransformError:     "delivery failed",
			Status:             MessageStatusPermanentFailure,
			FailureReason:      "delivery failed",
			TransformedMessage: InternalMessage{ID: "internal_1", Role: RoleAssistant},
			CreatedAt:          before.Add(-30 * time.Minute),
		},
	}
	store := &fakeMessageLogArchiveStore{listed: logs}
	sink := &fakeMessageLogArchiveSink{}

	result, err := NewService(nil).ArchiveExpiredMessageLogs(context.Background(), store, sink, MessageLogArchiveRequest{
		Before: before,
		Now:    now,
		Limit:  25,
	})

	if err != nil {
		t.Fatalf("ArchiveExpiredMessageLogs returned error: %v", err)
	}
	if !store.listInput.Before.Equal(before) || store.listInput.Limit != 25 {
		t.Fatalf("expected list input to preserve cutoff and limit, got %+v", store.listInput)
	}
	if len(sink.objects) != 1 {
		t.Fatalf("expected one archive object, got %+v", sink.objects)
	}
	object := sink.objects[0]
	if object.Key == "" || !strings.HasPrefix(object.Key, "channel-message-logs/") || !strings.HasSuffix(object.Key, ".json") {
		t.Fatalf("expected object key under channel-message-logs with json suffix, got %q", object.Key)
	}
	if !object.Before.Equal(before) || !object.CreatedAt.Equal(now) {
		t.Fatalf("expected archive object timestamps before=%s createdAt=%s, got before=%s createdAt=%s", before, now, object.Before, object.CreatedAt)
	}
	if len(object.Logs) != 2 || object.Logs[0].ID != "msg_old_recorded" || object.Logs[1].ID != "msg_old_failure" {
		t.Fatalf("expected archive object to contain listed logs, got %+v", object.Logs)
	}
	if !reflect.DeepEqual(store.deleteIDs, []string{"msg_old_recorded", "msg_old_failure"}) {
		t.Fatalf("expected delete after object archive for listed ids, got %+v", store.deleteIDs)
	}
	if result.Count != 2 || result.ObjectKey != object.Key {
		t.Fatalf("expected archive result to include count and object key, got %+v", result)
	}
}

func TestServiceArchiveExpiredMessageLogsDoesNotDeleteWhenObjectArchiveFails(t *testing.T) {
	before := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	store := &fakeMessageLogArchiveStore{listed: []*ChannelMessageLog{{
		ID:         "msg_old_recorded",
		ChannelID:  "channel_1",
		Direction:  DirectionInbound,
		RawMessage: json.RawMessage(`{"text":"old"}`),
		Status:     MessageStatusRecorded,
		CreatedAt:  before.Add(-time.Hour),
	}}}
	sink := &fakeMessageLogArchiveSink{err: errors.New("object storage unavailable")}

	_, err := NewService(nil).ArchiveExpiredMessageLogs(context.Background(), store, sink, MessageLogArchiveRequest{Before: before})

	if err == nil || !strings.Contains(err.Error(), "object storage unavailable") {
		t.Fatalf("expected object storage error, got %v", err)
	}
	if len(store.deleteIDs) != 0 {
		t.Fatalf("expected no database delete when archive sink fails, got %+v", store.deleteIDs)
	}
}

func TestArchiveWorkerRunsRetentionArchiveOnTicks(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	store := &fakeMessageLogArchiveStore{listed: []*ChannelMessageLog{{
		ID:         "msg_old_recorded",
		ChannelID:  "channel_1",
		Direction:  DirectionInbound,
		RawMessage: json.RawMessage(`{"text":"old"}`),
		Status:     MessageStatusRecorded,
		CreatedAt:  now.Add(-8 * 24 * time.Hour),
	}}}
	sink := &fakeMessageLogArchiveSink{}
	worker := NewArchiveWorker(NewService(nil), store, sink, ArchiveWorkerConfig{
		Retention: 7 * 24 * time.Hour,
		Limit:     10,
		Now:       func() time.Time { return now },
	})

	worker.runOnce(context.Background())

	if !store.listInput.Before.Equal(now.Add(-7*24*time.Hour)) || store.listInput.Limit != 10 {
		t.Fatalf("expected retention-based archive input, got %+v", store.listInput)
	}
	if len(sink.objects) != 1 {
		t.Fatalf("expected worker to write one archive object, got %+v", sink.objects)
	}
}

func TestMessageLogArchiveObjectKeyIsUniqueForSameSecondBatches(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	first := messageLogArchiveObjectKey(now)
	second := messageLogArchiveObjectKey(now)

	if first == second {
		t.Fatalf("expected archive object keys to be unique for same-second batches, got %q", first)
	}
	for _, key := range []string{first, second} {
		if !strings.HasPrefix(key, "channel-message-logs/20260607T100000Z-") || !strings.HasSuffix(key, ".json") {
			t.Fatalf("expected timestamped archive object key with unique suffix, got %q", key)
		}
	}
}

func TestFileMessageLogArchiveSinkWritesJSONObject(t *testing.T) {
	root := t.TempDir()
	sink := NewFileMessageLogArchiveSink(root)
	object := MessageLogArchiveObject{
		Key:       "channel-message-logs/20260607T100000Z.json",
		Before:    time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		Logs: []*ChannelMessageLog{{
			ID:         "msg_1",
			ChannelID:  "channel_1",
			Direction:  DirectionInbound,
			RawMessage: json.RawMessage(`{"text":"hello"}`),
			Status:     MessageStatusRecorded,
			CreatedAt:  time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC),
		}},
	}

	if err := sink.ArchiveMessageLogs(context.Background(), object); err != nil {
		t.Fatalf("ArchiveMessageLogs returned error: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(root, "channel-message-logs", "20260607T100000Z.json"))
	if err != nil {
		t.Fatalf("expected archive object file to be written: %v", err)
	}
	if !strings.Contains(string(payload), `"msg_1"`) || !strings.Contains(string(payload), `"raw_message"`) {
		t.Fatalf("expected archive payload to contain message log JSON, got %s", payload)
	}
}
