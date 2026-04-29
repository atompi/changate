FROM golang:1.26.2 as builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/changate ./cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/changate /app/changate
COPY config/config.yaml /app/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/changate"]
CMD ["server", "--config=config.yaml"]
