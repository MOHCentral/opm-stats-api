package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/google/uuid"
)

type MockPgPool struct {
	QueryFunc func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *MockPgPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, sql, args...)
	}
	return nil, nil
}
func (m *MockPgPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (m *MockPgPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }

type MockPgRows struct {
	count int
	curr  int
}

func (r *MockPgRows) Close() {}
func (r *MockPgRows) Err() error { return nil }
func (r *MockPgRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *MockPgRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *MockPgRows) Next() bool {
	r.curr++
	return r.curr <= r.count
}
func (r *MockPgRows) Scan(dest ...any) error {
	// Dest: ID, PlayerGUID, AchievementID, UnlockedAt, A.ID, A.Name, A.Desc, A.Category, A.Points, IconURL

	// 1. ID (uuid.UUID)
	if ptr, ok := dest[0].(*uuid.UUID); ok {
		*ptr = uuid.New()
	} else if ptr, ok := dest[0].(*string); ok {
		*ptr = uuid.New().String()
	}

	// 2. PlayerGUID
	if ptr, ok := dest[1].(*string); ok {
		*ptr = "test-guid"
	}

	// 3. AchievementID
	if ptr, ok := dest[2].(*string); ok {
		*ptr = "KILL_100"
	}

	// 4. UnlockedAt
	if ptr, ok := dest[3].(*time.Time); ok {
		*ptr = time.Now()
	}

	// 5. A.ID
	if ptr, ok := dest[4].(*string); ok {
		*ptr = "KILL_100"
	}

	// 6. Name
	if ptr, ok := dest[5].(*string); ok {
		*ptr = "First Blood"
	}

	// 7. Desc
	if ptr, ok := dest[6].(*string); ok {
		*ptr = "Get a kill"
	}

	// 8. Category
	if ptr, ok := dest[7].(*string); ok {
		*ptr = "Combat"
	}

	// 9. Points
	if ptr, ok := dest[8].(*int); ok {
		*ptr = 10
	}

	// 10. IconURL (*string)
	if ptr, ok := dest[9].(**string); ok {
		val := "icon.png"
		*ptr = &val
	}

	return nil
}
func (r *MockPgRows) Values() ([]any, error) { return nil, nil }
func (r *MockPgRows) RawValues() [][]byte { return nil }
func (r *MockPgRows) Conn() *pgx.Conn { return nil }

func TestGetPlayerAchievements(t *testing.T) {
	mockPg := &MockPgPool{
		QueryFunc: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return &MockPgRows{count: 1}, nil
		},
	}

	service := NewAchievementsService(nil, mockPg)

	list, err := service.GetPlayerAchievements(context.Background(), "test-guid")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 achievement, got %d", len(list))
	}

	if list[0].Achievement.Points != 10 {
		t.Errorf("Expected 10 points, got %d", list[0].Achievement.Points)
	}

	if list[0].Achievement.Tier != 1 {
		t.Errorf("Expected Tier 1 (Bronze), got %d", list[0].Achievement.Tier)
	}
}
