package models

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of game event
type EventType string

const (
	// ========================================
	// GAME FLOW & MATCH LIFECYCLE (11 events)
	// ========================================
	EventGameInit          EventType = "game_init"
	EventGameStart         EventType = "game_start"
	EventGameEnd           EventType = "game_end"
	EventMatchStart        EventType = "match_start"
	EventMatchEnd          EventType = "match_end"
	EventMatchOutcome      EventType = "match_outcome"
	EventRoundStart        EventType = "round_start"
	EventRoundEnd          EventType = "round_end"
	EventWarmupStart       EventType = "warmup_start"
	EventWarmupEnd         EventType = "warmup_end"
	EventIntermissionStart EventType = "intermission_start"

	// ========================================
	// COMBAT EVENTS (23 events)
	// ========================================
	// Core Combat
	EventKill       EventType = "kill"
	EventDeath      EventType = "death"
	EventDamage     EventType = "damage"
	EventPlayerPain EventType = "player_pain"
	EventHeadshot   EventType = "headshot"

	// Special Kills
	EventPlayerSuicide     EventType = "player_suicide"
	EventPlayerCrushed     EventType = "player_crushed"
	EventPlayerTelefragged EventType = "player_telefragged"
	EventPlayerRoadkill    EventType = "player_roadkill"
	EventPlayerBash        EventType = "player_bash"
	EventPlayerTeamkill    EventType = "player_teamkill"

	// Weapon Events
	EventWeaponFire       EventType = "weapon_fire"
	EventWeaponHit        EventType = "weapon_hit"
	EventWeaponChange     EventType = "weapon_change"
	EventWeaponReload     EventType = "weapon_reload"
	EventWeaponReloadDone EventType = "weapon_reload_done"
	EventWeaponReady      EventType = "weapon_ready"
	EventWeaponNoAmmo     EventType = "weapon_no_ammo"
	EventWeaponHolster    EventType = "weapon_holster"
	EventWeaponRaise      EventType = "weapon_raise"
	EventWeaponDrop       EventType = "weapon_drop"

	// Grenades
	EventGrenadeThrow   EventType = "grenade_throw"
	EventGrenadeExplode EventType = "grenade_explode"

	// ========================================
	// MOVEMENT EVENTS (10 events)
	// ========================================
	EventJump           EventType = "jump"
	EventLand           EventType = "land"
	EventCrouch         EventType = "crouch"
	EventProne          EventType = "prone"
	EventPlayerStand    EventType = "player_stand"
	EventPlayerSpawn    EventType = "player_spawn"
	EventPlayerRespawn  EventType = "player_respawn"
	EventDistance       EventType = "distance"
	EventLadderMount    EventType = "ladder_mount"
	EventLadderDismount EventType = "ladder_dismount"

	// ========================================
	// INTERACTION EVENTS (6 events)
	// ========================================
	EventPlayerUse             EventType = "player_use"
	EventPlayerUseObjectStart  EventType = "player_use_object_start"
	EventPlayerUseObjectFinish EventType = "player_use_object_finish"
	EventPlayerSpectate        EventType = "player_spectate"
	EventPlayerFreeze          EventType = "player_freeze"
	EventPlayerSay             EventType = "player_say"

	// ========================================
	// ITEM EVENTS (5 events)
	// ========================================
	EventItemPickup   EventType = "item_pickup"
	EventItemDrop     EventType = "item_drop"
	EventItemRespawn  EventType = "item_respawn"
	EventHealthPickup EventType = "health_pickup"
	EventAmmoPickup   EventType = "ammo_pickup"

	// ========================================
	// VEHICLE & TURRET EVENTS (6 events)
	// ========================================
	EventVehicleEnter     EventType = "vehicle_enter"
	EventVehicleExit      EventType = "vehicle_exit"
	EventVehicleDeath     EventType = "vehicle_death"
	EventVehicleCollision EventType = "vehicle_collision"
	EventTurretEnter      EventType = "turret_enter"
	EventTurretExit       EventType = "turret_exit"

	// ========================================
	// SERVER LIFECYCLE EVENTS (5 events)
	// ========================================
	EventServerInit           EventType = "server_init"
	EventServerStart          EventType = "server_start"
	EventServerShutdown       EventType = "server_shutdown"
	EventServerSpawned        EventType = "server_spawned"
	EventServerConsoleCommand EventType = "server_console_command"
	EventHeartbeat            EventType = "heartbeat"

	// ========================================
	// MAP LIFECYCLE EVENTS (4 events)
	// ========================================
	EventMapLoadStart   EventType = "map_load_start"
	EventMapLoadEnd     EventType = "map_load_end"
	EventMapChangeStart EventType = "map_change_start"
	EventMapRestart     EventType = "map_restart"

	// ========================================
	// TEAM & VOTE EVENTS (5 events)
	// ========================================
	EventTeamJoin   EventType = "team_join"
	EventTeamChange EventType = "team_change"
	EventVoteStart  EventType = "vote_start"
	EventVotePassed EventType = "vote_passed"
	EventVoteFailed EventType = "vote_failed"

	// ========================================
	// CLIENT/SESSION EVENTS (5 events)
	// ========================================
	EventClientConnect         EventType = "client_connect"
	EventClientDisconnect      EventType = "client_disconnect"
	EventClientBegin           EventType = "client_begin"
	EventClientUserinfoChanged EventType = "client_userinfo_changed"
	EventPlayerInactivityDrop  EventType = "player_inactivity_drop"

	// ========================================
	// WORLD EVENTS (3 events)
	// ========================================
	EventDoorOpen  EventType = "door_open"
	EventDoorClose EventType = "door_close"
	EventExplosion EventType = "explosion"

	// ========================================
	// AI/ACTOR/BOT EVENTS (7 events)
	// ========================================
	EventActorSpawn  EventType = "actor_spawn"
	EventActorKilled EventType = "actor_killed"
	EventBotSpawn    EventType = "bot_spawn"
	EventBotKilled   EventType = "bot_killed"
	EventBotRoam     EventType = "bot_roam"
	EventBotCurious  EventType = "bot_curious"
	EventBotAttack   EventType = "bot_attack"

	// ========================================
	// OBJECTIVE EVENTS (2 events)
	// ========================================
	EventObjectiveUpdate  EventType = "objective_update"
	EventObjectiveCapture EventType = "objective_capture"

	// ========================================
	// SCORE & ADMIN EVENTS (2 events)
	// ========================================
	EventScoreChange  EventType = "score_change"
	EventTeamkillKick EventType = "teamkill_kick"

	// ========================================
	// LEGACY/COMPATIBILITY (kept for backward compatibility)
	// ========================================
	EventConnect       EventType = "connect"        // Alias for client_connect
	EventDisconnect    EventType = "disconnect"     // Alias for client_disconnect
	EventSpawn         EventType = "spawn"          // Alias for player_spawn
	EventChat          EventType = "chat"           // Alias for player_say
	EventUse           EventType = "use"            // Alias for player_use
	EventReload        EventType = "reload"         // Alias for weapon_reload
	EventTeamWin       EventType = "team_win"       // Special event (not in 92)
	EventIdentityClaim EventType = "identity_claim" // Special event (not in 92)
)

