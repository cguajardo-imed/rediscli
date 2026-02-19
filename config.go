package main

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the Redis configuration
type Config struct {
	Host         string
	Port         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() (*Config, error) {
	config := &Config{
		PoolSize:     200,
		MinIdleConns: 50,
	}

	// Host is required
	config.Host = os.Getenv("REDIS_HOST")
	if config.Host == "" {
		return nil, fmt.Errorf("REDIS_HOST environment variable is required")
	}

	// Port with default
	config.Port = os.Getenv("REDIS_PORT")
	if config.Port == "" {
		config.Port = "6379"
	}

	// Password is optional
	config.Password = os.Getenv("REDIS_PASSWORD")

	// DB with default
	dbStr := os.Getenv("REDIS_DB")
	if dbStr == "" {
		config.DB = 0
	} else {
		db, err := strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_DB value: %w", err)
		}
		if db < 0 || db > 15 {
			return nil, fmt.Errorf("REDIS_DB must be between 0 and 15, got: %d", db)
		}
		config.DB = db
	}

	// Pool size with optional override
	if poolStr := os.Getenv("REDIS_POOL_SIZE"); poolStr != "" {
		poolSize, err := strconv.Atoi(poolStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_POOL_SIZE value: %w", err)
		}
		if poolSize > 0 {
			config.PoolSize = poolSize
		}
	}

	// Min idle connections with optional override
	if minIdleStr := os.Getenv("REDIS_MIN_IDLE_CONNS"); minIdleStr != "" {
		minIdle, err := strconv.Atoi(minIdleStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_MIN_IDLE_CONNS value: %w", err)
		}
		if minIdle > 0 {
			config.MinIdleConns = minIdle
		}
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	if c.DB < 0 || c.DB > 15 {
		return fmt.Errorf("database must be between 0 and 15")
	}
	if c.PoolSize <= 0 {
		return fmt.Errorf("pool size must be greater than 0")
	}
	if c.MinIdleConns < 0 {
		return fmt.Errorf("min idle connections cannot be negative")
	}
	return nil
}

// Address returns the Redis address in host:port format
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// String returns a string representation of the config (without password)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Host=%s Port=%s DB=%d PoolSize=%d MinIdleConns=%d",
		c.Host, c.Port, c.DB, c.PoolSize, c.MinIdleConns,
	)
}
