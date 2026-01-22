package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port int
	Env  string

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
	JWTSecret      string
	DeviceCodeTTL  time.Duration
	AccessTokenTTL time.Duration

	// Rate limiting
	RateLimitPerSecond int
	RateLimitBurst     int
}

func Load() *Config {
	return &Config{
		Port: getEnvInt("PORT", 8080),
		Env:  getEnv("ENV", "development"),

		PostgresURL:   getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/mohaa_stats?sslmode=disable"),
		ClickHouseURL: getEnv("CLICKHOUSE_URL", "clickhouse://localhost:9000/mohaa_stats"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379/0"),

		WorkerCount:   getEnvInt("WORKER_COUNT", 8),
		QueueSize:     getEnvInt("QUEUE_SIZE", 10000),
		BatchSize:     getEnvInt("BATCH_SIZE", 500),
		FlushInterval: getEnvDuration("FLUSH_INTERVAL", 1*time.Second),

		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-in-production"),
		DeviceCodeTTL:  getEnvDuration("DEVICE_CODE_TTL", 10*time.Minute),
		AccessTokenTTL: getEnvDuration("ACCESS_TOKEN_TTL", 24*time.Hour),

		RateLimitPerSecond: getEnvInt("RATE_LIMIT_PER_SECOND", 100),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", 200),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
