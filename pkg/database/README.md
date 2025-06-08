# DuckDB Implementation for Lokey RNG Service

## Overview

This document explains the improved DuckDB implementation for the Lokey random number generation service.

## Database Schema

### TRNG Data Table

```sql
CREATE TABLE trng_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hash BLOB NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    consumed BOOLEAN DEFAULT FALSE,
    source VARCHAR(20) DEFAULT 'hardware',
    chunk_size INTEGER DEFAULT 32,
    hash_hex VARCHAR(64) GENERATED ALWAYS AS (ENCODE(hash, 'hex')) STORED
)
```

### Fortuna Data Table

```sql
CREATE TABLE fortuna_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    data BLOB NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    consumed BOOLEAN DEFAULT FALSE,
    chunk_size INTEGER DEFAULT 32,
    amplification_factor INTEGER DEFAULT 4
)
```

### Metadata Table

```sql
CREATE TABLE metadata (
    key VARCHAR(50) PRIMARY KEY,
    value VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

## Optimizations

1. **Columnar Storage**: DuckDB's columnar storage is ideal for our time-series data
2. **Generated Columns**: The `hash_hex` column is automatically generated from the binary hash
3. **Indexes**: Time-based indexing for faster queries
4. **Memory Limits**: Explicit memory configuration for Raspberry Pi Zero environment
5. **Batch Processing**: All operations are designed to work in batches for efficiency

## Hardware vs Software Source Tracking

The database tracks whether random data came from:

- **Hardware**: True random data from the ATECC608A chip
- **Software**: Pseudo-random data from Go's crypto/rand when hardware is unavailable

This distinction is stored in the `source` column and exposed via the API, allowing users to know the quality of randomness they're consuming.

## Performance Considerations

- **Queue Size**: Default queue size is 1000 entries for better performance
- **Retention**: Data is automatically pruned to maintain performance
- **Batching**: All writes are batched for efficiency
- **Memory Usage**: Optimized for low-memory environments

## Testing Mode

The system will automatically fall back to software random generation when:

1. The I2C device is not detected
2. The ATECC608A chip fails to initialize
3. The `FORCE_MOCK_MODE=1` environment variable is set

This allows development and testing on machines without the hardware present.
