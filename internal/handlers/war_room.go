package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ============================================================================
// WAR ROOM ENHANCED ENDPOINTS
// ============================================================================

// GetPlayerPeakPerformance returns when/where a player performs best
// GET /api/v1/stats/player/{guid}/peak-performance
func (h *Handler) GetPlayerPeakPerformance(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing player GUID")
		return
	}

	pp, err := h.advancedStats.GetPeakPerformance(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get peak performance", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate peak performance")
		return
	}

	h.jsonResponse(w, http.StatusOK, pp)
}

// GetPlayerComboMetrics returns cross-event correlation metrics
// GET /api/v1/stats/player/{guid}/combos
func (h *Handler) GetPlayerComboMetrics(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing player GUID")
		return
	}

	cm, err := h.advancedStats.GetComboMetrics(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get combo metrics", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate combo metrics")
		return
	}

	h.jsonResponse(w, http.StatusOK, cm)
}

// GetPlayerDrillDown provides hierarchical stat exploration
// GET /api/v1/stats/player/{guid}/drilldown?stat=kd&dimensions[]=weapon&dimensions[]=map
func (h *Handler) GetPlayerDrillDown(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing player GUID")
		return
	}

	// Parse request parameters
	stat := r.URL.Query().Get("stat")
	if stat == "" {
		stat = "kd" // Default to K/D ratio
	}

	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "weapon"
	}
	
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	result, err := h.advancedStats.GetDrillDown(r.Context(), guid, stat, dimension, limit)
	if err != nil {
		h.logger.Errorw("Failed to get drilldown", "guid", guid, "stat", stat, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate drilldown")
		return
	}

	h.jsonResponse(w, http.StatusOK, result)
}

// GetPlayerDrillDownNested gets second-level breakdown within a dimension
// GET /api/v1/stats/player/{guid}/drilldown/{dimension}/{value}?child_dimension=map
func (h *Handler) GetPlayerDrillDownNested(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	parentDim := chi.URLParam(r, "dimension")
	parentValue := chi.URLParam(r, "value")

	if guid == "" || parentDim == "" || parentValue == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing required parameters")
		return
	}

	childDim := r.URL.Query().Get("child_dimension")
	if childDim == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing child_dimension parameter")
		return
	}

	stat := r.URL.Query().Get("stat")
	if stat == "" {
		stat = "kd"
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	items, err := h.advancedStats.GetDrillDownNested(r.Context(), guid, stat, parentDim, parentValue, childDim, limit)
	if err != nil {
		h.logger.Errorw("Failed to get nested drilldown", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate nested drilldown")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"parent_dimension": parentDim,
		"parent_value":     parentValue,
		"child_dimension":  childDim,
		"items":            items,
	})
}

// GetContextualLeaderboard returns top players for a specific context
// GET /api/v1/stats/leaderboard/contextual?stat=kd&dimension=map&value=mohdm6
func (h *Handler) GetContextualLeaderboard(w http.ResponseWriter, r *http.Request) {
	stat := r.URL.Query().Get("stat")
	if stat == "" {
		stat = "kd"
	}

	dimension := r.URL.Query().Get("dimension")
	value := r.URL.Query().Get("value")

	if dimension == "" || value == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing dimension or value parameter")
		return
	}

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	leaders, err := h.advancedStats.GetStatLeaders(r.Context(), stat, dimension, value, limit)
	if err != nil {
		h.logger.Errorw("Failed to get contextual leaderboard", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get leaderboard")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"stat":      stat,
		"dimension": dimension,
		"value":     value,
		"leaders":   leaders,
	})
}

// GetDrilldownOptions returns available dimensions for a stat
// GET /api/v1/stats/drilldown/options?stat=kd
func (h *Handler) GetDrilldownOptions(w http.ResponseWriter, r *http.Request) {
	stat := r.URL.Query().Get("stat")
	if stat == "" {
		stat = "kd"
	}

	options := h.advancedStats.GetAvailableDrilldowns(stat)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"stat":       stat,
		"dimensions": options,
	})
}

