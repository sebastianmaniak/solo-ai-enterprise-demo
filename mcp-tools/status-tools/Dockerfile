FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY status-tools/ /app/status-tools/
WORKDIR /app/status-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o status-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/status-tools/status-tools /status-tools
EXPOSE 8085
ENTRYPOINT ["/status-tools"]
