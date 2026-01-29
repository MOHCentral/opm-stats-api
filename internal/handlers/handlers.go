package handlers

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/openmohaa/stats-api/internal/logic"
	"github.com/openmohaa/stats-api/internal/models"
)

// MaxBodySize limits the size of request bodies to 1MB
const MaxBodySize = 1048576

// IngestQueue defines the interface for the event ingestion worker pool
type IngestQueue interface {
	Enqueue(event *models.RawEvent) bool
	QueueDepth() int
}

type Config struct {
	WorkerPool IngestQueue
	Postgres   *pgxpool.Pool
	ClickHouse driver.Conn
	Redis      *redis.Client
	Logger     *zap.Logger
	// Services
	PlayerStats   logic.PlayerStatsService
	ServerStats   logic.ServerStatsService
	Gamification  logic.GamificationService
	MatchReport   logic.MatchReportService
	AdvancedStats logic.AdvancedStatsService
	TeamStats     logic.TeamStatsService
	Tournament    logic.TournamentService
	Achievements  logic.AchievementsService
	Prediction    logic.PredictionService
}

type Handler struct {
	pool          IngestQueue
	pg            *pgxpool.Pool
	ch            driver.Conn
	redis         *redis.Client
	logger        *zap.SugaredLogger
	validator     *validator.Validate
	playerStats   logic.PlayerStatsService
	serverStats   logic.ServerStatsService
	gamification  logic.GamificationService
	matchReport   logic.MatchReportService
	advancedStats logic.AdvancedStatsService
	teamStats     logic.TeamStatsService
	tournament    logic.TournamentService
	achievements  logic.AchievementsService
	prediction    logic.PredictionService
}

func New(cfg Config) *Handler {
	return &Handler{
		pool:          cfg.WorkerPool,
		pg:            cfg.Postgres,
		ch:            cfg.ClickHouse,
		redis:         cfg.Redis,
		logger:        cfg.Logger.Sugar(),
		validator:     validator.New(),
		playerStats:   cfg.PlayerStats,
		serverStats:   cfg.ServerStats,
		gamification:  cfg.Gamification,
		matchReport:   cfg.MatchReport,
		advancedStats: cfg.AdvancedStats,
		teamStats:     cfg.TeamStats,
		tournament:    cfg.Tournament,
		achievements:  cfg.Achievements,
		prediction:    cfg.Prediction,
	}
}
