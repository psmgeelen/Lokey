package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lokey/rng-service/pkg/atecc608a"
	"github.com/lokey/rng-service/pkg/database"
)

const (
	DefaultPort          = 8081
	DefaultI2CBusNumber  = 1
	DefaultDbPath        = "/data/trng.db"
	DefaultHashInterval  = 1 * time.Second
	DefaultTRNGQueueSize = 100
)

type Controller struct {
	device        *atecc608a.Controller
	db            *database.DuckDBHandler
	port          int
	hashInterval  time.Duration
	router        *gin.Engine
	running       bool
	hasherWg      sync.WaitGroup
	hasherCancel  context.CancelFunc
	hasherContext context.Context
}

func NewController(i2cBusNumber int, dbPath string, port int, hashInterval time.Duration, trngQueueSize int) (*Controller, error) {
	// Initialize ATECC608A controller
	device, err := atecc608a.NewController(i2cBusNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ATECC608A: %w", err)
	}

	// Initialize database
	db, err := database.NewDuckDBHandler(dbPath, trngQueueSize, 0) // 0 for fortuna size as we don't manage it here
	if err != nil {
		device.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize router
	router := gin.Default()

	return &Controller{
		device:       device,
		db:           db,
		port:         port,
		hashInterval: hashInterval,
		router:       router,
		running:      false,
	}, nil
}

func (c *Controller) setupRoutes() {
	// API routes
	c.router.GET("/health", c.healthCheckHandler)
	c.router.GET("/info", c.infoHandler)
	c.router.GET("/generate", c.generateHashHandler)
}

func (c *Controller) Start() error {
	// Setup routes
	c.setupRoutes()

	// Start hash generator in background
	c.startHashGenerator()

	// Start HTTP server
	serverErr := make(chan error, 1)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.port),
		Handler: c.router,
	}

	go func() {
		log.Printf("Starting controller server on port %d", c.port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Printf("Received signal %s, shutting down", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Stop hash generator
	c.stopHashGenerator()

	// Close resources
	if err := c.device.Close(); err != nil {
		log.Printf("Error closing ATECC608A: %v", err)
	}

	if err := c.db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	return nil
}

func (c *Controller) startHashGenerator() {
	c.hasherContext, c.hasherCancel = context.WithCancel(context.Background())
	c.running = true
	c.hasherWg.Add(1)

	go func() {
		defer c.hasherWg.Done()
		ticker := time.NewTicker(c.hashInterval)
		defer ticker.Stop()

		log.Printf("Hash generator started with interval %s", c.hashInterval)

		for {
			select {
			case <-ticker.C:
				c.generateAndStoreHash()
			case <-c.hasherContext.Done():
				log.Println("Hash generator stopped")
				return
			}
		}
	}()
}

func (c *Controller) stopHashGenerator() {
	if c.running {
		c.hasherCancel()
		c.hasherWg.Wait()
		c.running = false
	}
}

func (c *Controller) generateAndStoreHash() {
	hash, err := c.device.GenerateHashFromRandom()
	if err != nil {
		log.Printf("Failed to generate hash: %v", err)
		return
	}

	// Determine source based on mock mode
	source := "hardware"
	if c.device.IsMockMode() {
		source = "software"
		log.Printf("Generated SOFTWARE random hash: %s", hex.EncodeToString(hash))
	} else {
		log.Printf("Generated HARDWARE random hash: %s", hex.EncodeToString(hash))
	}

	// Store with source information
	err = c.db.StoreTRNGHash(hash, source)
	if err != nil {
		log.Printf("Failed to store hash: %v", err)
	}
}

// HTTP Handlers

func (c *Controller) healthCheckHandler(ctx *gin.Context) {
	healthy := c.device.HealthCheck() && c.db.HealthCheck()
	if healthy {
		ctx.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	} else {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"details": gin.H{
				"device":   c.device.HealthCheck(),
				"database": c.db.HealthCheck(),
			},
		})
	}
}

