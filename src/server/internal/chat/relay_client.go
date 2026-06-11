package chat

import (
	"context"

	pb "oblivious/server/pkg/relay/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RelayClient struct {
	conn   *grpc.ClientConn
	client pb.RelayServiceClient
}

func NewRelayClient(relayAddr string) (*RelayClient, error) {
	conn, err := grpc.Dial(relayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &RelayClient{
		conn:   conn,
		client: pb.NewRelayServiceClient(conn),
	}, nil
}

func (c *RelayClient) Complete(ctx context.Context, model string, messages []Message) (string, error) {
	pbMessages := make([]*pb.Message, len(messages))
	for i, msg := range messages {
		pbMessages[i] = &pb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	req := &pb.CompletionRequest{Model: model, Messages: pbMessages}
	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *RelayClient) Close() error {
	return c.conn.Close()
}
