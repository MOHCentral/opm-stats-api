package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ============================================================================
// TOURNAMENT ENDPOINTS
// ============================================================================

// GetTournaments returns list of tournaments
// GET /api/v1/tournaments
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
// GET /api/v1/tournaments/{id}
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
// GET /api/v1/tournaments/{id}/stats
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
