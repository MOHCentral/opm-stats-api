package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
	"golang.org/x/sync/errgroup"
)

type playerStatsService struct {
	ch driver.Conn
}

func NewPlayerStatsService(ch driver.Conn) PlayerStatsService {
	return &playerStatsService{ch: ch}
}



// GetDeepStats fetches all categories for a player
func (s *playerStatsService) GetDeepStats(ctx context.Context, guid string) (*models.DeepStats, error) {
	stats := &models.DeepStats{}

	g, ctx := errgroup.WithContext(ctx)

	// Combat stats first, then Stance stats which depend on Combat.Kills
	g.Go(func() error {
		if err := s.fillCombatStats(ctx, guid, &stats.Combat); err != nil {
			return fmt.Errorf("combat stats: %w", err)
		}
		if err := s.fillStanceStats(ctx, guid, &stats.Stance, stats.Combat.Kills); err != nil {
			stats.Stance = models.StanceStats{}
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillWeaponStats(ctx, guid, &stats.Weapons); err != nil {
			return fmt.Errorf("weapon stats: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillMovementStats(ctx, guid, &stats.Movement); err != nil {
			return fmt.Errorf("movement stats: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillAccuracyStats(ctx, guid, &stats.Accuracy); err != nil {
			return fmt.Errorf("accuracy stats: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillSessionStats(ctx, guid, &stats.Session); err != nil {
			return fmt.Errorf("session stats: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillRivalStats(ctx, guid, &stats.Rivals); err != nil {
			// Non-critical, log only? For now just return empty
			stats.Rivals = models.RivalStats{}
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillInteractionStats(ctx, guid, &stats.Interaction); err != nil {
			// Log or ignore
			stats.Interaction = models.InteractionStats{}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *playerStatsService) fillCombatStats(ctx context.Context, guid string, out *models.CombatStats) error {
	query := `
		SELECT 
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type = 'kill' AND target_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			countIf(event_type = 'kill' AND actor_id = ? AND hitloc IN ('torso','torso_lower','torso_upper')) as torso,
			countIf(event_type = 'kill' AND actor_id = ? AND hitloc IN ('left_arm','right_arm','left_leg','right_leg','left_arm_lower','left_arm_upper','right_arm_lower','right_arm_upper','left_leg_lower','left_leg_upper','right_leg_lower','right_leg_upper')) as limbs,
			countIf((event_type = 'bash' OR event_type = 'player_bash') AND actor_id = ?) as melee,
			countIf(event_type = 'kill' AND actor_id = ? AND actor_id = target_id) as suicides,
			countIf(event_type = 'kill' AND actor_id = ? AND actor_team != '' AND actor_team = target_team AND actor_id != target_id) as team_kills,
			countIf(event_type = 'player_roadkill' AND actor_id = ?) as roadkills,
			countIf((event_type = 'bash' OR event_type = 'player_bash') AND actor_id = ?) as bash_kills,
			countIf(event_type = 'grenade_kill' AND actor_id = ?) as grenade_kills,
			countIf(event_type = 'grenade_throw' AND actor_id = ?) as grenades_thrown,
			sumIf(damage, event_type = 'damage' AND target_id = ?) as damage_dealt,
			sumIf(damage, event_type = 'damage' AND actor_id = ?) as damage_taken
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?)
	`
	if err := s.ch.QueryRow(ctx, query,
		guid, guid, guid, guid, guid, guid, guid, guid, guid, guid,
		guid, guid, // Grenade Kills, Grenade Throws
		guid, guid, // Damage Dealt, Damage Taken
		guid, guid, // WHERE clause
	).Scan(
		&out.Kills, &out.Deaths, &out.Headshots,
		&out.TorsoKills, &out.LimbKills, &out.MeleeKills, &out.Suicides,
		&out.TeamKills, &out.Roadkills, &out.BashKills,
		&out.GrenadeKills, &out.GrenadesThrown,
		&out.DamageDealt, &out.DamageTaken,
	); err != nil {
		return err
	}

	// Nutshots, Backstabs, FirstBloods, Longshots require specific hitloc values from game server
	// These fields stay at 0 until game server sends events with proper hitloc (e.g., 'groin', 'back')

	if out.Deaths > 0 {
		out.KDRatio = float64(out.Kills) / float64(out.Deaths)
	} else {
		out.KDRatio = float64(out.Kills)
	}

	if out.Kills > 0 {
		out.HeadshotPercent = (float64(out.Headshots) / float64(out.Kills)) * 100
	}

	return nil
}

func (s *playerStatsService) fillWeaponStats(ctx context.Context, guid string, out *[]models.PlayerWeaponStats) error {
	query := `
		SELECT 
			actor_weapon as weapon_name,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'headshot') as headshots,
			countIf(event_type = 'weapon_fire') as shots,
			countIf(event_type = 'weapon_hit') as hits,
			sumIf(damage, event_type = 'damage' AND actor_id = ?) as damage
		FROM mohaa_stats.raw_events
		WHERE actor_id = ? AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
	`
	rows, err := s.ch.Query(ctx, query, guid, guid)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var w models.PlayerWeaponStats
		if err := rows.Scan(&w.Name, &w.Kills, &w.Headshots, &w.Shots, &w.Hits, &w.Damage); err != nil {
			continue
		}
		if w.Shots > 0 {
			w.Accuracy = (float64(w.Hits) / float64(w.Shots)) * 100
		}
		*out = append(*out, w)
	}
	return nil
}

func (s *playerStatsService) fillMovementStats(ctx context.Context, guid string, out *models.MovementStats) error {
	// Distance event stores walked/sprinted/swam/driven in raw_json
	// Convert game units to kilometers (divide by 100000)
	query := `
		SELECT 
			(sumIf(JSONExtractFloat(raw_json, 'walked'), event_type = 'distance') + 
			 sumIf(JSONExtractFloat(raw_json, 'sprinted'), event_type = 'distance') + 
			 sumIf(JSONExtractFloat(raw_json, 'swam'), event_type = 'distance') + 
			 sumIf(JSONExtractFloat(raw_json, 'driven'), event_type = 'distance')) / 100000.0 as km,
			countIf(event_type = 'jump') as jumps,
			countIf(event_type = 'crouch') as crouches,
			countIf(event_type = 'prone') as prones
		FROM mohaa_stats.raw_events
		WHERE actor_id = ?
	`

	var crouches, prones uint64
	if err := s.ch.QueryRow(ctx, query, guid).Scan(&out.TotalDistanceKm, &out.JumpCount, &crouches, &prones); err != nil {
		return err
	}
	// CrouchTimeSec and ProneTimeSec would need duration tracking from events
	// For now, count events as proxy
	out.CrouchTimeSec = float64(crouches)
	out.ProneTimeSec = float64(prones)
	return nil
}

func (s *playerStatsService) fillAccuracyStats(ctx context.Context, guid string, out *models.AccuracyStats) error {
	var shots, hits, headshots uint64
	var avgDist *float64

	query := `
		SELECT 
			countIf(event_type = 'weapon_fire') as shots,
			countIf(event_type = 'weapon_hit') as hits,
			countIf(event_type = 'headshot') as headshots,
			sumIf(distance, event_type = 'kill') / NULLIF(countIf(event_type = 'kill'), 0) as avg_dist
		FROM mohaa_stats.raw_events
		WHERE actor_id = ?
	`
	if err := s.ch.QueryRow(ctx, query, guid).Scan(&shots, &hits, &headshots, &avgDist); err != nil {
		return err
	}

	if shots > 0 {
		out.Overall = (float64(hits) / float64(shots)) * 100.0
	}
	if hits > 0 {
		out.HeadHitPct = (float64(headshots) / float64(hits)) * 100.0
	}
	if avgDist != nil {
		out.AvgDistance = *avgDist
	}

	return nil
}

func (s *playerStatsService) fillSessionStats(ctx context.Context, guid string, out *models.SessionStats) error {
	// Count unique matches
	query := `SELECT uniq(match_id) as matches FROM mohaa_stats.raw_events WHERE actor_id = ?`
	if err := s.ch.QueryRow(ctx, query, guid).Scan(&out.MatchesPlayed); err != nil {
		return err
	}

	// Count wins using materialized view which tracks wins via match_outcome=1
	winsQuery := `
		SELECT sum(matches_won)
		FROM mohaa_stats.player_stats_daily_mv
		WHERE actor_id = ?
	`
	if err := s.ch.QueryRow(ctx, winsQuery, guid).Scan(&out.Wins); err != nil {
		// Default to 0 on error
		out.Wins = 0
	}

	if out.MatchesPlayed > 0 {
		out.WinRate = (float64(out.Wins) / float64(out.MatchesPlayed)) * 100
	}

	// Playtime: Use time difference between first and last event per match
	// Much more accurate than heartbeat counting
	playtimeQuery := `
		SELECT sum(duration) / 3600.0 as hours
		FROM (
			SELECT match_id, toUnixTimestamp(max(timestamp)) - toUnixTimestamp(min(timestamp)) as duration
			FROM mohaa_stats.raw_events
			WHERE actor_id = ?
			GROUP BY match_id
		)
	`
	if err := s.ch.QueryRow(ctx, playtimeQuery, guid).Scan(&out.PlaytimeHours); err != nil {
		out.PlaytimeHours = 0
	}
	return nil
}

func (s *playerStatsService) fillInteractionStats(ctx context.Context, guid string, out *models.InteractionStats) error {
	// Chat (both player_say and chat events)
	s.ch.QueryRow(ctx, "SELECT countIf((event_type='player_say' OR event_type='chat') AND actor_id=?) FROM mohaa_stats.raw_events", guid).Scan(&out.ChatMessages)

	// Vehicle/Turret Uses
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type='vehicle_enter' AND actor_id=?) as v_uses,
			countIf(event_type='turret_enter' AND actor_id=?) as t_uses
		FROM mohaa_stats.raw_events
	`, guid, guid).Scan(&out.VehicleUses, &out.TurretUses)

	// Top Pickups (item, ammo, health)
	rows, err := s.ch.Query(ctx, `
		WITH pickup_events AS (
			SELECT 
				CASE 
					WHEN event_type = 'item_pickup' THEN 'Item'
					WHEN event_type = 'ammo_pickup' THEN 'Ammo'
					WHEN event_type = 'health_pickup' THEN 'Health'
					ELSE 'Unknown'
				END as item_type
			FROM mohaa_stats.raw_events
			WHERE actor_id = ? AND event_type IN ('item_pickup', 'ammo_pickup', 'health_pickup')
		)
		SELECT item_type, count(*) as cnt
		FROM pickup_events
		GROUP BY item_type
		ORDER BY cnt DESC LIMIT 10
	`, guid)
	if err != nil {
		return nil // Ignore pickup errors
	}
	defer rows.Close()

	for rows.Next() {
		var p models.PickupStat
		if err := rows.Scan(&p.ItemName, &p.Count); err == nil {
			out.Pickups = append(out.Pickups, p)
		}
	}
	return nil
}

func (s *playerStatsService) fillRivalStats(ctx context.Context, guid string, out *models.RivalStats) error {
	// Find Nemesis (Player who killed me most)
	err := s.ch.QueryRow(ctx, `
		SELECT actor_name, count() as c 
		FROM mohaa_stats.raw_events 
		WHERE event_type='kill' AND target_id = ? AND actor_id != ? AND actor_id != '' AND actor_id != 'world'
		GROUP BY actor_name 
		ORDER BY c DESC LIMIT 1
	`, guid, guid).Scan(&out.NemesisName, &out.NemesisKills)
	if err != nil {
		// Ignore no-rows error
	}

	// Find Victim (Player I killed most)
	err = s.ch.QueryRow(ctx, `
		SELECT target_name, count() as c 
		FROM mohaa_stats.raw_events 
		WHERE event_type='kill' AND actor_id = ? AND target_id != ? AND target_id != '' AND target_id != 'world'
		GROUP BY target_name 
		ORDER BY c DESC LIMIT 1
	`, guid, guid).Scan(&out.VictimName, &out.VictimKills)

	return nil
}

func (s *playerStatsService) fillStanceStats(ctx context.Context, guid string, out *models.StanceStats, totalKills uint64) error {
	if totalKills == 0 {
		return nil
	}

	// Stance stats: Use stance-specific events that occurred near kill events
	// Since raw_events may not have actor_stance field, infer from recent stance events
	query := `
		SELECT 
			countIf(actor_stance = 'stand' OR actor_stance = 'standing') as standing,
			countIf(actor_stance = 'crouch' OR actor_stance = 'crouching') as crouching,
			countIf(actor_stance = 'prone') as prone
		FROM mohaa_stats.raw_events 
		WHERE event_type = 'kill' AND actor_id = ? AND actor_stance != ''
	`
	if err := s.ch.QueryRow(ctx, query, guid).Scan(
		&out.StandingKills, &out.CrouchKills, &out.ProneKills,
	); err != nil {
		// If query fails, leave at 0 - do not fabricate
		return nil
	}

	// Calculate percentages from real data only
	stanceTotal := out.StandingKills + out.CrouchKills + out.ProneKills
	if stanceTotal > 0 {
		out.StandingPct = (float64(out.StandingKills) / float64(stanceTotal)) * 100
		out.CrouchPct = (float64(out.CrouchKills) / float64(stanceTotal)) * 100
		out.PronePct = (float64(out.ProneKills) / float64(stanceTotal)) * 100
	}
	// If no stance data tracked, percentages stay at 0

	return nil
}

// ResolvePlayerGUID finds the most recent GUID associated with a player name
func (s *playerStatsService) ResolvePlayerGUID(ctx context.Context, name string) (string, error) {
	var guid string
	query := `
		SELECT actor_id 
		FROM mohaa_stats.raw_events 
		WHERE actor_name = ? AND actor_id != '' AND actor_id != 'world'
		ORDER BY timestamp DESC 
		LIMIT 1
	`
	if err := s.ch.QueryRow(ctx, query, name).Scan(&guid); err != nil {
		// Also check target_name in case they were only victims
		err2 := s.ch.QueryRow(ctx, `
			SELECT target_id 
			FROM mohaa_stats.raw_events 
			WHERE target_name = ? AND target_id != '' AND target_id != 'world'
			ORDER BY timestamp DESC 
			LIMIT 1
		`, name).Scan(&guid)
		if err2 != nil {
			return "", fmt.Errorf("player not found by name: %w", err2)
		}
	}
	return guid, nil
}

// GetPlayerStatsByGametype returns stats grouped by gametype (derived from map prefix)
func (s *playerStatsService) GetPlayerStatsByGametype(ctx context.Context, guid string) ([]models.GametypeStats, error) {
	// Derive gametype from map_name prefix (dm_, obj_, lib_, tdm_)
	// Aggregate kills, deaths, headshots per gametype
	rows, err := s.ch.Query(ctx, `
		SELECT
			multiIf(
				startsWith(map_name, 'dm_'), 'dm',
				startsWith(map_name, 'obj_'), 'obj',
				startsWith(map_name, 'lib_'), 'lib',
				startsWith(map_name, 'tdm_'), 'tdm',
				startsWith(map_name, 'ctf_'), 'ctf',
				'other'
			) as gametype,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type IN ('death', 'kill') AND target_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			uniq(match_id) as matches_played
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?)
		  AND map_name != ''
		GROUP BY gametype
		HAVING kills > 0 OR deaths > 0
		ORDER BY kills DESC
	`, guid, guid, guid, guid, guid)

	if err != nil {
		return nil, fmt.Errorf("failed to query gametype stats: %w", err)
	}
	defer rows.Close()

	stats := []models.GametypeStats{}
	for rows.Next() {
		var s models.GametypeStats
		if err := rows.Scan(&s.Gametype, &s.Kills, &s.Deaths, &s.Headshots, &s.MatchesPlayed); err != nil {
			continue
		}
		if s.Deaths > 0 {
			s.KDRatio = float64(s.Kills) / float64(s.Deaths)
		} else if s.Kills > 0 {
			s.KDRatio = float64(s.Kills)
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// GetPlayerStatsByMap returns detailed stats grouped by map
func (s *playerStatsService) GetPlayerStatsByMap(ctx context.Context, guid string) ([]models.PlayerMapStats, error) {
	// Query map stats - aggregating kills, deaths, headshots per map
	rows, err := s.ch.Query(ctx, `
		SELECT
			map_name,
			countIf(event_type = 'kill' AND actor_id = ?) as kills,
			countIf(event_type IN ('death', 'kill') AND target_id = ?) as deaths,
			countIf(event_type = 'headshot' AND actor_id = ?) as headshots,
			uniq(match_id) as matches_played,
			countIf(event_type = 'match_outcome' AND match_outcome = 1 AND actor_id = ?) as matches_won
		FROM mohaa_stats.raw_events
		WHERE (actor_id = ? OR target_id = ?)
		  AND map_name != ''
		GROUP BY map_name
		HAVING kills > 0 OR deaths > 0
		ORDER BY kills DESC
	`, guid, guid, guid, guid, guid, guid)

	if err != nil {
		return nil, fmt.Errorf("failed to query map breakdown: %w", err)
	}
	defer rows.Close()

	stats := []models.PlayerMapStats{}
	for rows.Next() {
		var s models.PlayerMapStats
		if err := rows.Scan(&s.MapName, &s.Kills, &s.Deaths, &s.Headshots, &s.MatchesPlayed, &s.MatchesWon); err != nil {
			continue
		}
		if s.Deaths > 0 {
			s.KDRatio = float64(s.Kills) / float64(s.Deaths)
		} else if s.Kills > 0 {
			s.KDRatio = float64(s.Kills)
		}
		stats = append(stats, s)
	}

	return stats, nil
}
