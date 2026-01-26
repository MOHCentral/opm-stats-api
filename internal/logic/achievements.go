package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type achievementsService struct {
	ch driver.Conn
}

func NewAchievementsService(ch driver.Conn) AchievementsService {
	return &achievementsService{ch: ch}
}

type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Tier        string `json:"tier"` // "gold", "silver", "bronze"
	IsUnlocked  bool   `json:"is_unlocked"`
	Progress    int    `json:"progress,omitempty"`
	MaxProgress int    `json:"max_progress,omitempty"`
}

type AchievementScope string

const (
	ScopeMatch      AchievementScope = "match"
	ScopeTournament AchievementScope = "tournament"
	ScopeGlobal     AchievementScope = "global"
)

// GetAchievements calculates achievements for a specific scope (match, tournament, etc.)
// contextID is the match_id or tournament_id
func (s *achievementsService) GetAchievements(ctx context.Context, scope AchievementScope, contextID string, playerID string) ([]Achievement, error) {
	switch scope {
	case ScopeMatch:
		return s.getMatchAchievements(ctx, contextID, playerID)
	case ScopeTournament:
		return s.getTournamentAchievements(ctx, contextID, playerID)
	default:
		return nil, fmt.Errorf("unsupported scope: %s", scope)
	}
}

func (s *achievementsService) getMatchAchievements(ctx context.Context, matchID, playerID string) ([]Achievement, error) {
	list := []Achievement{}

	// 1. Fetch Stats for this match
	var (
		kills, deaths, shotsFired, shotsHit float64
		win                                 int
	)

	// Updated to use correct event types and logic
	query := `
		SELECT 
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			countIf(event_type = 'weapon_fire' AND actor_id = ?) as shots,
			countIf(event_type = 'weapon_hit' AND actor_id = ?) as hits,
			countIf(event_type = 'match_outcome' AND match_outcome = 1 AND actor_id = ?) as wins
		FROM raw_events 
		WHERE match_id = ? AND (actor_id = ? OR target_id = ?)
	`

	if err := s.ch.QueryRow(ctx, query,
		playerID, playerID, playerID, playerID, playerID,
		matchID, playerID, playerID,
	).Scan(&kills, &deaths, &shotsFired, &shotsHit, &win); err != nil {
		return list, nil
	}

	// ------------------------------------------------------------------
	// A. "Untouchable" (Gold): 0 deaths, min 10 kills
	// ------------------------------------------------------------------
	untouchable := Achievement{
		ID: "match_untouchable", Name: "Untouchable", Description: "Finish a match with 0 deaths (min 10 kills)",
		Icon: "shield", Tier: "gold", MaxProgress: 1, IsUnlocked: false,
	}
	if deaths == 0 && kills >= 10 {
		untouchable.IsUnlocked = true
		untouchable.Progress = 1
	}
	list = append(list, untouchable)

	// ------------------------------------------------------------------
	// B. "Pacifist" (Silver): 0 kills, > 0 shots fired (tried but failed?) or just played
	// Actually typical pacifist is 0 stats. Let's say check time played?
	// For now: 0 kills, >= 1 death (participated) or shots > 0
	// ------------------------------------------------------------------
	pacifist := Achievement{
		ID: "match_pacifist", Name: "Pacifist", Description: "Finish a match with 0 kills",
		Icon: "dove", Tier: "silver", MaxProgress: 1, IsUnlocked: false,
	}
	if kills == 0 && (deaths > 0 || shotsFired > 0) {
		pacifist.IsUnlocked = true
		pacifist.Progress = 1
	}
	list = append(list, pacifist)

	// ------------------------------------------------------------------
	// C. "Sharpshooter" (Silver): Accuracy > 50% (min 10 shots)
	// ------------------------------------------------------------------
	sharpshooter := Achievement{
		ID: "match_sharpshooter", Name: "Sharpshooter", Description: "Achieve > 50% accuracy (min 10 shots)",
		Icon: "crosshair", Tier: "silver", MaxProgress: 100, IsUnlocked: false,
	}
	if shotsFired >= 10 {
		acc := (shotsHit / shotsFired) * 100
		sharpshooter.Progress = int(acc)
		if acc > 50 {
			sharpshooter.IsUnlocked = true
		}
	}
	list = append(list, sharpshooter)

	// "Wipeout" (Gold): Kill entire enemy team in one round
	// Requires time-window logic with round boundaries - implementation pending round event tracking

	return list, nil
}

func (s *achievementsService) getTournamentAchievements(ctx context.Context, tournamentID, playerID string) ([]Achievement, error) {
	list := []Achievement{}

	// Query tournament aggregated stats
	var (
		wins, matches int
	)

	// Get total wins and matches played in this tournament
	// Using match_outcome for wins
	query := `
		SELECT 
			countIf(event_type = 'match_outcome' AND match_outcome = 1) as wins,
			uniq(match_id) as matches
		FROM raw_events 
		WHERE tournament_id = ? AND actor_id = ?
	`
	if err := s.ch.QueryRow(ctx, query, tournamentID, playerID).Scan(&wins, &matches); err != nil {
		return list, nil
	}

	// ------------------------------------------------------------------
	// A. "Grand Slam" (Gold): Win 100% of matches (min 3)
	// ------------------------------------------------------------------
	grandSlam := Achievement{
		ID: "tourn_grand_slam", Name: "Grand Slam", Description: "Win all matches in a tournament (min 3)",
		Icon: "trophy", Tier: "gold", MaxProgress: 100, IsUnlocked: false,
	}
	if matches >= 3 && wins == matches {
		grandSlam.IsUnlocked = true
		grandSlam.Progress = 100
	} else if matches > 0 {
		grandSlam.Progress = int((float64(wins) / float64(matches)) * 100)
	}
	list = append(list, grandSlam)

	// ------------------------------------------------------------------
	// B. "Survivor" (Bronze): Play at least 5 matches
	// ------------------------------------------------------------------
	survivor := Achievement{
		ID: "tourn_survivor", Name: "Survivor", Description: "Play at least 5 matches in a tournament",
		Icon: "boot", Tier: "bronze", MaxProgress: 5, IsUnlocked: false,
	}
	survivor.Progress = matches
	if matches >= 5 {
		survivor.IsUnlocked = true
		survivor.Progress = 5
	}
	list = append(list, survivor)

	return list, nil
}
