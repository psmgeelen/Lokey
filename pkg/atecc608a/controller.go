package atecc608a

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/d2r2/go-i2c"
)

const (
	Atecc608Address = 0x60 // ATECC608A I2C address
	RandomCommand   = 0x1B // Random command opcode
	ShaCommand      = 0x47 // SHA command opcode
	WakeCommand     = 0x00 // Wake command
	WakeParameter   = 0x11 // Wake parameter
)

type Controller struct {
	i2c         *i2c.I2C
	initialWake bool
	LastError   error
	mockMode    bool
}

// NewController creates a new ATECC608A controller
func NewController(i2cBusNumber int) (*Controller, error) {
	var i2cDevice *i2c.I2C
	var err error
	var mockMode bool

	// Try to initialize the I2C device
	i2cDevice, err = i2c.NewI2C(Atecc608Address, i2cBusNumber)
	if err != nil {
		log.Printf("WARNING: Failed to open I2C bus: %v. Falling back to mock mode.", err)
		mockMode = true
	}

	controller := &Controller{
		i2c:         i2cDevice,
		initialWake: false,
		LastError:   nil,
		mockMode:    mockMode,
	}

	// In real mode, wake up the device on initialization
	if !mockMode {
		err = controller.WakeUp()
		if err != nil {
			i2cDevice.Close()
			log.Printf("WARNING: Failed to wake up ATECC608A: %v. Falling back to mock mode.", err)
			controller.mockMode = true
		} else {
			controller.initialWake = true
		}
	}

	if controller.mockMode {
		log.Println("ATECC608A controller running in MOCK MODE. Using software-generated pseudo-random numbers.")
	}

	return controller, nil
}

// NewControllerWithMockMode creates a controller with explicit mock mode setting
func NewControllerWithMockMode(i2cBusNumber int, forceMock bool) (*Controller, error) {
	if forceMock {
		controller := &Controller{
			i2c:         nil,
			initialWake: false,
			LastError:   nil,
			mockMode:    true,
		}
		log.Println("ATECC608A controller running in FORCED MOCK MODE. Using software-generated pseudo-random numbers.")
		return controller, nil
	}

	return NewController(i2cBusNumber)
}

// WakeUp wakes up the ATECC608A device
func (c *Controller) WakeUp() error {
	// No need to wake up in mock mode
	if c.mockMode {
		return nil
	}

	err := c.writeCommand(WakeCommand, []byte{WakeParameter})
	if err != nil {
		c.LastError = err
		return fmt.Errorf("wake command failed: %w", err)
	}

	// Wait for device to wake up
	time.Sleep(10 * time.Millisecond)
	return nil
}

// writeCommand sends a command to the ATECC608A
func (c *Controller) writeCommand(command byte, data []byte) error {
	buf := append([]byte{command}, data...)
	_, err := c.i2c.WriteBytes(buf)
	if err != nil {
		c.LastError = err
		return err
	}
	return nil
}

// readResponse reads a response from the ATECC608A
func (c *Controller) readResponse(length int) ([]byte, error) {
	// Create a buffer of the requested length
	buf := make([]byte, length)
	// Use ReadBytes with the buffer
	n, err := c.i2c.WriteBytes(buf)
	if err != nil {
		c.LastError = err
		return nil, err
	}
	if n != length {
		c.LastError = fmt.Errorf("expected to write %d bytes but got %d", length, n)
		return nil, c.LastError
	}
	return buf, nil
}

// GenerateRandomBytes generates random bytes using the ATECC608A's TRNG or software PRNG in mock mode
func (c *Controller) GenerateRandomBytes() ([]byte, error) {
	// Use software random generation in mock mode
	if c.mockMode {
		randomData := make([]byte, 32)
		_, err := rand.Read(randomData)
		if err != nil {
			return nil, fmt.Errorf("failed to generate software random data: %w", err)
		}
		return randomData, nil
	}

	// Real hardware mode
	// Ensure device is awake
	if !c.initialWake {
		err := c.WakeUp()
		if err != nil {
			return nil, fmt.Errorf("failed to wake up device: %w", err)
		}
	}

	// Send Random command
	err := c.writeCommand(0x03, []byte{RandomCommand})
	if err != nil {
		return nil, fmt.Errorf("failed to send Random command: %w", err)
	}

	// Wait for processing
	time.Sleep(5 * time.Millisecond)

	// Read 32-byte random number
	randomData, err := c.readResponse(32)
	if err != nil {
		return nil, fmt.Errorf("failed to read random data: %w", err)
	}

	if len(randomData) != 32 {
		return nil, errors.New("invalid random data length")
	}

	return randomData, nil
}

// GenerateHashFromRandom generates a SHA-256 hash of random data using the device's hardware or software in mock mode
func (c *Controller) GenerateHashFromRandom() ([]byte, error) {
	// Generate random data first
	randomData, err := c.GenerateRandomBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random data: %w", err)
	}

	// Use software SHA-256 in mock mode
	if c.mockMode {
		hash := sha256.Sum256(randomData)
		return hash[:], nil
	}

	// Real hardware mode SHA-256
	// Start SHA computation
	err = c.writeCommand(0x03, []byte{ShaCommand})
	if err != nil {
		return nil, fmt.Errorf("failed to send SHA command: %w", err)
	}

	// Wait for processing
	time.Sleep(5 * time.Millisecond)

	// Send Random data to SHA command
	err = c.writeCommand(0x04, randomData)
	if err != nil {
		return nil, fmt.Errorf("failed to send data to SHA command: %w", err)
	}

	// Wait for processing
	time.Sleep(10 * time.Millisecond)

	// Read SHA digest
	shaDigest, err := c.readResponse(32)
	if err != nil {
		return nil, fmt.Errorf("failed to read SHA digest: %w", err)
	}

	if len(shaDigest) != 32 {
		return nil, errors.New("invalid SHA digest length")
	}

	return shaDigest, nil
}

// Close closes the connection to the ATECC608A
func (c *Controller) Close() error {
	if c.mockMode || c.i2c == nil {
		return nil
	}
	return c.i2c.Close()
}

// HealthCheck checks if the ATECC608A device is responsive
func (c *Controller) HealthCheck() bool {
	// Always healthy in mock mode
	if c.mockMode {
		return true
	}

	// Try to wake up the device
	err := c.WakeUp()
	if err != nil {
		log.Printf("ATECC608A health check failed: %v", err)
		return false
	}

	// Try to generate random data as a test
	_, err = c.GenerateRandomBytes()
	if err != nil {
		log.Printf("ATECC608A random generation failed: %v", err)
		return false
	}

	return true
}

// IsMockMode returns whether the controller is in mock mode
func (c *Controller) IsMockMode() bool {
	return c.mockMode
}
