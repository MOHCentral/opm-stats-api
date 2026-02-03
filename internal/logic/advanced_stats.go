package logic

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/openmohaa/stats-api/internal/models"
)

// AdvancedStatsService provides comprehensive stats analysis
type advancedStatsService struct {
	ch driver.Conn
}

func NewAdvancedStatsService(ch driver.Conn) AdvancedStatsService {
	return &advancedStatsService{ch: ch}
}

// GetPeakPerformance returns when a player performs best
func (s *advancedStatsService) GetPeakPerformance(ctx context.Context, guid string) (*models.PeakPerformance, error) {
	peak := &models.PeakPerformance{}

	// Hourly breakdown
	rows, err := s.ch.Query(ctx, `
		SELECT 
			toHour(timestamp) as hour,
			toInt64(countIf(event_type IN ('player_kill', 'bot_killed') AND actor_id = ?)) as kills,
			toInt64(countIf(event_type = 'player_kill' AND actor_id = ?)) as player_kills,
			toInt64(countIf(event_type = 'bot_killed' AND actor_id = ?)) as bot_kills,
			toInt64(countIf((event_type IN ('player_kill', 'bot_killed') OR event_type = 'death') AND target_id = ?)) as deaths,
			toInt64(countIf(event_type = 'weapon_fire' AND actor_id = ?)) as shots,
			toInt64(countIf(event_type = 'weapon_hit' AND actor_id = ?)) as hits,
			toInt64(countIf(event_type = 'team_win' AND actor_id = ?)) as wins
		FROM raw_events
		WHERE actor_id = ? OR target_id = ?
		GROUP BY hour
		ORDER BY hour
	`, guid, guid, guid, guid, guid, guid, guid, guid, guid)
	if err != nil {
		return nil, fmt.Errorf("hourly query: %w", err)
	}
	defer rows.Close()

	var bestKD float64
	var bestAccHour int
	var bestAccuracy float64
	var mostWinsHour int
	var mostWins int64
	var mostLossesHour int
	var mostLosses int64

	for rows.Next() {
		var h models.HourStats
		var shots, hits int64
		if err := rows.Scan(&h.Hour, &h.Kills, &h.PlayerKills, &h.BotKills, &h.Deaths, &shots, &hits, &h.Wins); err != nil {
			continue
		}
		if h.Deaths > 0 {
			h.KDRatio = float64(h.Kills) / float64(h.Deaths)
		} else {
			h.KDRatio = float64(h.Kills)
		}
		if shots > 0 {
			h.Accuracy = (float64(hits) / float64(shots)) * 100
		}
		h.Losses = h.Deaths - h.Kills // Approx

		peak.HourlyBreakdown = append(peak.HourlyBreakdown, h)

		// Track best hour
		if h.KDRatio > bestKD && (h.Kills+h.Deaths) > 10 {
			bestKD = h.KDRatio
			peak.BestHour = h
		}
		if h.Accuracy > bestAccuracy && shots > 50 {
			bestAccuracy = h.Accuracy
			bestAccHour = h.Hour
		}
		if h.Wins > mostWins {
			mostWins = h.Wins
			mostWinsHour = h.Hour
		}
		if h.Losses > mostLosses {
			mostLosses = h.Losses
			mostLossesHour = h.Hour
		}
	}

	peak.MostAccurateAt = fmt.Sprintf("%02d:00", bestAccHour)
	peak.MostWinsAt = fmt.Sprintf("%02d:00", mostWinsHour)
	peak.MostLossesAt = fmt.Sprintf("%02d:00", mostLossesHour)

	// Daily breakdown
	dayRows, err := s.ch.Query(ctx, `
		SELECT 
			toDayOfWeek(timestamp) as dow,
			toInt64(countIf(event_type IN ('player_kill', 'bot_killed') AND actor_id = ?)) as kills,
			toInt64(countIf(event_type = 'player_kill' AND actor_id = ?)) as player_kills,
			toInt64(countIf(event_type = 'bot_killed' AND actor_id = ?)) as bot_kills,
			toInt64(countIf((event_type IN ('player_kill', 'bot_killed') OR event_type = 'death') AND target_id = ?)) as deaths,
			toInt64(countIf(event_type = 'weapon_fire' AND actor_id = ?)) as shots,
			toInt64(countIf(event_type = 'weapon_hit' AND actor_id = ?)) as hits
		FROM raw_events
		WHERE actor_id = ? OR target_id = ?
		GROUP BY dow
		ORDER BY dow
	`, guid, guid, guid, guid, guid, guid, guid, guid)
	if err == nil {
		defer dayRows.Close()
		dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
		var bestDayKD float64
		for dayRows.Next() {
			var d models.DayStats
			var dow int
			var shots, hits int64
			if err := dayRows.Scan(&dow, &d.Kills, &d.PlayerKills, &d.BotKills, &d.Deaths, &shots, &hits); err != nil {
				continue
			}
			d.DayNum = dow
			if dow >= 1 && dow <= 7 {
				d.DayOfWeek = dayNames[dow-1]
			}
			if d.Deaths > 0 {
				d.KDRatio = float64(d.Kills) / float64(d.Deaths)
			} else {
				d.KDRatio = float64(d.Kills)
			}
			if shots > 0 {
				d.Accuracy = (float64(hits) / float64(shots)) * 100
			}
			peak.DailyBreakdown = append(peak.DailyBreakdown, d)

			if d.KDRatio > bestDayKD && (d.Kills+d.Deaths) > 20 {
				bestDayKD = d.KDRatio
				peak.BestDay = d
			}
		}
	}

	// Best map
	s.ch.QueryRow(ctx, `
		SELECT 
			map_name,
			toInt64(countIf(event_type IN ('player_kill', 'bot_killed') AND actor_id = ?)) as kills,
			toInt64(countIf(event_type = 'player_kill' AND actor_id = ?)) as player_kills,
			toInt64(countIf(event_type = 'bot_killed' AND actor_id = ?)) as bot_kills,
			toInt64(countIf((event_type IN ('player_kill', 'bot_killed') OR event_type = 'death') AND target_id = ?)) as deaths
		FROM raw_events
		WHERE (actor_id = ? OR target_id = ?) AND map_name != ''
		GROUP BY map_name
		ORDER BY kills DESC
		LIMIT 1
	`, guid, guid, guid, guid, guid, guid).Scan(&peak.BestMap.MapName, &peak.BestMap.Kills, &peak.BestMap.PlayerKills, &peak.BestMap.BotKills, &peak.BestMap.Deaths)
	if peak.BestMap.Deaths > 0 {
		peak.BestMap.KDRatio = float64(peak.BestMap.Kills) / float64(peak.BestMap.Deaths)
	}

	// Best weapon
	s.ch.QueryRow(ctx, `
		SELECT 
			actor_weapon,
			toInt64(count()) as kills,
			toInt64(countIf(event_type = 'player_kill')) as player_kills,
			toInt64(countIf(event_type = 'bot_killed')) as bot_kills,
			toInt64(countIf(hitloc IN ('head', 'helmet'))) as headshots
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 1
	`, guid).Scan(&peak.BestWeapon.WeaponName, &peak.BestWeapon.Kills, &peak.BestWeapon.PlayerKills, &peak.BestWeapon.BotKills, &peak.BestWeapon.Headshots)
	if peak.BestWeapon.Kills > 0 {
		peak.BestWeapon.HSPercent = (float64(peak.BestWeapon.Headshots) / float64(peak.BestWeapon.Kills)) * 100
	}

	// Calculate Best Conditions for Summary
	peak.BestConditions.BestMap = peak.BestMap.MapName
	peak.BestConditions.BestDay = peak.BestDay.DayOfWeek

	// Convert hour to label
	h := peak.BestHour.Hour
	switch {
	case h >= 5 && h < 12:
		peak.BestConditions.BestHourLabel = "Morning"
	case h >= 12 && h < 17:
		peak.BestConditions.BestHourLabel = "Afternoon"
	case h >= 17 && h < 21:
		peak.BestConditions.BestHourLabel = "Evening"
	default:
		peak.BestConditions.BestHourLabel = "Night"
	}

	// Simple heuristic for optimal session length (mock for now or derived from playtime)
	peak.BestConditions.OptimalSessionMins = 45

	return peak, nil
}

