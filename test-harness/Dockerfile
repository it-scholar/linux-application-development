# Multi-stage Dockerfile for Test Harness
# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    sqlite-dev \
    make

WORKDIR /build

# Copy Go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy test harness source
COPY . .

# Build
RUN make build

# Stage 2: Runtime
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    sqlite-libs \
    curl \
    jq

# Create non-root user
RUN addgroup -g 1000 weather && \
    adduser -u 1000 -G weather -s /bin/sh -D weather

# Copy binary from builder
COPY --from=builder /build/bin/test-harness /usr/local/bin/test-harness

# Copy test files
COPY --from=builder /build/tests/ /opt/weather-station/tests/

WORKDIR /opt/weather-station

USER weather

# Default command runs CI pipeline against cluster services
ENTRYPOINT ["test-harness"]
CMD ["ci", "--verbose"]
