# Oblivious API 文档

## 概述

Oblivious API 是一个 RESTful HTTP 服务，使用 JSON 作为数据交换格式。所有业务端点均以 `/api/v1` 为前缀。

完整 API 规范参见 [openapi.yaml](openapi.yaml)（OpenAPI 3.0 格式）。

## 基础信息

| 项目 | 说明 |
|------|------|
| 基础路径 | `/api/v1` |
| 数据格式 | `application/json` |
| 认证方式 | Cookie 会话（登录后自动设置） |
| 响应结构 | 统一信封格式（见下文） |

## 统一响应格式

所有端点返回统一的 JSON 信封结构：

```json
{
  "ok": true,
  "data": { ... },
  "error": null
}
```

成功时 `ok` 为 `true`，`data` 包含业务数据；失败时 `ok` 为 `false`，`error` 包含错误信息：

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "invalid_request",
    "message": "email and password are required"
  }
}
```

## 认证

1. 通过 `/api/v1/auth/register` 或 `/api/v1/auth/login` 提交邮箱和密码
2. 服务端返回会话信息并设置 `session_id` Cookie
3. 后续请求自动携带 Cookie，无需额外操作
4. 通过 `/api/v1/auth/logout` 注销当前会话

## API 端点一览

### 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/metrics` | Prometheus 指标 |

### Auth - 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/login` | 登录 |
| GET | `/api/v1/auth/me` | 获取当前会话 |
| POST | `/api/v1/auth/logout` | 登出 |

### Preferences - 用户偏好

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/app/me/preferences` | 获取偏好设置 |
| PUT | `/api/v1/app/me/preferences` | 更新偏好设置 |

### Chat - 对话

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/app/models` | 获取可用模型列表 |
| GET | `/api/v1/app/conversations` | 获取对话列表 |
| POST | `/api/v1/app/conversations` | 创建新对话 |
| GET | `/api/v1/app/conversations/{id}/messages` | 获取对话消息 |
| POST | `/api/v1/app/conversations/{id}/messages` | 发送消息 |
| GET | `/api/v1/app/conversations/{id}/config` | 获取对话配置 |
| PUT | `/api/v1/app/conversations/{id}/config` | 更新对话配置 |
| POST | `/api/v1/app/conversations/{id}/convert-to-task` | 对话转任务 |

### Knowledge - 知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/app/knowledge-bases` | 获取知识库列表 |
| POST | `/api/v1/app/knowledge-bases` | 创建知识库 |
| GET | `/api/v1/app/knowledge-bases/{id}` | 获取知识库详情 |
| PUT | `/api/v1/app/knowledge-bases/{id}` | 更新知识库 |
| DELETE | `/api/v1/app/knowledge-bases/{id}` | 删除知识库 |
| GET | `/api/v1/app/knowledge-bases/{id}/documents` | 获取文档列表 |
| POST | `/api/v1/app/knowledge-bases/{id}/documents` | 创建文档 |
| PUT | `/api/v1/app/knowledge-bases/{id}/documents/{docId}` | 更新文档 |
| DELETE | `/api/v1/app/knowledge-bases/{id}/documents/{docId}` | 删除文档 |
| POST | `/api/v1/app/knowledge-bases/{id}/retrieve` | 知识检索 |

### Task - 任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/app/tasks` | 获取任务列表 |
| POST | `/api/v1/app/tasks` | 创建任务 |
| GET | `/api/v1/app/tasks/{id}` | 获取任务详情 |
| POST | `/api/v1/app/tasks/{id}/start` | 启动任务 |
| POST | `/api/v1/app/tasks/{id}/approve` | 审批任务 |
| POST | `/api/v1/app/tasks/{id}/pause` | 暂停任务 |
| POST | `/api/v1/app/tasks/{id}/resume` | 恢复任务 |
| POST | `/api/v1/app/tasks/{id}/cancel` | 取消任务 |
| POST | `/api/v1/app/tasks/{id}/budget` | 更新任务预算 |

### Billing / Console - 计费与控制台

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/console/usage` | 使用量概览 |
| GET | `/api/v1/console/access` | 访问权限概览 |
| GET | `/api/v1/console/models` | 模型概览 |
| GET | `/api/v1/console/billing` | 账单概览 |

## 常见错误码

| HTTP 状态码 | 错误码 | 说明 |
|------------|--------|------|
| 400 | `invalid_request` | 请求参数无效 |
| 401 | `unauthorized` | 未认证或会话已过期 |
| 401 | `invalid_credentials` | 邮箱或密码错误 |
| 405 | `method_not_allowed` | 请求方法不被允许 |
| 404 | `not_found` | 路由不存在 |
| 500 | `internal_error` | 服务器内部错误 |

## 本地调试

```bash
# 启动服务
bash scripts/dev.sh

# 健康检查
curl http://localhost:8080/healthz

# 注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"secret"}'

# 登录（后续请求自动携带 Cookie）
curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"secret"}'

# 获取对话列表
curl -b cookies.txt http://localhost:8080/api/v1/app/conversations
```

## OpenAPI 规范

完整的 OpenAPI 3.0 规范文件位于 [openapi.yaml](openapi.yaml)，可用于：

- 生成客户端 SDK
- 导入 Postman / Insomnia 等 API 测试工具
- 自动生成 API 参考文档
