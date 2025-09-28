package config

import (
	"fmt"
	"os"
)

type Config struct {
	RedisAddr     string
	RedisDB       int
	RedisPassword string
	GRPCAddr      string
	HTTPAddr      string
	AdminToken    string
	LogLevel      string
}

func Load() (*Config, error) {
	cfg := &Config{
		RedisAddr:  getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisDB:    0,
		HTTPAddr:   getHTTPAddr(),
		GRPCAddr:   getEnvOrDefault("GRPC_ADDR", ":50051"),
		LogLevel:   getEnvOrDefault("LOG_LEVEL", "info"),
	}

	redisDB := os.Getenv("REDIS_DB")
	if redisDB != "" {
		var db int
		if _, err := fmt.Sscanf(redisDB, "%d", &db); err == nil {
			cfg.RedisDB = db
		}
	}

	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")

	cfg.AdminToken = os.Getenv("ADMIN_TOKEN")
	if cfg.AdminToken == "" {
		return nil, fmt.Errorf("ADMIN_TOKEN environment variable is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getHTTPAddr() string {
	// Check PORT env var first (common in cloud environments)
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	// Fall back to HTTP_ADDR
	return getEnvOrDefault("HTTP_ADDR", ":8080")
}