# Multi-stage build for Controller service
FROM golang:1.24-bullseye AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    gcc g++ libc6-dev pkg-config linux-libc-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the module files first to leverage Docker cache
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY cmd/controller/go.mod cmd/controller/go.sum* ./cmd/controller/

# Create an empty go.mod in the root to make the build work
RUN echo "module github.com/lokey/rng-service\ngo 1.24" > go.mod

# Download dependencies for pkg module
RUN cd pkg && go mod tidy || true

# Download dependencies for controller module
RUN cd cmd/controller && go mod tidy || true

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN cd cmd/controller && CGO_ENABLED=1 GOOS=linux go build -a -o controller .

# Final stage
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    i2c-tools \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/cmd/controller/controller .

# Create directory for database
RUN mkdir -p /data

# Set environment variables
ENV PORT=8081 \
    I2C_BUS_NUMBER=1 \
    DB_PATH="/data/trng.db" \
    HASH_INTERVAL_MS=1000 \
    TRNG_QUEUE_SIZE=100

# Expose port
EXPOSE 8081

# Run the service
CMD ["./controller"]