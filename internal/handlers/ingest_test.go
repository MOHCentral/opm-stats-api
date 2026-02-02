package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

// MockIngestQueue implements IngestQueue for testing
type MockIngestQueue struct {
	EnqueueFunc    func(event *models.RawEvent) bool
	QueueDepthFunc func() int
}

func (m *MockIngestQueue) Enqueue(event *models.RawEvent) bool {
	if m.EnqueueFunc != nil {
		return m.EnqueueFunc(event)
	}
	return true
}

func (m *MockIngestQueue) QueueDepth() int {
	if m.QueueDepthFunc != nil {
		return m.QueueDepthFunc()
	}
	return 0
}

func TestIngestEvents(t *testing.T) {
	logger := zap.NewNop().Sugar()
	v := validator.New()

	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		setupMock      func(*MockIngestQueue)
		expectedStatus int
	}{
		{
			name:        "Valid JSON Event",
			method:      "POST",
			contentType: "application/json",
			body: `{"type": "kill", "match_id": "123", "server_id": "srv1", "timestamp": 1620000000}
`,
			setupMock: func(m *MockIngestQueue) {
				m.EnqueueFunc = func(event *models.RawEvent) bool {
					return event.Type == "kill" && event.MatchID == "123"
				}
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:        "Valid URL-Encoded Event",
			method:      "POST",
			contentType: "application/x-www-form-urlencoded",
			body:        "type=kill&match_id=123&server_id=srv1&timestamp=1620000000",
			setupMock: func(m *MockIngestQueue) {
				m.EnqueueFunc = func(event *models.RawEvent) bool {
					return event.Type == "kill" && event.MatchID == "123"
				}
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:        "Invalid Event (Missing MatchID)",
			method:      "POST",
			contentType: "application/json",
			body:        `{"type": "kill", "server_id": "srv1", "timestamp": 1620000000}`,
			setupMock: func(m *MockIngestQueue) {
				m.EnqueueFunc = func(event *models.RawEvent) bool {
					t.Errorf("Should not enqueue invalid event")
					return true
				}
			},
			expectedStatus: http.StatusAccepted, // It's accepted but skipped/logged in current implementation
		},
		{
			name:        "Queue Full",
			method:      "POST",
			contentType: "application/json",
			body:        `{"type": "kill", "match_id": "123", "server_id": "srv1", "timestamp": 1620000000}`,
			setupMock: func(m *MockIngestQueue) {
				m.EnqueueFunc = func(event *models.RawEvent) bool {
					return false // Queue full
				}
			},
			expectedStatus: http.StatusAccepted, // Still returns 202, logic handles full queue by dropping
		},
		{
			name:        "Oversized Payload",
			method:      "POST",
			contentType: "application/json",
			body:        strings.Repeat("a", MaxBodySize+1),
			setupMock:   nil,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := &MockIngestQueue{}
			if tt.setupMock != nil {
				tt.setupMock(mockQueue)
			}

			h := &Handler{
				pool:      mockQueue,
				logger:    logger,
				validator: v,
			}

			req := httptest.NewRequest(tt.method, "/api/v1/ingest/events", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			// Simulate ServerAuthMiddleware injecting server_id
			// context.WithValue(req.Context(), "server_id", "srv1")
			// But here we can just pass it directly if we want, or rely on payload.
			// The handler checks r.Context().Value("server_id")

			w := httptest.NewRecorder()
			h.IngestEvents(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("IngestEvents() status = %v, want %v", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestParseFormToEvent(t *testing.T) {
	h := &Handler{}

	form := url.Values{}
	form.Set("type", "kill")
	form.Set("match_id", "123")
	form.Set("damage", "100")
	form.Set("timestamp", "1234567890.123")

	event, err := h.parseFormToEvent(form)
	if err != nil {
		t.Fatalf("parseFormToEvent failed: %v", err)
	}

	if event.Type != "kill" {
		t.Errorf("expected type kill, got %v", event.Type)
	}
	if event.Damage != 100 {
		t.Errorf("expected damage 100, got %v", event.Damage)
	}
	if event.Timestamp != 1234567890.123 {
		t.Errorf("expected timestamp 1234567890.123, got %v", event.Timestamp)
	}
}
