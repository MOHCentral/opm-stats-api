package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
	"github.com/openmohaa/stats-api/internal/testutils"
	"go.uber.org/zap"
)

// MockPlayerStatsService implements logic.PlayerStatsService for testing
type MockPlayerStatsService struct {
	GetDeepStatsFunc func(ctx context.Context, guid string) (*models.DeepStats, error)
}

func (m *MockPlayerStatsService) GetDeepStats(ctx context.Context, guid string) (*models.DeepStats, error) {
	if m.GetDeepStatsFunc != nil {
		return m.GetDeepStatsFunc(ctx, guid)
	}
	return &models.DeepStats{}, nil
}

// Stubs for interface compliance
func (m *MockPlayerStatsService) ResolvePlayerGUID(ctx context.Context, name string) (string, error) {
	return "", nil
}
func (m *MockPlayerStatsService) GetPlayerStatsByGametype(ctx context.Context, guid string) ([]models.GametypeStats, error) {
	return nil, nil
}
func (m *MockPlayerStatsService) GetPlayerStatsByMap(ctx context.Context, guid string) ([]models.PlayerMapStats, error) {
	return nil, nil
}

func TestGetPlayerStats(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		guid           string
		mockSetup      func(*MockPlayerStatsService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Happy Path",
			guid: "test-guid-123",
			mockSetup: func(m *MockPlayerStatsService) {
				m.GetDeepStatsFunc = func(ctx context.Context, guid string) (*models.DeepStats, error) {
					return &models.DeepStats{
						Combat: models.CombatStats{Kills: 10, Deaths: 5},
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"kills":10`,
		},
		{
			name: "Service Error (Deep Stats Fallback)",
			guid: "error-guid",
			mockSetup: func(m *MockPlayerStatsService) {
				m.GetDeepStatsFunc = func(ctx context.Context, guid string) (*models.DeepStats, error) {
					return nil, errors.New("db error")
				}
			},
			expectedStatus: http.StatusOK, // Handler falls back to empty stats but returns 200
			expectedBody:   `"kills":0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Mocks
			mockService := &MockPlayerStatsService{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			}

			// Mock ClickHouse using the shared mock from worker package
			// Note: MockRows.Next() returns false, so the direct queries in the handler return 0 rows.
			mockCH := &testutils.MockClickHouseConn{}

			h := &Handler{
				playerStats: mockService,
				ch:          mockCH,
				logger:      logger.Sugar(),
			}

			r := httptest.NewRequest("GET", "/stats/player/"+tt.guid, nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("guid", tt.guid)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()

			h.GetPlayerStats(w, r)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" {
				if !strings.Contains(w.Body.String(), tt.expectedBody) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
				}
			}
		})
	}
}
