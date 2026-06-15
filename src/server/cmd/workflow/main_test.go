package main

import (
	"context"
	"net"
	"testing"
	"time"

	workflowv1 "oblivious/server/internal/grpc/workflowv1"
	internalworkflow "oblivious/server/internal/workflow"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestRegisterWorkflowGRPCServiceServesGeneratedClient(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	registerWorkflowGRPCService(grpcServer, internalworkflow.NewService(nil))
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
		t.Fatalf("dial bufconn workflow server: %v", err)
	}
	defer conn.Close()

	client := workflowv1.NewWorkflowServiceClient(conn)
	resp, err := client.TestNode(ctx, &workflowv1.TestNodeRequest{
		NodeId:         "grpc-smoke-node",
		OrganizationId: "grpc-smoke-org",
	})
	if err != nil {
		t.Fatalf("TestNode over generated client returned error: %v", err)
	}
	if resp.GetStatus() != "failed" || resp.GetError() == "" {
		t.Fatalf("unexpected validation response: %+v", resp)
	}
}