// GetPlayerWarRoomData returns all war room data in a single call for efficiency
// GET /api/v1/stats/player/{guid}/war-room
func (h *Handler) GetPlayerWarRoomData(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing player GUID")
		return
	}

	ctx := r.Context()
	response := make(map[string]interface{})

	// 1. Deep Stats (existing)
	deepStats, err := h.playerStats.GetDeepStats(ctx, guid)
	if err == nil {
		response["deep_stats"] = deepStats
	}

	// 2. Peak Performance
	peakPerf, err := h.advancedStats.GetPeakPerformance(ctx, guid)
	if err == nil {
		response["peak_performance"] = peakPerf
	}

	// 3. Combo Metrics
	combos, err := h.advancedStats.GetComboMetrics(ctx, guid)
	if err == nil {
		response["combo_metrics"] = combos
	}

	// 4. Default Drilldowns (K/D by weapon and map)
	// Simplified to separate calls or just first dimension
	kdDrill, err := h.advancedStats.GetDrillDown(ctx, guid, "kd", "weapon", 5)
	if err == nil {
		response["kd_drilldown"] = kdDrill
	}

	// 5. Playstyle badge
	badge, err := h.gamification.GetPlaystyle(ctx, guid)
	if err == nil {
		response["playstyle"] = badge
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// ============================================================================
// ENHANCED LEADERBOARD ENDPOINTS
// ============================================================================

// GetComboLeaderboard returns players ranked by combo metrics (RETAINED FROM ORIGINAL)
// GET /api/v1/stats/leaderboard/combos?metric=run_gun
func (h *Handler) GetComboLeaderboard(w http.ResponseWriter, r *http.Request) {
	// ... (Rest of function kept as manual SQL query in handler for specific combos)
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing metric parameter")
		return
	}

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx := r.Context()
	var query string
	// ... (Original switch case for run_gun, clutch, consistency)
	switch metric {
	case "run_gun":
		query = `
			WITH player_stats AS (
				SELECT 
					actor_id,
					any(actor_name) as name,
					countIf(event_type = 'player_kill') as kills,
					sumIf(toFloat64OrZero(extract(extra, 'velocity')), event_type = 'player_distance') as total_velocity
				FROM mohaa_stats.raw_events
				WHERE actor_id != ''
				GROUP BY actor_id
				HAVING kills >= 50
			)
			SELECT 
				actor_id,
				name,
				kills,
				total_velocity / kills as mobility_score
			FROM player_stats
			ORDER BY mobility_score DESC
			LIMIT ?
		`
	case "clutch":
		query = `
			SELECT 
				actor_id,
				any(actor_name) as name,
				countIf(event_type = 'team_win') as wins,
				uniq(match_id) as matches,
				wins / matches as clutch_rate
			FROM mohaa_stats.raw_events
			WHERE actor_id != ''
			GROUP BY actor_id
			HAVING matches >= 20
			ORDER BY clutch_rate DESC
			LIMIT ?
		`
	case "consistency":
		query = `
			WITH match_kd AS (
				SELECT 
					actor_id,
					match_id,
					countIf(event_type = 'player_kill') as kills,
					countIf(event_type = 'player_death') as deaths,
					if(deaths > 0, kills/deaths, kills) as kd
				FROM mohaa_stats.raw_events
				WHERE actor_id != ''
				GROUP BY actor_id, match_id
				HAVING kills + deaths >= 5
			)
			SELECT 
				actor_id,
				any(actor_name) as name,
				avg(kd) as avg_kd,
				stddevPop(kd) as kd_variance,
				count() as matches
			FROM match_kd
			GROUP BY actor_id
			HAVING matches >= 10
			ORDER BY kd_variance ASC
			LIMIT ?
		`
	default:
		h.errorResponse(w, http.StatusBadRequest, "Unknown metric: "+metric)
		return
	}

	rows, err := h.ch.Query(ctx, query, limit)
	if err != nil {
		h.logger.Errorw("Failed to query combo leaderboard", "metric", metric, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	// Simplified Entry struct locally
	type Entry struct {
		Rank       int     `json:"rank"`
		PlayerID   string  `json:"player_id"`
		PlayerName string  `json:"player_name"`
		Value      float64 `json:"value"`
		Secondary  float64 `json:"secondary,omitempty"`
	}

	var entries []Entry
	rank := 1
	for rows.Next() {
		var e Entry
		var secondary float64
		switch metric {
		case "run_gun":
			var kills int64
			if err := rows.Scan(&e.PlayerID, &e.PlayerName, &kills, &e.Value); err != nil { continue }
		case "clutch":
			var wins, matches int64
			if err := rows.Scan(&e.PlayerID, &e.PlayerName, &wins, &matches, &e.Value); err != nil { continue }
			secondary = float64(wins)
		case "consistency":
			var matches int64
			if err := rows.Scan(&e.PlayerID, &e.PlayerName, &secondary, &e.Value, &matches); err != nil { continue }
		}
		e.Rank = rank
		e.Secondary = secondary
		entries = append(entries, e)
		rank++
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"metric":  metric,
		"entries": entries,
	})
}

// GetPeakPerformanceLeaderboard returns players who perform best at certain times (RETAINED)
func (h *Handler) GetPeakPerformanceLeaderboard(w http.ResponseWriter, r *http.Request) {
	dimension := r.URL.Query().Get("dimension")
	if dimension == "" { dimension = "evening" }

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 { limit = parsed }
	}

	ctx := r.Context()
	var timeFilter string
	switch dimension {
	case "morning": timeFilter = "toHour(timestamp) BETWEEN 6 AND 11"
	case "afternoon": timeFilter = "toHour(timestamp) BETWEEN 12 AND 17"
	case "evening": timeFilter = "toHour(timestamp) BETWEEN 18 AND 23"
	case "night": timeFilter = "toHour(timestamp) BETWEEN 0 AND 5"
	case "weekend": timeFilter = "toDayOfWeek(timestamp) IN (6, 7)"
	default: 
		h.errorResponse(w, http.StatusBadRequest, "Unknown dimension")
		return
	}

	query := `
		SELECT 
			actor_id,
			any(actor_name) as name,
			countIf(event_type = 'player_kill') as kills,
			countIf(event_type = 'player_death') as deaths,
			if(deaths > 0, kills/deaths, kills) as kd
		FROM mohaa_stats.raw_events
		WHERE actor_id != '' AND ` + timeFilter + `
		GROUP BY actor_id
		HAVING kills >= 20
		ORDER BY kd DESC
		LIMIT ?
	`

	rows, err := h.ch.Query(ctx, query, limit)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type Entry struct {
		Rank       int     `json:"rank"`
		PlayerID   string  `json:"player_id"`
		PlayerName string  `json:"player_name"`
		Kills      int64   `json:"kills"`
		Deaths     int64   `json:"deaths"`
		KD         float64 `json:"kd"`
	}

	var entries []Entry
	rank := 1
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.PlayerID, &e.PlayerName, &e.Kills, &e.Deaths, &e.KD); err != nil { continue }
		e.Rank = rank
		entries = append(entries, e)
		rank++
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"dimension": dimension,
		"entries":   entries,
	})
}
