# Stage 1: Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o telemetry-streamer \
    ./cmd/telemetry-streamer

# Stage 2: Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
# RUN addgroup -D telemetry && adduser -D -G telemetry telemetry
RUN addgroup -S telemetry && adduser -S -G telemetry -D telemetry

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/telemetry-streamer .

# Create data directory for CSV files
RUN mkdir -p /data && chown -R telemetry:telemetry /data

# Change to non-root user
USER telemetry

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Expose health check port
EXPOSE 8080

# Set environment variables with defaults
ENV LOG_LEVEL=info \
    READ_SPEED=100 \
    BATCH_SIZE=50 \
    BATCH_TIMEOUT_MS=1000 \
    MAX_RETRIES=5 \
    RETRY_BACKOFF_MS=100 \
    CIRCUIT_BREAKER_THRESHOLD=10 \
    GRACEFUL_SHUTDOWN_TIMEOUT=30 \
    HEALTH_CHECK_INTERVAL=30

# Run the application
ENTRYPOINT ["./telemetry-streamer"]
