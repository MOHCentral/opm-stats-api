package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered user account
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email,omitempty" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`

	// Profile
	AvatarURL string `json:"avatar_url,omitempty" db:"avatar_url"`
	Country   string `json:"country,omitempty" db:"country"`
	Bio       string `json:"bio,omitempty" db:"bio"`

	// Flags
	IsAdmin    bool `json:"is_admin" db:"is_admin"`
	IsBanned   bool `json:"is_banned" db:"is_banned"`
	IsVerified bool `json:"is_verified" db:"is_verified"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty" db:"last_login"`
}

// UserIdentity links a User to an in-game identity (GUID)
type UserIdentity struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	PlayerGUID string    `json:"player_guid" db:"player_guid"` // In-game GUID
	PlayerName string    `json:"player_name" db:"player_name"` // Last known name
	IsPrimary  bool      `json:"is_primary" db:"is_primary"`
	VerifiedAt time.Time `json:"verified_at" db:"verified_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// PlayerAlias tracks name changes for a GUID
type PlayerAlias struct {
	ID         uuid.UUID `json:"id" db:"id"`
	PlayerGUID string    `json:"player_guid" db:"player_guid"`
	PlayerName string    `json:"player_name" db:"player_name"`
	FirstSeen  time.Time `json:"first_seen" db:"first_seen"`
	LastSeen   time.Time `json:"last_seen" db:"last_seen"`
	TimesUsed  int       `json:"times_used" db:"times_used"`
}

// IdentityClaim for linking game identity to web account
type IdentityClaim struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Code       string     `json:"code" db:"code"` // e.g., "MOH-9921"
	PlayerGUID string     `json:"player_guid,omitempty" db:"player_guid"`
	Status     string     `json:"status" db:"status"` // pending, verified, expired
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty" db:"verified_at"`
}

// DeviceAuthRequest for Device Code Flow (in-game identity verification)
type DeviceAuthRequest struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`        // Short code for user to enter
	VerificationURI string    `json:"verification_uri"` // URL to visit
	ExpiresAt       time.Time `json:"expires_at"`
	Interval        int       `json:"interval"` // Polling interval in seconds
}

// DeviceAuthState stored in Redis
type DeviceAuthState struct {
	DeviceCode string     `json:"device_code"`
	UserCode   string     `json:"user_code"`
	UserID     *uuid.UUID `json:"user_id,omitempty"` // Set when user completes auth
	PlayerGUID string     `json:"player_guid,omitempty"`
	Status     string     `json:"status"` // pending, authorized, expired
	ExpiresAt  time.Time  `json:"expires_at"`
}

// Session for authenticated API access
type Session struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"-" db:"token"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	IP        string    `json:"ip" db:"ip"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Server represents a registered game server
type Server struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Token     string    `json:"-" db:"token"` // Bearer token for auth
	TokenHash string    `json:"-" db:"token_hash"`
	OwnerID   uuid.UUID `json:"owner_id" db:"owner_id"`

	// Server info
	Address     string `json:"address" db:"address"`
	Port        int    `json:"port" db:"port"`
	Region      string `json:"region" db:"region"`
	Description string `json:"description,omitempty" db:"description"`

	// Status
	IsActive   bool       `json:"is_active" db:"is_active"`
	IsOfficial bool       `json:"is_official" db:"is_official"`
	LastSeen   *time.Time `json:"last_seen,omitempty" db:"last_seen"`

	// Stats
	TotalMatches int   `json:"total_matches" db:"total_matches"`
	TotalPlayers int64 `json:"total_players" db:"total_players"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ClanTeam for team-based tournaments
type ClanTeam struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Tag         string    `json:"tag" db:"tag"` // Short clan tag
	Description string    `json:"description,omitempty" db:"description"`
	LogoURL     string    `json:"logo_url,omitempty" db:"logo_url"`
	Country     string    `json:"country,omitempty" db:"country"`

	OwnerID   uuid.UUID `json:"owner_id" db:"owner_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TeamMember links users to teams
type TeamMember struct {
	ID       uuid.UUID `json:"id" db:"id"`
	TeamID   uuid.UUID `json:"team_id" db:"team_id"`
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	Role     string    `json:"role" db:"role"` // owner, captain, member
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}
