# Gateway API 文档

## 概述

统一网关提供 HTTP 到 gRPC 的转换服务，支持零代码接入新服务和接口。所有配置通过管理 API 动态管理，无需重启服务。

**基础地址**: `http://localhost:8080`

## 管理 API

所有管理 API 路径前缀为 `/admin`。

### Backend 管理

Backend 代表一个下游 gRPC 服务。

#### 1. 列出所有 Backend

**请求**
```
GET /admin/backends
```

**响应**
```json
[
  {
    "name": "account",
    "addr": "127.0.0.1:50051"
  },
  {
    "name": "order",
    "addr": "127.0.0.1:50052"
  }
]
```

**示例**
```bash
curl http://localhost:8080/admin/backends
```

#### 2. 添加/更新 Backend

如果 backend 已存在（根据 name），则更新；否则添加新 backend。

**请求**
```
POST /admin/backends
Content-Type: application/json
```

**请求体**
```json
{
  "name": "order",
  "addr": "127.0.0.1:50052"
}
```

**参数说明**
- `name` (string, 必填): Backend 名称，用于路由配置中引用
- `addr` (string, 必填): gRPC 服务地址，格式为 `host:port`

**响应**
- `204 No Content`: 成功
- `400 Bad Request`: 请求参数错误（name 或 addr 为空）
- `500 Internal Server Error`: 配置保存失败

**示例**
```bash
curl -X POST http://localhost:8080/admin/backends \
  -H "Content-Type: application/json" \
  -d '{
    "name": "order",
    "addr": "127.0.0.1:50052"
  }'
```

#### 3. 更新 Backend 地址

**请求**
```
PUT /admin/backends/{name}
Content-Type: application/json
```

**路径参数**
- `name` (string): Backend 名称

**请求体**
```json
{
  "addr": "127.0.0.1:50053"
}
```

**参数说明**
- `addr` (string, 必填): 新的 gRPC 服务地址

**响应**
- `204 No Content`: 成功
- `400 Bad Request`: 请求参数错误（addr 为空）
- `404 Not Found`: Backend 不存在
- `500 Internal Server Error`: 配置保存失败

**示例**
```bash
curl -X PUT http://localhost:8080/admin/backends/order \
  -H "Content-Type: application/json" \
  -d '{
    "addr": "127.0.0.1:50053"
  }'
```

#### 4. 删除 Backend

**请求**
```
DELETE /admin/backends/{name}
```

**路径参数**
- `name` (string): Backend 名称

**响应**
- `204 No Content`: 成功
- `404 Not Found`: Backend 不存在
- `500 Internal Server Error`: 配置保存失败

**示例**
```bash
curl -X DELETE http://localhost:8080/admin/backends/order
```

### 路由管理

路由定义了 HTTP 请求到 gRPC 方法的映射规则。

#### 1. 列出所有路由

**请求**
```
GET /admin/routes
```

**响应**
```json
[
  {
    "http_method": "POST",
    "http_pattern": "/v1/user/login",
    "backend_name": "account",
    "backend_service": "user.v1.UserService",
    "backend_method": "Login",
    "timeout_ms": 1000
  }
]
```

**参数说明**
- `http_method` (string): HTTP 方法（GET/POST/PUT/DELETE 等）
- `http_pattern` (string): HTTP 路径模式
- `backend_name` (string): 引用的 backend 名称
- `backend_service` (string): gRPC 服务全名，格式为 `package.Service`
- `backend_method` (string): gRPC 方法名
- `timeout_ms` (int): 请求超时时间（毫秒），默认 5000ms
- `request_type` (string, 可选): 请求 proto 类型（仅用于文档）
- `response_type` (string, 可选): 响应 proto 类型（仅用于文档）

**示例**
```bash
curl http://localhost:8080/admin/routes
```

#### 2. 添加/更新路由

如果路由已存在（根据 http_method + http_pattern），则更新；否则添加新路由。

**请求**
```
POST /admin/routes
Content-Type: application/json
```

**请求体**
```json
{
  "http_method": "POST",
  "http_pattern": "/v1/order/create",
  "backend_name": "order",
  "backend_service": "order.v1.OrderService",
  "backend_method": "CreateOrder",
  "timeout_ms": 2000
}
```

**参数说明**
- `http_method` (string, 必填): HTTP 方法
- `http_pattern` (string, 必填): HTTP 路径模式
- `backend_name` (string, 必填): Backend 名称（必须在 backends 中已存在）
- `backend_service` (string, 必填): gRPC 服务全名
- `backend_method` (string, 必填): gRPC 方法名
- `timeout_ms` (int, 可选): 超时时间（毫秒），默认 5000
- `request_type` (string, 可选): 请求类型元数据
- `response_type` (string, 可选): 响应类型元数据

**响应**
- `204 No Content`: 成功，路由立即生效
- `400 Bad Request`: 请求参数错误
- `500 Internal Server Error`: 配置保存失败

**示例**
```bash
curl -X POST http://localhost:8080/admin/routes \
  -H "Content-Type: application/json" \
  -d '{
    "http_method": "POST",
    "http_pattern": "/v1/order/create",
    "backend_name": "order",
    "backend_service": "order.v1.OrderService",
    "backend_method": "CreateOrder",
    "timeout_ms": 2000
  }'
```

#### 3. 删除路由

**请求**
```
DELETE /admin/routes?method={http_method}&pattern={http_pattern}
```

**查询参数**
- `method` (string, 必填): HTTP 方法
- `pattern` (string, 必填): HTTP 路径模式

