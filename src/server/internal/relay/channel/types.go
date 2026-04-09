package channel

import "time"

// Channel 渠道配置
type Channel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Provider  string `json:"provider"` // "openai"
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"-"` // 加密存储，不暴露
	Models    []string `json:"models"`

	// 限速
	RPMLimit int `json:"rpm_limit"`
	TPMLimit int `json:"tpm_limit"`

	// 熔断
	CBThreshold int `json:"cb_threshold"`
	CBTimeout   int `json:"cb_timeout"` // 秒

	// 健康检查
	HealthCheckStrategy string `json:"health_check_strategy"` // "models_api" | "realtime_probe" | "disabled"
	ProbeModel          string `json:"probe_model"`
	ProbePrompt         string `json:"probe_prompt"`

	// 路由策略
	Strategy string `json:"strategy"` // "weighted" | "priority" | "cost_aware"
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

// ModelRoute 模型路由
type ModelRoute struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Strategy string `json:"strategy"`
	Channels []RouteChannel `json:"channels"`
}

// RouteChannel 模型-渠道关联
type RouteChannel struct {
	Channel  *Channel `json:"channel"`
	ChannelID string `json:"channel_id"`
	Weight    int    `json:"weight"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
	// 运行时状态（内存）
	Healthy bool `json:"healthy"`
	EstimatedCostPer1K float64 `json:"estimated_cost_per_1k"` // 用于 cost_aware 策略
}

// ChannelStats 运行时状态（内存）
type ChannelStats struct {
	ChannelID string `json:"channel_id"`

	// 熔断状态机
	CBState       string    `json:"cb_state"` // "closed" | "open" | "half_open"
	CBFailures    int       `json:"cb_failures"`
	CBLastFailure time.Time `json:"cb_last_failure"`
	CBProbeCount  int       `json:"cb_probe_count"`  // half_open 状态下已处理的探测请求数
	CBHalfOpenReq int       `json:"cb_half_open_req"` // half_open 状态下已处理的请求数

	// 限流器
	RPMCurrent   int       `json:"rpm_current"`
	TPMCurrent   int       `json:"tpm_current"`
	RPMLastReset time.Time `json:"rpm_last_reset"`
	TPMLastReset time.Time `json:"tpm_last_reset"`

	// 监控
	TotalRequests int64 `json:"total_requests"`
	SuccessCount  int64 `json:"success_count"`
	FailureCount  int64 `json:"failure_count"`
	LatencySumUs  int64 `json:"latency_sum_us"`
	LatencyCount  int64 `json:"latency_count"`

	LastProbeSuccess time.Time `json:"last_probe_success"`
	LastProbeTime    time.Time `json:"last_probe_time"`
}
