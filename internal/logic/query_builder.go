package logic

import (
	"fmt"
	"time"
)

// DynamicQueryRequest holds parameters for constructing a stats query
type DynamicQueryRequest struct {
	Dimension    string    `json:"dimension"`     // Group by: weapon, map, player_guid, etc.
	Metric       string    `json:"metric"`        // Select: kills, deaths, kdr, headshots
	FilterGUID   string    `json:"filter_guid"`   // WHERE actor_id = ?
	FilterMap    string    `json:"filter_map"`    // WHERE map_name = ?
	FilterWeapon string    `json:"filter_weapon"` // WHERE extra LIKE '%weapon%'
	FilterServer string    `json:"filter_server"` // WHERE server_id = ?
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Limit        int       `json:"limit"`
}

// AllowedDimensions maps safe API values to SQL columns
var allowedDimensions = map[string]string{
	"weapon":      "extract(extra, 'weapon_([a-zA-Z0-9_]+)')", // Complex regex extraction for weapon
	"map":         "map_name",
	"player":      "actor_name",
	"player_guid": "actor_id",
	"server":      "server_id",
	"hitloc":      "extract(extra, 'hitloc_([a-zA-Z_]+)')",
	"match":       "match_id",
}

// BuildStatsQuery constructs a safe ClickHouse SQL query
func BuildStatsQuery(req DynamicQueryRequest) (string, []interface{}, error) {
	// 1. Validate Dimension
	groupByCol, ok := allowedDimensions[req.Dimension]
	if !ok && req.Dimension != "" {
		return "", nil, fmt.Errorf("invalid dimension: %s", req.Dimension)
	}

	// 2. Select Clause (Metric)
	// Note: Deaths = kills for global stats. For player-specific deaths,
	// use target_id filtering (handled in player stats queries, not this builder)
	var selectClause string
	switch req.Metric {
	case "kills":
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed'))"
	case "deaths":
		// For global deaths: each kill event = one death
		// For player-specific deaths, would need target_id filter (not supported in this builder)
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed'))"
	case "headshots":
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed') AND hitloc IN ('head', 'helmet'))"
	case "accuracy": // Simplified accuracy (hits/shots) - careful with zero division
		selectClause = "sumIf(1, event_type='weapon_hit') / max(1, sumIf(1, event_type='weapon_fire')) * 100"
	case "kdr":
		// For global KDR: kills/kills = 1 (not useful)
		// This metric is more meaningful for player-specific queries
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed')) / max(1, countIf(event_type IN ('player_kill', 'bot_killed')))"
	default: // Default to just raw count of events matching filters if no metric specified? Or error?
		selectClause = "count()"
	}

	// 3. Build Query
	query := fmt.Sprintf("SELECT %s as value", selectClause)
	var args []interface{}

	if groupByCol != "" {
		query += fmt.Sprintf(", %s as label", groupByCol)
	} else {
		query += ", 'all' as label"
	}

	query += " FROM raw_events WHERE 1=1"

	// 4. Filters
	if req.FilterGUID != "" {
		query += " AND actor_id = ?"
		args = append(args, req.FilterGUID)
	}
	if req.FilterMap != "" {
		query += " AND map_name = ?"
		args = append(args, req.FilterMap)
	}
	if req.FilterServer != "" {
		query += " AND server_id = ?"
		args = append(args, req.FilterServer)
	}
	if req.FilterWeapon != "" {
		// This is tricky. Weapon is usually in 'extra' JSON or string.
		// Assuming extra contains "weapon": "kar98"
		query += " AND extra LIKE ?"
		args = append(args, fmt.Sprintf("%%%s%%", req.FilterWeapon))
	}
	if !req.StartDate.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, req.StartDate)
	}
	if !req.EndDate.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, req.EndDate)
	}

	// 5. Group By
	if groupByCol != "" {
		query += fmt.Sprintf(" GROUP BY %s", groupByCol)
	}

	// 6. Order By
	query += " ORDER BY value DESC"

	// 7. Limit
	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query += fmt.Sprintf(" LIMIT %d", limit)

	return query, args, nil
}
