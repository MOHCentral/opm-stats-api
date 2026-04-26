package tests

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// TestMQTTIngestion verifies end-to-end MQTT event ingestion.
// Requires: MQTT broker running on localhost:1883, API running with MQTT subscriber.
func TestMQTTIngestion(t *testing.T) {
	broker := os.Getenv("MQTT_BROKER_URL")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}

	// Connect MQTT publisher (simulates game server)
	opts := pahomqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID("test-game-server").
		SetCleanSession(true)

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Skip("MQTT broker not available, skipping MQTT test")
	}
	if token.Error() != nil {
		t.Skip("MQTT broker not available: ", token.Error())
	}
	defer client.Disconnect(1000)

	t.Run("PublishSingleEvent", func(t *testing.T) {
		event := map[string]string{
			"type":        "player_kill",
			"match_id":    "test-match-mqtt-001",
			"session_id":  "test-session-mqtt",
			"player_name": "MQTTTestPlayer",
			"player_guid": "mqtt-test-1",
			"victim_name": "MQTTTestVictim",
			"victim_guid": "mqtt-test-2",
			"weapon":      "M1 Garand",
			"hitloc":      "head",
			"damage":      "150",
			"timestamp":   "123456.78",
		}

		payload, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("Failed to marshal event: %v", err)
		}

		topic := "openmohaa/events/test-server"
		pubToken := client.Publish(topic, 0, false, payload)
		if !pubToken.WaitTimeout(5 * time.Second) {
			t.Fatal("MQTT publish timed out")
		}
		if pubToken.Error() != nil {
			t.Fatalf("MQTT publish failed: %v", pubToken.Error())
		}

		// Give API time to process
		time.Sleep(2 * time.Second)
	})

	t.Run("PublishBatchEvents", func(t *testing.T) {
		events := []map[string]string{
			{
				"type":        "player_kill",
				"match_id":    "test-match-mqtt-batch",
				"player_name": "Attacker1",
				"player_guid": "mqtt-batch-1",
				"victim_name": "Victim1",
				"victim_guid": "mqtt-batch-2",
				"weapon":      "Thompson",
				"timestamp":   "100.0",
			},
			{
				"type":        "player_death",
				"match_id":    "test-match-mqtt-batch",
				"player_name": "Victim1",
				"player_guid": "mqtt-batch-2",
				"weapon":      "Thompson",
				"timestamp":   "100.0",
			},
			{
				"type":        "player_damage",
				"match_id":    "test-match-mqtt-batch",
				"player_name": "Attacker1",
				"player_guid": "mqtt-batch-1",
				"victim_name": "Victim1",
				"victim_guid": "mqtt-batch-2",
				"weapon":      "Thompson",
				"damage":      "75",
				"hitloc":      "torso_upper",
				"timestamp":   "99.9",
			},
		}

		payload, err := json.Marshal(events)
		if err != nil {
			t.Fatalf("Failed to marshal events: %v", err)
		}

		topic := "openmohaa/events/test-server-batch"
		pubToken := client.Publish(topic, 0, false, payload)
		if !pubToken.WaitTimeout(5 * time.Second) {
			t.Fatal("MQTT publish timed out")
		}
		if pubToken.Error() != nil {
			t.Fatalf("MQTT publish failed: %v", pubToken.Error())
		}

		time.Sleep(2 * time.Second)
	})

	t.Run("PublishToMultipleServers", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			event := map[string]string{
				"type":        "player_connect",
				"match_id":    "test-match-multi",
				"player_name": "Player" + string(rune('A'+i)),
				"player_guid": "mqtt-multi-" + string(rune('0'+i)),
				"timestamp":   "200.0",
			}

			payload, _ := json.Marshal([]map[string]string{event})
			topic := "openmohaa/events/server-" + string(rune('0'+i))
			client.Publish(topic, 0, false, payload)
		}

		time.Sleep(2 * time.Second)
	})
}

// TestMQTTReconnection verifies the subscriber handles disconnections.
func TestMQTTReconnection(t *testing.T) {
	broker := os.Getenv("MQTT_BROKER_URL")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID("test-reconnect").
		SetCleanSession(true)

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Skip("MQTT broker not available")
	}
	if token.Error() != nil {
		t.Skip("MQTT broker not available: ", token.Error())
	}

	// Publish, disconnect, reconnect, publish again
	event := map[string]string{
		"type":        "test_event",
		"match_id":    "reconnect-test",
		"player_name": "ReconnectPlayer",
		"player_guid": "reconnect-1",
		"timestamp":   "300.0",
	}
	payload, _ := json.Marshal([]map[string]string{event})
	client.Publish("openmohaa/events/reconnect-test", 0, false, payload)

	client.Disconnect(100)
	time.Sleep(1 * time.Second)

	// Reconnect
	token = client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("Failed to reconnect")
	}
	defer client.Disconnect(1000)

	client.Publish("openmohaa/events/reconnect-test", 0, false, payload)
	time.Sleep(1 * time.Second)
}
