# Oblivious 系统设计文档 - 第四部分

## 6. 可观测性设计

### 6.1 日志系统

#### 6.1.1 日志分级

```go
type LogLevel string

const (
    DEBUG LogLevel = "DEBUG"  // 开发调试
    INFO  LogLevel = "INFO"   // 常规信息
    WARN  LogLevel = "WARN"   // 警告（不影响功能）
    ERROR LogLevel = "ERROR"  // 错误（影响单个请求）
    FATAL LogLevel = "FATAL"  // 致命错误（服务不可用）
)
```

#### 6.1.2 结构化日志

**日志格式（JSON）**
```json
{
  "timestamp": "2026-06-03T10:15:30.123Z",
  "level": "INFO",
  "service": "chat-service",
  "instance": "chat-service-pod-1",
  "trace_id": "abc123def456",
  "span_id": "span789",
  "user_id": "user-uuid",
  "request_id": "req-uuid",
  "method": "POST",
  "path": "/v1/chat/completions",
  "status_code": 200,
  "latency_ms": 1250,
  "message": "Chat completion request processed",
  "metadata": {
    "model": "gpt-4",
    "tokens": 150
  }
}
```

**关键日志场景**
```go
// 1. 请求开始
logger.Info("request started",
    zap.String("request_id", requestID),
    zap.String("method", r.Method),
    zap.String("path", r.URL.Path),
    zap.String("user_id", userID),
)

// 2. 外部调用
logger.Info("calling external LLM API",
    zap.String("provider", "openai"),
    zap.String("model", "gpt-4"),
    zap.Int("prompt_tokens", promptTokens),
)

// 3. 错误处理
logger.Error("LLM API call failed",
    zap.String("provider", "openai"),
    zap.Error(err),
    zap.Int("retry_count", retryCount),
)

// 4. 性能监控
logger.Warn("slow query detected",
    zap.String("query", query),
    zap.Duration("duration", duration),
    zap.Int64("threshold_ms", 1000),
)
```

#### 6.1.3 日志收集与存储

**ELK Stack 架构**
```
应用日志 -> Filebeat -> Kafka -> Logstash -> Elasticsearch -> Kibana
                           ↓
                      长期存储 (S3)
```

**日志保留策略**
```yaml
retention_policy:
  # 热存储（Elasticsearch）
  hot_tier:
    duration: 7_days
    indices:
      - logs-api-*
      - logs-error-*
  
  # 温存储（Elasticsearch）
  warm_tier:
    duration: 30_days
    indices:
      - logs-api-*
  
  # 冷存储（S3）
  cold_tier:
    duration: 365_days
    compression: gzip
  
  # 删除策略
  delete_after: 365_days
```

### 6.2 监控系统

#### 6.2.1 监控指标

**系统级指标（Prometheus）**
```yaml
# CPU
- node_cpu_seconds_total
- process_cpu_seconds_total

# 内存
- node_memory_MemTotal_bytes
- node_memory_MemAvailable_bytes
- process_resident_memory_bytes

# 磁盘
- node_disk_io_time_seconds_total
- node_disk_read_bytes_total
- node_disk_write_bytes_total

# 网络
- node_network_receive_bytes_total
- node_network_transmit_bytes_total
```

**应用级指标（自定义）**
```go
// HTTP 请求
var (
    httpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )
)

// LLM 调用
var (
    llmRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_requests_total",
            Help: "Total number of LLM API requests",
        },
        []string{"provider", "model", "status"},
    )
    
    llmTokensUsed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "llm_tokens_used_total",
            Help: "Total tokens used",
        },
        []string{"provider", "model", "type"}, // type: prompt/completion
    )
    
    llmFirstTokenLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "llm_first_token_latency_seconds",
            Help:    "Time to first token",
            Buckets: []float64{0.5, 1, 2, 3, 5, 10},
        },
        []string{"provider", "model"},
    )
)

// 业务指标
var (
    activeUsers = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_users",
            Help: "Number of currently active users",
        },
    )
    
    workflowExecutions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "workflow_executions_total",
            Help: "Total number of workflow executions",
        },
        []string{"status"}, // status: completed/failed/cancelled
    )
)
```

#### 6.2.2 Grafana 仪表盘

**仪表盘列表**

**1. 系统概览仪表盘**
- 总请求数 / QPS
- 平均响应时间 / P95 / P99
- 错误率
- 活跃用户数
- CPU / 内存 / 磁盘使用率

**2. LLM 性能仪表盘**
- 各提供商调用量分布
- 各模型使用量
- Token 使用统计
- 首字延迟分布
- 成本统计

**3. 数据库仪表盘**
- 连接池使用情况
- 查询 QPS
- 慢查询统计
- 主从延迟
- 缓存命中率

**4. 业务指标仪表盘**
- 用户注册趋势
- 对话会话数
- 工作流执行统计
- 知识库文档数
- 收入统计

#### 6.2.3 告警规则

