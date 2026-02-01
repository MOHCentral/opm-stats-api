package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
	"github.com/openmohaa/stats-api/internal/models"
)

// ============================================================================
// SERVER STATS ENDPOINTS
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

// GetServerPulse returns high-level "vital signs" of the server
// @Summary Server Pulse (Main)
// @Description Real-time heartbeat of server activity and chaos
// @Tags Server
// @Produce json
// @Success 200 {object} models.ServerPulse "Pulse Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/server/pulse [get]
func (h *Handler) GetServerPulse(w http.ResponseWriter, r *http.Request) {
	pulse, err := h.serverStats.GetServerPulse(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get server pulse", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get server pulse")
		return
	}
	h.jsonResponse(w, http.StatusOK, pulse)
}

// GetServerActivity returns a heatmap of activity
// @Summary Global Server Activity
// @Tags Server
// @Produce json
// @Success 200 {object} map[string]interface{} "Activity Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/server/activity [get]
func (h *Handler) GetServerActivity(w http.ResponseWriter, r *http.Request) {
	activity, err := h.serverStats.GetGlobalActivity(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get server activity", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get server activity")
		return
	}
	h.jsonResponse(w, http.StatusOK, activity)
}

// GetServerMaps returns map popularity stats
// @Summary Global Map Stats
// @Tags Server
// @Produce json
// @Success 200 {object} []models.MapStats "Map Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/server/maps [get]
func (h *Handler) GetServerMaps(w http.ResponseWriter, r *http.Request) {
	maps, err := h.serverStats.GetMapPopularity(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get map popularity", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get map stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, maps)
}

// ============================================================================
// SERVER TRACKING ENDPOINTS (New Dashboard System)
// ============================================================================

// getServerTracking returns the server tracking service
func (h *Handler) getServerTracking() *logic.ServerTrackingService {
	return logic.NewServerTrackingService(h.ch, h.pg, h.redis)
}

// GetAllServers returns list of all registered servers with live status
// @Summary List All Servers
// @Description List active servers with status
// @Tags Server
// @Produce json
// @Success 200 {array} models.ServerOverview "Server List"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers [get]
func (h *Handler) GetAllServers(w http.ResponseWriter, r *http.Request) {
	svc := h.getServerTracking()
	servers, err := svc.GetServerList(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get server list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get servers")
		return
	}
	h.jsonResponse(w, http.StatusOK, servers)
}

// GetServersGlobalStats returns aggregate stats across all servers
// @Summary Global Network Stats
// @Description Aggregate stats across all servers
// @Tags Server
// @Produce json
// @Success 200 {object} map[string]interface{} "Network Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/stats [get]
func (h *Handler) GetServersGlobalStats(w http.ResponseWriter, r *http.Request) {
	svc := h.getServerTracking()
	stats, err := svc.GetServerGlobalStats(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get global server stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, stats)
}

// GetServerRankings returns ranked list of servers
// @Summary Get Server Rankings
// @Tags Server
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Success 200 {array} models.ServerOverview "Rankings"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/rankings [get]
func (h *Handler) GetServerRankings(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, _ := strconv.Atoi(l); parsed > 0 {
			limit = parsed
		}
	}

	svc := h.getServerTracking()
	rankings, err := svc.GetServerRankings(r.Context(), limit)
	if err != nil {
		h.logger.Errorw("Failed to get server rankings", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get rankings")
		return
	}
	h.jsonResponse(w, http.StatusOK, rankings)
}

// GetServerDetail returns comprehensive details for a specific server
// @Summary Server Details
// @Description Detailed server info including lifetime stats
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {object} models.ServerOverview "Server Detail"
// @Failure 400 {object} map[string]string "Missing ID"
// @Failure 404 {object} map[string]string "Not Found"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id} [get]
func (h *Handler) GetServerDetail(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	if serverID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing server ID")
		return
	}

	svc := h.getServerTracking()
	detail, err := svc.GetServerDetail(r.Context(), serverID)
	if err != nil {
		h.logger.Errorw("Failed to get server detail", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusNotFound, "Server not found")
		return
	}
	h.jsonResponse(w, http.StatusOK, detail)
}

// GetServerLiveStatus returns real-time status for a server
// @Summary Get Server Live Status
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {object} models.ServerLiveStatusResponse "Live Status"
// @Failure 400 {object} map[string]string "Missing ID"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/live [get]
func (h *Handler) GetServerLiveStatus(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	if serverID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing server ID")
		return
	}

	svc := h.getServerTracking()
	status, err := svc.GetLiveServerStatus(r.Context(), serverID)
	if err != nil {
		h.logger.Errorw("Failed to get live status", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get live status")
		return
	}
	h.jsonResponse(w, http.StatusOK, status)
}

