# Oblivious 系统设计文档 - 第三部分

## 4. 安全设计

### 4.1 认证与授权

#### 4.1.1 认证机制

**JWT Token 结构**
```json
{
  "header": {
    "alg": "HS256",
    "typ": "JWT"
  },
  "payload": {
    "sub": "user_id",
    "email": "user@example.com",
    "role": "user",
    "scopes": ["chat", "workflow", "rag"],
    "iat": 1709481600,
    "exp": 1709485200,
    "jti": "token_id"
  }
}
```

**Token 类型**
- **Access Token**: 有效期 1 小时，用于 API 访问
- **Refresh Token**: 有效期 7 天，用于刷新 Access Token
- **API Key**: 长期有效，用于服务间调用

**Token 刷新流程**
```
1. 客户端检测 Access Token 即将过期 (< 5分钟)
2. 使用 Refresh Token 请求新的 Access Token
3. 服务端验证 Refresh Token 有效性
4. 颁发新的 Access Token 和 Refresh Token
5. 旧 Refresh Token 加入黑名单
```

#### 4.1.2 授权模型 (RBAC)

**角色定义**
```yaml
roles:
  - name: admin
    permissions:
      - "*"  # 全部权限
  
  - name: user
    permissions:
      - "chat:*"
      - "workflow:read"
      - "workflow:create"
      - "workflow:update:own"
      - "workflow:delete:own"
      - "kb:*:own"
      - "api_key:*:own"
  
  - name: viewer
    permissions:
      - "chat:read"
      - "workflow:read"
      - "kb:read"
  
  - name: guest
    permissions:
      - "chat:create:limited"  # 限制次数
```

**权限检查流程**
```go
func CheckPermission(user User, resource string, action string, owner string) bool {
    // 1. 检查用户角色
    role := user.Role
    
    // 2. 获取角色权限
    permissions := GetRolePermissions(role)
    
    // 3. 检查通配符权限
    if HasPermission(permissions, "*") {
        return true
    }
    
    // 4. 检查资源级权限
    permission := fmt.Sprintf("%s:%s", resource, action)
    if HasPermission(permissions, permission) {
        return true
    }
    
    // 5. 检查所有权限限
    if action == "own" && user.ID == owner {
        permission := fmt.Sprintf("%s:%s:own", resource, action)
        return HasPermission(permissions, permission)
    }
    
    return false
}
```

### 4.2 数据加密

#### 4.2.1 传输加密
- **协议**: TLS 1.3
- **证书**: Let's Encrypt 自动续期
- **强制 HTTPS**: 所有 HTTP 请求重定向到 HTTPS

#### 4.2.2 存储加密

**敏感字段加密**
```go
// 使用 AES-256-GCM 加密
type EncryptedField struct {
    Ciphertext string  // Base64 编码的密文
    Nonce      string  // Base64 编码的随机数
    Tag        string  // Base64 编码的认证标签
}

// 需要加密的字段
- users.password_hash (bcrypt, cost=12)
- api_keys.key_hash (SHA-256)
- users.metadata (可能包含 PII)
- messages.content (可选，企业版功能)
```

**密钥管理**
```yaml
encryption_keys:
  # 主密钥（环境变量）
  master_key: ${MASTER_ENCRYPTION_KEY}
  
  # 数据加密密钥（DEK）
  dek_rotation_days: 90
  
  # 密钥层次
  # KEK (Key Encryption Key) -> DEK (Data Encryption Key) -> Data
```

### 4.3 安全防护

#### 4.3.1 速率限制

**分层限流**
```yaml
rate_limits:
  # IP 级别
  ip_level:
    requests_per_minute: 100
    requests_per_hour: 1000
  
  # 用户级别
  user_level:
    free_tier:
      requests_per_minute: 10
      tokens_per_minute: 10000
    
    pro_tier:
      requests_per_minute: 60
      tokens_per_minute: 100000
    
    enterprise_tier:
      requests_per_minute: 600
      tokens_per_minute: 1000000
  
  # API Key 级别
  api_key_level:
    custom: true  # 每个 API Key 可自定义限制
```

**限流算法**
- **Token Bucket**: 用于突发流量控制
- **Sliding Window**: 用于精确的时间窗口控制
- **Leaky Bucket**: 用于平滑流量

#### 4.3.2 防护措施

**SQL 注入防护**
```go
// 使用参数化查询（GORM ORM）
db.Where("email = ?", userInput).First(&user)  // ✅ 安全

// 避免拼接 SQL
db.Raw("SELECT * FROM users WHERE email = '" + userInput + "'")  // ❌ 危险
```

