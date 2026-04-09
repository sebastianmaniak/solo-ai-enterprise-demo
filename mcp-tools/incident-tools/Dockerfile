FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY incident-tools/ /app/incident-tools/
WORKDIR /app/incident-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o incident-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/incident-tools/incident-tools /incident-tools
EXPOSE 8086
ENTRYPOINT ["/incident-tools"]
