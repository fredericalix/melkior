package sim

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BackendAddr     string
	BackendToken    string
	SimLabelPrefix  string
	SimSeed         int64
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		BackendAddr:    getEnvOrDefault("BACKEND_ADDR", "localhost:50051"),
		BackendToken:   os.Getenv("BACKEND_TOKEN"),
		SimLabelPrefix: getEnvOrDefault("SIM_LABEL_PREFIX", "demo-sim/"),
	}

	seedStr := getEnvOrDefault("SIM_SEED", "")
	if seedStr == "" || seedStr == "random" {
		cfg.SimSeed = time.Now().UnixNano()
	} else {
		seed, err := strconv.ParseInt(seedStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid SIM_SEED: %w", err)
		}
		cfg.SimSeed = seed
	}

	return cfg, nil
}

func (c *Config) NewRand() *rand.Rand {
	return rand.New(rand.NewSource(c.SimSeed))
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}