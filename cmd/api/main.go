package main

import (
	"fmt"
	"log"
	"os"

	"github.com/lokey/rng-service/pkg/api"
	"github.com/lokey/rng-service/pkg/database"
)

const (
	DefaultPort             = 8080
	DefaultDbPath           = "/data/api.db"
	DefaultControllerAddr   = "http://controller:8081"
	DefaultFortunaAddr      = "http://fortuna:8082"
	DefaultTRNGQueueSize    = 100
	DefaultFortunaQueueSize = 100
)

func main() {
	// Read configuration from environment variables
	port := DefaultPort
	if val, ok := os.LookupEnv("PORT"); ok {
		if n, err := fmt.Sscanf(val, "%d", &port); n != 1 || err != nil {
			log.Printf("Invalid PORT, using default: %d", DefaultPort)
			port = DefaultPort
		}
	}

	dbPath := DefaultDbPath
	if val, ok := os.LookupEnv("DB_PATH"); ok && val != "" {
		dbPath = val
	}

	controllerAddr := DefaultControllerAddr
	if val, ok := os.LookupEnv("CONTROLLER_ADDR"); ok && val != "" {
		controllerAddr = val
	}

	fortunaAddr := DefaultFortunaAddr
	if val, ok := os.LookupEnv("FORTUNA_ADDR"); ok && val != "" {
		fortunaAddr = val
	}

	trngQueueSize := DefaultTRNGQueueSize
	if val, ok := os.LookupEnv("TRNG_QUEUE_SIZE"); ok {
		if n, err := fmt.Sscanf(val, "%d", &trngQueueSize); n != 1 || err != nil {
			log.Printf("Invalid TRNG_QUEUE_SIZE, using default: %d", DefaultTRNGQueueSize)
			trngQueueSize = DefaultTRNGQueueSize
		}
	}

	fortunaQueueSize := DefaultFortunaQueueSize
	if val, ok := os.LookupEnv("FORTUNA_QUEUE_SIZE"); ok {
		if n, err := fmt.Sscanf(val, "%d", &fortunaQueueSize); n != 1 || err != nil {
			log.Printf("Invalid FORTUNA_QUEUE_SIZE, using default: %d", DefaultFortunaQueueSize)
			fortunaQueueSize = DefaultFortunaQueueSize
		}
	}

	// Initialize database
	db, err := database.NewDuckDBHandler(dbPath, trngQueueSize, fortunaQueueSize)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create and start API server
	server := api.NewServer(db, controllerAddr, fortunaAddr, port)

	log.Printf("Starting API server with configuration:")
	log.Printf("  Database Path: %s", dbPath)
	log.Printf("  Port: %d", port)
	log.Printf("  Controller Address: %s", controllerAddr)
	log.Printf("  Fortuna Address: %s", fortunaAddr)
	log.Printf("  TRNG Queue Size: %d", trngQueueSize)
	log.Printf("  Fortuna Queue Size: %d", fortunaQueueSize)

	if err := server.Run(); err != nil {
		log.Fatalf("API server error: %v", err)
	}
}
