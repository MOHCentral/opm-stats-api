package logic

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// MockRow implements driver.Row
type MockRow struct {
	err error
}

func (m *MockRow) Err() error {
	return m.err
}

func (m *MockRow) Scan(dest ...any) error {
	return nil
}

func (m *MockRow) ScanStruct(dest any) error {
	return nil
}

// MockRows implements driver.Rows
type MockRows struct {
	count int
	limit int
}

func (m *MockRows) Next() bool {
	if m.count < m.limit {
		m.count++
		return true
	}
	return false
}

func (m *MockRows) Scan(dest ...any) error {
	// For top weapon query, we expect 2 args: team, weapon
	if len(dest) >= 2 {
		if teamPtr, ok := dest[0].(*string); ok {
			*teamPtr = "axis" // simplify for benchmark
		}
		if weaponPtr, ok := dest[1].(*string); ok {
			*weaponPtr = "mp40"
		}
	}
	return nil
}

func (m *MockRows) ScanStruct(dest any) error        { return nil }
func (m *MockRows) ColumnTypes() []driver.ColumnType { return nil }
func (m *MockRows) Totals(dest ...any) error         { return nil }
func (m *MockRows) Columns() []string                { return nil }
func (m *MockRows) Close() error                     { return nil }
func (m *MockRows) Err() error                       { return nil }

// MockConn implements driver.Conn
type MockConn struct {
	QueryCount int
	Latency    time.Duration
}

func (m *MockConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	m.QueryCount++
	time.Sleep(m.Latency)
	return &MockRow{}
}

func (m *MockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	m.QueryCount++
	time.Sleep(m.Latency)
	return &MockRows{limit: 2}, nil
}

// Stubs for other methods
func (m *MockConn) Contributors() []string                                                { return nil }
func (m *MockConn) ServerVersion() (*driver.ServerVersion, error)                         { return nil, nil }
func (m *MockConn) Select(ctx context.Context, dest any, query string, args ...any) error { return nil }
func (m *MockConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, nil
}
func (m *MockConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (m *MockConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (m *MockConn) Ping(context.Context) error { return nil }
func (m *MockConn) Stats() driver.Stats        { return driver.Stats{} }
func (m *MockConn) Close() error               { return nil }

func BenchmarkGetFactionComparison(b *testing.B) {
	// Simulate 1ms latency per query
	mockConn := &MockConn{
		Latency: 1 * time.Millisecond,
	}
	service := NewTeamStatsService(mockConn)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GetFactionComparison(ctx, 30)
		if err != nil {
			b.Fatalf("GetFactionComparison failed: %v", err)
		}
	}
}

func TestGetFactionComparisonQueryCount(t *testing.T) {
	mockConn := &MockConn{}
	service := NewTeamStatsService(mockConn)
	ctx := context.Background()

	_, err := service.GetFactionComparison(ctx, 30)
	if err != nil {
		t.Fatalf("GetFactionComparison failed: %v", err)
	}

	t.Logf("Total queries executed: %d", mockConn.QueryCount)
}
