package models

import "time"

// ServerStatsResponse contains comprehensive server analytics
type ServerStatsResponse struct {
	ServerID      string    `json:"server_id"`
	ServerName    string    `json:"server_name,omitempty"`
	TotalMatches  uint64    `json:"total_matches"`
	TotalKills    uint64    `json:"total_kills"`
	TotalDeaths   uint64    `json:"total_deaths"`
	TotalPlaytime float64   `json:"total_playtime_seconds"`
	UniquePlayers uint64    `json:"unique_players"`
	LastActivity  time.Time `json:"last_activity"`

	// Leaders
	TopKillers  []ServerLeaderboardEntry `json:"top_killers"`
	TopKDR      []ServerLeaderboardEntry `json:"top_kdr"`
	TopPlaytime []ServerLeaderboardEntry `json:"top_playtime"`

	// Map Stats
	MapStats []ServerMapStat `json:"map_stats"`

	// Activity
	Activity []ActivityPoint `json:"activity_graph"`
}

type ServerLeaderboardEntry struct {
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Value      float64 `json:"value"` // Generic value (kills, K/D, time)
	Rank       int     `json:"rank"`
}

type ServerMapStat struct {
	MapName     string  `json:"map_name"`
	TimesPlayed uint64  `json:"times_played"`
	TotalKills  uint64  `json:"total_kills"`
	AvgDuration float64 `json:"avg_duration_seconds"`
}

type ActivityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Players   int       `json:"players"`
}
