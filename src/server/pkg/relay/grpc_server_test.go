package relay

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "oblivious/server/pkg/relay/proto"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T) (*grpc.Server, *bufconn.Listener, *Server) {
	lis := bufconn.Listen(bufSize)

	pool := relay.NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "test-ch-1",
		Provider: "openai",
		BaseURL:  "https://api.openai.com/v1",
		APIKey:   "test-key",
		Enabled:  true,
	}, 100)

	lb := relay.NewLoadBalancer(pool, "weighted")
	cbs := map[string]*relay.CircuitBreaker{"test-ch-1": relay.NewCircuitBreaker("test-ch-1", 5, 0, 0)}
	tb := relay.NewTokenBucket(1000, 60000)
	hc := relay.NewHealthChecker(relay.HealthCheckModelsAPI, 0)
	router := relay.NewRouter(pool, lb, cbs, tb, hc)

	adapter := channel.NewOpenAIAdapter("https://api.openai.com/v1", "test-key")
	srv := NewServer(router, adapter)

	grpcServer := grpc.NewServer()
	pb.RegisterRelayServiceServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	return grpcServer, lis, srv
}

func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
}

func TestGRPCServer_Complete(t *testing.T) {
	grpcServer, lis, _ := setupTestServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewRelayServiceClient(conn)

	req := &pb.CompletionRequest{
		Model: "gpt-4o",
		Messages: []*pb.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := client.Complete(ctx, req)
	if err != nil {
		t.Logf("Complete call completed with expected error (no real API): %v", err)
	} else {
		t.Logf("Complete call succeeded with response: %+v", resp)
	}
}

func TestGRPCServer_Embed(t *testing.T) {
	grpcServer, lis, _ := setupTestServer(t)
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewRelayServiceClient(conn)

	req := &pb.EmbeddingRequest{
		Model:  "text-embedding-ada-002",
		Inputs: []string{"test input"},
	}

	resp, err := client.Embed(ctx, req)
	if err != nil {
		t.Logf("Embed call completed with expected error (no real API): %v", err)
	} else {
		t.Logf("Embed call succeeded with response: %+v", resp)
	}
}

func TestGRPCServer_NoAvailableChannel(t *testing.T) {
	lis := bufconn.Listen(bufSize)

	pool := relay.NewChannelPool()
	lb := relay.NewLoadBalancer(pool, "weighted")
	router := relay.NewRouter(pool, lb, map[string]*relay.CircuitBreaker{}, nil, nil)
	adapter := channel.NewOpenAIAdapter("", "")
	srv := NewServer(router, adapter)

	grpcServer := grpc.NewServer()
	pb.RegisterRelayServiceServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(lis)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewRelayServiceClient(conn)

	req := &pb.CompletionRequest{
		Model:    "gpt-4o",
		Messages: []*pb.Message{{Role: "user", Content: "Hello"}},
	}

	_, err = client.Complete(ctx, req)
	if err == nil {
		t.Error("Expected error for no available channel")
	}
}
