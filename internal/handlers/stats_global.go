package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
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
