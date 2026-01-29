package logic

import (
	"context"
	"math"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type predictionService struct {
	ch driver.Conn
}

func NewPredictionService(ch driver.Conn) PredictionService {
	return &predictionService{ch: ch}
}

func (s *predictionService) GetPlayerPredictions(ctx context.Context, guid string) (*models.PlayerPredictions, error) {
	pred := &models.PlayerPredictions{
		GUID:        guid,
		LastUpdated: time.Now(),
	}

	// Fetch recent performance (last 10 matches)
	rows, err := s.ch.Query(ctx, `
		SELECT 
			kills / nullIf(deaths, 0) as kd
		FROM mohaa_stats.raw_events
		WHERE actor_id = ? AND event_type = 'kill'
		GROUP BY match_id, kills, deaths, timestamp
		ORDER BY max(timestamp) DESC
		LIMIT 10
	`, guid)
	if err == nil {
		defer rows.Close()
		var sumKD float64
		for rows.Next() {
			var kd float64
			if err := rows.Scan(&kd); err == nil {
				pred.RecentPerformance = append(pred.RecentPerformance, kd)
				sumKD += kd
			}
		}

		if len(pred.RecentPerformance) > 0 {
			pred.ExpectedKD = sumKD / float64(len(pred.RecentPerformance))
			
			// Simple trend analysis
			if len(pred.RecentPerformance) >= 3 {
				latest := pred.RecentPerformance[0]
				avg := pred.ExpectedKD
				if latest > avg*1.1 {
					pred.Trend = "improving"
				} else if latest < avg*0.9 {
					pred.Trend = "declining"
				} else {
					pred.Trend = "stable"
				}
			}
		}
	}

	// Predicted stats for next match (simple heuristic)
	pred.PredictedKills = int(math.Max(10, pred.ExpectedKD*15))
	pred.PredictedDeaths = 15
	pred.Confidence = 0.75 // Default visibility

	// Rival Analysis
	rivalRows, err := s.ch.Query(ctx, `
		SELECT 
			target_id, 
			any(target_name),
			count() as kills
		FROM mohaa_stats.raw_events
		WHERE actor_id = ? AND event_type = 'kill' AND target_id != ''
		GROUP BY target_id
		ORDER BY kills DESC
		LIMIT 3
	`, guid)
	if err == nil {
		defer rivalRows.Close()
		for rivalRows.Next() {
			var r models.RivalPrediction
			var kills int
			if err := rivalRows.Scan(&r.OpponentGUID, &r.OpponentName, &kills); err == nil {
				r.WinProb = 0.5 + (0.05 * float64(kills))
				if r.WinProb > 0.95 { r.WinProb = 0.95 }
				r.Nemesis = kills > 20
				pred.RivalAnalysis = append(pred.RivalAnalysis, r)
			}
		}
	}

	return pred, nil
}

func (s *predictionService) GetMatchPredictions(ctx context.Context, matchID string) (*models.MatchPredictions, error) {
	// For upcoming matches, matchID might be a placeholder or lobby ID
	// For this MVP, we provide a placeholder response
	return &models.MatchPredictions{
		MatchID:        matchID,
		AlliesWinProb:  55.5,
		AxisWinProb:    44.5,
		ExpectedWinner: "allies",
		KeyPlayers:     []string{"KillerOne", "SniperTwo"},
		Factors:        []string{"Map familiarity (mohdm1)", "Recent team chemistry"},
	}, nil
}
