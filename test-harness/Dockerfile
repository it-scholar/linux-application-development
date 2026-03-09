# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o test-harness ./cmd/harness

# Runtime stage
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

# Copy binary from builder
COPY --from=builder /build/test-harness /usr/local/bin/test-harness

# Create directories for reports and data
RUN mkdir -p /app/reports /app/data

# Set environment variables
ENV TEST_HARNESS_DATA_DIR=/app/data
ENV TEST_HARNESS_REPORTS_DIR=/app/reports

# Run as non-root user
RUN adduser -D -u 1000 testuser && \
    chown -R testuser:testuser /app
USER testuser

# Default command
ENTRYPOINT ["test-harness"]
CMD ["--help"]
