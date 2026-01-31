package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

// CapturingDBStore captures Exec calls
type CapturingDBStore struct {
	MockDBStore
	ExecCalls []string
	ExecArgs  [][]interface{}
	// Mock definitions for loadAchievementDefinitions
	Definitions []AchievementDefinition
}

func (m *CapturingDBStore) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	m.ExecCalls = append(m.ExecCalls, sql)
	m.ExecArgs = append(m.ExecArgs, args)
	return pgconn.CommandTag{}, nil
}

func (m *CapturingDBStore) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	// Handle loadAchievementDefinitions
	if strings.Contains(sql, "FROM mohaa_achievements") {
		return &MockDefinitionsRows{Defs: m.Definitions}, nil
	}
	return m.MockDBStore.Query(ctx, sql, args...)
}

func (m *CapturingDBStore) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	// Handle getIDQuery (unlockAchievement)
	if strings.Contains(sql, "SELECT achievement_id FROM mohaa_achievements") {
		return &MockIDRow{ID: 101}
	}
	// Handle checkQuery (unlockAchievement) - returns EXISTS boolean
	if strings.Contains(sql, "SELECT EXISTS") {
		return &MockExistRow{Exists: false}
	}
	return m.MockDBStore.QueryRow(ctx, sql, args...)
}

// MockDefinitionsRows iterates over definitions
type MockDefinitionsRows struct {
	Defs  []AchievementDefinition
	index int
}

func (m *MockDefinitionsRows) Close() {}
func (m *MockDefinitionsRows) Err() error { return nil }
func (m *MockDefinitionsRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *MockDefinitionsRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *MockDefinitionsRows) Next() bool {
	if m.index < len(m.Defs) {
		m.index++
		return true
	}
	return false
}
func (m *MockDefinitionsRows) Scan(dest ...any) error {
	def := m.Defs[m.index-1]
	// slug, category, tier, points, criteria, description
	*dest[0].(*string) = def.Slug
	*dest[1].(*string) = def.Category
	*dest[2].(*string) = def.Tier
	*dest[3].(*int) = def.Points
	*dest[4].(*string) = def.Criteria
	*dest[5].(*string) = def.Description
	return nil
}
func (m *MockDefinitionsRows) Values() ([]any, error) { return nil, nil }
func (m *MockDefinitionsRows) RawValues() [][]byte { return nil }
func (m *MockDefinitionsRows) Conn() *pgx.Conn { return nil }

type MockIDRow struct {
	ID int
}

func (m *MockIDRow) Scan(dest ...any) error {
	*dest[0].(*int) = m.ID
	return nil
}

type MockExistRow struct {
	Exists bool
}

func (m *MockExistRow) Scan(dest ...any) error {
	*dest[0].(*bool) = m.Exists
	return nil
}

func TestVehicleKillAchievementUnlock(t *testing.T) {
	// Setup
	db := &CapturingDBStore{
		Definitions: []AchievementDefinition{
			{
				Slug:        "tank_destroyer_bronze",
				Category:    "vehicle",
				Tier:        "bronze",
				Points:      10,
				Criteria:    "5",
				Description: "Destroy 5 vehicles",
			},
		},
	}

	// Mock ClickHouse to return vehicle_kills = 5
	ch := &mockConn{
		queryRowFunc: func(ctx context.Context, query string, args ...interface{}) driver.Row {
			// Query checks for inflictor LIKE '%vehicle%'
			if strings.Contains(query, "vehicle") {
				return &mockRow{
					scanFunc: func(dest ...interface{}) error {
						// Return 5 (assuming CH includes current kill)
						*dest[0].(*uint64) = 5
						return nil
					},
				}
			}
			// Return 0 for other stats (total_kills etc) to avoid noise
			return &mockRow{
				scanFunc: func(dest ...interface{}) error {
					*dest[0].(*uint64) = 0
					return nil
				},
			}
		},
	}

	statStore := NewMockStatStore()
	logger := zap.NewNop().Sugar()

	worker := NewAchievementWorker(db, ch, statStore, logger)

	// Simulate Event
	event := &models.RawEvent{
		Type:          models.EventKill,
		Inflictor:     "vehicle", // Triggers vehicle logic
		AttackerGUID:  "guid123",
		AttackerSMFID: 999,
		Timestamp:     float64(time.Now().Unix()),
	}

	// Execute
	worker.ProcessEvent(event)

	// Verify
	// We expect "tank_destroyer_bronze" to be unlocked because we mocked prev count=4, increment -> 5
	// Check if INSERT was called on DB
	found := false
	for _, call := range db.ExecCalls {
		if strings.Contains(call, "INSERT INTO mohaa_player_achievements") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected achievement unlock INSERT in DB, but not found. ExecCalls: %v", db.ExecCalls)
	}

	// Also verify that vehicle_kills was incremented in Redis (via MockStatStore)
	val, ok := statStore.Stats["stats:smf:999:vehicle_kills"]
	if !ok || val != 5 {
		t.Errorf("Expected vehicle_kills to be 5, got %v", val)
	}
}
