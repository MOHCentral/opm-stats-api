package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ============================================================================
// TOURNAMENT ENDPOINTS
// ============================================================================

// GetTournaments returns list of tournaments
// @Summary List Tournaments
// @Tags Tournaments
// @Produce json
// @Success 200 {array} models.Tournament
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /tournaments [get]
func (h *Handler) GetTournaments(w http.ResponseWriter, r *http.Request) {
	list, err := h.tournament.GetTournaments(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get tournaments", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get tournaments")
		return
	}
	h.jsonResponse(w, http.StatusOK, list)
}

// GetTournament returns details
// @Summary Get Tournament Details
// @Tags Tournaments
// @Produce json
// @Param id path string true "Tournament ID"
// @Success 200 {object} models.Tournament
// @Failure 404 {object} map[string]string "Not Found"
// @Router /tournaments/{id} [get]
func (h *Handler) GetTournament(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing tournament ID")
		return
	}

	t, err := h.tournament.GetTournament(r.Context(), id)
	if err != nil {
		h.logger.Errorw("Failed to get tournament", "id", id, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get tournament")
		return
	}
	h.jsonResponse(w, http.StatusOK, t)
}

// GetTournamentStats returns aggregated stats
// @Summary Get Tournament Stats
// @Tags Tournaments
// @Produce json
// @Param id path string true "Tournament ID"
// @Success 200 {object} map[string]interface{} "Stats"
// @Failure 404 {object} map[string]string "Not Found"
// @Router /tournaments/{id}/stats [get]
func (h *Handler) GetTournamentStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing tournament ID")
		return
	}

	stats, err := h.tournament.GetTournamentStats(r.Context(), id)
	if err != nil {
		h.logger.Errorw("Failed to get tournament stats", "id", id, "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}
	h.jsonResponse(w, http.StatusOK, stats)
}
