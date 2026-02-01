package handlers

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// MockCHConn implements a minimal driver.Conn for testing
type MockCHConn struct {
	driver.Conn
	QueryFunc func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error)
}

func (m *MockCHConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return nil, errors.New("not implemented")
}

func (m *MockCHConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	// Mock Row
	return &MockRow{}
}

type MockRow struct{}

func (m *MockRow) Scan(dest ...interface{}) error { return nil }
func (m *MockRow) ScanStruct(dest interface{}) error { return nil }
func (m *MockRow) Err() error                     { return nil }

func TestGetLeaderboard_SQLSafety(t *testing.T) {
	tests := []struct {
		name          string
		stat          string
		expectedOrder string
	}{
		{
			name:          "Valid Stat Kills",
			stat:          "kills",
			expectedOrder: "kills",
		},
		{
			name:          "Valid Stat Headshots",
			stat:          "headshots",
			expectedOrder: "headshots",
		},
		{
			name:          "Invalid Stat Injection",
			stat:          "kills; DROP TABLE raw_events",
			expectedOrder: "kills", // Should fallback to default or safe value
		},
		{
			name:          "Empty Stat",
			stat:          "",
			expectedOrder: "kills", // Default
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCH := &MockCHConn{
				QueryFunc: func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
					if !strings.Contains(query, "ORDER BY "+tt.expectedOrder) {
						t.Errorf("Query does not contain expected ORDER BY clause: %s\nQuery: %s", tt.expectedOrder, query)
					}
					// Return error to stop processing (we only care about query construction)
					return nil, errors.New("stop")
				},
			}

			h := &Handler{
				logger: logger.Sugar(),
				ch:     mockCH,
			}

			// Use url.PathEscape to prevent malformed URLs in test
			req := httptest.NewRequest("GET", "/stats/leaderboard/"+url.PathEscape(tt.stat), nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("stat", tt.stat)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			h.GetLeaderboard(w, req)
		})
	}
}
