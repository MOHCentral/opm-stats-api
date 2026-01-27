package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GetPlayerStatsByGametype returns stats grouped by gametype (derived from map prefix)
func (h *Handler) GetPlayerStatsByGametype(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetPlayerStatsByGametype(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get gametype stats", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get gametype stats")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerStatsByMap returns detailed stats grouped by map
func (h *Handler) GetPlayerStatsByMap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	stats, err := h.playerStats.GetPlayerStatsByMap(ctx, guid)
	if err != nil {
		h.logger.Errorw("Failed to get map breakdown", "guid", guid, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get map breakdown")
		return
	}

	h.jsonResponse(w, http.StatusOK, stats)
}
