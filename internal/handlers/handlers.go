package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/go-chi/chi/v5"
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
// INGESTION ENDPOINTS
// ============================================================================

// IngestEvents handles POST /api/v1/ingest/events
// @Summary Ingest Game Events
// @Description Accepts newline-separated JSON events from game servers
// @Tags Ingestion
// @Accept json
// @Produce json
// @Security ServerToken
// @Param body body []models.RawEvent true "Events"
// @Success 202 {object} map[string]string "Accepted"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /ingest/events [post]
func (h *Handler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.errorResponse(w, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	defer r.Body.Close()

	h.logger.Infow("IngestEvents called", "bodyLength", len(body), "preview", string(body[:min(len(body), 200)]))

	lines := strings.Split(string(body), "\n")
	h.logger.Infow("Split body into lines", "lineCount", len(lines))
	processed := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			h.logger.Debugw("Skipping empty line", "lineNum", i)
			continue
		}

		h.logger.Infow("Processing line", "lineNum", i, "preview", line[:min(len(line), 100)])
		var event models.RawEvent
		// Support both JSON (if line starts with {) and URL-encoded
		if strings.HasPrefix(line, "{") {
			h.logger.Infow("Parsing as JSON", "lineNum", i)
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				h.logger.Warnw("Failed to unmarshal JSON event in batch", "error", err, "line", line)
				continue
			}
			h.logger.Infow("JSON parsed successfully", "eventType", event.Type)
		} else {
			h.logger.Infow("Parsing as URL-encoded", "lineNum", i)
			values, err := url.ParseQuery(line)
			if err != nil {
				h.logger.Warnw("Failed to parse URL-encoded event in batch", "error", err, "line", line)
				continue
			}
			event = h.parseFormToEvent(values)
			h.logger.Infow("URL-encoded parsed", "eventType", event.Type)
		}

		// Inject ServerID from context if authenticated
		if sid, ok := r.Context().Value("server_id").(string); ok && sid != "" {
			if event.ServerID == "" {
				event.ServerID = sid
			}
		}

		if event.Type == "" {
			h.logger.Warnw("Event has empty type, skipping", "lineNum", i, "line", line[:min(len(line), 100)])
			continue
		}

		h.logger.Infow("Enqueueing event", "type", event.Type, "match_id", event.MatchID)
		if !h.pool.Enqueue(&event) {
			h.logger.Warn("Worker pool queue full, dropping remaining events in batch")
			break
		}
		processed++
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "accepted",
		"processed": processed,
	})
}

// parseFormToEvent converts URL-encoded form data to RawEvent
func (h *Handler) parseFormToEvent(form url.Values) models.RawEvent {
	event := models.RawEvent{
		Type:        models.EventType(form.Get("type")),
		MatchID:     form.Get("match_id"),
		SessionID:   form.Get("session_id"),
		ServerID:    form.Get("server_id"),
		ServerToken: form.Get("server_token"),
		MapName:     form.Get("map_name"),

		PlayerName: form.Get("player_name"),
		PlayerGUID: form.Get("player_guid"),
		PlayerTeam: form.Get("player_team"),

		AttackerName: form.Get("attacker_name"),
		AttackerGUID: form.Get("attacker_guid"),
		AttackerTeam: form.Get("attacker_team"),

		VictimName: form.Get("victim_name"),
		VictimGUID: form.Get("victim_guid"),
		VictimTeam: form.Get("victim_team"),

		Weapon:    form.Get("weapon"),
		OldWeapon: form.Get("old_weapon"),
		NewWeapon: form.Get("new_weapon"),
		Hitloc:    form.Get("hitloc"),
		Inflictor: form.Get("inflictor"),

		TargetName: form.Get("target_name"),
		TargetGUID: form.Get("target_guid"),

		OldTeam: form.Get("old_team"),
		NewTeam: form.Get("new_team"),
		Message: form.Get("message"),

		Gametype:    form.Get("gametype"),
		Timelimit:   form.Get("timelimit"),
		Fraglimit:   form.Get("fraglimit"),
		Maxclients:  form.Get("maxclients"),
		WinningTeam: form.Get("winning_team"),

		Item:       form.Get("item"),
		Entity:     form.Get("entity"),
		Projectile: form.Get("projectile"),
		Code:       form.Get("code"),

		Objective:       form.Get("objective"), // Also check objective_index if needed
		ObjectiveStatus: form.Get("objective_status"),
		BotID:           form.Get("bot_id"),
		Seat:            form.Get("seat"),
	}

	// Parse numeric fields
	event.Timestamp, _ = strconv.ParseFloat(form.Get("timestamp"), 64)
	event.Damage, _ = strconv.Atoi(form.Get("damage"))
	event.AmmoRemaining, _ = strconv.Atoi(form.Get("ammo_remaining"))
	event.AlliesScore, _ = strconv.Atoi(form.Get("allies_score"))
	event.AxisScore, _ = strconv.Atoi(form.Get("axis_score"))
	event.RoundNumber, _ = strconv.Atoi(form.Get("round_number"))
	event.TotalRounds, _ = strconv.Atoi(form.Get("total_rounds"))
	event.PlayerCount, _ = strconv.Atoi(form.Get("player_count"))
	event.ClientNum, _ = strconv.Atoi(form.Get("client_num"))
	event.Count, _ = strconv.Atoi(form.Get("count"))
	event.Duration, _ = strconv.ParseFloat(form.Get("duration"), 64)

	// Parse SMF ID fields (Int64 for member IDs)
	event.PlayerSMFID = parseInt64(form.Get("player_smf_id"))
	event.AttackerSMFID = parseInt64(form.Get("attacker_smf_id"))
	event.VictimSMFID = parseInt64(form.Get("victim_smf_id"))
	event.TargetSMFID = parseInt64(form.Get("target_smf_id"))

	// Parse float fields (positions)
	event.PosX = parseFloat32(form.Get("pos_x"))
	event.PosY = parseFloat32(form.Get("pos_y"))
	event.PosZ = parseFloat32(form.Get("pos_z"))
	event.AttackerX = parseFloat32(form.Get("attacker_x"))
	event.AttackerY = parseFloat32(form.Get("attacker_y"))
	event.AttackerZ = parseFloat32(form.Get("attacker_z"))
	event.AttackerPitch = parseFloat32(form.Get("attacker_pitch"))
	event.AttackerYaw = parseFloat32(form.Get("attacker_yaw"))
	event.AttackerStance = form.Get("attacker_stance")
	event.VictimX = parseFloat32(form.Get("victim_x"))
	event.VictimY = parseFloat32(form.Get("victim_y"))
	event.VictimZ = parseFloat32(form.Get("victim_z"))
	event.VictimStance = form.Get("victim_stance")
	event.PlayerStance = form.Get("player_stance")
	event.TargetStance = form.Get("target_stance")
	event.AimPitch = parseFloat32(form.Get("aim_pitch"))
	event.AimYaw = parseFloat32(form.Get("aim_yaw"))
	event.FallHeight = parseFloat32(form.Get("fall_height"))
	event.Walked = parseFloat32(form.Get("walked"))
	event.Sprinted = parseFloat32(form.Get("sprinted"))
	event.Swam = parseFloat32(form.Get("swam"))
	event.Driven = parseFloat32(form.Get("driven"))
	event.Distance = parseFloat32(form.Get("distance"))

	return event
}

func parseInt64(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func parseFloat32(s string) float32 {
	f, _ := strconv.ParseFloat(s, 32)
	return float32(f)
}

// IngestMatchResult handles POST /api/v1/ingest/match-result
// Synchronous processing for tournament integration
// @Summary Ingest Match Result
// @Tags Ingestion
// @Security ServerToken
// @Accept json
// @Produce json
// @Param body body models.MatchResult true "Match Result"
// @Success 200 {object} map[string]string "Processed"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /ingest/match-result [post]
func (h *Handler) IngestMatchResult(w http.ResponseWriter, r *http.Request) {
	var result models.MatchResult

	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Tournament match results are handled by SMF plugin
	// See: smf-plugins/mohaa_tournaments/ for bracket management

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "processed",
	})
}

