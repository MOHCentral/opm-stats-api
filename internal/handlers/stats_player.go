package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

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
			match_id,
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
			match_id,
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

// GetPlayerMatches returns recent matches for a player
func (h *Handler) GetPlayerMatches(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT
			match_id,
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
			match_id,
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
