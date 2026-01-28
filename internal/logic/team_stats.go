package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type teamStatsService struct {
	ch driver.Conn
}

func NewTeamStatsService(ch driver.Conn) TeamStatsService {
	return &teamStatsService{ch: ch}
}



// GetFactionPerformance returns aggregated stats for Axis vs Allies
// @Summary Faction Performance Stats
// @Description Get consolidated stats for Axis vs Allies over a period
// @Tags Teams
// @Produce json
// @Param days query int false "Days to look back" default(30)
// @Success 200 {object} models.FactionStats
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/teams/performance [get]
func (s *teamStatsService) GetFactionComparison(ctx context.Context, days int) (*models.FactionStats, error) {
	if days <= 0 {
		days = 30
	}

	stats := &models.FactionStats{}

	// Query 1: Metrics (Kills, Deaths, Objectives)
	// We combine both teams into one query using countIf with conditions
	metricsQuery := `
		SELECT
			-- Axis
			countIf(event_type = 'kill' AND actor_team = 'axis') as axis_kills,
			countIf(event_type = 'kill' AND target_team = 'axis') as axis_deaths,
			countIf(event_type IN ('objective_update', 'objective_capture') AND actor_team = 'axis') as axis_objs,
			-- Allies
			countIf(event_type = 'kill' AND actor_team = 'allies') as allies_kills,
			countIf(event_type = 'kill' AND target_team = 'allies') as allies_deaths,
			countIf(event_type IN ('objective_update', 'objective_capture') AND actor_team = 'allies') as allies_objs
		FROM raw_events
		WHERE timestamp >= now() - INTERVAL ? DAY
	`
	err := s.ch.QueryRow(ctx, metricsQuery, days).Scan(
		&stats.Axis.Kills, &stats.Axis.Deaths, &stats.Axis.ObjectivesDone,
		&stats.Allies.Kills, &stats.Allies.Deaths, &stats.Allies.ObjectivesDone,
	)
	if err != nil {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	// Query 2: Wins (and derived Losses)
	// Axis Wins = Allies Losses, Allies Wins = Axis Losses
	winsQuery := `
		SELECT
			countIf(event_type = 'team_win' AND actor_team = 'axis') as axis_wins,
			countIf(event_type = 'team_win' AND actor_team = 'allies') as allies_wins
		FROM raw_events
		WHERE event_type = 'team_win'
		  AND timestamp >= now() - INTERVAL ? DAY
	`
	err = s.ch.QueryRow(ctx, winsQuery, days).Scan(&stats.Axis.Wins, &stats.Allies.Wins)
	if err != nil {
		return nil, fmt.Errorf("wins query failed: %w", err)
	}

	// Assign losses based on opponent wins
	stats.Axis.Losses = stats.Allies.Wins
	stats.Allies.Losses = stats.Axis.Wins

	// Calculate derived stats for Axis
	if stats.Axis.Deaths > 0 {
		stats.Axis.KDRatio = float64(stats.Axis.Kills) / float64(stats.Axis.Deaths)
	} else {
		stats.Axis.KDRatio = float64(stats.Axis.Kills)
	}
	axisTotalGames := stats.Axis.Wins + stats.Axis.Losses
	if axisTotalGames > 0 {
		stats.Axis.WinRate = (float64(stats.Axis.Wins) / float64(axisTotalGames)) * 100
	}

	// Calculate derived stats for Allies
	if stats.Allies.Deaths > 0 {
		stats.Allies.KDRatio = float64(stats.Allies.Kills) / float64(stats.Allies.Deaths)
	} else {
		stats.Allies.KDRatio = float64(stats.Allies.Kills)
	}
	alliesTotalGames := stats.Allies.Wins + stats.Allies.Losses
	if alliesTotalGames > 0 {
		stats.Allies.WinRate = (float64(stats.Allies.Wins) / float64(alliesTotalGames)) * 100
	}

	// Query 3: Top Weapon
	// Get top weapon for each team in one query using LIMIT 1 BY
	topWeaponQuery := `
		SELECT actor_team, actor_weapon
		FROM raw_events
		WHERE event_type = 'kill' AND actor_team IN ('axis', 'allies')
		  AND timestamp >= now() - INTERVAL ? DAY
		  AND actor_weapon != ''
		GROUP BY actor_team, actor_weapon
		ORDER BY count() DESC LIMIT 1 BY actor_team
	`
	rows, err := s.ch.Query(ctx, topWeaponQuery, days)
	if err != nil {
		return nil, fmt.Errorf("top weapon query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var team, weapon string
		if err := rows.Scan(&team, &weapon); err != nil {
			// Log error but don't fail the whole request? Or fail?
			// Current implementation behavior was to fail on error, so we should probably fail or just continue.
			// Given it's a loop, let's just continue or maybe break.
			// But sticking to fail fast is usually safer if we want to be strict.
			// However, previous implementation didn't have loop.
			// Let's just log or ignore for now, or better yet, return error.
			return nil, fmt.Errorf("failed to scan top weapon: %w", err)
		}
		if team == "axis" {
			stats.Axis.TopWeapon = weapon
		} else if team == "allies" {
			stats.Allies.TopWeapon = weapon
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("top weapon row iteration failed: %w", err)
	}

	return stats, nil
}
