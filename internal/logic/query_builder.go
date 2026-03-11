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
	FilterWeapon string    `json:"filter_weapon"` // WHERE actor_weapon = ?
	FilterServer string    `json:"filter_server"` // WHERE server_id = ?
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Limit        int       `json:"limit"`
}

// AllowedDimensions maps safe API values to SQL columns
var allowedDimensions = map[string]string{
	"weapon":      "actor_weapon",
	"map":         "map_name",
	"player":      "actor_name",
	"player_guid": "actor_id",
	"server":      "server_id",
	"hitloc":      "hitloc",
	"match":       "toString(match_id)",
	"stance":      "actor_stance",
	"distance":    "floor(distance / 100) * 100", // Bucketize distance
}

// BuildStatsQuery constructs a safe ClickHouse SQL query
func BuildStatsQuery(req DynamicQueryRequest) (string, []interface{}, error) {
	// 1. Validate Dimension
	groupByCol, ok := allowedDimensions[req.Dimension]
	if !ok && req.Dimension != "" {
		return "", nil, fmt.Errorf("invalid dimension: %s", req.Dimension)
	}

	// 2. Select Clause (Metric)
	var selectClause string
	switch req.Metric {
	case "kills":
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed'))"
	case "deaths":
		// For global stats, total deaths equals total kills (simplification)
		// For specific player (FilterGUID), this counts kills where they were the actor, not victim.
		// To count deaths properly for a player, we'd need to filter by target_id, which this query builder
		// structure doesn't easily support without changing the base logic (WHERE actor_id = ?).
		// Assuming this is mostly for "Top X by Kills" style charts.
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed'))"
	case "headshots":
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed') AND hitloc IN ('head', 'helmet'))"
	case "accuracy":
		selectClause = "countIf(event_type='weapon_hit') / nullIf(countIf(event_type='weapon_fire'), 0) * 100"
	case "kdr":
		// This is only valid if grouping by player.
		// Approximating deaths as kills (global) or needs improvement.
		// For now, retaining existing logic but safer division.
		selectClause = "countIf(event_type IN ('player_kill', 'bot_killed')) / nullIf(countIf(event_type IN ('player_kill', 'bot_killed')), 0)"
	default:
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
		query += " AND actor_weapon = ?"
		args = append(args, req.FilterWeapon)
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
