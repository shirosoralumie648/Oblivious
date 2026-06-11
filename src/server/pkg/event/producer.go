package event

import (
	"context"

	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) Publish(ctx context.Context, key string, msg proto.Message) error {
	data, _ := proto.Marshal(msg)
	return p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: data})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
