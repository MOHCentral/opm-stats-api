package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
	"github.com/openmohaa/stats-api/internal/models"
)

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

	// Map stat name to ClickHouse column/expression - Strictly whitelisted
	var orderExpr, havingExpr string
	havingExpr = "kills > 0" // Default having

	switch stat {
	case "kills":
		orderExpr = "kills"
	case "deaths":
		orderExpr = "deaths"
		havingExpr = "deaths > 0"
	case "kd_ratio", "kd":
		orderExpr = "kills / nullIf(deaths, 0)"
	case "headshots":
		orderExpr = "headshots"
	case "accuracy":
		orderExpr = "shots_hit / nullIf(shots_fired, 0)"
	case "shots_fired":
		orderExpr = "shots_fired"
	case "damage":
		orderExpr = "total_damage"
	case "bash_kills":
		orderExpr = "bash_kills"
	case "grenade_kills":
		orderExpr = "grenade_kills"
	case "roadkills":
		orderExpr = "roadkills"
	case "telefrags":
		orderExpr = "telefrags"
	case "crushed":
		orderExpr = "crushed"
	case "teamkills":
		orderExpr = "teamkills"
	case "suicides":
		orderExpr = "suicides"
	case "reloads":
		orderExpr = "reloads"
	case "weapon_swaps":
		orderExpr = "weapon_swaps"
	case "no_ammo":
		orderExpr = "no_ammo"
	case "looter":
		orderExpr = "items_picked"
	case "distance":
		orderExpr = "distance_units"
	case "sprinted":
		orderExpr = "sprinted"
	case "swam":
		orderExpr = "swam"
	case "driven":
		orderExpr = "driven"
	case "jumps":
		orderExpr = "jumps"
	case "crouch_time":
		orderExpr = "crouch_events"
	case "prone_time":
		orderExpr = "prone_events"
	case "ladders":
		orderExpr = "ladders"
	case "health_picked":
		orderExpr = "health_picked"
	case "ammo_picked":
		orderExpr = "ammo_picked"
	case "armor_picked":
		orderExpr = "armor_picked"
	case "items_picked":
		orderExpr = "items_picked"
	case "wins":
		orderExpr = "matches_won"
	case "team_wins":
		orderExpr = "matches_won"
	case "ffa_wins":
		orderExpr = "matches_won"
	case "losses":
		orderExpr = "matches_played - matches_won"
	case "objectives":
		orderExpr = "objectives"
	case "rounds":
		orderExpr = "matches_played"
	case "playtime":
		orderExpr = "playtime_seconds"
	case "games":
		orderExpr = "games_finished"
	default:
		orderExpr = "kills"
	}

	whereExpr := "player_id != ''"
	switch period {
	case "week":
		whereExpr += " AND day >= now() - INTERVAL 7 DAY"
	case "month":
		whereExpr += " AND day >= now() - INTERVAL 30 DAY"
	case "year":
		whereExpr += " AND day >= now() - INTERVAL 365 DAY"
	}

	// Query the unified Aggregation Table
	// Safe usage of fmt.Sprintf because inputs are strictly whitelisted string constants
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
			max(last_active) AS max_last_active
		FROM mohaa_stats.player_stats_daily
		WHERE %s
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
		case "kills":
			entry.Value = entry.Kills
		case "deaths":
			entry.Value = entry.Deaths
		case "headshots":
			entry.Value = entry.Headshots
		case "accuracy":
			entry.Value = fmt.Sprintf("%.1f%%", entry.Accuracy)
		case "damage", "total_damage":
			entry.Value = entry.Damage
		case "wins":
			entry.Value = entry.Wins
		case "rounds":
			entry.Value = entry.Rounds
		case "looter":
			entry.Value = entry.ItemsPicked
		case "distance", "distance_km":
			entry.Value = fmt.Sprintf("%.2fkm", entry.Distance/1000.0) // Convert units to km if distance is in units
		default:
			entry.Value = entry.Kills
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

// formatGameTypeName converts prefix to display name
func formatGameTypeName(prefix string) string {
	if info, ok := gameTypeInfo[prefix]; ok {
		return info.Name
	}
	return strings.ToUpper(prefix)
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
