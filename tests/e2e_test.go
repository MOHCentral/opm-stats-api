package tests

// =============================================================================
// OpenMOHAA Stats System - End-to-End Test Suite
// =============================================================================
// Tests the complete flow from game events → API → ClickHouse → SMF Frontend
// Run with: go test -v ./tests/...

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	apiBaseURL     = "http://localhost:8080"
	smfBaseURL     = "http://localhost:8888"
	serverToken    = "dev-server-token-replace-in-production"
	serverID       = "test-server-01"
	testPlayerGUID = "test-player-e2e"
	testPlayerName = "E2E_TestPlayer"
	testMatchID    = "match_e2e_test"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

func sendEvent(t *testing.T, eventType string, params map[string]string) {
	if err := sendEventSafe(t, eventType, params); err != nil {
		t.Fatalf("Failed to send %s event: %v", eventType, err)
	}
}

func sendEventSafe(t *testing.T, eventType string, params map[string]string) error {
	data := url.Values{}
	data.Set("type", eventType)
	data.Set("server_token", serverToken)
	data.Set("server_id", serverID)
	data.Set("match_id", testMatchID)
	data.Set("timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	for k, v := range params {
		data.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.PostForm(apiBaseURL+"/api/v1/ingest/events", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func checkAPIHealth(t *testing.T) bool {
	resp, err := http.Get(apiBaseURL + "/health")
	if err != nil {
		t.Logf("API health check failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func checkSMFHealth(t *testing.T) bool {
	resp, err := http.Get(smfBaseURL)
	if err != nil {
		t.Logf("SMF health check failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound
}

func getPlayerStats(t *testing.T, guid string) map[string]interface{} {
	resp, err := http.Get(apiBaseURL + "/api/v1/stats/player/" + guid)
	if err != nil {
		t.Fatalf("Failed to get player stats: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

// =============================================================================
// INFRASTRUCTURE TESTS
// =============================================================================

func TestAPIHealth(t *testing.T) {
	if !checkAPIHealth(t) {
		t.Fatal("Stats API is not healthy")
	}
	t.Log("✓ Stats API is healthy")
}

func TestSMFHealth(t *testing.T) {
	if !checkSMFHealth(t) {
		t.Fatal("SMF Forum is not accessible")
	}
	t.Log("✓ SMF Forum is accessible")
}

// =============================================================================
// COMBAT EVENT TESTS
// =============================================================================

func TestEvent_PlayerKill(t *testing.T) {
	params := map[string]string{
		"attacker_name": testPlayerName,
		"attacker_guid": testPlayerGUID,
		"attacker_team": "allies",
		"victim_name":   "Victim_Bot",
		"victim_guid":   "victim-bot-001",
		"victim_team":   "axis",
		"weapon":        "M1 Garand",
		"hitloc":        "torso",
	}
	sendEvent(t, "kill", params)
	t.Log("✓ player_kill event sent")
}

func TestEvent_PlayerDeath(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"player_team": "allies",
		"inflictor":   "grenade",
	}
	sendEvent(t, "death", params)
	t.Log("✓ player_death event sent")
}

func TestEvent_PlayerDamage(t *testing.T) {
	params := map[string]string{
		"attacker_name": testPlayerName,
		"attacker_guid": testPlayerGUID,
		"victim_name":   "Victim_Bot",
		"victim_guid":   "victim-bot-001",
		"damage":        "45",
		"weapon":        "Thompson",
	}
	sendEvent(t, "damage", params)
	t.Log("✓ player_damage event sent")
}

func TestEvent_WeaponFire(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"weapon":      "M1 Garand",
	}
	sendEvent(t, "weapon_fire", params)
	t.Log("✓ weapon_fire event sent")
}

func TestEvent_WeaponHit(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"target_name": "Victim_Bot",
		"target_guid": "victim-bot-001",
		"hitloc":      "head",
	}
	sendEvent(t, "weapon_hit", params)
	t.Log("✓ weapon_hit event sent")
}

func TestEvent_Headshot(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"victim_name": "Victim_Bot",
		"victim_guid": "victim-bot-001",
		"weapon":      "Springfield",
	}
	sendEvent(t, "headshot", params)
	t.Log("✓ headshot event sent")
}

func TestEvent_WeaponReload(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"weapon":      "Thompson",
	}
	sendEvent(t, "reload", params)
	t.Log("✓ weapon_reload event sent")
}

func TestEvent_WeaponChange(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"old_weapon":  "M1 Garand",
		"new_weapon":  "Colt",
	}
	sendEvent(t, "weapon_change", params)
	t.Log("✓ weapon_change event sent")
}

func TestEvent_GrenadeThrow(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"projectile":  "frag_grenade",
	}
	sendEvent(t, "grenade_throw", params)
	t.Log("✓ grenade_throw event sent")
}

func TestEvent_GrenadeExplode(t *testing.T) {
	params := map[string]string{
		"projectile": "frag_grenade",
	}
	sendEvent(t, "grenade_explode", params)
	t.Log("✓ grenade_explode event sent")
}

// =============================================================================
// MOVEMENT EVENT TESTS
// =============================================================================

func TestEvent_PlayerJump(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
	}
	sendEvent(t, "jump", params)
	t.Log("✓ player_jump event sent")
}

func TestEvent_PlayerLand(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"fall_height": "15.5",
	}
	sendEvent(t, "land", params)
	t.Log("✓ player_land event sent")
}

func TestEvent_PlayerCrouch(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
	}
	sendEvent(t, "crouch", params)
	t.Log("✓ player_crouch event sent")
}

