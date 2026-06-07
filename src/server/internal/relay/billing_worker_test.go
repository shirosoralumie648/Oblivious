package relay

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestBillingTimeoutTask_Payload(t *testing.T) {
	task := &BillingTimeoutTask{
		SessionID:     "sess_123",
		ChannelID:     "ch_1",
		APIType:       types.APITypeChat,
		Model:         "gpt-4o",
		AuthAmt:       10.0,
		IdempotencyKey: "idem_123",
	}
	payload, err := structToPayload(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded BillingTimeoutTask
	if err := payloadToStruct(payload, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.SessionID != task.SessionID {
		t.Fatalf("expected %s, got %s", task.SessionID, decoded.SessionID)
	}
	if decoded.AuthAmt != task.AuthAmt {
		t.Fatalf("expected %f, got %f", task.AuthAmt, decoded.AuthAmt)
	}
}

func TestBillingPollingTask_Payload(t *testing.T) {
	task := &BillingPollingTask{
		SessionID:        "sess_123",
		ChannelID:        "ch_1",
		APIType:          types.APITypeChat,
		Model:            "gpt-4o",
		PreAuthorizedAmt: 10.0,
		IdempotencyKey:   "idem_123",
		MaxAttempts:      5,
		AttemptNo:        1,
	}
	payload, err := structToPayload(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded BillingPollingTask
	if err := payloadToStruct(payload, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.MaxAttempts != task.MaxAttempts {
		t.Fatalf("expected %d, got %d", task.MaxAttempts, decoded.MaxAttempts)
	}
	if decoded.AttemptNo != task.AttemptNo {
		t.Fatalf("expected %d, got %d", task.AttemptNo, decoded.AttemptNo)
	}
}

func TestNewBillingWorker(t *testing.T) {
	store := NewPricingStoreWithDefaults()
	billing := NewBillingHook(store, nil)
	worker := NewBillingWorker("localhost:6379", billing)
	if worker == nil {
		t.Fatal("NewBillingWorker should not return nil")
	}
	if worker.redisAddr != "localhost:6379" {
		t.Fatalf("expected localhost:6379, got %s", worker.redisAddr)
	}
	if worker.billing == nil {
		t.Fatal("billing hook should be set")
	}
}
