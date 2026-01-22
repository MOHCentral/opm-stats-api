package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type TeamStatsService struct {
	ch driver.Conn
}

func NewTeamStatsService(ch driver.Conn) *TeamStatsService {
	return &TeamStatsService{ch: ch}
}

// FactionStats comparison
type FactionStats struct {
	Axis   TeamMetrics `json:"axis"`
	Allies TeamMetrics `json:"allies"`
}

type TeamMetrics struct {
	Kills          uint64  `json:"kills"`
	Deaths         uint64  `json:"deaths"`
	Wins           uint64  `json:"wins"`
	Losses         uint64  `json:"losses"`
	KDRatio        float64 `json:"kd_ratio"`
	WinRate        float64 `json:"win_rate"`
	ObjectivesDone uint64  `json:"objectives_done"`
	TopWeapon      string  `json:"top_weapon"`
}

// GetFactionComparison returns aggregated stats for Axis vs Allies over the last N days
func (s *TeamStatsService) GetFactionComparison(ctx context.Context, days int) (*FactionStats, error) {
	if days <= 0 {
		days = 30
	}

	// Calculate stats for both teams
	getMetrics := func(team string) (TeamMetrics, error) {
		var m TeamMetrics
		// Kills, Deaths, Objectives
		query := `
			SELECT 
				countIf(event_type = 'player_kill' AND actor_team = ?) as kills,
				countIf(event_type = 'player_death' AND actor_team = ?) as deaths,
				countIf(event_type = 'objective_complete' AND actor_team = ?) as objs
			FROM raw_events
			WHERE timestamp >= now() - INTERVAL ? DAY
		`
		err := s.ch.QueryRow(ctx, query, team, team, team, days).Scan(&m.Kills, &m.Deaths, &m.ObjectivesDone)
		if err != nil {
			return m, err
		}

		// Wins (event_type = 'round_end' with winning_team = ?)
		// Assuming round_end has winning_team field extracted or in extra?
		// Usually round_end events might just say "winning_team": "axis"
		// Using raw count for now assuming explicit event
		winQuery := `
			SELECT count() 
			FROM raw_events 
			WHERE event_type = 'team_win' AND actor_team = ? 
			  AND timestamp >= now() - INTERVAL ? DAY
		`
		s.ch.QueryRow(ctx, winQuery, team, days).Scan(&m.Wins)

		// Losses (team_win for other team)
		otherTeam := "allies"
		if team == "allies" {
			otherTeam = "axis"
		}

		lossQuery := `
			SELECT count() 
			FROM raw_events 
			WHERE event_type = 'team_win' AND actor_team = ? 
			  AND timestamp >= now() - INTERVAL ? DAY
		`
		s.ch.QueryRow(ctx, lossQuery, otherTeam, days).Scan(&m.Losses)

		// Derived
		if m.Deaths > 0 {
			m.KDRatio = float64(m.Kills) / float64(m.Deaths)
		} else {
			m.KDRatio = float64(m.Kills)
		}
		totalGames := m.Wins + m.Losses
		if totalGames > 0 {
			m.WinRate = (float64(m.Wins) / float64(totalGames)) * 100
		}

		// Top Weapon
		s.ch.QueryRow(ctx, `
			SELECT actor_weapon 
			FROM raw_events 
			WHERE event_type = 'player_kill' AND actor_team = ? 
			  AND timestamp >= now() - INTERVAL ? DAY
			GROUP BY actor_weapon 
			ORDER BY count() DESC LIMIT 1
		`, team, days).Scan(&m.TopWeapon)

		return m, nil
	}

	axis, err := getMetrics("axis")
	if err != nil {
		return nil, fmt.Errorf("axis stats failed: %w", err)
	}

	allies, err := getMetrics("allies")
	if err != nil {
		return nil, fmt.Errorf("allies stats failed: %w", err)
	}

	return &FactionStats{
		Axis:   axis,
		Allies: allies,
	}, nil
}
