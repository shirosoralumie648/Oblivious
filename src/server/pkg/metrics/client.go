package metrics

import (
	"context"
	"time"

	observabilityv1 "oblivious/server/api/proto/observability/v1"
	"google.golang.org/grpc"
)

type Client struct {
	serviceName string
	instanceID  string
	conn        *grpc.ClientConn
	client      observabilityv1.ObservabilityServiceClient
}

func NewClient(serviceName, instanceID, observabilityAddr string) (*Client, error) {
	conn, err := grpc.Dial(observabilityAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Client{
		serviceName: serviceName,
		instanceID:  instanceID,
		conn:        conn,
		client:      observabilityv1.NewObservabilityServiceClient(conn),
	}, nil
}

func (c *Client) Publish(ctx context.Context, metrics []*observabilityv1.Metric, labels map[string]string) error {
	_, err := c.client.PublishMetrics(ctx, &observabilityv1.PublishMetricsRequest{
		ServiceName: c.serviceName,
		InstanceId:  c.instanceID,
		Timestamp:   time.Now().Unix(),
		Metrics:     metrics,
		Labels:      labels,
	})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