// GetDrillDown breaks down a stat by a dimension
func (s *advancedStatsService) GetDrillDown(ctx context.Context, guid string, stat string, dimension string, limit int) (*models.DrillDownResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	result := &models.DrillDownResult{
		Stat:      stat,
		Dimension: dimension,
		Items:     []models.DrillDownItem{},
	}

	// Build query based on stat and dimension
	var query string
	var groupCol string

	switch dimension {
	case "weapon":
		groupCol = "actor_weapon"
	case "map":
		groupCol = "map_name"
	case "hour":
		groupCol = "toHour(timestamp)"
	case "day":
		groupCol = "toDayOfWeek(timestamp)"
	case "victim":
		groupCol = "target_name"
	case "killer":
		groupCol = "actor_name"
	case "hitloc":
		groupCol = "hitloc"
	case "server":
		groupCol = "server_id"
	default:
		groupCol = "map_name"
	}

	var eventType string
	var actorFilter string

	switch stat {
	case "kills":
		eventType = "player_kill"
		actorFilter = "actor_id = ?"
	case "deaths":
		eventType = "player_kill"
		actorFilter = "target_id = ?"
	case "headshots":
		eventType = "player_headshot"
		actorFilter = "actor_id = ?"
	case "damage":
		eventType = "player_damage"
		actorFilter = "actor_id = ?"
	case "shots":
		eventType = "weapon_fire"
		actorFilter = "actor_id = ?"
	case "hits":
		eventType = "weapon_hit"
		actorFilter = "actor_id = ?"
	default:
		eventType = "player_kill"
		actorFilter = "actor_id = ?"
	}

	query = fmt.Sprintf(`
		SELECT 
			%s as dim_value,
			toInt64(count()) as count
		FROM raw_events
		WHERE event_type = ? AND %s AND %s != ''
		GROUP BY dim_value
		ORDER BY count DESC
		LIMIT ?
	`, groupCol, actorFilter, groupCol)

	rows, err := s.ch.Query(ctx, query, eventType, guid, limit)
	if err != nil {
		return nil, fmt.Errorf("drill-down query: %w", err)
	}
	defer rows.Close()

	var total int64
	for rows.Next() {
		var item models.DrillDownItem
		if err := rows.Scan(&item.Label, &item.Value); err != nil {
			continue
		}
		total += item.Value
		result.Items = append(result.Items, item)
	}

	// Calculate percentages
	result.Total = total
	for i := range result.Items {
		if total > 0 {
			result.Items[i].Percentage = (float64(result.Items[i].Value) / float64(total)) * 100
		}
	}

	return result, nil
}

