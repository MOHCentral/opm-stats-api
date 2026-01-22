package db

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// NewPostgresPool creates a connection pool to PostgreSQL
func NewPostgresPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

// NewClickHouseConn creates a connection to ClickHouse
func NewClickHouseConn(ctx context.Context, connString string) (driver.Conn, error) {
	opts, err := clickhouse.ParseDSN(connString)
	if err != nil {
		return nil, err
	}

	// Increased pool size: 8 workers + multiple query handlers
	opts.MaxOpenConns = 50
	opts.MaxIdleConns = 20
	opts.ConnMaxLifetime = time.Hour
	opts.DialTimeout = 10 * time.Second // Increased from default 1s
	opts.Compression = &clickhouse.Compression{
		Method: clickhouse.CompressionLZ4,
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}

	return conn, nil
}

// NewRedisClient creates a Redis client
func NewRedisClient(connString string) *redis.Client {
	opt, _ := redis.ParseURL(connString)
	if opt == nil {
		opt = &redis.Options{
			Addr: "localhost:6379",
		}
	}

	opt.PoolSize = 50
	opt.MinIdleConns = 10
	opt.MaxRetries = 3
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	return redis.NewClient(opt)
}
