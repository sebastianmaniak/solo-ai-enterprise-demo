FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /account-summary-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /account-summary-tools /account-summary-tools
EXPOSE 8085
ENTRYPOINT ["/account-summary-tools"]