// GetComboMetrics returns cross-dimensional stat combinations
func (s *advancedStatsService) GetComboMetrics(ctx context.Context, guid string) (*models.ComboMetrics, error) {
	combo := &models.ComboMetrics{}

	// Weapon on Map (best weapon per map)
	rows, err := s.ch.Query(ctx, `
		SELECT 
			map_name,
			actor_weapon,
			toInt64(count()) as kills
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND actor_weapon != '' AND map_name != ''
		GROUP BY map_name, actor_weapon
		ORDER BY map_name, kills DESC
	`, guid)
	if err == nil {
		defer rows.Close()
		seenMaps := make(map[string]bool)
		for rows.Next() {
			var wm models.WeaponMapCombo
			if err := rows.Scan(&wm.MapName, &wm.WeaponName, &wm.Kills); err != nil {
				continue
			}
			if !seenMaps[wm.MapName] {
				combo.WeaponOnMap = append(combo.WeaponOnMap, wm)
				seenMaps[wm.MapName] = true
			}
		}
	}

	// Victim patterns (who you dominate)
	victimRows, err := s.ch.Query(ctx, `
		WITH 
			kills AS (
				SELECT target_name as name, toInt64(count()) as k, any(actor_weapon) as wpn
				FROM raw_events
				WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND target_name != ''
				GROUP BY target_name
			),
			deaths AS (
				SELECT actor_name as name, toInt64(count()) as d
				FROM raw_events
				WHERE event_type IN ('player_kill', 'bot_killed') AND target_id = ? AND actor_name != ''
				GROUP BY actor_name
			)
		SELECT 
			kills.name,
			kills.k,
			COALESCE(deaths.d, 0) as d,
			kills.wpn
		FROM kills
		LEFT JOIN deaths ON kills.name = deaths.name
		ORDER BY kills.k DESC
		LIMIT 10
	`, guid, guid)
	if err == nil {
		defer victimRows.Close()
		for victimRows.Next() {
			var vp models.VictimPattern
			if err := victimRows.Scan(&vp.VictimName, &vp.Kills, &vp.DeathsTo, &vp.FavoriteWeapon); err != nil {
				continue
			}
			if vp.DeathsTo > 0 {
				vp.Ratio = float64(vp.Kills) / float64(vp.DeathsTo)
			} else {
				vp.Ratio = float64(vp.Kills)
			}
			combo.VictimPatterns = append(combo.VictimPatterns, vp)
		}
	}

	// Killer patterns (who dominates you)
	killerRows, err := s.ch.Query(ctx, `
		WITH 
			deaths AS (
				SELECT actor_name as name, toInt64(count()) as d, any(actor_weapon) as wpn
				FROM raw_events
				WHERE event_type IN ('player_kill', 'bot_killed') AND target_id = ? AND actor_name != ''
				GROUP BY actor_name
			),
			kills AS (
				SELECT target_name as name, toInt64(count()) as k
				FROM raw_events
				WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND target_name != ''
				GROUP BY target_name
			)
		SELECT 
			deaths.name,
			deaths.d,
			COALESCE(kills.k, 0) as k,
			deaths.wpn
		FROM deaths
		LEFT JOIN kills ON deaths.name = kills.name
		ORDER BY deaths.d DESC
		LIMIT 10
	`, guid, guid)
	if err == nil {
		defer killerRows.Close()
		for killerRows.Next() {
			var kp models.KillerPattern
			if err := killerRows.Scan(&kp.KillerName, &kp.DeathsTo, &kp.KillsAgainst, &kp.MostUsedWeapon); err != nil {
				continue
			}
			combo.KillerPatterns = append(combo.KillerPatterns, kp)
		}
	}

	// Distance by weapon
	distRows, err := s.ch.Query(ctx, `
		SELECT 
			actor_weapon,
			avg(distance) as avg_dist,
			max(distance) as max_dist,
			min(distance) as min_dist
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND actor_weapon != '' AND distance > 0
		GROUP BY actor_weapon
		ORDER BY avg_dist DESC
		LIMIT 10
	`, guid)
	if err == nil {
		defer distRows.Close()
		for distRows.Next() {
			var dw models.DistanceWeapon
			if err := distRows.Scan(&dw.WeaponName, &dw.AvgDistance, &dw.MaxDistance, &dw.MinDistance); err != nil {
				continue
			}
			combo.DistanceByWeapon = append(combo.DistanceByWeapon, dw)
		}
	}

	// Hitloc by weapon
	hitlocRows, err := s.ch.Query(ctx, `
		SELECT 
			actor_weapon,
			countIf(hitloc = 'head') * 100.0 / count() as head_pct,
			countIf(hitloc = 'torso') * 100.0 / count() as torso_pct,
			countIf(hitloc IN ('left_arm', 'right_arm', 'left_leg', 'right_leg')) * 100.0 / count() as limb_pct
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND actor_weapon != '' AND hitloc != ''
		GROUP BY actor_weapon
		HAVING count() >= 10
		ORDER BY head_pct DESC
		LIMIT 10
	`, guid)
	if err == nil {
		defer hitlocRows.Close()
		for hitlocRows.Next() {
			var hw models.HitlocWeapon
			if err := hitlocRows.Scan(&hw.WeaponName, &hw.HeadPct, &hw.TorsoPct, &hw.LimbPct); err != nil {
				continue
			}
			combo.HitlocByWeapon = append(combo.HitlocByWeapon, hw)
		}
	}

	// --- Calculate Derived Signatures ---

	// 1. Play Style Analysis (Simple Heuristic for now)
	// Could be based on weapon classes, movement, etc.
	// Defaulting to "Soldier" if no clear pattern, or use best weapon class
	combo.Signature.PlayStyle = "Soldier"

	// Check best weapon for style hint
	s.ch.QueryRow(ctx, `
		SELECT any(actor_weapon) FROM raw_events 
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? 
		GROUP BY actor_weapon ORDER BY count() DESC LIMIT 1
	`, guid).Scan(&combo.Signature.PlayStyle)

	// Map specific weapons to styles
	switch combo.Signature.PlayStyle {
	case "kar98", "springfield":
		combo.Signature.PlayStyle = "Sniper"
	case "thompson", "mp40":
		combo.Signature.PlayStyle = "Rusher"
	case "shotgun":
		combo.Signature.PlayStyle = "Aggressor"
	case "bar", "stg44", "mp44":
		combo.Signature.PlayStyle = "Soldier"
	case "bazooka", "panzerschreck":
		combo.Signature.PlayStyle = "Demolitionist"
	case "":
		combo.Signature.PlayStyle = "Rookie"
	}

	// 2. Clutch Rate (Wins / Matches)
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'team_win') / nullIf(uniq(match_id), 0) * 100
		FROM raw_events WHERE actor_id = ?
	`, guid).Scan(&combo.Signature.ClutchRate)

	// 3. First Blood Rate (First kill in match / Matches) - Approximate by early timestamps
	// Skipping complex first-blood logic for speed, using placeholder or simple ratio
	combo.Signature.FirstBloodRate = 0.0

	// 4. Run & Gun Index (Velocity while killing)
	s.ch.QueryRow(ctx, `
		SELECT avg(toFloat64OrZero(extract(extra, 'velocity'))) 
		FROM raw_events WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ?
	`, guid).Scan(&combo.MovementCombat.RunGunIndex)

	// Normalize index (0-100), assuming max velocity ~300-400
	if combo.MovementCombat.RunGunIndex > 0 {
		combo.MovementCombat.RunGunIndex = (combo.MovementCombat.RunGunIndex / 300.0) * 100
		if combo.MovementCombat.RunGunIndex > 100 {
			combo.MovementCombat.RunGunIndex = 100
		}
	}

	// 5. Bunny Hop Efficiency (Jumps vs Distance or Kills)
	// Using Jumps per Kill as a rough proxy for "active" movement combat
	var jumps, kills float64
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'jump'),
			countIf(event_type IN ('player_kill', 'bot_killed'))
		FROM raw_events WHERE actor_id = ?
	`, guid).Scan(&jumps, &kills)

	if kills > 0 {
		combo.MovementCombat.BunnyHopEfficiency = min(100, (jumps/kills)*20) // Arbitrary scaling
	}

	return combo, nil
}

