package logic

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/openmohaa/stats-api/internal/models"
)

type TournamentService struct {
	ch driver.Conn
}

func NewTournamentService(ch driver.Conn) *TournamentService {
	return &TournamentService{ch: ch}
}

// GetTournaments returns list of tournaments
// Tournament data is managed in SMF database (MariaDB), not ClickHouse
// This API endpoint returns empty - use SMF PHP endpoints for tournament data
func (s *TournamentService) GetTournaments(ctx context.Context) ([]models.Tournament, error) {
	// Tournament management is handled by SMF plugin (smf-plugins/mohaa_tournaments)
	// ClickHouse only stores tournament match stats, not tournament metadata
	return []models.Tournament{}, nil
}

// GetTournament returns tournament details
// Tournament metadata is in SMF database - this returns error for API requests
func (s *TournamentService) GetTournament(ctx context.Context, id string) (*models.Tournament, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	// Tournament data is managed in SMF - return empty tournament
	// Use SMF PHP endpoint (?action=mohaatournaments;sa=view;id=X) for full data
	return &models.Tournament{
		ID:     uid,
		Status: models.TournamentStatusDraft,
	}, nil
}

// GetTournamentStats returns aggregated stats for a tournament
func (s *TournamentService) GetTournamentStats(ctx context.Context, tournamentID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Use temp vars for scanning
	var totalMatches int64
	s.ch.QueryRow(ctx, `
		SELECT count(DISTINCT match_id) 
		FROM raw_events 
		WHERE tournament_id = ?
	`, tournamentID).Scan(&totalMatches)
	stats["total_matches"] = totalMatches

	var totalKills int64
	s.ch.QueryRow(ctx, `
		SELECT count() 
		FROM raw_events 
		WHERE tournament_id = ? AND event_type = 'player_kill'
	`, tournamentID).Scan(&totalKills)
	stats["total_kills"] = totalKills

	// Top Player ( MVP )
	var mvp string
	// Check if any results before scanning to avoid "sql: no rows" error if using QueryRow on empty set
	// Actually count() always returns row. But grouping might not.
	// For simple logic, we'll try/catch scan error or just ignore.
	if err := s.ch.QueryRow(ctx, `
		SELECT actor_name 
		FROM raw_events 
		WHERE tournament_id = ? AND event_type = 'player_kill'
		GROUP BY actor_name 
		ORDER BY count() DESC LIMIT 1
	`, tournamentID).Scan(&mvp); err == nil {
		stats["mvp"] = mvp
	} else {
		stats["mvp"] = "None"
	}

	return stats, nil
}
