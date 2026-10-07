// Package config loads service configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the API service.
type Config struct {
	Port         string
	JWTSecret    string
	TokenTTL     time.Duration
	DatabaseURL  string // empty means use the in-memory store
	RateLimitRPS float64
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		JWTSecret:    getEnv("JWT_SECRET", "dev-only-change-me"),
		TokenTTL:     getDurationEnv("TOKEN_TTL", 24*time.Hour),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RateLimitRPS: getFloatEnv("RATE_LIMIT_RPS", 10),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getFloatEnv(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
