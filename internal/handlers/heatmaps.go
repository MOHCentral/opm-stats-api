package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type HeatmapPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Count uint64  `json:"value"` // Intensity
}

// GetMapHeatmap returns global heatmap for a map (all players)
// GET /api/v1/stats/map/{map}/heatmap
func (h *Handler) GetMapHeatmap(w http.ResponseWriter, r *http.Request) {
	mapName := chi.URLParam(r, "map")
	if mapName == "" {
		mapName = "dm/mohdm1" // Default
	}

	heatmapType := r.URL.Query().Get("type") // "kills" or "deaths"
	if heatmapType == "" {
		heatmapType = "kills"
	}

	ctx := r.Context()

	var query string
	// We aggregate by grid cells (50 units) to reduce data volume
	if heatmapType == "deaths" {
		query = `
			SELECT 
				round(pos_x / 50) * 50 as x,
				round(pos_y / 50) * 50 as y,
				count() as intensity
			FROM mohaa_stats.raw_events
			WHERE (event_type = 'player_death' OR event_type = 'player_kill')
			  AND map_name = ?
			  AND pos_x != 0 AND pos_y != 0
			GROUP BY x, y
			HAVING intensity > 0
			LIMIT 3000
		`
	} else {
		// Kills - use actor position (from raw_json)
		query = `
			SELECT 
				round(JSONExtractFloat(raw_json, 'actor_x') / 50) * 50 as x,
				round(JSONExtractFloat(raw_json, 'actor_y') / 50) * 50 as y,
				count() as intensity
			FROM mohaa_stats.raw_events
			WHERE event_type = 'kill'
			  AND map_name = ?
			  AND JSONExtractFloat(raw_json, 'actor_x') != 0 
			  AND JSONExtractFloat(raw_json, 'actor_y') != 0
			GROUP BY x, y
			HAVING intensity > 0
			LIMIT 3000
		`
	}

	rows, err := h.ch.Query(ctx, query, mapName)
	if err != nil {
		h.logger.Errorw("Failed to query heatmap data", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	points := make([]HeatmapPoint, 0)
	for rows.Next() {
		var p HeatmapPoint
		if err := rows.Scan(&p.X, &p.Y, &p.Count); err != nil {
			continue
		}
		points = append(points, p)
	}

	h.jsonResponse(w, http.StatusOK, points)
}
