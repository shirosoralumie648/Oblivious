package main

import (
	"context"
	"log"
	"oblivious/server/internal/queue"
)

func startTaskConsumer(ctx context.Context, brokers []string) {
	consumer := queue.NewConsumer(brokers, "task.events", "relay-service")
	defer consumer.Close()

	err := consumer.Consume(ctx, func(event queue.TaskEvent) error {
		log.Printf("Relay received task event: %s for task %s", event.Event, event.TaskID)
		return nil
	})
	if err != nil {
		log.Printf("Consumer error: %v", err)
	}
}
