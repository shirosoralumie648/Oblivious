package main

import (
	"context"
	"log"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/billing"
	"oblivious/server/internal/observability"
	"oblivious/server/pkg/events"
)

// InitializeEventBus 初始化事件总线并连接 Agent、Observability 和 Billing
func InitializeEventBus(billingService *billing.Service, obsReporter *observability.MemoryReporter) *events.Bus {
	bus := events.NewBus()

	// 订阅 Observability 事件
	obsConsumer := observability.NewEventConsumer(obsReporter)
	obsConsumer.Subscribe(bus)

	// 订阅 Billing 事件
	billingConsumer := billing.NewAgentEventConsumer(billingService)
	billingConsumer.Subscribe(bus)

	log.Println("[EventBus] Agent events wired: Observability and Billing subscribed")
	return bus
}

// SetupAgentEventPublisher 为 Agent Runner 配置事件发布器
func SetupAgentEventPublisher(runner *agent.Runner, bus *events.Bus) {
	publisher := agent.NewEventPublisher(nil, bus)
	runner.SetEventPublisher(publisher)
	log.Println("[EventBus] Agent event publisher configured")
}

// Example usage:
func Example() {
	ctx := context.Background()

	// 初始化服务
	// billingService := billing.NewService(billingStore)
	// obsReporter := observability.NewMemoryReporter()
	// agentRunner := agent.NewRunner(store, gateway, executor, memory, config)

	// 初始化事件总线
	// bus := InitializeEventBus(billingService, obsReporter)

	// 连接 Agent 到事件总线
	// SetupAgentEventPublisher(agentRunner, bus)

	// 现在 Agent 运行时会自动发布事件，Observability 和 Billing 会消费
	_ = ctx
}
