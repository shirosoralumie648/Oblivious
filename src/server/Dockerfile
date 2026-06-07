# =============================================================================
# Oblivious Server - Multi-stage Docker Build
# =============================================================================
# Stage 1: Go compilation
# Stage 2: Minimal runtime image
# =============================================================================

# ---------------------------------------------------------------------------
# Stage 1: Build
# ---------------------------------------------------------------------------
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/server ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /build/migrate ./cmd/migrate

# ---------------------------------------------------------------------------
# Stage 2: Runtime
# ---------------------------------------------------------------------------
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/server  /app/server
COPY --from=builder /build/migrate /app/migrate

# Copy migration files
COPY --from=builder /build/migrations /app/migrations

# Copy default config (can be overridden at runtime via env vars)
COPY config/config.yaml /app/config/config.yaml 2>/dev/null || true

RUN chown -R app:app /app
USER app

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/server"]
