FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY shared/ /app/shared/
COPY customer-tools/ /app/customer-tools/
WORKDIR /app/customer-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o customer-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/customer-tools/customer-tools /customer-tools
EXPOSE 8081
ENTRYPOINT ["/customer-tools"]
