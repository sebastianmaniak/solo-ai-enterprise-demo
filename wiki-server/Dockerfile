FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wiki-server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/wiki-server /wiki-server
EXPOSE 8080
ENTRYPOINT ["/wiki-server"]
