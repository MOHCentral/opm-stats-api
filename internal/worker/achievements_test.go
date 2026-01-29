package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
)

type mockConn struct {
	driver.Conn
	queryRowFunc func(ctx context.Context, query string, args ...interface{}) driver.Row
}

func (m *mockConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(ctx, query, args...)
	}
	return nil
}

type mockRow struct {
	driver.Row
	scanFunc func(dest ...interface{}) error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return nil
}

func TestFetchWeaponKillsFromDB(t *testing.T) {
	expectedCount := uint64(42)
	smfID := 123
	weapon := "kar98k"

	mockCh := &mockConn{
		queryRowFunc: func(ctx context.Context, query string, args ...interface{}) driver.Row {
			// Verify query and args
			expectedQuery := `SELECT count() FROM mohaa_stats.raw_events WHERE actor_smf_id = ? AND event_type = 'kill' AND actor_weapon = ?`
			if query != expectedQuery {
				t.Errorf("expected query %q, got %q", expectedQuery, query)
			}
			if len(args) != 2 {
				t.Fatalf("expected 2 args, got %d", len(args))
			}
			if args[0] != smfID {
				t.Errorf("expected arg[0] %v, got %v", smfID, args[0])
			}
			if args[1] != weapon {
				t.Errorf("expected arg[1] %v, got %v", weapon, args[1])
			}

			return &mockRow{
				scanFunc: func(dest ...interface{}) error {
					if len(dest) != 1 {
						t.Errorf("expected 1 dest, got %d", len(dest))
					}
					// Assign value
					ptr, ok := dest[0].(*uint64)
					if !ok {
						t.Errorf("dest[0] is not *uint64")
					}
					*ptr = expectedCount
					return nil
				},
			}
		},
	}

	mockStatStore := NewMockStatStore()
	logger := zap.NewNop().Sugar()

	worker := &AchievementWorker{
		ch:        mockCh,
		statStore: mockStatStore,
		logger:    logger,
		ctx:       context.Background(),
	}

	// incrementPlayerStat will increment Redis (returning 1) then call fetchFromDB
	count := worker.incrementPlayerStat(smfID, "weapon_kills:"+weapon)
	if count != int(expectedCount) {
		t.Errorf("expected count %d, got %d", expectedCount, count)
	}

	// Verify Redis is updated
	key := fmt.Sprintf("stats:smf:%d:weapon_kills:%s", smfID, weapon)
	val, err := mockStatStore.Get(context.Background(), key)
	if err != nil {
		t.Errorf("expected key %s to be present in Redis", key)
	}
	if val != fmt.Sprintf("%d", expectedCount) {
		t.Errorf("expected Redis value %d, got %s", expectedCount, val)
	}
}

func TestNotifyPlayer(t *testing.T) {
	mockStatStore := NewMockStatStore()
	logger := zap.NewNop().Sugar()
	worker := &AchievementWorker{
		statStore: mockStatStore,
		logger:    logger,
		ctx:       context.Background(),
	}

	def := &AchievementDefinition{
		Slug:        "test-achievement",
		Category:    "general",
		Tier:        "gold",
		Points:      100,
		Description: "Test Description",
	}

	smfID := 12345
	slug := "test-achievement"

	worker.notifyPlayer(smfID, slug, def)

	if len(mockStatStore.PublishedMessages) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(mockStatStore.PublishedMessages))
	}

	msg := mockStatStore.PublishedMessages[0]
	if msg.Channel != "achievement_unlocks" {
		t.Errorf("expected channel 'achievement_unlocks', got '%s'", msg.Channel)
	}

	// Verify payload
	payloadBytes, ok := msg.Message.([]byte)
	if !ok {
		t.Fatalf("expected message to be []byte, got %T", msg.Message)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload["type"] != "achievement_unlock" {
		t.Errorf("expected type 'achievement_unlock', got '%v'", payload["type"])
	}
	// JSON numbers are float64 in interface{}
	if payload["smf_id"].(float64) != float64(smfID) {
		t.Errorf("expected smf_id %d, got %v", smfID, payload["smf_id"])
	}
	if payload["slug"] != slug {
		t.Errorf("expected slug '%s', got '%v'", slug, payload["slug"])
	}
	if payload["points"].(float64) != float64(def.Points) {
		t.Errorf("expected points %d, got %v", def.Points, payload["points"])
	}
}
