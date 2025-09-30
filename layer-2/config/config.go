package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for an L2 shard
type Config struct {
	// Shard Identity
	ShardID     string
	ClientGroup string
	L2NodeID    string

	// Server Configuration
	HTTPPort string

	// Database Configuration
	DatabaseHost string
	DatabasePort string
	DatabaseUser string
	DatabasePass string
	DatabaseName string

	// L1 Configuration
	L1Endpoint string // e.g., "http://localhost:5000"
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		// Shard Identity - REQUIRED
		ShardID:     getEnv("SHARD_ID", "shard-a"),
		ClientGroup: getEnv("CLIENT_GROUP", "group-a"),
		L2NodeID:    getEnv("L2_NODE_ID", "l2-node-a"),

		// Server
		HTTPPort: getEnv("HTTP_PORT", "6000"),

		// Database
		DatabaseHost: getEnv("DB_HOST", "localhost"),
		DatabasePort: getEnv("DB_PORT", "5433"),
		DatabaseUser: getEnv("DB_USER", "postgres"),
		DatabasePass: getEnv("DB_PASS", "postgrespassword"),
		DatabaseName: getEnv("DB_NAME", "l2_shard_db"),

		// L1
		L1Endpoint: getEnv("L1_ENDPOINT", "http://localhost:5000"),
	}
}

// GetDSN returns the PostgreSQL connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DatabaseHost,
		c.DatabasePort,
		c.DatabaseUser,
		c.DatabasePass,
		c.DatabaseName,
	)
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.ShardID == "" {
		return fmt.Errorf("SHARD_ID is required")
	}
	if c.ClientGroup == "" {
		return fmt.Errorf("CLIENT_GROUP is required")
	}
	if c.L2NodeID == "" {
		return fmt.Errorf("L2_NODE_ID is required")
	}
	if c.L1Endpoint == "" {
		return fmt.Errorf("L1_ENDPOINT is required")
	}
	return nil
}

// Helper function to get environment variable with default
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
