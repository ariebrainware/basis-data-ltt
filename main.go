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
	"gorm.io/gorm"
)

// @title           LTT Backend API
// @version         v1.7.2
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
	// Keep main small: delegate work to helper functions
	cfg := config.LoadConfig()

	if err := initJWT(cfg); err != nil {
		log.Fatalf("JWT init failed: %v", err)
	}

	if err := setTimezone(); err != nil {
		log.Fatalf("Timezone init failed: %v", err)
	}

	db, err := initDB()
	if err != nil {
		log.Fatalf("Error connecting to DB: %v", err)
	}

	util.SetSecurityLoggerDB(db)

	// Initialize optional services (GeoIP, user email cache, Redis)
	initServices()
	defer util.CloseGeoIP()

	if err := migrateAndSeed(db); err != nil {
		log.Fatalf("Migration/seed failed: %v", err)
	}

	r := setupRouter(cfg, db)

	srv := createServer(cfg, r)

	go startServer(srv)

	waitForShutdown(srv, cfg, db)
}

func initJWT(cfg *config.Config) error {
	util.InitJWTSecretFromEnv()
	if cfg.AppEnv != "test" {
		return util.ValidateJWTSecret()
	}
	return nil
}

func setTimezone() error {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return err
	}
	time.Local = location
	return nil
}

func initDB() (*gorm.DB, error) {
	return config.ConnectMySQL()
}

func migrateAndSeed(db *gorm.DB) error {
	applyDiseaseCodenameMigrationFix(db)

	if err := db.AutoMigrate(&model.Patient{}, &model.Disease{}, &model.User{}, &model.Session{}, &model.Therapist{}, &model.Role{}, &model.Treatment{}, &model.Pricing{}, &model.Transaction{}, &model.PatientCode{}, &model.SecurityLog{}, &model.Item{}); err != nil {
		return err
	}

	runLegacyMigrations(db)

	return model.SeedRoles(db)
}

func applyDiseaseCodenameMigrationFix(db *gorm.DB) {
	// Pre-migration fix for diseases.codename to avoid unique-index failures.
	if !db.Migrator().HasTable(&model.Disease{}) {
		return
	}

	if err := db.Exec("ALTER TABLE diseases MODIFY codename varchar(191) NULL").Error; err != nil {
		log.Printf("Warning: failed to alter diseases.codename to NULL-able: %v", err)
	} else {
		log.Println("Converted diseases.codename to allow NULLs (if applicable)")
	}

	if err := db.Exec("UPDATE diseases SET codename = NULL WHERE codename = ''").Error; err != nil {
		log.Printf("Warning: failed to nullify empty codename values: %v", err)
	} else {
		log.Println("Nullified empty codename values in diseases table (if any)")
	}
}

func runLegacyMigrations(db *gorm.DB) {
	// Legacy column drops: only run when RUN_LEGACY_MIGRATIONS=true to avoid
	// table locks or unintended schema changes on every startup.
	if os.Getenv("RUN_LEGACY_MIGRATIONS") != "true" {
		return
	}

	dropLegacyColumn(db, &model.Pricing{}, "treatment_id", "pricings.treatment_id")
	dropLegacyColumn(db, &model.Transaction{}, "additional_charge", "transactions.additional_charge")
}

func dropLegacyColumn(db *gorm.DB, model interface{}, columnName, label string) {
	if !db.Migrator().HasColumn(model, columnName) {
		return
	}

	if err := db.Migrator().DropColumn(model, columnName); err != nil {
		log.Printf("Warning: failed to drop %s: %v", label, err)
		return
	}

	log.Printf("Dropped legacy %s column", label)
}

func setupRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.DatabaseMiddleware(db))
	r.Use(middleware.EndpointCallLogger())

	registerPublicRoutes(r, cfg)
	registerAuthenticatedRoutes(r, cfg)

	return r
}

func registerPublicRoutes(r *gin.Engine, cfg *config.Config) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Welcome to %s!", cfg.AppName)})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.POST("/patient", endpoint.CreatePatient)

	authRateLimit := middleware.RateLimiter(middleware.RateLimitConfig{Limit: 5, Window: 15 * time.Minute})
	r.POST("/login", authRateLimit, endpoint.Login)
	r.POST("/signup", authRateLimit, endpoint.Signup)
	r.GET("/token/validate", endpoint.ValidateToken)
}

func registerAuthenticatedRoutes(r *gin.Engine, cfg *config.Config) {
	auth := r.Group("/")
	auth.Use(middleware.ValidateLoginToken())

	auth.DELETE("/logout", endpoint.Logout)
	auth.PATCH("/user", endpoint.UpdateUser)
	auth.POST("/verify-password", endpoint.VerifyPassword)

	registerUserRoutes(auth)
	registerPatientRoutes(auth)
	registerTreatmentRoutes(auth)
	registerDiseaseRoutes(auth)
	registerPricingRoutes(auth)
	registerItemRoutes(auth)
	registerTransactionRoutes(auth)
	registerTherapistRoutes(auth)

	if cfg.AppEnv != "production" {
		auth.GET("/debug/dbinfo", middleware.RequireRole(model.RoleAdmin), endpoint.DebugDBInfo)
	}
}

func registerUserRoutes(auth *gin.RouterGroup) {
	userAdmin := auth.Group("/user")
	userAdmin.Use(middleware.RequireRole(model.RoleAdmin))
	userAdmin.GET("", endpoint.ListUsers)
	userAdmin.DELETE("/:id", endpoint.DeleteUser)

	auth.GET("/user/:id", middleware.RequireRoleOrOwner(model.RoleAdmin), endpoint.GetUserInfo)
	auth.PATCH("/user/:id", middleware.RequireRole(model.RoleAdmin), endpoint.UpdateUserByID)
}

