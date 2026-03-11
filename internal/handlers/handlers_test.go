package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/openmohaa/stats-api/internal/models"
)

// Mocks

type MockIngestQueue struct {
	EnqueueFunc func(event *models.RawEvent) bool
}

func (m *MockIngestQueue) Enqueue(event *models.RawEvent) bool {
	if m.EnqueueFunc != nil {
		return m.EnqueueFunc(event)
	}
	return true
}
func (m *MockIngestQueue) QueueDepth() int { return 0 }

type MockDBQuerier struct {
	QueryFunc    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRowFunc func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (m *MockDBQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return nil, nil
}
func (m *MockDBQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.QueryRowFunc != nil {
		return m.QueryRowFunc(ctx, sql, args...)
	}
	return &MockRow{}
}
func (m *MockDBQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *MockDBQuerier) Begin(ctx context.Context) (pgx.Tx, error) { return nil, nil }

type MockRedisClient struct {
	redis.Cmdable
}

type MockClickHouseConn struct {
	driver.Conn
	QueryFunc func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error)
}

func (m *MockClickHouseConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return &MockCHRows{}, nil
}

type MockRow struct {
	ScanFunc func(dest ...any) error
}

func (m *MockRow) Scan(dest ...any) error {
	if m.ScanFunc != nil {
		return m.ScanFunc(dest...)
	}
	return nil
}

type MockCHRows struct {
	driver.Rows
	NextFunc  func() bool
	ScanFunc  func(dest ...interface{}) error
	CloseFunc func() error
}

func (m *MockCHRows) Next() bool {
	if m.NextFunc != nil {
		return m.NextFunc()
	}
	return false
}
func (m *MockCHRows) Scan(dest ...interface{}) error {
	if m.ScanFunc != nil {
		return m.ScanFunc(dest...)
	}
	return nil
}
func (m *MockCHRows) Close() error { return nil }
func (m *MockCHRows) Err() error   { return nil }

// Tests

func TestIngestEvents_TableDriven(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		body           string
		mockEnqueue    func(*models.RawEvent) bool
		expectedStatus int
	}{
		{
			name:           "Valid JSON Event",
			body:           `[{"type": "kill", "match_id": "123"}]`,
			mockEnqueue:    func(e *models.RawEvent) bool { return true },
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "Invalid JSON Array",
			body:           `[{invalid json`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Validation Failure (Missing Type)",
			body:           `[{"match_id": "123"}]`,
			expectedStatus: http.StatusAccepted, // 202 because we drop invalid events but don't fail the whole batch unless JSON is invalid?
			// Wait, IngestEvents logs warn and continues if validation fails. It writes 202 Accepted.
			// Let's verify behavior.
		},
		{
			name:           "Queue Full",
			body:           `[{"type": "kill", "match_id": "123"}]`,
			mockEnqueue:    func(e *models.RawEvent) bool { return false },
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				logger: logger.Sugar(),
				pool:   &MockIngestQueue{EnqueueFunc: tt.mockEnqueue},
			}

			req := httptest.NewRequest("POST", "/api/v1/ingest/events", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.IngestEvents(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetLeaderboard_TableDriven(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		stat           string
		mockQuery      func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error)
		expectedStatus int
	}{
		{
			name: "Happy Path - Kills",
			stat: "kills",
			mockQuery: func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
				// Verify query contains "kills"
				if !strings.Contains(query, "sum(kills) AS kills") {
					return nil, errors.New("query missing expected metric")
				}
				return &MockCHRows{
					NextFunc: func() bool { return false }, // No rows for simplicity
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Database Error",
			stat: "kills",
			mockQuery: func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
				return nil, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCH := &MockClickHouseConn{QueryFunc: tt.mockQuery}

			h := &Handler{
				logger: logger.Sugar(),
				ch:     mockCH, // This needs to implement driver.Conn fully including QueryRow
			}

			// Add QueryRow to MockClickHouseConn
			// Wait, I can't dynamically add methods.
			// I need to update MockClickHouseConn definition above.

			req := httptest.NewRequest("GET", "/api/v1/stats/leaderboard?stat="+tt.stat, nil)
			w := httptest.NewRecorder()

			h.GetLeaderboard(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func (m *MockClickHouseConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return &MockRow{
		ScanFunc: func(dest ...any) error {
			for _, d := range dest {
				if v, ok := d.(*uint64); ok {
					*v = 100 // Mock total count
				}
			}
			return nil
		},
	}
}

func (m *MockRow) ScanStruct(dest interface{}) error { return nil }
func (m *MockRow) Err() error                        { return nil }

// Implement other driver.Conn methods for MockClickHouseConn
func (m *MockClickHouseConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) { return nil, nil }
func (m *MockClickHouseConn) Exec(ctx context.Context, query string, args ...interface{}) error { return nil }
func (m *MockClickHouseConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...interface{}) error { return nil }
func (m *MockClickHouseConn) Ping(ctx context.Context) error { return nil }
func (m *MockClickHouseConn) Stats() driver.Stats { return driver.Stats{} }
func (m *MockClickHouseConn) Close() error { return nil }
