package api

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/lokey/rng-service/pkg/database"
	"github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title RNG Service API
// @version 1.0
// @description API for accessing hardware-generated random data
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@rng-service.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// Server represents the API server
type Server struct {
	db             *database.DuckDBHandler
	controllerAddr string
	fortunaAddr    string
	port           int
	router         *gin.Engine
	validate       *validator.Validate
}

// QueueConfig represents the queue configuration
type QueueConfig struct {
	TRNGQueueSize    int `json:"trng_queue_size" validate:"required,min=10,max=10000"`
	FortunaQueueSize int `json:"fortuna_queue_size" validate:"required,min=10,max=10000"`
}

// ConsumptionConfig represents the consumption behavior configuration
type ConsumptionConfig struct {
	DeleteOnConsumption bool `json:"delete_on_consumption"`
}

// DataRequest represents a request for random data
type DataRequest struct {
	Format    string `json:"format" validate:"required,oneof=int8 int16 int32 int64 uint8 uint16 uint32 uint64 binary"`
	ChunkSize int    `json:"chunk_size" validate:"required,min=1,max=1048576"`
	Limit     int    `json:"limit" validate:"required,min=1,max=1000"`
	Offset    int    `json:"offset" validate:"min=0"`
	Source    string `json:"source" validate:"required,oneof=trng fortuna"`
}

// HealthCheckResponse represents the health check response
type HealthCheckResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Details   struct {
		API        bool `json:"api"`
		Controller bool `json:"controller"`
		Fortuna    bool `json:"fortuna"`
		Database   bool `json:"database"`
	} `json:"details"`
}

// NewServer creates a new API server
func NewServer(db *database.DuckDBHandler, controllerAddr, fortunaAddr string, port int) *Server {
	router := gin.Default()
	validate := validator.New()

	server := &Server{
		db:             db,
		controllerAddr: controllerAddr,
		fortunaAddr:    fortunaAddr,
		port:           port,
		router:         router,
		validate:       validate,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// Swagger docs
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 group
	api := s.router.Group("/api/v1")
	{
		// Configuration endpoints
		api.GET("/config/queue", s.GetQueueConfig)
		api.PUT("/config/queue", s.UpdateQueueConfig)
		api.GET("/config/consumption", s.GetConsumptionConfig)
		api.PUT("/config/consumption", s.UpdateConsumptionConfig)

		// Data retrieval endpoints
		api.POST("/data", s.GetRandomData)

		// Status endpoints
		api.GET("/status", s.GetStatus)
		api.GET("/health", s.HealthCheck)
	}
}

// Run starts the API server
func (s *Server) Run() error {
	return s.router.Run(fmt.Sprintf(":%d", s.port))
}

// @Summary Get queue configuration
// @Description Get current queue size configuration for TRNG and Fortuna data
// @Tags configuration
// @Accept json
// @Produce json
// @Success 200 {object} QueueConfig
// @Router /config/queue [get]
func (s *Server) GetQueueConfig(c *gin.Context) {
	stats, err := s.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue configuration"})
		return
	}

	config := QueueConfig{
		TRNGQueueSize:    stats["trng_queue_size"].(int),
		FortunaQueueSize: stats["fortuna_queue_size"].(int),
	}

	c.JSON(http.StatusOK, config)
}

