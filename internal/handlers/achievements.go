package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/logic"
)

// ============================================================================
// ACHIEVEMENT ENDPOINTS
// ============================================================================

// GetPlayerAchievementProgress returns all achievements with progress for a player (SMF ID-based)
// GET /api/v1/achievements/player/{smf_id}/progress
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

	type UnlockedAchievement struct {
		Slug        string    `json:"slug"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Points      int       `json:"points"`
		Tier        string    `json:"tier"`
		Icon        string    `json:"icon"`
		UnlockedAt  time.Time `json:"unlocked_at"`
	}

	achievements := []UnlockedAchievement{}
	for rows.Next() {
		var a UnlockedAchievement
		if err := rows.Scan(&a.Slug, &a.Name, &a.Description, &a.Points, &a.Tier, &a.Icon, &a.UnlockedAt); err != nil {
			continue
		}
		achievements = append(achievements, a)
	}

	// Also get recent feed if empty? No, just return what we have.
	
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"smf_member_id": smfID,
		"achievements":  achievements,
	})
}

// GetPlayerAchievementStats returns achievement statistics for a player
// GET /api/v1/achievements/player/{smf_id}/stats
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

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"smf_member_id":      smfID,
		"total_achievements": totalAchievements,
		"unlocked_count":     unlockedCount,
		"total_points":       totalPoints,
	})
}

// GetMatchAchievements returns achievements earned in a specific match
// GET /api/v1/achievements/match/{match_id}?player_id={guid}
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
// GET /api/v1/achievements/tournament/{tournament_id}?player_id={guid}
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
