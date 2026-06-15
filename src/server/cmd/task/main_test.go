package main

import (
	"context"
	"net"
	"testing"
	"time"

	taskv1 "oblivious/server/internal/grpc/taskv1"
	"oblivious/server/internal/task/scheduler"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestRegisterTaskGRPCServiceServesGeneratedClient(t *testing.T) {
	taskScheduler := scheduler.NewCronScheduler(scheduler.CronSchedulerConfig{})
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	registerTaskGRPCService(grpcServer, taskScheduler)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn task server: %v", err)
	}
	defer conn.Close()

	client := taskv1.NewTaskServiceClient(conn)
	resp, err := client.Cancel(ctx, &taskv1.CancelRequest{TaskId: "grpc-smoke-missing"})
	if err != nil {
		t.Fatalf("Cancel over generated client returned error: %v", err)
	}
	if resp.GetSuccess() || resp.GetMessage() != "task not found" {
		t.Fatalf("unexpected validation response: %+v", resp)
	}
}
