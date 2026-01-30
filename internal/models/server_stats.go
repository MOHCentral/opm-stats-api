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
	PlayerID   string  `json:"id"`
	PlayerName string  `json:"name"`
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

// ServerOverview represents a server in the list view
type ServerOverview struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Address        string    `json:"address"`
	Port           int       `json:"port"`
	DisplayName    string    `json:"display_name"` // Name:Port format
	IsOnline       bool      `json:"is_online"`
	CurrentPlayers int       `json:"current_players"`
	MaxPlayers     int       `json:"max_players"`
	CurrentMap     string    `json:"current_map"`
	Gametype       string    `json:"gametype"`
	Rank           int       `json:"rank"` // Server ranking
	TotalKills     int64     `json:"total_kills"`
	TotalMatches   int64     `json:"total_matches"`
	UniquePlayers  int64     `json:"unique_players"`
	AvgPlayers24h  float64   `json:"avg_players_24h"`
	PeakPlayers24h int       `json:"peak_players_24h"`
	UptimePercent  float64   `json:"uptime_percent"`
	LastSeen       time.Time `json:"last_seen"`
	Country        string    `json:"country"`
	Region         string    `json:"region"`
}

// ServerGlobalStats represents aggregate stats across all servers
type ServerGlobalStats struct {
	TotalServers      int     `json:"total_servers"`
	OnlineServers     int     `json:"online_servers"`
	TotalPlayersNow   int     `json:"total_players_now"`
	TotalKillsToday   int64   `json:"total_kills_today"`
	TotalMatchesToday int64   `json:"total_matches_today"`
	PeakPlayersToday  int     `json:"peak_players_today"`
	AvgPlayersNow     float64 `json:"avg_players_now"`
	TotalKillsAllTime int64   `json:"total_kills_all_time"`
}

// ServerDetail contains comprehensive server information
type ServerDetail struct {
	// Basic Info
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	IsOnline    bool   `json:"is_online"`
	IsOfficial  bool   `json:"is_official"`

	// Current Status
	CurrentPlayers int      `json:"current_players"`
	MaxPlayers     int      `json:"max_players"`
	CurrentMap     string   `json:"current_map"`
	Gametype       string   `json:"gametype"`
	PlayerList     []string `json:"player_list"`

	// Rankings
	Rank       int `json:"rank"`
	WorldRank  int `json:"world_rank"`
	RegionRank int `json:"region_rank"`

	// Lifetime Stats
	Stats ServerLifetimeStats `json:"stats"`

	// Time-based Stats
	Stats24h ServerTimeStats `json:"stats_24h"`
	Stats7d  ServerTimeStats `json:"stats_7d"`
	Stats30d ServerTimeStats `json:"stats_30d"`

	// Uptime
	Uptime ServerUptime `json:"uptime"`
}

// ServerLifetimeStats represents all-time server statistics
type ServerLifetimeStats struct {
	TotalKills       int64   `json:"total_kills"`
	TotalDeaths      int64   `json:"total_deaths"`
	TotalHeadshots   int64   `json:"total_headshots"`
	TotalMatches     int64   `json:"total_matches"`
	UniquePlayers    int64   `json:"unique_players"`
	TotalPlaytime    float64 `json:"total_playtime_hours"`
	AvgMatchDuration float64 `json:"avg_match_duration_mins"`
	FirstSeen        string  `json:"first_seen"`
	TotalDays        int     `json:"total_days"`
}

// ServerTimeStats represents time-windowed stats
type ServerTimeStats struct {
	Kills         int64   `json:"kills"`
	Matches       int64   `json:"matches"`
	UniquePlayers int64   `json:"unique_players"`
	AvgPlayers    float64 `json:"avg_players"`
	PeakPlayers   int     `json:"peak_players"`
	PeakTime      string  `json:"peak_time"`
	Playtime      float64 `json:"playtime_hours"`
}

// ServerUptime represents uptime tracking
type ServerUptime struct {
	Uptime24h  float64 `json:"uptime_24h"`
	Uptime7d   float64 `json:"uptime_7d"`
	Uptime30d  float64 `json:"uptime_30d"`
	LastOnline string  `json:"last_online"`
	LastDown   string  `json:"last_down"`
}

