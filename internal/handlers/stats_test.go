package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

func TestGetGlobalStats(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name           string
		mockFunc       func(ctx context.Context) (map[string]interface{}, error)
		expectedStatus int
	}{
		{
			name: "Success",
			mockFunc: func(ctx context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{"matches": 100}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Error",
			mockFunc: func(ctx context.Context) (map[string]interface{}, error) {
				return nil, context.DeadlineExceeded
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockServerStatsService{
				GetGlobalStatsFunc: tt.mockFunc,
			}

			h := &Handler{
				serverStats: mockService,
				logger:      logger,
			}

			req := httptest.NewRequest("GET", "/stats/global", nil)
			w := httptest.NewRecorder()

			h.GetGlobalStats(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name           string
		stat           string
		expectedStatus int
	}{
		{
			name:           "Default",
			stat:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid Stat Kills",
			stat:           "kills",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Injection Attempt",
			stat:           "kills; DROP TABLE users;",
			expectedStatus: http.StatusOK, // Should fallback to default or handle safely, not 500 or execute injection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock ClickHouse
			mockCH := &MockClickHouseConn{}

			h := &Handler{
				ch:     mockCH,
				logger: logger,
			}

			// Chi router to handle URL params
			r := chi.NewRouter()
			r.Get("/stats/leaderboard", h.GetLeaderboard)
			r.Get("/stats/leaderboard/{stat}", h.GetLeaderboard)

			path := "/stats/leaderboard"
			if tt.stat != "" {
				path += "/" + url.PathEscape(tt.stat)
			}
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestGetPlayerStats(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name           string
		guid           string
		mockFunc       func(ctx context.Context, guid string) (*models.DeepStats, error)
		expectedStatus int
	}{
		{
			name: "Success",
			guid: "12345",
			mockFunc: func(ctx context.Context, guid string) (*models.DeepStats, error) {
				return &models.DeepStats{
					Combat: models.CombatStats{Kills: 10},
				}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Service Error",
			guid: "error-guid",
			mockFunc: func(ctx context.Context, guid string) (*models.DeepStats, error) {
				return nil, fmt.Errorf("db error")
			},
			expectedStatus: http.StatusOK, // Endpoint currently swallows error and returns empty stats with error log
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockPlayerStatsService{
				GetDeepStatsFunc: tt.mockFunc,
			}
			mockCH := &MockClickHouseConn{}

			h := &Handler{
				playerStats: mockService,
				ch:          mockCH,
				logger:      logger,
			}

			r := chi.NewRouter()
			r.Get("/stats/player/{guid}", h.GetPlayerStats)

			req := httptest.NewRequest("GET", "/stats/player/"+tt.guid, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %v, want %v", w.Code, tt.expectedStatus)
			}
		})
	}
}
