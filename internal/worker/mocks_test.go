package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MockStatStore implements StatStore for testing
type MockStatStore struct {
	Stats             map[string]float64
	PublishedMessages []PublishedMessage
}

type PublishedMessage struct {
	Channel string
	Message interface{}
}

func NewMockStatStore() *MockStatStore {
	return &MockStatStore{
		Stats:             make(map[string]float64),
		PublishedMessages: make([]PublishedMessage, 0),
	}
}

func (m *MockStatStore) Incr(ctx context.Context, key string) (int64, error) {
	m.Stats[key]++
	return int64(m.Stats[key]), nil
}

func (m *MockStatStore) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	m.Stats[key] += value
	return m.Stats[key], nil
}

func (m *MockStatStore) Get(ctx context.Context, key string) (string, error) {
	val, ok := m.Stats[key]
	if !ok {
		return "", fmt.Errorf("redis: nil")
	}
	return fmt.Sprintf("%v", val), nil
}

func (m *MockStatStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	switch v := value.(type) {
	case int:
		m.Stats[key] = float64(v)
	case int64:
		m.Stats[key] = float64(v)
	case float64:
		m.Stats[key] = v
	default:
		// handle other types if needed
	}
	return nil
}

func (m *MockStatStore) Publish(ctx context.Context, channel string, message interface{}) error {
	m.PublishedMessages = append(m.PublishedMessages, PublishedMessage{
		Channel: channel,
		Message: message,
	})
	return nil
}

// MockClickHouseConn implements driver.Conn for testing
type MockClickHouseConn struct {
	driver.Conn
	QueryDuration time.Duration
}

func (m *MockClickHouseConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	if m.QueryDuration > 0 {
		time.Sleep(m.QueryDuration)
	}
	return &MockRow{}
}

func (m *MockClickHouseConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	if m.QueryDuration > 0 {
		time.Sleep(m.QueryDuration)
	}
	return &MockRows{}, nil
}

func (m *MockClickHouseConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &MockBatch{}, nil
}

type MockBatch struct{}

func (m *MockBatch) IsSent() bool {
	return false
}

func (m *MockBatch) Rows() int {
	return 0
}

func (m *MockBatch) Append(v ...interface{}) error {
	return nil
}

func (m *MockBatch) AppendStruct(v interface{}) error {
	return nil
}

func (m *MockBatch) Column(int) driver.BatchColumn {
	return nil
}

func (m *MockBatch) Send() error {
	return nil
}

func (m *MockBatch) Flush() error {
	return nil
}

func (m *MockBatch) Abort() error {
	return nil
}

// MockRow implements driver.Row
type MockRow struct{}

func (m *MockRow) Scan(dest ...interface{}) error {
	// Simulate returning 0 or some value
	// For benchmarking, we just want it to succeed
	for _, d := range dest {
		switch v := d.(type) {
		case *uint64:
			*v = 0 // Return 0 so no achievements unlocked
		case *int:
			*v = 0
		}
	}
	return nil
}

func (m *MockRow) ScanStruct(dest interface{}) error {
	return nil
}

func (m *MockRow) Err() error {
	return nil
}

// MockRows implements driver.Rows
type MockRows struct{}

func (m *MockRows) Next() bool {
	return false // No rows for definitions load
}

func (m *MockRows) Scan(dest ...interface{}) error {
	return nil
}

func (m *MockRows) ScanStruct(dest interface{}) error {
	return nil
}

func (m *MockRows) Close() error {
	return nil
}

func (m *MockRows) Err() error {
	return nil
}

func (m *MockRows) Columns() []string {
	return []string{}
}

func (m *MockRows) ColumnTypes() []driver.ColumnType {
	return []driver.ColumnType{}
}

func (m *MockRows) Totals(dest ...interface{}) error {
	return nil
}

// MockDBStore implements DBStore
type MockDBStore struct{}

func (m *MockDBStore) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &MockPGXRows{}, nil
}

func (m *MockDBStore) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &MockPGXRow{}
}

func (m *MockDBStore) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// MockPGXRow
type MockPGXRow struct{}

func (m *MockPGXRow) Scan(dest ...any) error {
	for _, d := range dest {
		switch v := d.(type) {
		case *int:
			*v = 1
		case *bool:
			*v = false
		}
	}
	return nil
}

// MockPGXRows
type MockPGXRows struct{}

func (m *MockPGXRows) Close() {}
func (m *MockPGXRows) Err() error { return nil }
func (m *MockPGXRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *MockPGXRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *MockPGXRows) Next() bool { return false }
func (m *MockPGXRows) Scan(dest ...any) error { return nil }
func (m *MockPGXRows) Values() ([]any, error) { return nil, nil }
func (m *MockPGXRows) RawValues() [][]byte { return nil }
func (m *MockPGXRows) Conn() *pgx.Conn { return nil }
