package handlers

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/logic"
	"github.com/openmohaa/stats-api/internal/models"
)

// MockServerStatsService
type MockServerStatsService struct {
	GetGlobalStatsFunc    func(ctx context.Context) (map[string]interface{}, error)
	GetGlobalActivityFunc func(ctx context.Context) ([]map[string]interface{}, error)
	GetMapPopularityFunc  func(ctx context.Context) ([]models.MapStats, error)
	GetServerPulseFunc    func(ctx context.Context) (*models.ServerPulse, error)
}

func (m *MockServerStatsService) GetGlobalStats(ctx context.Context) (map[string]interface{}, error) {
	if m.GetGlobalStatsFunc != nil {
		return m.GetGlobalStatsFunc(ctx)
	}
	return map[string]interface{}{"status": "mock"}, nil
}

func (m *MockServerStatsService) GetGlobalActivity(ctx context.Context) ([]map[string]interface{}, error) {
	if m.GetGlobalActivityFunc != nil {
		return m.GetGlobalActivityFunc(ctx)
	}
	return nil, nil
}

func (m *MockServerStatsService) GetMapPopularity(ctx context.Context) ([]models.MapStats, error) {
	if m.GetMapPopularityFunc != nil {
		return m.GetMapPopularityFunc(ctx)
	}
	return nil, nil
}

func (m *MockServerStatsService) GetServerPulse(ctx context.Context) (*models.ServerPulse, error) {
	if m.GetServerPulseFunc != nil {
		return m.GetServerPulseFunc(ctx)
	}
	return nil, nil
}

// MockPlayerStatsService
type MockPlayerStatsService struct {
	GetDeepStatsFunc             func(ctx context.Context, guid string) (*models.DeepStats, error)
	ResolvePlayerGUIDFunc        func(ctx context.Context, name string) (string, error)
	GetPlayerStatsByGametypeFunc func(ctx context.Context, guid string) ([]models.GametypeStats, error)
	GetPlayerStatsByMapFunc      func(ctx context.Context, guid string) ([]models.PlayerMapStats, error)
}

func (m *MockPlayerStatsService) GetDeepStats(ctx context.Context, guid string) (*models.DeepStats, error) {
	if m.GetDeepStatsFunc != nil {
		return m.GetDeepStatsFunc(ctx, guid)
	}
	return &models.DeepStats{}, nil
}

func (m *MockPlayerStatsService) ResolvePlayerGUID(ctx context.Context, name string) (string, error) {
	if m.ResolvePlayerGUIDFunc != nil {
		return m.ResolvePlayerGUIDFunc(ctx, name)
	}
	return "mock-guid", nil
}

func (m *MockPlayerStatsService) GetPlayerStatsByGametype(ctx context.Context, guid string) ([]models.GametypeStats, error) {
	if m.GetPlayerStatsByGametypeFunc != nil {
		return m.GetPlayerStatsByGametypeFunc(ctx, guid)
	}
	return nil, nil
}

func (m *MockPlayerStatsService) GetPlayerStatsByMap(ctx context.Context, guid string) ([]models.PlayerMapStats, error) {
	if m.GetPlayerStatsByMapFunc != nil {
		return m.GetPlayerStatsByMapFunc(ctx, guid)
	}
	return nil, nil
}

// Stub other services (Gamification, etc.) - implementing minimal required methods for compilation if handlers struct needs them
// but Handler struct fields are interfaces, so nil is fine for tests unless the handler method uses them.
// We initialize Handler with nils for unused services in tests.

// MockGamificationService
type MockGamificationService struct{}
func (m *MockGamificationService) GetPlaystyle(ctx context.Context, playerID string) (*models.PlaystyleBadge, error) { return nil, nil }

// MockMatchReportService
type MockMatchReportService struct{}
func (m *MockMatchReportService) GetMatchDetails(ctx context.Context, matchID string) (*logic.MatchDetail, error) { return nil, nil }

