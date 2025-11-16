## 镜像名和默认 tag（与 account 项目风格保持一致）
IMAGE_NAME := 93maoshui/assistant-gateway
IMAGE_TAG  ?= latest
COMPOSE_PROJECT_NAME ?= assistant

## 本地构建 Go 二进制（可选）
go-build:
	go build -o bin/gateway ./cmd/gateway

go-run:
	GATEWAY_CONFIG=configs/config.yaml go run ./cmd/gateway

## 本地构建镜像（用于 docker-compose / K8s 部署）
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

## 构建并推送到镜像仓库（例如 Docker Hub）
push: build
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

## 使用时间戳生成 dev tag 并推送
push-dev:
	IMAGE_TAG=dev-$(shell date +%Y%m%d-%H%M%S) $(MAKE) push

## 本地启动 gateway（依赖外部已存在的 assistant-net 网络）
run:
	docker compose -p $(COMPOSE_PROJECT_NAME) up -d --build gateway

## 查看网关日志
logs:
	docker compose -p $(COMPOSE_PROJECT_NAME) logs -f gateway

## 停止容器
stop:
	docker compose -p $(COMPOSE_PROJECT_NAME) stop gateway

## 删除容器（保留卷）
down:
	docker compose -p $(COMPOSE_PROJECT_NAME) down

.PHONY: go-build go-run build push push-dev run logs stop down

