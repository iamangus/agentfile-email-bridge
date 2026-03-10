# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agentfile-email-bridge .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/agentfile-email-bridge /usr/local/bin/agentfile-email-bridge

ENTRYPOINT ["agentfile-email-bridge"]
CMD ["--config", "/etc/agentfile-email-bridge/config.yaml"]
