package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

const (
	APIURL      = "http://localhost:8080"
	ServerID    = "test-server-1"
	ServerToken = "test-token-123"
)

// Test all 92 event types can be ingested
func TestAllEventTypes(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}

	eventTypes := []string{
		// Game Flow (11)
		"server_init", "server_start", "server_shutdown", "server_spawned",
		"map_load_start", "map_load_end", "map_change_start", "map_change_end",
		"game_init", "game_start", "game_end",

		// Match & Round Flow (11)
		"match_start", "match_end", "round_start", "round_end",
		"warmup_start", "warmup_end", "intermission_start", "intermission_end",
		"team_win", "team_draw", "team_stalemate",

		// Combat Core (10)
		"player_kill", "player_death", "player_damage", "player_pain",
		"player_headshot", "player_suicide", "player_bash", "player_roadkill",
		"player_telefragged", "player_respawn",

		// Weapons (9)
		"weapon_fire", "weapon_hit", "weapon_pickup", "weapon_drop",
		"weapon_ready", "weapon_reload", "weapon_reload_done",
		"weapon_swap", "weapon_switch",

		// Grenades (4)
		"grenade_throw", "grenade_explode", "grenade_pickup", "explosion",

		// Movement (10)
		"player_jump", "player_land", "player_crouch", "player_prone",
		"player_stand", "player_sprint_start", "player_sprint_end",
		"player_distance", "ladder_mount", "ladder_unmount",

		// Items (5)
		"item_pickup", "item_drop", "health_pickup", "ammo_pickup", "armor_pickup",

		// Vehicles (6)
		"vehicle_enter", "vehicle_exit", "vehicle_death",
		"turret_enter", "turret_exit", "turret_fire",

		// Objectives (2)
		"objective_capture", "objective_complete",

		// World Interaction (3)
		"door_open", "door_close", "world_explosion",

		// Bots/AI (7)
		"bot_spawn", "bot_killed", "bot_roam", "bot_curious",
		"bot_attack", "bot_grenade", "bot_pain",

		// Client Connection (5)
		"client_connect", "client_disconnect", "client_userinfo",
		"client_team_change", "player_use_object",

		// Advanced Interactions (4)
		"player_use_object_start", "player_use_object_finish",
		"player_push", "player_chat",

		// Voting (3)
		"vote_start", "vote_passed", "vote_failed",

		// Chat & Social (2)
		"player_say", "player_say_team",

		// Score & Placement (2)
		"player_score", "player_placement",
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			event := createTestEvent(eventType)
			if err := sendEvent(client, event); err != nil {
				t.Errorf("Failed to send %s event: %v", eventType, err)
			}
		})
	}
}

// Test combat sequence flow
func TestCombatSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-combat"
	now := float64(time.Now().Unix())

	// Sequence: fire → hit → pain → kill → death → respawn
	sequence := []map[string]interface{}{
		{
			"type":           "weapon_fire",
			"timestamp":      now,
			"match_id":       matchID,
			"player_name":    "TestPlayer1",
			"player_guid":    "test-guid-1",
			"weapon":         "Kar98",
			"ammo_remaining": 5,
		},
		{
			"type":        "weapon_hit",
			"timestamp":   now + 0.1,
			"match_id":    matchID,
			"player_name": "TestPlayer1",
			"target_name": "TestPlayer2",
			"weapon":      "Kar98",
			"hitloc":      "head",
		},
		{
			"type":          "player_pain",
			"timestamp":     now + 0.15,
			"match_id":      matchID,
			"player_name":   "TestPlayer2",
			"attacker_name": "TestPlayer1",
			"damage":        100,
			"hitloc":        "head",
		},
		{
			"type":          "player_headshot",
			"timestamp":     now + 0.2,
			"match_id":      matchID,
			"attacker_name": "TestPlayer1",
			"victim_name":   "TestPlayer2",
			"weapon":        "Kar98",
		},
		{
			"type":          "player_kill",
			"timestamp":     now + 0.25,
			"match_id":      matchID,
			"attacker_name": "TestPlayer1",
			"victim_name":   "TestPlayer2",
			"weapon":        "Kar98",
			"hitloc":        "head",
		},
		{
			"type":        "player_death",
			"timestamp":   now + 0.3,
			"match_id":    matchID,
			"player_name": "TestPlayer2",
		},
		{
			"type":        "player_respawn",
			"timestamp":   now + 5.0,
			"match_id":    matchID,
			"player_name": "TestPlayer2",
			"pos_x":       100.5,
			"pos_y":       200.3,
			"pos_z":       50.0,
		},
	}

	for _, event := range sequence {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send event %s in sequence: %v", event["type"], err)
		}
		time.Sleep(50 * time.Millisecond) // Small delay between events
	}
}

