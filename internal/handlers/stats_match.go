package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openmohaa/stats-api/internal/models"
)

// GetMatches returns a list of recent matches
// @Summary Get Recent Matches
// @Tags Match
// @Produce json
// @Param limit query int false "Limit" default(25)
// @Success 200 {array} models.MatchSummary "Matches"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/matches [get]
func (h *Handler) GetMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// Fetch matches
	rows, err := h.ch.Query(ctx, `
		SELECT
			toString(match_id) as match_id,
			map_name,
			any(server_id) as server_id,
			min(timestamp) as start_time,
			toFloat64(dateDiff('second', min(timestamp), max(timestamp))) as duration,
			uniq(actor_id) as player_count,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		GROUP BY match_id, map_name
		ORDER BY start_time DESC
		LIMIT ? OFFSET ?
	`, limit, offset)

	if err != nil {
		h.logger.Errorw("Failed to fetch matches", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	matches := make([]models.MatchSummary, 0)
	serverIDs := make(map[string]bool)
	for rows.Next() {
		var m models.MatchSummary
		if err := rows.Scan(&m.ID, &m.Map, &m.ServerID, &m.StartTime, &m.Duration, &m.PlayerCount, &m.Kills); err != nil {
			h.logger.Warnw("Scan error in GetMatches", "error", err)
			continue
		}
		matches = append(matches, m)
		serverIDs[m.ServerID] = true
	}

	// Look up server names from PostgreSQL
	serverNames := make(map[string]string)
	for serverID := range serverIDs {
		if serverID == "" {
			continue
		}
		var name string
		err := h.pg.QueryRow(ctx, "SELECT name FROM servers WHERE id = $1", serverID).Scan(&name)
		if err == nil {
			serverNames[serverID] = name
		}
	}

	// Apply server names to matches
	for i := range matches {
		if name, ok := serverNames[matches[i].ServerID]; ok {
			matches[i].ServerName = name
		} else if matches[i].ServerID != "" {
			matches[i].ServerName = "Unknown Server"
		}
	}

	h.jsonResponse(w, http.StatusOK, matches)
}

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

// GetLiveMatches returns currently active matches
func (h *Handler) GetLiveMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all live matches from Redis
	matchData, err := h.redis.HGetAll(ctx, "live_matches").Result()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch live matches")
		return
	}

	var matches []models.LiveMatch
	for _, data := range matchData {
		var match models.LiveMatch
		if err := json.Unmarshal([]byte(data), &match); err == nil {
			matches = append(matches, match)
		}
	}

	h.jsonResponse(w, http.StatusOK, matches)
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

// GetMatchPredictions returns predictions for a match
func (h *Handler) GetMatchPredictions(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	predictions, err := h.prediction.GetMatchPredictions(r.Context(), matchID)
	if err != nil {
		h.logger.Errorw("Failed to get match predictions", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal error")
		return
	}
	h.jsonResponse(w, http.StatusOK, predictions)
}
