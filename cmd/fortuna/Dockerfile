# Multi-stage build for Fortuna service
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the module files first to leverage Docker cache
COPY go.mod go.sum ./
COPY cmd/fortuna/go.mod cmd/fortuna/go.sum ./cmd/fortuna/

# Download dependencies
RUN go mod download && cd cmd/fortuna && go mod download

# Copy source code
COPY . .

# Build the application
RUN cd cmd/fortuna && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fortuna .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

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
