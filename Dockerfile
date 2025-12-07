FROM golang:1.24 AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gateway ./cmd/gateway

FROM debian:bookworm-slim

RUN apt-get -o Acquire::Check-Valid-Until=false update || true && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/gateway /app/gateway
COPY --from=builder /app/configs /app/configs

ENV GATEWAY_CONFIG=/app/configs/config.docker.yaml

EXPOSE 8080

ENTRYPOINT ["/app/gateway"]