**XSS 防护**
```go
// 前端渲染时转义 HTML
import "html/template"

template.HTMLEscapeString(userInput)  // ✅ 安全

// 设置 CSP 头
Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.example.com
```

**CSRF 防护**
```go
// 使用 CSRF Token
// 1. 生成 Token 并存储在 Session
csrfToken := GenerateRandomToken()
session.Set("csrf_token", csrfToken)

// 2. 前端提交时携带 Token
// X-CSRF-Token: <token>

// 3. 服务端验证 Token
if r.Header.Get("X-CSRF-Token") != session.Get("csrf_token") {
    return errors.New("invalid CSRF token")
}
```

**DDoS 防护**
```yaml
ddos_protection:
  # Nginx 层
  nginx:
    limit_req_zone: "$binary_remote_addr zone=api:10m rate=10r/s"
    limit_conn_zone: "$binary_remote_addr zone=addr:10m"
    limit_conn: "addr 10"
  
  # 应用层
  app:
    slowloris_timeout: 30s
    max_request_body_size: 10MB
    max_concurrent_connections: 10000
  
  # CloudFlare (可选)
  cloudflare:
    ddos_protection: true
    rate_limiting: true
    bot_management: true
```

### 4.4 审计日志

**审计事件**
```go
type AuditLog struct {
    ID          uuid.UUID `json:"id"`
    Timestamp   time.Time `json:"timestamp"`
    UserID      uuid.UUID `json:"user_id"`
    Action      string    `json:"action"`      // login, logout, create, update, delete
    Resource    string    `json:"resource"`    // user, api_key, workflow, etc.
    ResourceID  uuid.UUID `json:"resource_id"`
    IPAddress   string    `json:"ip_address"`
    UserAgent   string    `json:"user_agent"`
    RequestID   string    `json:"request_id"`
    Status      string    `json:"status"`      // success, failed
    ErrorMsg    string    `json:"error_msg,omitempty"`
    Metadata    json.RawMessage `json:"metadata,omitempty"`
}
```

**敏感操作审计**
- 用户登录/登出
- 权限变更
- API Key 创建/删除
- 敏感数据访问
- 系统配置修改
- 批量操作

---

## 5. 性能设计

### 5.1 性能目标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| API 响应时间 (P95) | < 200ms | 不包含 LLM 调用时间 |
| API 响应时间 (P99) | < 500ms | 不包含 LLM 调用时间 |
| LLM 首字延迟 (P95) | < 2s | 流式响应第一个 token |
| 并发用户数 | 10,000+ | 单节点支持 |
| 每秒请求数 (QPS) | 1,000+ | 单节点支持 |
| 数据库连接池 | 100 | 每个服务 |
| 缓存命中率 | > 80% | Redis 缓存 |
| 可用性 | 99.9% | 年停机时间 < 8.76 小时 |

### 5.2 性能优化策略

#### 5.2.1 数据库优化

**连接池配置**
```go
db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
sqlDB, _ := db.DB()

// 最大连接数
sqlDB.SetMaxOpenConns(100)

// 最大空闲连接数
sqlDB.SetMaxIdleConns(10)

// 连接最大生存时间
sqlDB.SetConnMaxLifetime(time.Hour)

// 连接最大空闲时间
sqlDB.SetConnMaxIdleTime(10 * time.Minute)
```

**索引优化**
```sql
-- 复合索引（最左前缀原则）
CREATE INDEX idx_billing_user_time ON billing_records(user_id, created_at DESC);

-- 部分索引（减少索引大小）
CREATE INDEX idx_active_users ON users(id) WHERE status = 'active';

-- 表达式索引
CREATE INDEX idx_user_email_lower ON users(LOWER(email));
```

**查询优化**
```go
// ❌ N+1 查询问题
users := []User{}
db.Find(&users)
for _, user := range users {
    db.Model(&user).Association("ApiKeys").Find(&user.ApiKeys)  // N 次查询
}

// ✅ 预加载关联数据
users := []User{}
db.Preload("ApiKeys").Find(&users)  // 2 次查询
```

**分页优化**
```go
// ❌ OFFSET 分页（大偏移量性能差）
db.Offset(10000).Limit(20).Find(&users)

// ✅ 游标分页（使用 ID 或时间戳）
db.Where("id > ?", lastID).Limit(20).Order("id ASC").Find(&users)
```

#### 5.2.2 缓存优化

**缓存模式**

