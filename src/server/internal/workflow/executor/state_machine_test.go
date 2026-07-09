package executor

import (
	"context"
	"errors"
	"testing"
)

func TestStateMachineRecordsTransitionToSink(t *testing.T) {
	sink := &recordingTransitionSink{}
	sm := NewStateMachineWithTransitionSink(sink)

	status, err := sm.TransitionWithContext(context.Background(), "start")
	if err != nil {
		t.Fatalf("TransitionWithContext returned error: %v", err)
	}
	if status != "running" {
		t.Fatalf("status = %q, want running", status)
	}
	if len(sink.records) != 1 {
		t.Fatalf("sink records = %d, want 1", len(sink.records))
	}
	record := sink.records[0]
	if record.From != "draft" || record.To != "running" || record.Event != "start" {
		t.Fatalf("sink record = %+v, want draft -> running by start", record)
	}
	if record.Timestamp.IsZero() {
		t.Fatalf("sink record timestamp is zero")
	}
	if history := sm.History(); len(history) != 1 || history[0] != record {
		t.Fatalf("history = %+v, want same persisted transition record", history)
	}
}

func TestStateMachineKeepsStateWhenSinkFails(t *testing.T) {
	sinkErr := errors.New("sink unavailable")
	sm := NewStateMachineWithTransitionSink(&recordingTransitionSink{err: sinkErr})

	status, err := sm.TransitionWithContext(context.Background(), "start")
	if !errors.Is(err, sinkErr) {
		t.Fatalf("TransitionWithContext err = %v, want sink error", err)
	}
	if status != "draft" {
		t.Fatalf("status = %q, want unchanged draft", status)
	}
	if current := sm.CurrentStatus(); current != "draft" {
		t.Fatalf("current status = %q, want draft", current)
	}
	if history := sm.History(); len(history) != 0 {
		t.Fatalf("history = %+v, want no memory-only transition when sink fails", history)
	}
}

func TestStateMachineAllowsPausedExecutionToFail(t *testing.T) {
	sink := &recordingTransitionSink{}
	sm := NewStateMachineWithStatusAndTransitionSink("paused", sink)

	status, err := sm.TransitionWithContext(context.Background(), "fail")
	if err != nil {
		t.Fatalf("TransitionWithContext returned error: %v", err)
	}
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	if len(sink.records) != 1 {
		t.Fatalf("sink records = %d, want 1", len(sink.records))
	}
	record := sink.records[0]
	if record.From != "paused" || record.To != "failed" || record.Event != "fail" {
		t.Fatalf("sink record = %+v, want paused -> failed by fail", record)
	}
}

type recordingTransitionSink struct {
	err     error
	records []TransitionRecord
}

func (s *recordingTransitionSink) RecordTransition(_ context.Context, record TransitionRecord) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, record)
	return nil
}
