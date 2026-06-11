package task

import (
	"context"
	"log"
	"testing"

	taskpb "oblivious/server/internal/grpc/taskv1"
	"oblivious/server/internal/task/scheduler"
)

func TestServer_Schedule(t *testing.T) {
	s := NewServer(scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{}), nil)

	tests := []struct {
		name    string
		req     *taskpb.ScheduleRequest
		wantErr bool
		wantMsg string
	}{
		{
			name:    "empty task_id",
			req:     &taskpb.ScheduleRequest{TaskId: "", CronExpr: "* * * * *"},
			wantErr: false,
			wantMsg: "task_id is required",
		},
		{
			name:    "empty cron_expr",
			req:     &taskpb.ScheduleRequest{TaskId: "test", CronExpr: ""},
			wantErr: false,
			wantMsg: "cron_expr is required",
		},
		{
			name:    "valid request",
			req:     &taskpb.ScheduleRequest{TaskId: "test1", CronExpr: "* * * * *", Payload: []byte("test")},
			wantErr: false,
			wantMsg: "task scheduled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := s.Schedule(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Schedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("Schedule() message = %v, want %v", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestServer_Cancel(t *testing.T) {
	s := NewServer(scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{}), nil)

	s.scheduler.Add("task1", "* * * * *", func(ctx context.Context, entry scheduler.ScheduledEntry) error {
		return nil
	})

	tests := []struct {
		name    string
		req     *taskpb.CancelRequest
		wantErr bool
		wantMsg string
	}{
		{
			name:    "empty task_id",
			req:     &taskpb.CancelRequest{TaskId: ""},
			wantErr: false,
			wantMsg: "task_id is required",
		},
		{
			name:    "task not found",
			req:     &taskpb.CancelRequest{TaskId: "nonexistent"},
			wantErr: false,
			wantMsg: "task not found",
		},
		{
			name:    "valid cancel",
			req:     &taskpb.CancelRequest{TaskId: "task1"},
			wantErr: false,
			wantMsg: "task cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := s.Cancel(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Cancel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp.Message != tt.wantMsg {
				t.Errorf("Cancel() message = %v, want %v", resp.Message, tt.wantMsg)
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	s := NewServer(nil, nil)
	if s.logger == nil {
		t.Error("NewServer() should set default logger")
	}

	customLogger := log.Default()
	s = NewServer(nil, customLogger)
	if s.logger != customLogger {
		t.Error("NewServer() should use provided logger")
	}
}
