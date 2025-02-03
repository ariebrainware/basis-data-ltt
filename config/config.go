package config

import (
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

// Config holds the application's configuration values.
type Config struct {
	AppName string
	AppEnv  string
	AppPort uint16
	GinMode string
	DBHost  string
	DBPort  uint16
	DBName  string
	DBUSER  string
	DBPass  string
}

var config *Config
var once sync.Once

// LoadConfig loads the environment variables from a .env file, and returns a singleton Config instance.
func LoadConfig() *Config {
	once.Do(func() {
		// Load environment variables from .env file.
		if err := godotenv.Load(); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}

		appPort, _ := strconv.ParseUint("APPPORT", 10, 16)
		dbPort, _ := strconv.ParseUint("DBPORT", 10, 16)

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
