// main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load the configuration
	cfg := config.LoadConfig()

	// Set Gin mode from config
	gin.SetMode(cfg.GinMode)

	// Create a Gin router with default middleware
	router := gin.Default()

	// Basic HTTP handler for root path
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, world!",
		})
	})

	// Start server on specified port
	address := fmt.Sprintf(":%d", cfg.AppPort)
	if err := router.Run(address); err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}
