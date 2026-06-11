package queue

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
)

type TaskEvent struct {
	TaskID      string `json:"taskId"`
	WorkspaceID string `json:"workspaceId"`
	Event       string `json:"event"`
	Status      string `json:"status"`
}

type Publisher struct {
	writer *kafka.Writer
}

func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Publisher) Publish(ctx context.Context, event TaskEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Value: data})
}

func (p *Publisher) Close() error {
	return p.writer.Close()
}
