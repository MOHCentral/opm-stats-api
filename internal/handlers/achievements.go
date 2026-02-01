package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
	"github.com/openmohaa/stats-api/internal/models"
)

// ============================================================================
// ACHIEVEMENT ENDPOINTS
// ============================================================================

// GetPlayerAchievementProgress returns all achievements with progress for a player (SMF ID-based)
// @Summary Get Player Achievement Progress
// @Description Returns all unlocked achievements for a player by SMF ID
// @Tags Achievements
// @Produce json
// @Param smf_id path int true "SMF Member ID"
// @Success 200 {object} models.PlayerAchievementProgressResponse "Achievement Progress"
// @Failure 400 {object} map[string]string "Invalid ID"
// @Failure 500 {object} map[string]string "Database Error"
// @Router /achievements/player/{smf_id}/progress [get]
func (h *Handler) GetPlayerAchievementProgress(w http.ResponseWriter, r *http.Request) {
	smfIDStr := chi.URLParam(r, "smf_id")
	smfID, err := strconv.Atoi(smfIDStr)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid SMF ID")
		return
	}

	ctx := r.Context()

	// Query unlocked achievements
	rows, err := h.pg.Query(ctx, `
		SELECT 
			a.achievement_code,
			a.achievement_name,
			a.description,
			a.points,
			a.tier,
			a.icon_url,
			pa.unlocked_at
		FROM mohaa_player_achievements pa
		JOIN mohaa_achievements a ON pa.achievement_id = a.achievement_id
		WHERE pa.smf_member_id = $1 AND pa.unlocked = true
		ORDER BY pa.unlocked_at DESC
	`, smfID)

	if err != nil {
		h.logger.Errorw("Failed to fetch player achievements", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	achievements := []models.UnlockedAchievement{}
	for rows.Next() {
		var a models.UnlockedAchievement
		if err := rows.Scan(&a.Slug, &a.Name, &a.Description, &a.Points, &a.Tier, &a.Icon, &a.UnlockedAt); err != nil {
			continue
		}
		achievements = append(achievements, a)
	}

	// Also get recent feed if empty? No, just return what we have.

	h.jsonResponse(w, http.StatusOK, models.PlayerAchievementProgressResponse{
		SmfMemberID:  smfID,
		Achievements: achievements,
	})
}

// GetPlayerAchievementStats returns achievement statistics for a player
// @Summary Get Player Achievement Stats
// @Description Returns achievement totals and points for a player
// @Tags Achievements
// @Produce json
// @Param smf_id path int true "SMF Member ID"
// @Success 200 {object} models.PlayerAchievementStatsResponse "Achievement Stats"
// @Failure 400 {object} map[string]string "Invalid ID"
// @Router /achievements/player/{smf_id}/stats [get]
func (h *Handler) GetPlayerAchievementStats(w http.ResponseWriter, r *http.Request) {
	smfIDStr := chi.URLParam(r, "smf_id")
	smfID, err := strconv.Atoi(smfIDStr)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid SMF ID")
		return
	}

	ctx := r.Context()
	var totalAchievements, unlockedCount, totalPoints int

	// Get total available achievements
	err = h.pg.QueryRow(ctx, "SELECT COUNT(*) FROM mohaa_achievements").Scan(&totalAchievements)
	if err != nil {
		h.logger.Errorw("Failed to count achievements", "error", err)
	}

	// Get unlocked count and points
	err = h.pg.QueryRow(ctx, `
		SELECT 
			COUNT(*), 
			COALESCE(SUM(a.points), 0)
		FROM mohaa_player_achievements pa
		JOIN mohaa_achievements a ON pa.achievement_id = a.achievement_id
		WHERE pa.smf_member_id = $1 AND pa.unlocked = true
	`, smfID).Scan(&unlockedCount, &totalPoints)

	if err != nil {
		h.logger.Errorw("Failed to get player achievement stats", "error", err)
	}

	h.jsonResponse(w, http.StatusOK, models.PlayerAchievementStatsResponse{
		SmfMemberID:       smfID,
		TotalAchievements: totalAchievements,
		UnlockedCount:     unlockedCount,
		TotalPoints:       totalPoints,
	})
}

// GetMatchAchievements returns achievements earned in a specific match
// @Summary Get Match Achievements
// @Description Returns achievements earned during a specific match
// @Tags Achievements
// @Produce json
// @Param match_id path string true "Match ID"
// @Param player_id query string true "Player GUID"
// @Success 200 {array} models.Achievement "Achievements"
// @Failure 400 {object} map[string]string "Invalid Params"
// @Failure 500 {object} map[string]string "Server Error"
// @Router /achievements/match/{match_id} [get]
func (h *Handler) GetMatchAchievements(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "match_id")
	playerID := r.URL.Query().Get("player_id")

	if matchID == "" || playerID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing match_id or player_id")
		return
	}

	list, err := h.achievements.GetAchievements(r.Context(), logic.ScopeMatch, matchID, playerID)
	if err != nil {
		h.logger.Errorw("Failed to get match achievements", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get achievements")
		return
	}
	h.jsonResponse(w, http.StatusOK, list)
}

// GetTournamentAchievements returns achievements earned in a tournament
// @Summary Get Tournament Achievements
// @Description Returns achievements earned during a specific tournament
// @Tags Achievements
// @Produce json
// @Param tournament_id path string true "Tournament ID"
// @Param player_id query string true "Player GUID"
// @Success 200 {array} models.Achievement "Achievements"
// @Failure 400 {object} map[string]string "Invalid Params"
// @Failure 500 {object} map[string]string "Server Error"
// @Router /achievements/tournament/{tournament_id} [get]
func (h *Handler) GetTournamentAchievements(w http.ResponseWriter, r *http.Request) {
	tournID := chi.URLParam(r, "tournament_id")
	playerID := r.URL.Query().Get("player_id")

	if tournID == "" || playerID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing tournament_id or player_id")
		return
	}

	list, err := h.achievements.GetAchievements(r.Context(), logic.ScopeTournament, tournID, playerID)
	if err != nil {
		h.logger.Errorw("Failed to get tournament achievements", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get achievements")
		return
	}
	h.jsonResponse(w, http.StatusOK, list)
}

// GetPlayerAchievements returns player achievements (GUID based)
func (h *Handler) GetPlayerAchievements(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	achievements, err := h.achievements.GetPlayerAchievements(r.Context(), guid)
	if err != nil {
		h.logger.Errorw("Failed to get player achievements", "error", err, "guid", guid)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get achievements")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"achievements": achievements,
	})
}

// ListAchievements returns a message directing to SMF database
// Achievement definitions are stored in SMF MariaDB, not Go
func (h *Handler) ListAchievements(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Achievement definitions are stored in SMF database (smf_mohaa_achievement_defs). Use the SMF forum to view achievements.",
		"source":  "smf_database",
	})
}

// GetAchievement returns a message directing to SMF database
func (h *Handler) GetAchievement(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Achievement definitions are stored in SMF database. Use the SMF forum to view achievements.",
		"source":  "smf_database",
	})
}

// GetRecentAchievements returns a global feed of recent unlocks from database
func (h *Handler) GetRecentAchievements(w http.ResponseWriter, r *http.Request) {
	// Recent achievement unlocks are stored in SMF database
	// Return empty array - frontend should query SMF directly or use PHP endpoint
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}

// GetAchievementLeaderboard returns players ranked by achievement points
func (h *Handler) GetAchievementLeaderboard(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	// Achievement data is stored in SMF database - return empty array
	h.jsonResponse(w, http.StatusOK, []interface{}{})
}
