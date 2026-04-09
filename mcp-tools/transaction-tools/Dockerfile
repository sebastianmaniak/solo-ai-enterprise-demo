FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY shared/ /app/shared/
COPY transaction-tools/ /app/transaction-tools/
WORKDIR /app/transaction-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o transaction-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/transaction-tools/transaction-tools /transaction-tools
EXPOSE 8083
ENTRYPOINT ["/transaction-tools"]
