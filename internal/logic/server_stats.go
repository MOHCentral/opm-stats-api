package logic

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type serverStatsService struct {
	ch driver.Conn
}

func NewServerStatsService(ch driver.Conn) ServerStatsService {
	return &serverStatsService{ch: ch}
}

// GlobalActivity returns a heatmap of activity (Day of Week vs Hour of Day)
func (s *serverStatsService) GetGlobalActivity(ctx context.Context) ([]map[string]interface{}, error) {
	// Remove time filter to show all activity data (test data may have future dates)
	query := `
		SELECT 
			toDayOfWeek(toDateTime(timestamp)) as day_idx, -- 1=Mon, 7=Sun
			toHour(toDateTime(timestamp)) as hour,
			count() as intensity
		FROM mohaa_stats.raw_events
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
		var day, hour uint8
		var intensity uint64
		if err := rows.Scan(&day, &hour, &intensity); err != nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"day":   int(day),
			"hour":  int(hour),
			"value": intensity,
		})
	}
	return result, nil
}

// MapPopularity returns top maps by matches played
func (s *serverStatsService) GetMapPopularity(ctx context.Context) ([]models.MapStats, error) {
	query := `
		SELECT 
			map_name,
			count(DISTINCT match_id) as matches,
			countIf(event_type='player_kill') as kills,
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
			countIf(event_type='player_kill') as kills
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

// GetGlobalStats returns aggregate statistics for the dashboard
func (s *serverStatsService) GetGlobalStats(ctx context.Context) (map[string]interface{}, error) {
	var totalKills, totalMatches, activePlayers, serverCount uint64

	// Total Kills from aggregated daily stats
	s.ch.QueryRow(ctx, "SELECT sum(kills) FROM mohaa_stats.player_stats_daily").Scan(&totalKills)

	// Total Matches (unique match IDs from raw events for accuracy)
	s.ch.QueryRow(ctx, "SELECT uniq(match_id) FROM mohaa_stats.raw_events").Scan(&totalMatches)

	// Active Players (last 24 hours)
	if err := s.ch.QueryRow(ctx, "SELECT uniq(player_id) FROM mohaa_stats.player_stats_daily WHERE day >= today() - 1 AND player_id != ''").Scan(&activePlayers); err != nil {
		// Fallback to all-time if no recent activity (to show something in dev)
		s.ch.QueryRow(ctx, "SELECT uniq(player_id) FROM mohaa_stats.player_stats_daily WHERE player_id != ''").Scan(&activePlayers)
	}

	// Server Count
	s.ch.QueryRow(ctx, `SELECT uniq(server_id) FROM mohaa_stats.raw_events WHERE server_id != ''`).Scan(&serverCount)

	// Average Accuracy
	var avgAccuracy float64
	s.ch.QueryRow(ctx, `
		SELECT
			sum(shots_hit) / nullif(sum(shots_fired), 0) * 100
		FROM mohaa_stats.player_stats_daily
	`).Scan(&avgAccuracy)

	// Average KD
	var avgKD float64
	s.ch.QueryRow(ctx, `
		SELECT
			sum(kills) / nullif(sum(deaths), 0)
		FROM mohaa_stats.player_stats_daily
	`).Scan(&avgKD)

	return map[string]interface{}{
		"total_kills":         totalKills,
		"total_matches":       totalMatches,
		"active_players_24h":  activePlayers,
		"server_count":        serverCount,
		"server_avg_accuracy": avgAccuracy,
		"server_avg_kd":       avgKD,
	}, nil
}

func (s *serverStatsService) GetServerPulse(ctx context.Context) (*models.ServerPulse, error) {
	pulse := &models.ServerPulse{}

	// 1. Lethality (Kills per hour in last 24h)
	// Total kills / 24 to get kills per hour average
	if err := s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type='player_kill') / 24.0 as kph
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

	// 3. Meat Grinder Map (map with most kills/deaths)
	s.ch.QueryRow(ctx, `
		SELECT map_name 
		FROM raw_events 
		WHERE event_type IN ('player_kill', 'bot_killed') AND map_name != ''
		GROUP BY map_name 
		ORDER BY count() DESC 
		LIMIT 1
	`).Scan(&pulse.MeatGrinderMap)

	// 4. Active Players (unique IDs in last 24 hours - wider window for test data)
	s.ch.QueryRow(ctx, `
		SELECT uniq(actor_id) 
		FROM raw_events 
		WHERE timestamp >= now() - INTERVAL 24 HOUR AND actor_id != '' AND actor_id != 'world'
	`).Scan(&pulse.ActivePlayers)

	// 5. Lead Exchange Rate - requires kill streak tracking per match
	// Set to 0 if not specifically tracked
	pulse.LeadExchangeRate = 0

	return pulse, nil
}
