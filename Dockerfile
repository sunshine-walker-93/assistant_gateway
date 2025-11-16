FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gateway ./cmd/gateway

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/gateway /app/gateway
COPY --from=builder /app/configs /app/configs

ENV GATEWAY_CONFIG=/app/configs/config.docker.yaml

EXPOSE 8080

ENTRYPOINT ["/app/gateway"]