// GetVehicleStats returns vehicle and turret statistics
func (s *advancedStatsService) GetVehicleStats(ctx context.Context, guid string) (*models.VehicleStats, error) {
	stats := &models.VehicleStats{}

	// Basic vehicle stats
	err := s.ch.QueryRow(ctx, `
		SELECT 
			toInt64(countIf(event_type = 'vehicle_enter' AND actor_id = ?)) as uses,
			toInt64(countIf(event_type = 'player_roadkill' AND actor_id = ?)) as kills,
			toInt64(countIf(event_type = 'vehicle_death' AND actor_id = ?)) as deaths,
			sumIf(JSONExtractFloat(raw_json, 'driven', 'Float64'), event_type = 'distance' AND actor_id = ?) / 100000.0 as driven_km
		FROM raw_events
		WHERE actor_id = ?
	`, guid, guid, guid, guid, guid).Scan(&stats.VehicleUses, &stats.VehicleKills, &stats.VehicleDeaths, &stats.TotalDriven)
	if err != nil {
		return nil, err
	}

	// Turret stats
	s.ch.QueryRow(ctx, `
		SELECT 
			toInt64(countIf(event_type = 'turret_enter' AND actor_id = ?)) as uses,
			toInt64(countIf(event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND actor_weapon LIKE '%turret%')) as kills,
			toInt64(countIf(event_type IN ('player_kill', 'bot_killed') AND target_id = ? AND actor_weapon LIKE '%turret%')) as deaths
		FROM raw_events
		WHERE actor_id = ? OR target_id = ?
	`, guid, guid, guid, guid, guid).Scan(&stats.TurretStats.TurretUses, &stats.TurretStats.TurretKills, &stats.TurretStats.TurretDeaths)

	// Vehicle breakdown by type
	rows, err := s.ch.Query(ctx, `
		SELECT 
			JSONExtractString(raw_json, 'vehicle') as vehicle,
			count() as uses
		FROM raw_events
		WHERE event_type = 'vehicle_enter' AND actor_id = ? AND JSONExtractString(raw_json, 'vehicle') != ''
		GROUP BY vehicle
		ORDER BY uses DESC
		LIMIT 10
	`, guid)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var vt models.VehicleType
			if err := rows.Scan(&vt.VehicleName, &vt.Uses); err != nil {
				continue
			}
			stats.VehicleTypes = append(stats.VehicleTypes, vt)
		}
	}

	return stats, nil
}

