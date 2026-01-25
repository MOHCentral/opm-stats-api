package worker

import (
	"context"
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

func TestGetWeaponKills(t *testing.T) {
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
				t.Errorf("expected 2 args, got %d", len(args))
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

	logger := zap.NewNop().Sugar()
	// passing nil for db because we don't use it in getWeaponKills
	worker := &AchievementWorker{
		ch:     mockCh,
		logger: logger,
		ctx:    context.Background(),
	}

	count := worker.getWeaponKills(smfID, weapon)
	if count != int(expectedCount) {
		t.Errorf("expected count %d, got %d", expectedCount, count)
	}
}
