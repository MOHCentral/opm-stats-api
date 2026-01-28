package models

// LeaderboardEntry for leaderboard display with ALL stats
type LeaderboardEntry struct {
	Rank       int    `json:"rank"`
	PlayerID   string `json:"id"`
	PlayerName string `json:"name"`

	// Combat Stats
	Kills      uint64  `json:"kills"`
	Deaths     uint64  `json:"deaths"`
	Headshots  uint64  `json:"headshots"`
	Accuracy   float64 `json:"accuracy"`
	ShotsFired uint64  `json:"shots_fired"`
	ShotsHit   uint64  `json:"shots_hit"`
	Damage     uint64  `json:"damage"`

	// Special Kills
	Suicides  uint64 `json:"suicides"`
	TeamKills uint64 `json:"teamkills"`
	Roadkills uint64 `json:"roadkills"`
	BashKills uint64 `json:"bash_kills"`
	Grenades  uint64 `json:"grenades_thrown"`

	// Game Flow
	Wins       uint64 `json:"wins"`
	FFAWins    uint64 `json:"ffa_wins"`
	TeamWins   uint64 `json:"team_wins"`
	Losses     uint64 `json:"losses"`
	Rounds     uint64 `json:"rounds"`
	Objectives uint64 `json:"objectives"`

	// Movement
	Distance float64 `json:"distance_km"`
	Jumps    uint64  `json:"jumps"`

	// Time
	Playtime uint64 `json:"playtime_seconds"`
}

type LeaderboardCard struct {
	Title  string             `json:"title"`
	Metric string             `json:"metric"`
	Icon   string             `json:"icon"`
	Top    []LeaderboardCardEntry `json:"top"`
}

type LeaderboardCardEntry struct {
	PlayerID   string  `json:"id"`
	PlayerName string  `json:"name"`
	Value      float64 `json:"value"`
	Rank       int     `json:"rank"`
	// Optional Display string if value needs formatting (e.g. "42.5%" or "10:30")
	DisplayValue string `json:"display_value,omitempty"` 
}

type LeaderboardDashboard struct {
	Combat   map[string]LeaderboardCard `json:"combat"`
	GameFlow map[string]LeaderboardCard `json:"game_flow"`
	Niche    map[string]LeaderboardCard `json:"niche"`
}