// GetGameFlowStats returns round/objective/team statistics
func (s *advancedStatsService) GetGameFlowStats(ctx context.Context, guid string) (*models.GameFlowStats, error) {
	stats := &models.GameFlowStats{}

	// Basic round stats
	err := s.ch.QueryRow(ctx, `
		SELECT 
			toInt64(countIf(event_type = 'round_end' AND actor_id = ?)) as rounds,
			toInt64(countIf(event_type = 'team_win' AND actor_id = ?)) as wins,
			toInt64(countIf(event_type = 'objective_update' AND actor_id = ?)) as objectives
		FROM raw_events
		WHERE actor_id = ?
	`, guid, guid, guid, guid).Scan(&stats.RoundsPlayed, &stats.RoundsWon, &stats.ObjectivesTotal)
	if err != nil {
		return nil, err
	}

	stats.RoundsLost = stats.RoundsPlayed - stats.RoundsWon
	if stats.RoundsPlayed > 0 {
		stats.RoundWinRate = (float64(stats.RoundsWon) / float64(stats.RoundsPlayed)) * 100
	}

	// Objectives by type
	rows, err := s.ch.Query(ctx, `
		SELECT 
			JSONExtractString(raw_json, 'objective_type') as obj_type,
			count() as count
		FROM raw_events
		WHERE event_type = 'objective_update' AND actor_id = ? AND JSONExtractString(raw_json, 'objective_type') != ''
		GROUP BY obj_type
		ORDER BY count DESC
	`, guid)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var os models.ObjectiveStat
			if err := rows.Scan(&os.ObjectiveType, &os.Count); err != nil {
				continue
			}
			stats.ObjectivesByType = append(stats.ObjectivesByType, os)
		}
	}

	// Team stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(team = 'allies') * 100.0 / count() as allies_pct,
			countIf(team = 'axis') * 100.0 / count() as axis_pct
		FROM raw_events
		WHERE event_type = 'team_join' AND actor_id = ? AND team IN ('allies', 'axis')
	`, guid).Scan(&stats.TeamStats.AlliesPlaytime, &stats.TeamStats.AxisPlaytime)

	return stats, nil
}

// GetWorldStats returns world interaction statistics
func (s *advancedStatsService) GetWorldStats(ctx context.Context, guid string) (*models.WorldStats, error) {
	stats := &models.WorldStats{}

	err := s.ch.QueryRow(ctx, `
		SELECT 
			toInt64(countIf(event_type = 'ladder_mount')) as ladder_mounts,
			sumIf(JSONExtractFloat(raw_json, 'height_climbed', 'Float64'), event_type = 'ladder_dismount') as ladder_dist,
			toInt64(countIf(event_type = 'door_open')) as doors_opened,
			toInt64(countIf(event_type = 'door_close')) as doors_closed,
			toInt64(countIf(event_type = 'item_pickup')) as items_picked,
			toInt64(countIf(event_type = 'item_drop')) as items_dropped,
			toInt64(countIf(event_type = 'use')) as use_interactions,
			toInt64(countIf(event_type = 'chat')) as chat_messages,
			sumIf(JSONExtractInt(raw_json, 'fall_damage', 'Int64'), event_type = 'land') as fall_damage,
			toInt64(countIf(event_type = 'death' AND JSONExtractString(raw_json, 'mod') = 'MOD_FALLING')) as fall_deaths
		FROM raw_events
		WHERE actor_id = ?
	`, guid).Scan(
		&stats.LadderMounts, &stats.LadderDistance,
		&stats.DoorsOpened, &stats.DoorsClosed,
		&stats.ItemsPickedUp, &stats.ItemsDropped,
		&stats.UseInteractions, &stats.ChatMessages,
		&stats.FallDamage, &stats.FallDeaths,
	)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetBotStats returns bot-related statistics
func (s *advancedStatsService) GetBotStats(ctx context.Context, guid string) (*models.BotStats, error) {
	stats := &models.BotStats{}

	// Bot kills use the bot_killed event type
	// Deaths to bots currently not tracked (bots don't emit kill events when they kill players)
	err := s.ch.QueryRow(ctx, `
		SELECT 
			toInt64(countIf(event_type = 'bot_killed' AND actor_id = ?)) as bot_kills,
			toInt64(0) as deaths_to_bots,
			ifNotFinite(avgIf(distance, event_type = 'bot_killed' AND actor_id = ?), 0) as avg_dist
		FROM raw_events
		WHERE actor_id = ?
	`, guid, guid, guid).Scan(&stats.BotKills, &stats.DeathsToBots, &stats.AvgBotKillDist)
	if err != nil {
		return nil, err
	}

	if stats.DeathsToBots > 0 {
		stats.BotKDRatio = float64(stats.BotKills) / float64(stats.DeathsToBots)
	} else {
		stats.BotKDRatio = float64(stats.BotKills)
	}

	return stats, nil
}

// =============================================================================
// NESTED DRILLDOWNS & CONTEXTUAL LEADERBOARDS
// =============================================================================

// GetDrillDownNested returns a second-level breakdown
func (s *advancedStatsService) GetDrillDownNested(ctx context.Context, guid, stat, parentDim, parentValue, childDim string, limit int) ([]models.DrillDownItem, error) {
	if limit <= 0 {
		limit = 10
	}

	var parentCol, childCol string
	// Mapping dimensions to columns... simplified
	getCol := func(dim string) string {
		switch dim {
		case "weapon":
			return "actor_weapon"
		case "map":
			return "map_name"
		case "hour":
			return "toHour(timestamp)"
		case "day":
			return "toDayOfWeek(timestamp)"
		case "victim":
			return "target_name"
		case "hitloc":
			return "hitloc"
		default:
			return "actor_weapon"
		}
	}
	parentCol = getCol(parentDim)
	childCol = getCol(childDim)

	query := fmt.Sprintf(`
		SELECT 
			%s as child_val,
			toInt64(count()) as count
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND actor_id = ? AND %s = ? AND %s != ''
		GROUP BY child_val
		ORDER BY count DESC
		LIMIT ?
	`, childCol, parentCol, childCol)

	rows, err := s.ch.Query(ctx, query, guid, parentValue, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.DrillDownItem
	var total int64
	for rows.Next() {
		var item models.DrillDownItem
		if err := rows.Scan(&item.Label, &item.Value); err != nil {
			continue
		}
		items = append(items, item)
		total += item.Value
	}

	for i := range items {
		if total > 0 {
			items[i].Percentage = (float64(items[i].Value) / float64(total)) * 100
		}
	}
	return items, nil
}

// GetStatLeaders returns players ranked by a stat in a specific context (e.g. Best with MP40)
func (s *advancedStatsService) GetStatLeaders(ctx context.Context, stat, dimension, value string, limit int) ([]models.StatLeaderboardEntry, error) {
	if limit <= 0 {
		limit = 25
	}

	var filterCol string
	switch dimension {
	case "weapon":
		filterCol = "actor_weapon"
	case "map":
		filterCol = "map_name"
	case "time":
		filterCol = "toHour(timestamp)" // Value expected as string number
	default:
		filterCol = "map_name"
	}

	// Dynamic query construction
	// Assume stat is 'kills' for now, can expand later
	query := fmt.Sprintf(`
		SELECT 
			actor_id,
			any(actor_name) as name,
			toInt64(count()) as val
		FROM raw_events
		WHERE event_type IN ('player_kill', 'bot_killed') AND %s = ? AND actor_id != ''
		GROUP BY actor_id
		ORDER BY val DESC
		LIMIT ?
	`, filterCol)

	rows, err := s.ch.Query(ctx, query, value, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leaders []models.StatLeaderboardEntry
	rank := 1
	for rows.Next() {
		var id, name string
		var val int64
		if err := rows.Scan(&id, &name, &val); err != nil {
			continue
		}
		leaders = append(leaders, models.StatLeaderboardEntry{
			Rank:       rank,
			PlayerID:   id,
			PlayerName: name,
			Value:      float64(val),
		})
		rank++
	}
	return leaders, nil
}

// GetAvailableDrilldowns returns valid dimensions for a stat
func (s *advancedStatsService) GetAvailableDrilldowns(stat string) []string {
	// Static return for now
	return []string{"weapon", "map", "victim", "hitloc", "hour", "day"}
}
