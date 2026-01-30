package logic

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type matchReportService struct {
	ch driver.Conn
}

func NewMatchReportService(ch driver.Conn) MatchReportService {
	return &matchReportService{ch: ch}
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
	Info       models.LiveMatch       `json:"info"`
	Timeline   []MatchTimelineEvent   `json:"timeline"`
	Versus     map[string][]VersusRow `json:"versus"` // map[PlayerID] -> []VersusRow
	TopWeapons []models.WeaponStats   `json:"top_weapons"`
}

// GetMatchDetails fetches comprehensive match report
func (s *matchReportService) GetMatchDetails(ctx context.Context, matchID string) (*MatchDetail, error) {
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

func (s *matchReportService) getMatchInfo(ctx context.Context, matchID string) (*models.LiveMatch, error) {
	var m models.LiveMatch
	m.MatchID = matchID

	// Start/End timestamps
	query := `
		SELECT 
			any(map_name), 
			anyIf(JSONExtractString(raw_json, 'gametype'), event_type = 'match_start'), 
			dateDiff('second', min(timestamp), max(timestamp)),
			anyIf(JSONExtractString(raw_json, 'server_id'), event_type = 'match_start'),
			toInt32(maxIf(JSONExtractInt(raw_json, 'allies_score'), event_type IN ('match_end', 'heartbeat'))),
			toInt32(maxIf(JSONExtractInt(raw_json, 'axis_score'), event_type IN ('match_end', 'heartbeat'))),
			toInt32(maxIf(JSONExtractInt(raw_json, 'player_count'), event_type IN ('match_start', 'heartbeat'))),
			toInt32(anyIf(JSONExtractInt(raw_json, 'maxclients'), event_type = 'match_start')),
			min(timestamp)
		FROM mohaa_stats.raw_events
		WHERE match_id = toUUID(?)
	`
	var duration int64
	var alliesScore, axisScore, playerCount, maxPlayers int32
	if err := s.ch.QueryRow(ctx, query, matchID).Scan(
		&m.MapName, &m.Gametype, &duration, &m.ServerID, 
		&alliesScore, &axisScore, &playerCount, &maxPlayers, &m.StartedAt,
	); err != nil {
		return nil, err
	}

	m.AlliesScore = int(alliesScore)
	m.AxisScore = int(axisScore)
	m.PlayerCount = int(playerCount)
	m.MaxPlayers = int(maxPlayers)
	// m.Duration = float64(duration)

	return &m, nil
}

func (s *matchReportService) getTimeline(ctx context.Context, matchID string) ([]MatchTimelineEvent, error) {
	query := `
		SELECT 
			timestamp, 
			event_type, 
			actor_name, 
			target_name, 
			JSONExtractString(raw_json, 'weapon') as detail
		FROM mohaa_stats.raw_events
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
		var ts time.Time
		if err := rows.Scan(&ts, &t.Type, &t.Actor, &t.Target, &t.Detail); err != nil {
			continue
		}
		t.Timestamp = float64(ts.UnixNano()) / 1e9
		timeline = append(timeline, t)
	}
	return timeline, nil
}

func (s *matchReportService) getVersusMatrix(ctx context.Context, matchID string) (map[string][]VersusRow, error) {
	// Matrix: For every pair (A, B), count kills A->B and B->A
	query := `
		SELECT 
			actor_name,
			target_name,
			toInt32(count()) as kills
		FROM mohaa_stats.raw_events
		WHERE match_id = toUUID(?) AND event_type = 'kill' AND actor_name != '' AND target_name != ''
		GROUP BY actor_name, target_name
	`
	rows, err := s.ch.Query(ctx, query, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Intermediary map to hold bidirectional data: map[PlayerName][OpponentName] = VersusRow
	data := make(map[string]map[string]*VersusRow)

	for rows.Next() {
		var actor, target string
		var kills int32
		if err := rows.Scan(&actor, &target, &kills); err != nil {
			continue
		}

		// Actor killed Target
		if data[actor] == nil {
			data[actor] = make(map[string]*VersusRow)
		}
		if data[actor][target] == nil {
			data[actor][target] = &VersusRow{OpponentName: target}
		}
		data[actor][target].Kills += int(kills)

		// Target was killed by Actor (Target has a death from Actor)
		if data[target] == nil {
			data[target] = make(map[string]*VersusRow)
		}
		if data[target][actor] == nil {
			data[target][actor] = &VersusRow{OpponentName: actor}
		}
		data[target][actor].Deaths += int(kills)
	}

	// Flatten map to slices
	matrix := make(map[string][]VersusRow)
	for player, opponents := range data {
		rows := make([]VersusRow, 0, len(opponents))
		for _, row := range opponents {
			rows = append(rows, *row)
		}
		matrix[player] = rows
	}

	return matrix, nil
}
