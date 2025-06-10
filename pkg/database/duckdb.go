package database

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"sync"

	_ "github.com/marcboeker/go-duckdb"
)

type DuckDBHandler struct {
	db               *sql.DB
	trngQueueSize    int
	fortunaQueueSize int
	mutex            sync.Mutex
}

// NewDuckDBHandler creates a new DuckDB database handler
func NewDuckDBHandler(dbPath string, trngQueueSize, fortunaQueueSize int) (*DuckDBHandler, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	handler := &DuckDBHandler{
		db:               db,
		trngQueueSize:    trngQueueSize,
		fortunaQueueSize: fortunaQueueSize,
		mutex:            sync.Mutex{},
	}

	err = handler.setupTables()
	if err != nil {
		return nil, err
	}

	return handler, nil
}

// setupTables creates necessary tables if they don't exist
func (d *DuckDBHandler) setupTables() error {
	// Create TRNG data table with improved schema
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS trng_data (
			id INTEGER PRIMARY KEY,
			hash BLOB NOT NULL,
			hash_hex VARCHAR(64) NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			consumed BOOLEAN DEFAULT FALSE,
			source VARCHAR(20) DEFAULT 'hardware',
			chunk_size INTEGER DEFAULT 32
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create trng_data table: %w", err)
	}

	// Create Fortuna data table with improved schema
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS fortuna_data (
			id INTEGER PRIMARY KEY,
			data BLOB NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			consumed BOOLEAN DEFAULT FALSE,
			chunk_size INTEGER DEFAULT 32,
			amplification_factor INTEGER DEFAULT 4
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create fortuna_data table: %w", err)
	}

	// Create metadata table for configuration
	_, err = d.db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			key VARCHAR(50) PRIMARY KEY,
			value VARCHAR(255) NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Create indexes for better query performance
	_, err = d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_trng_timestamp ON trng_data(timestamp)`)
	if err != nil {
		return fmt.Errorf("failed to create index on trng_data: %w", err)
	}

	_, err = d.db.Exec(`CREATE INDEX IF NOT EXISTS idx_fortuna_timestamp ON fortuna_data(timestamp)`)
	if err != nil {
		return fmt.Errorf("failed to create index on fortuna_data: %w", err)
	}

	// Configure DuckDB for better performance
	_, err = d.db.Exec(`PRAGMA memory_limit='256MB'`)
	if err != nil {
		log.Printf("Warning: Failed to set memory limit: %v", err)
	}

	return nil
}

// StoreTRNGHash stores a new TRNG hash and maintains queue size
func (d *DuckDBHandler) StoreTRNGHash(hash []byte, source string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Generate hex representation
	hashHex := hex.EncodeToString(hash)

	// Use batched insertions for better performance
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert new hash with source information
	_, err = tx.Exec("INSERT INTO trng_data (hash, hash_hex, source, chunk_size) VALUES (?, ?, ?, 32)", hash, hashHex, source)
	if err != nil {
		return fmt.Errorf("failed to insert TRNG hash: %w", err)
	}

	// Maintain queue size by removing oldest entries more efficiently
	_, err = tx.Exec(`
		DELETE FROM trng_data
		WHERE id IN (
			SELECT id FROM trng_data
			ORDER BY timestamp ASC
			LIMIT (SELECT MAX(0, COUNT(*) - ?) FROM trng_data)
		)
	`, d.trngQueueSize)
	if err != nil {
		return fmt.Errorf("failed to maintain TRNG queue size: %w", err)
	}

	return tx.Commit()
}

// Legacy method for backward compatibility
func (d *DuckDBHandler) StoreTRNGHashLegacy(hash []byte) error {
	return d.StoreTRNGHash(hash, "hardware")
}

// StoreFortunaData stores Fortuna-generated data and maintains queue size
func (d *DuckDBHandler) StoreFortunaData(data []byte, chunkSize int, amplificationFactor int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Insert new data with additional metadata
	_, err = tx.Exec("INSERT INTO fortuna_data (data, chunk_size, amplification_factor) VALUES (?, ?, ?)",
		data, chunkSize, amplificationFactor)
	if err != nil {
		return fmt.Errorf("failed to insert Fortuna data: %w", err)
	}

	// Maintain queue size using more efficient query
	_, err = tx.Exec(`
		DELETE FROM fortuna_data
		WHERE id IN (
			SELECT id FROM fortuna_data
			ORDER BY timestamp ASC
			LIMIT (SELECT MAX(0, COUNT(*) - ?) FROM fortuna_data)
		)
	`, d.fortunaQueueSize)
	if err != nil {
		return fmt.Errorf("failed to maintain Fortuna queue size: %w", err)
	}

	return tx.Commit()
}

// Legacy method for backward compatibility
func (d *DuckDBHandler) StoreFortunaDataLegacy(data []byte) error {
	return d.StoreFortunaData(data, 32, 4)
}

// GetTRNGHashes retrieves TRNG hashes with pagination and optional consumption
func (d *DuckDBHandler) GetTRNGHashes(limit, offset int, consume bool) ([][]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	query := `
		SELECT id, hash
		FROM trng_data
		WHERE consumed = FALSE
		ORDER BY timestamp ASC
		LIMIT ? OFFSET ?
	`

	rows, err := d.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query TRNG hashes: %w", err)
	}
	defer rows.Close()

	var hashes [][]byte
	var ids []int

	for rows.Next() {
		var id int
		var hash []byte
		err = rows.Scan(&id, &hash)
		if err != nil {
			return nil, fmt.Errorf("failed to scan TRNG hash: %w", err)
		}

		hashes = append(hashes, hash)
		ids = append(ids, id)
	}

	if consume && len(ids) > 0 {
		// Mark hashes as consumed
		tx, err := d.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}

		for _, id := range ids {
			_, err = tx.Exec("UPDATE trng_data SET consumed = TRUE WHERE id = ?", id)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to mark TRNG hash as consumed: %w", err)
			}
		}

		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return hashes, nil
}

// GetFortunaData retrieves Fortuna-generated data with pagination and optional consumption
func (d *DuckDBHandler) GetFortunaData(limit, offset int, consume bool) ([][]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	query := `
		SELECT id, data
		FROM fortuna_data
		WHERE consumed = FALSE
		ORDER BY timestamp ASC
		LIMIT ? OFFSET ?
	`

	rows, err := d.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query Fortuna data: %w", err)
	}
	defer rows.Close()

	var dataSlices [][]byte
	var ids []int

	for rows.Next() {
		var id int
		var data []byte
		err = rows.Scan(&id, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to scan Fortuna data: %w", err)
		}

		dataSlices = append(dataSlices, data)
		ids = append(ids, id)
	}

	if consume && len(ids) > 0 {
		// Mark data as consumed
		tx, err := d.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}

		for _, id := range ids {
			_, err = tx.Exec("UPDATE fortuna_data SET consumed = TRUE WHERE id = ?", id)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to mark Fortuna data as consumed: %w", err)
			}
		}

		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return dataSlices, nil
}

// GetStats returns statistics about the database
func (d *DuckDBHandler) GetStats() (map[string]interface{}, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	stats := make(map[string]interface{})

	// Get TRNG queue stats
	var trngCount, trngUnconsumedCount int
	err := d.db.QueryRow("SELECT COUNT(*) FROM trng_data").Scan(&trngCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get TRNG count: %w", err)
	}

	err = d.db.QueryRow("SELECT COUNT(*) FROM trng_data WHERE consumed = FALSE").Scan(&trngUnconsumedCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get unconsumed TRNG count: %w", err)
	}

	// Get TRNG source stats
	var hardwareCount, softwareCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM trng_data WHERE source = 'hardware'").Scan(&hardwareCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get hardware TRNG count: %w", err)
	}

	err = d.db.QueryRow("SELECT COUNT(*) FROM trng_data WHERE source = 'software'").Scan(&softwareCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get software TRNG count: %w", err)
	}

	// Get Fortuna queue stats
	var fortunaCount, fortunaUnconsumedCount int
	err = d.db.QueryRow("SELECT COUNT(*) FROM fortuna_data").Scan(&fortunaCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get Fortuna count: %w", err)
	}

	err = d.db.QueryRow("SELECT COUNT(*) FROM fortuna_data WHERE consumed = FALSE").Scan(&fortunaUnconsumedCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get unconsumed Fortuna count: %w", err)
	}

	// Build stats object
	stats["trng_total"] = trngCount
	stats["trng_unconsumed"] = trngUnconsumedCount
	stats["trng_queue_full"] = trngCount >= d.trngQueueSize
	stats["trng_hardware_count"] = hardwareCount
	stats["trng_software_count"] = softwareCount
	stats["trng_hardware_percent"] = float64(hardwareCount) / float64(math.Max(float64(trngCount), 1.0)) * 100.0
	stats["fortuna_total"] = fortunaCount
	stats["fortuna_unconsumed"] = fortunaUnconsumedCount
	stats["fortuna_queue_full"] = fortunaCount >= d.fortunaQueueSize
	stats["database_size_bytes"] = d.getDatabaseSizeEstimate()

	return stats, nil
}

// GetSourceStats returns detailed statistics about hardware vs software generated data
func (d *DuckDBHandler) GetSourceStats() (map[string]interface{}, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	stats := make(map[string]interface{})

	// Get source distribution by time periods
	rows, err := d.db.Query(`
		SELECT 
			strftime('%Y-%m-%d', timestamp) as day,
			source,
			COUNT(*) as count
		FROM trng_data
		GROUP BY day, source
		ORDER BY day DESC
		LIMIT 30
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get source stats: %w", err)
	}
	defer rows.Close()

	// Parse the results
	dailyStats := make(map[string]map[string]int)
	for rows.Next() {
		var day, source string
		var count int
		err := rows.Scan(&day, &source, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source stats: %w", err)
		}

		if _, ok := dailyStats[day]; !ok {
			dailyStats[day] = make(map[string]int)
		}
		dailyStats[day][source] = count
	}

	// Calculate percentages
	sourcePercentages := make(map[string]map[string]float64)
	for day, counts := range dailyStats {
		sourcePercentages[day] = make(map[string]float64)
		total := 0
		for _, count := range counts {
			total += count
		}

		for source, count := range counts {
			sourcePercentages[day][source] = float64(count) / float64(total) * 100.0
		}
	}

	stats["daily_counts"] = dailyStats
	stats["daily_percentages"] = sourcePercentages

	return stats, nil
}