// Test vehicle interaction flow
func TestVehicleSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-vehicle"
	now := float64(time.Now().Unix())

	// Sequence: enter → roadkill → exit
	events := []map[string]interface{}{
		{
			"type":        "vehicle_enter",
			"timestamp":   now,
			"match_id":    matchID,
			"player_name": "TankDriver",
			"vehicle":     "Sherman Tank",
		},
		{
			"type":          "player_roadkill",
			"timestamp":     now + 10.0,
			"match_id":      matchID,
			"attacker_name": "TankDriver",
			"victim_name":   "Victim1",
		},
		{
			"type":        "vehicle_exit",
			"timestamp":   now + 20.0,
			"match_id":    matchID,
			"player_name": "TankDriver",
			"vehicle":     "Sherman Tank",
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send vehicle event: %v", err)
		}
	}
}

// Test grenade sequence
func TestGrenadeSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-grenade"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":        "grenade_throw",
			"timestamp":   now,
			"match_id":    matchID,
			"player_name": "Grenadier",
			"projectile":  "grenade",
		},
		{
			"type":        "grenade_explode",
			"timestamp":   now + 3.0,
			"match_id":    matchID,
			"player_name": "Grenadier",
			"projectile":  "grenade",
		},
		{
			"type":          "explosion",
			"timestamp":     now + 3.0,
			"match_id":      matchID,
			"attacker_name": "Grenadier",
			"damage":        75,
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send grenade event: %v", err)
		}
	}
}

// Test bot interaction
func TestBotSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-bot"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":      "bot_spawn",
			"timestamp": now,
			"match_id":  matchID,
			"bot_id":    "bot_1",
		},
		{
			"type":      "bot_roam",
			"timestamp": now + 5.0,
			"match_id":  matchID,
			"bot_id":    "bot_1",
		},
		{
			"type":      "bot_attack",
			"timestamp": now + 10.0,
			"match_id":  matchID,
			"bot_id":    "bot_1",
		},
		{
			"type":          "bot_killed",
			"timestamp":     now + 15.0,
			"match_id":      matchID,
			"bot_id":        "bot_1",
			"attacker_name": "BotHunter",
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send bot event: %v", err)
		}
	}
}

// Test movement patterns
func TestMovementSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-movement"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":        "player_jump",
			"timestamp":   now,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_land",
			"timestamp":   now + 0.8,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_crouch",
			"timestamp":   now + 2.0,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_prone",
			"timestamp":   now + 4.0,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_stand",
			"timestamp":   now + 6.0,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_sprint_start",
			"timestamp":   now + 8.0,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_sprint_end",
			"timestamp":   now + 12.0,
			"match_id":    matchID,
			"player_name": "Jumper",
		},
		{
			"type":        "player_distance",
			"timestamp":   now + 12.5,
			"match_id":    matchID,
			"player_name": "Jumper",
			"walked":      50.0,
			"sprinted":    100.0,
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send movement event: %v", err)
		}
	}
}

// Test voting system
func TestVoteSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-vote"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":        "vote_start",
			"timestamp":   now,
			"match_id":    matchID,
			"player_name": "Voter",
			"vote_name":   "map_change",
			"vote_string": "Change to dm/mohdm3",
		},
		{
			"type":      "vote_passed",
			"timestamp": now + 30.0,
			"match_id":  matchID,
			"vote_name": "map_change",
			"yes_count": 8,
			"no_count":  2,
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send vote event: %v", err)
		}
	}
}

// Test objective gameplay
func TestObjectiveSequence(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-objective"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":        "player_use_object_start",
			"timestamp":   now,
			"match_id":    matchID,
			"player_name": "ObjPlayer",
			"object":      "obj_1",
		},
		{
			"type":        "player_use_object_finish",
			"timestamp":   now + 8.0,
			"match_id":    matchID,
			"player_name": "ObjPlayer",
			"object":      "obj_1",
		},
		{
			"type":      "objective_capture",
			"timestamp": now + 8.5,
			"match_id":  matchID,
			"objective": "obj_1",
			"team":      "allies",
		},
		{
			"type":      "objective_complete",
			"timestamp": now + 60.0,
			"match_id":  matchID,
			"objective": "obj_1",
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send objective event: %v", err)
		}
	}
}

// Test world interactions
func TestWorldInteraction(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-world"
	now := float64(time.Now().Unix())

	events := []map[string]interface{}{
		{
			"type":        "door_open",
			"timestamp":   now,
			"match_id":    matchID,
			"door":        "door_1",
			"player_name": "DoorUser",
		},
		{
			"type":      "door_close",
			"timestamp": now + 5.0,
			"match_id":  matchID,
			"door":      "door_1",
		},
		{
			"type":      "world_explosion",
			"timestamp": now + 10.0,
			"match_id":  matchID,
			"damage":    100,
		},
	}

	for _, event := range events {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send world event: %v", err)
		}
	}
}

