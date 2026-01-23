package handlers

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"reflect"
)

// Redefining types locally as they are local in the original function
type PlayerAggMock struct {
	ID      string
	Name    string
	Metrics map[string]float64
}

var testCategories = []string{
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

func generatePlayers(n int) []PlayerAggMock {
	players := make([]PlayerAggMock, n)
	for i := 0; i < n; i++ {
		p := PlayerAggMock{
			ID:      fmt.Sprintf("player-%d", i),
			Name:    fmt.Sprintf("Player %d", i),
			Metrics: make(map[string]float64),
		}
		for _, cat := range testCategories {
			// Random value between 0 and 1000
			if rand.Float64() > 0.3 { // 70% chance of having a value
				p.Metrics[cat] = rand.Float64() * 1000
			}
		}
		players[i] = p
	}
	return players
}

func currentImplementation(players []PlayerAggMock) map[string][]map[string]interface{} {
	result := make(map[string][]map[string]interface{})

	for _, cat := range testCategories {
		// Flatten structure for sorting
		type entry struct {
			Name  string
			Value float64
			ID    string
		}
		list := make([]entry, 0, len(players))
		for _, p := range players {
			if val := p.Metrics[cat]; val > 0 {
				list = append(list, entry{Name: p.Name, Value: val, ID: p.ID})
			}
		}

		// Sort Descending
		sort.Slice(list, func(i, j int) bool {
			// Secondary sort by ID for deterministic results in tests
			if list[i].Value == list[j].Value {
				return list[i].ID < list[j].ID
			}
			return list[i].Value > list[j].Value
		})

		// Take top 3
		top3 := []map[string]interface{}{}
		for i := 0; i < 3 && i < len(list); i++ {
			valStr := fmt.Sprintf("%.0f", list[i].Value)
			top3 = append(top3, map[string]interface{}{
				"name":  list[i].Name,
				"value": valStr,
				"id":    list[i].ID,
			})
		}
		result[cat] = top3
	}
	return result
}

func optimizedImplementation(players []PlayerAggMock) map[string][]map[string]interface{} {
	result := make(map[string][]map[string]interface{})

	for _, cat := range testCategories {
		type entry struct {
			Name  string
			Value float64
			ID    string
		}

		// Maintain top 3
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
					if best[k].Value > best[k-1].Value || (best[k].Value == best[k-1].Value && best[k].ID < best[k-1].ID) {
						best[k], best[k-1] = best[k-1], best[k]
					}
				}
			} else if val > best[2].Value || (val == best[2].Value && p.ID < best[2].ID) {
				// Replace last one and shift up if needed
				best[2] = entry{Name: p.Name, Value: val, ID: p.ID}
				if best[2].Value > best[1].Value || (best[2].Value == best[1].Value && best[2].ID < best[1].ID) {
					best[2], best[1] = best[1], best[2]
					if best[1].Value > best[0].Value || (best[1].Value == best[0].Value && best[1].ID < best[0].ID) {
						best[1], best[0] = best[0], best[1]
					}
				}
			}
		}

		// Take top 3 (or less if not enough players)
		top3 := []map[string]interface{}{}
		for i := 0; i < count; i++ {
			valStr := fmt.Sprintf("%.0f", best[i].Value)
			top3 = append(top3, map[string]interface{}{
				"name":  best[i].Name,
				"value": valStr,
				"id":    best[i].ID,
			})
		}
		result[cat] = top3
	}
	return result
}

func BenchmarkLeaderboardCurrent(b *testing.B) {
	players := generatePlayers(1000)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		currentImplementation(players)
	}
}

func BenchmarkLeaderboardOptimized(b *testing.B) {
	players := generatePlayers(1000)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		optimizedImplementation(players)
	}
}

func TestLeaderboardCorrectness(t *testing.T) {
	// Use deterministic seed
	rand.Seed(42)
	players := generatePlayers(1000)

	expected := currentImplementation(players)
	actual := optimizedImplementation(players)

	for _, cat := range testCategories {
		expList := expected[cat]
		actList := actual[cat]

		if len(expList) != len(actList) {
			t.Errorf("Category %s: expected length %d, got %d", cat, len(expList), len(actList))
			continue
		}

		for i := 0; i < len(expList); i++ {
			if !reflect.DeepEqual(expList[i], actList[i]) {
				t.Errorf("Category %s, Rank %d: expected %v, got %v", cat, i+1, expList[i], actList[i])
			}
		}
	}
}
