FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY shared/ /app/shared/
COPY policy-tools/ /app/policy-tools/
WORKDIR /app/policy-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o policy-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/policy-tools/policy-tools /policy-tools
EXPOSE 8082
ENTRYPOINT ["/policy-tools"]
