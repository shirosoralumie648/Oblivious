package queue

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
	}
}

func (c *Consumer) Consume(ctx context.Context, handler func(TaskEvent) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		var event TaskEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			continue
		}
		if err := handler(event); err == nil {
			c.reader.CommitMessages(ctx, msg)
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