// ============================================================================
// STATS ENDPOINTS
// ============================================================================

// GetGlobalStats returns aggregate statistics for the dashboard
// @Summary Global Network Stats
// @Tags Server
// @Produce json
// @Success 200 {object} map[string]interface{} "Global Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/global [get]
func (h *Handler) GetGlobalStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.serverStats.GetGlobalStats(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get global stats", "error", err)
		// We could return 500, but legacy behavior was partial.
		// If implementation returns error on critical stats, 500 might be appropriate.
		// For now, if we get data, use it. If completely failed, error.
		if stats == nil {
			h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}
	h.jsonResponse(w, http.StatusOK, stats)
}

// GetMatches returns a list of recent matches
// @Summary Get Recent Matches
// @Tags Match
// @Produce json
// @Param limit query int false "Limit" default(25)
// @Success 200 {array} models.MatchSummary "Matches"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/matches [get]
func (h *Handler) GetMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// Fetch matches
	rows, err := h.ch.Query(ctx, `
		SELECT 
			toString(match_id) as match_id,
			map_name,
			any(server_id) as server_id,
			min(timestamp) as start_time,
			toFloat64(dateDiff('second', min(timestamp), max(timestamp))) as duration,
			uniq(actor_id) as player_count,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		GROUP BY match_id, map_name
		ORDER BY start_time DESC
		LIMIT ? OFFSET ?
	`, limit, offset)

	if err != nil {
		h.logger.Errorw("Failed to fetch matches", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	matches := make([]models.MatchSummary, 0)
	serverIDs := make(map[string]bool)
	for rows.Next() {
		var m models.MatchSummary
		if err := rows.Scan(&m.ID, &m.Map, &m.ServerID, &m.StartTime, &m.Duration, &m.PlayerCount, &m.Kills); err != nil {
			h.logger.Warnw("Scan error in GetMatches", "error", err)
			continue
		}
		matches = append(matches, m)
		serverIDs[m.ServerID] = true
	}

	// Look up server names from PostgreSQL
	serverNames := make(map[string]string)
	for serverID := range serverIDs {
		if serverID == "" {
			continue
		}
		var name string
		err := h.pg.QueryRow(ctx, "SELECT name FROM servers WHERE id = $1", serverID).Scan(&name)
		if err == nil {
			serverNames[serverID] = name
		}
	}

	// Apply server names to matches
	for i := range matches {
		if name, ok := serverNames[matches[i].ServerID]; ok {
			matches[i].ServerName = name
		} else if matches[i].ServerID != "" {
			matches[i].ServerName = "Unknown Server"
		}
	}

	h.jsonResponse(w, http.StatusOK, matches)
}

// GetGlobalWeaponStats returns weapon usage statistics
// @Summary Get Global Weapon Stats
// @Tags Server
// @Produce json
// @Success 200 {array} models.WeaponStats "Weapon Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/weapons [get]
func (h *Handler) GetGlobalWeaponStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_weapon as weapon,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'headshot') as headshots
		FROM mohaa_stats.raw_events
		WHERE actor_weapon != '' 
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 10
	`)
	if err != nil {
		h.logger.Errorw("Failed to query weapon stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type WeaponStats struct {
		Name      string `json:"name"`
		Kills     uint64 `json:"kills"`
		Headshots uint64 `json:"headshots"`
	}

	stats := make([]WeaponStats, 0)
	for rows.Next() {
		var s WeaponStats
		if err := rows.Scan(&s.Name, &s.Kills, &s.Headshots); err != nil {
			continue
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetLeaderboard returns rankings based on various criteria
// @Summary Get Global Leaderboard
// @Tags Leaderboards
// @Produce json
// @Param limit query int false "Limit" default(25)
// @Param page query int false "Page" default(1)
// @Success 200 {object} map[string]interface{} "Leaderboard"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/leaderboard [get]
// GetLeaderboard returns ranked list of players by a specific stat
// @Summary Global Leaderboard
// @Description Get ranked list of players by any of the 38 supported metrics
// @Tags Stats
// @Produce json
// @Param stat path string false "Stat to sort by (e.g. kills, headshots, distance)" default(kills)
// @Param period query string false "Period (all, week, month)" default(all)
// @Param limit query int false "Limit" default(25)
// @Param page query int false "Page" default(1)
// @Success 200 {object} map[string]interface{} "Leaderboard Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/leaderboard/{stat} [get]
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parameters
	stat := chi.URLParam(r, "stat")
	if stat == "" {
		stat = r.URL.Query().Get("stat")
	}
	if stat == "" {
		stat = "kills"
	}

	limit := 25
	page := 1
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	offset := (page - 1) * limit

	// Map stat name to ClickHouse column/expression
	orderExpr := "kills"
	havingExpr := "kills > 0"

	switch stat {
	case "kills": orderExpr = "kills"
	case "deaths": orderExpr = "deaths"; havingExpr = "deaths > 0"
	case "kd_ratio", "kd": orderExpr = "kills / nullIf(deaths, 0)"
	case "headshots": orderExpr = "headshots"
	case "accuracy": orderExpr = "shots_hit / nullIf(shots_fired, 0)"
	case "shots_fired": orderExpr = "shots_fired"
	case "damage": orderExpr = "total_damage"
	case "bash_kills": orderExpr = "bash_kills"
	case "grenade_kills": orderExpr = "grenade_kills"
	case "roadkills": orderExpr = "roadkills"
	case "telefrags": orderExpr = "telefrags"
	case "crushed": orderExpr = "crushed"
	case "teamkills": orderExpr = "teamkills"
	case "suicides": orderExpr = "suicides"
	case "reloads": orderExpr = "reloads"
	case "weapon_swaps": orderExpr = "weapon_swaps"
	case "no_ammo": orderExpr = "no_ammo"
	case "looter": orderExpr = "items_picked"
	case "distance": orderExpr = "distance_units"
	case "sprinted": orderExpr = "sprinted"
	case "swam": orderExpr = "swam"
	case "driven": orderExpr = "driven"
	case "jumps": orderExpr = "jumps"
	case "crouch_time": orderExpr = "crouch_events"
	case "prone_time": orderExpr = "prone_events"
	case "ladders": orderExpr = "ladders"
	case "health_picked": orderExpr = "health_picked"
	case "ammo_picked": orderExpr = "ammo_picked"
	case "armor_picked": orderExpr = "armor_picked"
	case "items_picked": orderExpr = "items_picked"
	case "wins": orderExpr = "matches_won"
	case "team_wins": orderExpr = "matches_won" // Simplify for now
	case "ffa_wins": orderExpr = "matches_won"
	case "losses": orderExpr = "matches_played - matches_won"
	case "objectives": orderExpr = "objectives"
	case "rounds": orderExpr = "matches_played"
	case "playtime": orderExpr = "playtime_seconds"
	case "games": orderExpr = "games_finished"
	default: orderExpr = "kills"
	}

	whereExpr := "actor_id != ''"
	switch period {
	case "week":
		whereExpr += " AND day >= now() - INTERVAL 7 DAY"
	case "month":
		whereExpr += " AND day >= now() - INTERVAL 30 DAY"
	case "year":
		whereExpr += " AND day >= now() - INTERVAL 365 DAY"
	}

	// Query the unified Aggregation Table
	query := fmt.Sprintf(`
		SELECT 
			player_id AS actor_id,
			argMax(player_name, last_active) AS actor_name,
			sum(kills) AS kills,
			sum(deaths) AS deaths,
			sum(headshots) AS headshots,
			sum(shots_fired) AS shots_fired,
			sum(shots_hit) AS shots_hit,
			sum(total_damage) AS total_damage,
			sum(bash_kills) AS bash_kills,
			sum(grenade_kills) AS grenade_kills,
			sum(roadkills) AS roadkills,
			sum(telefrags) AS telefrags,
			sum(crushed) AS crushed,
			sum(teamkills) AS teamkills,
			sum(suicides) AS suicides,
			sum(reloads) AS reloads,
			sum(weapon_swaps) AS weapon_swaps,
			sum(no_ammo) AS no_ammo,
			sum(distance_units) AS distance,
			sum(sprinted) AS sprinted,
			sum(swam) AS swam,
			sum(driven) AS driven,
			sum(jumps) AS jumps,
			sum(crouch_events) AS crouches,
			sum(prone_events) AS prone,
			sum(ladders) AS ladders,
			sum(health_picked) AS health_picked,
			sum(ammo_picked) AS ammo_picked,
			sum(armor_picked) AS armor_picked,
			sum(items_picked) AS items_picked,
			sum(matches_won) AS wins,
			uniqExactMerge(matches_played) AS rounds, -- Using uniqExactMerge on the state
			sum(games_finished) AS games,
			toUInt64(0) AS playtime, -- Not calculated by MV currently
			max(last_active) AS last_active
		FROM mohaa_stats.player_stats_daily
		WHERE player_id != '' AND %s
		GROUP BY player_id
		HAVING %s
		ORDER BY %s DESC
		LIMIT ? OFFSET ?
	`, whereExpr, havingExpr, orderExpr)

	rows, err := h.ch.Query(ctx, query, limit, offset)
	if err != nil {
		h.logger.Errorw("Failed to query leaderboard", "stat", stat, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	entries := make([]models.LeaderboardEntry, 0)
	rank := offset + 1
	for rows.Next() {
		var entry models.LeaderboardEntry
		var lastActive time.Time
		if err := rows.Scan(
			&entry.PlayerID, &entry.PlayerName, &entry.Kills, &entry.Deaths,
			&entry.Headshots, &entry.ShotsFired, &entry.ShotsHit, &entry.Damage,
			&entry.BashKills, &entry.GrenadeKills, &entry.Roadkills, &entry.Telefrags,
			&entry.Crushed, &entry.TeamKills, &entry.Suicides, &entry.Reloads,
			&entry.WeaponSwaps, &entry.NoAmmo, &entry.Distance, &entry.Sprinted,
			&entry.Swam, &entry.Driven, &entry.Jumps, &entry.Crouches,
			&entry.Prone, &entry.Ladders, &entry.HealthPicked, &entry.AmmoPicked,
			&entry.ArmorPicked, &entry.ItemsPicked, &entry.Wins, &entry.Rounds,
			&entry.GamesFinished, &entry.Playtime, &lastActive,
		); err != nil {
			h.logger.Warnw("Failed to scan leaderboard row", "error", err)
			continue
		}

		if entry.ShotsFired > 0 {
			entry.Accuracy = (float64(entry.ShotsHit) / float64(entry.ShotsFired)) * 100.0
		}

		// Map the requested stat to the Value field for AG Grid
		switch stat {
		case "kills": entry.Value = entry.Kills
		case "deaths": entry.Value = entry.Deaths
		case "headshots": entry.Value = entry.Headshots
		case "accuracy": entry.Value = fmt.Sprintf("%.1f%%", entry.Accuracy)
		case "damage", "total_damage": entry.Value = entry.Damage
		case "wins": entry.Value = entry.Wins
		case "rounds": entry.Value = entry.Rounds
		case "looter": entry.Value = entry.ItemsPicked
		case "distance", "distance_km": entry.Value = fmt.Sprintf("%.2fkm", entry.Distance/1000.0) // Convert units to km if distance is in units
		default: entry.Value = entry.Kills
		}

		entry.Rank = rank
		entries = append(entries, entry)
		rank++
	}

	var total uint64
	if err := h.ch.QueryRow(ctx, "SELECT uniq(player_id) FROM mohaa_stats.player_stats_daily").Scan(&total); err != nil {
		h.logger.Errorw("Failed to scan total leaderboard count", "error", err)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"players": entries,
		"total":   total,
		"page":    page,
		"stat":    stat,
	})
}

// GetWeeklyLeaderboard returns weekly stats
func (h *Handler) GetWeeklyLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			argMax(actor_name, timestamp) as actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' 
		  AND actor_id != 'world'
		  AND timestamp >= now() - INTERVAL 7 DAY
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT 100
	`)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry models.LeaderboardEntry
		var name string
		if err := rows.Scan(&entry.PlayerID, &name, &entry.Kills); err != nil {
			continue
		}
		entry.Rank = rank
		entry.PlayerName = name
		entries = append(entries, entry)
		rank++
	}

	h.jsonResponse(w, http.StatusOK, entries)
}