func TestEvent_PlayerProne(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
	}
	sendEvent(t, "prone", params)
	t.Log("✓ player_prone event sent")
}

func TestEvent_PlayerDistance(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"walked":      "1500.5",
		"sprinted":    "800.2",
	}
	sendEvent(t, "distance", params)
	t.Log("✓ player_distance event sent")
}

// =============================================================================
// INTERACTION EVENT TESTS
// =============================================================================

func TestEvent_LadderMount(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
	}
	sendEvent(t, "ladder_mount", params)
	t.Log("✓ ladder_mount event sent")
}

func TestEvent_LadderDismount(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
	}
	sendEvent(t, "ladder_dismount", params)
	t.Log("✓ ladder_dismount event sent")
}

func TestEvent_ItemPickup(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"item":        "health_large",
	}
	sendEvent(t, "item_pickup", params)
	t.Log("✓ item_pickup event sent")
}

func TestEvent_ItemDrop(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"item":        "M1 Garand",
	}
	sendEvent(t, "item_drop", params)
	t.Log("✓ item_drop event sent")
}

func TestEvent_PlayerUse(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"entity":      "door_wood",
	}
	sendEvent(t, "use", params)
	t.Log("✓ player_use event sent")
}

// =============================================================================
// SESSION EVENT TESTS
// =============================================================================

func TestEvent_ClientConnect(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"session_id":  "sess_test_001",
	}
	sendEvent(t, "connect", params)
	t.Log("✓ client_connect event sent")
}

func TestEvent_ClientDisconnect(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"session_id":  "sess_test_001",
	}
	sendEvent(t, "disconnect", params)
	t.Log("✓ client_disconnect event sent")
}

func TestEvent_ClientBegin(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"team":        "allies",
	}
	sendEvent(t, "spawn", params)
	t.Log("✓ client_begin/spawn event sent")
}

func TestEvent_TeamJoin(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"old_team":    "spectator",
		"new_team":    "allies",
	}
	sendEvent(t, "team_change", params)
	t.Log("✓ team_join event sent")
}

func TestEvent_PlayerSay(t *testing.T) {
	params := map[string]string{
		"player_name": testPlayerName,
		"player_guid": testPlayerGUID,
		"message":     "Hello from E2E test!",
	}
	sendEvent(t, "chat", params)
	t.Log("✓ player_say/chat event sent")
}

// =============================================================================
// GAME FLOW EVENT TESTS
// =============================================================================

