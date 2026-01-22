// Web page helper methods for dashboard and page data fetching
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ============================================================================
// Web Page Helper Methods
// ============================================================================

func (h *Handler) getDashboardStats(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats
	row := h.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as kills,
			uniq(actor_id) as players,
			uniq(match_id) as matches
		FROM mohaa_stats.raw_events
	`)
	if err := row.Scan(&stats.TotalKills, &stats.ActivePlayers, &stats.MatchesPlayed); err != nil {
		h.logger.Errorw("Failed to get dashboard stats", "error", err)
		return &DashboardStats{}, err
	}

	// Live matches from Redis
	vals, _ := h.redis.HVals(ctx, "live_matches").Result()
	stats.LiveMatches = len(vals)

	return &stats, nil
}

func (h *Handler) getLiveMatchCount(ctx context.Context) int {
	count, _ := h.redis.HLen(ctx, "live_matches").Result()
	return int(count)
}

func (h *Handler) getUserFromSession(r *http.Request) (interface{}, error) {
	// Session data is extracted via JWT middleware and stored in context
	// Returns nil if no user is authenticated (anonymous access is allowed)
	return r.Context().Value("user"), nil
}

func (h *Handler) getPlayerProfile(ctx context.Context, guid string) (*PlayerProfile, error) {
	// Try to get name and last activity from ClickHouse
	var name string
	var lastActive time.Time
	err := h.ch.QueryRow(ctx, `
		SELECT any(actor_name), max(timestamp) FROM mohaa_stats.raw_events WHERE actor_id = ?
	`, guid).Scan(&name, &lastActive)

	if err != nil || name == "" {
		name = "Unknown Soldier"
	}
	if lastActive.IsZero() {
		lastActive = time.Time{} // Keep as zero time if no activity
	}

	return &PlayerProfile{
		GUID:       guid,
		Name:       name,
		Verified:   false,
		Rank:       0, // Requires separate ranking query
		LastActive: lastActive,
	}, nil
}

func (h *Handler) getPlayerStats(ctx context.Context, guid string) (*PlayerStats, error) {
	var stats PlayerStats
	// Reuse logic from GetPlayerStats query
	row := h.ch.QueryRow(ctx, `
		SELECT
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'death' AND actor_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			countIf(event_type = 'weapon_fire' AND actor_id = ?) as shots,
			countIf(event_type = 'weapon_hit' AND actor_id = ?) as hits,
			uniq(match_id) as matches
		FROM mohaa_stats.raw_events
		WHERE actor_id = ?
	`, guid, guid, guid, guid, guid, guid)

	var shots, hits int64
	if err := row.Scan(&stats.Kills, &stats.Deaths, &stats.Headshots, &shots, &hits, &stats.Matches); err != nil {
		return nil, err
	}

	if stats.Deaths > 0 {
		stats.KDRatio = float64(stats.Kills) / float64(stats.Deaths)
	} else {
		stats.KDRatio = float64(stats.Kills)
	}

	if stats.Kills > 0 {
		stats.HSPercent = float64(stats.Headshots) / float64(stats.Kills) * 100
	}

	if shots > 0 {
		stats.Accuracy = float64(hits) / float64(shots) * 100
	}

	return &stats, nil
}

func (h *Handler) getPlayerTopWeapons(ctx context.Context, guid string, limit int) ([]WeaponStat, error) {
	rows, err := h.ch.Query(ctx, `
		SELECT 
			extract(extra, 'weapon_([a-zA-Z0-9_]+)') as weapon,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'headshot') as headshots
		FROM mohaa_stats.raw_events 
		WHERE actor_id = ? AND event_type IN ('kill', 'headshot')
		GROUP BY weapon
		ORDER BY kills DESC
		LIMIT ?
	`, guid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var weapons []WeaponStat
	for rows.Next() {
		var w WeaponStat
		if err := rows.Scan(&w.Name, &w.Kills, &w.Headshots); err != nil {
			continue
		}
		weapons = append(weapons, w)
	}
	return weapons, nil
}

func (h *Handler) getPlayerAchievements(ctx context.Context, guid string) ([]Achievement, error) {
	// Query the new player_achievements table
	// For now, return empty or implement full query if table is populated
	return []Achievement{}, nil
}

func (h *Handler) getPlayerAllWeaponStats(ctx context.Context, guid string) ([]WeaponStat, error) {
	return h.getPlayerTopWeapons(ctx, guid, 100)
}

func (h *Handler) getMatchDetails(ctx context.Context, matchID string) (*MatchDetails, error) {
	// Query match details from ClickHouse
	var mapName string
	var startTime, endTime time.Time
	err := h.ch.QueryRow(ctx, `
		SELECT 
			any(map_name),
			min(timestamp),
			max(timestamp)
		FROM mohaa_stats.raw_events 
		WHERE match_id = ?
	`, matchID).Scan(&mapName, &startTime, &endTime)

	if err != nil {
		return &MatchDetails{ID: matchID, MapName: ""}, err
	}

	return &MatchDetails{
		ID:      matchID,
		MapName: mapName,
	}, nil
}

func (h *Handler) getGlobalRecords(ctx context.Context) (*GlobalRecords, error) {
	records := &GlobalRecords{}

	// Max Kills in a match
	h.ch.QueryRow(ctx, `
		SELECT any(actor_name), max(kills) 
		FROM (
			SELECT match_id, actor_name, count() as kills 
			FROM mohaa_stats.raw_events WHERE event_type='kill' 
			GROUP BY match_id, actor_name
		)
	`).Scan(&records.MostKillsMatch.PlayerName, &records.MostKillsMatch.Value)

	// Longest Shot
	h.ch.QueryRow(ctx, `
		SELECT any(actor_name), max(distance)
		FROM mohaa_stats.raw_events WHERE event_type='kill' AND distance < 10000 -- Sanity check
	`).Scan(&records.LongestShot.PlayerName, &records.LongestShot.Value)

	return records, nil
}

func (h *Handler) getMapsList(ctx context.Context) ([]MapInfo, error) {
	rows, err := h.ch.Query(ctx, `
		SELECT 
			map_name,
			countIf(event_type = 'match_start') as matches,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		WHERE map_name != ''
		GROUP BY map_name
		ORDER BY matches DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var maps []MapInfo
	for rows.Next() {
		var name string
		var matches, kills uint64
		if err := rows.Scan(&name, &matches, &kills); err != nil {
			h.logger.Warnw("Failed to scan map row", "error", err)
			continue
		}
		maps = append(maps, MapInfo{
			ID:           name,
			Name:         name,
			TotalMatches: int64(matches),
			TotalKills:   int64(kills),
			AvgDuration:  0,
		})
	}
	return maps, nil
}

