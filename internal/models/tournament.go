package models

import (
	"time"

	"github.com/google/uuid"
)

// TournamentStatus represents the state of a tournament
type TournamentStatus string

const (
	TournamentStatusDraft      TournamentStatus = "draft"
	TournamentStatusOpen       TournamentStatus = "registration_open"
	TournamentStatusClosed     TournamentStatus = "registration_closed"
	TournamentStatusCheckin    TournamentStatus = "checkin"
	TournamentStatusInProgress TournamentStatus = "in_progress"
	TournamentStatusCompleted  TournamentStatus = "completed"
	TournamentStatusCancelled  TournamentStatus = "cancelled"
)

// TournamentFormat represents the bracket type
type TournamentFormat string

const (
	FormatSingleElim TournamentFormat = "single_elimination"
	FormatDoubleElim TournamentFormat = "double_elimination"
	FormatSwiss      TournamentFormat = "swiss"
	FormatRoundRobin TournamentFormat = "round_robin"
	FormatGroupStage TournamentFormat = "group_stage"
)

// BracketType for double elimination
type BracketType string

const (
	BracketUpper      BracketType = "upper"
	BracketLower      BracketType = "lower"
	BracketGrandFinal BracketType = "grand_final"
)

// MatchStatus represents the state of a tournament match
type MatchStatus string

const (
	MatchStatusPending    MatchStatus = "pending"
	MatchStatusScheduled  MatchStatus = "scheduled"
	MatchStatusInProgress MatchStatus = "in_progress"
	MatchStatusCompleted  MatchStatus = "completed"
	MatchStatusDisputed   MatchStatus = "disputed"
	MatchStatusForfeited  MatchStatus = "forfeited"
)

