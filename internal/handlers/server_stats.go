package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
)

// ============================================================================
// SERVER STATS ENDPOINTS
// ============================================================================

// GetServerPulse returns high-level "vital signs" of the server
// GET /api/v1/stats/server/pulse
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
// GET /api/v1/stats/server/activity
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
// GET /api/v1/stats/server/maps
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
// GET /api/v1/servers
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
// GET /api/v1/servers/stats
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
// GET /api/v1/servers/rankings
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
// GET /api/v1/servers/{id}
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
// GET /api/v1/servers/{id}/live
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
// GET /api/v1/servers/{id}/player-history
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
// GET /api/v1/servers/{id}/peak-hours
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
// GET /api/v1/servers/{id}/top-players
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
// GET /api/v1/servers/{id}/maps
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
// GET /api/v1/servers/{id}/weapons
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
// GET /api/v1/servers/{id}/matches
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
// GET /api/v1/servers/{id}/activity-timeline
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
// POST /api/v1/servers/{id}/favorite
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
// DELETE /api/v1/servers/{id}/favorite
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
// GET /api/v1/servers/favorites
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
// GET /api/v1/servers/{id}/favorite
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
// GET /api/v1/servers/{id}/players
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
// GET /api/v1/servers/{id}/map-rotation
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
// GET /api/v1/servers/{id}/countries
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