func (h *Handler) getMapDetails(ctx context.Context, mapID string) (*MapInfo, error) {
	// Re-use list logic for now, but filtered
	// Ideally this would be a specific detailed query
	m := &MapInfo{ID: mapID, Name: mapID}

	var matches, kills uint64
	err := h.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'match_start') as matches,
			countIf(event_type = 'kill') as kills
		FROM mohaa_stats.raw_events
		WHERE map_name = ?
	`, mapID).Scan(&matches, &kills)

	if err != nil {
		return nil, err
	}
	m.TotalMatches = int64(matches)
	m.TotalKills = int64(kills)
	return m, nil
}

// NOTE: Tournament helpers removed - SMF MariaDB is the source of truth
// See: smf-plugins/mohaa_tournaments/ for tournament management

func (h *Handler) getLiveMatches(ctx context.Context) ([]interface{}, error) {
	matchData, err := h.redis.HGetAll(ctx, "live_matches").Result()
	if err != nil {
		return nil, err
	}

	var matches []interface{}
	for _, data := range matchData {
		var match map[string]interface{}
		if err := json.Unmarshal([]byte(data), &match); err == nil {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (h *Handler) getTopPlayers(ctx context.Context, limit int) ([]interface{}, error) {
	rows, err := h.ch.Query(ctx, `
		SELECT 
			actor_id,
			any(actor_name) as name,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'death') as deaths,
			countIf(event_type = 'headshot') as headshots,
			uniq(match_id) as matches
		FROM mohaa_stats.raw_events
		WHERE actor_id != ''
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []interface{}
	for rows.Next() {
		var p struct {
			GUID      string  `json:"guid"`
			Name      string  `json:"name"`
			Kills     int64   `json:"kills"`
			Deaths    int64   `json:"deaths"`
			Headshots int64   `json:"headshots"`
			Matches   int64   `json:"matches"`
			KDRatio   float64 `json:"kd_ratio"`
		}
		if err := rows.Scan(&p.GUID, &p.Name, &p.Kills, &p.Deaths, &p.Headshots, &p.Matches); err != nil {
			continue
		}
		if p.Deaths > 0 {
			p.KDRatio = float64(p.Kills) / float64(p.Deaths)
		} else {
			p.KDRatio = float64(p.Kills)
		}
		players = append(players, p)
	}
	return players, nil
}

func (h *Handler) getRecentMatches(ctx context.Context, offset, limit int) ([]interface{}, bool) {
	return []interface{}{}, false
}

func (h *Handler) getPlayerMatchHistory(ctx context.Context, guid string, offset, limit int) ([]interface{}, bool) {
	return []interface{}{}, false
}