// Team represents a player's team
type Team string

const (
	TeamSpectator Team = "spectator"
	TeamAllies    Team = "allies"
	TeamAxis      Team = "axis"
)

// RawEvent is the incoming event from game servers
type RawEvent struct {
	Type        EventType `json:"type"`
	MatchID     string    `json:"match_id"`
	SessionID   string    `json:"session_id"`
	ServerID    string    `json:"server_id"`
	ServerToken string    `json:"server_token"`
	Timestamp   float64   `json:"timestamp"`
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
	Inflictor     string `json:"inflictor,omitempty"`
	Damage        int    `json:"damage,omitempty"`
	AmmoRemaining int    `json:"ammo_remaining,omitempty"`

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

	// Items
	Item  string `json:"item,omitempty"`
	Count int    `json:"count,omitempty"`

	// Target info (for hits)
	TargetName   string `json:"target_name,omitempty"`
	TargetGUID   string `json:"target_guid,omitempty"`
	TargetSMFID  int64  `json:"target_smf_id,omitempty"` // SMF member ID (if authenticated)
	TargetStance string `json:"target_stance,omitempty"`

	// Team change
	OldTeam string `json:"old_team,omitempty"`
	NewTeam string `json:"new_team,omitempty"`

	// Chat
	Message string `json:"message,omitempty"`

	// Match lifecycle
	Gametype    string  `json:"gametype,omitempty"`
	Timelimit   string  `json:"timelimit,omitempty"`
	Fraglimit   string  `json:"fraglimit,omitempty"`
	Maxclients  string  `json:"maxclients,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	WinningTeam string  `json:"winning_team,omitempty"`
	AlliesScore int     `json:"allies_score,omitempty"`
	AxisScore   int     `json:"axis_score,omitempty"`
	RoundNumber int     `json:"round_number,omitempty"`
	TotalRounds int     `json:"total_rounds,omitempty"`
	PlayerCount int     `json:"player_count,omitempty"`
	ClientNum   int     `json:"client_num,omitempty"`

	// Identity claim
	Code string `json:"code,omitempty"`

	// Entity
	Entity     string `json:"entity,omitempty"`
	Projectile string `json:"projectile,omitempty"`

	// New Tracker Fields
	Objective       string `json:"objective,omitempty"`
	ObjectiveStatus string `json:"objective_status,omitempty"`
	BotID           string `json:"bot_id,omitempty"`
	Seat            string `json:"seat,omitempty"`
}

// ClickHouseEvent is the normalized event for ClickHouse storage
type ClickHouseEvent struct {
	Timestamp time.Time
	MatchID   uuid.UUID
	ServerID  string
	MapName   string
	EventType string

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
	Damage   uint32
	Hitloc   string
	Distance float32

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
type PlayerStats struct {
	PlayerID       string    `json:"player_id"`
	PlayerName     string    `json:"player_name"`
	Name           string    `json:"name"` // Alias for PHP
	TotalKills     uint64    `json:"total_kills"`
	TotalDeaths    uint64    `json:"total_deaths"`
	TotalDamage    uint64    `json:"total_damage"`
	TotalHeadshots uint64    `json:"total_headshots"`
	ShotsFired     uint64    `json:"shots_fired"`
	ShotsHit       uint64    `json:"shots_hit"`
	MatchesPlayed  uint64    `json:"matches_played"`
	MatchesWon     uint64    `json:"matches_won"`
	PlayTime       float64   `json:"play_time_seconds"`
	LastActive     time.Time `json:"last_active"`

	// Computed
	KDRatio        float64 `json:"kd_ratio"`
	Accuracy       float64 `json:"accuracy"`
	HSPercent      float64 `json:"headshot_percent"`
	WinRate        float64 `json:"win_rate"`
	KillsPerMinute float64 `json:"kills_per_minute"`

	// Granular Combat Metrics
	LongRangeKills  uint64 `json:"long_range_kills"`  // >100m
	CloseRangeKills uint64 `json:"close_range_kills"` // <5m
	WallbangKills   uint64 `json:"wallbang_kills"`
	CollateralKills uint64 `json:"collateral_kills"`

	// Stance Metrics
	KillsWhileProne      uint64 `json:"kills_while_prone"`
	KillsWhileCrouching  uint64 `json:"kills_while_crouching"`
	KillsWhileStanding   uint64 `json:"kills_while_standing"`
	KillsWhileMoving     uint64 `json:"kills_while_moving"`
	KillsWhileStationary uint64 `json:"kills_while_stationary"`

	// Movement Metrics
	TotalDistance  float64 `json:"total_distance_km"`
	SprintDistance float64 `json:"sprint_distance_km"`
	JumpCount      uint64  `json:"jump_count"`
	CrouchTime     float64 `json:"crouch_time_seconds"`
	ProneTime      float64 `json:"prone_time_seconds"`
}

// WeaponStats per-weapon statistics
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

// MapStats per-map statistics
type MapStats struct {
	MapName       string  `json:"map_name"`
	Kills         uint64  `json:"kills"`
	Deaths        uint64  `json:"deaths"`
	MatchesPlayed uint64  `json:"matches_played"`
	MatchesWon    uint64  `json:"matches_won"`
	WinRate       float64 `json:"win_rate"`
}

// GametypeStats per-gametype statistics
type GametypeStats struct {
	Gametype      string  `json:"gametype"`
	MatchesPlayed uint64  `json:"matches_played"`
	MatchesWon    uint64  `json:"matches_won"`
	MatchesLost   uint64  `json:"matches_lost"`
	WinRate       float64 `json:"win_rate"`
}

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
