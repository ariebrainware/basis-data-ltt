// main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	_ "time/tzdata"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load the configuration
	cfg := config.LoadConfig()

	// Set the timezone to Asia/Jakarta
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Fatalf("Error loading timezone: %v", err)
	}
	time.Local = location
	gormConfig := &gorm.Config{}
	if cfg.AppEnv == "production" {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}
	db, err := config.ConnectMySQL()
	if err != nil {
		log.Fatalf("Error connecting to MySQL: %v", err)
	}
	err = db.AutoMigrate(&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Schedule{})
	if err != nil {
		log.Fatalf("Error migrating database: %v", err)
	}

	// Seed data example.
	if err := model.SeedRoles(db); err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

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
	// Group routes that require a valid login token
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		auth.GET("/patient", endpoint.ListPatients)
		auth.GET("/patient/:id", endpoint.GetPatientInfo)
		auth.PATCH("/patient/:id", endpoint.UpdatePatient)
		auth.DELETE("/patient/:id", endpoint.DeletePatient)

		auth.DELETE("/logout", endpoint.Logout)

		auth.GET("/disease", endpoint.ListDiseases)
		auth.POST("/disease", endpoint.CreateDisease)
		auth.GET("/disease/:id", endpoint.GetDiseaseInfo)
		auth.PATCH("/disease/:id", endpoint.UpdateDisease)
		auth.DELETE("/disease/:id", endpoint.DeleteDisease)

		therapist := auth.Group("/therapist")
		therapist.GET("/", endpoint.ListTherapist)
		therapist.POST("/", endpoint.CreateTherapist)
		therapist.GET("/:id", endpoint.GetTherapistInfo)
		therapist.PATCH("/:id", endpoint.UpdateTherapist)
		therapist.DELETE("/:id", endpoint.DeleteTherapist)
		therapist.PUT("/:id", endpoint.TherapistApproval)
		therapist.POST("/schedule", endpoint.CreateTherapistSchedule)
		therapist.GET("/schedule", endpoint.GetTherapistSchedule)
		therapist.PATCH("/schedule/:id", endpoint.UpdateTherapistSchedule)
		therapist.DELETE("/schedule/:id", endpoint.DeleteTherapistSchedule)
	}

	// the exception for create patient so it can be accessed without login
	r.POST("/patient", endpoint.CreatePatient)

	r.POST("/login", endpoint.Login)
	r.POST("/signup", endpoint.Signup)
	r.GET("/token/validate", endpoint.ValidateToken)

	// Start server on specified port
	address := fmt.Sprintf(":%d", cfg.AppPort)
	if err := r.Run(address); err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}