**响应**
- `204 No Content`: 成功，路由立即失效
- `400 Bad Request`: 缺少必要参数
- `500 Internal Server Error`: 配置保存失败

**示例**
```bash
curl -X DELETE "http://localhost:8080/admin/routes?method=POST&pattern=/v1/order/create"
```

## 公共网关 API

网关会根据配置的路由规则，将 HTTP 请求转发到对应的 gRPC 服务。

### 请求格式

**HTTP 方法**: 根据路由配置（通常为 POST）

**路径**: 根据路由配置的 `http_pattern`

**请求头**
```
Content-Type: application/json
```

**请求体**: JSON 格式，对应 gRPC 方法的请求参数

### 响应格式

**响应头**
```
Content-Type: application/json; charset=utf-8
```

**响应体**: JSON 格式，对应 gRPC 方法的响应

### 示例

假设已配置路由：
```json
{
  "http_method": "POST",
  "http_pattern": "/v1/user/login",
  "backend_name": "account",
  "backend_service": "user.v1.UserService",
  "backend_method": "Login",
  "timeout_ms": 1000
}
```

**请求**
```bash
curl -X POST http://localhost:8080/v1/user/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user@example.com",
    "password": "secret123"
  }'
```

**响应**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "12345"
}
```

### 错误响应

网关会将 gRPC 错误码映射到合适的 HTTP 状态码，并返回 JSON 格式的错误信息：

**400 Bad Request**: 请求参数错误（对应 gRPC `InvalidArgument`）
```json
{
  "error": "password is required"
}
```

**401 Unauthorized**: 未认证（对应 gRPC `Unauthenticated`）
```json
{
  "error": "authentication required"
}
```

**403 Forbidden**: 权限不足（对应 gRPC `PermissionDenied`）
```json
{
  "error": "permission denied"
}
```

**404 Not Found**: 资源不存在（对应 gRPC `NotFound`）
```json
{
  "error": "resource not found"
}
```

**409 Conflict**: 资源冲突（对应 gRPC `AlreadyExists` 或 `Aborted`）
```json
{
  "error": "resource already exists"
}
```

**429 Too Many Requests**: 请求频率超限（对应 gRPC `ResourceExhausted`）
```json
{
  "error": "rate limit exceeded"
}
```

**502 Bad Gateway**: 后端服务错误（对应 gRPC `Unknown` 或其他未映射错误）
```json
{
  "error": "backend error"
}
```

**503 Service Unavailable**: 服务不可用（对应 gRPC `Unavailable`）
```json
{
  "error": "service temporarily unavailable"
}
```

**504 Gateway Timeout**: 请求超时（对应 gRPC `DeadlineExceeded`）
```json
{
  "error": "request timeout"
}
```

**500 Internal Server Error**: 服务器内部错误（对应 gRPC `Internal`）
```json
{
  "error": "internal server error"
}
```

## 其他端点

### Prometheus 指标

**请求**
```
GET /metrics
```

返回 Prometheus 格式的指标数据，包括：
- `gateway_http_requests_total`: HTTP 请求总数
- `gateway_http_request_duration_seconds`: HTTP 请求延迟直方图

**示例**
```bash
curl http://localhost:8080/metrics
```

## 完整接入流程示例

### 1. 添加新服务 Backend

```bash
curl -X POST http://localhost:8080/admin/backends \
  -H "Content-Type: application/json" \
  -d '{
    "name": "payment",
    "addr": "127.0.0.1:50053"
  }'
```

### 2. 添加新接口路由

```bash
curl -X POST http://localhost:8080/admin/routes \
  -H "Content-Type: application/json" \
  -d '{
    "http_method": "POST",
    "http_pattern": "/v1/payment/charge",
    "backend_name": "payment",
    "backend_service": "payment.v1.PaymentService",
    "backend_method": "Charge",
    "timeout_ms": 3000
  }'
```

### 3. 立即使用新接口

```bash
curl -X POST http://localhost:8080/v1/payment/charge \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 100.00,
    "currency": "USD",
    "card_token": "tok_123456"
  }'
```

## 注意事项

1. **零代码接入**: 所有配置通过管理 API 完成，无需修改代码或重启服务
2. **配置持久化**: 所有配置变更会自动保存到配置文件
3. **立即生效**: 路由和 backend 配置变更后立即生效
4. **JSON 转换**: 网关使用 `google.protobuf.Struct` 进行 JSON 和 gRPC 消息的转换
5. **超时控制**: 每个路由可配置独立的超时时间
6. **认证**: 如果设置了 `GATEWAY_API_KEY` 环境变量，管理 API 需要提供 `X-API-Key` 请求头

## 错误码说明

网关会将 gRPC 错误码自动映射到合适的 HTTP 状态码：

| HTTP 状态码 | gRPC 错误码 | 说明 |
|------------|------------|------|
| 200 | - | 成功 |
| 204 | - | 成功（无响应体） |
| 400 | InvalidArgument, OutOfRange | 请求参数错误 |
| 401 | Unauthenticated | 未认证 |
| 403 | PermissionDenied | 权限不足 |
| 404 | NotFound | 资源不存在 |
| 409 | AlreadyExists, Aborted | 资源冲突 |
| 412 | FailedPrecondition | 前置条件失败 |
| 429 | ResourceExhausted | 请求频率超限 |
| 500 | Internal, Unknown | 服务器内部错误 |
| 502 | 其他未映射错误 | 后端服务错误 |
| 503 | Unavailable | 服务不可用 |
| 504 | DeadlineExceeded | 请求超时 |

所有错误响应均为 JSON 格式：
```json
{
  "error": "错误描述信息"
}
```

