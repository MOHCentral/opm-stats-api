package logic

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/openmohaa/stats-api/internal/models"
	"github.com/redis/go-redis/v9"
)

// PgPool defines the interface for PostgreSQL connection pool
type PgPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// RedisClient defines the interface for Redis client
type RedisClient interface {
	HGet(ctx context.Context, key string, field string) *redis.StringCmd
	HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd
}

type PlayerStatsService interface {
	GetDeepStats(ctx context.Context, guid string) (*models.DeepStats, error)
	ResolvePlayerGUID(ctx context.Context, name string) (string, error)
	GetPlayerStatsByGametype(ctx context.Context, guid string) ([]models.GametypeStats, error)
	GetPlayerStatsByMap(ctx context.Context, guid string) ([]models.PlayerMapStats, error)
}

type ServerStatsService interface {
	GetGlobalActivity(ctx context.Context) ([]map[string]interface{}, error)
	GetMapPopularity(ctx context.Context) ([]models.MapStats, error)
	GetServerPulse(ctx context.Context) (*models.ServerPulse, error)
	GetGlobalStats(ctx context.Context) (map[string]interface{}, error)
}

type GamificationService interface {
	GetPlaystyle(ctx context.Context, playerID string) (*models.PlaystyleBadge, error)
}

type MatchReportService interface {
	GetMatchDetails(ctx context.Context, matchID string) (*MatchDetail, error)
}

type AdvancedStatsService interface {
	GetPeakPerformance(ctx context.Context, guid string) (*models.PeakPerformance, error)
	GetDrillDown(ctx context.Context, guid string, stat string, dimension string, limit int) (*models.DrillDownResult, error)
	GetComboMetrics(ctx context.Context, guid string) (*models.ComboMetrics, error)
	GetVehicleStats(ctx context.Context, guid string) (*models.VehicleStats, error)
	GetGameFlowStats(ctx context.Context, guid string) (*models.GameFlowStats, error)
	GetWorldStats(ctx context.Context, guid string) (*models.WorldStats, error)
	GetBotStats(ctx context.Context, guid string) (*models.BotStats, error)
	GetDrillDownNested(ctx context.Context, guid, stat, parentDim, parentValue, childDim string, limit int) ([]models.DrillDownItem, error)
	GetStatLeaders(ctx context.Context, stat, dimension, value string, limit int) ([]models.StatLeaderboardEntry, error)
	GetAvailableDrilldowns(stat string) []string
}

type TeamStatsService interface {
	GetFactionComparison(ctx context.Context, days int) (*models.FactionStats, error)
}

type TournamentService interface {
	GetTournaments(ctx context.Context) ([]models.Tournament, error)
	GetTournament(ctx context.Context, id string) (*models.Tournament, error)
	GetTournamentStats(ctx context.Context, tournamentID string) (map[string]interface{}, error)
}

type AchievementsService interface {
	GetAchievements(ctx context.Context, scope AchievementScope, contextID string, playerID string) ([]models.ContextualAchievement, error)
	GetPlayerAchievements(ctx context.Context, playerGUID string) ([]models.PlayerAchievement, error)
	GetRecentAchievements(ctx context.Context, limit int) ([]models.PlayerAchievement, error)
}

type PredictionService interface {
	GetPlayerPredictions(ctx context.Context, guid string) (*models.PlayerPredictions, error)
	GetMatchPredictions(ctx context.Context, matchID string) (*models.MatchPredictions, error)
}
