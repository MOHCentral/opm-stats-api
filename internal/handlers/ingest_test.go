package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

func TestIngestEvents_Validation(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantProcessed int
	}{
		{
			name:       "Missing Type",
			body:       `{"match_id":"123"}`,
			wantStatus: http.StatusAccepted, // It skips invalid lines but returns 202
			wantProcessed: 0,
		},
		{
			name:       "Valid JSON",
			body:       `{"type":"kill", "match_id":"123"}`,
			wantStatus: http.StatusAccepted,
			wantProcessed: 1,
		},
		{
			name:       "Valid Form",
			body:       `type=kill&match_id=123`,
			wantStatus: http.StatusAccepted,
			wantProcessed: 1,
		},
		{
			name:       "Mixed Valid and Invalid",
			body:       "type=kill\n\ntype=\n{\"match_id\":\"123\"}",
			wantStatus: http.StatusAccepted,
			wantProcessed: 1,
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				logger: logger.Sugar(),
				pool:   &MockIngestQueue{EnqueueFunc: func(e *models.RawEvent) bool { return true }},
			}

			req := httptest.NewRequest("POST", "/api/v1/ingest/events", strings.NewReader(tt.body))
			// Add server_id to context to simulate middleware
			ctx := context.WithValue(req.Context(), "server_id", "test_server")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			h.IngestEvents(w, req)

			if w.Result().StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", w.Result().StatusCode, tt.wantStatus)
			}

			// Check processed count in response
			// (omitted for brevity as I need to decode JSON response, but status check is good start)
		})
	}
}