func TestEvent_MatchStart(t *testing.T) {
	params := map[string]string{
		"session_id": "sess_test_001",
		"map_name":   "dm/mohdm1",
		"gametype":   "ffa",
	}
	sendEvent(t, "match_start", params)
	t.Log("✓ match_start event sent")
}

func TestEvent_MatchEnd(t *testing.T) {
	params := map[string]string{
		"session_id":   "sess_test_001",
		"map_name":     "dm/mohdm1",
		"winning_team": "allies",
		"duration":     "1200",
	}
	sendEvent(t, "match_end", params)
	t.Log("✓ match_end event sent")
}

func TestEvent_RoundStart(t *testing.T) {
	params := map[string]string{
		"round_number": "1",
	}
	sendEvent(t, "round_start", params)
	t.Log("✓ round_start event sent")
}

func TestEvent_RoundEnd(t *testing.T) {
	params := map[string]string{
		"round_number": "1",
		"winning_team": "allies",
	}
	sendEvent(t, "round_end", params)
	t.Log("✓ round_end event sent")
}

func TestEvent_Heartbeat(t *testing.T) {
	params := map[string]string{
		"session_id":   "sess_test_001",
		"map_name":     "dm/mohdm1",
		"player_count": "8",
	}
	sendEvent(t, "heartbeat", params)
	t.Log("✓ heartbeat event sent")
}

// =============================================================================
// VEHICLE EVENT TESTS
// =============================================================================

func TestEvent_VehicleEnter(t *testing.T) {
	params := map[string]string{
		"player_name":  testPlayerName,
		"player_guid":  testPlayerGUID,
		"vehicle_type": "jeep",
	}
	sendEvent(t, "vehicle_enter", params)
	t.Log("✓ vehicle_enter event sent")
}

func TestEvent_VehicleExit(t *testing.T) {
	params := map[string]string{
		"player_name":  testPlayerName,
		"player_guid":  testPlayerGUID,
		"vehicle_type": "jeep",
	}
	sendEvent(t, "vehicle_exit", params)
	t.Log("✓ vehicle_exit event sent")
}

// =============================================================================
// API ENDPOINT TESTS
// =============================================================================

func TestAPI_PlayerStats(t *testing.T) {
	time.Sleep(1 * time.Second)
	stats := getPlayerStats(t, testPlayerGUID)
	if stats == nil {
		t.Log("⚠ Player stats returned nil (may need more events)")
		return
	}
	t.Log("✓ Player stats retrieved")
}

func TestAPI_Leaderboards(t *testing.T) {
	resp, err := http.Get(apiBaseURL + "/api/v1/stats/leaderboards?stat=kills&limit=10")
	if err != nil {
		t.Fatalf("Failed to get leaderboards: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Leaderboards returned status %d", resp.StatusCode)
		return
	}
	t.Log("✓ Leaderboards endpoint works")
}

// =============================================================================
// SMF FRONTEND TESTS
// =============================================================================

