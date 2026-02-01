package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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
// DYNAMIC STATS ENDPOINT
// ============================================================================

// GetDynamicStats handles flexible stats queries
func (h *Handler) GetDynamicStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse parameters
	req := logic.DynamicQueryRequest{
		Dimension:    q.Get("dimension"),
		Metric:       q.Get("metric"),
		FilterGUID:   q.Get("filter_player_guid"),
		FilterMap:    q.Get("filter_map"),
		FilterWeapon: q.Get("filter_weapon"),
		FilterServer: q.Get("filter_server"),
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = l
		}
	}

	if startStr := q.Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			req.StartDate = t
		}
	}
	if endStr := q.Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			req.EndDate = t
		}
	}

	// Build query
	sql, args, err := logic.BuildStatsQuery(req)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Execute
	ctx := r.Context()
	rows, err := h.ch.Query(ctx, sql, args...)
	if err != nil {
		h.logger.Errorw("Dynamic stats query failed", "error", err, "query", sql)
		h.errorResponse(w, http.StatusInternalServerError, "Query execution failed")
		return
	}
	defer rows.Close()

	// Generic result structure
	type Result struct {
		Label string  `json:"label"`
		Value float64 `json:"value"`
	}

	var results []Result
	for rows.Next() {
		var r Result
		// Note: The order of scan vars must match the SELECT order in query_builder (value, label)
		if err := rows.Scan(&r.Value, &r.Label); err != nil {
			h.logger.Errorw("Failed to scan row", "error", err)
			continue
		}
		results = append(results, r)
	}

	h.jsonResponse(w, http.StatusOK, results)
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
