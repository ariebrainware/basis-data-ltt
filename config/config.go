package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds the application's configuration values.
type Config struct {
	AppName         string `json:"appname"`
	AppEnv          string `json:"appenv"`
	AppPort         uint16 `json:"appport"`
	GinMode         string `json:"ginmode"`
	ShutdownTimeout int    `json:"shutdowntimeout"`
	DBHost          string `json:"dbhost"`
	DBPort          uint16 `json:"dbport"`
	DBName          string `json:"dbname"`
	DBUSER          string `json:"dbuser"`
	DBPass          string `json:"dbpass"`
}

var config *Config
var once sync.Once

// LoadConfig loads the environment variables from a .env file, and returns a singleton Config instance.
func LoadConfig() *Config {
	once.Do(func() {
		// Load environment variables based on APPENV
		appEnv := os.Getenv("APPENV")
		if appEnv == "" {
			appEnv = "local"
		}

		var envFile string
		switch appEnv {
		case "local":
			envFile = ".env"
		case "development":
			envFile = ".env.dev2"
		case "production":
			// Production uses system environment variables
			envFile = ""
		default:
			envFile = ".env"
		}

		if envFile != "" {
			if err := godotenv.Load(envFile); err != nil {
				log.Printf("Error loading %s file: %v", envFile, err)
			}
		}

		appPort, _ := strconv.ParseUint(os.Getenv("APPPORT"), 10, 16)
		dbPort, _ := strconv.ParseUint(os.Getenv("DBPORT"), 10, 16)
		shutdownTimeoutStr := os.Getenv("SHUTDOWNTIMEOUT")
		shutdownTimeout, err := strconv.Atoi(shutdownTimeoutStr)
		if err != nil && shutdownTimeoutStr != "" {
			log.Printf("Invalid SHUTDOWNTIMEOUT value, using default (5 seconds): %v", err)
		}
		if shutdownTimeout <= 0 {
			shutdownTimeout = 5 // Default to 5 seconds if not specified or invalid
		}

		// Initialize the Config struct with values from environment variables.
		config = &Config{
			AppName:         os.Getenv("APPNAME"),
			AppEnv:          os.Getenv("APPENV"),
			AppPort:         uint16(appPort),
			GinMode:         os.Getenv("GINMODE"),
			ShutdownTimeout: shutdownTimeout,
			DBHost:          os.Getenv("DBHOST"),
			DBPort:          uint16(dbPort),
			DBName:          os.Getenv("DBNAME"),
			DBUSER:          os.Getenv("DBUSER"),
			DBPass:          os.Getenv("DBPASS"),
		}
	})
	return config
}

// ConnectMySQL establishes a connection to a MySQL database using the configuration values.
func ConnectMySQL() (*gorm.DB, error) {

	cfg := LoadConfig()
	// For tests, use an in-memory SQLite DB to avoid needing an external MySQL instance.
	if cfg.AppEnv == "test" {
		dsn := "file::memory:?cache=shared"
		gormConfig := &gorm.Config{}
		if cfg.AppEnv == "production" {
			gormConfig.Logger = logger.Default.LogMode(logger.Silent)
		} else {
			gormConfig = &gorm.Config{
				Logger: logger.New(
					log.New(os.Stdout, "\r\n", log.LstdFlags),
					logger.Config{
						SlowThreshold: 200 * time.Millisecond,
						LogLevel:      logger.Info,
						Colorful:      true,
					},
				),
			}
		}
		db, err := gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			return nil, err
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, err
		}

		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)

		return db, nil
	}
	// Build the Data Source Name (DSN) using the configuration values.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local", cfg.DBUSER, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	gormConfig := &gorm.Config{}
	if cfg.AppEnv == "production" {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	} else {
		gormConfig = &gorm.Config{
			Logger: logger.New(
				log.New(os.Stdout, "\r\n", log.LstdFlags),
				logger.Config{
					SlowThreshold: 200 * time.Millisecond, // Slow SQL threshold
					LogLevel:      logger.Info,            // Log level
					Colorful:      true,                   // Enable colorful output
				},
			),
		}
	}
	// Open a database connection.
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool limits to avoid too many connections.
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