**Prometheus 告警规则**
```yaml
groups:
  - name: api_alerts
    interval: 30s
    rules:
      # 高错误率告警
      - alert: HighErrorRate
        expr: |
          rate(http_requests_total{status=~"5.."}[5m]) /
          rate(http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }} for {{ $labels.endpoint }}"
      
      # 高延迟告警
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95,
            rate(http_request_duration_seconds_bucket[5m])
          ) > 1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "P95 latency is {{ $value }}s for {{ $labels.endpoint }}"
      
      # 服务不可用告警
      - alert: ServiceDown
        expr: up{job="api-service"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Service is down"
          description: "{{ $labels.instance }} is down"
      
      # 数据库连接池耗尽
      - alert: DatabaseConnectionPoolExhausted
        expr: |
          db_connections_in_use / db_connections_max > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Database connection pool nearly exhausted"
          description: "Connection pool usage is {{ $value | humanizePercentage }}"
      
      # 缓存命中率低
      - alert: LowCacheHitRate
        expr: |
          rate(cache_hits[5m]) /
          (rate(cache_hits[5m]) + rate(cache_misses[5m])) < 0.7
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Low cache hit rate"
          description: "Cache hit rate is {{ $value | humanizePercentage }}"
      
      # LLM API 调用失败率高
      - alert: HighLLMFailureRate
        expr: |
          rate(llm_requests_total{status="error"}[5m]) /
          rate(llm_requests_total[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High LLM API failure rate"
          description: "LLM failure rate is {{ $value | humanizePercentage }} for {{ $labels.provider }}"
```

**告警通知渠道**
```yaml
alerting:
  # Slack 通知
  slack:
    webhook_url: ${SLACK_WEBHOOK_URL}
    channels:
      critical: "#alerts-critical"
      warning: "#alerts-warning"
  
  # 邮件通知
  email:
    smtp_host: smtp.gmail.com
    smtp_port: 587
    from: alerts@oblivious.ai
    to:
      - oncall@oblivious.ai
      - devops@oblivious.ai
  
  # PagerDuty (生产环境)
  pagerduty:
    service_key: ${PAGERDUTY_SERVICE_KEY}
    severity_mapping:
      critical: "critical"
      warning: "warning"
```

### 6.3 分布式追踪

#### 6.3.1 OpenTelemetry 集成

**追踪架构**
```
应用 (OpenTelemetry SDK)
    ↓
OpenTelemetry Collector
    ↓
Jaeger / Tempo (存储)
    ↓
Grafana (可视化)
```

**Span 设计**
```go
func HandleChatRequest(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 1. 创建根 Span
    ctx, span := tracer.Start(ctx, "HandleChatRequest")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("user_id", userID),
        attribute.String("model", model),
    )
    
    // 2. 数据库查询 Span
    ctx, dbSpan := tracer.Start(ctx, "GetConversation")
    conversation, err := db.GetConversation(ctx, conversationID)
    dbSpan.End()
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return
    }
    
    // 3. LLM 调用 Span
    ctx, llmSpan := tracer.Start(ctx, "CallLLM")
    llmSpan.SetAttributes(
        attribute.String("provider", "openai"),
        attribute.String("model", "gpt-4"),
    )
    response, err := llmClient.Chat(ctx, messages)
    llmSpan.SetAttributes(
        attribute.Int("prompt_tokens", response.Usage.PromptTokens),
        attribute.Int("completion_tokens", response.Usage.CompletionTokens),
    )
    llmSpan.End()
    
    // 4. 缓存写入 Span
    ctx, cacheSpan := tracer.Start(ctx, "CacheResponse")
    cache.Set(ctx, cacheKey, response)
    cacheSpan.End()
    
    span.SetStatus(codes.Ok, "success")
}
```

**追踪数据示例**
```
Trace ID: abc123def456
├─ HandleChatRequest (150ms)
   ├─ AuthenticateUser (10ms)
   ├─ GetConversation (20ms)
   │  └─ PostgreSQL Query (18ms)
   ├─ CheckRateLimit (5ms)
   │  └─ Redis GET (3ms)
   ├─ CallLLM (100ms)
   │  ├─ HTTP Request to OpenAI (95ms)
   │  └─ Response Parsing (5ms)
   ├─ SaveMessage (10ms)
   │  └─ PostgreSQL Insert (8ms)
   └─ CacheResponse (5ms)
      └─ Redis SET (3ms)
```

### 6.4 健康检查

#### 6.4.1 健康检查端点

**基础健康检查**
```go
// GET /health
func HealthCheck(w http.ResponseWriter, r *http.Request) {
    health := HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Checks:    make(map[string]CheckResult),
    }
    
    // 数据库检查
    if err := db.Ping(); err != nil {
        health.Status = "unhealthy"
        health.Checks["database"] = CheckResult{
            Status: "down",
            Error:  err.Error(),
        }
    } else {
        health.Checks["database"] = CheckResult{Status: "up"}
    }
    
    // Redis 检查
    if err := redis.Ping(ctx).Err(); err != nil {
        health.Status = "unhealthy"
        health.Checks["redis"] = CheckResult{
            Status: "down",
            Error:  err.Error(),
        }
    } else {
        health.Checks["redis"] = CheckResult{Status: "up"}
    }
    
    // 向量数据库检查
    if err := vectorDB.Health(); err != nil {
        health.Status = "degraded"  // 非关键依赖
        health.Checks["vector_db"] = CheckResult{
            Status: "down",
            Error:  err.Error(),
        }
    } else {
        health.Checks["vector_db"] = CheckResult{Status: "up"}
    }
    
    statusCode := http.StatusOK
    if health.Status == "unhealthy" {
        statusCode = http.StatusServiceUnavailable
    }
    
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(health)
}
```

