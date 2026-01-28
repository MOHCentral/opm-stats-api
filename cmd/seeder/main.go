package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Config
const (
	API_URL    = "http://localhost:8084/api/v1/ingest/events"
	JWT_TOKEN  = "seed-secret-123" 
	EVENT_TYPE = "player_kill"
)

// Event matches models.RawEvent structure (simplified)
type Event struct {
	Type        string  `json:"type"`
	MatchID     string  `json:"match_id"`
	Timestamp   float64 `json:"timestamp"`
	ServerToken string  `json:"server_token"` // Although header is used, sometimes redundant payload helps debug
	
	// Actor
	PlayerGUID   string `json:"player_guid"`
	PlayerName   string `json:"player_name"`
	PlayerTeam   string `json:"player_team"`
	
	// Attacker
	AttackerGUID string `json:"attacker_guid"`
	AttackerName string `json:"attacker_name"`
	AttackerTeam string `json:"attacker_team"`
	
	// Victim
	VictimGUID string `json:"victim_guid"`
	VictimName string `json:"victim_name"`
	VictimTeam string `json:"victim_team"`
	
	// Data
	Weapon string `json:"weapon"`
	Hitloc string `json:"hitloc"`
	Damage int    `json:"damage"`

	// Heartbeat fields
	ServerID    string `json:"server_id"`
	MapName     string `json:"map_name"`
	Gametype    string `json:"gametype"`
	PlayerCount int    `json:"player_count"`
	RoundNumber int    `json:"round_number"`
}

func main() {
	// Create a mock kill event
	event := Event{
		Type:        "kill",
		MatchID:     "test-match-001",
		Timestamp:   float64(time.Now().Unix()),
		
		PlayerGUID:   "attacker-guid-456", // In kill event, Player is usually attacker
		PlayerName:   "TestAttacker",
		PlayerTeam:   "axis",

		AttackerGUID: "attacker-guid-456",
		AttackerName: "TestAttacker",
		AttackerTeam: "axis",
		
		VictimGUID:   "victim-guid-123",
		VictimName:   "TestVictim",
		VictimTeam:   "allies",
		
		Weapon: "Thompson",
		Hitloc: "head",
		Damage: 100,
	}

	// Send a HEARTBEAT first to trigger online status
	heartbeat := Event{
		Type:        "heartbeat",
		MatchID:     "test-match-001",
		ServerID:    "00876eb7-5888-4210-b51d-84e65b97ae1d",
		Timestamp:   float64(time.Now().Unix()),
		ServerToken: "test-token",
		MapName:     "obj_team2",
		Gametype:    "obj",
		PlayerCount: 16,
		RoundNumber: 3,
	}
	hbPayload, _ := json.Marshal(heartbeat)

	var buffer bytes.Buffer
	buffer.Write(hbPayload)
	buffer.Write([]byte("\n"))

	// Send a batch of kill events
	for i := 0; i < 5; i++ {
		event.MatchID = fmt.Sprintf("test-match-%d", i)
		event.Timestamp = float64(time.Now().Unix())
		
		payload, _ := json.Marshal(event)
		buffer.Write(payload)
		buffer.Write([]byte("\n")) // NDJSON
	}

	req, err := http.NewRequest("POST", API_URL, &buffer)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json") 
	req.Header.Set("Authorization", JWT_TOKEN)         

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(body))

	if resp.StatusCode == 202 {
		fmt.Println("✅ Injection Successful!")
	} else {
		fmt.Println("❌ Injection Failed!")
	}
}