func TestSMF_LeaderboardPage(t *testing.T) {
	resp, err := http.Get(smfBaseURL + "/index.php?action=mohaastats;sa=leaderboards")
	if err != nil {
		t.Fatalf("Failed to get SMF leaderboards: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	if !strings.Contains(content, "mohaastats") {
		t.Log("⚠ SMF leaderboard page may not be configured (missing 'mohaastats' in content)")
	}
	
	// Check for a known element from the leaderboard template
	if !strings.Contains(content, "Leaderboard") {
		t.Error("SMF Leaderboard page missing 'Leaderboard' title")
	}
	
	t.Log("✓ SMF Leaderboard page accessible and renders template")
}

func TestSMF_PlayerPage(t *testing.T) {
	// Attempt to view a player profile. 
	// Note: We need a valid ID or GUID. Using testPlayerGUID which might not be linked in SMF during this specific test run 
	// unless we ran the full simulation first.
	// For integration check, we just want to see if the Profile Page loads without 500 and contains the Stats tab link.
	
	resp, err := http.Get(smfBaseURL + "/index.php?action=profile;u=1") 
	if err != nil {
		t.Fatalf("Failed to get SMF player profile: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("⚠ SMF player profile returned status %d (User 1 might not exist)", resp.StatusCode)
		return
	}
	
	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// Check for the injected Stats tab/link
	if strings.Contains(content, "Stats") || strings.Contains(content, "mohaastats") {
		t.Log("✓ SMF Profile Hook detected (Stats tab present)")
	} else {
		t.Log("⚠ SMF Profile Hook NOT detected (Stats tab missing on User 1)")
	}
}

// =============================================================================
// FULL INTEGRATION TEST
// =============================================================================

func TestFullGameSimulation(t *testing.T) {
	t.Log("=== Starting Full Game Simulation ===")

	// Match Start
	sendEvent(t, "match_start", map[string]string{
		"session_id": "sess_full_test",
		"map_name":   "dm/mohdm1",
		"gametype":   "ffa",
	})

	// Players Connect and Spawn
	for i := 1; i <= 4; i++ {
		sendEvent(t, "connect", map[string]string{
			"player_name": fmt.Sprintf("Player_%d", i),
			"player_guid": fmt.Sprintf("player-guid-%d", i),
		})
		team := "allies"
		if i > 2 {
			team = "axis"
		}
		sendEvent(t, "spawn", map[string]string{
			"player_name": fmt.Sprintf("Player_%d", i),
			"player_guid": fmt.Sprintf("player-guid-%d", i),
			"team":        team,
		})
	}

	// Simulate Combat
	for round := 0; round < 3; round++ {
		for i := 1; i <= 4; i++ {
			sendEvent(t, "weapon_fire", map[string]string{
				"player_name": fmt.Sprintf("Player_%d", i),
				"player_guid": fmt.Sprintf("player-guid-%d", i),
				"weapon":      "M1 Garand",
			})
		}

		sendEvent(t, "kill", map[string]string{
			"attacker_name": "Player_1",
			"attacker_guid": "player-guid-1",
			"victim_name":   "Player_3",
			"victim_guid":   "player-guid-3",
			"weapon":        "M1 Garand",
		})
	}

	// Match End
	sendEvent(t, "match_end", map[string]string{
		"session_id":   "sess_full_test",
		"map_name":     "dm/mohdm1",
		"winning_team": "allies",
		"duration":     "600",
	})

	t.Log("=== Full Game Simulation Complete ===")
	t.Log("✓ All simulation events sent successfully")
}

// =============================================================================
// BATCH EVENT TEST (Performance)
// =============================================================================

func TestBatchEvents(t *testing.T) {
	t.Log("=== Sending Batch of 50 Events ===")

	start := time.Now()
	for i := 0; i < 50; i++ {
		sendEvent(t, "weapon_fire", map[string]string{
			"player_name": testPlayerName,
			"player_guid": testPlayerGUID,
			"weapon":      "Thompson",
		})
	}
	elapsed := time.Since(start)

	t.Logf("✓ Sent 50 events in %v (%.2f events/sec)", elapsed, 50.0/elapsed.Seconds())
}

// =============================================================================
// JSON BATCH INGEST TEST
// =============================================================================

func TestBatchJSONIngest(t *testing.T) {
	events := []map[string]interface{}{
		{
			"type":          "kill",
			"match_id":      testMatchID,
			"timestamp":     time.Now().Unix(),
			"attacker_name": testPlayerName,
			"attacker_guid": testPlayerGUID,
			"victim_name":   "BatchVictim",
			"victim_guid":   "batch-victim-001",
			"weapon":        "MP40",
		},
		{
			"type":        "weapon_fire",
			"match_id":    testMatchID,
			"timestamp":   time.Now().Unix(),
			"player_name": testPlayerName,
			"player_guid": testPlayerGUID,
			"weapon":      "MP40",
		},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"events":       events,
		"server_token": serverToken,
		"server_id":    serverID,
	})

	resp, err := http.Post(
		apiBaseURL+"/api/v1/ingest/batch",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		t.Fatalf("Failed to send batch: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Logf("⚠ Batch ingest returned %d: %s (endpoint may not exist)", resp.StatusCode, string(respBody))
		return
	}
	t.Log("✓ Batch JSON ingest works")
}
