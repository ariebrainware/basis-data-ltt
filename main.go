// main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load the configuration
	cfg := config.LoadConfig()

	db, err := config.ConnectMySQL()
	if err != nil {
		log.Fatalf("Error connecting to MySQL: %v", err)
	}
	db.AutoMigrate(&model.Patient{}, &model.Decease{})

	// Set Gin mode from config
	gin.SetMode(cfg.GinMode)

	// Create a Gin router with default middleware
	router := gin.Default()

	// Basic HTTP handler for root path
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Welcome to %s!", cfg.AppName),
		})
	})

	router.GET("/patient", endpoint.ListPatients)
	router.POST("/patient", endpoint.CreatePatient)
	router.PATCH("/patient/:id", endpoint.UpdatePatient)

	// Start server on specified port
	address := fmt.Sprintf(":%d", cfg.AppPort)
	if err := router.Run(address); err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}