// Test full match lifecycle
func TestMatchLifecycle(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	matchID := "test-match-lifecycle"
	now := float64(time.Now().Unix())

	lifecycle := []map[string]interface{}{
		// Server initialization
		{"type": "server_init", "timestamp": now},
		{"type": "server_start", "timestamp": now + 1},
		{"type": "server_spawned", "timestamp": now + 2},

		// Map loading
		{"type": "map_load_start", "timestamp": now + 3, "map": "dm/mohdm1"},
		{"type": "map_load_end", "timestamp": now + 5, "map": "dm/mohdm1"},

		// Match start
		{"type": "match_start", "timestamp": now + 6, "match_id": matchID},
		{"type": "warmup_start", "timestamp": now + 7, "match_id": matchID},
		{"type": "warmup_end", "timestamp": now + 37, "match_id": matchID},

		// Game flow
		{"type": "game_init", "timestamp": now + 38, "match_id": matchID},
		{"type": "game_start", "timestamp": now + 39, "match_id": matchID},
		{"type": "round_start", "timestamp": now + 40, "match_id": matchID, "round": 1},

		// Gameplay (abbreviated)
		{"type": "client_connect", "timestamp": now + 41, "player_name": "Player1", "player_guid": "guid1"},
		{"type": "client_connect", "timestamp": now + 42, "player_name": "Player2", "player_guid": "guid2"},

		// Round end
		{"type": "round_end", "timestamp": now + 300, "match_id": matchID, "round": 1},
		{"type": "team_win", "timestamp": now + 301, "match_id": matchID, "team": "allies"},

		// Match end
		{"type": "game_end", "timestamp": now + 302, "match_id": matchID},
		{"type": "match_end", "timestamp": now + 303, "match_id": matchID},
		{"type": "intermission_start", "timestamp": now + 304, "match_id": matchID},
		{"type": "intermission_end", "timestamp": now + 334, "match_id": matchID},

		// Disconnect
		{"type": "client_disconnect", "timestamp": now + 335, "player_name": "Player1"},
		{"type": "client_disconnect", "timestamp": now + 336, "player_name": "Player2"},
	}

	for _, event := range lifecycle {
		if err := sendEvent(client, event); err != nil {
			t.Errorf("Failed to send lifecycle event %s: %v", event["type"], err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Helper functions
func createTestEvent(eventType string) map[string]interface{} {
	baseEvent := map[string]interface{}{
		"type":         eventType,
		"timestamp":    float64(time.Now().Unix()),
		"server_id":    ServerID,
		"server_token": ServerToken,
	}

	// Add type-specific required fields
	switch eventType {
	case "player_kill", "player_headshot", "player_bash", "player_roadkill":
		baseEvent["match_id"] = "test-match"
		baseEvent["attacker_name"] = "Attacker"
		baseEvent["attacker_guid"] = "test-attacker-guid"
		baseEvent["victim_name"] = "Victim"
		baseEvent["victim_guid"] = "test-victim-guid"
		baseEvent["weapon"] = "Kar98"

	case "player_death", "player_respawn":
		baseEvent["match_id"] = "test-match"
		baseEvent["player_name"] = "TestPlayer"
		baseEvent["player_guid"] = "test-guid"

	case "weapon_fire", "weapon_reload":
		baseEvent["match_id"] = "test-match"
		baseEvent["player_name"] = "TestPlayer"
		baseEvent["weapon"] = "Kar98"

	case "vehicle_enter", "vehicle_exit":
		baseEvent["match_id"] = "test-match"
		baseEvent["player_name"] = "TestPlayer"
		baseEvent["vehicle"] = "Sherman Tank"

	case "objective_capture":
		baseEvent["match_id"] = "test-match"
		baseEvent["objective"] = "obj_1"
		baseEvent["team"] = "allies"

	case "bot_spawn", "bot_killed":
		baseEvent["match_id"] = "test-match"
		baseEvent["bot_id"] = "bot_1"

	case "vote_start":
		baseEvent["match_id"] = "test-match"
		baseEvent["player_name"] = "Voter"
		baseEvent["vote_name"] = "map_change"
		baseEvent["vote_string"] = "Change map"

	case "door_open", "door_close":
		baseEvent["match_id"] = "test-match"
		baseEvent["door"] = "door_1"

	case "map_load_start", "map_load_end":
		baseEvent["map"] = "dm/mohdm1"

	case "match_start", "match_end":
		baseEvent["match_id"] = "test-match"
	}

	return baseEvent
}

func sendEvent(client *http.Client, event map[string]interface{}) error {
	// Ensure server credentials are set
	if _, ok := event["server_id"]; !ok {
		event["server_id"] = ServerID
	}
	if _, ok := event["server_token"]; !ok {
		event["server_token"] = ServerToken
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", APIURL+"/ingest/events", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("request creation error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ServerToken)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
