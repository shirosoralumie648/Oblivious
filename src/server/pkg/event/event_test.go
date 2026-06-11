package event

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestProducerPublish(t *testing.T) {
	brokers := []string{"localhost:9092"}
	topic := "test-topic"

	producer := NewProducer(brokers, topic)
	defer producer.Close()

	msg := wrapperspb.String("test message")
	err := producer.Publish(context.Background(), "test-key", msg)
	if err != nil {
		t.Skipf("Kafka not available: %v", err)
	}
}

func TestConsumerSubscribe(t *testing.T) {
	brokers := []string{"localhost:9092"}
	topic := "test-topic"

	consumer := NewConsumer(brokers, topic, "test-group")
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan bool, 1)

	go func() {
		err := consumer.Subscribe(ctx, func(key string, msg []byte) error {
			received <- true
			cancel()
			return nil
		})
		if err != nil && err != context.Canceled {
			t.Logf("Subscribe error: %v", err)
		}
	}()

	producer := NewProducer(brokers, topic)
	defer producer.Close()

	testMsg := wrapperspb.String("test")
	err := producer.Publish(context.Background(), "key", testMsg)
	if err != nil {
		t.Skipf("Kafka not available: %v", err)
	}

	select {
	case <-received:
		t.Log("Message received successfully")
	case <-ctx.Done():
		t.Log("Test completed")
	}
}
