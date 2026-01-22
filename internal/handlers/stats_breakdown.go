package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// GetPlayerStatsByGametype returns win/loss stats grouped by gametype
func (h *Handler) GetPlayerStatsByGametype(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Query match_outcome events, grouping by actor_weapon (where we stored gametype)
	// We use 'actor_weapon' column to retrieve the gametype string
	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_weapon as gametype,
			count() as matches_played,
			countIf(damage = 1) as matches_won,
			countIf(damage = 0) as matches_lost
		FROM mohaa_stats.raw_events
		WHERE event_type = 'match_outcome' 
		  AND actor_id = ?
		  AND gametype != ''
		GROUP BY gametype
		ORDER BY matches_played DESC
	`, guid)

	if err != nil {
		h.logger.Errorw("Failed to query gametype stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	stats := []models.GametypeStats{}
	for rows.Next() {
		var s models.GametypeStats
		if err := rows.Scan(&s.Gametype, &s.MatchesPlayed, &s.MatchesWon, &s.MatchesLost); err != nil {
			continue
		}
		if s.MatchesPlayed > 0 {
			s.WinRate = float64(s.MatchesWon) / float64(s.MatchesPlayed) * 100
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerStatsByMap returns detailed stats grouped by map
func (h *Handler) GetPlayerStatsByMap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Query map stats - aggregating kills, deaths, matches
	// For matches, we use uniq(match_id) from all events on that map
	// For wins, we check match_outcome
	rows, err := h.ch.Query(ctx, `
		SELECT 
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'death' AND target_id = ?) as deaths,
			uniq(match_id) as matches_played,
			countIf(event_type = 'match_outcome' AND actor_id = ? AND damage = 1) as matches_won
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?) 
		  AND map_name != ''
		GROUP BY map_name
		ORDER BY matches_played DESC
	`, guid, guid, guid, guid, guid)

	if err != nil {
		h.logger.Errorw("Failed to query map breakdown", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	stats := []models.MapStats{}
	for rows.Next() {
		var s models.MapStats
		if err := rows.Scan(&s.MapName, &s.Kills, &s.Deaths, &s.MatchesPlayed, &s.MatchesWon); err != nil {
			continue
		}
		if s.MatchesPlayed > 0 {
			s.WinRate = float64(s.MatchesWon) / float64(s.MatchesPlayed) * 100
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}
