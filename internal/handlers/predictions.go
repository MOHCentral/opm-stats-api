package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GetPlayerPredictions returns AI-driven performance forecasts for a player
// @Summary Get Player Predictions
// @Tags AI
// @Accept json
// @Produce json
// @Param guid path string true "Player GUID"
// @Success 200 {object} models.PlayerPredictions
// @Failure 404 {object} map[string]string "Not Found"
// @Router /stats/player/{guid}/predictions [get]
func (h *Handler) GetPlayerPredictions(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		h.errorResponse(w, http.StatusBadRequest, "GUID is required")
		return
	}

	pred, err := h.prediction.GetPlayerPredictions(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get player predictions", "error", err, "guid", guid)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	h.jsonResponse(w, http.StatusOK, pred)
}

// GetMatchPredictions returns AI forecasts for a specific match
// @Summary Get Match Predictions
// @Tags AI
// @Accept json
// @Produce json
// @Param matchId path string true "Match ID"
// @Success 200 {object} models.MatchPredictions
// @Router /stats/match/{matchId}/predictions [get]
func (h *Handler) GetMatchPredictions(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	if matchID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Match ID is required")
		return
	}

	pred, err := h.prediction.GetMatchPredictions(r.Context(), matchID)
	if err != nil {
		h.logger.Errorw("Failed to get match predictions", "error", err, "matchID", matchID)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	h.jsonResponse(w, http.StatusOK, pred)
}
