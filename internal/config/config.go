package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Server
	Port int
	Env  string

	// CORS
	AllowedOrigins []string

	// Database URLs
	PostgresURL   string
	ClickHouseURL string
	RedisURL      string

	// Worker pool
	WorkerCount   int
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration

	// Auth
	DeviceCodeTTL  time.Duration
	AccessTokenTTL time.Duration

	// Rate limiting
	RateLimitPerSecond int
	RateLimitBurst     int
}

// Load loads configuration from environment variables.
// It returns an error if critical configuration is missing.
func Load() (*Config, error) {
	cfg := &Config{
		Port: getEnvInt("PORT", 8080),
		Env:  getEnv("ENV", "development"),

		WorkerCount:   getEnvInt("WORKER_COUNT", 8),
		QueueSize:     getEnvInt("QUEUE_SIZE", 10000),
		BatchSize:     getEnvInt("BATCH_SIZE", 500),
		FlushInterval: getEnvDuration("FLUSH_INTERVAL", 1*time.Second),

		DeviceCodeTTL:  getEnvDuration("DEVICE_CODE_TTL", 10*time.Minute),
		AccessTokenTTL: getEnvDuration("ACCESS_TOKEN_TTL", 24*time.Hour),

		RateLimitPerSecond: getEnvInt("RATE_LIMIT_PER_SECOND", 100),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 200),
	}

	// CORS
	origins := getEnv("ALLOWED_ORIGINS", "http://localhost:3000")
	rawOrigins := strings.Split(origins, ",")
	for _, o := range rawOrigins {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
		}
	}

	// Critical configuration - fail if missing
	var err error
	if cfg.PostgresURL, err = getEnvRequired("POSTGRES_URL"); err != nil {
		return nil, err
	}
	if cfg.ClickHouseURL, err = getEnvRequired("CLICKHOUSE_URL"); err != nil {
		return nil, err
	}
	if cfg.RedisURL, err = getEnvRequired("REDIS_URL"); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvRequired(key string) (string, error) {
	if value := os.Getenv(key); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("missing required environment variable: %s", key)
}

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}
