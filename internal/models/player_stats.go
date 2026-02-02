package models

// DeepStats represents the massive aggregated stats object
type DeepStats struct {
	Combat      CombatStats         `json:"combat"`
	Weapons     []PlayerWeaponStats `json:"weapons"` // Renamed to PlayerWeaponStats to avoid conflict if needed, or keep WeaponStats
	Movement    MovementStats       `json:"movement"`
	Accuracy    AccuracyStats       `json:"accuracy"`
	Session     SessionStats        `json:"session"`
	Rivals      RivalStats          `json:"rivals"`
	Stance      StanceStats         `json:"stance"`
	Interaction InteractionStats    `json:"interaction"`
}

type RivalStats struct {
	NemesisName  string `json:"nemesis_name,omitempty"`
	NemesisKills uint64 `json:"nemesis_kills"` // How many times they killed me
	VictimName   string `json:"victim_name,omitempty"`
	VictimKills  uint64 `json:"victim_kills"` // How many times I killed them
}

type StanceStats struct {
	StandingKills       uint64  `json:"standing_kills"`
	StandingPlayerKills uint64  `json:"standing_player_kills"`
	StandingBotKills    uint64  `json:"standing_bot_kills"`
	CrouchKills         uint64  `json:"crouch_kills"`
	CrouchPlayerKills   uint64  `json:"crouch_player_kills"`
	CrouchBotKills      uint64  `json:"crouch_bot_kills"`
	ProneKills          uint64  `json:"prone_kills"`
	PronePlayerKills    uint64  `json:"prone_player_kills"`
	ProneBotKills       uint64  `json:"prone_bot_kills"`
	StandingPct         float64 `json:"standing_pct"`
	CrouchPct           float64 `json:"crouch_pct"`
	PronePct            float64 `json:"prone_pct"`
}

type CombatStats struct {
	Kills           uint64  `json:"kills"`
	PlayerKills     uint64  `json:"player_kills"`
	BotKills        uint64  `json:"bot_kills"`
	Deaths          uint64  `json:"deaths"`
	KDRatio         float64 `json:"kd_ratio"`
	Headshots       uint64  `json:"headshots"`
	HeadshotPercent float64 `json:"headshot_percent"`
	TorsoKills      uint64  `json:"torso_kills"`
	LimbKills       uint64  `json:"limb_kills"`
	MeleeKills      uint64  `json:"melee_kills"`
	Gibs            uint64  `json:"gibs"`
	Suicides        uint64  `json:"suicides"`
	TeamKills       uint64  `json:"team_kills"`
	TradingKills    uint64  `json:"trading_kills"` // Killed within 3s of tm death
	RevengeKills    uint64  `json:"revenge_kills"`
	HighestStreak   uint64  `json:"highest_streak"`
	Nutshots        uint64  `json:"nutshots"`
	Backstabs       uint64  `json:"backstabs"`
	FirstBloods     uint64  `json:"first_bloods"`
	Longshots       uint64  `json:"longshots"`
	Roadkills       uint64  `json:"roadkills"`
	BashKills       uint64  `json:"bash_kills"`
	GrenadeKills    uint64  `json:"grenade_kills"`
	GrenadesThrown  uint64  `json:"grenades_thrown"`
	DamageDealt     uint64  `json:"damage_dealt"`
	DamageTaken     uint64  `json:"damage_taken"`
}

type PlayerWeaponStats struct {
	Name        string  `json:"name"`
	Kills       uint64  `json:"kills"`
	PlayerKills uint64  `json:"player_kills"`
	BotKills    uint64  `json:"bot_kills"`
	Deaths      uint64  `json:"deaths"`
	Headshots   uint64  `json:"headshots"`
	Accuracy    float64 `json:"accuracy"`
	Shots       uint64  `json:"shots"`
	Hits        uint64  `json:"hits"`
	Damage      uint64  `json:"damage"`
}

type MovementStats struct {
	TotalDistanceKm float64 `json:"total_distance_km"`
	JumpCount       uint64  `json:"jump_count"`
	CrouchTimeSec   float64 `json:"crouch_time_sec"`
	ProneTimeSec    float64 `json:"prone_time_sec"`
	SprintTimeSec   float64 `json:"sprint_time_sec"`
}

type AccuracyStats struct {
	Overall     float64 `json:"overall"`
	HeadHitPct  float64 `json:"head_hit_pct"`
	AvgDistance float64 `json:"avg_distance"`
}

type SessionStats struct {
	PlaytimeHours float64 `json:"playtime_hours"`
	MatchesPlayed uint64  `json:"matches_played"`
	Wins          uint64  `json:"wins"`
	WinRate       float64 `json:"win_rate"`
}

