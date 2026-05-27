FROM golang:1.26.2 AS builder
WORKDIR /app
COPY . .
RUN GOPROXY='https://goproxy.io,direct' CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/changate ./cmd/server/main.go

FROM alpine:3.23
RUN apk --no-cache add ca-certificates && mkdir -p /app/logs
WORKDIR /app
COPY --from=builder /app/changate /app/changate
COPY configs/config.yaml /app/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/changate"]
CMD ["server", "--config=config.yaml"]
