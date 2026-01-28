package handlers

import (
	"net/http"
	"strconv"
)

// ============================================================================
// TEAM STATS ENDPOINTS
// ============================================================================

// GetFactionPerformance returns aggregated stats for Axis vs Allies
// @Summary Faction Performance Stats
// @Description Get consolidated stats for Axis vs Allies over a period
// @Tags Teams
// @Produce json
// @Param days query int false "Days to look back" default(30)
// @Success 200 {object} models.FactionStats
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/teams/performance [get]
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