// GetWeaponLeaderboard returns top players for a specific weapon
func (h *Handler) GetWeaponLeaderboard(w http.ResponseWriter, r *http.Request) {
	weapon := chi.URLParam(r, "weapon")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			argMax(actor_name, timestamp) as actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' 
		  AND actor_weapon = ?
		  AND actor_id != 'world'
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT 100
	`, weapon)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry models.LeaderboardEntry
		var name string
		if err := rows.Scan(&entry.PlayerID, &name, &entry.Kills); err != nil {
			continue
		}
		entry.Rank = rank
		entry.PlayerName = name
		entries = append(entries, entry)
		rank++
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"weapon":      weapon,
		"leaderboard": entries,
	})
}

// GetMapLeaderboard returns top players on a specific map
func (h *Handler) GetMapLeaderboard(w http.ResponseWriter, r *http.Request) {
	mapName := chi.URLParam(r, "map")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			argMax(actor_name, timestamp) as actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' 
		  AND map_name = ?
		  AND actor_id != 'world'
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT 100
	`, mapName)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry models.LeaderboardEntry
		var name string
		if err := rows.Scan(&entry.PlayerID, &name, &entry.Kills); err != nil {
			continue
		}
		entry.Rank = rank
		entry.PlayerName = name
		entries = append(entries, entry)
		rank++
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"map":         mapName,
		"leaderboard": entries,
	})
}

