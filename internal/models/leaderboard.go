package models

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