// GetServerPlayerHistory returns player count history for charts
// @Summary Server Player History
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param hours query int false "Hours" default(24)
// @Success 200 {array} models.PlayerHistoryPoint "History Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/player-history [get]
func (h *Handler) GetServerPlayerHistory(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if parsed, _ := strconv.Atoi(h); parsed > 0 {
			hours = parsed
		}
	}

	svc := h.getServerTracking()
	history, err := svc.GetServerPlayerHistory(r.Context(), serverID, hours)
	if err != nil {
		h.logger.Errorw("Failed to get player history", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get history")
		return
	}
	h.jsonResponse(w, http.StatusOK, history)
}

// GetServerPeakHours returns activity heatmap by day/hour
// @Summary Server Peak Hours Heatmap
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param days query int false "Days" default(30)
// @Success 200 {object} models.PeakHoursHeatmap "Heatmap Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/peak-hours [get]
func (h *Handler) GetServerPeakHours(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, _ := strconv.Atoi(d); parsed > 0 {
			days = parsed
		}
	}

	svc := h.getServerTracking()
	heatmap, err := svc.GetServerPeakHours(r.Context(), serverID, days)
	if err != nil {
		h.logger.Errorw("Failed to get peak hours", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get peak hours")
		return
	}
	h.jsonResponse(w, http.StatusOK, heatmap)
}

