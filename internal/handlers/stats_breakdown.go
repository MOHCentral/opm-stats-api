package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// GetPlayerStatsByGametype returns stats grouped by gametype (derived from map prefix)
func (h *Handler) GetPlayerStatsByGametype(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Derive gametype from map_name prefix (dm_, obj_, lib_, tdm_)
	// Aggregate kills, deaths, headshots per gametype
	rows, err := h.ch.Query(ctx, `
		SELECT 
			multiIf(
				startsWith(map_name, 'dm_'), 'dm',
				startsWith(map_name, 'obj_'), 'obj',
				startsWith(map_name, 'lib_'), 'lib',
				startsWith(map_name, 'tdm_'), 'tdm',
				startsWith(map_name, 'ctf_'), 'ctf',
				'other'
			) as gametype,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type IN ('death', 'kill') AND target_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			uniq(match_id) as matches_played
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?)
		  AND map_name != ''
		GROUP BY gametype
		HAVING kills > 0 OR deaths > 0
		ORDER BY kills DESC
	`, guid, guid, guid, guid, guid)

	if err != nil {
		h.logger.Errorw("Failed to query gametype stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	stats := []models.GametypeStats{}
	for rows.Next() {
		var s models.GametypeStats
		if err := rows.Scan(&s.Gametype, &s.Kills, &s.Deaths, &s.Headshots, &s.MatchesPlayed); err != nil {
			continue
		}
		if s.Deaths > 0 {
			s.KDRatio = float64(s.Kills) / float64(s.Deaths)
		} else if s.Kills > 0 {
			s.KDRatio = float64(s.Kills)
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetPlayerStatsByMap returns detailed stats grouped by map
func (h *Handler) GetPlayerStatsByMap(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	ctx := r.Context()

	// Query map stats - aggregating kills, deaths, headshots per map
	rows, err := h.ch.Query(ctx, `
		SELECT 
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type IN ('death', 'kill') AND target_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			uniq(match_id) as matches_played
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?) 
		  AND map_name != ''
		GROUP BY map_name
		HAVING kills > 0 OR deaths > 0
		ORDER BY kills DESC
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
		if err := rows.Scan(&s.MapName, &s.Kills, &s.Deaths, &s.Headshots, &s.MatchesPlayed); err != nil {
			continue
		}
		if s.Deaths > 0 {
			s.KDRatio = float64(s.Kills) / float64(s.Deaths)
		} else if s.Kills > 0 {
			s.KDRatio = float64(s.Kills)
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}
