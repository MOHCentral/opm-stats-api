package models

import "time"

// PlayerPredictions represents AI-driven performance forecasts
type PlayerPredictions struct {
	GUID              string             `json:"guid"`
	ExpectedKD        float64            `json:"expected_kd"`
	Trend             string             `json:"trend"` // "improving", "declining", "stable"
	Confidence        float64            `json:"confidence"`
	RecentPerformance []float64          `json:"recent_performance"`
	PredictedKills    int                `json:"predicted_kills"`
	PredictedDeaths   int                `json:"predicted_deaths"`
	RivalAnalysis     []RivalPrediction  `json:"rival_analysis"`
	LastUpdated       time.Time          `json:"last_updated"`
}

// RivalPrediction analyzes potential outcome against a specific opponent
type RivalPrediction struct {
	OpponentGUID string  `json:"opponent_guid"`
	OpponentName string  `json:"opponent_name"`
	WinProb      float64 `json:"win_prob"`
	Nemesis      bool    `json:"nemesis"`
}

// MatchPredictions forecasts the outcome of an ongoing or upcoming match
type MatchPredictions struct {
	MatchID        string            `json:"match_id"`
	AlliesWinProb  float64           `json:"allies_win_prob"`
	AxisWinProb    float64           `json:"axis_win_prob"`
	ExpectedWinner string            `json:"expected_winner"`
	KeyPlayers     []string          `json:"key_players"`
	Factors        []string          `json:"factors"`
}
