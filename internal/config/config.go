// Package config provides configuration management for the application.
package config

import (
	"errors"
	"os"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	Port             string
	AllowedOrigin    string
	AWSRegion        string
	S3Bucket         string
	CloudfrontDomain string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		AllowedOrigin:    getEnv("ALLOWED_ORIGIN", "http://localhost:5173"),
		AWSRegion:        getEnv("AWS_REGION", "ap-northeast-1"),
		S3Bucket:         getEnv("S3_BUCKET", ""),
		CloudfrontDomain: getEnv("CLOUDFRONT_DOMAIN", ""),
	}

	return cfg, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate port is a number
	if _, err := strconv.Atoi(c.Port); err != nil {
		return errors.New("invalid port: must be a number")
	}

	return nil
}

// getEnv returns the value of an environment variable or a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
