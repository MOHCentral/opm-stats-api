package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// ============================================================================
// WAR ROOM ENHANCED ENDPOINTS
// ============================================================================

// GetPlayerPeakPerformance returns when/where a player performs best
// @Summary Get player peak performance
// @Description Returns stats showing when and where a player performs best (best hour, day, map, weapon)
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {object} models.PeakPerformance
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/player/{guid}/peak-performance [get]
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
// @Summary Get player combo metrics
// @Description Returns complex cross-referenced stats like best weapon per map, victim/killer patterns, etc.
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {object} models.ComboMetrics
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/player/{guid}/combos [get]
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
// @Summary Drill down into player stats
// @Description Allows drilling down into a specific stat by a dimension (e.g. K/D by Weapon)
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Param stat query string false "Stat to analyze" default(kd)
// @Param dimension query string false "Dimension to group by" default(weapon)
// @Param limit query int false "Max items to return" default(10)
// @Success 200 {object} models.DrillDownResult
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/player/{guid}/drilldown [get]
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
// @Summary Nested drill down
// @Description Provides a second level of breakdown (e.g. K/D with Thompson on Mohdm6)
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Param dimension path string true "Parent Dimension (e.g. weapon)"
// @Param value path string true "Parent Value (e.g. Thompson)"
// @Param child_dimension query string true "Child Dimension (e.g. map)"
// @Param stat query string false "Stat to analyze" default(kd)
// @Param limit query int false "Max items to return" default(10)
// @Success 200 {object} models.DrillDownNestedResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/player/{guid}/drilldown/{dimension}/{value} [get]
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

	// ...
	h.jsonResponse(w, http.StatusOK, models.DrillDownNestedResponse{
		ParentDimension: parentDim,
		ParentValue:     parentValue,
		ChildDimension:  childDim,
		Items:           items,
	})
}

// GetContextualLeaderboard returns top players for a specific context
// @Summary Contextual leaderboard
// @Description Get best players for a specific context (e.g. Best Snipers on Mohdm6)
// @Tags Leaderboards
// @Accept json
// @Produce json
// @Param stat query string false "Stat to rank by" default(kd)
// @Param dimension query string true "Context dimension (e.g. map, weapon)"
// @Param value query string true "Context value (e.g. mohdm6, Thompson)"
// @Param limit query int false "Max items to return" default(25)
// @Success 200 {object} models.ContextualLeaderboardResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/leaderboard/contextual [get]
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

	// ...
	h.jsonResponse(w, http.StatusOK, models.ContextualLeaderboardResponse{
		Stat:      stat,
		Dimension: dimension,
		Value:     value,
		Leaders:   leaders,
	})
}

// GetDrilldownOptions returns available dimensions for a stat
// @Summary Get drilldown options
// @Description Returns available dimensions for drilling down into a stat
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param stat query string false "Stat" default(kd)
// @Success 200 {object} models.DrilldownOptionsResponse
// @Failure 500 {object} map[string]string
// @Router /stats/drilldown/options [get]
func (h *Handler) GetDrilldownOptions(w http.ResponseWriter, r *http.Request) {
	stat := r.URL.Query().Get("stat")
	if stat == "" {
		stat = "kd"
	}

	options := h.advancedStats.GetAvailableDrilldowns(stat)

	// ...
	h.jsonResponse(w, http.StatusOK, models.DrilldownOptionsResponse{
		Stat:       stat,
		Dimensions: options,
	})
}

// GetPlayerWarRoomData returns all war room data in a single call for efficiency
// @Summary Get war room data
// @Description Returns comprehensive stats for the war room dashboard
// @Tags Advanced Stats
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {object} models.WarRoomDataResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/player/{guid}/war-room [get]
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

	// ...
	h.jsonResponse(w, http.StatusOK, models.WarRoomDataResponse{
		DeepStats:       deepStats,
		PeakPerformance: peakPerf,
		ComboMetrics:    combos,
		KDDrilldown:     kdDrill,
		Playstyle:       badge,
	})
}

// ============================================================================
// ENHANCED LEADERBOARD ENDPOINTS
// ============================================================================

// GetComboLeaderboard returns players ranked by combo metrics (RETAINED FROM ORIGINAL)
// @Summary Combo leaderboard
// @Description Get leaderboard for derived combo metrics (run_gun, clutch, consistency)
// @Tags Leaderboards
// @Accept json
// @Produce json
// @Param metric query string true "Metric (run_gun, clutch, consistency)"
// @Param limit query int false "Max items to return" default(25)
// @Success 200 {object} models.ComboLeaderboardResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/leaderboard/combos [get]
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
					countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
					sumIf(toFloat64OrZero(extract(extra, 'velocity')), event_type = 'distance') as total_velocity
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
					countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
					countIf(event_type = 'death') as deaths,
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

	// ...


	// ...
	var entries []models.StatLeaderboardEntry
	rank := 1
	for rows.Next() {
		var e models.StatLeaderboardEntry
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


	h.jsonResponse(w, http.StatusOK, models.ComboLeaderboardResponse{
		Metric:  metric,
		Entries: entries,
	})
}

// GetPeakPerformanceLeaderboard returns players who perform best at certain times (RETAINED)
// @Summary Peak performance leaderboard
// @Description Get players who excel in specific time windows (morning, night, weekend)
// @Tags Leaderboards
// @Accept json
// @Produce json
// @Param dimension query string false "Time Dimension (morning, afternoon, evening, night, weekend)" default(evening)
// @Param limit query int false "Max items to return" default(25)
// @Success 200 {object} models.PeakLeaderboardResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /stats/leaderboard/peak [get]
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
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			countIf(event_type = 'death') as deaths,
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

	// ...


	var entries []models.PeakLeaderboardEntry
	rank := 1
	for rows.Next() {
		var e models.PeakLeaderboardEntry
		if err := rows.Scan(&e.PlayerID, &e.PlayerName, &e.Kills, &e.Deaths, &e.KD); err != nil { continue }
		e.Rank = rank
		entries = append(entries, e)
		rank++
	}

	// ...
	h.jsonResponse(w, http.StatusOK, models.PeakLeaderboardResponse{
		Dimension: dimension,
		Entries:   entries,
	})
}