// MockAdvancedStatsService
type MockAdvancedStatsService struct{}
func (m *MockAdvancedStatsService) GetPeakPerformance(ctx context.Context, guid string) (*models.PeakPerformance, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetDrillDown(ctx context.Context, guid string, stat string, dimension string, limit int) (*models.DrillDownResult, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetComboMetrics(ctx context.Context, guid string) (*models.ComboMetrics, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetVehicleStats(ctx context.Context, guid string) (*models.VehicleStats, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetGameFlowStats(ctx context.Context, guid string) (*models.GameFlowStats, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetWorldStats(ctx context.Context, guid string) (*models.WorldStats, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetBotStats(ctx context.Context, guid string) (*models.BotStats, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetDrillDownNested(ctx context.Context, guid, stat, parentDim, parentValue, childDim string, limit int) ([]models.DrillDownItem, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetStatLeaders(ctx context.Context, stat, dimension, value string, limit int) ([]models.StatLeaderboardEntry, error) { return nil, nil }
func (m *MockAdvancedStatsService) GetAvailableDrilldowns(stat string) []string { return nil }

// MockTeamStatsService
type MockTeamStatsService struct{}
func (m *MockTeamStatsService) GetFactionComparison(ctx context.Context, days int) (*models.FactionStats, error) { return nil, nil }

// MockTournamentService
type MockTournamentService struct{}
func (m *MockTournamentService) GetTournaments(ctx context.Context) ([]models.Tournament, error) { return nil, nil }
func (m *MockTournamentService) GetTournament(ctx context.Context, id string) (*models.Tournament, error) { return nil, nil }
func (m *MockTournamentService) GetTournamentStats(ctx context.Context, tournamentID string) (map[string]interface{}, error) { return nil, nil }

// MockAchievementsService
type MockAchievementsService struct{
	GetAchievementsFunc func(ctx context.Context, scope logic.AchievementScope, contextID string, playerID string) ([]models.ContextualAchievement, error)
	GetPlayerAchievementsFunc func(ctx context.Context, playerGUID string) ([]models.PlayerAchievement, error)
}
func (m *MockAchievementsService) GetAchievements(ctx context.Context, scope logic.AchievementScope, contextID string, playerID string) ([]models.ContextualAchievement, error) {
	if m.GetAchievementsFunc != nil {
		return m.GetAchievementsFunc(ctx, scope, contextID, playerID)
	}
	return nil, nil
}
func (m *MockAchievementsService) GetPlayerAchievements(ctx context.Context, playerGUID string) ([]models.PlayerAchievement, error) {
	if m.GetPlayerAchievementsFunc != nil {
		return m.GetPlayerAchievementsFunc(ctx, playerGUID)
	}
	return nil, nil
}

// MockPredictionService
type MockPredictionService struct{}
func (m *MockPredictionService) GetPlayerPredictions(ctx context.Context, guid string) (*models.PlayerPredictions, error) { return nil, nil }
func (m *MockPredictionService) GetMatchPredictions(ctx context.Context, matchID string) (*models.MatchPredictions, error) { return nil, nil }

// MockClickHouseConn implements driver.Conn for testing
type MockClickHouseConn struct {
	driver.Conn
}

func (m *MockClickHouseConn) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return &MockRows{}, nil
}

func (m *MockClickHouseConn) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return &MockRow{}
}

type MockRow struct{}
func (m *MockRow) Scan(dest ...interface{}) error { return nil }
func (m *MockRow) Err() error { return nil }
func (m *MockRow) ScanStruct(dest interface{}) error { return nil }

type MockRows struct{}
func (m *MockRows) Next() bool { return false }
func (m *MockRows) Scan(dest ...interface{}) error { return nil }
func (m *MockRows) ScanStruct(dest interface{}) error { return nil }
func (m *MockRows) Close() error { return nil }
func (m *MockRows) Err() error { return nil }
func (m *MockRows) Columns() []string { return []string{} }
func (m *MockRows) ColumnTypes() []driver.ColumnType { return []driver.ColumnType{} }
func (m *MockRows) Totals(dest ...interface{}) error { return nil }
