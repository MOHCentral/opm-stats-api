package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// DrilldownService provides hierarchical stat exploration
type DrilldownService struct {
	ch driver.Conn
}

func NewDrilldownService(ch driver.Conn) *DrilldownService {
	return &DrilldownService{ch: ch}
}

// DrilldownRequest specifies what to drill down on
type DrilldownRequest struct {
	Stat       string   `json:"stat"`       // kd, accuracy, winrate, kills, headshots
	Dimensions []string `json:"dimensions"` // weapon, map, time_of_day, range, stance
	GUID       string   `json:"guid"`       // Player to analyze
	Limit      int      `json:"limit"`      // Max results per dimension
}

// DrilldownResponse contains the hierarchical breakdown
type DrilldownResponse struct {
	BaseStat   string           `json:"base_stat"`
	BaseValue  float64          `json:"base_value"`
	SampleSize int64            `json:"sample_size"`
	Breakdown  []DimensionBreak `json:"breakdown"`
}

// DimensionBreak is one level of the drill-down
type DimensionBreak struct {
	Dimension string      `json:"dimension"`
	Items     []DrillItem `json:"items"`
}

// DrillItem is a single item in a dimension breakdown
type DrillItem struct {
	Label        string      `json:"label"`
	Value        float64     `json:"value"`
	SampleSize   int64       `json:"sample_size"`
	Kills        int64       `json:"kills,omitempty"`
	Deaths       int64       `json:"deaths,omitempty"`
	Delta        float64     `json:"delta"`              // Difference from base value
	DeltaPercent float64     `json:"delta_pct"`          // % difference from base
	Children     []DrillItem `json:"children,omitempty"` // Nested breakdown
}

// ValidDimensions lists all supported drill-down dimensions
var ValidDimensions = map[string]bool{
	"weapon":       true,
	"weapon_class": true,
	"map":          true,
	"time_of_day":  true,
	"day_of_week":  true,
	"range":        true, // close/medium/long
	"stance":       true, // standing/crouching/prone
	"hitloc":       true, // head/torso/limbs
	"server":       true,
	"game_mode":    true,
}

// ValidStats lists all drillable stats
var ValidStats = map[string]bool{
	"kd":        true,
	"accuracy":  true,
	"kills":     true,
	"deaths":    true,
	"headshots": true,
	"winrate":   true,
	"damage":    true,
}

