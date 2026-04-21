FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY hybrid-infra-tools/ /app/hybrid-infra-tools/
WORKDIR /app/hybrid-infra-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o hybrid-infra-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/hybrid-infra-tools/hybrid-infra-tools /hybrid-infra-tools
EXPOSE 8087
ENTRYPOINT ["/hybrid-infra-tools"]
