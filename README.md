# Lokey: True Random Number Generation Service

<img src="docs/logo.jpeg" alt="Description" width="300"/>

<!-- TOC -->
* [Lokey: True Random Number Generation Service](#lokey-true-random-number-generation-service)
  * [Project Overview](#project-overview)
  * [System Components](#system-components)
  * [Architecture](#architecture)
  * [Features](#features)
  * [API Endpoints](#api-endpoints)
    * [Configuration](#configuration)
    * [Data Retrieval](#data-retrieval)
    * [Status](#status)
  * [Getting Started](#getting-started)
    * [Prerequisites](#prerequisites)
    * [Running the System](#running-the-system)
  * [Hardware](#hardware)
  * [Development](#development)
    * [Using Taskfile](#using-taskfile)
    * [Building from Source Manually](#building-from-source-manually)
    * [Testing the API](#testing-the-api)
  * [Project Goals](#project-goals)
  * [License](#license)
<!-- TOC -->


## Project Overview

Lokey is a high-availability, high-bandwidth true random number generation service. The name derives from Loki, the Norse god of chaos, reflecting the unpredictable nature of true randomness, and "key" indicating its accessibility and utility as a keystone service for cryptographic applications.

This project aims to provide affordable and accessible true randomness through off-the-shelf components with a Go implementation. The first prototype uses a Raspberry Pi Zero 2W and an ATECC608A cryptographic chip, creating a hardware-based solution with a modest bill of materials costing approximately €50.

## System Components

1. **Controller**: Interfaces with the ATECC608A chip to harvest true random numbers (TRNG) and process SHA-256 hashes
2. **Fortuna Processor**: Amplifies the entropy using the Fortuna algorithm for enhanced randomness
3. **API Server**: Provides endpoints for configuration and both raw TRNG and Fortuna-amplified data retrieval
4. **DuckDB**: Stores and manages the queue-based data storage system

## Architecture

The system uses a microservices architecture with four containerized applications:

- **Controller Service**: Generates random data using the ATECC608A hardware TRNG
- **Fortuna Service**: Amplifies random data using the Fortuna algorithm
- **API Service**: Provides a unified API for configuration and data retrieval
- **DuckDB Service**: Manages the queue-based storage of random data

Each service operates independently and communicates via HTTP APIs, with shared volumes for database access.

## Features

- Hardware-based True Random Number Generation (TRNG) using ATECC608A cryptographic chip
- High availability and bandwidth of true randomness
- Dual access to both raw TRNG and Fortuna-amplified randomness
- Cryptographic amplification using the Fortuna algorithm for enhanced entropy
- Queue-based storage with configurable sizes for efficient data management
- Multiple data format options (int8, int16, int32, int64, uint8, uint16, uint32, uint64, binary)
- Configurable consumption behavior (delete-on-read)
- Comprehensive health monitoring
- Swagger documentation for all API endpoints

## API Endpoints

### Configuration

- `GET /api/v1/config/queue` - Get queue configuration
- `PUT /api/v1/config/queue` - Update queue configuration
- `GET /api/v1/config/consumption` - Get consumption behavior configuration
- `PUT /api/v1/config/consumption` - Update consumption behavior configuration

### Data Retrieval

- `POST /api/v1/data` - Retrieve random data in specified format

### Status

- `GET /api/v1/status` - Get system status and queue information
- `GET /api/v1/health` - Check health of all system components

## Getting Started

### Prerequisites

- Docker and Docker Compose
- ATECC608A connected via I2C
- Go 1.24+ (for development)

### Running the System

1. Clone the repository
2. Configure I2C settings in `docker-compose.yaml`
3. Start the services:

```bash
docker-compose up -d
```

4. Access the API at http://localhost:8080
5. View API documentation at http://localhost:8080/swagger/index.html

### Running Without Hardware (Testing Mode)

You can run the system without the ATECC608A hardware using mock mode:

```bash
# Using Taskfile (recommended)
task run-mock

# Or using Docker Compose
task docker-up-mock
```

The system will also automatically detect if hardware is unavailable and fall back to software mode:

```bash
# Auto-detection (will use hardware if available, otherwise mock mode)
task run-local
```

In mock mode:
- The system uses Go's crypto/rand for software random generation
- All features work normally, with data marked as "software" source
- The API includes source information so you know which data is hardware vs software generated

## Hardware

Lokey uses minimal hardware to achieve its goals:

- **Raspberry Pi Zero 2W**: Serves as the main computing platform
- **ATECC608A**: Cryptographic chip providing true random number generation capabilities

The total bill of materials is approximately €50, making this a cost-effective solution for organizations requiring true randomness for cryptographic applications, simulations, or other purposes requiring high-quality random data.

## Development

### Using Taskfile

Lokey uses [Task](https://taskfile.dev/) as a convenient build tool. Install Task following the instructions at [taskfile.dev](https://taskfile.dev/installation/), then use these commands:

```bash
# Display all available tasks
task

# Build all components
task build

# Build specific components
task build-controller
task build-api
task build-fortuna

# Run tests
task test

# Tidy Go modules
task tidy

# Format code
task fmt

# Build Docker images
task docker-build

# Start all services
task docker-up

# Start all services in mock mode (no hardware)
task docker-up-mock

# Stop all services
task docker-down

# Run locally with hardware detection
task run-local

# Run locally in mock mode
task run-mock
```

### Building from Source Manually

```bash
# Build the controller
cd cmd/controller
go build

# Build the Fortuna processor
cd ../fortuna
go build

# Build the API server
cd ../api
go build
```

### Testing the API

```bash
# Get queue configuration
curl -X GET http://localhost:8080/api/v1/config/queue

# Retrieve random data in int32 format
curl -X POST http://localhost:8080/api/v1/data \
  -H "Content-Type: application/json" \
  -d '{"format":"int32","chunk_size":32,"limit":10,"offset":0,"source":"fortuna"}'

# Check system health
curl -X GET http://localhost:8080/api/v1/health
```

## Project Goals

Lokey aims to democratize access to true randomness with these key objectives:

1. **Accessibility**: Provide true randomness through affordable, off-the-shelf components
2. **High Availability**: Ensure the service is reliable and continuously available
3. **High Bandwidth**: Deliver sufficient random data throughput for demanding applications
4. **Flexibility**: Offer both raw TRNG and cryptographically amplified random data
5. **Extensibility**: Build a foundation that can be expanded with additional entropy sources

Future versions may incorporate additional entropy sources or higher-performance hardware, while maintaining the core design principles of accessibility and reliability.

## License

MIT
