package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// GetPlayerStatsByGametype returns stats grouped by gametype (derived from map prefix)
// @Summary Get Player Stats by Gametype
// @Description Returns player statistics grouped by gametype
// @Tags Player
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {array} models.GametypeStats "Gametype Stats"
// @Failure 500 {object} map[string]string "Server Error"
// @Router /stats/player/{guid}/gametype [get]
func (h *Handler) GetPlayerStatsByGametype(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	var stats []models.GametypeStats
	var err error
	stats, err = h.playerStats.GetPlayerStatsByGametype(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get gametype stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get gametype stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerStatsByMap returns detailed stats grouped by map
// @Summary Get Player Stats by Map
// @Description Returns player statistics grouped by map
// @Tags Player
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {array} models.PlayerMapStats "Map Stats"
// @Failure 500 {object} map[string]string "Server Error"
// @Router /stats/player/{guid}/maps [get]
func (h *Handler) GetPlayerStatsByMap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	var stats []models.PlayerMapStats
	var err error
	stats, err = h.playerStats.GetPlayerStatsByMap(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get map breakdown", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get map breakdown")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}
