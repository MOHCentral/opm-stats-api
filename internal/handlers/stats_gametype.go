package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Game type metadata - maps prefix to display info
var gameTypeInfo = map[string]struct {
	Name        string
	Description string
	Icon        string
}{
	"dm":  {"Deathmatch", "Free-for-all combat", "💀"},
	"tdm": {"Team Deathmatch", "Team-based combat", "⚔️"},
	"obj": {"Objective", "Mission-based gameplay", "🎯"},
	"lib": {"Liberation", "Territory control", "🏴"},
	"ctf": {"Capture the Flag", "Flag-based objectives", "🚩"},
	"ffa": {"Free For All", "Every player for themselves", "🔥"},
}

// extractGameType derives game type from map name prefix
func extractGameType(mapName string) string {
	parts := strings.Split(mapName, "/")
	if len(parts) > 0 {
		prefix := strings.ToLower(parts[0])
		// Handle common prefixes
		if strings.HasPrefix(prefix, "dm") {
			return "dm"
		} else if strings.HasPrefix(prefix, "tdm") {
			return "tdm"
		} else if strings.HasPrefix(prefix, "obj") {
			return "obj"
		} else if strings.HasPrefix(prefix, "lib") {
			return "lib"
		} else if strings.HasPrefix(prefix, "ctf") {
			return "ctf"
		} else if strings.HasPrefix(prefix, "ffa") {
			return "ffa"
		}
		return prefix
	}
	// Fallback: check underscore prefix
	if idx := strings.Index(mapName, "_"); idx > 0 {
		return strings.ToLower(mapName[:idx])
	}
	return "unknown"
}

// formatGameTypeName converts prefix to display name
func formatGameTypeName(prefix string) string {
	if info, ok := gameTypeInfo[prefix]; ok {
		return info.Name
	}
	return strings.ToUpper(prefix)
}

// GetGameTypeStats returns all game types with their statistics (derived from map prefixes)
func (h *Handler) GetGameTypeStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Query to aggregate stats by game type prefix derived from map_name
	rows, err := h.ch.Query(ctx, `
		SELECT
			multiIf(
				startsWith(lower(map_name), 'dm'), 'dm',
				startsWith(lower(map_name), 'tdm'), 'tdm',
				startsWith(lower(map_name), 'obj'), 'obj',
				startsWith(lower(map_name), 'lib'), 'lib',
				startsWith(lower(map_name), 'ctf'), 'ctf',
				startsWith(lower(map_name), 'ffa'), 'ffa',
				'other'
			) as game_type,
			count(DISTINCT match_id) as total_matches,
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'kill') as total_deaths,
			count(DISTINCT actor_id) as unique_players,
			count(DISTINCT map_name) as map_count
		FROM mohaa_stats.raw_events
		WHERE map_name != ''
		GROUP BY game_type
		ORDER BY total_matches DESC
	`)
	if err != nil {
		h.logger.Errorw("Failed to get game type stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var gameType string
		var matches, kills, deaths, players, mapCount uint64
		if err := rows.Scan(&gameType, &matches, &kills, &deaths, &players, &mapCount); err == nil {
			info := gameTypeInfo[gameType]
			result = append(result, map[string]interface{}{
				"id":             gameType,
				"name":           formatGameTypeName(gameType),
				"description":    info.Description,
				"icon":           info.Icon,
				"total_matches":  matches,
				"total_kills":    kills,
				"total_deaths":   deaths,
				"unique_players": players,
				"map_count":      mapCount,
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, result)
}

// GetGameTypesList returns a simple list of game types for dropdowns
func (h *Handler) GetGameTypesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT DISTINCT
			multiIf(
				startsWith(lower(map_name), 'dm'), 'dm',
				startsWith(lower(map_name), 'tdm'), 'tdm',
				startsWith(lower(map_name), 'obj'), 'obj',
				startsWith(lower(map_name), 'lib'), 'lib',
				startsWith(lower(map_name), 'ctf'), 'ctf',
				startsWith(lower(map_name), 'ffa'), 'ffa',
				'other'
			) as game_type
		FROM mohaa_stats.raw_events
		WHERE map_name != ''
		ORDER BY game_type
	`)
	if err != nil {
		h.logger.Errorw("Failed to get game types list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	var result []map[string]string
	for rows.Next() {
		var gameType string
		if err := rows.Scan(&gameType); err == nil {
			result = append(result, map[string]string{
				"id":           gameType,
				"name":         formatGameTypeName(gameType),
				"display_name": formatGameTypeName(gameType),
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"gametypes": result})
}

// GetGameTypeDetail returns detailed statistics for a single game type
func (h *Handler) GetGameTypeDetail(w http.ResponseWriter, r *http.Request) {
	gameType := chi.URLParam(r, "gameType")
	if gameType == "" {
		h.errorResponse(w, http.StatusBadRequest, "Game type required")
		return
	}

	ctx := r.Context()

	// Build map pattern for this game type
	mapPattern := gameType + "%"

	// Get aggregate stats
	// Note: total_deaths = total_kills for global stats (each kill = one death)
	var totalMatches, totalKills, totalDeaths, uniquePlayers, mapCount uint64
	row := h.ch.QueryRow(ctx, `
		SELECT
			count(DISTINCT match_id) as total_matches,
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'kill') as total_deaths,
			count(DISTINCT actor_id) as unique_players,
			count(DISTINCT map_name) as map_count
		FROM mohaa_stats.raw_events
		WHERE lower(map_name) LIKE ?
	`, mapPattern)
	row.Scan(&totalMatches, &totalKills, &totalDeaths, &uniquePlayers, &mapCount)

	// Get maps in this game type
	mapRows, err := h.ch.Query(ctx, `
		SELECT
			map_name,
			count(DISTINCT match_id) as matches,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		WHERE lower(map_name) LIKE ?
		GROUP BY map_name
		ORDER BY matches DESC
	`, mapPattern)

	var maps []map[string]interface{}
	if err == nil {
		defer mapRows.Close()
		for mapRows.Next() {
			var mapName string
			var matches, kills uint64
			if err := mapRows.Scan(&mapName, &matches, &kills); err == nil {
				maps = append(maps, map[string]interface{}{
					"name":         mapName,
					"display_name": formatMapName(mapName),
					"matches":      matches,
					"kills":        kills,
				})
			}
		}
	}

	info := gameTypeInfo[gameType]
	response := map[string]interface{}{
		"id":             gameType,
		"name":           formatGameTypeName(gameType),
		"description":    info.Description,
		"icon":           info.Icon,
		"total_matches":  totalMatches,
		"total_kills":    totalKills,
		"total_deaths":   totalDeaths,
		"unique_players": uniquePlayers,
		"map_count":      mapCount,
		"maps":           maps,
	}

	h.jsonResponse(w, http.StatusOK, response)
}
