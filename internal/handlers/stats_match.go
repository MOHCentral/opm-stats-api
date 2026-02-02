package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// GetMatchDetails returns full details for a match
func (h *Handler) GetMatchDetails(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	// Get match summary - use any() for map_name since we need aggregate
	row := h.ch.QueryRow(ctx, `
		SELECT
			any(map_name) as map_name,
			min(timestamp) as started,
			max(timestamp) as ended,
			countIf(event_type = 'kill') as total_kills,
			uniq(actor_id) as unique_players
		FROM mohaa_stats.raw_events
		WHERE match_id = ?
	`, matchID)

	var summary struct {
		MapName       string    `json:"map_name"`
		StartedAt     time.Time `json:"started_at"`
		EndedAt       time.Time `json:"ended_at"`
		TotalKills    uint64    `json:"total_kills"`
		UniquePlayers uint64    `json:"unique_players"`
	}

	if err := row.Scan(&summary.MapName, &summary.StartedAt, &summary.EndedAt, &summary.TotalKills, &summary.UniquePlayers); err != nil {
		h.errorResponse(w, http.StatusNotFound, "Match not found")
		return
	}

	// Get player scoreboard - needs subquery for deaths since death = being target_id in kill events
	rows, err := h.ch.Query(ctx, `
		SELECT
			p.player_id as actor_id,
			p.player_name as actor_name,
			p.kills,
			ifNull(d.deaths, 0) as deaths,
			p.headshots
		FROM (
			SELECT
				actor_id as player_id,
				any(actor_name) as player_name,
				countIf(event_type = 'kill') as kills,
				countIf(event_type = 'headshot') as headshots
			FROM mohaa_stats.raw_events
			WHERE match_id = ? AND actor_id != '' AND actor_id != 'world'
			GROUP BY actor_id
		) p
		LEFT JOIN (
			SELECT target_id, count() as deaths
			FROM mohaa_stats.raw_events
			WHERE match_id = ? AND event_type = 'kill' AND target_id != ''
			GROUP BY target_id
		) d ON p.player_id = d.target_id
		ORDER BY p.kills DESC
	`, matchID, matchID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type PlayerScore struct {
		PlayerID   string `json:"player_id"`
		PlayerName string `json:"player_name"`
		Kills      uint64 `json:"kills"`
		Deaths     uint64 `json:"deaths"`
		Headshots  uint64 `json:"headshots"`
	}

	var scoreboard []PlayerScore
	for rows.Next() {
		var p PlayerScore
		if err := rows.Scan(&p.PlayerID, &p.PlayerName, &p.Kills, &p.Deaths, &p.Headshots); err != nil {
			continue
		}
		scoreboard = append(scoreboard, p)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"match_id":   matchID,
		"summary":    summary,
		"scoreboard": scoreboard,
	})
}

// GetMatchHeatmap returns kill/death locations for a specific match
func (h *Handler) GetMatchHeatmap(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	// Query individual kill events with coordinates
	rows, err := h.ch.Query(ctx, `
		SELECT
			actor_id,
			target_id,
			actor_pos_x,
			actor_pos_y,
			target_pos_x,
			target_pos_y
		FROM mohaa_stats.raw_events
		WHERE match_id = ?
		  AND event_type = 'kill'
		  AND actor_pos_x != 0 AND target_pos_x != 0
		LIMIT 2000
	`, matchID)
	if err != nil {
		h.logger.Errorw("Failed to query match heatmap", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type Point struct {
		ID    int     `json:"id"`
		Type  string  `json:"type"` // "kill" or "death"
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		Label string  `json:"label"`
	}

	var points []Point
	id := 0

	for rows.Next() {
		var actorID, targetID string
		var ax, ay, tx, ty float64
		if err := rows.Scan(&actorID, &targetID, &ax, &ay, &tx, &ty); err != nil {
			continue
		}

		// Killer position (green)
		points = append(points, Point{
			ID:    id,
			Type:  "kill",
			X:     ax,
			Y:     ay,
			Label: "Killer: " + actorID,
		})
		id++

		// Victim position (red)
		points = append(points, Point{
			ID:    id,
			Type:  "death",
			X:     tx,
			Y:     ty,
			Label: "Victim: " + targetID,
		})
		id++
	}

	h.jsonResponse(w, http.StatusOK, points)
}

// GetMatchTimeline returns chronological events for match replay
func (h *Handler) GetMatchTimeline(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT
			timestamp,
			event_type,
			actor_name,
			target_name,
			actor_weapon,
			hitloc
		FROM mohaa_stats.raw_events
		WHERE match_id = ? AND event_type IN ('kill', 'round_start', 'round_end')
		ORDER BY timestamp
		LIMIT 1000
	`, matchID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type TimelineEvent struct {
		Timestamp  time.Time `json:"timestamp"`
		EventType  string    `json:"event_type"`
		ActorName  string    `json:"actor_name"`
		TargetName string    `json:"target_name"`
		Weapon     string    `json:"weapon"`
		Hitloc     string    `json:"hitloc"`
	}

	var events []TimelineEvent
	for rows.Next() {
		var e TimelineEvent
		if err := rows.Scan(&e.Timestamp, &e.EventType, &e.ActorName, &e.TargetName, &e.Weapon, &e.Hitloc); err != nil {
			continue
		}
		events = append(events, e)
	}

	h.jsonResponse(w, http.StatusOK, events)
}

// GetMatchAdvancedDetails returns deep analysis for a match
func (h *Handler) GetMatchAdvancedDetails(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	details, err := h.matchReport.GetMatchDetails(r.Context(), matchID)
	if err != nil {
		h.logger.Errorw("Failed to get match details", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal error")
		return
	}
	h.jsonResponse(w, http.StatusOK, details)
}
