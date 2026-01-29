package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

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

	query := fmt.Sprintf(`
		SELECT
			actor_id,
			actor_name,
			sum(kills) as kills,
			sum(deaths) as deaths,
			sum(headshots) as headshots,
			sum(shots_fired) as shots_fired,
			sum(shots_hit) as shots_hit,
			sum(total_damage) as total_damage,
			sum(bash_kills) as bash_kills,
			sum(grenade_kills) as grenade_kills,
			sum(roadkills) as roadkills,
			sum(telefrags) as telefrags,
			sum(crushed) as crushed,
			sum(teamkills) as teamkills,
			sum(suicides) as suicides,
			sum(reloads) as reloads,
			sum(weapon_swaps) as weapon_swaps,
			sum(no_ammo) as no_ammo,
			sum(distance_units) as distance,
			sum(sprinted) as sprinted,
			sum(swam) as swam,
			sum(driven) as driven,
			sum(jumps) as jumps,
			sum(crouch_events) as crouches,
			sum(prone_events) as prone,
			sum(ladders) as ladders,
			sum(health_picked) as health_picked,
			sum(ammo_picked) as ammo_picked,
			sum(armor_picked) as armor_picked,
			sum(items_picked) as items_picked,
			sum(matches_won) as wins,
			sum(matches_played) as rounds,
			sum(games_finished) as games,
			sum(playtime_seconds) as playtime,
			max(last_active) as last_active
		FROM mohaa_stats.player_stats_daily_mv
		WHERE %s
		GROUP BY actor_id
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
		entry.Rank = rank
		entries = append(entries, entry)
		rank++
	}

	var total int64
	h.ch.QueryRow(ctx, "SELECT uniq(actor_id) FROM mohaa_stats.player_stats_daily_mv").Scan(&total)

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
			actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill'
		  AND actor_id != 'world'
		  AND timestamp >= now() - INTERVAL 7 DAY
		GROUP BY actor_id, actor_name
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
			actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill'
		  AND actor_weapon = ?
		  AND actor_id != 'world'
		GROUP BY actor_id, actor_name
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
			actor_name,
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill'
		  AND map_name = ?
		  AND actor_id != 'world'
		GROUP BY actor_id, actor_name
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
