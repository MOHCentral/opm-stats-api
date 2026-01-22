package logic

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type ServerStatsService struct {
	ch driver.Conn
}

func NewServerStatsService(ch driver.Conn) *ServerStatsService {
	return &ServerStatsService{ch: ch}
}

// GlobalActivity returns a heatmap of activity (Day of Week vs Hour of Day)
func (s *ServerStatsService) GetGlobalActivity(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			toDayOfWeek(toDateTime(timestamp)) as day_idx, -- 1=Mon, 7=Sun
			toHour(toDateTime(timestamp)) as hour,
			count() as intensity
		FROM raw_events
		WHERE timestamp >= toUnixTimestamp(now() - INTERVAL 30 DAY)
		GROUP BY day_idx, hour
		ORDER BY day_idx, hour
	`
	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	// Initialize grid? Or sparse return.
	// Frontend apexcharts heatmap expects: [{name: 'Monday', data: [{x: '00:00', y: 10}, ...]}, ...]

	// We'll return raw for now and let handler/frontend format it
	for rows.Next() {
		var day, hour int
		var intensity uint64
		if err := rows.Scan(&day, &hour, &intensity); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"day":   day,
			"hour":  hour,
			"value": intensity,
		})
	}
	return result, nil
}

// MapPopularity returns top maps by matches played
func (s *ServerStatsService) GetMapPopularity(ctx context.Context) ([]models.MapStats, error) {
	query := `
		SELECT 
			map_name,
			count(DISTINCT match_id) as matches,
			countIf(event_type='kill') as kills,
			floor(avg(duration_sec)) as avg_duration
		FROM (
			SELECT 
				match_id, 
				map_name, 
				event_type, 
				(max(timestamp) - min(timestamp)) as duration_sec
			FROM raw_events
			WHERE map_name != ''
			GROUP BY match_id, map_name, event_type
		)
		GROUP BY map_name
		ORDER BY matches DESC
		LIMIT 10
	`
	// Simplified query without subquery for speed if raw_events is huge
	// But getting duration requires match grouping.
	// Alternative:
	query = `
		SELECT 
			map_name,
			count(DISTINCT match_id) as matches,
			countIf(event_type='kill') as kills
		FROM raw_events
		WHERE map_name != ''
		GROUP BY map_name
		ORDER BY matches DESC
		LIMIT 10
	`

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.MapStats
	for rows.Next() {
		var m models.MapStats
		// Scan matches struct logic
		if err := rows.Scan(&m.MapName, &m.MatchesPlayed, &m.Kills); err != nil {
			continue
		}
		stats = append(stats, m)
	}
	return stats, nil
}

// ServerPulse represents the heartbeat of the server
type ServerPulse struct {
	LethalityRating  float64 `json:"lethality_rating"`   // Kills per minute
	LeadExchangeRate float64 `json:"lead_exchange_rate"` // Estimated lead changes per match
	TotalLeadPoured  int64   `json:"total_lead_poured"`  // Total bullets hit
	MeatGrinderMap   string  `json:"meat_grinder_map"`   // Map with most deaths/minute
	ActivePlayers    int64   `json:"active_players"`     // Currently online (approx)
}

// GetServerPulse returns high-level metrics about the server's "chaos level"
func (s *ServerStatsService) GetServerPulse(ctx context.Context) (*ServerPulse, error) {
	pulse := &ServerPulse{}

	// 1. Lethality (Kills per distinct minute of gameplay, approx)
	// We'll take total kills / total playtime hours
	if err := s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type='player_kill') / (sumIf(toFloat64OrZero(extract(extra, 'duration')), event_type='round_end') / 60 + 1) as kpm
		FROM raw_events
		WHERE timestamp >= now() - INTERVAL 24 HOUR
	`).Scan(&pulse.LethalityRating); err != nil {
		// Default to 0 if fails
		pulse.LethalityRating = 0
	}

	// 2. Total Lead Poured (all weapon hits)
	// Using a simple count for now, optimized
	s.ch.QueryRow(ctx, `
		SELECT count() FROM raw_events 
		WHERE event_type = 'weapon_hit' AND timestamp >= now() - INTERVAL 24 HOUR
	`).Scan(&pulse.TotalLeadPoured)

	// 3. Meat Grinder Map
	s.ch.QueryRow(ctx, `
		SELECT map_name 
		FROM raw_events 
		WHERE event_type = 'player_death'
		GROUP BY map_name 
		ORDER BY count() DESC 
		LIMIT 1
	`).Scan(&pulse.MeatGrinderMap)

	// 4. Active Players (unique IDs in last 15 mins)
	s.ch.QueryRow(ctx, `
		SELECT uniq(actor_id) 
		FROM raw_events 
		WHERE timestamp >= now() - INTERVAL 15 MINUTE AND actor_id != ''
	`).Scan(&pulse.ActivePlayers)

	// 5. Lead Exchange Rate - requires kill streak tracking per match
	// Set to 0 if not specifically tracked
	pulse.LeadExchangeRate = 0

	return pulse, nil
}