**就绪检查**
```go
// GET /ready
func ReadinessCheck(w http.ResponseWriter, r *http.Request) {
    // 检查服务是否准备好接收流量
    ready := true
    
    // 检查数据库迁移是否完成
    if !db.IsMigrationComplete() {
        ready = false
    }
    
    // 检查缓存预热是否完成
    if !cache.IsWarmedUp() {
        ready = false
    }
    
    // 检查依赖服务是否可用
    if !dependencyChecker.AllReady() {
        ready = false
    }
    
    if ready {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}
```

**存活检查**
```go
// GET /alive
func LivenessCheck(w http.ResponseWriter, r *http.Request) {
    // 简单检查进程是否存活
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}
```

---

## 7. 部署架构

### 7.1 容器化部署

**Docker Compose (开发/测试环境)**
```yaml
version: '3.8'

services:
  # API 网关
  gateway:
    build: ./gateway
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://postgres:password@postgres:5432/oblivious
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis
  
  # 对话服务
  chat-service:
    build: ./chat-service
    ports:
      - "8082:8082"
    environment:
      - DATABASE_URL=postgresql://postgres:password@postgres:5432/oblivious
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - redis
  
  # 工作流服务
  workflow-service:
    build: ./workflow-service
    ports:
      - "8083:8083"
    environment:
      - DATABASE_URL=postgresql://postgres:password@postgres:5432/oblivious
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - postgres
      - kafka
  
  # PostgreSQL
  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=oblivious
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
  
  # Redis
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
  
  # Qdrant
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
    volumes:
      - qdrant-data:/qdrant/storage
  
  # ClickHouse
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - "8123:8123"
      - "9000:9000"
    volumes:
      - clickhouse-data:/var/lib/clickhouse
  
  # Kafka
  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://kafka:9092
    depends_on:
      - zookeeper
  
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      - ZOOKEEPER_CLIENT_PORT=2181

volumes:
  postgres-data:
  redis-data:
  qdrant-data:
  clickhouse-data:
```

### 7.2 Kubernetes 部署

**部署清单示例**
```yaml
# api-gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
  labels:
    app: api-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-gateway
  template:
    metadata:
      labels:
        app: api-gateway
    spec:
      containers:
      - name: api-gateway
        image: oblivious/api-gateway:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-secret
              key: url
        - name: REDIS_URL
          valueFrom:
            configMapKeyRef:
              name: redis-config
              key: url
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: api-gateway
spec:
  selector:
    app: api-gateway
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

---

## 8. 灾难恢复

### 8.1 备份策略

**数据库备份**
```bash
# 每日全量备份
0 2 * * * pg_dump -h postgres -U postgres oblivious | gzip > /backup/oblivious-$(date +\%Y\%m\%d).sql.gz

# 每小时增量备份（WAL 归档）
pg_basebackup -h postgres -U postgres -D /backup/wal -Fp -Xs -P
```

**备份保留策略**
- 每日备份保留 30 天
- 每周备份保留 3 个月
- 每月备份保留 1 年

### 8.2 恢复流程

**数据库恢复**
```bash
# 1. 停止服务
kubectl scale deployment --replicas=0 --all

# 2. 恢复数据库
gunzip -c /backup/oblivious-20260603.sql.gz | psql -h postgres -U postgres oblivious

# 3. 重启服务
kubectl scale deployment --replicas=3 --all
```

---

## 9. 总结

本文档涵盖了 Oblivious 系统的完整设计方案，包括：

✅ **架构设计**: 微服务架构、服务划分、通信机制  
✅ **数据库设计**: PostgreSQL 表结构、Redis 缓存、向量数据库、ClickHouse 分析  
✅ **API 设计**: RESTful API 规范、核心端点、OpenAI 兼容接口  
✅ **安全设计**: 认证授权、加密、防护措施、审计日志  
✅ **性能设计**: 优化策略、缓存方案、异步处理、负载均衡  
✅ **可观测性**: 日志系统、监控告警、分布式追踪、健康检查  
✅ **部署架构**: Docker Compose、Kubernetes、灾难恢复  

**下一步行动**:
1. 根据本设计文档初始化项目结构
2. 实现核心 API 网关
3. 搭建基础设施（数据库、缓存、消息队列）
4. 逐步实现各个微服务
5. 集成监控与告警系统
6. 编写自动化测试
7. 生产环境部署

---

**文档版本**: v1.0  
**最后更新**: 2026-06-03  
**维护团队**: System Architecture Team
