# Multi-stage build for Controller service
FROM golang:1.24-alpine AS builder

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

# Build the application
RUN cd cmd/controller && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o controller .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates i2c-tools

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