// GetPlayerStats returns comprehensive stats for a player
// @Summary Get Player Stats
// @Description Fetch detailed statistics for a player using their GUID
// @Tags Player
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {object} models.PlayerStatsResponse "Player Stats"
// @Failure 404 {object} map[string]string "Not Found"
// @Router /stats/player/{guid} [get]
func (h *Handler) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// 1. Get Deep Stats (Combines Combat, Weapons, Movement, Stance, etc.)
	deepStats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get deep stats", "guid", guid, "error", err)
		// Fallback to empty if failed, but try to proceed
		deepStats = &models.DeepStats{}
	}

	// 2. Get Performance History (Trend)
	// We re-implement the query here to ensure data flow
	perfRows, err := h.ch.Query(ctx, `
		SELECT 
			toString(match_id) as match_id,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			min(timestamp) as played_at
		FROM mohaa_stats.raw_events
		WHERE match_id IN (
			SELECT match_id FROM mohaa_stats.raw_events 
			WHERE actor_id = ? OR target_id = ?
			GROUP BY match_id 
			ORDER BY max(timestamp) DESC 
			LIMIT 20
		)
		GROUP BY match_id
		ORDER BY played_at ASC
	`, guid, guid, guid, guid)

	performance := make([]models.PerformancePoint, 0)
	if err == nil {
		defer perfRows.Close()
		for perfRows.Next() {
			var mid string
			var k, d uint64
			var t time.Time
			if err := perfRows.Scan(&mid, &k, &d, &t); err == nil {
				kd := float64(k)
				if d > 0 {
					kd = float64(k) / float64(d)
				}
				performance = append(performance, models.PerformancePoint{
					MatchID:  mid,
					Kills:    k,
					Deaths:   d,
					KD:       kd,
					PlayedAt: t.Unix(),
				})
			}
		}
	}

	// 3. Get Map Stats (Summary for dashboard)
	mapRows, err := h.ch.Query(ctx, `
		SELECT 
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			count(DISTINCT match_id) as matches,
			0 as wins
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?) AND map_name != ''
		GROUP BY map_name
		ORDER BY matches DESC
		LIMIT 5
	`, guid, guid, guid, guid) // Fixed params for OR clause

	maps := make([]models.PlayerMapStats, 0)
	if err == nil {
		defer mapRows.Close()
		for mapRows.Next() {
			var name string
			var k, d, m, w uint64
			if err := mapRows.Scan(&name, &k, &d, &m, &w); err == nil {
				maps = append(maps, models.PlayerMapStats{
					MapName:       name,
					Kills:         k,
					Deaths:        d,
					MatchesPlayed: m,
					MatchesWon:    w,
				})
			}
		}
	}

	// 4. Get Matches List (Recent)
	matchRows, err := h.ch.Query(ctx, `
		SELECT 
			toString(match_id) as match_id,
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			min(timestamp) as started
		FROM mohaa_stats.raw_events
		WHERE actor_id = ? OR target_id = ?
		GROUP BY match_id, map_name
		ORDER BY started DESC
		LIMIT 10
	`, guid, guid, guid, guid)

	matches := make([]models.RecentMatch, 0)
	if err == nil {
		defer matchRows.Close()
		for matchRows.Next() {
			var mid, mn string
			var k, d uint64
			var t time.Time
			if err := matchRows.Scan(&mid, &mn, &k, &d, &t); err == nil {
				matches = append(matches, models.RecentMatch{
					MatchID: mid,
					MapName: mn,
					Kills:   k,
					Deaths:  d,
					Date:    t.Unix(),
				})
			}
		}
	}

	// Construct Flat Player Object
	player := models.PlayerStats{
		GUID:       guid,
		Name:       "Unknown Soldier",
		PlayerName: "Unknown Soldier",

		// Combat
		Kills:       deepStats.Combat.Kills,
		Deaths:      deepStats.Combat.Deaths,
		KDRatio:     deepStats.Combat.KDRatio,
		Headshots:   deepStats.Combat.Headshots,
		Accuracy:    deepStats.Accuracy.Overall,
		DamageDealt: deepStats.Combat.DamageDealt,
		DamageTaken: deepStats.Combat.DamageTaken,
		Suicides:    deepStats.Combat.Suicides,
		TeamKills:   deepStats.Combat.TeamKills,
		BashKills:   deepStats.Combat.BashKills,

		// Body Parts
		TorsoKills: deepStats.Combat.TorsoKills,
		LimbKills:  deepStats.Combat.LimbKills,

		// Session
		MatchesPlayed:   deepStats.Session.MatchesPlayed,
		MatchesWon:      deepStats.Session.Wins,
		WinRate:         deepStats.Session.WinRate,
		PlaytimeSeconds: deepStats.Session.PlaytimeHours * 3600,

		// Movement
		DistanceMeters: deepStats.Movement.TotalDistanceKm * 1000, // Return meters
		Jumps:          deepStats.Movement.JumpCount,

		// Stance
		StandingKills:  deepStats.Stance.StandingKills,
		CrouchingKills: deepStats.Stance.CrouchKills,
		ProneKills:     deepStats.Stance.ProneKills,

		// Lists
		Weapons:       deepStats.Weapons,
		Maps:          maps,
		Performance:   performance,
		RecentMatches: matches,
		Achievements:  []string{},
	}

	// Try to get name
	var name string
	if err := h.ch.QueryRow(ctx, "SELECT any(actor_name) FROM mohaa_stats.raw_events WHERE actor_id = ?", guid).Scan(&name); err == nil && name != "" {
		player.Name = name
		player.PlayerName = name
	}

	h.jsonResponse(w, http.StatusOK, models.PlayerStatsResponse{
		Player: player,
	})
}

// GetPlayerAchievements returns player achievements
func (h *Handler) GetPlayerAchievements(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	achievements, err := h.achievements.GetPlayerAchievements(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get player achievements", "error", err, "guid", guid)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get achievements")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"achievements": achievements,
	})
}

// ListAchievements returns a message directing to SMF database
// Achievement definitions are stored in SMF MariaDB, not Go
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
	// Recent achievement unlocks are stored in SMF database
	// Return empty array - frontend should query SMF directly or use PHP endpoint
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}

// GetAchievementLeaderboard returns players ranked by achievement points
func (h *Handler) GetAchievementLeaderboard(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	// Achievement data is stored in SMF database - return empty array
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}

// GetPlayerMatches returns recent matches for a player
func (h *Handler) GetPlayerMatches(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			toString(match_id) as match_id,
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			min(timestamp) as started,
			max(timestamp) as ended
		FROM mohaa_stats.raw_events
		WHERE match_id IN (
			SELECT DISTINCT match_id FROM mohaa_stats.raw_events WHERE actor_id = ? OR target_id = ?
		)
		GROUP BY match_id, map_name
		ORDER BY started DESC
		LIMIT 50
	`, guid, guid, guid, guid)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type MatchSummary struct {
		MatchID   string    `json:"match_id"`
		MapName   string    `json:"map_name"`
		Kills     uint64    `json:"kills"`
		Deaths    uint64    `json:"deaths"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
	}

	var matches []MatchSummary
	for rows.Next() {
		var m MatchSummary
		if err := rows.Scan(&m.MatchID, &m.MapName, &m.Kills, &m.Deaths, &m.StartedAt, &m.EndedAt); err != nil {
			continue
		}
		matches = append(matches, m)
	}

	h.jsonResponse(w, http.StatusOK, matches)
}

// GetPlayerDeepStats returns massive aggregated stats for a player
func (h *Handler) GetPlayerDeepStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get deep stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate deep stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerCombatStats returns only combat subset of deep stats
func (h *Handler) GetPlayerCombatStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get combat stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate combat stats")
		return
	}

	// Return only combat section
	h.jsonResponse(w, http.StatusOK, stats.Combat)
}

// GetPlayerMovementStats returns only movement subset of deep stats
func (h *Handler) GetPlayerMovementStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get movement stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate movement stats")
		return
	}

	// Return only movement section
	h.jsonResponse(w, http.StatusOK, stats.Movement)
}

// GetPlayerStanceStats returns only stance subset of deep stats
func (h *Handler) GetPlayerStanceStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get stance stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate stance stats")
		return
	}

	// Return only stance section
	h.jsonResponse(w, http.StatusOK, stats.Stance)
}

