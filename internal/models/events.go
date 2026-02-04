package models

import (
	"time"

	"github.com/google/uuid"
)

// NOTE: EventType and event constants are now in event_types_generated.go
// Edit openapi.yaml and run `make generate-types` to add new event types.

// Team represents a player's team
type Team string

const (
	TeamSpectator Team = "spectator"
	TeamAllies    Team = "allies"
	TeamAxis      Team = "axis"
)

// MatchSummary provides a summary of a match
type MatchSummary struct {
	ID          string    `json:"id"`
	Map         string    `json:"map"`
	ServerID    string    `json:"server_id"`
	ServerName  string    `json:"server_name"`
	StartTime   time.Time `json:"start_time"`
	Duration    float64   `json:"duration"`
	PlayerCount uint64    `json:"player_count"`
	Kills       uint64    `json:"kills"`
}

// RawEvent is the incoming event from game servers
type RawEvent struct {
	Type        EventType `json:"type" validate:"required"`
	MatchID     string    `json:"match_id" validate:"required"`
	SessionID   string    `json:"session_id"`
	ServerID    string    `json:"server_id" validate:"required"`
	ServerToken string    `json:"server_token"`
	Timestamp   float64   `json:"timestamp" validate:"required"`
	MapName     string    `json:"map_name,omitempty"`

	// Player info (primary actor for single-player events)
	PlayerName   string  `json:"player_name,omitempty"`
	PlayerGUID   string  `json:"player_guid,omitempty"`
	PlayerTeam   string  `json:"player_team,omitempty"`
	PlayerSMFID  int64   `json:"player_smf_id,omitempty"` // SMF member ID (if authenticated)
	PosX         float32 `json:"pos_x,omitempty"`
	PosY         float32 `json:"pos_y,omitempty"`
	PosZ         float32 `json:"pos_z,omitempty"`
	PlayerStance string  `json:"player_stance,omitempty"`

	// Attacker info (for kill/damage events)
	AttackerName   string  `json:"attacker_name,omitempty"`
	AttackerGUID   string  `json:"attacker_guid,omitempty"`
	AttackerTeam   string  `json:"attacker_team,omitempty"`
	AttackerSMFID  int64   `json:"attacker_smf_id,omitempty"` // SMF member ID (if authenticated)
	AttackerX      float32 `json:"attacker_x,omitempty"`
	AttackerY      float32 `json:"attacker_y,omitempty"`
	AttackerZ      float32 `json:"attacker_z,omitempty"`
	AttackerPitch  float32 `json:"attacker_pitch,omitempty"`
	AttackerYaw    float32 `json:"attacker_yaw,omitempty"`
	AttackerStance string  `json:"attacker_stance,omitempty"`

	// Victim info
	VictimName   string  `json:"victim_name,omitempty"`
	VictimGUID   string  `json:"victim_guid,omitempty"`
	VictimTeam   string  `json:"victim_team,omitempty"`
	VictimSMFID  int64   `json:"victim_smf_id,omitempty"` // SMF member ID (if authenticated)
	VictimX      float32 `json:"victim_x,omitempty"`
	VictimY      float32 `json:"victim_y,omitempty"`
	VictimZ      float32 `json:"victim_z,omitempty"`
	VictimStance string  `json:"victim_stance,omitempty"`

	// Weapon/damage info
	Weapon        string `json:"weapon,omitempty"`
	OldWeapon     string `json:"old_weapon,omitempty"`
	NewWeapon     string `json:"new_weapon,omitempty"`
	Hitloc        string `json:"hitloc,omitempty"`
	Mod           string `json:"mod,omitempty"`           // Means of death (MOD_PISTOL, MOD_RIFLE, etc.)
	MeansOfDeath  string `json:"means_of_death,omitempty"` // Alias for mod
	Inflictor     string `json:"inflictor,omitempty"`
	Damage        int    `json:"damage,omitempty"`
	AmmoRemaining int    `json:"ammo_remaining,omitempty"`
	AmmoType      string `json:"ammo_type,omitempty"`
	Amount        int    `json:"amount,omitempty"` // Generic amount field (ammo, health, etc.)

	// Movement
	FallHeight float32 `json:"fall_height,omitempty"`
	Walked     float32 `json:"walked,omitempty"`
	Sprinted   float32 `json:"sprinted,omitempty"`
	Swam       float32 `json:"swam,omitempty"`
	Driven     float32 `json:"driven,omitempty"`
	Distance   float32 `json:"distance,omitempty"`

	// Aim angles
	AimPitch float32 `json:"aim_pitch,omitempty"`
	AimYaw   float32 `json:"aim_yaw,omitempty"`

	// Items & Pickups
	Item           string  `json:"item,omitempty"`
	Count          int     `json:"count,omitempty"`
	HealthRestored int     `json:"health_restored,omitempty"`
	ArmorAmount    int     `json:"armor_amount,omitempty"`
	Location       string  `json:"location,omitempty"` // Generic location description

	// Target info (for hits)
	TargetName     string `json:"target_name,omitempty"`
	TargetGUID     string `json:"target_guid,omitempty"`
	TargetSMFID    int64  `json:"target_smf_id,omitempty"` // SMF member ID (if authenticated)
	TargetStance   string `json:"target_stance,omitempty"`
	TargetLocation string `json:"target_location,omitempty"` // For AI/bot target tracking

	// Team change
	OldTeam string `json:"old_team,omitempty"`
	NewTeam string `json:"new_team,omitempty"`
	Team    string `json:"team,omitempty"` // Generic team field

	// Chat
	Message  string `json:"message,omitempty"`
	TeamOnly bool   `json:"team_only,omitempty"` // True if team-only chat

	// Match lifecycle
	Gametype    string  `json:"gametype,omitempty"`
	GameType    string  `json:"game_type,omitempty"` // Alias for gametype
	Timelimit   string  `json:"timelimit,omitempty"`
	Fraglimit   string  `json:"fraglimit,omitempty"`
	Maxclients  string  `json:"maxclients,omitempty"`
	MaxPlayers  string  `json:"max_players,omitempty"` // Alias for maxclients
	Duration    float64 `json:"duration,omitempty"`
	WinningTeam string  `json:"winning_team,omitempty"`
	Winner      string  `json:"winner,omitempty"` // Alias for winning_team
	AlliesScore int     `json:"allies_score,omitempty"`
	AlliedScore int     `json:"allied_score,omitempty"` // Alias (singular)
	AxisScore   int     `json:"axis_score,omitempty"`
	RoundNumber int     `json:"round_number,omitempty"`
	TotalRounds int     `json:"total_rounds,omitempty"`
	PlayerCount int     `json:"player_count,omitempty"`
	Players     int     `json:"players,omitempty"` // Alias for player_count (heartbeat)
	ClientNum   int     `json:"client_num,omitempty"`

	// Identity claim & Auth
	Code       string `json:"code,omitempty"`
	ClaimedID  string `json:"claimed_id,omitempty"`
	SMFID      int64  `json:"smf_id,omitempty"` // Generic SMF ID field
	AuthToken  string `json:"auth_token,omitempty"`

	// Entity
	Entity     string `json:"entity,omitempty"`
	Projectile string `json:"projectile,omitempty"`

	// Actor/Bot Events
	ActorID   string `json:"actor_id,omitempty"`
	ActorType string `json:"actor_type,omitempty"`

	// Kill Events
	KillerGUID string `json:"killer_guid,omitempty"` // Alias for attacker_guid

	// Objectives
	Objective       string `json:"objective,omitempty"`
	ObjectiveID     string `json:"objective_id,omitempty"`
	ObjectiveStatus string `json:"objective_status,omitempty"`
	Status          string `json:"status,omitempty"`  // Generic status field
	Progress        int    `json:"progress,omitempty"` // Objective progress percentage
	CapturingTeam   string `json:"capturing_team,omitempty"`

	// Vehicle/Turret
	BotID string `json:"bot_id,omitempty"`
	Seat  string `json:"seat,omitempty"`

	// Door Events
	Door       string `json:"door,omitempty"`
	OpenerGUID string `json:"opener_guid,omitempty"`

	// Explosion
	Radius float32 `json:"radius,omitempty"`

	// Map Events
	FromMap  string  `json:"from_map,omitempty"`
	ToMap    string  `json:"to_map,omitempty"`
	LoadTime float64 `json:"load_time,omitempty"`

	// Connection Events
	IP       string `json:"ip,omitempty"`
	Name     string `json:"name,omitempty"`   // Generic name field
	Reason   string `json:"reason,omitempty"` // Disconnect/kick/freeze reason
	IdleTime int    `json:"idle_time,omitempty"` // Inactivity time in seconds

	// Server Info
	Version  string `json:"version,omitempty"`  // Server version
	Protocol string `json:"protocol,omitempty"` // Network protocol version

	// Server Metrics
	CPUUsage float32 `json:"cpu_usage,omitempty"`

	// Server Commands
	Command  string `json:"command,omitempty"`  // Console command
	Executor string `json:"executor,omitempty"` // Who executed the command

	// Score Events
	ScoreDelta int `json:"score_delta,omitempty"` // Score change amount
	NewScore   int `json:"new_score,omitempty"`   // New score after change
	Score      int `json:"score,omitempty"`       // Generic score field

	// Team Events
	FromTeam      string `json:"from_team,omitempty"` // Alias for old_team
	ToTeam        string `json:"to_team,omitempty"`   // Alias for new_team
	TeamkillCount int    `json:"teamkill_count,omitempty"`

	// Vehicle Events
	Vehicle       string  `json:"vehicle,omitempty"`
	FromVehicle   string  `json:"from_vehicle,omitempty"`
	ToVehicle     string  `json:"to_vehicle,omitempty"`
	Position      string  `json:"position,omitempty"` // Seat position
	DriverGUID    string  `json:"driver_guid,omitempty"`
	DestroyerGUID string  `json:"destroyer_guid,omitempty"`
	Speed         float32 `json:"speed,omitempty"`

	// Turret Events
	Turret string `json:"turret,omitempty"`

	// Vote Events
	VoteType   string `json:"vote_type,omitempty"`
	VoteTarget string `json:"vote_target,omitempty"` // Target of vote (player, map, etc.)
	YesVotes   int    `json:"yes_votes,omitempty"`
	NoVotes    int    `json:"no_votes,omitempty"`

	// Weapon Events
	AmmoCount int    `json:"ammo_count,omitempty"` // Current ammo count
	Method    string `json:"method,omitempty"`     // Suicide method, etc.

	// Object Interaction
	Object string `json:"object,omitempty"` // Object being used/interacted with

	// Accuracy Stats
	ShotsFired int     `json:"shots_fired,omitempty"`
	ShotsHit   int     `json:"shots_hit,omitempty"`
	Accuracy   float32 `json:"accuracy,omitempty"`

	// Match Outcome (1 = Win, 0 = Loss)
	MatchOutcome uint8 `json:"match_outcome,omitempty"`
}

