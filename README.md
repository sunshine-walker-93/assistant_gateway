# Assistant Gateway

统一网关服务，提供 HTTP 到 gRPC 的转换能力，支持零代码接入新服务和接口。

## 特性

- ✅ **零代码接入**: 通过管理 API 配置即可接入新服务，无需修改代码
- ✅ **配置热重载**: 配置变更立即生效，无需重启服务
- ✅ **动态路由**: 支持运行时添加、更新、删除路由
- ✅ **统一转换**: 自动处理 HTTP JSON 到 gRPC 的转换
- ✅ **监控指标**: 集成 Prometheus 指标
- ✅ **请求限流**: 内置 IP 基础限流
- ✅ **结构化日志**: 使用 zap 进行日志记录

## 快速开始

### 构建

```bash
# 本地构建
make go-build

# 或直接运行
make go-run
```

### 配置

编辑 `configs/config.yaml`:

```yaml
http:
  listen_address: ":8080"

backends:
  - name: "account"
    addr: "127.0.0.1:50051"

routes:
  - http_method: "POST"
    http_pattern: "/v1/user/login"
    backend_name: "account"
    backend_service: "user.v1.UserService"
    backend_method: "Login"
    timeout_ms: 1000
```

### 运行

```bash
# 使用默认配置
go run ./cmd/gateway

# 或指定配置文件
GATEWAY_CONFIG=configs/config.yaml go run ./cmd/gateway
```

## Docker 部署

### 构建镜像

```bash
make build
```

### 使用 docker-compose

```bash
make run
```

### 查看日志

```bash
make logs
```

## 接入新服务

### 方式一：通过管理 API（推荐）

**1. 添加 Backend**
```bash
curl -X POST http://localhost:8080/admin/backends \
  -H "Content-Type: application/json" \
  -d '{
    "name": "order",
    "addr": "127.0.0.1:50052"
  }'
```

**2. 添加路由**
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

配置立即生效，无需重启！

### 方式二：修改配置文件

编辑 `configs/config.yaml`，添加 backend 和 route 配置，然后重启服务。

## API 文档

详细 API 文档请参考 [API.md](./API.md)

## 管理 API 概览

### Backend 管理
- `GET /admin/backends` - 列出所有 backend
- `POST /admin/backends` - 添加/更新 backend
- `PUT /admin/backends/{name}` - 更新 backend 地址
- `DELETE /admin/backends/{name}` - 删除 backend

### 路由管理
- `GET /admin/routes` - 列出所有路由
- `POST /admin/routes` - 添加/更新路由
- `DELETE /admin/routes` - 删除路由

## 配置说明

### Backend 配置

```yaml
backends:
  - name: "service_name"      # Backend 名称，用于路由引用
    addr: "127.0.0.1:50051"    # gRPC 服务地址
```

### 路由配置

```yaml
routes:
  - http_method: "POST"                    # HTTP 方法
    http_pattern: "/v1/api/endpoint"       # HTTP 路径
    backend_name: "service_name"           # 引用的 backend 名称
    backend_service: "package.Service"    # gRPC 服务全名
    backend_method: "MethodName"           # gRPC 方法名
    timeout_ms: 1000                       # 超时时间（毫秒）
    request_type: "package.Request"        # 可选：请求类型元数据
    response_type: "package.Response"      # 可选：响应类型元数据
```

## 环境变量

- `GATEWAY_CONFIG`: 配置文件路径（默认: `configs/config.yaml`）
- `GATEWAY_API_KEY`: 管理 API 的 API Key（可选，设置后需要提供 `X-API-Key` 请求头）

## 监控

### Prometheus 指标

访问 `http://localhost:8080/metrics` 获取 Prometheus 格式的指标：

- `gateway_http_requests_total`: HTTP 请求总数（按 method, path, status 分组）
- `gateway_http_request_duration_seconds`: HTTP 请求延迟直方图

## 架构说明

网关使用 `google.protobuf.Struct` 进行 JSON 和 gRPC 消息的自动转换，因此：

- ✅ 无需导入 proto 包
- ✅ 支持任意 gRPC 服务
- ✅ 配置即可接入

## 开发

### 项目结构

```
.
├── cmd/gateway/          # 主程序入口
├── internal/
│   ├── config/          # 配置管理
│   ├── grpcclient/      # gRPC 客户端
│   ├── http/            # HTTP 路由和处理器
│   └── middleware/      # 中间件（认证、日志、限流等）
├── configs/             # 配置文件
└── Makefile            # 构建脚本
```

### 构建命令

```bash
make go-build    # 本地构建
make go-run      # 本地运行
make build       # 构建 Docker 镜像
make push        # 推送镜像
make run         # 使用 docker-compose 运行
```

## License

MIT

