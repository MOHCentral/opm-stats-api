package models

import (
	"time"

	"github.com/google/uuid"
)

// Achievement definition
type Achievement struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	Category    string `json:"category" db:"category"`
	IconURL     string `json:"icon_url" db:"icon_url"`
	Tier        int    `json:"tier" db:"tier"`
	Points      int    `json:"points" db:"points"`

	// Unlock criteria
	EventType string `json:"event_type" db:"event_type"`
	Threshold int    `json:"threshold" db:"threshold"`
	Condition string `json:"condition,omitempty" db:"condition"`

	// Rarity
	UnlockCount uint64  `json:"unlock_count" db:"unlock_count"`
	UnlockRate  float64 `json:"unlock_rate" db:"unlock_rate"`

	IsHidden  bool      `json:"is_hidden" db:"is_hidden"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Computed/Transient
	Progress uint64 `json:"progress,omitempty" db:"-"`
	Target   uint64 `json:"target,omitempty" db:"-"`
}

// AchievementCategory groups achievements
type AchievementCategory string

const (
	CategoryFirstSteps  AchievementCategory = "First Steps"
	CategoryCombat      AchievementCategory = "Combat Novice"
	CategoryWeapon      AchievementCategory = "Weapon Specialist"
	CategoryTactical    AchievementCategory = "Tactical Excellence"
	CategoryHumiliation AchievementCategory = "Humiliation"
	CategoryHallOfShame AchievementCategory = "Hall of Shame"
	CategoryDedication  AchievementCategory = "Dedication & Milestones"
	CategoryHidden      AchievementCategory = "Hidden & Secret"
	CategoryTournament  AchievementCategory = "Tournament"
	CategoryCommunity   AchievementCategory = "Special & Community"

	// New Categories
	CategoryMovement  AchievementCategory = "Movement & Agility"
	CategoryVehicles  AchievementCategory = "Vehicle Warfare"
	CategorySocial    AchievementCategory = "Social Butterfly"
	CategoryObjective AchievementCategory = "Objective & Teamwork"
)

// AchievementDefinition holds the static data for an achievement
type AchievementDefinition struct {
	ID          string
	Name        string
	Description string
	Tier        int
	Category    AchievementCategory
	Target      uint64 // Value needed to unlock (e.g., 100 kills)
	Metric      string // The metric name to check against (e.g., "kills", "headshots")
}

// PlayerAchievement tracks unlocked achievements
type PlayerAchievement struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	PlayerGUID    string     `json:"player_guid" db:"player_guid"`
	AchievementID string     `json:"achievement_id" db:"achievement_id"`
	UnlockedAt    time.Time  `json:"unlocked_at" db:"unlocked_at"`
	MatchID       string     `json:"match_id,omitempty" db:"match_id"`
	NotifiedAt    *time.Time `json:"notified_at,omitempty" db:"notified_at"`

	// Join fields
	Achievement *Achievement `json:"achievement,omitempty" db:"-"`
}

// NOTE: Achievement definitions are stored in SMF MariaDB (smf_mohaa_achievement_defs table)
// The Go API does NOT manage achievement definitions - PHP/SMF is the source of truth.
// This API only provides player stats that PHP uses to calculate achievement progress.
