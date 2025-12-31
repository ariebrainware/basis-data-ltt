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

	// Initialize database once
	db, err := config.ConnectMySQL()
	if err != nil {
		log.Fatalf("Error connecting to MySQL: %v", err)
	}
	err = db.AutoMigrate(&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.PatientCode{})
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
	// Pass db to all handlers via context middleware
	r.Use(middleware.DatabaseMiddleware(db))

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
		// Logout is available to all authenticated users
		auth.DELETE("/logout", endpoint.Logout)

		// Patient routes - accessible by Admin only
		patient := auth.Group("/patient")
		patient.Use(middleware.RequireRole(model.RoleAdmin))
		{
			patient.GET("", endpoint.ListPatients)
			patient.GET("/:id", endpoint.GetPatientInfo)
			patient.PATCH("/:id", endpoint.UpdatePatient)
			patient.DELETE("/:id", endpoint.DeletePatient)
		}

		// Treatment routes - accessible by Admin and Therapist
		treatment := auth.Group("/treatment")
		treatment.Use(middleware.RequireRole(model.RoleAdmin, model.RoleTherapist))
		{
			treatment.GET("", endpoint.ListTreatments)
			treatment.POST("", endpoint.CreateTreatment)
			treatment.PATCH("/:id", endpoint.UpdateTreatment)
			treatment.DELETE("/:id", endpoint.DeleteTreatment)
		}

		// Disease routes - accessible by Admin only
		disease := auth.Group("/disease")
		disease.Use(middleware.RequireRole(model.RoleAdmin))
		{
			disease.GET("", endpoint.ListDiseases)
			disease.POST("", endpoint.CreateDisease)
			disease.GET("/:id", endpoint.GetDiseaseInfo)
			disease.PATCH("/:id", endpoint.UpdateDisease)
			disease.DELETE("/:id", endpoint.DeleteDisease)
		}

		// Therapist routes - accessible by Admin only
		therapist := auth.Group("/therapist")
		therapist.Use(middleware.RequireRole(model.RoleAdmin))
		{
			therapist.GET("", endpoint.ListTherapist)
			therapist.GET("/:id", endpoint.GetTherapistInfo)
			therapist.POST("", endpoint.CreateTherapist)
			therapist.PATCH("/:id", endpoint.UpdateTherapist)
			therapist.DELETE("/:id", endpoint.DeleteTherapist)
			therapist.PUT("/:id", endpoint.TherapistApproval)
		}
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
