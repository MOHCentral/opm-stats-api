package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

// MockIngestQueue implements IngestQueue for testing
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

func TestIngestEvents(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		mockEnqueue func(*models.RawEvent) bool
		wantStatus  int
	}{
		{
			name:        "Oversized Payload",
			body:        strings.Repeat("a", MaxBodySize+1),
			mockEnqueue: nil, // Should not be called
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name:        "Valid Payload",
			body:        "type=kill&match_id=123&server_id=srv1&timestamp=1620000000",
			mockEnqueue: func(e *models.RawEvent) bool { return true },
			wantStatus:  http.StatusAccepted,
		},
		{
			name:        "Invalid Payload",
			body:        "type=kill", // Missing required fields
			mockEnqueue: func(e *models.RawEvent) bool { panic("should not be called") },
			wantStatus:  http.StatusAccepted,
		},
		{
			name:        "Queue Full",
			body:        "type=kill&match_id=123&server_id=srv1&timestamp=1620000000",
			mockEnqueue: func(e *models.RawEvent) bool { return false },
			wantStatus:  http.StatusAccepted,
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{
				logger:   logger.Sugar(),
				validate: validator.New(),
				pool:     &MockIngestQueue{EnqueueFunc: tt.mockEnqueue},
			}

			req := httptest.NewRequest("POST", "/api/v1/ingest/events", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.IngestEvents(w, req)

			if w.Result().StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", w.Result().StatusCode, tt.wantStatus)
			}
		})
	}
}
