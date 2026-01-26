package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"golang.org/x/sync/errgroup"
)

type playerStatsService struct {
	ch driver.Conn
}

func NewPlayerStatsService(ch driver.Conn) PlayerStatsService {
	return &playerStatsService{ch: ch}
}

// DeepStats represents the massive aggregated stats object
type DeepStats struct {
	Combat      CombatStats      `json:"combat"`
	Weapons     []WeaponStats    `json:"weapons"`
	Movement    MovementStats    `json:"movement"`
	Accuracy    AccuracyStats    `json:"accuracy"`
	Session     SessionStats     `json:"session"`
	Rivals      RivalStats       `json:"rivals"`
	Stance      StanceStats      `json:"stance"`
	Interaction InteractionStats `json:"interaction"`
}

type RivalStats struct {
	NemesisName  string `json:"nemesis_name,omitempty"`
	NemesisKills uint64 `json:"nemesis_kills"` // How many times they killed me
	VictimName   string `json:"victim_name,omitempty"`
	VictimKills  uint64 `json:"victim_kills"` // How many times I killed them
}

type StanceStats struct {
	StandingKills uint64  `json:"standing_kills"`
	CrouchKills   uint64  `json:"crouch_kills"`
	ProneKills    uint64  `json:"prone_kills"`
	StandingPct   float64 `json:"standing_pct"`
	CrouchPct     float64 `json:"crouch_pct"`
	PronePct      float64 `json:"prone_pct"`
}

type CombatStats struct {
	Kills           uint64  `json:"kills"`
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

type WeaponStats struct {
	Name      string  `json:"name"`
	Kills     uint64  `json:"kills"`
	Deaths    uint64  `json:"deaths"`
	Headshots uint64  `json:"headshots"`
	Accuracy  float64 `json:"accuracy"`
	Shots     uint64  `json:"shots"`
	Hits      uint64  `json:"hits"`
	Damage    uint64  `json:"damage"`
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

// GetDeepStats fetches all categories for a player
func (s *playerStatsService) GetDeepStats(ctx context.Context, guid string) (*DeepStats, error) {
	stats := &DeepStats{}

	g, ctx := errgroup.WithContext(ctx)

	// Combat stats first, then Stance stats which depend on Combat.Kills
	g.Go(func() error {
		if err := s.fillCombatStats(ctx, guid, &stats.Combat); err != nil {
			return fmt.Errorf("combat stats: %w", err)
		}
		if err := s.fillStanceStats(ctx, guid, &stats.Stance, stats.Combat.Kills); err != nil {
			stats.Stance = StanceStats{}
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
			stats.Rivals = RivalStats{}
		}
		return nil
	})

	g.Go(func() error {
		if err := s.fillInteractionStats(ctx, guid, &stats.Interaction); err != nil {
			// Log or ignore
			stats.Interaction = InteractionStats{}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *playerStatsService) fillCombatStats(ctx context.Context, guid string, out *CombatStats) error {
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

func (s *playerStatsService) fillWeaponStats(ctx context.Context, guid string, out *[]WeaponStats) error {
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
		var w WeaponStats
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

func (s *playerStatsService) fillMovementStats(ctx context.Context, guid string, out *MovementStats) error {
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

func (s *playerStatsService) fillAccuracyStats(ctx context.Context, guid string, out *AccuracyStats) error {
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

func (s *playerStatsService) fillSessionStats(ctx context.Context, guid string, out *SessionStats) error {
	// Count unique matches
	query := `SELECT uniq(match_id) as matches FROM mohaa_stats.raw_events WHERE actor_id = ?`
	if err := s.ch.QueryRow(ctx, query, guid).Scan(&out.MatchesPlayed); err != nil {
		return err
	}

	// Count wins: matches where a team_win event exists for the player's team
	// This requires joining to find the player's team in each match, then checking if that team won
	winsQuery := `
		WITH player_matches AS (
			SELECT DISTINCT match_id, any(actor_team) as team
			FROM mohaa_stats.raw_events
			WHERE actor_id = ? AND actor_team != ''
			GROUP BY match_id
		),
		winning_teams AS (
			SELECT match_id, any(JSONExtractString(raw_json, 'team')) as winning_team
			FROM mohaa_stats.raw_events
			WHERE event_type = 'team_win'
			GROUP BY match_id
		)
		SELECT count()
		FROM player_matches pm
		JOIN winning_teams wt ON pm.match_id = wt.match_id
		WHERE pm.team = wt.winning_team
	`
	if err := s.ch.QueryRow(ctx, winsQuery, guid).Scan(&out.Wins); err != nil {
		// If error, set wins to 0 (team_win might not have team field)
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

func (s *playerStatsService) fillInteractionStats(ctx context.Context, guid string, out *InteractionStats) error {
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
		var p PickupStat
		if err := rows.Scan(&p.ItemName, &p.Count); err == nil {
			out.Pickups = append(out.Pickups, p)
		}
	}
	return nil
}

func (s *playerStatsService) fillRivalStats(ctx context.Context, guid string, out *RivalStats) error {
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

func (s *playerStatsService) fillStanceStats(ctx context.Context, guid string, out *StanceStats, totalKills uint64) error {
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