// getDatabaseSizeEstimate returns an estimate of the database size in bytes
func (d *DuckDBHandler) getDatabaseSizeEstimate() int64 {
	// Get approximate size based on row counts and average row sizes
	var trngCount, fortunaCount int64
	d.db.QueryRow("SELECT COUNT(*) FROM trng_data").Scan(&trngCount)
	d.db.QueryRow("SELECT COUNT(*) FROM fortuna_data").Scan(&fortunaCount)

	// Approximate sizes: TRNG ~100 bytes/row, Fortuna ~150 bytes/row, plus overhead
	return (trngCount * 100) + (fortunaCount * 150) + 10240 // 10KB overhead
}

// Close closes the database connection
func (d *DuckDBHandler) Close() error {
	return d.db.Close()
}

// UpdateQueueSizes updates the queue sizes for TRNG and Fortuna data
func (d *DuckDBHandler) UpdateQueueSizes(trngQueueSize, fortunaQueueSize int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.trngQueueSize = trngQueueSize
	d.fortunaQueueSize = fortunaQueueSize

	// Trim queues if they exceed new sizes
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Trim TRNG queue
	_, err = tx.Exec(`
		DELETE FROM trng_data
		WHERE id IN (
			SELECT id FROM trng_data
			ORDER BY timestamp ASC
			LIMIT (SELECT MAX(0, COUNT(*) - ?) FROM trng_data)
		)
	`, trngQueueSize)
	if err != nil {
		return fmt.Errorf("failed to trim TRNG queue: %w", err)
	}

	// Trim Fortuna queue
	_, err = tx.Exec(`
		DELETE FROM fortuna_data
		WHERE id IN (
			SELECT id FROM fortuna_data
			ORDER BY timestamp ASC
			LIMIT (SELECT MAX(0, COUNT(*) - ?) FROM fortuna_data)
		)
	`, fortunaQueueSize)
	if err != nil {
		return fmt.Errorf("failed to trim Fortuna queue: %w", err)
	}

	return tx.Commit()
}

// HealthCheck checks if the database is accessible
func (d *DuckDBHandler) HealthCheck() bool {
	err := d.db.Ping()
	if err != nil {
		log.Printf("Database health check failed: %v", err)
		return false
	}
	return true
}
