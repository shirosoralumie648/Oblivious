package task

import (
	"context"
	"io"
	"log"
	"net"
	"testing"

	taskpb "oblivious/server/internal/grpc/taskv1"
	"oblivious/server/internal/task/scheduler"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const taskBufSize = 1024 * 1024

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

func TestServer_ScheduleWithoutSchedulerFailsClosed(t *testing.T) {
	s := NewServer(nil, nil)

	resp, err := s.Schedule(context.Background(), &taskpb.ScheduleRequest{TaskId: "task1", CronExpr: "* * * * *"})
	if err != nil {
		t.Fatalf("Schedule returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("Schedule should fail when scheduler is not configured")
	}
	if resp.Message != "scheduler is not configured" {
		t.Fatalf("Schedule message = %q, want scheduler is not configured", resp.Message)
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

func TestServer_CancelWithoutSchedulerFailsClosed(t *testing.T) {
	s := NewServer(nil, nil)

	resp, err := s.Cancel(context.Background(), &taskpb.CancelRequest{TaskId: "task1"})
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if resp.Success {
		t.Fatal("Cancel should fail when scheduler is not configured")
	}
	if resp.Message != "scheduler is not configured" {
		t.Fatalf("Cancel message = %q, want scheduler is not configured", resp.Message)
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

func TestTaskGeneratedClientDispatchesThroughRegisteredServer(t *testing.T) {
	listener := bufconn.Listen(taskBufSize)
	grpcServer := grpc.NewServer()
	taskpb.RegisterTaskServiceServer(grpcServer, NewServer(
		scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{}),
		log.New(io.Discard, "", 0),
	))

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn task server: %v", err)
	}
	defer conn.Close()

	client := taskpb.NewTaskServiceClient(conn)
	scheduleResp, err := client.Schedule(ctx, &taskpb.ScheduleRequest{
		TaskId:   "task-generated-client",
		CronExpr: "* * * * *",
		Payload:  []byte("payload"),
	})
	if err != nil {
		t.Fatalf("Schedule via generated client returned error: %v", err)
	}
	if !scheduleResp.Success {
		t.Fatalf("Schedule success = false, message = %q", scheduleResp.Message)
	}

	cancelResp, err := client.Cancel(ctx, &taskpb.CancelRequest{TaskId: "task-generated-client"})
	if err != nil {
		t.Fatalf("Cancel via generated client returned error: %v", err)
	}
	if !cancelResp.Success {
		t.Fatalf("Cancel success = false, message = %q", cancelResp.Message)
	}
}
