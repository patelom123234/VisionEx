package env

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Load loads environment variables from .env file
func Load() {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found")
	}
}

// RequiredStringVariable returns the value of an environment variable or panics if not set
func RequiredStringVariable(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", name))
	}
	return value
}

// RequiredIntVariable returns the value of an environment variable as int or panics if not set
func RequiredIntVariable(name string) int {
	value := RequiredStringVariable(name)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("environment variable %s must be an integer, got: %s", name, value))
	}
	return intValue
}

// StringVariable returns the value of an environment variable or a default value
func StringVariable(name, defaultValue string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return defaultValue
}
