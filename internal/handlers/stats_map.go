package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// GetMapStats returns all maps with their statistics
func (h *Handler) GetMapStats(w http.ResponseWriter, r *http.Request) {
	maps, err := h.getMapsList(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get map stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	h.jsonResponse(w, http.StatusOK, maps)
}

// GetMapsList returns a simple list of maps for dropdowns
func (h *Handler) GetMapsList(w http.ResponseWriter, r *http.Request) {
	maps, err := h.getMapsList(r.Context())
	if err != nil {
		h.logger.Errorw("Failed to get maps list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Return simplified list for dropdown
	type mapItem struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}

	result := make([]mapItem, len(maps))
	for i, m := range maps {
		result[i] = mapItem{
			Name:        m.Name,
			DisplayName: formatMapName(m.Name),
		}
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"maps": result})
}

// GetMapDetail returns detailed statistics for a single map
func (h *Handler) GetMapDetail(w http.ResponseWriter, r *http.Request) {
	mapID := chi.URLParam(r, "mapId")
	if mapID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Map ID required")
		return
	}

	ctx := r.Context()
	mapInfo, err := h.getMapDetails(ctx, mapID)
	if err != nil {
		h.logger.Errorw("Failed to get map details", "error", err, "map", mapID)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Get top players on this map
	var topPlayers []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Kills  int    `json:"kills"`
		Deaths int    `json:"deaths"`
	}

	rows, err := h.ch.Query(ctx, `
		SELECT
			player_guid as id,
			any(player_name) as name,
			countIf(event_type = 'kill' AND raw_json->>'attacker_guid' = player_guid) as kills,
			countIf(event_type = 'kill' AND raw_json->>'victim_guid' = player_guid) as deaths
		FROM mohaa_stats.raw_events
		WHERE map_name = ?
		GROUP BY player_guid
		ORDER BY kills DESC
		LIMIT 25
	`, mapID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Kills  int    `json:"kills"`
				Deaths int    `json:"deaths"`
			}
			if err := rows.Scan(&p.ID, &p.Name, &p.Kills, &p.Deaths); err == nil {
				topPlayers = append(topPlayers, p)
			}
		}
	}

	// Get heatmap data
	heatmapData := make(map[string]interface{})
	killsHeatmap, _ := h.getMapHeatmapData(ctx, mapID, "kills")
	deathsHeatmap, _ := h.getMapHeatmapData(ctx, mapID, "deaths")
	heatmapData["kills"] = killsHeatmap
	heatmapData["deaths"] = deathsHeatmap

	response := map[string]interface{}{
		"map_name":       mapInfo.Name,
		"display_name":   formatMapName(mapInfo.Name),
		"total_matches":  mapInfo.TotalMatches,
		"total_kills":    mapInfo.TotalKills,
		"total_playtime": int64(mapInfo.AvgDuration) * mapInfo.TotalMatches,
		"avg_duration":   mapInfo.AvgDuration,
		"top_players":    topPlayers,
		"heatmap_data":   heatmapData,
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// formatMapName converts map filename to display name
func formatMapName(name string) string {
	// Remove common prefixes
	displayName := name
	prefixes := []string{"mp_", "dm_", "obj_", "lib_"}
	for _, prefix := range prefixes {
		if len(displayName) > len(prefix) && displayName[:len(prefix)] == prefix {
			displayName = displayName[len(prefix):]
			break
		}
	}
	// Capitalize first letter
	if len(displayName) > 0 {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}
	return displayName
}

// getMapHeatmapData returns heatmap coordinates for a map
func (h *Handler) getMapHeatmapData(ctx context.Context, mapID, heatmapType string) ([]map[string]interface{}, error) {
	eventType := "kill"
	if heatmapType == "deaths" {
		eventType = "death"
	}

	rows, err := h.ch.Query(ctx, `
		SELECT
			toFloat64OrZero(raw_json->>'pos_x') as x,
			toFloat64OrZero(raw_json->>'pos_y') as y,
			count() as intensity
		FROM mohaa_stats.raw_events
		WHERE map_name = ? AND event_type = ?
			AND raw_json->>'pos_x' != '' AND raw_json->>'pos_y' != ''
		GROUP BY x, y
		HAVING intensity > 0
		ORDER BY intensity DESC
		LIMIT 500
	`, mapID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var x, y float64
		var intensity int64
		if err := rows.Scan(&x, &y, &intensity); err == nil {
			result = append(result, map[string]interface{}{
				"x":     x,
				"y":     y,
				"value": intensity,
			})
		}
	}
	return result, nil
}