// GetDrilldown performs hierarchical stat analysis
func (s *DrilldownService) GetDrilldown(ctx context.Context, req DrilldownRequest) (*DrilldownResponse, error) {
	// Validate request
	if !ValidStats[req.Stat] {
		return nil, fmt.Errorf("invalid stat: %s", req.Stat)
	}
	for _, dim := range req.Dimensions {
		if !ValidDimensions[dim] {
			return nil, fmt.Errorf("invalid dimension: %s", dim)
		}
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	resp := &DrilldownResponse{
		BaseStat: req.Stat,
	}

	// Get base value
	baseValue, sampleSize, err := s.getBaseStat(ctx, req.GUID, req.Stat)
	if err != nil {
		return nil, fmt.Errorf("base stat: %w", err)
	}
	resp.BaseValue = baseValue
	resp.SampleSize = sampleSize

	// Get breakdown for each dimension
	for _, dim := range req.Dimensions {
		breakdown, err := s.getDimensionBreakdown(ctx, req.GUID, req.Stat, dim, baseValue, req.Limit)
		if err != nil {
			continue // Skip failed dimensions
		}
		resp.Breakdown = append(resp.Breakdown, breakdown)
	}

	return resp, nil
}

func (s *DrilldownService) getBaseStat(ctx context.Context, guid string, stat string) (float64, int64, error) {
	var query string

	switch stat {
	case "kd":
		query = `
			SELECT 
				if(deaths > 0, kills/deaths, kills) as value,
				kills + deaths as sample
			FROM (
				SELECT 
					countIf(event_type = 'player_kill' AND actor_id = ?) as kills,
					countIf((event_type = 'player_kill' OR event_type = 'player_death') AND target_id = ?) as deaths
				FROM raw_events
				WHERE actor_id = ? OR target_id = ?
			)
		`
	case "accuracy":
		query = `
			SELECT 
				if(shots > 0, hits/shots * 100, 0) as value,
				shots as sample
			FROM (
				SELECT 
					countIf(event_type = 'weapon_fire' AND actor_id = ?) as shots,
					countIf(event_type = 'weapon_hit' AND actor_id = ?) as hits
				FROM raw_events
				WHERE actor_id = ?
			)
		`
	case "kills":
		query = `
			SELECT 
				count() as value,
				count() as sample
			FROM raw_events
			WHERE event_type = 'player_kill' AND actor_id = ?
		`
	case "headshots":
		query = `
			SELECT 
				count() as value,
				count() as sample
			FROM raw_events
			WHERE event_type = 'player_headshot' AND actor_id = ?
		`
	case "winrate":
		query = `
			SELECT 
				if(matches > 0, wins/matches * 100, 0) as value,
				matches as sample
			FROM (
				SELECT 
					countIf(event_type = 'team_win' AND actor_id = ?) as wins,
					uniq(match_id) as matches
				FROM raw_events
				WHERE actor_id = ?
			)
		`
	default:
		return 0, 0, fmt.Errorf("unsupported stat: %s", stat)
	}

	var value float64
	var sample int64

	// Adjust query params based on stat type
	var err error
	switch stat {
	case "kd":
		err = s.ch.QueryRow(ctx, query, guid, guid, guid, guid).Scan(&value, &sample)
	case "accuracy":
		err = s.ch.QueryRow(ctx, query, guid, guid, guid).Scan(&value, &sample)
	case "kills", "headshots":
		err = s.ch.QueryRow(ctx, query, guid).Scan(&value, &sample)
	case "winrate":
		err = s.ch.QueryRow(ctx, query, guid, guid).Scan(&value, &sample)
	}

	return value, sample, err
}

func (s *DrilldownService) getDimensionBreakdown(ctx context.Context, guid, stat, dimension string, baseValue float64, limit int) (DimensionBreak, error) {
	db := DimensionBreak{
		Dimension: dimension,
	}

	// Build dimension-specific query
	dimExpr := s.getDimensionExpression(dimension)
	statExpr := s.getStatExpression(stat, guid)

	query := fmt.Sprintf(`
		SELECT 
			%s as dim_label,
			%s as stat_value,
			count() as sample_size,
			countIf(event_type = 'player_kill' AND actor_id = ?) as kills,
			countIf((event_type = 'player_kill' OR event_type = 'player_death') AND target_id = ?) as deaths
		FROM raw_events
		WHERE (actor_id = ? OR target_id = ?) AND %s != ''
		GROUP BY dim_label
		HAVING sample_size >= 10
		ORDER BY stat_value DESC
		LIMIT ?
	`, dimExpr, statExpr, dimExpr)

	rows, err := s.ch.Query(ctx, query, guid, guid, guid, guid, limit)
	if err != nil {
		return db, err
	}
	defer rows.Close()

	for rows.Next() {
		var item DrillItem
		if err := rows.Scan(&item.Label, &item.Value, &item.SampleSize, &item.Kills, &item.Deaths); err != nil {
			continue
		}

		// Calculate deltas
		item.Delta = item.Value - baseValue
		if baseValue != 0 {
			item.DeltaPercent = (item.Delta / baseValue) * 100
		}

		db.Items = append(db.Items, item)
	}

	return db, nil
}

func (s *DrilldownService) getDimensionExpression(dim string) string {
	switch dim {
	case "weapon":
		return "extract(extra, 'weapon')"
	case "weapon_class":
		return `multiIf(
			extract(extra, 'weapon') IN ('M1 Garand', 'Kar98k', 'Springfield', 'Mosin'), 'Rifles',
			extract(extra, 'weapon') IN ('Thompson', 'MP40', 'STG44'), 'SMGs',
			extract(extra, 'weapon') IN ('Colt', 'P38', 'Webley'), 'Pistols',
			extract(extra, 'weapon') IN ('Shotgun', 'Trench Gun'), 'Shotguns',
			extract(extra, 'weapon') LIKE '%Grenade%', 'Grenades',
			extract(extra, 'weapon') IN ('BAR', 'MG42', 'Bren'), 'MGs',
			'Other'
		)`
	case "map":
		return "map_name"
	case "time_of_day":
		return `multiIf(
			toHour(timestamp) BETWEEN 6 AND 11, 'Morning (6-12)',
			toHour(timestamp) BETWEEN 12 AND 17, 'Afternoon (12-18)',
			toHour(timestamp) BETWEEN 18 AND 23, 'Evening (18-24)',
			'Night (0-6)'
		)`
	case "day_of_week":
		return `multiIf(
			toDayOfWeek(timestamp) = 1, 'Monday',
			toDayOfWeek(timestamp) = 2, 'Tuesday',
			toDayOfWeek(timestamp) = 3, 'Wednesday',
			toDayOfWeek(timestamp) = 4, 'Thursday',
			toDayOfWeek(timestamp) = 5, 'Friday',
			toDayOfWeek(timestamp) = 6, 'Saturday',
			'Sunday'
		)`
	case "range":
		return `multiIf(
			toFloat64OrZero(extract(extra, 'distance')) < 10, 'Close (<10m)',
			toFloat64OrZero(extract(extra, 'distance')) < 30, 'Medium (10-30m)',
			toFloat64OrZero(extract(extra, 'distance')) < 60, 'Long (30-60m)',
			'Extreme (>60m)'
		)`
	case "stance":
		return "extract(extra, 'stance')"
	case "hitloc":
		return "extract(extra, 'hitloc')"
	case "server":
		return "server_id"
	case "game_mode":
		return "extract(extra, 'gametype')"
	default:
		return "'unknown'"
	}
}

func (s *DrilldownService) getStatExpression(stat, guid string) string {
	// Note: These are simplified - real implementation would need proper aggregation
	switch stat {
	case "kd":
		return fmt.Sprintf(`if(
			countIf((event_type = 'player_kill' OR event_type = 'player_death') AND target_id = '%s') > 0,
			countIf(event_type = 'player_kill' AND actor_id = '%s') / 
			countIf((event_type = 'player_kill' OR event_type = 'player_death') AND target_id = '%s'),
			countIf(event_type = 'player_kill' AND actor_id = '%s')
		)`, guid, guid, guid, guid)
	case "accuracy":
		return fmt.Sprintf(`if(
			countIf(event_type = 'weapon_fire' AND actor_id = '%s') > 0,
			countIf(event_type = 'weapon_hit' AND actor_id = '%s') / 
			countIf(event_type = 'weapon_fire' AND actor_id = '%s') * 100,
			0
		)`, guid, guid, guid)
	case "kills":
		return fmt.Sprintf("countIf(event_type = 'player_kill' AND actor_id = '%s')", guid)
	case "headshots":
		return fmt.Sprintf("countIf(event_type = 'player_headshot' AND actor_id = '%s')", guid)
	case "winrate":
		return fmt.Sprintf(`if(
			uniq(match_id) > 0,
			countIf(event_type = 'team_win' AND actor_id = '%s') / uniq(match_id) * 100,
			0
		)`, guid)
	default:
		return "0"
	}
}

// GetDrilldownNested gets a second-level breakdown within a dimension
func (s *DrilldownService) GetDrilldownNested(ctx context.Context, req DrilldownRequest, parentDim, parentValue string) ([]DrillItem, error) {
	if len(req.Dimensions) == 0 {
		return nil, fmt.Errorf("no child dimension specified")
	}

	childDim := req.Dimensions[0]
	dimExpr := s.getDimensionExpression(childDim)
	parentExpr := s.getDimensionExpression(parentDim)
	statExpr := s.getStatExpression(req.Stat, req.GUID)

	query := fmt.Sprintf(`
		SELECT 
			%s as dim_label,
			%s as stat_value,
			count() as sample_size
		FROM raw_events
		WHERE (actor_id = ? OR target_id = ?) 
		  AND %s = ?
		  AND %s != ''
		GROUP BY dim_label
		HAVING sample_size >= 5
		ORDER BY stat_value DESC
		LIMIT ?
	`, dimExpr, statExpr, parentExpr, dimExpr)

	rows, err := s.ch.Query(ctx, query, req.GUID, req.GUID, parentValue, req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DrillItem
	for rows.Next() {
		var item DrillItem
		if err := rows.Scan(&item.Label, &item.Value, &item.SampleSize); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// GetStatLeaders returns top players for a specific stat+dimension combo
func (s *DrilldownService) GetStatLeaders(ctx context.Context, stat, dimension, dimensionValue string, limit int) ([]LeaderEntry, error) {
	dimExpr := s.getDimensionExpression(dimension)

	var statSelect string
	switch stat {
	case "kd":
		statSelect = "if(deaths > 0, kills/deaths, kills) as stat_value"
	case "kills":
		statSelect = "kills as stat_value"
	case "accuracy":
		statSelect = "if(shots > 0, hits/shots * 100, 0) as stat_value"
	default:
		statSelect = "kills as stat_value"
	}

	query := fmt.Sprintf(`
		SELECT 
			actor_id as player_id,
			any(actor_name) as player_name,
			countIf(event_type = 'player_kill') as kills,
			countIf(event_type = 'player_death') as deaths,
			countIf(event_type = 'weapon_fire') as shots,
			countIf(event_type = 'weapon_hit') as hits,
			%s
		FROM raw_events
		WHERE %s = ? AND actor_id != ''
		GROUP BY actor_id
		HAVING kills >= 10
		ORDER BY stat_value DESC
		LIMIT ?
	`, statSelect, dimExpr)

	rows, err := s.ch.Query(ctx, query, dimensionValue, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaders []LeaderEntry
	rank := 1
	for rows.Next() {
		var l LeaderEntry
		var kills, deaths, shots, hits int64
		if err := rows.Scan(&l.PlayerID, &l.PlayerName, &kills, &deaths, &shots, &hits, &l.Value); err != nil {
			continue
		}
		l.Rank = rank
		leaders = append(leaders, l)
		rank++
	}

	return leaders, nil
}

// LeaderEntry represents a player in a contextual leaderboard
type LeaderEntry struct {
	Rank       int     `json:"rank"`
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Value      float64 `json:"value"`
}

// GetAvailableDrilldowns returns what dimensions are available for a stat
func (s *DrilldownService) GetAvailableDrilldowns(stat string) []string {
	// All stats can be drilled by these dimensions
	common := []string{"weapon", "map", "time_of_day", "day_of_week", "server"}

	// Some stats have additional dimensions
	switch stat {
	case "kd", "kills":
		return append(common, "range", "stance", "hitloc", "weapon_class")
	case "accuracy":
		return append(common, "weapon_class")
	default:
		return common
	}
}

// FormatDrilldownPath returns a breadcrumb-style path
func FormatDrilldownPath(dimensions []string, values []string) string {
	parts := make([]string, len(dimensions))
	for i, dim := range dimensions {
		if i < len(values) {
			parts[i] = fmt.Sprintf("%s: %s", dim, values[i])
		} else {
			parts[i] = dim
		}
	}
	return strings.Join(parts, " → ")
}
