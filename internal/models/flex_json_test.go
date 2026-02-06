package models

import (
	"encoding/json"
	"testing"
)

func TestFlexUnmarshal_AllStrings(t *testing.T) {
	input := `[{"session_id": "sess_53e89f0cd1cea438", "type": "player_kill", "damage": "200.000", "victim_yaw": "80.673", "attacker_yaw": "240.612", "attacker_team": "american", "victim_pitch": "0.000", "timestamp": "37.900", "attacker_z": "0.125", "attacker_y": "-215.872", "attacker_x": "704.477", "attacker_name": "Hiroshi", "attacker_pitch": "0.000", "victim_z": "0.125", "victim_y": "-702.150", "weapon": "Explosion", "attacker_guid": "unauth_34", "victim_x": "288.405", "attacker_stance": "stand", "mod": "grenade", "victim_team": "american", "match_id": "no_match", "victim_stance": "stand", "victim_name": "Yuta", "hitloc": "general", "victim_guid": "unauth_41"}]`

	var events []RawEvent
	err := json.Unmarshal([]byte(input), &events)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Type != "player_kill" {
		t.Errorf("Type = %q, want player_kill", e.Type)
	}
	if e.Damage != 200.0 {
		t.Errorf("Damage = %f, want 200.0", e.Damage)
	}
	if e.Timestamp != 37.9 {
		t.Errorf("Timestamp = %f, want 37.9", e.Timestamp)
	}
	if e.AttackerName != "Hiroshi" {
		t.Errorf("AttackerName = %q, want Hiroshi", e.AttackerName)
	}
	if e.VictimName != "Yuta" {
		t.Errorf("VictimName = %q, want Yuta", e.VictimName)
	}
	if e.Weapon != "Explosion" {
		t.Errorf("Weapon = %q, want Explosion", e.Weapon)
	}
}

func TestFlexUnmarshal_NativeTypes(t *testing.T) {
	input := `[{"type": "player_kill", "damage": 112.487, "timestamp": 9.8, "attacker_name": "Daichi", "match_id": "no_match"}]`

	var events []RawEvent
	err := json.Unmarshal([]byte(input), &events)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	e := events[0]
	if e.Damage != 112.487 {
		t.Errorf("Damage = %f, want 112.487", e.Damage)
	}
}