**1. Cache Aside (旁路缓存)**
```go
func GetUser(userID string) (*User, error) {
    // 1. 尝试从缓存获取
    cacheKey := fmt.Sprintf("user:%s", userID)
    if val, err := redis.Get(ctx, cacheKey).Result(); err == nil {
        var user User
        json.Unmarshal([]byte(val), &user)
        return &user, nil
    }
    
    // 2. 缓存未命中，从数据库获取
    var user User
    if err := db.First(&user, "id = ?", userID).Error; err != nil {
        return nil, err
    }
    
    // 3. 写入缓存
    userJSON, _ := json.Marshal(user)
    redis.Set(ctx, cacheKey, userJSON, time.Hour)
    
    return &user, nil
}
```

**2. Write Through (写穿)**
```go
func UpdateUser(user *User) error {
    // 1. 先更新数据库
    if err := db.Save(user).Error; err != nil {
        return err
    }
    
    // 2. 更新缓存
    cacheKey := fmt.Sprintf("user:%s", user.ID)
    userJSON, _ := json.Marshal(user)
    redis.Set(ctx, cacheKey, userJSON, time.Hour)
    
    return nil
}
```

**3. 缓存预热**
```go
func WarmUpCache() {
    // 预热热点数据
    // 1. 活跃用户
    activeUsers := []User{}
    db.Where("last_login_at > ?", time.Now().Add(-24*time.Hour)).Find(&activeUsers)
    
    for _, user := range activeUsers {
        cacheKey := fmt.Sprintf("user:%s", user.ID)
        userJSON, _ := json.Marshal(user)
        redis.Set(ctx, cacheKey, userJSON, time.Hour)
    }
    
    // 2. 系统配置
    // 3. 常用模型列表
}
```

**语义缓存**
```go
func SemanticCache(prompt string, model string) (string, bool) {
    // 1. 计算 prompt 的 embedding
    embedding := GetEmbedding(prompt)
    
    // 2. 在向量数据库中搜索相似 prompt
    results := vectorDB.Search(embedding, topK=1, minScore=0.95)
    
    // 3. 如果找到高度相似的缓存，直接返回
    if len(results) > 0 && results[0].Score > 0.95 {
        cacheKey := results[0].ID
        cachedResponse := redis.Get(ctx, cacheKey).Val()
        return cachedResponse, true
    }
    
    return "", false
}
```

#### 5.2.3 异步处理

**消息队列架构**
```
Producer (API) -> Kafka Topic -> Consumer (Worker)
                                      ↓
                                 Processing
                                      ↓
                                   Database
```

**异步任务场景**
- 文档解析与向量化
- 批量数据导入
- 定时报表生成
- 邮件/通知发送
- 日志聚合与分析

**Kafka Topic 设计**
```yaml
topics:
  - name: document-processing
    partitions: 10
    replication_factor: 3
    retention_ms: 604800000  # 7 天
  
  - name: billing-events
    partitions: 5
    replication_factor: 3
    retention_ms: 2592000000  # 30 天
  
  - name: audit-logs
    partitions: 20
    replication_factor: 3
    retention_ms: 7776000000  # 90 天
```

#### 5.2.4 负载均衡

**多层负载均衡**
```
DNS 轮询 (地理位置)
    ↓
CloudFlare / CDN
    ↓
Nginx (L7 负载均衡)
    ↓
应用服务器集群 (gRPC 客户端负载均衡)
    ↓
数据库读写分离 (主从复制)
```

**Nginx 配置**
```nginx
upstream api_backend {
    least_conn;  # 最少连接算法
    
    server api1.internal:8080 weight=3 max_fails=3 fail_timeout=30s;
    server api2.internal:8080 weight=3 max_fails=3 fail_timeout=30s;
    server api3.internal:8080 weight=2 max_fails=3 fail_timeout=30s;
    
    keepalive 100;  # 保持连接
}

server {
    listen 443 ssl http2;
    server_name api.oblivious.ai;
    
    location /v1/ {
        proxy_pass http://api_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # 超时设置
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

### 5.3 性能监控

**关键指标**
```go
type PerformanceMetrics struct {
    // 请求指标
    RequestsPerSecond float64
    AverageLatency    time.Duration
    P95Latency        time.Duration
    P99Latency        time.Duration
    ErrorRate         float64
    
    // 资源指标
    CPUUsage          float64  // %
    MemoryUsage       float64  // %
    DiskIOPS          int64
    NetworkBandwidth  int64    // bytes/s
    
    // 数据库指标
    DBConnections     int
    DBQueryTime       time.Duration
    DBSlowQueries     int64
    
    // 缓存指标
    CacheHitRate      float64  // %
    CacheMissRate     float64  // %
    
    // 业务指标
    ActiveUsers       int64
    TotalTokens       int64
    TotalCost         float64
}
```

继续第四部分...
