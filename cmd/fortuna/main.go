package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lokey/rng-service/pkg/database"
	"github.com/lokey/rng-service/pkg/fortuna"
)

const (
	DefaultPort                = 8082
	DefaultDbPath              = "/data/fortuna.db"
	DefaultControllerURL       = "http://controller:8081"
	DefaultProcessInterval     = 5 * time.Second
	DefaultFortunaQueueSize    = 100
	DefaultAmplificationFactor = 4
	DefaultSeedCount           = 3
)

type FortunaProcessor struct {
	generator           *fortuna.Generator
	db                  *database.DuckDBHandler
	controllerURL       string
	port                int
	processInterval     time.Duration
	amplificationFactor int
	seedCount           int
	router              *gin.Engine
	running             bool
	processorWg         sync.WaitGroup
	processorCancel     context.CancelFunc
	processorContext    context.Context
}

func NewFortunaProcessor(dbPath, controllerURL string, port int, processInterval time.Duration, fortunaQueueSize, amplificationFactor, seedCount int) (*FortunaProcessor, error) {
	// Initialize database
	db, err := database.NewDuckDBHandler(dbPath, 0, fortunaQueueSize) // 0 for TRNG size as we don't manage it here
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize router
	router := gin.Default()

	// Initialize Fortuna with a temporary seed (will be reseeded from TRNG later)
	initialSeed := make([]byte, 32)
	for i := range initialSeed {
		initialSeed[i] = byte(i)
	}

	generator, err := fortuna.NewGenerator(initialSeed)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize Fortuna generator: %w", err)
	}

	return &FortunaProcessor{
		generator:           generator,
		db:                  db,
		controllerURL:       controllerURL,
		port:                port,
		processInterval:     processInterval,
		amplificationFactor: amplificationFactor,
		seedCount:           seedCount,
		router:              router,
		running:             false,
	}, nil
}

func (p *FortunaProcessor) setupRoutes() {
	// API routes
	p.router.GET("/health", p.healthCheckHandler)
	p.router.GET("/info", p.infoHandler)
	p.router.GET("/generate", p.generateDataHandler)
}

func (p *FortunaProcessor) Start() error {
	// Setup routes
	p.setupRoutes()

	// Initial seeding from TRNG
	if err := p.initialSeed(); err != nil {
		log.Printf("Warning: initial seeding failed: %v", err)
	}

	// Start processor in background
	p.startProcessor()

	// Start HTTP server
	serverErr := make(chan error, 1)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", p.port),
		Handler: p.router,
	}

	go func() {
		log.Printf("Starting Fortuna processor server on port %d", p.port)
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

	// Stop processor
	p.stopProcessor()

	// Close resources
	if err := p.db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	return nil
}

func (p *FortunaProcessor) startProcessor() {
	p.processorContext, p.processorCancel = context.WithCancel(context.Background())
	p.running = true
	p.processorWg.Add(1)

	go func() {
		defer p.processorWg.Done()
		ticker := time.NewTicker(p.processInterval)
		defer ticker.Stop()

		log.Printf("Fortuna processor started with interval %s", p.processInterval)

		for {
			select {
			case <-ticker.C:
				p.processAndAmplify()
			case <-p.processorContext.Done():
				log.Println("Fortuna processor stopped")
				return
			}
		}
	}()
}

func (p *FortunaProcessor) stopProcessor() {
	if p.running {
		p.processorCancel()
		p.processorWg.Wait()
		p.running = false
	}
}

func (p *FortunaProcessor) initialSeed() error {
	// Fetch initial TRNG data from controller
	seeds, err := p.fetchTRNGData(p.seedCount)
	if err != nil {
		return fmt.Errorf("failed to fetch initial TRNG data: %w", err)
	}

	if len(seeds) < p.seedCount {
		return fmt.Errorf("insufficient TRNG data for initial seeding, got %d, need %d", len(seeds), p.seedCount)
	}

	// Add seeds to Fortuna pools
	for i, seed := range seeds {
		p.generator.AddRandomEvent(byte(i), seed)
	}

	// Reseed the generator
	err = p.generator.ReseedFromPools()
	if err != nil {
		return fmt.Errorf("failed to reseed Fortuna: %w", err)
	}

	log.Println("Fortuna generator successfully seeded from TRNG")
	return nil
}

func (p *FortunaProcessor) processAndAmplify() {
	// Fetch new TRNG data
	seeds, err := p.fetchTRNGData(p.seedCount)
	if err != nil {
		log.Printf("Failed to fetch TRNG data: %v", err)
		return
	}

	if len(seeds) == 0 {
		log.Println("No new TRNG data available")
		return
	}

	// Combine all seeds into one
	combinedSeed := make([]byte, 0)
	for _, seed := range seeds {
		combinedSeed = append(combinedSeed, seed...)
	}

	// Amplify the random data
	outputLength := len(combinedSeed) * p.amplificationFactor
	amplifiedData, err := p.generator.AmplifyRandomData(combinedSeed, outputLength)
	if err != nil {
		log.Printf("Failed to amplify random data: %v", err)
		return
	}

	// Store amplified data using the newer method
	err = p.db.StoreFortunaData(amplifiedData, len(combinedSeed), p.amplificationFactor)
	if err != nil {
		log.Printf("Failed to store Fortuna data: %v", err)
		return
	}

	log.Printf("Generated and stored %d bytes of Fortuna-amplified data from %d bytes of TRNG data",
		len(amplifiedData), len(combinedSeed))
}

