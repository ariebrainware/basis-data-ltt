// main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/middleware"
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
	db.AutoMigrate(&model.Patient{}, &model.Decease{}, &model.User{}, &model.Session{})

	// Set Gin mode from config
	gin.SetMode(cfg.GinMode)

	// Create a Gin router with default middleware
	r := gin.Default()

	// Use custom CORS middleware
	r.Use(middleware.CORSMiddleware())

	// Basic HTTP handler for root path
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Welcome to %s!", cfg.AppName),
		})
	})

	r.POST("/login", endpoint.Login)
	r.POST("/signup", endpoint.Signup)
	r.DELETE("/logout", endpoint.Logout)

	r.GET("/patient", endpoint.ListPatients)
	r.POST("/patient", endpoint.CreatePatient)
	r.PATCH("/patient/:id", endpoint.UpdatePatient)
	r.DELETE("/patient/:id", endpoint.DeletePatient)

	// Start server on specified port
	address := fmt.Sprintf(":%d", cfg.AppPort)
	if err := r.Run(address); err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}
