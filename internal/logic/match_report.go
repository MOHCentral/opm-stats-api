package logic

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type MatchReportService struct {
	ch driver.Conn
}

func NewMatchReportService(ch driver.Conn) *MatchReportService {
	return &MatchReportService{ch: ch}
}

type MatchTimelineEvent struct {
	Timestamp float64 `json:"timestamp"`
	Type      string  `json:"type"`
	Actor     string  `json:"actor"`
	Target    string  `json:"target,omitempty"`
	Detail    string  `json:"detail,omitempty"` // Weapon, Item, etc
}

type VersusRow struct {
	OpponentName string `json:"opponent_name"`
	Kills        int    `json:"kills"`
	Deaths       int    `json:"deaths"`
}

type MatchDetail struct {
	Info       models.LiveMatch     `json:"info"`
	Timeline   []MatchTimelineEvent `json:"timeline"`
	Versus     map[string][]VersusRow `json:"versus"` // map[PlayerID] -> []VersusRow
	TopWeapons []models.WeaponStats `json:"top_weapons"`
}

// GetMatchDetails fetches comprehensive match report
func (s *MatchReportService) GetMatchDetails(ctx context.Context, matchID string) (*MatchDetail, error) {
	// 1. Basic Info
	info, err := s.getMatchInfo(ctx, matchID)
	if err != nil {
		return nil, err
	}

	// 2. Timeline
	timeline, err := s.getTimeline(ctx, matchID)
	if err != nil {
		// Log error but continue?
	}

	// 3. Versus Matrix (Who killed who)
	versus, err := s.getVersusMatrix(ctx, matchID)
	if err != nil {
		// Log error
	}

	return &MatchDetail{
		Info:     *info,
		Timeline: timeline,
		Versus:   versus,
	}, nil
}

func (s *MatchReportService) getMatchInfo(ctx context.Context, matchID string) (*models.LiveMatch, error) {
	var m models.LiveMatch
	m.MatchID = matchID
	
	// Start/End timestamps
	query := `
		SELECT 
			any(map_name), 
			any(gametype), 
			toInt64(max(timestamp) - min(timestamp))
		FROM raw_events
		WHERE match_id = toUUID(?)
	`
	var duration int64
	if err := s.ch.QueryRow(ctx, query, matchID).Scan(&m.MapName, &m.Gametype, &duration); err != nil {
		// If fails, just return partial
		return &m, nil
	}
	// duration is seconds, m.Duration is float64 usually
	// m.Duration = float64(duration) 
	
	return &m, nil
}

func (s *MatchReportService) getTimeline(ctx context.Context, matchID string) ([]MatchTimelineEvent, error) {
	query := `
		SELECT 
			timestamp, 
			event_type, 
			actor_name, 
			target_name, 
			extract(extra, 'weapon') as detail
		FROM raw_events
		WHERE match_id = toUUID(?) AND event_type IN ('kill', 'flag_capture', 'match_start', 'match_end')
		ORDER BY timestamp ASC
		LIMIT 500
	`
	rows, err := s.ch.Query(ctx, query, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timeline []MatchTimelineEvent
	for rows.Next() {
		var t MatchTimelineEvent
		if err := rows.Scan(&t.Timestamp, &t.Type, &t.Actor, &t.Target, &t.Detail); err != nil {
			continue
		}
		timeline = append(timeline, t)
	}
	return timeline, nil
}

func (s *MatchReportService) getVersusMatrix(ctx context.Context, matchID string) (map[string][]VersusRow, error) {
	// Matrix: For every pair (A, B), count kills A->B and B->A
	query := `
		SELECT 
			actor_name,
			target_name,
			count() as kills
		FROM raw_events
		WHERE match_id = toUUID(?) AND event_type = 'kill' AND actor_name != '' AND target_name != ''
		GROUP BY actor_name, target_name
	`
	rows, err := s.ch.Query(ctx, query, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Organize by Actor -> [List of Victims]
	// Ideally checking logical matrix
	matrix := make(map[string][]VersusRow)
	
	type record struct {
		Actor string
		Target string
		Kills int
	}
	var records []record

	for rows.Next() {
		var r record
		rows.Scan(&r.Actor, &r.Target, &r.Kills)
		records = append(records, r)
	}

	// Transform to struct expected by frontend
	// ... logic to aggregate ...
	// Simple pass: key by Actor
	for _, r := range records {
		matrix[r.Actor] = append(matrix[r.Actor], VersusRow{OpponentName: r.Target, Kills: r.Kills, Deaths: 0})
	}
	
	return matrix, nil
}
