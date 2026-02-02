package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

// hashToken creates a SHA256 hash of a token for secure storage lookup
func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
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
	validator     *validator.Validate // Added validator
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
		validator:     validator.New(), // Initialize validator
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

// ============================================================================
// HEALTH ENDPOINTS
// ============================================================================

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check all dependencies
	checks := map[string]bool{
		"postgres":   h.pg.Ping(ctx) == nil,
		"clickhouse": h.ch.Ping(ctx) == nil,
		"redis":      h.redis.Ping(ctx).Err() == nil,
	}

	allHealthy := true
	for _, ok := range checks {
		if !ok {
			allHealthy = false
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":      allHealthy,
		"checks":     checks,
		"queueDepth": h.pool.QueueDepth(),
	})
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

// ServerAuthMiddleware validates server tokens
func (h *Handler) ServerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Server-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
		}
		if token == "" {
			token = r.URL.Query().Get("server_token")
		}

		if token == "" {
			// DO NOT call r.FormValue here as it consumes the body
			// Game servers use the header now
			h.errorResponse(w, http.StatusUnauthorized, "Missing server token")
			return
		}

		// Validate token against database - lookup server by token hash
		ctx := r.Context()
		var serverID string
		hashedToken := hashToken(token)
		h.logger.Infow("Auth Debug", "received_token", token, "computed_hash", hashedToken)

		err := h.pg.QueryRow(ctx,
			"SELECT id FROM servers WHERE token = $1 AND is_active = true",
			hashedToken).Scan(&serverID)

		if err != nil {
			h.logger.Errorw("Auth Database Error", "error", err, "hash", hashedToken)
		}

		if err != nil || serverID == "" {
			h.errorResponse(w, http.StatusUnauthorized, "Invalid server token")
			return
		}

		// Add server ID to context for handlers
		ctx = context.WithValue(ctx, "server_id", serverID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getUserIDFromContext extracts user ID from request context (currently unused since JWT removal)
func (h *Handler) getUserIDFromContext(ctx context.Context) int {
	return 0
}

func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}

// ============================================================================
// STUBS FOR REMOVED METHODS (To avoid breaking routes if any)
// ============================================================================

// ListAchievements returns a message directing to SMF database
func (h *Handler) ListAchievements(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Achievement definitions are stored in SMF database (smf_mohaa_achievement_defs). Use the SMF forum to view achievements.",
		"source":  "smf_database",
	})
}

// GetAchievement returns a message directing to SMF database
func (h *Handler) GetAchievement(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Achievement definitions are stored in SMF database. Use the SMF forum to view achievements.",
		"source":  "smf_database",
	})
}

// GetRecentAchievements returns a global feed of recent unlocks from database
func (h *Handler) GetRecentAchievements(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}

// GetAchievementLeaderboard returns players ranked by achievement points
func (h *Handler) GetAchievementLeaderboard(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}
