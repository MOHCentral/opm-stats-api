package models

// =============================================================================
// PEAK PERFORMANCE - "WHEN" ANALYSIS
// =============================================================================

// PeakPerformance shows when a player performs best
type PeakPerformance struct {
	BestHour        HourStats      `json:"best_hour"`
	BestDay         DayStats       `json:"best_day"`
	BestMap         MapPeakStats   `json:"best_map"`
	BestWeapon      WeaponPeak     `json:"best_weapon"`
	HourlyBreakdown []HourStats    `json:"hourly_breakdown"`
	DailyBreakdown  []DayStats     `json:"daily_breakdown"`
	Streaks         StreakStats    `json:"streaks"`
	MostAccurateAt  string         `json:"most_accurate_at"`
	MostWinsAt      string         `json:"most_wins_at"`
	MostLossesAt    string         `json:"most_losses_at"`
	BestConditions  BestConditions `json:"best_conditions"`
}

type BestConditions struct {
	BestHourLabel      string `json:"best_hour_label"`
	BestDay            string `json:"best_day"`
	BestMap            string `json:"best_map"`
	OptimalSessionMins int    `json:"optimal_session_mins"`
}

type HourStats struct {
	Hour     int     `json:"hour"`
	Kills    int64   `json:"kills"`
	Deaths   int64   `json:"deaths"`
	KDRatio  float64 `json:"kd_ratio"`
	Accuracy float64 `json:"accuracy"`
	Wins     int64   `json:"wins"`
	Losses   int64   `json:"losses"`
}

type DayStats struct {
	DayOfWeek string  `json:"day_of_week"`
	DayNum    int     `json:"day_num"` // 0=Sunday
	Kills     int64   `json:"kills"`
	Deaths    int64   `json:"deaths"`
	KDRatio   float64 `json:"kd_ratio"`
	Accuracy  float64 `json:"accuracy"`
	Playtime  float64 `json:"playtime_hours"`
}

type MapPeakStats struct {
	MapName string  `json:"map_name"`
	Kills   int64   `json:"kills"`
	Deaths  int64   `json:"deaths"`
	KDRatio float64 `json:"kd_ratio"`
	WinRate float64 `json:"win_rate"`
}

type WeaponPeak struct {
	WeaponName string  `json:"weapon_name"`
	Kills      int64   `json:"kills"`
	Headshots  int64   `json:"headshots"`
	HSPercent  float64 `json:"hs_percent"`
	Accuracy   float64 `json:"accuracy"`
}

type StreakStats struct {
	CurrentStreak   int64 `json:"current_streak"`
	BestKillStreak  int64 `json:"best_kill_streak"`
	BestWinStreak   int64 `json:"best_win_streak"`
	WorstLossStreak int64 `json:"worst_loss_streak"`
}

// =============================================================================
// DRILL-DOWN STATS
// =============================================================================

// DrillDownRequest specifies what to drill into
type DrillDownRequest struct {
	Stat      string `json:"stat"`      // e.g., "kills", "headshots", "accuracy"
	Dimension string `json:"dimension"` // e.g., "weapon", "map", "hour", "victim", "hitloc"
	Limit     int    `json:"limit"`
}

// DrillDownResult is a breakdown of the stat
type DrillDownResult struct {
	Stat      string          `json:"stat"`
	Dimension string          `json:"dimension"`
	Total     int64           `json:"total"`
	Items     []DrillDownItem `json:"items"`
}

type DrillDownItem struct {
	Label      string  `json:"label"`
	Value      int64   `json:"value"`
	Percentage float64 `json:"percentage"`
	Sublabel   string  `json:"sublabel,omitempty"`
}

// =============================================================================
// COMBO METRICS - Cross-dimensional analysis
// =============================================================================

