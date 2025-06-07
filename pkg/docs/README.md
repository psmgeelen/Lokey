# DuckDB in Microservices

## Overview

This document explains how DuckDB is used in the Lokey microservices architecture.

## Important Notes

### DuckDB as an Embedded Database

Unlike traditional client-server databases (PostgreSQL, MySQL, etc.), DuckDB is an **embedded database**, similar to SQLite. This means:

1. Each service includes its own instance of DuckDB
2. There's no separate DuckDB server process
3. DuckDB files are accessed directly by the application

### Service Data Access

Each microservice (Controller, Fortuna, API) accesses its own DuckDB database file:

- **Controller**: Uses `/data/trng.db`
- **Fortuna**: Uses `/data/fortuna.db`
- **API**: Uses `/data/api.db`

### Docker Volume Usage

Docker volumes are used to persist the database files across container restarts. The volumes are:

- `trng-data`: Stores the Controller's data
- `fortuna-data`: Stores the Fortuna service's data
- `api-data`: Stores the API service's data

## Alternative Approaches

If a shared database is required, consider:

1. PostgreSQL for relational data
2. Redis for queue-based data storage
3. MongoDB for document-based storage

These alternatives offer better solutions for true client-server database needs.

## Performance Considerations

While DuckDB provides excellent analytical query performance, be aware of potential concurrency limitations in high-throughput scenarios.
