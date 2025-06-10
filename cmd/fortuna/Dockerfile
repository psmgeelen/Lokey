# Multi-stage build for Fortuna service
FROM golang:1.24-bullseye AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc g++ libc6-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy ALL go.mod and go.sum files first
COPY go.mod go.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY cmd/fortuna/go.mod cmd/fortuna/go.sum ./cmd/fortuna/

# Download dependencies for pkg module first
RUN cd pkg && go mod download

# Download dependencies for fortuna
RUN cd cmd/fortuna && go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN cd cmd/fortuna && CGO_ENABLED=1 GOOS=linux go build -a -o fortuna .

# Final stage
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/cmd/fortuna/fortuna .

# Create directory for database
RUN mkdir -p /data

# Set environment variables
ENV PORT=8082 \
    DB_PATH="/data/fortuna.db" \
    CONTROLLER_URL="http://controller:8081" \
    PROCESS_INTERVAL_MS=5000 \
    FORTUNA_QUEUE_SIZE=100 \
    AMPLIFICATION_FACTOR=4 \
    SEED_COUNT=3

# Expose port
EXPOSE 8082

# Run the service
CMD ["./fortuna"]