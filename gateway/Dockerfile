# AegisAI Gateway — Multi-stage Docker build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /aegis-gateway ./cmd/proxy/

# Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 aegis

COPY --from=builder /aegis-gateway /usr/local/bin/aegis-gateway

USER aegis

EXPOSE 8443

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8443/health || exit 1

ENTRYPOINT ["aegis-gateway"]