// ComboMetrics are creative stat combinations
type ComboMetrics struct {
	WeaponOnMap       []WeaponMapCombo  `json:"weapon_on_map"`      // Best weapon per map
	TimeOfDayWeapon   []TimeWeaponCombo `json:"time_of_day_weapon"` // Best weapon by time
	VictimPatterns    []VictimPattern   `json:"victim_patterns"`    // Who you dominate
	KillerPatterns    []KillerPattern   `json:"killer_patterns"`    // Who dominates you
	DistanceByWeapon  []DistanceWeapon  `json:"distance_by_weapon"` // Avg kill distance per weapon
	StanceByMap       []StanceMapCombo  `json:"stance_by_map"`      // Playstyle per map
	HitlocByWeapon    []HitlocWeapon    `json:"hitloc_by_weapon"`   // Accuracy zone per weapon
	WeaponProgression []WeaponProgress  `json:"weapon_progression"` // Skill improvement over time
	Signature         SignatureStats    `json:"signature"`
	MovementCombat    MovementCombat    `json:"movement_combat"`
}

type SignatureStats struct {
	PlayStyle      string  `json:"play_style"`
	ClutchRate     float64 `json:"clutch_rate"`
	FirstBloodRate float64 `json:"first_blood_rate"`
}

type MovementCombat struct {
	RunGunIndex        float64 `json:"run_gun_index"`
	BunnyHopEfficiency float64 `json:"bunny_hop_efficiency"`
}

type WeaponMapCombo struct {
	MapName    string  `json:"map_name"`
	WeaponName string  `json:"weapon_name"`
	Kills      int64   `json:"kills"`
	KDRatio    float64 `json:"kd_ratio"`
}

type TimeWeaponCombo struct {
	TimeSlot   string  `json:"time_slot"` // "Morning", "Afternoon", "Evening", "Night"
	WeaponName string  `json:"weapon_name"`
	Kills      int64   `json:"kills"`
	Accuracy   float64 `json:"accuracy"`
}

type VictimPattern struct {
	VictimName     string  `json:"victim_name"`
	Kills          int64   `json:"kills"`
	DeathsTo       int64   `json:"deaths_to"`
	Ratio          float64 `json:"ratio"`
	FavoriteWeapon string  `json:"favorite_weapon"`
}

type KillerPattern struct {
	KillerName     string `json:"killer_name"`
	DeathsTo       int64  `json:"deaths_to"`
	KillsAgainst   int64  `json:"kills_against"`
	MostUsedWeapon string `json:"most_used_weapon"`
}

type DistanceWeapon struct {
	WeaponName  string  `json:"weapon_name"`
	AvgDistance float64 `json:"avg_distance"`
	MaxDistance float64 `json:"max_distance"`
	MinDistance float64 `json:"min_distance"`
}

type StanceMapCombo struct {
	MapName     string  `json:"map_name"`
	StandingPct float64 `json:"standing_pct"`
	CrouchPct   float64 `json:"crouch_pct"`
	PronePct    float64 `json:"prone_pct"`
}

type HitlocWeapon struct {
	WeaponName string  `json:"weapon_name"`
	HeadPct    float64 `json:"head_pct"`
	TorsoPct   float64 `json:"torso_pct"`
	LimbPct    float64 `json:"limb_pct"`
}

type WeaponProgress struct {
	WeaponName string  `json:"weapon_name"`
	Month      string  `json:"month"`
	Kills      int64   `json:"kills"`
	Accuracy   float64 `json:"accuracy"`
}

// =============================================================================
// VEHICLE & TURRET STATS
// =============================================================================

// VehicleStats represents vehicle-related statistics
type VehicleStats struct {
	VehicleUses   int64         `json:"vehicle_uses"`
	VehicleKills  int64         `json:"vehicle_kills"`
	VehicleDeaths int64         `json:"vehicle_deaths"`
	TotalDriven   float64       `json:"total_driven_km"`
	VehicleTypes  []VehicleType `json:"vehicle_types"`
	TurretStats   TurretStats   `json:"turret_stats"`
}

type VehicleType struct {
	VehicleName string  `json:"vehicle_name"`
	Uses        int64   `json:"uses"`
	Kills       int64   `json:"kills"`
	Deaths      int64   `json:"deaths"`
	DistanceKm  float64 `json:"distance_km"`
}

type TurretStats struct {
	TurretUses   int64 `json:"turret_uses"`
	TurretKills  int64 `json:"turret_kills"`
	TurretDeaths int64 `json:"turret_deaths"`
}

// =============================================================================
// GAME FLOW STATS
// =============================================================================

