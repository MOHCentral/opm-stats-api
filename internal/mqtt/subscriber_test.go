package mqtt

import (
	"encoding/json"
	"testing"

	"github.com/openmohaa/stats-api/internal/models"
)

func TestExtractServerID(t *testing.T) {
	tests := []struct {
		topic    string
		prefix   string
		expected string
	}{
		{"openmohaa/events/server-123", "openmohaa/events/", "server-123"},
		{"openmohaa/events/my-server/extra", "openmohaa/events/", "my-server"},
		{"openmohaa/events/", "openmohaa/events/", ""},
		{"openmohaa/servers/srv1/status", "openmohaa/servers/", "srv1"},
		{"other/topic", "openmohaa/events/", ""},
	}

	for _, tc := range tests {
		got := extractServerID(tc.topic, tc.prefix)
		if got != tc.expected {
			t.Errorf("extractServerID(%q, %q) = %q, want %q", tc.topic, tc.prefix, got, tc.expected)
		}
	}
}

func TestParsePayload_SingleObject(t *testing.T) {
	payload := `{"type":"player_kill","match_id":"m1","player_name":"Alice","player_guid":"g1"}`
	events, err := parsePayload([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "player_kill" {
		t.Errorf("expected type player_kill, got %s", events[0].Type)
	}
	if events[0].MatchID != "m1" {
		t.Errorf("expected match_id m1, got %s", events[0].MatchID)
	}
}

func TestParsePayload_JSONArray(t *testing.T) {
	events := []map[string]string{
		{"type": "player_kill", "match_id": "m1", "player_guid": "g1"},
		{"type": "player_death", "match_id": "m1", "player_guid": "g2"},
		{"type": "player_damage", "match_id": "m1", "player_guid": "g3"},
	}
	payload, _ := json.Marshal(events)
	parsed, err := parsePayload(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 events, got %d", len(parsed))
	}
	if parsed[0].Type != "player_kill" {
		t.Errorf("expected player_kill, got %s", parsed[0].Type)
	}
	if parsed[1].Type != "player_death" {
		t.Errorf("expected player_death, got %s", parsed[1].Type)
	}
}

func TestParsePayload_Empty(t *testing.T) {
	events, err := parsePayload([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParsePayload_Whitespace(t *testing.T) {
	events, err := parsePayload([]byte("   \n  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	_, err := parsePayload([]byte(`{"type":"broken`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePayload_InvalidArray(t *testing.T) {
	_, err := parsePayload([]byte(`[{"type":"ok"},broken]`))
	if err == nil {
		t.Fatal("expected error for invalid JSON array")
	}
}

func TestParsePayload_UnknownFormat(t *testing.T) {
	_, err := parsePayload([]byte(`type=player_kill&match_id=m1`))
	if err == nil {
		t.Fatal("expected error for non-JSON format")
	}
}

func TestPayloadSizeGuard(t *testing.T) {
	// Verify MaxPayloadSize is defined and reasonable
	if MaxPayloadSize != 1048576 {
		t.Errorf("expected MaxPayloadSize 1048576, got %d", MaxPayloadSize)
	}
}

// mockQueue implements handlers.IngestQueue for testing
type mockQueue struct {
	events []*models.RawEvent
}

func (m *mockQueue) Enqueue(event *models.RawEvent) bool {
	m.events = append(m.events, event)
	return true
}

func (m *mockQueue) QueueDepth() int {
	return len(m.events)
}
