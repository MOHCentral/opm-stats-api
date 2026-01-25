package logic

import (
	"context"
	"reflect"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type MockConn struct {
	driver.Conn
	QueryCalls    int
	QueryRowCalls int
}

func (m *MockConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	m.QueryCalls++
	return &MockRows{
		callIndex: m.QueryCalls,
		rowIndex:  0,
	}, nil
}

func (m *MockConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	m.QueryRowCalls++
	return &MockRow{}
}

type MockRows struct {
	driver.Rows
	callIndex int
	rowIndex  int
}

func (m *MockRows) Next() bool {
	m.rowIndex++
	// Return true for 2 rows
	return m.rowIndex <= 2
}

func (m *MockRows) Scan(dest ...interface{}) error {
	// Map Stats is the first query
	if m.callIndex == 1 {
		// Map Stats: map_name, plays, avg_dur, avg_players, kills, peak_hour, popularity
		mapName := "map1"
		if m.rowIndex == 2 {
			mapName = "map2"
		}

		assign(dest[0], mapName)
		assign(dest[1], int64(10))
		assign(dest[2], float64(10.0))
		assign(dest[3], float64(5.0))
		assign(dest[4], int64(100))
		assign(dest[5], int(20))
		assign(dest[6], float64(50.0))
	} else if m.callIndex == 2 {
		// Rotation Pattern: map_name
		assign(dest[0], "map_pattern")
	} else {
		// Transitions: next_map, prob
		// Note: The N+1 optimization will merge this.
		// If we are optimized, callIndex might be different or we handle it differently.
		// For now this supports the existing loop behavior.

		// If optimized, callIndex 3 might be "Transitions for all maps"
		// which scans: map_name, next_map, prob

		if len(dest) == 3 {
			// Optimized query
			assign(dest[0], "map1") // map_name
			assign(dest[1], "next_map") // next_map
			assign(dest[2], float64(0.5)) // prob
		} else {
			// Legacy loop query
			assign(dest[0], "next_map")
			assign(dest[1], float64(0.5))
		}
	}
	return nil
}

func (m *MockRows) Close() error {
	return nil
}

func (m *MockRows) Err() error {
	return nil
}

type MockRow struct {
	driver.Row
}

func (m *MockRow) Scan(dest ...interface{}) error {
	// Avg maps per day: float64
	assign(dest[0], float64(1.0))
	return nil
}

func (m *MockRow) Err() error {
	return nil
}

func assign(dest interface{}, val interface{}) {
	// Simple reflection to assign value to pointer
	v := reflect.ValueOf(dest).Elem()
	v.Set(reflect.ValueOf(val))
}
