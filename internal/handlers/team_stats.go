package handlers

import (
	"net/http"
	"strconv"
)

// ============================================================================
// TEAM STATS ENDPOINTS
// ============================================================================

// GetFactionPerformance returns aggregated stats for Axis vs Allies
// GET /api/v1/stats/teams/performance?days=30
func (h *Handler) GetFactionPerformance(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days := 30
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
		days = d
	}

	stats, err := h.teamStats.GetFactionComparison(r.Context(), days)
	if err != nil {
		h.logger.Errorw("Failed to get faction comparison", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to calculate faction stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}