// PlayerHistoryPoint represents a data point for player count chart
type PlayerHistoryPoint struct {
	Timestamp string  `json:"timestamp"`
	Hour      int     `json:"hour"`
	Players   int     `json:"players"`
	Peak      int     `json:"peak"`
	Avg       float64 `json:"avg"`
}

// PeakHoursHeatmap represents activity by hour and day
type PeakHoursHeatmap struct {
	Data  [][]int  `json:"data"`  // [day][hour] = player count
	Hours []string `json:"hours"` // 0-23
	Days  []string `json:"days"`  // Mon-Sun
	Peak  PeakInfo `json:"peak"`
}

type PeakInfo struct {
	Day     string `json:"day"`
	Hour    int    `json:"hour"`
	Players int    `json:"players"`
}

// ServerTopPlayer represents a top player on a specific server
type ServerTopPlayer struct {
	Rank       int     `json:"rank"`
	GUID       string  `json:"guid"`
	Name       string  `json:"name"`
	Kills      int64   `json:"kills"`
	Deaths     int64   `json:"deaths"`
	KDRatio    float64 `json:"kd_ratio"`
	Headshots  int64   `json:"headshots"`
	HSPercent  float64 `json:"hs_percent"`
	TimePlayed float64 `json:"time_played_hours"`
	LastSeen   string  `json:"last_seen"`
	Sessions   int64   `json:"sessions"`
}

// ServerMapStats represents map usage on a server
type ServerMapStats struct {
	MapName     string  `json:"map_name"`
	Matches     int64   `json:"matches"`
	Kills       int64   `json:"kills"`
	AvgPlayers  float64 `json:"avg_players"`
	AvgDuration float64 `json:"avg_duration_mins"`
	Popularity  float64 `json:"popularity_pct"`
	LastPlayed  string  `json:"last_played"`
}

// ServerWeaponStats represents weapon usage on a server
type ServerWeaponStats struct {
	WeaponName string  `json:"weapon_name"`
	Kills      int64   `json:"kills"`
	Headshots  int64   `json:"headshots"`
	HSPercent  float64 `json:"hs_percent"`
	AvgDist    float64 `json:"avg_distance"`
	UsageRate  float64 `json:"usage_rate_pct"`
}

// ServerMatch represents a match played on the server
type ServerMatch struct {
	MatchID     string    `json:"match_id"`
	MapName     string    `json:"map_name"`
	Gametype    string    `json:"gametype"`
	PlayerCount int       `json:"player_count"`
	Duration    int       `json:"duration_mins"`
	TotalKills  int64     `json:"total_kills"`
	Winner      string    `json:"winner"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
}

// ActivityTimelinePoint represents activity at a point in time
type ActivityTimelinePoint struct {
	Timestamp   string `json:"timestamp"`
	Kills       int64  `json:"kills"`
	Deaths      int64  `json:"deaths"`
	Players     int    `json:"players"`
	MatchStarts int64  `json:"match_starts"`
}

// ServerPulse represents the heartbeat of the server
type ServerPulse struct {
	LethalityRating  float64 `json:"lethality_rating"`   // Kills per minute
	LeadExchangeRate float64 `json:"lead_exchange_rate"` // Estimated lead changes per match
	TotalLeadPoured  int64   `json:"total_lead_poured"`  // Total bullets hit
	MeatGrinderMap   string  `json:"meat_grinder_map"`   // Map with most deaths/minute
	ActivePlayers    int64   `json:"active_players"`     // Currently online (approx)
}

type ServerLiveStatusResponse struct {
	IsOnline       bool   `json:"is_online"`
	CurrentMap     string `json:"current_map"`
	CurrentPlayers int    `json:"current_players"`
	MaxPlayers     int    `json:"max_players"`
	Gametype       string `json:"gametype"`
	LastUpdate     string `json:"last_update"`
}

type ServerCountryStatsResponse struct {
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	PlayerCount int64   `json:"player_count"`
	Percentage  float64 `json:"percentage"`
}

type ServerMapRotationResponse struct {
	MapName       string  `json:"map_name"`
	RotationCount int64   `json:"rotation_count"`
	AvgDuration   float64 `json:"avg_duration_mins"`
	Popularity    float64 `json:"popularity_pct"`
}
