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

	var chEventWin models.ClickHouseEvent
	p.fillClickHouseEvent(eventWin, "{}", time.Now(), &chEventWin)

	if chEventWin.MatchOutcome != 1 {
		t.Errorf("Expected MatchOutcome 1 (Win), got %d", chEventWin.MatchOutcome)
	}
	if chEventWin.ActorWeapon != gametype {
		t.Errorf("Expected ActorWeapon to store gametype %s, got %s", gametype, chEventWin.ActorWeapon)
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

	var chEventLoss models.ClickHouseEvent
	p.fillClickHouseEvent(eventLoss, "{}", time.Now(), &chEventLoss)

	if chEventLoss.MatchOutcome != 0 {
		t.Errorf("Expected MatchOutcome 0 (Loss), got %d", chEventLoss.MatchOutcome)
	}
}
