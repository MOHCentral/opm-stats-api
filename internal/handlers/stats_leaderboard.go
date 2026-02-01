package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// Allowed order expressions for leaderboard
var allowedOrderExprs = map[string]string{
	"kills":          "kills",
	"deaths":         "deaths",
	"kd_ratio":       "kills / nullIf(deaths, 0)",
	"kd":             "kills / nullIf(deaths, 0)",
	"headshots":      "headshots",
	"accuracy":       "shots_hit / nullIf(shots_fired, 0)",
	"shots_fired":    "shots_fired",
	"damage":         "total_damage",
	"bash_kills":     "bash_kills",
	"grenade_kills":  "grenade_kills",
	"roadkills":      "roadkills",
	"telefrags":      "telefrags",
	"crushed":        "crushed",
	"teamkills":      "teamkills",
	"suicides":       "suicides",
	"reloads":        "reloads",
	"weapon_swaps":   "weapon_swaps",
	"no_ammo":        "no_ammo",
	"looter":         "items_picked",
	"distance":       "distance_units",
	"sprinted":       "sprinted",
	"swam":           "swam",
	"driven":         "driven",
	"jumps":          "jumps",
	"crouch_time":    "crouch_events",
	"prone_time":     "prone_events",
	"ladders":        "ladders",
	"health_picked":  "health_picked",
	"ammo_picked":    "ammo_picked",
	"armor_picked":   "armor_picked",
	"items_picked":   "items_picked",
	"wins":           "matches_won",
	"team_wins":      "matches_won",
	"ffa_wins":       "matches_won",
	"losses":         "matches_played - matches_won",
	"objectives":     "objectives",
	"rounds":         "matches_played",
	"playtime":       "playtime_seconds",
	"games":          "games_finished",
}

// Allowed having clauses
var allowedHavingClauses = map[string]string{
	"deaths": "deaths > 0",
}

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

	// Validate stat against whitelist
	orderExpr, ok := allowedOrderExprs[stat]
	if !ok {
		// Fallback to default if invalid stat
		stat = "kills"
		orderExpr = allowedOrderExprs["kills"]
	}

	havingExpr := ""
	if hExpr, ok := allowedHavingClauses[stat]; ok {
		havingExpr = hExpr
	}

	// Validate period
	periodDays := 0
	switch period {
	case "week":
		periodDays = 7
	case "month":
		periodDays = 30
	case "year":
		periodDays = 365
	}

	// Query the unified Aggregation Table
	// Construct the query carefully using parameterized period if possible, but ClickHouse interval syntax
	// typically needs literals or we pass the date threshold as parameter.
	// Using date parameter is safer.

	// Base WHERE clause
	whereClause := "player_id != ''"
	var queryArgs []interface{}

	if periodDays > 0 {
		whereClause += " AND day >= now() - INTERVAL ? DAY"
		queryArgs = append(queryArgs, periodDays)
	}

	// Construct full query
	// Note: orderExpr is trusted from whitelist map
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
			uniqExactMerge(matches_played) AS rounds,
			sum(games_finished) AS games,
			toUInt64(0) AS playtime,
			max(last_active) AS max_last_active
		FROM mohaa_stats.player_stats_daily
		WHERE %s
		GROUP BY player_id
	`, whereClause)

	if havingExpr != "" {
		query += fmt.Sprintf(" HAVING %s", havingExpr)
	}

	query += fmt.Sprintf(" ORDER BY %s DESC LIMIT ? OFFSET ?", orderExpr)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := h.ch.Query(ctx, query, queryArgs...)
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
