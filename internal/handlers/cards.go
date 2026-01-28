package handlers

import (
	"fmt"
	"net/http"
)

// GetLeaderboardCards returns the Top 3 players for ALL 40 dashboard categories
// This uses a single massive aggregation query for performance
func (h *Handler) GetLeaderboardCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Aggregation Query - using correct event types from seeder
	// Deaths are counted as kills where player is target_id, so we use a CTE
	query := `
		WITH deaths_cte AS (
			SELECT target_id as player_id, count() as death_count
			FROM mohaa_stats.raw_events
			WHERE event_type = 'kill' AND target_id != '' AND target_id != 'world'
			GROUP BY target_id
		)
		SELECT 
			a.actor_id,
			anyLast(a.actor_name) as name,
			
			-- A. Lethality & Combat (using correct event types: kill, headshot)
			countIf(a.event_type = 'kill') as kills,
			ifNull(max(d.death_count), 0) as deaths,
			countIf(a.event_type = 'headshot') as headshots,
			countIf(a.event_type = 'weapon_fire') as shots_fired,
			countIf(a.event_type = 'weapon_hit') as shots_hit,
			sumIf(a.damage, a.event_type = 'damage') as total_damage,
			countIf(a.event_type IN ('player_bash', 'bash')) as bash_kills,
			countIf(a.event_type IN ('grenade_throw', 'explosion', 'grenade_explode')) as grenade_kills,
			countIf(a.event_type IN ('player_roadkill', 'roadkill')) as roadkills,
			countIf(a.event_type = 'player_telefragged') as telefrags,
			countIf(a.event_type IN ('player_crushed', 'crushed')) as crushed,
			countIf(a.event_type IN ('player_teamkill', 'teamkill')) as teamkills,
			countIf(a.event_type IN ('player_suicide', 'suicide')) as suicides,
			countIf(a.event_type IN ('player_spawn', 'spawn')) as mystery_kills,

			-- B. Weapon Handling
			countIf(a.event_type IN ('weapon_reload', 'reload')) as reloads,
			countIf(a.event_type IN ('weapon_change', 'weapon_swap')) as weapon_swaps,
			countIf(a.event_type = 'weapon_no_ammo') as no_ammo,
			countIf(a.event_type = 'item_pickup') as looter,

			-- C. Movement
			sumIf(JSONExtractFloat(a.raw_json, 'walked'), a.event_type = 'distance') as walked,
			sumIf(JSONExtractFloat(a.raw_json, 'sprinted'), a.event_type = 'distance') as sprinted,
			sumIf(JSONExtractFloat(a.raw_json, 'swam'), a.event_type = 'distance') as swam,
			sumIf(JSONExtractFloat(a.raw_json, 'driven'), a.event_type = 'distance') as driven,
			countIf(a.event_type = 'jump') as jumps,
			countIf(a.event_type = 'crouch') as crouch_events,
			countIf(a.event_type = 'prone') as prone_events,
			countIf(a.event_type = 'ladder_mount') as ladders,

			-- D. Survival & Items
			countIf(a.event_type = 'health_pickup') as health_picked,
			countIf(a.event_type = 'ammo_pickup') as ammo_picked,
			countIf(a.event_type = 'armor_pickup') as armor_picked,
			countIf(a.event_type = 'item_pickup') as items_picked,

			-- E. Objectives & Game Flow
			countIf(a.event_type = 'match_outcome' AND a.match_outcome = 1) as wins,
			countIf(a.event_type = 'match_outcome' AND a.match_outcome = 1 AND a.actor_weapon = 'dm') as ffa_wins,
			countIf(a.event_type = 'match_outcome' AND a.match_outcome = 1 AND a.actor_weapon != 'dm') as team_wins,
			countIf(a.event_type IN ('objective_update', 'objective_capture')) as objectives_done,
			countIf(a.event_type IN ('round_end', 'round_start')) as rounds_played,
			countIf(a.event_type = 'match_outcome') as games_finished,

			-- F. Vehicles
			countIf(a.event_type IN ('vehicle_enter', 'turret_enter')) as vehicle_enter,
			countIf(a.event_type = 'turret_enter') as turret_enter,
			countIf(a.event_type = 'kill' AND a.actor_id = 'vehicle') as vehicle_kills,

			-- G. Social & Misc
			countIf(a.event_type IN ('chat', 'player_say')) as chat_msgs,
			countIf(a.event_type = 'player_spectate') as spectating,
			countIf(a.event_type = 'door_open') as doors_opened,
			
			-- H. Creative Stats
			countIf(a.event_type IN ('ladder_mount', 'jump')) as verticality,
			uniqIf(a.actor_weapon, a.event_type IN ('kill', 'player_bash', 'bash')) as unique_weapon_kills,
			countIf(a.event_type = 'item_drop') as items_dropped,
			countIf(a.event_type = 'vehicle_collision') as vehicle_collisions,
			countIf(a.event_type = 'bot_killed') as bot_kills,

            -- Movement specific
            sumIf(a.distance, a.event_type = 'distance') as total_distance,
            countIf(a.event_type = 'weapon_reload') as reload_count,
            countIf(a.event_type = 'ladder_mount') as ladder_mounts,
            countIf(a.event_type = 'crouch') as manual_crouches

		FROM mohaa_stats.raw_events a
		LEFT JOIN deaths_cte d ON a.actor_id = d.player_id
		WHERE a.actor_id != 'world' AND a.actor_id != ''
		GROUP BY a.actor_id
		HAVING countIf(a.event_type = 'kill') > 0 OR max(d.death_count) > 0 OR countIf(a.event_type = 'weapon_fire') > 0
	`

	rows, err := h.ch.Query(ctx, query)
	if err != nil {
		h.logger.Errorw("Leaderboard cards query failed", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	// 2. Scan Results
	type PlayerAgg struct {
		ID      string
		Name    string
		Metrics map[string]float64
	}

	players := []PlayerAgg{}

	for rows.Next() {
		var p PlayerAgg
		p.Metrics = make(map[string]float64)

		var (
			kills, deaths, headshots, shotsFired, shotsHit, damage, bash, nade, road, tele, crush, tk, self, mystery uint64
			reloads, swaps, noAmmo, looter                                                                           uint64
			walked, sprinted, swam, driven                                                                           float64
			jumps, crouch, prone, ladders                                                                            uint64
			health, ammo, armor, items                                                                               uint64
			wins, ffaWins, teamWins, obj, rounds, games                                                              uint64
			vEnter, tEnter, vKills                                                                                   uint64
			chat, spec, doors                                                                                        uint64
			verticality, uniqueWeapons, itemsDropped, vehicleCollisions, botKills                                    uint64
			totalDistance                                                                                            float64
			reloadCnt, ladMnt, manCrouch                                                                             uint64
		)

		if err := rows.Scan(
			&p.ID, &p.Name,
			&kills, &deaths, &headshots, &shotsFired, &shotsHit, &damage, &bash, &nade, &road, &tele, &crush, &tk, &self, &mystery,
			&reloads, &swaps, &noAmmo, &looter,
			&walked, &sprinted, &swam, &driven, &jumps, &crouch, &prone, &ladders,
			&health, &ammo, &armor, &items,
			&wins, &ffaWins, &teamWins, &obj, &rounds, &games,
			&vEnter, &tEnter, &vKills,
			&chat, &spec, &doors,
			&verticality, &uniqueWeapons, &itemsDropped, &vehicleCollisions, &botKills,
			&totalDistance, &reloadCnt, &ladMnt, &manCrouch,
		); err != nil {
			h.logger.Errorw("Row scan failed", "error", err)
			continue
		}

		// Basic Metrics - convert uint64 to float64
		p.Metrics["kills"] = float64(kills)
		p.Metrics["deaths"] = float64(deaths)
		p.Metrics["headshots"] = float64(headshots)
		p.Metrics["shots_fired"] = float64(shotsFired)
		p.Metrics["shots_hit"] = float64(shotsHit)
		p.Metrics["damage"] = float64(damage)
		p.Metrics["bash_kills"] = float64(bash)
		p.Metrics["grenade_kills"] = float64(nade)
		p.Metrics["roadkills"] = float64(road)
		p.Metrics["vehicle_kills"] = float64(vKills)
		p.Metrics["telefrags"] = float64(tele)
		p.Metrics["crushed"] = float64(crush)
		p.Metrics["teamkills"] = float64(tk)
		p.Metrics["suicides"] = float64(self)

		// Derived Combat Metrics
		p.Metrics["kd"] = float64(kills)
		if deaths > 0 {
			p.Metrics["kd"] = float64(kills) / float64(deaths)
		}

		p.Metrics["accuracy"] = 0
		if shotsFired > 0 {
			p.Metrics["accuracy"] = (float64(shotsHit) / float64(shotsFired)) * 100
		}

		p.Metrics["headshot_ratio"] = 0
		if kills > 0 {
			p.Metrics["headshot_ratio"] = (float64(headshots) / float64(kills)) * 100
		}

		// Creative / Fun Stats
		p.Metrics["trigger_happy"] = float64(shotsFired) // Most shots fired
		p.Metrics["stormtrooper"] = float64(shotsFired)  // Most shots missed (if accuracy low)
		if shotsFired > 100 && p.Metrics["accuracy"] < 15 {
			p.Metrics["stormtrooper"] = float64(shotsFired) // High volume, low aim
		} else {
			p.Metrics["stormtrooper"] = 0
		}

		p.Metrics["pacifist"] = 0
		if kills == 0 && games > 0 {
			p.Metrics["pacifist"] = float64(games) // Games played without killing
		}

		p.Metrics["executioner"] = float64(headshots)
		p.Metrics["gravedigger"] = float64(bash)
		p.Metrics["demolitionist"] = float64(nade)

		// New Creative Stats
		p.Metrics["verticality"] = float64(verticality)
		p.Metrics["swiss_army_knife"] = float64(uniqueWeapons)
		p.Metrics["the_architect"] = float64(itemsDropped)
		p.Metrics["road_rage"] = float64(vehicleCollisions)
		p.Metrics["bot_bully"] = float64(botKills)
		if kills > 0 {
			p.Metrics["bot_bully_ratio"] = float64(botKills) / float64(kills)
		}

		p.Metrics["wins"] = float64(wins)
		p.Metrics["ffa_wins"] = float64(ffaWins)
		p.Metrics["team_wins"] = float64(teamWins)
		p.Metrics["objectives_done"] = float64(obj)
		p.Metrics["rounds_played"] = float64(rounds)
		p.Metrics["games_finished"] = float64(games)

		// Extended Stats (Phase 2)
		p.Metrics["butterfingers"] = 0
		p.Metrics["ocd_reloading"] = float64(reloadCnt)
		p.Metrics["fireman"] = float64(ladMnt)
		p.Metrics["sneaky"] = float64(manCrouch)
		p.Metrics["chatterbox"] = float64(chat)

		// Movement - use totalDistance from query
		p.Metrics["distance"] = totalDistance
		p.Metrics["sprinted"] = sprinted
		p.Metrics["swam"] = swam
		p.Metrics["driven"] = driven
		p.Metrics["jumps"] = float64(jumps)
		p.Metrics["ladders"] = float64(ladders)
		p.Metrics["marathon"] = totalDistance

		p.Metrics["bunny_hopper"] = float64(jumps)
		p.Metrics["camper"] = 0
		if kills > 5 && totalDistance < 1000 { // High kills, low movement
			p.Metrics["camper"] = float64(kills)
		}

		// Items & Survival
		p.Metrics["health_picked"] = float64(health)
		p.Metrics["ammo_picked"] = float64(ammo)
		p.Metrics["armor_picked"] = float64(armor)
		p.Metrics["items_picked"] = float64(items)
		p.Metrics["medic"] = float64(health) // Most health picked up
		p.Metrics["loot_goblin"] = float64(items)

		// Social & Misc
		p.Metrics["watcher"] = float64(spec)
		p.Metrics["door_opener"] = float64(doors)

		players = append(players, p)
	}

	// 3. Process Top 3 for each category
	categories := []string{
		"kills", "deaths", "kd", "headshots", "accuracy", "headshot_ratio",
		"damage", "bash_kills", "grenade_kills", "roadkills", "telefrags", "crushed", "teamkills", "suicides",
		"executioner", "trigger_happy", "stormtrooper", "gravedigger", "demolitionist",
		"reloads", "weapon_swaps", "no_ammo", "looter",
		"distance", "sprinted", "swam", "driven", "jumps", "ladders",
		"marathon", "bunny_hopper", "camper",
		"health_picked", "ammo_picked", "armor_picked", "items_picked", "medic", "loot_goblin",
		"wins", "ffa_wins", "team_wins", "objectives_done", "rounds_played", "games_finished", "pacifist",
		"vehicle_enter", "turret_enter", "vehicle_kills",
		"chat_msgs", "spectating", "doors_opened", "watcher", "door_opener",
		"verticality", "swiss_army_knife", "the_architect", "road_rage", "bot_bully",
		"butterfingers", "ocd_reloading", "fireman", "sneaky", "chatterbox",
	}

	result := make(map[string][]map[string]interface{})

	for _, cat := range categories {
		type entry struct {
			Name  string
			Value float64
			ID    string
		}

		var best [3]entry
		count := 0

		for _, p := range players {
			val := p.Metrics[cat]
			if val <= 0 {
				continue
			}

			if count < 3 {
				best[count] = entry{Name: p.Name, Value: val, ID: p.ID}
				count++
				// Keep sorted descending
				for k := count - 1; k > 0; k-- {
					if best[k].Value > best[k-1].Value {
						best[k], best[k-1] = best[k-1], best[k]
					}
				}
			} else if val > best[2].Value {
				// Replace last one and shift up if needed
				best[2] = entry{Name: p.Name, Value: val, ID: p.ID}
				if best[2].Value > best[1].Value {
					best[2], best[1] = best[1], best[2]
					if best[1].Value > best[0].Value {
						best[1], best[0] = best[0], best[1]
					}
				}
			}
		}

		// Take top 3
		top3 := []map[string]interface{}{}
		for i := 0; i < count; i++ {
			valStr := fmt.Sprintf("%.0f", best[i].Value)

			// Formatting rules
			switch cat {
			case "kd":
				valStr = fmt.Sprintf("%.2f", best[i].Value)
			case "accuracy", "headshot_ratio":
				valStr = fmt.Sprintf("%.1f%%", best[i].Value)
			case "distance", "sprinted", "swam", "driven", "marathon":
				valStr = fmt.Sprintf("%.0fm", best[i].Value)
			case "playtime", "spectating", "watcher":
				// assuming events are just counts for now, but if time:
				// valStr = fmt.Sprintf("%.1fh", best[i].Value / 3600)
			}

			top3 = append(top3, map[string]interface{}{
				"name":  best[i].Name,
				"value": valStr,
				"id":    best[i].ID,
			})
		}
		result[cat] = top3
	}

	h.jsonResponse(w, http.StatusOK, result)
}
