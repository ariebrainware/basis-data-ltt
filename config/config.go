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
	"gorm.io/gorm"
)

// Config holds the application's configuration values.
type Config struct {
	AppName string `json:"appname"`
	AppEnv  string `json:"appenv"`
	AppPort uint16 `json:"appport"`
	GinMode string `json:"ginmode"`
	DBHost  string `json:"dbhost"`
	DBPort  uint16 `json:"dbport"`
	DBName  string `json:"dbname"`
	DBUSER  string `json:"dbuser"`
	DBPass  string `json:"dbpass"`
}

var config *Config
var once sync.Once

// LoadConfig loads the environment variables from a .env file, and returns a singleton Config instance.
func LoadConfig() *Config {
	once.Do(func() {
		// Only load environment variables from .env when running in local mode.
		if os.Getenv("APPENV") == "local" {
			if err := godotenv.Load(); err != nil {
				log.Printf("Error loading .env file: %v", err)
			}
		}

		appPort, _ := strconv.ParseUint(os.Getenv("APPPORT"), 10, 16)
		dbPort, _ := strconv.ParseUint(os.Getenv("DBPORT"), 10, 16)

		// Initialize the Config struct with values from environment variables.
		config = &Config{
			AppName: os.Getenv("APPNAME"),
			AppEnv:  os.Getenv("APPENV"),
			AppPort: uint16(appPort),
			GinMode: os.Getenv("GINMODE"),
			DBHost:  os.Getenv("DBHOST"),
			DBPort:  uint16(dbPort),
			DBName:  os.Getenv("DBNAME"),
			DBUSER:  os.Getenv("DBUSER"),
			DBPass:  os.Getenv("DBPASS"),
		}
	})
	return config
}

// ConnectMySQL establishes a connection to a MySQL database using the configuration values.
func ConnectMySQL() (*gorm.DB, error) {
	cfg := LoadConfig()
	// Build the Data Source Name (DSN) using the configuration values.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local", cfg.DBUSER, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	// Open a database connection.
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
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
