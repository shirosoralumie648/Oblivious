package agent

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RAGClient struct {
	conn   *grpc.ClientConn
	client RAGServiceClient
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
		client: NewRAGServiceClient(conn),
	}, nil
}

func (c *RAGClient) Search(ctx context.Context, query string, topK int32) (*SearchResponse, error) {
	return c.client.Search(ctx, &SearchRequest{
		Query: query,
		TopK:  topK,
	})
}

func (c *RAGClient) Close() error {
	return c.conn.Close()
}