// Tournament represents a competitive event
type Tournament struct {
	ID          uuid.UUID        `json:"id" db:"id"`
	Name        string           `json:"name" db:"name"`
	Description string           `json:"description" db:"description"`
	Format      TournamentFormat `json:"format" db:"format"`
	Status      TournamentStatus `json:"status" db:"status"`

	// Configuration
	MaxParticipants int      `json:"max_participants" db:"max_participants"`
	MinParticipants int      `json:"min_participants" db:"min_participants"`
	TeamSize        int      `json:"team_size" db:"team_size"` // 1 for solo, 2+ for teams
	GameMode        string   `json:"game_mode" db:"game_mode"`
	MapPool         []string `json:"map_pool" db:"map_pool"`

	// Match settings (sent to game servers)
	Timelimit  int `json:"timelimit" db:"timelimit"`
	Fraglimit  int `json:"fraglimit" db:"fraglimit"`
	Roundlimit int `json:"roundlimit" db:"roundlimit"`
	BestOf     int `json:"best_of" db:"best_of"`

	// Schedule
	RegistrationStart time.Time  `json:"registration_start" db:"registration_start"`
	RegistrationEnd   time.Time  `json:"registration_end" db:"registration_end"`
	CheckinStart      time.Time  `json:"checkin_start" db:"checkin_start"`
	CheckinEnd        time.Time  `json:"checkin_end" db:"checkin_end"`
	StartTime         time.Time  `json:"start_time" db:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty" db:"end_time"`

	// Ownership
	OrganizerID uuid.UUID `json:"organizer_id" db:"organizer_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Stats
	ParticipantCount int `json:"participant_count" db:"participant_count"`
	CurrentRound     int `json:"current_round" db:"current_round"`
}

// TournamentParticipant represents a player/team in a tournament
type TournamentParticipant struct {
	ID           uuid.UUID `json:"id" db:"id"`
	TournamentID uuid.UUID `json:"tournament_id" db:"tournament_id"`

	// For solo tournaments
	UserID   *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	PlayerID string     `json:"player_id" db:"player_id"` // Game GUID

	// For team tournaments
	TeamID   *uuid.UUID `json:"team_id,omitempty" db:"team_id"`
	TeamName string     `json:"team_name,omitempty" db:"team_name"`

	// Tournament data
	Seed        int        `json:"seed" db:"seed"`
	CheckedIn   bool       `json:"checked_in" db:"checked_in"`
	CheckinTime *time.Time `json:"checkin_time,omitempty" db:"checkin_time"`
	Eliminated  bool       `json:"eliminated" db:"eliminated"`
	FinalPlace  int        `json:"final_place,omitempty" db:"final_place"`

	// Stats within tournament
	Wins   int `json:"wins" db:"wins"`
	Losses int `json:"losses" db:"losses"`
	Draws  int `json:"draws" db:"draws"`
	Points int `json:"points" db:"points"` // For Swiss/RR

	RegisteredAt time.Time `json:"registered_at" db:"registered_at"`
}

// TournamentMatch represents a match within a tournament bracket
type TournamentMatch struct {
	ID           uuid.UUID `json:"id" db:"id"`
	TournamentID uuid.UUID `json:"tournament_id" db:"tournament_id"`

	// Bracket position
	BracketType BracketType `json:"bracket_type" db:"bracket_type"`
	RoundNumber int         `json:"round_number" db:"round_number"`
	MatchNumber int         `json:"match_number" db:"match_number"`

	// Participants (nullable until seeded/advanced)
	Participant1ID *uuid.UUID `json:"participant1_id,omitempty" db:"participant1_id"`
	Participant2ID *uuid.UUID `json:"participant2_id,omitempty" db:"participant2_id"`

	// Results
	Participant1Score int        `json:"participant1_score" db:"participant1_score"`
	Participant2Score int        `json:"participant2_score" db:"participant2_score"`
	WinnerID          *uuid.UUID `json:"winner_id,omitempty" db:"winner_id"`
	LoserID           *uuid.UUID `json:"loser_id,omitempty" db:"loser_id"`

	// Bracket navigation (DAG structure)
	WinnerNextMatchID *uuid.UUID `json:"winner_next_match_id,omitempty" db:"winner_next_match_id"`
	LoserNextMatchID  *uuid.UUID `json:"loser_next_match_id,omitempty" db:"loser_next_match_id"`

	// Game server assignment
	ServerID    string `json:"server_id,omitempty" db:"server_id"`
	MatchToken  string `json:"match_token,omitempty" db:"match_token"`     // Secure token for verification
	GameMatchID string `json:"game_match_id,omitempty" db:"game_match_id"` // Links to telemetry

	// Scheduling
	ScheduledTime *time.Time  `json:"scheduled_time,omitempty" db:"scheduled_time"`
	StartedAt     *time.Time  `json:"started_at,omitempty" db:"started_at"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty" db:"completed_at"`
	Status        MatchStatus `json:"status" db:"status"`

	// Map info
	MapName    string   `json:"map_name,omitempty" db:"map_name"`
	MapVotes   []string `json:"map_votes,omitempty" db:"map_votes"`
	VetoResult string   `json:"veto_result,omitempty" db:"veto_result"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TournamentBracket represents the full bracket structure for display
type TournamentBracket struct {
	TournamentID uuid.UUID        `json:"tournament_id"`
	Format       TournamentFormat `json:"format"`

	// For single/double elimination
	UpperBracket [][]BracketMatch `json:"upper_bracket,omitempty"`
	LowerBracket [][]BracketMatch `json:"lower_bracket,omitempty"`
	GrandFinal   *BracketMatch    `json:"grand_final,omitempty"`

	// For Swiss/Round Robin
	Rounds    []SwissRound `json:"rounds,omitempty"`
	Standings []Standing   `json:"standings,omitempty"`
}

// BracketMatch is a simplified match for bracket display
type BracketMatch struct {
	ID            uuid.UUID           `json:"id"`
	RoundNumber   int                 `json:"round_number"`
	MatchNumber   int                 `json:"match_number"`
	Participant1  *BracketParticipant `json:"participant1"`
	Participant2  *BracketParticipant `json:"participant2"`
	Score1        int                 `json:"score1"`
	Score2        int                 `json:"score2"`
	WinnerID      *uuid.UUID          `json:"winner_id,omitempty"`
	Status        MatchStatus         `json:"status"`
	ScheduledTime *time.Time          `json:"scheduled_time,omitempty"`
}

type BracketParticipant struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Seed     int       `json:"seed"`
	IsWinner bool      `json:"is_winner"`
}

// SwissRound for Swiss format
type SwissRound struct {
	RoundNumber int            `json:"round_number"`
	Matches     []BracketMatch `json:"matches"`
	IsComplete  bool           `json:"is_complete"`
}

// Standing for standings display
type Standing struct {
	Rank        int                `json:"rank"`
	Participant BracketParticipant `json:"participant"`
	Wins        int                `json:"wins"`
	Losses      int                `json:"losses"`
	Draws       int                `json:"draws"`
	Points      int                `json:"points"`
	Buchholz    float64            `json:"buchholz,omitempty"` // Tiebreaker for Swiss
}

// CreateTournamentRequest for API
type CreateTournamentRequest struct {
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Format            TournamentFormat `json:"format"`
	MaxParticipants   int              `json:"max_participants"`
	MinParticipants   int              `json:"min_participants"`
	TeamSize          int              `json:"team_size"`
	GameMode          string           `json:"game_mode"`
	MapPool           []string         `json:"map_pool"`
	Timelimit         int              `json:"timelimit"`
	Fraglimit         int              `json:"fraglimit"`
	Roundlimit        int              `json:"roundlimit"`
	BestOf            int              `json:"best_of"`
	RegistrationStart time.Time        `json:"registration_start"`
	RegistrationEnd   time.Time        `json:"registration_end"`
	CheckinStart      time.Time        `json:"checkin_start"`
	CheckinEnd        time.Time        `json:"checkin_end"`
	StartTime         time.Time        `json:"start_time"`
}
