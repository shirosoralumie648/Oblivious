package relay

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hibiken/asynq"

	"oblivious/server/internal/relay/types"
)

const (
	BillingTimeoutQueue    = "billing_timeout"
	BillingPollingQueue    = "billing_polling"
	BillingTimeoutPayload  = "billing_timeout_payload"
	BillingPollingPayload = "billing_polling_payload"
)

type BillingTimeoutTask struct {
	SessionID     string
	ChannelID     string
	APIType       types.APIType
	Model         string
	AuthAmt       float64
	IdempotencyKey string
}

type BillingPollingTask struct {
	SessionID        string
	ChannelID        string
	APIType          types.APIType
	Model            string
	PreAuthorizedAmt float64
	IdempotencyKey   string
	MaxAttempts      int
	AttemptNo        int
}

type BillingWorker struct {
	server    *asynq.Server
	client    *asynq.Client
	billing   *BillingHook
	redisAddr string
}

func NewBillingWorker(redisAddr string, billing *BillingHook) *BillingWorker {
	redisConnOpt := asynq.RedisClientOpt{Addr: redisAddr}
	cfg := asynq.Config{}
	server := asynq.NewServer(redisConnOpt, cfg)
	client := asynq.NewClient(redisConnOpt)
	return &BillingWorker{
		server:    server,
		client:    client,
		billing:   billing,
		redisAddr: redisAddr,
	}
}

func EnqueueBillingTimeoutTask(redisAddr string, task *BillingTimeoutTask, delay time.Duration) error {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()
	payload, err := structToPayload(task)
	if err != nil {
		return err
	}
	_, err = client.Enqueue(asynq.NewTask(BillingTimeoutPayload, payload),
		asynq.MaxRetry(0),
		asynq.Timeout(delay),
		asynq.Queue(BillingTimeoutQueue))
	return err
}

func EnqueueBillingPollingTask(redisAddr string, task *BillingPollingTask) error {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()
	payload, err := structToPayload(task)
	if err != nil {
		return err
	}
	_, err = client.Enqueue(asynq.NewTask(BillingPollingPayload, payload),
		asynq.MaxRetry(0),
		asynq.Queue(BillingPollingQueue))
	return err
}

func (w *BillingWorker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(BillingTimeoutPayload, w.handleTimeout)
	mux.HandleFunc(BillingPollingPayload, w.handlePolling)
	return w.server.Run(mux)
}

func (w *BillingWorker) Stop() {
	w.server.Shutdown()
}

func structToPayload(v any) ([]byte, error) {
	return json.Marshal(v)
}

func payloadToStruct(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (w *BillingWorker) handleTimeout(ctx context.Context, t *asynq.Task) error {
	var task BillingTimeoutTask
	if err := payloadToStruct(t.Payload(), &task); err != nil {
		return err
	}
	session := w.billing.BuildBillingSession(task.ChannelID, task.Model, task.APIType, task.IdempotencyKey)
	session.PreAuthorizedAmt = task.AuthAmt
	refund, err := w.billing.Refund(session)
	if err != nil {
		log.Printf("billing timeout refund error: %v", err)
		return err
	}
	log.Printf("billing timeout: session=%s refunded=%.4f", task.SessionID, refund)
	return nil
}

func (w *BillingWorker) handlePolling(ctx context.Context, t *asynq.Task) error {
	var task BillingPollingTask
	if err := payloadToStruct(t.Payload(), &task); err != nil {
		return err
	}
	session := w.billing.BuildBillingSession(task.ChannelID, task.Model, task.APIType, task.IdempotencyKey)
	session.PreAuthorizedAmt = task.PreAuthorizedAmt
	session.AttemptNo = task.AttemptNo
	settled, err := w.billing.PostBill(session, &types.Usage{})
	if err != nil {
		if task.AttemptNo < task.MaxAttempts {
			task.AttemptNo++
			return EnqueueBillingPollingTask(w.redisAddr, &task)
		}
		return err
	}
	log.Printf("billing polling: session=%s settled=%.4f", task.SessionID, settled)
	return nil
}
