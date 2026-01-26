package logic

import (
	"context"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// BenchMockRow implements driver.Row
type BenchMockRow struct {
	err error
}

func (m *BenchMockRow) Err() error {
	return m.err
}

func (m *BenchMockRow) Scan(dest ...any) error {
	return nil
}

func (m *BenchMockRow) ScanStruct(dest any) error {
	return nil
}

// BenchMockRows implements driver.Rows
type BenchMockRows struct {
	count int
	limit int
}

func (m *BenchMockRows) Next() bool {
	if m.count < m.limit {
		m.count++
		return true
	}
	return false
}

func (m *BenchMockRows) Scan(dest ...any) error {
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

func (m *BenchMockRows) ScanStruct(dest any) error        { return nil }
func (m *BenchMockRows) ColumnTypes() []driver.ColumnType { return nil }
func (m *BenchMockRows) Totals(dest ...any) error         { return nil }
func (m *BenchMockRows) Columns() []string                { return nil }
func (m *BenchMockRows) Close() error                     { return nil }
func (m *BenchMockRows) Err() error                       { return nil }

// BenchMockConn implements driver.Conn
type BenchMockConn struct {
	QueryCount int
	Latency    time.Duration
}

func (m *BenchMockConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	m.QueryCount++
	time.Sleep(m.Latency)
	return &BenchMockRow{}
}

func (m *BenchMockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	m.QueryCount++
	time.Sleep(m.Latency)
	return &BenchMockRows{limit: 2}, nil
}

// Stubs for other methods
func (m *BenchMockConn) Contributors() []string                                                { return nil }
func (m *BenchMockConn) ServerVersion() (*driver.ServerVersion, error)                         { return nil, nil }
func (m *BenchMockConn) Select(ctx context.Context, dest any, query string, args ...any) error { return nil }
func (m *BenchMockConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, nil
}
func (m *BenchMockConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (m *BenchMockConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (m *BenchMockConn) Ping(context.Context) error { return nil }
func (m *BenchMockConn) Stats() driver.Stats        { return driver.Stats{} }
func (m *BenchMockConn) Close() error               { return nil }

func BenchmarkGetFactionComparison(b *testing.B) {
	// Simulate 1ms latency per query
	mockConn := &BenchMockConn{
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
	mockConn := &BenchMockConn{}
	service := NewTeamStatsService(mockConn)
	ctx := context.Background()

	_, err := service.GetFactionComparison(ctx, 30)
	if err != nil {
		t.Fatalf("GetFactionComparison failed: %v", err)
	}

	t.Logf("Total queries executed: %d", mockConn.QueryCount)
}
