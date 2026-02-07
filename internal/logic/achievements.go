package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

type achievementsService struct {
	ch driver.Conn
	pg PgPool
}

func NewAchievementsService(ch driver.Conn, pg PgPool) AchievementsService {
	return &achievementsService{ch: ch, pg: pg}
}



type AchievementScope string

const (
	ScopeMatch      AchievementScope = "match"
	ScopeTournament AchievementScope = "tournament"
	ScopeGlobal     AchievementScope = "global"
)

// GetAchievements calculates achievements for a specific scope (match, tournament, etc.)
// contextID is the match_id or tournament_id
func (s *achievementsService) GetAchievements(ctx context.Context, scope AchievementScope, contextID string, playerID string) ([]models.ContextualAchievement, error) {
	switch scope {
	case ScopeMatch:
		return s.getMatchAchievements(ctx, contextID, playerID)
	case ScopeTournament:
		return s.getTournamentAchievements(ctx, contextID, playerID)
	default:
		return nil, fmt.Errorf("unsupported scope: %s", scope)
	}
}

func (s *achievementsService) getMatchAchievements(ctx context.Context, matchID, playerID string) ([]models.ContextualAchievement, error) {
	list := []models.ContextualAchievement{}

	// 1. Fetch Stats for this match
	var (
		kills, deaths, shotsFired, shotsHit float64
		win                                 int
	)

	// Updated to use correct event types and logic
	query := `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed') AND actor_id = ?) as kills,
			countIf(event_type IN ('player_kill', 'bot_killed') AND target_id = ?) as deaths,
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
	untouchable := models.ContextualAchievement{
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
	pacifist := models.ContextualAchievement{
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
	sharpshooter := models.ContextualAchievement{
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

	// ------------------------------------------------------------------
	// D. "Wipeout" (Gold): Kill entire enemy team in one round
	// ------------------------------------------------------------------
	wipeout := models.ContextualAchievement{
		ID: "match_wipeout", Name: "Wipeout", Description: "Eliminate the entire enemy team in a single round",
		Icon: "skull", Tier: "gold", MaxProgress: 1, IsUnlocked: false,
	}

	// Logic: Find rounds in this match where the player killed all unique enemies
	// Query to find rounds and enemy counts
	wipeoutQuery := `
		SELECT 
			round_number,
			uniq(target_id) as enemies_killed
		FROM raw_events
		WHERE match_id = ? AND actor_id = ? AND event_type IN ('player_kill', 'bot_killed') AND target_id != '' AND target_id != 'world'
		GROUP BY round_number
	`
	rows, err := s.ch.Query(ctx, wipeoutQuery, matchID, playerID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rNum int
			var killedCount int
			if err := rows.Scan(&rNum, &killedCount); err != nil {
				continue
			}

			// We need to know how many enemies were in that team during that round
			var totalEnemies int
			// First get the player's team in that round
			var pTeam string
			s.ch.QueryRow(ctx, "SELECT actor_team FROM raw_events WHERE match_id = ? AND actor_id = ? AND round_number = ? AND actor_team != '' LIMIT 1", matchID, playerID, rNum).Scan(&pTeam)

			if pTeam != "" {
				enemyTeam := "axis"
				if pTeam == "axis" {
					enemyTeam = "allies"
				}

				// Count all unique enemies who either acted or were targeted in this round
				enemyCountQuery := `
					SELECT uniqExact(player_id) FROM (
						SELECT actor_id as player_id FROM raw_events 
						WHERE match_id = ? AND round_number = ? AND actor_team = ? AND actor_id != ''
						UNION ALL
						SELECT target_id as player_id FROM raw_events 
						WHERE match_id = ? AND round_number = ? AND target_team = ? AND target_id != ''
					)
				`
				s.ch.QueryRow(ctx, enemyCountQuery, matchID, rNum, enemyTeam, matchID, rNum, enemyTeam).Scan(&totalEnemies)

				if killedCount > 0 && totalEnemies > 0 && killedCount >= totalEnemies {
					wipeout.IsUnlocked = true
					wipeout.Progress = 1
					break
				}
			}
		}
	}
	list = append(list, wipeout)

	return list, nil
}

func (s *achievementsService) getTournamentAchievements(ctx context.Context, tournamentID, playerID string) ([]models.ContextualAchievement, error) {
	list := []models.ContextualAchievement{}

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
	grandSlam := models.ContextualAchievement{
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
	survivor := models.ContextualAchievement{
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

func (s *achievementsService) GetPlayerAchievements(ctx context.Context, playerGUID string) ([]models.PlayerAchievement, error) {
	// Query persistent achievements from Postgres
	// Join with definitions to get metadata
	// Schema 001: player_achievements (smf_member_id, achievement_id) -> achievements (id)
	query := `
		SELECT
			pa.player_achievement_id, reg.player_guid, pa.achievement_id, pa.unlocked_at,
			a.achievement_id, a.achievement_name, a.description, a.category, a.points, a.icon_url
		FROM mohaa_player_achievements pa
		JOIN mohaa_achievements a ON pa.achievement_id = a.achievement_id
		JOIN player_guid_registry reg ON pa.smf_member_id = reg.smf_member_id
		WHERE reg.player_guid = $1
	`
	rows, err := s.pg.Query(ctx, query, playerGUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.PlayerAchievement
	for rows.Next() {
		var pa models.PlayerAchievement
		pa.Achievement = &models.Achievement{}

		var iconURL *string // Handle nullable

		if err := rows.Scan(
			&pa.ID, &pa.PlayerGUID, &pa.AchievementID, &pa.UnlockedAt,
			&pa.Achievement.ID, &pa.Achievement.Name, &pa.Achievement.Description,
			&pa.Achievement.Category, &pa.Achievement.Points, &iconURL,
		); err != nil {
			return nil, err
		}

		if iconURL != nil {
			pa.Achievement.IconURL = *iconURL
		}

		// Set default Tier based on points (10=Bronze/1, 25=Silver/2, 50=Gold/3, 100=Platinum/4)
		switch pa.Achievement.Points {
		case 10:
			pa.Achievement.Tier = 1
		case 25:
			pa.Achievement.Tier = 2
		case 50:
			pa.Achievement.Tier = 3
		case 100:
			pa.Achievement.Tier = 4
		case 200, 250:
			pa.Achievement.Tier = 5
		default:
			pa.Achievement.Tier = 1
		}

		list = append(list, pa)
	}
	return list, nil
}

func (s *achievementsService) GetRecentAchievements(ctx context.Context, limit int) ([]models.PlayerAchievement, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT
			pa.player_achievement_id, reg.player_guid, pa.achievement_id, pa.unlocked_at,
			a.achievement_id, a.achievement_name, a.description, a.category, a.points, a.icon_url
		FROM mohaa_player_achievements pa
		JOIN mohaa_achievements a ON pa.achievement_id = a.achievement_id
		JOIN player_guid_registry reg ON pa.smf_member_id = reg.smf_member_id
		ORDER BY pa.unlocked_at DESC
		LIMIT $1
	`
	rows, err := s.pg.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.PlayerAchievement
	for rows.Next() {
		var pa models.PlayerAchievement
		pa.Achievement = &models.Achievement{}

		var iconURL *string // Handle nullable

		if err := rows.Scan(
			&pa.ID, &pa.PlayerGUID, &pa.AchievementID, &pa.UnlockedAt,
			&pa.Achievement.ID, &pa.Achievement.Name, &pa.Achievement.Description,
			&pa.Achievement.Category, &pa.Achievement.Points, &iconURL,
		); err != nil {
			return nil, err
		}

		if iconURL != nil {
			pa.Achievement.IconURL = *iconURL
		}

		// Set default Tier based on points
		switch pa.Achievement.Points {
		case 10:
			pa.Achievement.Tier = 1
		case 25:
			pa.Achievement.Tier = 2
		case 50:
			pa.Achievement.Tier = 3
		case 100:
			pa.Achievement.Tier = 4
		case 200, 250:
			pa.Achievement.Tier = 5
		default:
			pa.Achievement.Tier = 1
		}

		list = append(list, pa)
	}
	return list, nil
}
