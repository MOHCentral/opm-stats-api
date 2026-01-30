package models

// LeaderboardEntry for leaderboard display with ALL stats
type LeaderboardEntry struct {
	Rank       int    `json:"rank"`
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	Value      interface{} `json:"value,omitempty"` // For AG Grid dynamic stat column

	// Combat Stats
	Kills      uint64  `json:"kills"`
	Deaths     uint64  `json:"deaths"`
	Headshots  uint64  `json:"headshots"`
	Accuracy   float64 `json:"accuracy"`
	ShotsFired uint64  `json:"shots_fired"`
	ShotsHit   uint64  `json:"shots_hit"`
	Damage     uint64  `json:"damage"`

	// Special Kills
	Suicides     uint64 `json:"suicides"`
	TeamKills    uint64 `json:"teamkills"`
	Roadkills    uint64 `json:"roadkills"`
	BashKills    uint64 `json:"bash_kills"`
	GrenadeKills uint64 `json:"grenade_kills"`
	Telefrags    uint64 `json:"telefrags"`
	Crushed      uint64 `json:"crushed"`

	// Weapon Handling
	Reloads     uint64 `json:"reloads"`
	WeaponSwaps uint64 `json:"weapon_swaps"`
	NoAmmo      uint64 `json:"no_ammo"`
	ItemsPicked uint64 `json:"looter"`

	// Movement
	Distance float64 `json:"distance_km"`
	Sprinted float64 `json:"sprinted"`
	Swam     float64 `json:"swam"`
	Driven   float64 `json:"driven"`
	Jumps    uint64  `json:"jumps"`
	Crouches uint64  `json:"crouch_time"`
	Prone    uint64  `json:"prone_time"`
	Ladders  uint64  `json:"ladders"`

	// Survival
	HealthPicked uint64 `json:"health_picked"`
	AmmoPicked   uint64 `json:"ammo_picked"`
	ArmorPicked  uint64 `json:"armor_picked"`

	// Results
	Wins           uint64 `json:"wins"`
	FFAWins        uint64 `json:"ffa_wins"`
	TeamWins       uint64 `json:"team_wins"`
	Losses         uint64 `json:"losses"`
	Rounds         uint64 `json:"rounds"`
	Objectives     uint64 `json:"objectives"`
	GamesFinished  uint64 `json:"games"`
	Playtime       uint64 `json:"playtime_seconds"`
}

type LeaderboardCard struct {
	Title  string                 `json:"title"`
	Metric string                 `json:"metric"`
	Icon   string                 `json:"icon"`
	Top    []LeaderboardCardEntry `json:"top"`
}

type LeaderboardCardEntry struct {
	PlayerID     string  `json:"player_id"`
	PlayerName   string  `json:"player_name"`
	Value        float64 `json:"value"`
	Rank         int     `json:"rank"`
	DisplayValue string  `json:"display_value,omitempty"`
}

type LeaderboardDashboard struct {
	Combat   map[string]LeaderboardCard `json:"combat"`
	GameFlow map[string]LeaderboardCard `json:"game_flow"`
	Niche    map[string]LeaderboardCard `json:"niche"`
}
