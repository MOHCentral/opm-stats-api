package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openmohaa/stats-api/internal/models"
)

func TestConvertToClickHouseEvent_MatchOutcome(t *testing.T) {
	// Setup
	p := &Pool{} // We only need p for the method call, config is not used in convertToClickHouseEvent

	matchID := uuid.New().String()
	playerGUID := "test-guid"
	gametype := "obj/obj_team2"

	// Test case: Win
	eventWin := &models.RawEvent{
		Type:         models.EventMatchOutcome,
		MatchID:      matchID,
		PlayerGUID:   playerGUID,
		PlayerName:   "TestPlayer",
		MatchOutcome: 1, // Win
		Gametype:     gametype,
		Timestamp:    float64(time.Now().Unix()),
	}

	var chEvent models.ClickHouseEvent
	p.fillClickHouseEvent(eventWin, "{}", time.Now(), &chEvent)

	if chEvent.MatchOutcome != 1 {
		t.Errorf("Expected MatchOutcome 1 (Win), got %d", chEvent.MatchOutcome)
	}
	if chEvent.ActorWeapon != gametype {
		t.Errorf("Expected ActorWeapon to store gametype %s, got %s", gametype, chEvent.ActorWeapon)
	}

	// Test case: Loss
	eventLoss := &models.RawEvent{
		Type:         models.EventMatchOutcome,
		MatchID:      matchID,
		PlayerGUID:   playerGUID,
		PlayerName:   "TestPlayer",
		MatchOutcome: 0, // Loss
		Gametype:     gametype,
		Timestamp:    float64(time.Now().Unix()),
	}

	p.fillClickHouseEvent(eventLoss, "{}", time.Now(), &chEvent)

	if chEvent.MatchOutcome != 0 {
		t.Errorf("Expected MatchOutcome 0 (Loss), got %d", chEvent.MatchOutcome)
	}
}