func (p *FortunaProcessor) fetchTRNGData(count int) ([][]byte, error) {
	// Make a request to the controller's API to get TRNG hashes
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/data?limit=%d&consume=true", p.controllerURL, count))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("controller returned error: %s, status: %d", string(body), resp.StatusCode)
	}

	// Parse response
	var hashes []string
	err = json.NewDecoder(resp.Body).Decode(&hashes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse controller response: %w", err)
	}

	// Convert hex strings to byte slices
	result := make([][]byte, 0, len(hashes))
	for _, hexStr := range hashes {
		hashBytes, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hash: %w", err)
		}
		result = append(result, hashBytes)
	}

	return result, nil
}

// HTTP Handlers

func (p *FortunaProcessor) healthCheckHandler(ctx *gin.Context) {
	healthy := p.generator.HealthCheck() && p.db.HealthCheck()
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
				"generator": p.generator.HealthCheck(),
				"database":  p.db.HealthCheck(),
			},
		})
	}
}

func (p *FortunaProcessor) infoHandler(ctx *gin.Context) {
	stats, err := p.db.GetStats()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":               "running",
		"process_interval_ms":  p.processInterval.Milliseconds(),
		"amplification_factor": p.amplificationFactor,
		"seed_count":           p.seedCount,
		"stats":                stats,
	})
}

func (p *FortunaProcessor) generateDataHandler(ctx *gin.Context) {
	// Get requested size parameter
	sizeStr := ctx.DefaultQuery("size", "128")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 || size > 1024*1024 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size parameter (1-1048576)"})
		return
	}

	// Generate random data
	data, err := p.generator.GenerateRandomData(size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate data"})
		return
	}

	// Store the generated data using the newer method with appropriate parameters
	err = p.db.StoreFortunaData(data, size, 1) // amplification factor of 1 for direct generation
	if err != nil {
		log.Printf("Warning: failed to store generated data: %v", err)
		// Continue anyway to return the data to the client
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": hex.EncodeToString(data),
		"size": len(data),
	})
}

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

	controllerURL := DefaultControllerURL
	if val, ok := os.LookupEnv("CONTROLLER_URL"); ok && val != "" {
		controllerURL = val
	}

	processIntervalMs := DefaultProcessInterval.Milliseconds()
	if val, ok := os.LookupEnv("PROCESS_INTERVAL_MS"); ok {
		if n, err := fmt.Sscanf(val, "%d", &processIntervalMs); n != 1 || err != nil {
			log.Printf("Invalid PROCESS_INTERVAL_MS, using default: %d", DefaultProcessInterval.Milliseconds())
			processIntervalMs = DefaultProcessInterval.Milliseconds()
		}
	}
	processInterval := time.Duration(processIntervalMs) * time.Millisecond

	fortunaQueueSize := DefaultFortunaQueueSize
	if val, ok := os.LookupEnv("FORTUNA_QUEUE_SIZE"); ok {
		if n, err := fmt.Sscanf(val, "%d", &fortunaQueueSize); n != 1 || err != nil {
			log.Printf("Invalid FORTUNA_QUEUE_SIZE, using default: %d", DefaultFortunaQueueSize)
			fortunaQueueSize = DefaultFortunaQueueSize
		}
	}

	amplificationFactor := DefaultAmplificationFactor
	if val, ok := os.LookupEnv("AMPLIFICATION_FACTOR"); ok {
		if n, err := fmt.Sscanf(val, "%d", &amplificationFactor); n != 1 || err != nil {
			log.Printf("Invalid AMPLIFICATION_FACTOR, using default: %d", DefaultAmplificationFactor)
			amplificationFactor = DefaultAmplificationFactor
		}
	}

	seedCount := DefaultSeedCount
	if val, ok := os.LookupEnv("SEED_COUNT"); ok {
		if n, err := fmt.Sscanf(val, "%d", &seedCount); n != 1 || err != nil {
			log.Printf("Invalid SEED_COUNT, using default: %d", DefaultSeedCount)
			seedCount = DefaultSeedCount
		}
	}

	// Create and start Fortuna processor
	processor, err := NewFortunaProcessor(dbPath, controllerURL, port, processInterval, fortunaQueueSize, amplificationFactor, seedCount)
	if err != nil {
		log.Fatalf("Failed to create Fortuna processor: %v", err)
	}

	log.Printf("Starting Fortuna processor with configuration:")
	log.Printf("  Controller URL: %s", controllerURL)
	log.Printf("  Database Path: %s", dbPath)
	log.Printf("  Port: %d", port)
	log.Printf("  Process Interval: %s", processInterval)
	log.Printf("  Fortuna Queue Size: %d", fortunaQueueSize)
	log.Printf("  Amplification Factor: %d", amplificationFactor)
	log.Printf("  Seed Count: %d", seedCount)

	err = processor.Start()
	if err != nil {
		log.Fatalf("Fortuna processor error: %v", err)
	}

	log.Println("Fortuna processor gracefully shut down")
}
