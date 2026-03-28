package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// MockCH captures queries for verification
type MockCH struct {
	driver.Conn
	CapturedQuery string
	CapturedArgs  []interface{}
}

func (m *MockCH) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	m.CapturedQuery = query
	m.CapturedArgs = args
	return &EmptyRows{}, nil
}

func (m *MockCH) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return &EmptyRow{}
}

type EmptyRows struct {
	driver.Rows
}

func (m *EmptyRows) Next() bool { return false }
func (m *EmptyRows) Close() error { return nil }

type EmptyRow struct{}

func (m *EmptyRow) Scan(dest ...interface{}) error { return nil }
func (m *EmptyRow) ScanStruct(dest interface{}) error { return nil }
func (m *EmptyRow) Err() error { return nil }

func TestGetLeaderboard_SQLInjection(t *testing.T) {
	tests := []struct {
		name          string
		statParam     string
		wantOrderExpr string // Expected ORDER BY clause part
	}{
		{
			name:          "Valid Stat Kills",
			statParam:     "kills",
			wantOrderExpr: "ORDER BY kills DESC",
		},
		{
			name:          "Valid Stat Headshots",
			statParam:     "headshots",
			wantOrderExpr: "ORDER BY headshots DESC",
		},
		{
			name:          "Injection Attempt 1",
			statParam:     "kills; DROP TABLE users;",
			wantOrderExpr: "ORDER BY kills DESC", // Should fallback to default
		},
		{
			name:          "Invalid Stat",
			statParam:     "invalid_stat_name",
			wantOrderExpr: "ORDER BY kills DESC", // Should fallback to default
		},
		{
			name:          "Complex Expr",
			statParam:     "accuracy",
			wantOrderExpr: "ORDER BY shots_hit / nullIf(shots_fired, 0) DESC",
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCH := &MockCH{}
			h := &Handler{
				ch:     mockCH,
				logger: logger.Sugar(),
			}

			// Setup router to parse URL params
			r := chi.NewRouter()
			r.Get("/stats/leaderboard/{stat}", h.GetLeaderboard)

			req := httptest.NewRequest("GET", "/stats/leaderboard/"+url.PathEscape(tt.statParam), nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Verify status code (should be 200 even with empty rows)
			if w.Result().StatusCode != http.StatusOK {
				t.Errorf("StatusCode = %d, want %d", w.Result().StatusCode, http.StatusOK)
			}

			// Verify Query Construction
			if !strings.Contains(mockCH.CapturedQuery, tt.wantOrderExpr) {
				t.Errorf("Query expected to contain %q, got \n%s", tt.wantOrderExpr, mockCH.CapturedQuery)
			}
		})
	}
}