// GetServerTopPlayers returns top players for a specific server
// @Summary Server Top Players
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param limit query int false "Limit" default(25)
// @Success 200 {array} models.PlayerStats "Top Players"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/top-players [get]
func (h *Handler) GetServerTopPlayers(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, _ := strconv.Atoi(l); parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	svc := h.getServerTracking()
	players, err := svc.GetServerTopPlayers(r.Context(), serverID, limit)
	if err != nil {
		h.logger.Errorw("Failed to get top players", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get top players")
		return
	}
	h.jsonResponse(w, http.StatusOK, players)
}

// GetServerMapStats returns map statistics for a server
// @Summary Server Map Stats
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {array} models.MapStats "Map Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/maps [get]
func (h *Handler) GetServerMapStats(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	svc := h.getServerTracking()
	maps, err := svc.GetServerMapStats(r.Context(), serverID)
	if err != nil {
		h.logger.Errorw("Failed to get server map stats", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get map stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, maps)
}

// GetServerWeaponStats returns weapon statistics for a server
// @Summary Server Weapon Stats
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {array} models.WeaponStats "Weapon Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/weapons [get]
func (h *Handler) GetServerWeaponStats(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	svc := h.getServerTracking()
	weapons, err := svc.GetServerWeaponStats(r.Context(), serverID)
	if err != nil {
		h.logger.Errorw("Failed to get server weapon stats", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get weapon stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, weapons)
}

// GetServerRecentMatches returns recent matches for a server
// @Summary Server Recent Matches
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param limit query int false "Limit" default(20)
// @Success 200 {array} models.MatchResult "Matches"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/matches [get]
func (h *Handler) GetServerRecentMatches(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, _ := strconv.Atoi(l); parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	svc := h.getServerTracking()
	matches, err := svc.GetServerRecentMatches(r.Context(), serverID, limit)
	if err != nil {
		h.logger.Errorw("Failed to get server matches", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get matches")
		return
	}
	h.jsonResponse(w, http.StatusOK, matches)
}

// GetServerActivityTimeline returns hourly activity timeline
// @Summary Server Activity Timeline
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param days query int false "Days" default(7)
// @Success 200 {array} models.ActivityTimelinePoint "Timeline"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/activity-timeline [get]
func (h *Handler) GetServerActivityTimeline(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, _ := strconv.Atoi(d); parsed > 0 {
			days = parsed
		}
	}

	svc := h.getServerTracking()
	timeline, err := svc.GetServerActivityTimeline(r.Context(), serverID, days)
	if err != nil {
		h.logger.Errorw("Failed to get activity timeline", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get timeline")
		return
	}
	h.jsonResponse(w, http.StatusOK, timeline)
}

// ============================================================================
// SERVER FAVORITES
// ============================================================================

// AddServerFavorite adds a server to user's favorites
// @Summary Add Favorite Server
// @Tags Server
// @Security BearerAuth
// @Produce json
// @Param id path string true "Server ID"
// @Param nickname query string false "Nickname"
// @Success 200 {object} map[string]bool "Success"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/favorite [post]
func (h *Handler) AddServerFavorite(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	userID := h.getUserIDFromContext(r.Context())
	if userID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	nickname := r.URL.Query().Get("nickname")

	svc := h.getServerTracking()
	err := svc.AddServerFavorite(r.Context(), userID, serverID, nickname)
	if err != nil {
		h.logger.Errorw("Failed to add favorite", "server_id", serverID, "user_id", userID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to add favorite")
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]bool{"success": true})
}

// RemoveServerFavorite removes a server from user's favorites
// @Summary Remove Favorite Server
// @Tags Server
// @Security BearerAuth
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {object} map[string]bool "Success"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/favorite [delete]
func (h *Handler) RemoveServerFavorite(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	userID := h.getUserIDFromContext(r.Context())
	if userID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	svc := h.getServerTracking()
	err := svc.RemoveServerFavorite(r.Context(), userID, serverID)
	if err != nil {
		h.logger.Errorw("Failed to remove favorite", "server_id", serverID, "user_id", userID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to remove favorite")
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]bool{"success": true})
}

// GetUserFavoriteServers returns user's favorite servers
// @Summary Get User Favorites
// @Tags Server
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ServerOverview "Favorites"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/favorites [get]
func (h *Handler) GetUserFavoriteServers(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserIDFromContext(r.Context())
	if userID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	svc := h.getServerTracking()
	servers, err := svc.GetUserFavoriteServers(r.Context(), userID)
	if err != nil {
		h.logger.Errorw("Failed to get favorites", "user_id", userID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get favorites")
		return
	}
	h.jsonResponse(w, http.StatusOK, servers)
}

// CheckServerFavorite checks if server is in user's favorites
// @Summary Check Favorite Status
// @Tags Server
// @Security BearerAuth
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {object} map[string]bool "Status"
// @Router /servers/{id}/favorite [get]
func (h *Handler) CheckServerFavorite(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	userID := h.getUserIDFromContext(r.Context())
	if userID == 0 {
		h.jsonResponse(w, http.StatusOK, map[string]bool{"is_favorite": false})
		return
	}

	svc := h.getServerTracking()
	isFavorite, _ := svc.IsServerFavorite(r.Context(), userID, serverID)
	h.jsonResponse(w, http.StatusOK, map[string]bool{"is_favorite": isFavorite})
}

// ============================================================================
// HISTORICAL PLAYER DATA
// ============================================================================

// GetServerHistoricalPlayers returns all players with historical data for a server
// @Summary Server Historical Players
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset"
// @Success 200 {object} map[string]interface{} "Players List"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/players [get]
func (h *Handler) GetServerHistoricalPlayers(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, _ := strconv.Atoi(l); parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, _ := strconv.Atoi(o); parsed >= 0 {
			offset = parsed
		}
	}

	svc := h.getServerTracking()
	players, total, err := svc.GetServerHistoricalPlayers(r.Context(), serverID, limit, offset)
	if err != nil {
		h.logger.Errorw("Failed to get historical players", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get players")
		return
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"players": players,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// ============================================================================
// MAP ROTATION ANALYSIS
// ============================================================================

// GetServerMapRotation returns detailed map rotation analysis
// @Summary Server Map Rotation
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Param days query int false "Days" default(30)
// @Success 200 {array} models.ServerMapRotationResponse "Rotation Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/map-rotation [get]
func (h *Handler) GetServerMapRotation(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, _ := strconv.Atoi(d); parsed > 0 {
			days = parsed
		}
	}

	svc := h.getServerTracking()
	rotation, err := svc.GetServerMapRotation(r.Context(), serverID, days)
	if err != nil {
		h.logger.Errorw("Failed to get map rotation", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get map rotation")
		return
	}
	h.jsonResponse(w, http.StatusOK, rotation)
}

// ============================================================================
// COUNTRY STATS
// ============================================================================

// GetServerCountryStats returns player distribution by country
// @Summary Server Country Stats
// @Tags Server
// @Produce json
// @Param id path string true "Server ID"
// @Success 200 {array} models.ServerCountryStatsResponse "Country Data"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/{id}/countries [get]
func (h *Handler) GetServerCountryStats(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")

	svc := h.getServerTracking()
	countries, err := svc.GetServerCountryStats(r.Context(), serverID)
	if err != nil {
		h.logger.Errorw("Failed to get country stats", "server_id", serverID, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get country stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, countries)
}