// GetPlayerVehicleStats returns vehicle and turret statistics
func (h *Handler) GetPlayerVehicleStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.advancedStats.GetVehicleStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get vehicle stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate vehicle stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerGameFlowStats returns round/objective/team statistics
func (h *Handler) GetPlayerGameFlowStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.advancedStats.GetGameFlowStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get game flow stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate game flow stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerWorldStats returns world interaction statistics
func (h *Handler) GetPlayerWorldStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.advancedStats.GetWorldStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get world stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate world stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerBotStats returns bot-related statistics
func (h *Handler) GetPlayerBotStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.advancedStats.GetBotStats(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get bot stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate bot stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerWeaponStats returns per-weapon stats for a player
func (h *Handler) GetPlayerWeaponStats(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	h.logger.Infow("GetPlayerWeaponStats", "guid", guid)

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_weapon,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' AND actor_id = ? AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
	`, guid)
	if err != nil {
		h.logger.Errorw("Failed to query weapon stats", "error", err, "guid", guid)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed: "+err.Error())
		return
	}
	defer rows.Close()

	weapons := []models.WeaponStats{} // Initialize as empty slice, not nil
	for rows.Next() {
		var w models.WeaponStats
		if err := rows.Scan(&w.Weapon, &w.Kills); err != nil {
			h.logger.Errorw("Failed to scan weapon row", "error", err)
			continue
		}
		weapons = append(weapons, w)
	}

	h.logger.Infow("GetPlayerWeaponStats result", "guid", guid, "count", len(weapons))
	h.jsonResponse(w, http.StatusOK, weapons)
}

// GetPlayerHeatmap returns kill position data for heatmap visualization
func (h *Handler) GetPlayerHeatmap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	mapName := chi.URLParam(r, "map")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_pos_x,
			actor_pos_y,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' 
		  AND actor_id = ? 
		  AND map_name = ?
		  AND actor_pos_x != 0
		GROUP BY 
			round(actor_pos_x / 100) * 100 as actor_pos_x,
			round(actor_pos_y / 100) * 100 as actor_pos_y
	`, guid, mapName)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	var points []models.HeatmapPoint
	for rows.Next() {
		var p models.HeatmapPoint
		if err := rows.Scan(&p.X, &p.Y, &p.Count); err != nil {
			continue
		}
		points = append(points, p)
	}

	h.jsonResponse(w, http.StatusOK, models.HeatmapData{
		MapName: mapName,
		Points:  points,
	})
}

// GetPlayerDeathHeatmap returns death position data for heatmap visualization
func (h *Handler) GetPlayerDeathHeatmap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	mapName := chi.URLParam(r, "map")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			target_pos_x,
			target_pos_y,
			count() as deaths
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' 
		  AND target_id = ? 
		  AND map_name = ?
		  AND target_pos_x != 0
		GROUP BY 
			round(target_pos_x / 100) * 100 as target_pos_x,
			round(target_pos_y / 100) * 100 as target_pos_y
	`, guid, mapName)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	var points []models.HeatmapPoint
	for rows.Next() {
		var p models.HeatmapPoint
		if err := rows.Scan(&p.X, &p.Y, &p.Count); err != nil {
			continue
		}
		points = append(points, p)
	}

	h.jsonResponse(w, http.StatusOK, models.HeatmapData{
		MapName: mapName,
		Points:  points,
		Type:    "deaths",
	})
}

// GetPlayerPerformanceHistory returns K/D history over last 20 matches
func (h *Handler) GetPlayerPerformanceHistory(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Fetch matches chronologically
	// Deaths = when player is target of a kill event (target_id = guid)
	rows, err := h.ch.Query(ctx, `
		SELECT 
			toString(match_id) as match_id,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			min(timestamp) as played_at
		FROM mohaa_stats.raw_events
		WHERE match_id IN (
			SELECT match_id FROM mohaa_stats.raw_events 
			WHERE actor_id = ? OR target_id = ?
			GROUP BY match_id 
			ORDER BY max(timestamp) DESC 
			LIMIT 20
		)
		GROUP BY match_id
		ORDER BY played_at ASC
	`, guid, guid, guid, guid)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type PerformancePoint struct {
		MatchID  string  `json:"match_id"`
		Kills    uint64  `json:"kills"`
		Deaths   uint64  `json:"deaths"`
		KD       float64 `json:"kd"`
		PlayedAt float64 `json:"played_at"`
	}

	history := []PerformancePoint{} // Ensure non-nil
	for rows.Next() {
		var p PerformancePoint
		var t time.Time // Scan into time.Time
		if err := rows.Scan(&p.MatchID, &p.Kills, &p.Deaths, &t); err != nil {
			h.logger.Warnw("Scan failed in performance", "error", err)
			continue
		}
		p.PlayedAt = float64(t.Unix()) // Convert to unix timestamp for JSON
		if p.Deaths > 0 {
			p.KD = float64(p.Kills) / float64(p.Deaths)
		} else {
			p.KD = float64(p.Kills)
		}
		history = append(history, p)
	}

	h.jsonResponse(w, http.StatusOK, history)
}

// GetPlayerBodyHeatmap returns hit location distribution
// GetPlayerBodyHeatmap returns hit location distribution
func (h *Handler) GetPlayerBodyHeatmap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Query breakdown of hit locations where this player was the TARGET (victim)
	rows, err := h.ch.Query(ctx, `
		SELECT 
			hitloc as body_part,
			count() as hits
		FROM mohaa_stats.raw_events
		WHERE event_type IN ('weapon_hit', 'kill') 
		  AND target_id = ? 
		  AND hitloc != ''
		GROUP BY body_part
	`, guid)
	if err != nil {
		h.logger.Errorw("Failed to query body heatmap", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	heatmap := make(map[string]uint64)
	for rows.Next() {
		var part string
		var hits uint64
		if err := rows.Scan(&part, &hits); err != nil {
			continue
		}
		heatmap[part] = hits
	}

	h.jsonResponse(w, http.StatusOK, heatmap)
}

// GetMatchDetails returns full details for a match
func (h *Handler) GetMatchDetails(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	// Get match summary - use any() for map_name since we need aggregate
	row := h.ch.QueryRow(ctx, `
		SELECT 
			any(map_name) as map_name,
			min(timestamp) as started,
			max(timestamp) as ended,
			countIf(event_type = 'kill') as total_kills,
			uniq(actor_id) as unique_players
		FROM mohaa_stats.raw_events
		WHERE match_id = ?
	`, matchID)

	var summary struct {
		MapName       string    `json:"map_name"`
		StartedAt     time.Time `json:"started_at"`
		EndedAt       time.Time `json:"ended_at"`
		TotalKills    uint64    `json:"total_kills"`
		UniquePlayers uint64    `json:"unique_players"`
	}

	if err := row.Scan(&summary.MapName, &summary.StartedAt, &summary.EndedAt, &summary.TotalKills, &summary.UniquePlayers); err != nil {
		h.errorResponse(w, http.StatusNotFound, "Match not found")
		return
	}

	// Get player scoreboard - needs subquery for deaths since death = being target_id in kill events
	rows, err := h.ch.Query(ctx, `
		SELECT 
			p.player_id as actor_id,
			p.player_name as actor_name,
			p.kills,
			ifNull(d.deaths, 0) as deaths,
			p.headshots
		FROM (
			SELECT 
				actor_id as player_id,
				any(actor_name) as player_name,
				countIf(event_type = 'kill') as kills,
				countIf(event_type = 'headshot') as headshots
			FROM mohaa_stats.raw_events
			WHERE match_id = ? AND actor_id != '' AND actor_id != 'world'
			GROUP BY actor_id
		) p
		LEFT JOIN (
			SELECT target_id, count() as deaths
			FROM mohaa_stats.raw_events
			WHERE match_id = ? AND event_type = 'kill' AND target_id != ''
			GROUP BY target_id
		) d ON p.player_id = d.target_id
		ORDER BY p.kills DESC
	`, matchID, matchID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type PlayerScore struct {
		PlayerID   string `json:"player_id"`
		PlayerName string `json:"player_name"`
		Kills      uint64 `json:"kills"`
		Deaths     uint64 `json:"deaths"`
		Headshots  uint64 `json:"headshots"`
	}

	var scoreboard []PlayerScore
	for rows.Next() {
		var p PlayerScore
		if err := rows.Scan(&p.PlayerID, &p.PlayerName, &p.Kills, &p.Deaths, &p.Headshots); err != nil {
			continue
		}
		scoreboard = append(scoreboard, p)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"match_id":   matchID,
		"summary":    summary,
		"scoreboard": scoreboard,
	})
}

// GetMatchHeatmap returns kill/death locations for a specific match
func (h *Handler) GetMatchHeatmap(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	// Query individual kill events with coordinates
	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			target_id,
			actor_pos_x,
			actor_pos_y,
			target_pos_x,
			target_pos_y
		FROM mohaa_stats.raw_events
		WHERE match_id = ? 
		  AND event_type = 'kill'
		  AND actor_pos_x != 0 AND target_pos_x != 0
		LIMIT 2000
	`, matchID)
	if err != nil {
		h.logger.Errorw("Failed to query match heatmap", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type Point struct {
		ID    int     `json:"id"`
		Type  string  `json:"type"` // "kill" or "death"
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		Label string  `json:"label"`
	}

	var points []Point
	id := 0

	for rows.Next() {
		var actorID, targetID string
		var ax, ay, tx, ty float64
		if err := rows.Scan(&actorID, &targetID, &ax, &ay, &tx, &ty); err != nil {
			continue
		}

		// Killer position (green)
		points = append(points, Point{
			ID:    id,
			Type:  "kill",
			X:     ax,
			Y:     ay,
			Label: "Killer: " + actorID,
		})
		id++

		// Victim position (red)
		points = append(points, Point{
			ID:    id,
			Type:  "death",
			X:     tx,
			Y:     ty,
			Label: "Victim: " + targetID,
		})
		id++
	}

	h.jsonResponse(w, http.StatusOK, points)
}

// GetMatchTimeline returns chronological events for match replay
func (h *Handler) GetMatchTimeline(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT 
			timestamp,
			event_type,
			actor_name,
			target_name,
			actor_weapon,
			hitloc
		FROM mohaa_stats.raw_events
		WHERE match_id = ? AND event_type IN ('kill', 'round_start', 'round_end')
		ORDER BY timestamp
		LIMIT 1000
	`, matchID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type TimelineEvent struct {
		Timestamp  time.Time `json:"timestamp"`
		EventType  string    `json:"event_type"`
		ActorName  string    `json:"actor_name"`
		TargetName string    `json:"target_name"`
		Weapon     string    `json:"weapon"`
		Hitloc     string    `json:"hitloc"`
	}

	var events []TimelineEvent
	for rows.Next() {
		var e TimelineEvent
		if err := rows.Scan(&e.Timestamp, &e.EventType, &e.ActorName, &e.TargetName, &e.Weapon, &e.Hitloc); err != nil {
			continue
		}
		events = append(events, e)
	}

	h.jsonResponse(w, http.StatusOK, events)
}

// GetServerStats returns stats for a specific server
func (h *Handler) GetServerStats(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "serverId")
	ctx := r.Context()

	var response models.ServerStatsResponse
	response.ServerID = serverID

	// 1. Get Aggregate Totals
	// Using a single query to get multiple aggregates
	// Note: total_deaths = total_kills for global stats (each kill = one death)
	row := h.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'kill') as total_deaths,
			uniq(match_id) as total_matches,
			uniq(actor_id) as unique_players,
			toFloat64(0) as total_playtime,
			max(timestamp) as last_activity
		FROM mohaa_stats.raw_events
		WHERE server_id = ?
	`, serverID)

	if err := row.Scan(
		&response.TotalKills,
		&response.TotalDeaths,
		&response.TotalMatches,
		&response.UniquePlayers,
		&response.TotalPlaytime,
		&response.LastActivity,
	); err != nil {
		h.logger.Errorw("Failed to query server totals", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}

	// 2. Top Killers Leaderboard
	rows, err := h.ch.Query(ctx, `
		SELECT actor_id, any(actor_name), count() as val
		FROM mohaa_stats.raw_events
		WHERE server_id = ? AND event_type = 'kill' AND actor_id != ''
		GROUP BY actor_id
		ORDER BY val DESC
		LIMIT 10
	`, serverID)
	if err == nil {
		rank := 1
		for rows.Next() {
			var e models.ServerLeaderboardEntry
			rows.Scan(&e.PlayerID, &e.PlayerName, &e.Value)
			e.Rank = rank
			response.TopKillers = append(response.TopKillers, e)
			rank++
		}
		rows.Close()
	}

	// 3. Map Stats
	rows, err = h.ch.Query(ctx, `
		SELECT map_name, count() as times_played
		FROM mohaa_stats.raw_events
		WHERE server_id = ? AND event_type = 'match_start'
		GROUP BY map_name
		ORDER BY times_played DESC
		LIMIT 10
	`, serverID)
	if err == nil {
		for rows.Next() {
			var m models.ServerMapStat
			rows.Scan(&m.MapName, &m.TimesPlayed)
			response.MapStats = append(response.MapStats, m)
		}
		rows.Close()
	}

	h.jsonResponse(w, http.StatusOK, response)
}

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

// GetLiveMatches returns currently active matches
func (h *Handler) GetLiveMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all live matches from Redis
	matchData, err := h.redis.HGetAll(ctx, "live_matches").Result()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch live matches")
		return
	}

	var matches []models.LiveMatch
	for _, data := range matchData {
		var match models.LiveMatch
		if err := json.Unmarshal([]byte(data), &match); err == nil {
			matches = append(matches, match)
		}
	}

	h.jsonResponse(w, http.StatusOK, matches)
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