// @Summary Update queue configuration
// @Description Update queue size configuration for TRNG and Fortuna data
// @Tags configuration
// @Accept json
// @Produce json
// @Param config body QueueConfig true "Queue configuration"
// @Success 200 {object} QueueConfig
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Server error"
// @Router /config/queue [put]
func (s *Server) UpdateQueueConfig(c *gin.Context) {
	var config QueueConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate config
	if err := s.validate.Struct(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update queue sizes
	err := s.db.UpdateQueueSizes(config.TRNGQueueSize, config.FortunaQueueSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// @Summary Get consumption configuration
// @Description Get current consumption behavior configuration
// @Tags configuration
// @Accept json
// @Produce json
// @Success 200 {object} ConsumptionConfig
// @Router /config/consumption [get]
func (s *Server) GetConsumptionConfig(c *gin.Context) {
	// For now, this is hardcoded since it's stored in memory
	// In a real implementation, this would be stored in a configuration store
	config := ConsumptionConfig{
		DeleteOnConsumption: true,
	}

	c.JSON(http.StatusOK, config)
}

// @Summary Update consumption configuration
// @Description Update consumption behavior configuration
// @Tags configuration
// @Accept json
// @Produce json
// @Param config body ConsumptionConfig true "Consumption configuration"
// @Success 200 {object} ConsumptionConfig
// @Failure 400 {object} map[string]string "Invalid request"
// @Router /config/consumption [put]
func (s *Server) UpdateConsumptionConfig(c *gin.Context) {
	var config ConsumptionConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// In a real implementation, this would update a configuration store
	// For now, we just return the received configuration

	c.JSON(http.StatusOK, config)
}

// @Summary Get random data
// @Description Retrieve random data in various formats with pagination
// @Tags data
// @Accept json
// @Produce json
// @Produce application/octet-stream
// @Param request body DataRequest true "Data request parameters"
// @Success 200 {array} interface{} "Random data in requested format"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Not enough data available"
// @Failure 500 {object} map[string]string "Server error"
// @Router /data [post]
func (s *Server) GetRandomData(c *gin.Context) {
	var request DataRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate request
	if err := s.validate.Struct(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get consumption configuration
	var consumeData bool = true // Default to true

	// Retrieve data based on source
	var rawData [][]byte
	var err error
	if request.Source == "trng" {
		rawData, err = s.db.GetTRNGHashes(request.Limit, request.Offset, consumeData)
	} else {
		rawData, err = s.db.GetFortunaData(request.Limit, request.Offset, consumeData)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data"})
		return
	}

	if len(rawData) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No data available"})
		return
	}

	// Process data based on requested format
	switch request.Format {
	case "binary":
		// For binary format, just concatenate the raw bytes
		binaryData := make([]byte, 0)
		for _, data := range rawData {
			// Take only the requested chunk size or less if data is smaller
			size := request.ChunkSize
			if size > len(data) {
				size = len(data)
			}
			binaryData = append(binaryData, data[:size]...)
		}

		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", "attachment; filename=random.bin")
		c.Data(http.StatusOK, "application/octet-stream", binaryData)

	case "int8":
		response := convertToIntFormat(rawData, request.ChunkSize, 1, true)
		c.JSON(http.StatusOK, response)

	case "uint8":
		response := convertToIntFormat(rawData, request.ChunkSize, 1, false)
		c.JSON(http.StatusOK, response)

	case "int16":
		response := convertToIntFormat(rawData, request.ChunkSize, 2, true)
		c.JSON(http.StatusOK, response)

	case "uint16":
		response := convertToIntFormat(rawData, request.ChunkSize, 2, false)
		c.JSON(http.StatusOK, response)

	case "int32":
		response := convertToIntFormat(rawData, request.ChunkSize, 4, true)
		c.JSON(http.StatusOK, response)

	case "uint32":
		response := convertToIntFormat(rawData, request.ChunkSize, 4, false)
		c.JSON(http.StatusOK, response)

	case "int64":
		response := convertToIntFormat(rawData, request.ChunkSize, 8, true)
		c.JSON(http.StatusOK, response)

	case "uint64":
		response := convertToIntFormat(rawData, request.ChunkSize, 8, false)
		c.JSON(http.StatusOK, response)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format"})
	}
}

// convertToIntFormat converts raw byte data to various integer formats
func convertToIntFormat(data [][]byte, chunkSize, bytesPerValue int, signed bool) []interface{} {
	var result []interface{}

	for _, chunk := range data {
		// Ensure we only use the requested chunk size
		size := chunkSize
		if size > len(chunk) {
			size = len(chunk)
		}

		// Process each value (bytesPerValue at a time)
		for i := 0; i <= size-bytesPerValue; i += bytesPerValue {
			switch bytesPerValue {
			case 1:
				if signed {
					result = append(result, int8(chunk[i]))
				} else {
					result = append(result, uint8(chunk[i]))
				}
			case 2:
				if signed {
					result = append(result, int16(binary.BigEndian.Uint16(chunk[i:i+2])))
				} else {
					result = append(result, binary.BigEndian.Uint16(chunk[i:i+2]))
				}
			case 4:
				if signed {
					result = append(result, int32(binary.BigEndian.Uint32(chunk[i:i+4])))
				} else {
					result = append(result, binary.BigEndian.Uint32(chunk[i:i+4]))
				}
			case 8:
				if signed {
					result = append(result, int64(binary.BigEndian.Uint64(chunk[i:i+8])))
				} else {
					result = append(result, binary.BigEndian.Uint64(chunk[i:i+8]))
				}
			}
		}
	}

	return result
}

// @Summary Get system status
// @Description Get status of TRNG and Fortuna queues and data availability
// @Tags status
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string "Server error"
// @Router /status [get]
func (s *Server) GetStatus(c *gin.Context) {
	stats, err := s.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get system status"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// @Summary Check system health
// @Description Check health of all system components
// @Tags status
// @Accept json
// @Produce json
// @Success 200 {object} HealthCheckResponse
// @Router /health [get]
func (s *Server) HealthCheck(c *gin.Context) {
	// Check database health
	dbHealthy := s.db.HealthCheck()

	// Check controller health (simplified for example)
	controllerHealthy := checkServiceHealth(s.controllerAddr + "/health")

	// Check fortuna service health (simplified for example)
	fortunaHealthy := checkServiceHealth(s.fortunaAddr + "/health")

	// Determine overall status
	overallStatus := "healthy"
	if !dbHealthy || !controllerHealthy || !fortunaHealthy {
		overallStatus = "unhealthy"
	}

	response := HealthCheckResponse{
		Status:    overallStatus,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	response.Details.API = true // API is running if we're handling this request
	response.Details.Controller = controllerHealthy
	response.Details.Fortuna = fortunaHealthy
	response.Details.Database = dbHealthy

	c.JSON(http.StatusOK, response)
}

// checkServiceHealth checks if a service is healthy by making an HTTP request
func checkServiceHealth(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Health check failed for %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
