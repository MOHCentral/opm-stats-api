package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// Constants for configuration
const (
	API_URL      = "http://localhost:8080/api/v1/ingest/events"
	TOTAL_EVENTS = 10000
	BATCH_SIZE   = 100
)

// Data pools for random generation
var (
	Weapons = []string{
		"Thompson", "MP40", "BAR", "StG44", "M1 Garand", "Kar98k", 
		"Springfield", "Bazooka", "Colt 45", "Grenade",
	}
	Maps = []string{
		"obj/obj_team1", "obj/obj_team2", "dm/mohdm1", "dm/mohdm2", 
		"obj/obj_team3", "dm/mohdm6", "dm/mohdm7",
	}
	Players = []struct {
		ID   int
		Name string
	}{
		{1, "Major_Payne"},
		{2, "Pvt_Ryan"},
		{3, "Sgt_Rock"},
		{4, "Cpt_Miller"},
		{5, "Sniper_Wolf"},
		{6, "Tank_Buster"},
		{7, "Medic_One"},
		{8, "General_Chaos"},
		{9, "Lt_Dan"},
		{10, "Noob_Saibot"},
	}
	HitLocations = []string{
		"HEAD", "TORSO", "LEFT_ARM", "RIGHT_ARM", "LEFT_LEG", "RIGHT_LEG",
	}
	Mods = []string{
		"MOD_PISTOL", "MOD_RIFLE", "MOD_SMG", "MOD_SNIPER", "MOD_EXPLOSIVE",
	}
)

type EventPayload struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func main() {
	fmt.Printf("Starting data seeder. Target: %s, Events: %d\n", API_URL, TOTAL_EVENTS)
	
	rand.Seed(time.Now().UnixNano())
	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < TOTAL_EVENTS; i++ {
		event := generateRandomEvent()
		
		// Send event individually (or implement batching if API supports it, currently singular)
		if err := sendEvent(client, event); err != nil {
			fmt.Printf("Error sending event %d: %v\n", i, err)
		}

		if i%100 == 0 {
			fmt.Printf("Progress: %d/%d received\n", i, TOTAL_EVENTS)
		}
		
		// Tiny sleep to prevent overflowing local port limits if running extremely fast
		time.Sleep(1 * time.Millisecond)
	}

	fmt.Println("Seeding complete!")
}

func generateRandomEvent() EventPayload {
	// 80% chance of KILL event, 20% other (join, chat, etc - simplified to just KILL/DEATH for stats)
	// Actually, for stats dashboard, we primarily care about Kills/Deaths/Damage.
	
	attacker := Players[rand.Intn(len(Players))]
	victim := Players[rand.Intn(len(Players))]
	
	// Ensure attacker != victim (suicides possible but rare, let's just avoid for noise)
	for attacker.ID == victim.ID {
		victim = Players[rand.Intn(len(Players))]
	}

	weapon := Weapons[rand.Intn(len(Weapons))]
	mapName := Maps[rand.Intn(len(Maps))]
	hitLoc := HitLocations[rand.Intn(len(HitLocations))]
	mod := Mods[rand.Intn(len(Mods))]
	damage := 10 + rand.Intn(90) // Random damage 10-100

	// Create a "Kill" event structure derived from OpenMOHAA event reference
	// Event: "player_kill" or similar.
	// Based on STATS_MASTER.md, we ingest atomic events. 
	// The API likely expects a specific format. 
	// Looking at previous context, we have an "ingest" endpoint. 
	
	data := map[string]interface{}{
		"attacker_id":   attacker.ID,
		"attacker_name": attacker.Name,
		"victim_id":     victim.ID,
		"victim_name":   victim.Name,
		"weapon":        weapon,
		"damage":        damage,
		"location":      hitLoc,
		"mod":           mod,
		"map":           mapName,
	}

	return EventPayload{
		Type:      "EVT_KILL", // Using a consistent event type
		Timestamp: time.Now().Add(-time.Duration(rand.Intn(7*24)) * time.Hour).Unix(), // Random time in last 7 days
		Data:      data,
	}
}

func sendEvent(client *http.Client, event EventPayload) error {
	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", API_URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer dev-seed-token")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("API returned status: %s", resp.Status)
	}

	return nil
}