// GameFlowStats represents round/objective/team statistics
type GameFlowStats struct {
	RoundsPlayed     int64           `json:"rounds_played"`
	RoundsWon        int64           `json:"rounds_won"`
	RoundsLost       int64           `json:"rounds_lost"`
	RoundWinRate     float64         `json:"round_win_rate"`
	ObjectivesTotal  int64           `json:"objectives_total"`
	ObjectivesByType []ObjectiveStat `json:"objectives_by_type"`
	FirstBloods      int64           `json:"first_bloods"`
	ClutchWins       int64           `json:"clutch_wins"`
	TeamStats        TeamStats       `json:"team_stats"`
}

type ObjectiveStat struct {
	ObjectiveType string `json:"objective_type"`
	Count         int64  `json:"count"`
}

type TeamStats struct {
	AlliesPlaytime float64 `json:"allies_playtime_pct"`
	AxisPlaytime   float64 `json:"axis_playtime_pct"`
	AlliesWins     int64   `json:"allies_wins"`
	AxisWins       int64   `json:"axis_wins"`
}

// =============================================================================
// WORLD INTERACTION STATS
// =============================================================================

// WorldStats represents world interaction statistics
type WorldStats struct {
	LadderMounts    int64   `json:"ladder_mounts"`
	LadderDistance  float64 `json:"ladder_distance"`
	DoorsOpened     int64   `json:"doors_opened"`
	DoorsClosed     int64   `json:"doors_closed"`
	ItemsPickedUp   int64   `json:"items_picked_up"`
	ItemsDropped    int64   `json:"items_dropped"`
	UseInteractions int64   `json:"use_interactions"`
	ChatMessages    int64   `json:"chat_messages"`
	FallDamage      int64   `json:"fall_damage"`
	FallDeaths      int64   `json:"fall_deaths"`
}

// =============================================================================
// BOT STATS
// =============================================================================

// BotStats represents bot-related statistics
type BotStats struct {
	BotKills       int64         `json:"bot_kills"`
	DeathsToBots   int64         `json:"deaths_to_bots"`
	BotKDRatio     float64       `json:"bot_kd_ratio"`
	BotsByType     []BotTypeStat `json:"bots_by_type"`
	AvgBotKillDist float64       `json:"avg_bot_kill_distance"`
}

type BotTypeStat struct {
	BotType string `json:"bot_type"`
	Kills   int64  `json:"kills"`
	Deaths  int64  `json:"deaths"`
}

type LeaderboardEntry struct {
	Rank       int     `json:"rank"`
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Value      float64 `json:"value"`
	Secondary  float64 `json:"secondary,omitempty"`
}

type WarRoomDataResponse struct {
	DeepStats       *DeepStats       `json:"deep_stats,omitempty"`
	PeakPerformance *PeakPerformance `json:"peak_performance,omitempty"`
	ComboMetrics    *ComboMetrics    `json:"combo_metrics,omitempty"`
	KDDrilldown     *DrillDownResult `json:"kd_drilldown,omitempty"`
	Playstyle       *PlaystyleBadge  `json:"playstyle,omitempty"`
}

type PlaystyleBadge struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type DrillDownNestedResponse struct {
	ParentDimension string          `json:"parent_dimension"`
	ParentValue     string          `json:"parent_value"`
	ChildDimension  string          `json:"child_dimension"`
	Items           []DrillDownItem `json:"items"`
}

type DrilldownOptionsResponse struct {
	Stat       string   `json:"stat"`
	Dimensions []string `json:"dimensions"`
}

type ContextualLeaderboardResponse struct {
	Stat      string             `json:"stat"`
	Dimension string             `json:"dimension"`
	Value     string             `json:"value"`
	Leaders   []LeaderboardEntry `json:"leaders"`
}

type ComboLeaderboardResponse struct {
	Metric  string             `json:"metric"`
	Entries []LeaderboardEntry `json:"entries"`
}

type PeakLeaderboardResponse struct {
	Dimension string                   `json:"dimension"`
	Entries   []PeakLeaderboardEntry `json:"entries"`
}

type PeakLeaderboardEntry struct {
	Rank       int     `json:"rank"`
	PlayerID   string  `json:"player_id"`
	PlayerName string  `json:"player_name"`
	Kills      int64   `json:"kills"`
	Deaths     int64   `json:"deaths"`
	KD         float64 `json:"kd"`
}