// GetGlobalActivity returns heat map data for server activity
func (h *Handler) GetGlobalActivity(w http.ResponseWriter, r *http.Request) {
	activity, err := h.serverStats.GetGlobalActivity(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get global activity", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.jsonResponse(w, http.StatusOK, activity)
}

// GetMapPopularity returns stats for map usage
func (h *Handler) GetMapPopularity(w http.ResponseWriter, r *http.Request) {
	stats, err := h.serverStats.GetMapPopularity(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get map popularity", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerPlaystyle returns the calculated playstyle badge
func (h *Handler) GetPlayerPlaystyle(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	badge, err := h.gamification.GetPlaystyle(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get playstyle", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal error")
		return
	}
	h.jsonResponse(w, http.StatusOK, badge)
}

// GetMatchAdvancedDetails returns deep analysis for a match
func (h *Handler) GetMatchAdvancedDetails(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	details, err := h.matchReport.GetMatchDetails(r.Context(), matchID)
	if err != nil {
		h.logger.Errorw("Failed to get match details", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal error")
		return
	}
	h.jsonResponse(w, http.StatusOK, details)
}

// GetLeaderboardCards was moved to cards.go to support the massive dashboard

// ============================================================================
// HELPERS
// ============================================================================
// MAP ENDPOINTS
// ============================================================================

// GetMapStats returns all maps with their statistics
func (h *Handler) GetMapStats(w http.ResponseWriter, r *http.Request) {
	maps, err := h.getMapsList(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get map stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.jsonResponse(w, http.StatusOK, maps)
}

// GetMapsList returns a simple list of maps for dropdowns
func (h *Handler) GetMapsList(w http.ResponseWriter, r *http.Request) {
	maps, err := h.getMapsList(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get maps list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Return simplified list for dropdown
	type mapItem struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}

	result := make([]mapItem, len(maps))
	for i, m := range maps {
		result[i] = mapItem{
			Name:        m.Name,
			DisplayName: formatMapName(m.Name),
		}
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"maps": result})
}

// GetMapDetail returns detailed statistics for a single map
func (h *Handler) GetMapDetail(w http.ResponseWriter, r *http.Request) {
	mapID := chi.URLParam(r, "mapId")
	if mapID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Map ID required")
		return
	}

	ctx := r.Context()
	mapInfo, err := h.getMapDetails(ctx, mapID)
	if err != nil {
		h.logger.Errorw("Failed to get map details", "error", err, "map", mapID)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get top players on this map
	var topPlayers []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Kills  int    `json:"kills"`
		Deaths int    `json:"deaths"`
	}

	rows, err := h.ch.Query(ctx, `
		SELECT 
			player_guid as id,
			any(player_name) as name,
			countIf(event_type = 'kill' AND raw_json->>'attacker_guid' = player_guid) as kills,
			countIf(event_type = 'kill' AND raw_json->>'victim_guid' = player_guid) as deaths
		FROM mohaa_stats.raw_events
		WHERE map_name = ?
		GROUP BY player_guid
		ORDER BY kills DESC
		LIMIT 25
	`, mapID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Kills  int    `json:"kills"`
				Deaths int    `json:"deaths"`
			}
			if err := rows.Scan(&p.ID, &p.Name, &p.Kills, &p.Deaths); err == nil {
				topPlayers = append(topPlayers, p)
			}
		}
	}

	// Get heatmap data
	heatmapData := make(map[string]interface{})
	killsHeatmap, _ := h.getMapHeatmapData(ctx, mapID, "kills")
	deathsHeatmap, _ := h.getMapHeatmapData(ctx, mapID, "deaths")
	heatmapData["kills"] = killsHeatmap
	heatmapData["deaths"] = deathsHeatmap

	response := map[string]interface{}{
		"map_name":       mapInfo.Name,
		"display_name":   formatMapName(mapInfo.Name),
		"total_matches":  mapInfo.TotalMatches,
		"total_kills":    mapInfo.TotalKills,
		"total_playtime": int64(mapInfo.AvgDuration) * mapInfo.TotalMatches,
		"avg_duration":   mapInfo.AvgDuration,
		"top_players":    topPlayers,
		"heatmap_data":   heatmapData,
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// formatMapName converts map filename to display name
func formatMapName(name string) string {
	// Remove common prefixes
	displayName := name
	prefixes := []string{"mp_", "dm_", "obj_", "lib_"}
	for _, prefix := range prefixes {
		if len(displayName) > len(prefix) && displayName[:len(prefix)] == prefix {
			displayName = displayName[len(prefix):]
			break
		}
	}
	// Capitalize first letter
	if len(displayName) > 0 {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}
	return displayName
}

// getMapHeatmapData returns heatmap coordinates for a map
func (h *Handler) getMapHeatmapData(ctx context.Context, mapID, heatmapType string) ([]map[string]interface{}, error) {
	eventType := "kill"
	if heatmapType == "deaths" {
		eventType = "death"
	}

	rows, err := h.ch.Query(ctx, `
		SELECT 
			toFloat64OrZero(raw_json->>'pos_x') as x,
			toFloat64OrZero(raw_json->>'pos_y') as y,
			count() as intensity
		FROM mohaa_stats.raw_events
		WHERE map_name = ? AND event_type = ?
			AND raw_json->>'pos_x' != '' AND raw_json->>'pos_y' != ''
		GROUP BY x, y
		HAVING intensity > 0
		ORDER BY intensity DESC
		LIMIT 500
	`, mapID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var x, y float64
		var intensity int64
		if err := rows.Scan(&x, &y, &intensity); err == nil {
			result = append(result, map[string]interface{}{
				"x":     x,
				"y":     y,
				"value": intensity,
			})
		}
	}
	return result, nil
}

// ============================================================================
// GAME TYPE ENDPOINTS
// ============================================================================

// GetGameTypeStats returns all game types with their statistics (derived from map prefixes)
func (h *Handler) GetGameTypeStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query to aggregate stats by game type prefix derived from map_name
	rows, err := h.ch.Query(ctx, `
		SELECT 
			multiIf(
				startsWith(lower(map_name), 'dm'), 'dm',
				startsWith(lower(map_name), 'tdm'), 'tdm',
				startsWith(lower(map_name), 'obj'), 'obj',
				startsWith(lower(map_name), 'lib'), 'lib',
				startsWith(lower(map_name), 'ctf'), 'ctf',
				startsWith(lower(map_name), 'ffa'), 'ffa',
				'other'
			) as game_type,
			count(DISTINCT match_id) as total_matches,
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'kill') as total_deaths,
			count(DISTINCT actor_id) as unique_players,
			count(DISTINCT map_name) as map_count
		FROM mohaa_stats.raw_events
		WHERE map_name != ''
		GROUP BY game_type
		ORDER BY total_matches DESC
	`)
	if err != nil {
		h.logger.Errorw("Failed to get game type stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var gameType string
		var matches, kills, deaths, players, mapCount uint64
		if err := rows.Scan(&gameType, &matches, &kills, &deaths, &players, &mapCount); err == nil {
			info := gameTypeInfo[gameType]
			result = append(result, map[string]interface{}{
				"id":             gameType,
				"name":           formatGameTypeName(gameType),
				"description":    info.Description,
				"icon":           info.Icon,
				"total_matches":  matches,
				"total_kills":    kills,
				"total_deaths":   deaths,
				"unique_players": players,
				"map_count":      mapCount,
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, result)
}

// GetGameTypesList returns a simple list of game types for dropdowns
func (h *Handler) GetGameTypesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT DISTINCT
			multiIf(
				startsWith(lower(map_name), 'dm'), 'dm',
				startsWith(lower(map_name), 'tdm'), 'tdm',
				startsWith(lower(map_name), 'obj'), 'obj',
				startsWith(lower(map_name), 'lib'), 'lib',
				startsWith(lower(map_name), 'ctf'), 'ctf',
				startsWith(lower(map_name), 'ffa'), 'ffa',
				'other'
			) as game_type
		FROM mohaa_stats.raw_events
		WHERE map_name != ''
		ORDER BY game_type
	`)
	if err != nil {
		h.logger.Errorw("Failed to get game types list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	var result []map[string]string
	for rows.Next() {
		var gameType string
		if err := rows.Scan(&gameType); err == nil {
			result = append(result, map[string]string{
				"id":           gameType,
				"name":         formatGameTypeName(gameType),
				"display_name": formatGameTypeName(gameType),
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"gametypes": result})
}

// GetGameTypeDetail returns detailed statistics for a single game type
func (h *Handler) GetGameTypeDetail(w http.ResponseWriter, r *http.Request) {
	gameType := chi.URLParam(r, "gameType")
	if gameType == "" {
		h.errorResponse(w, http.StatusBadRequest, "Game type required")
		return
	}

	ctx := r.Context()

	// Build map pattern for this game type
	mapPattern := gameType + "%"

	// Get aggregate stats
	// Note: total_deaths = total_kills for global stats (each kill = one death)
	var totalMatches, totalKills, totalDeaths, uniquePlayers, mapCount uint64
	row := h.ch.QueryRow(ctx, `
		SELECT 
			count(DISTINCT match_id) as total_matches,
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'kill') as total_deaths,
			count(DISTINCT actor_id) as unique_players,
			count(DISTINCT map_name) as map_count
		FROM mohaa_stats.raw_events
		WHERE lower(map_name) LIKE ?
	`, mapPattern)
	row.Scan(&totalMatches, &totalKills, &totalDeaths, &uniquePlayers, &mapCount)

	// Get maps in this game type
	mapRows, err := h.ch.Query(ctx, `
		SELECT 
			map_name,
			count(DISTINCT match_id) as matches,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		WHERE lower(map_name) LIKE ?
		GROUP BY map_name
		ORDER BY matches DESC
	`, mapPattern)

	var maps []map[string]interface{}
	if err == nil {
		defer mapRows.Close()
		for mapRows.Next() {
			var mapName string
			var matches, kills uint64
			if err := mapRows.Scan(&mapName, &matches, &kills); err == nil {
				maps = append(maps, map[string]interface{}{
					"name":         mapName,
					"display_name": formatMapName(mapName),
					"matches":      matches,
					"kills":        kills,
				})
			}
		}
	}

	info := gameTypeInfo[gameType]
	response := map[string]interface{}{
		"id":             gameType,
		"name":           formatGameTypeName(gameType),
		"description":    info.Description,
		"icon":           info.Icon,
		"total_matches":  totalMatches,
		"total_kills":    totalKills,
		"total_deaths":   totalDeaths,
		"unique_players": uniquePlayers,
		"map_count":      mapCount,
		"maps":           maps,
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// GetGameTypeLeaderboard returns top players for a specific game type
func (h *Handler) GetGameTypeLeaderboard(w http.ResponseWriter, r *http.Request) {
	gameType := chi.URLParam(r, "gameType")
	if gameType == "" {
		h.errorResponse(w, http.StatusBadRequest, "Game type required")
		return
	}

	ctx := r.Context()
	mapPattern := gameType + "%"

	// For per-player deaths we need to join kills as actor with kills as target
	rows, err := h.ch.Query(ctx, `
		SELECT 
			p.player_id as id,
			p.player_name as name,
			p.kills,
			ifNull(d.deaths, 0) as deaths
		FROM (
			SELECT 
				actor_id as player_id,
				any(actor_name) as player_name,
				countIf(event_type = 'kill') as kills
			FROM mohaa_stats.raw_events
			WHERE lower(map_name) LIKE ? AND actor_id != ''
			GROUP BY actor_id
		) p
		LEFT JOIN (
			SELECT target_id, count() as deaths
			FROM mohaa_stats.raw_events
			WHERE lower(map_name) LIKE ? AND event_type = 'kill' AND target_id != ''
			GROUP BY target_id
		) d ON p.player_id = d.target_id
		ORDER BY p.kills DESC
		LIMIT 25
	`, mapPattern, mapPattern)

	if err != nil {
		h.logger.Errorw("Failed to get game type leaderboard", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	var leaderboard []map[string]interface{}
	rank := 1
	for rows.Next() {
		var id, name string
		var kills, deaths uint64
		if err := rows.Scan(&id, &name, &kills, &deaths); err == nil {
			leaderboard = append(leaderboard, map[string]interface{}{
				"rank":   rank,
				"id":     id,
				"name":   name,
				"kills":  kills,
				"deaths": deaths,
			})
			rank++
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"leaderboard": leaderboard,
		"game_type":   gameType,
	})
}

// ============================================================================
// HELPERS
// ============================================================================

// Game type metadata - maps prefix to display info
var gameTypeInfo = map[string]struct {
	Name        string
	Description string
	Icon        string
}{
	"dm":  {"Deathmatch", "Free-for-all combat", "💀"},
	"tdm": {"Team Deathmatch", "Team-based combat", "⚔️"},
	"obj": {"Objective", "Mission-based gameplay", "🎯"},
	"lib": {"Liberation", "Territory control", "🏴"},
	"ctf": {"Capture the Flag", "Flag-based objectives", "🚩"},
	"ffa": {"Free For All", "Every player for themselves", "🔥"},
}

// extractGameType derives game type from map name prefix
func extractGameType(mapName string) string {
	parts := strings.Split(mapName, "/")
	if len(parts) > 0 {
		prefix := strings.ToLower(parts[0])
		// Handle common prefixes
		if strings.HasPrefix(prefix, "dm") {
			return "dm"
		} else if strings.HasPrefix(prefix, "tdm") {
			return "tdm"
		} else if strings.HasPrefix(prefix, "obj") {
			return "obj"
		} else if strings.HasPrefix(prefix, "lib") {
			return "lib"
		} else if strings.HasPrefix(prefix, "ctf") {
			return "ctf"
		} else if strings.HasPrefix(prefix, "ffa") {
			return "ffa"
		}
		return prefix
	}
	// Fallback: check underscore prefix
	if idx := strings.Index(mapName, "_"); idx > 0 {
		return strings.ToLower(mapName[:idx])
	}
	return "unknown"
}

// formatGameTypeName converts prefix to display name
func formatGameTypeName(prefix string) string {
	if info, ok := gameTypeInfo[prefix]; ok {
		return info.Name
	}
	return strings.ToUpper(prefix)
}

// ============================================================================
// WEAPON ENDPOINTS
// ============================================================================

// GetWeaponsList returns all weapons for dropdowns
func (h *Handler) GetWeaponsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT DISTINCT actor_weapon 
		FROM mohaa_stats.raw_events 
		WHERE actor_weapon != '' AND event_type IN ('kill', 'weapon_fire')
		ORDER BY actor_weapon
	`)
	if err != nil {
		h.logger.Errorw("Failed to get weapons list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	type weaponItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var result []weaponItem
	for rows.Next() {
		var wName string
		if err := rows.Scan(&wName); err == nil {
			result = append(result, weaponItem{
				ID:   wName,
				Name: wName,
			})
		}
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"weapons": result})
}

// GetWeaponDetail returns detailed statistics for a single weapon
func (h *Handler) GetWeaponDetail(w http.ResponseWriter, r *http.Request) {
	weapon := chi.URLParam(r, "weapon")
	if weapon == "" {
		h.errorResponse(w, http.StatusBadRequest, "Weapon required")
		return
	}

	ctx := r.Context()

	// Aggregate stats
	row := h.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'headshot') as total_headshots,
			countIf(event_type = 'weapon_fire') as shots_fired,
			countIf(event_type = 'weapon_hit') as shots_hit,
			uniq(actor_id) as unique_users,
			max(timestamp) as last_used,
			avgIf(distance, event_type='kill') as avg_kill_distance
		FROM mohaa_stats.raw_events
		WHERE actor_weapon = ?
	`, weapon)

	var stats struct {
		Name            string    `json:"name"`
		TotalKills      uint64    `json:"total_kills"`
		TotalHeadshots  uint64    `json:"total_headshots"`
		ShotsFired      uint64    `json:"shots_fired"`
		ShotsHit        uint64    `json:"shots_hit"`
		UniqueUsers     uint64    `json:"unique_users"`
		LastUsed        time.Time `json:"last_used"`
		AvgKillDistance float64   `json:"avg_kill_distance"`
		Accuracy        float64   `json:"accuracy"`
		HeadshotRatio   float64   `json:"headshot_ratio"`
	}
	stats.Name = weapon

	if err := row.Scan(
		&stats.TotalKills,
		&stats.TotalHeadshots,
		&stats.ShotsFired,
		&stats.ShotsHit,
		&stats.UniqueUsers,
		&stats.LastUsed,
		&stats.AvgKillDistance,
	); err != nil {
		h.logger.Errorw("Failed to get weapon details", "error", err, "weapon", weapon)
	}

	if stats.ShotsFired > 0 {
		stats.Accuracy = float64(stats.ShotsHit) / float64(stats.ShotsFired) * 100
	}
	if stats.TotalKills > 0 {
		stats.HeadshotRatio = float64(stats.TotalHeadshots) / float64(stats.TotalKills) * 100
	}

	// Get top users for this weapon
	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			any(actor_name) as name,
			count() as kills,
			countIf(event_type = 'headshot') as headshots,
			if(count() > 0, toFloat64(countIf(event_type='headshot'))/count()*100, 0) as hs_ratio
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' AND actor_weapon = ? AND actor_id != ''
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT 10
	`, weapon)

	type TopUser struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Kills     uint64  `json:"kills"`
		Headshots uint64  `json:"headshots"`
		HSRatio   float64 `json:"hs_ratio"`
	}
	var topUsers []TopUser

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u TopUser
			if err := rows.Scan(&u.ID, &u.Name, &u.Kills, &u.Headshots, &u.HSRatio); err == nil {
				topUsers = append(topUsers, u)
			}
		}
	}

	response := map[string]interface{}{
		"stats":       stats,
		"top_players": topUsers,
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// GetPlayerStatsByName resolves a name to a GUID and returns its stats
func (h *Handler) GetPlayerStatsByName(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing player name")
		return
	}

	guid, err := h.playerStats.ResolvePlayerGUID(r.Context(), name)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Player not found: "+err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"guid": guid,
		"name": name,
	})
}

func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