func registerPatientRoutes(auth *gin.RouterGroup) {
	patient := auth.Group("/patient")
	patient.Use(middleware.RequireRole(model.RoleAdmin))
	patient.GET("", endpoint.ListPatients)
	patient.GET("/:id", endpoint.GetPatientInfo)
	patient.PATCH("/:id", endpoint.UpdatePatient)
	patient.DELETE("/:id", endpoint.DeletePatient)
}

func registerTreatmentRoutes(auth *gin.RouterGroup) {
	treatment := auth.Group("/treatment")
	treatment.Use(middleware.RequireRole(model.RoleAdmin, model.RoleTherapist))
	treatment.GET("", endpoint.ListTreatments)
	treatment.POST("", endpoint.CreateTreatment)
	treatment.PATCH("/:id", endpoint.UpdateTreatment)
	treatment.DELETE("/:id", endpoint.DeleteTreatment)
}

func registerDiseaseRoutes(auth *gin.RouterGroup) {
	disease := auth.Group("/disease")
	disease.Use(middleware.RequireRole(model.RoleAdmin))
	disease.GET("", endpoint.ListDiseases)
	disease.POST("", endpoint.CreateDisease)
	disease.GET("/:id", endpoint.GetDiseaseInfo)
	disease.PATCH("/:id", endpoint.UpdateDisease)
	disease.DELETE("/:id", endpoint.DeleteDisease)
}

func registerPricingRoutes(auth *gin.RouterGroup) {
	pricing := auth.Group("/pricing")
	pricing.Use(middleware.RequireRole(model.RoleAdmin))
	pricing.GET("", endpoint.ListPricings)
	pricing.POST("", endpoint.CreatePricing)
	pricing.GET("/:id", endpoint.GetPricingInfo)
	pricing.PATCH("/:id", endpoint.UpdatePricing)
	pricing.DELETE("/:id", endpoint.DeletePricing)
}

func registerItemRoutes(auth *gin.RouterGroup) {
	item := auth.Group("/item")
	item.Use(middleware.RequireRole(model.RoleAdmin))
	item.GET("", endpoint.ListItems)
	item.POST("", endpoint.CreateItem)
	item.GET("/:id", endpoint.GetItemInfo)
	item.PATCH("/:id", endpoint.UpdateItem)
	item.DELETE("/:id", endpoint.DeleteItem)
}

func registerTransactionRoutes(auth *gin.RouterGroup) {
	transaction := auth.Group("/transaction")
	transaction.Use(middleware.RequireRole(model.RoleAdmin))
	transaction.GET("", endpoint.ListTransactions)
	transaction.GET("/:id", endpoint.GetTransactionInfo)
	transaction.PATCH("/:id", endpoint.UpdateTransaction)
}

func registerTherapistRoutes(auth *gin.RouterGroup) {
	therapist := auth.Group("/therapist")
	therapist.GET("", middleware.RequireRole(model.RoleAdmin, model.RoleTherapist), endpoint.ListTherapist)
	therapist.GET("/:id", middleware.RequireRole(model.RoleAdmin, model.RoleTherapist), endpoint.GetTherapistInfo)
	therapist.POST("", middleware.RequireRole(model.RoleAdmin), endpoint.CreateTherapist)
	therapist.PATCH("/:id", middleware.RequireRole(model.RoleAdmin), endpoint.UpdateTherapist)
	therapist.DELETE("/:id", middleware.RequireRole(model.RoleAdmin), endpoint.DeleteTherapist)
	therapist.PUT("/:id", middleware.RequireRole(model.RoleAdmin), endpoint.TherapistApproval)
}

func createServer(cfg *config.Config, handler http.Handler) *http.Server {
	address := fmt.Sprintf(":%d", cfg.AppPort)
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func startServer(srv *http.Server) {
	address := srv.Addr
	enabled, cert, key := isTLSEnabled()
	if enabled {
		log.Printf("Starting HTTPS server on %s", address)
		if err := srv.ListenAndServeTLS(cert, key); err != nil && err != http.ErrServerClosed {
			log.Fatalf("error starting HTTPS server: %v", err)
		}
		return
	}

	log.Printf("Starting HTTP server on %s", address)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error starting HTTP server: %v", err)
	}
}

// isTLSEnabled returns whether TLS should be used and the cert/key paths
func isTLSEnabled() (bool, string, string) {
	cert := os.Getenv("TLS_CERT_FILE")
	key := os.Getenv("TLS_KEY_FILE")

	enabledEnv := os.Getenv("ENABLE_TLS") == "true"
	if !enabledEnv {
		return false, "", ""
	}

	// Ensure both certificate and key are provided
	if cert == "" || key == "" {
		return false, "", ""
	}

	return true, cert, key
}

// initServices initializes optional runtime services like GeoIP, user cache, and Redis.
func initServices() {
	if err := util.InitGeoIP(os.Getenv("GEOIP_DB_PATH")); err != nil {
		log.Printf("Warning: could not initialize GeoIP DB: %v", err)
	}

	util.InitUserEmailCacheFromEnv()

	if _, err := config.ConnectRedis(); err != nil {
		log.Printf("Warning: could not connect to Redis: %v", err)
	} else {
		log.Println("Redis initialization complete")
	}
}

func waitForShutdown(srv *http.Server, cfg *config.Config, db *gorm.DB) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ShutdownTimeout)*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	} else {
		log.Printf("Failed to get raw DB from GORM: %v", err)
	}

	log.Println("Server exiting")
}
