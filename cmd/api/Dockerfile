
# Multi-stage build for API service
FROM golang:1.24-bullseye AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc g++ libc6-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy ALL go.mod and go.sum files first
COPY go.mod go.sum ./
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY cmd/api/go.mod cmd/api/go.sum ./cmd/api/

# Download dependencies for pkg module first
RUN cd pkg && go mod download

# Download dependencies for api
RUN cd cmd/api && go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN cd cmd/api && CGO_ENABLED=1 GOOS=linux go build -a -o api .

# Final stage
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/cmd/api/api .

# Create directories
RUN mkdir -p /data /app/swagger

# Set environment variables
ENV PORT=8080 \
    DB_PATH="/data/api.db" \
    CONTROLLER_ADDR="http://controller:8081" \
    FORTUNA_ADDR="http://fortuna:8082" \
    TRNG_QUEUE_SIZE=100 \
    FORTUNA_QUEUE_SIZE=100

# Expose port
EXPOSE 8080

# Run the service
CMD ["./api"]