// ClickHouseEvent is the normalized event for ClickHouse storage
type ClickHouseEvent struct {
	Timestamp time.Time
	MatchID   uuid.UUID
	ServerID  string
	MapName   string
	EventType string

	// Match Outcome
	MatchOutcome uint8

	// Actor (player performing action)
	ActorID     string
	ActorName   string
	ActorTeam   string
	ActorSMFID  int64 // SMF member ID (0 if not authenticated)
	ActorWeapon string
	ActorPosX   float32
	ActorPosY   float32
	ActorPosZ   float32
	ActorPitch  float32
	ActorYaw    float32
	ActorStance string

	// Target (recipient of action)
	TargetID     string
	TargetName   string
	TargetTeam   string
	TargetSMFID  int64 // SMF member ID (0 if not authenticated)
	TargetPosX   float32
	TargetPosY   float32
	TargetPosZ   float32
	TargetStance string

	// Metrics
	Damage      uint32
	Hitloc      string
	Distance    float32
	RoundNumber uint16

	// Raw JSON for debugging
	RawJSON string
}

// MatchResult is sent at the end of a match
type MatchResult struct {
	MatchID     string  `json:"match_id"`
	ServerID    string  `json:"server_id"`
	MapName     string  `json:"map_name"`
	Gametype    string  `json:"gametype"`
	Duration    float64 `json:"duration"`
	WinningTeam string  `json:"winning_team"`
	AlliesScore int     `json:"allies_score"`
	AxisScore   int     `json:"axis_score"`
	TotalRounds int     `json:"total_rounds"`

	// Tournament context (optional)
	TournamentID string `json:"tournament_id,omitempty"`
	BracketMatch string `json:"bracket_match,omitempty"`
}

// PlayerStats aggregated stats for a player

// WeaponStats per-weapon statistics

// MapStats per-map statistics

// GametypeStats per-gametype statistics

// LeaderboardEntry for leaderboard display with ALL stats

// HeatmapData for spatial analysis
type HeatmapData struct {
	MapName string         `json:"map_name"`
	Type    string         `json:"type,omitempty"` // "kills" or "deaths"
	Points  []HeatmapPoint `json:"points"`
}

type HeatmapPoint struct {
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Count int     `json:"count"`
}

// LiveMatch for real-time match display
type LiveMatch struct {
	MatchID      string    `json:"match_id"`
	ServerID     string    `json:"server_id"`
	ServerName   string    `json:"server_name"`
	MapName      string    `json:"map_name"`
	Gametype     string    `json:"gametype"`
	AlliesScore  int       `json:"allies_score"`
	AxisScore    int       `json:"axis_score"`
	PlayerCount  int       `json:"player_count"`
	MaxPlayers   int       `json:"max_players"`
	RoundNumber  int       `json:"round_number"`
	StartedAt    time.Time `json:"started_at"`
	TournamentID string    `json:"tournament_id,omitempty"`
}
