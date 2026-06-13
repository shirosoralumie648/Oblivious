package agent

import (
	"context"
	"time"

	ragv1 "oblivious/server/internal/grpc/ragv1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RAGClient struct {
	conn   *grpc.ClientConn
	client ragv1.RAGServiceClient
}

func NewRAGClient(addr string) (*RAGClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	return &RAGClient{
		conn:   conn,
		client: ragv1.NewRAGServiceClient(conn),
	}, nil
}

func (c *RAGClient) Search(ctx context.Context, query string, topK int32) (*ragv1.RetrieveResponse, error) {
	return c.client.Retrieve(ctx, &ragv1.RetrieveRequest{
		Query: query,
		TopK:  topK,
	})
}

func (c *RAGClient) Close() error {
	return c.conn.Close()
}
