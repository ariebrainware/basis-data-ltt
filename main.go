// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "time/tzdata"

	"github.com/ariebrainware/basis-data-ltt/config"
	_ "github.com/ariebrainware/basis-data-ltt/docs"
	"github.com/ariebrainware/basis-data-ltt/endpoint"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           LTT Backend API
// @version         v1.6.0
// @description     REST API for managing patient data, treatments, and therapy sessions
// @description     This API provides endpoints for patient management, disease tracking, treatment records, and therapist management.

// @contact.name   Arie Brainware
// @contact.email  support@ariebrainware.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:19091
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey SessionToken
// @in header
// @name session-token
// @description Session token for authenticated requests

func main() {
	// Load the configuration
	cfg := config.LoadConfig()

	// Initialize JWT secret from environment and validate it.
	util.InitJWTSecretFromEnv()
	if cfg.AppEnv != "test" {
		if err := util.ValidateJWTSecret(); err != nil {
			log.Fatalf("%v", err)
		}
	}

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

	// Initialize Redis (optional, warn on failure)
	if _, err := config.ConnectRedis(); err != nil {
		log.Printf("Warning: could not connect to Redis: %v", err)
	} else {
		log.Println("Redis initialization complete")
	}
	// If the diseases table already exists, there may be rows with empty
	// `codename` values which will cause a unique-index creation to fail
	// (duplicate entry '' for unique index). To avoid that, first alter the
	// column to allow NULLs and convert empty strings to NULL, then run
	// AutoMigrate which will (re)create indexes safely.
	if db.Migrator().HasTable(&model.Disease{}) {
		// Make column nullable to allow converting empty strings to NULL.
		if err := db.Exec("ALTER TABLE diseases MODIFY codename varchar(191) NULL").Error; err != nil {
			log.Printf("Warning: failed to alter diseases.codename to NULL-able: %v", err)
		} else {
			log.Println("Converted diseases.codename to allow NULLs (if applicable)")
		}

		// Convert any existing empty-string codename values to NULL so they
		// won't violate the unique index constraint (MySQL treats NULLs as
		// distinct, allowing multiple NULLs).
		if err := db.Exec("UPDATE diseases SET codename = NULL WHERE codename = ''").Error; err != nil {
			log.Printf("Warning: failed to nullify empty codename values: %v", err)
		} else {
			log.Println("Nullified empty codename values in diseases table (if any)")
		}
	}

	// Proceed with auto-migration for all models
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

	// Swagger documentation route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// Group routes that require a valid login token
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())
	{
		// Logout is available to all authenticated users
		auth.DELETE("/logout", endpoint.Logout)

		// Current user profile update
		auth.PATCH("/user", endpoint.UpdateUser)

		// Admin-only user management
		userAdmin := auth.Group("/user")
		userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
		{
			userAdmin.GET("", endpoint.ListUsers)
			userAdmin.GET("/:id", endpoint.GetUserInfo)
			userAdmin.PATCH("/:id", endpoint.UpdateUserByID)
			userAdmin.DELETE("/:id", endpoint.DeleteUser)
		}

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

	// Start server on specified port with graceful shutdown
	address := fmt.Sprintf(":%d", cfg.AppPort)
	srv := &http.Server{
		Addr:              address,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting server: %v", err)
		}
	}()

	log.Printf("Server is listening on %s", address)

	// Wait for interrupt signal to gracefully shutdown the server with a timeout
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close DB connection pool
	sqlDB, err := db.DB()
	if err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	} else {
		log.Printf("Failed to get raw DB from GORM: %v", err)
	}

	log.Println("Server exiting")
}