func (c *Controller) infoHandler(ctx *gin.Context) {
	stats, err := c.db.GetStats()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	// Get source stats if detailed=true query parameter is present
	detailed := ctx.DefaultQuery("detailed", "false") == "true"
	var sourceStats map[string]interface{}
	if detailed {
		sourceStats, err = c.db.GetSourceStats()
		if err != nil {
			log.Printf("Failed to get source stats: %v", err)
		}
	}

	response := gin.H{
		"status":           "running",
		"hash_interval_ms": c.hashInterval.Milliseconds(),
		"mock_mode":        c.device.IsMockMode(),
		"source":           map[string]bool{"hardware": !c.device.IsMockMode(), "software": c.device.IsMockMode()},
		"stats":            stats,
	}

	if detailed && sourceStats != nil {
		response["source_stats"] = sourceStats
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *Controller) generateHashHandler(ctx *gin.Context) {
	hash, err := c.device.GenerateHashFromRandom()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate hash"})
		return
	}

	// Determine source based on mock mode
	source := "hardware"
	if c.device.IsMockMode() {
		source = "software"
	}

	err = c.db.StoreTRNGHash(hash, source)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store hash"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"hash": hex.EncodeToString(hash),
	})
}

func main() {
	// Read configuration from environment variables

	// Check for forced mock mode for testing
	forceMockMode := false
	if val, ok := os.LookupEnv("FORCE_MOCK_MODE"); ok && (val == "1" || val == "true") {
		forceMockMode = true
		log.Println("FORCE_MOCK_MODE enabled. Will use software random generation.")
	}

	i2cBusNumber := DefaultI2CBusNumber
	if val, ok := os.LookupEnv("I2C_BUS_NUMBER"); ok {
		if n, err := fmt.Sscanf(val, "%d", &i2cBusNumber); n != 1 || err != nil {
			log.Printf("Invalid I2C_BUS_NUMBER, using default: %d", DefaultI2CBusNumber)
			i2cBusNumber = DefaultI2CBusNumber
		}
	}

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

	hashIntervalMs := DefaultHashInterval.Milliseconds()
	if val, ok := os.LookupEnv("HASH_INTERVAL_MS"); ok {
		if n, err := fmt.Sscanf(val, "%d", &hashIntervalMs); n != 1 || err != nil {
			log.Printf("Invalid HASH_INTERVAL_MS, using default: %d", DefaultHashInterval.Milliseconds())
			hashIntervalMs = DefaultHashInterval.Milliseconds()
		}
	}
	hashInterval := time.Duration(hashIntervalMs) * time.Millisecond

	trngQueueSize := DefaultTRNGQueueSize
	if val, ok := os.LookupEnv("TRNG_QUEUE_SIZE"); ok {
		if n, err := fmt.Sscanf(val, "%d", &trngQueueSize); n != 1 || err != nil {
			log.Printf("Invalid TRNG_QUEUE_SIZE, using default: %d", DefaultTRNGQueueSize)
			trngQueueSize = DefaultTRNGQueueSize
		}
	}

	// Create and start controller
	var controller *Controller
	var err error

	if forceMockMode {
		// Use the ATECC608A controller with forced mock mode
		device, err := atecc608a.NewControllerWithMockMode(i2cBusNumber, true)
		if err != nil {
			log.Fatalf("Failed to create mock controller: %v", err)
		}

		// Initialize database
		db, err := database.NewDuckDBHandler(dbPath, trngQueueSize, 0)
		if err != nil {
			device.Close()
			log.Fatalf("Failed to initialize database: %v", err)
		}

		// Initialize router
		router := gin.Default()

		controller = &Controller{
			device:       device,
			db:           db,
			port:         port,
			hashInterval: hashInterval,
			router:       router,
			running:      false,
		}
	} else {
		controller, err = NewController(i2cBusNumber, dbPath, port, hashInterval, trngQueueSize)
		if err != nil {
			log.Fatalf("Failed to create controller: %v", err)
		}
	}

	log.Printf("Starting TRNG controller with configuration:")
	log.Printf("  I2C Bus Number: %d", i2cBusNumber)
	log.Printf("  Database Path: %s", dbPath)
	log.Printf("  Port: %d", port)
	log.Printf("  Hash Interval: %s", hashInterval)
	log.Printf("  TRNG Queue Size: %d", trngQueueSize)

	err = controller.Start()
	if err != nil {
		log.Fatalf("Controller error: %v", err)
	}

	log.Println("Controller gracefully shut down")
}