type InteractionStats struct {
	ChatMessages uint64       `json:"chat_messages"`
	Pickups      []PickupStat `json:"pickups"`
	VehicleUses  uint64       `json:"vehicle_uses"`
	TurretUses   uint64       `json:"turret_uses"`
}

type PickupStat struct {
	ItemName string `json:"item_name"`
	Count    uint64 `json:"count"`
}

type GametypeStats struct {
	Gametype      string  `json:"gametype"`
	Kills         uint64  `json:"kills"`
	PlayerKills   uint64  `json:"player_kills"`
	BotKills      uint64  `json:"bot_kills"`
	Deaths        uint64  `json:"deaths"`
	Headshots     uint64  `json:"headshots"`
	MatchesPlayed uint64  `json:"matches_played"`
	KDRatio       float64 `json:"kd_ratio"`
}

type PlayerStats struct {
	GUID            string  `json:"guid"`
	Name            string  `json:"name,omitempty"`
	PlayerName      string  `json:"player_name,omitempty"` // Duplicate for legacy
	Kills           uint64  `json:"kills"`
	Deaths          uint64  `json:"deaths"`
	KDRatio         float64 `json:"kd_ratio"`
	Headshots       uint64  `json:"headshots"`
	Accuracy        float64 `json:"accuracy"`
	DamageDealt     uint64  `json:"damage_dealt"`
	DamageTaken     uint64  `json:"damage_taken"`
	Suicides        uint64  `json:"suicides"`
	TeamKills       uint64  `json:"team_kills"`
	BashKills       uint64  `json:"bash_kills"`
	TorsoKills      uint64  `json:"torso_kills"`
	LimbKills       uint64  `json:"limb_kills"`
	MatchesPlayed   uint64  `json:"matches_played"`
	MatchesWon      uint64  `json:"matches_won"`
	WinRate         float64 `json:"win_rate"`
	PlaytimeSeconds float64 `json:"playtime_seconds"`
	DistanceMeters  float64 `json:"distance_traveled"` // Note: meters
	Jumps           uint64  `json:"jumps"`
	StandingKills   uint64  `json:"standing_kills"`
	CrouchingKills  uint64  `json:"crouching_kills"`
	ProneKills      uint64  `json:"prone_kills"`

	Weapons       []PlayerWeaponStats `json:"weapons"`
	Maps          []PlayerMapStats    `json:"maps"`
	Performance   []PerformancePoint  `json:"performance"`
	RecentMatches []RecentMatch       `json:"recent_matches"`
	Achievements  []string            `json:"achievements"`
}

type PlayerStatsResponse struct {
	Player PlayerStats `json:"player"`
}

type PerformancePoint struct {
	MatchID  string  `json:"match_id"`
	Kills    uint64  `json:"kills"`
	Deaths   uint64  `json:"deaths"`
	KD       float64 `json:"kd"`
	PlayedAt int64   `json:"played_at"`
}

type RecentMatch struct {
	MatchID string `json:"match_id"`
	MapName string `json:"map_name"`
	Kills   uint64 `json:"kills"`
	Deaths  uint64 `json:"deaths"`
	Date    int64  `json:"date"`
}

type PlayerMapStats struct {
	MapName       string  `json:"map_name"`
	Kills         uint64  `json:"kills"`
	PlayerKills   uint64  `json:"player_kills"`
	BotKills      uint64  `json:"bot_kills"`
	Deaths        uint64  `json:"deaths"`
	MatchesPlayed uint64  `json:"matches_played"`
	MatchesWon    uint64  `json:"matches_won"`
	Headshots     uint64  `json:"headshots"`
	KDRatio       float64 `json:"kd_ratio"`
}

// MapStats per-map statistics (Legacy/General)
type MapStats struct {
	MapName       string  `json:"map_name"`
	Kills         uint64  `json:"kills"`
	Deaths        uint64  `json:"deaths"`
	KDRatio       float64 `json:"kd_ratio"`
	Headshots     uint64  `json:"headshots"`
	MatchesPlayed uint64  `json:"matches_played"`
}

// WeaponStats per-weapon statistics (Legacy/General)
type WeaponStats struct {
	Weapon     string  `json:"weapon"`
	Kills      uint64  `json:"kills"`
	Deaths     uint64  `json:"deaths"`
	Damage     uint64  `json:"damage"`
	Headshots  uint64  `json:"headshots"`
	ShotsFired uint64  `json:"shots_fired"`
	ShotsHit   uint64  `json:"shots_hit"`
	Accuracy   float64 `json:"accuracy"`
}
