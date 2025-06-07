# Multi-stage build for API service
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the module files first to leverage Docker cache
COPY go.mod go.sum ./
COPY cmd/api/go.mod cmd/api/go.sum ./cmd/api/

# Download dependencies
RUN go mod download && cd cmd/api && go mod download

# Copy source code
COPY . .

# Build the application
RUN cd cmd/api && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/cmd/api/api .

# Create directory for database and swagger docs
RUN mkdir -p /data /app/swagger
COPY pkg/api/docs /app/swagger

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