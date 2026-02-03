package logic

import (
	"context"
	"reflect"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

// MockPlayerConn implements driver.Conn for testing
type MockPlayerConn struct {
	driver.Conn
	QueryFunc func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error)
}

func (m *MockPlayerConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, args...)
	}
	return &MockPlayerRows{}, nil
}

// MockPlayerRows implements driver.Rows for testing
type MockPlayerRows struct {
	driver.Rows
	Data  [][]interface{}
	Index int
}

func (m *MockPlayerRows) Next() bool {
	m.Index++
	return m.Index <= len(m.Data)
}

func (m *MockPlayerRows) Scan(dest ...interface{}) error {
	if m.Index > len(m.Data) {
		return nil
	}
	row := m.Data[m.Index-1]
	for i, val := range row {
		if i < len(dest) {
			setDest(dest[i], val)
		}
	}
	return nil
}

func (m *MockPlayerRows) Close() error { return nil }
func (m *MockPlayerRows) Err() error   { return nil }

func setDest(dest interface{}, val interface{}) {
	v := reflect.ValueOf(dest).Elem()
	valV := reflect.ValueOf(val)
	// Handle type conversion if needed (e.g. int to int64)
	if valV.Type().ConvertibleTo(v.Type()) {
		v.Set(valV.Convert(v.Type()))
	} else {
		v.Set(valV)
	}
}

func TestGetPlayerStatsByGametype(t *testing.T) {
	tests := []struct {
		name      string
		guid      string
		mockRows  [][]interface{}
		wantStats []models.GametypeStats
		wantErr   bool
	}{
		{
			name: "Success",
			guid: "player1",
			mockRows: [][]interface{}{
				{"dm", uint64(10), uint64(5), uint64(2), uint64(3)},
				{"obj", uint64(20), uint64(10), uint64(5), uint64(5)},
			},
			wantStats: []models.GametypeStats{
				{Gametype: "dm", Kills: 10, Deaths: 5, Headshots: 2, MatchesPlayed: 3, KDRatio: 2.0},
				{Gametype: "obj", Kills: 20, Deaths: 10, Headshots: 5, MatchesPlayed: 5, KDRatio: 2.0},
			},
			wantErr: false,
		},
		{
			name:      "Empty",
			guid:      "player2",
			mockRows:  [][]interface{}{},
			wantStats: []models.GametypeStats{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockPlayerConn{
				QueryFunc: func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
					return &MockPlayerRows{Data: tt.mockRows}, nil
				},
			}
			s := NewPlayerStatsService(mockConn)
			got, err := s.GetPlayerStatsByGametype(context.Background(), tt.guid)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPlayerStatsByGametype() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantStats) {
				t.Errorf("GetPlayerStatsByGametype() = %v, want %v", got, tt.wantStats)
			}
		})
	}
}

func TestGetPlayerStatsByMap(t *testing.T) {
	tests := []struct {
		name      string
		guid      string
		mockRows  [][]interface{}
		wantStats []models.PlayerMapStats
		wantErr   bool
	}{
		{
			name: "Success",
			guid: "player1",
			mockRows: [][]interface{}{
				{"map1", uint64(10), uint64(5), uint64(2), uint64(3), uint64(0)},
			},
			wantStats: []models.PlayerMapStats{
				{MapName: "map1", Kills: 10, Deaths: 5, Headshots: 2, MatchesPlayed: 3, MatchesWon: 0, KDRatio: 2.0},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &MockPlayerConn{
				QueryFunc: func(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
					return &MockPlayerRows{Data: tt.mockRows}, nil
				},
			}
			s := NewPlayerStatsService(mockConn)
			got, err := s.GetPlayerStatsByMap(context.Background(), tt.guid)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPlayerStatsByMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantStats) {
				t.Errorf("GetPlayerStatsByMap() = %v, want %v", got, tt.wantStats)
			}
		})
	}
}
