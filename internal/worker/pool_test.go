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

	chEventWin := p.convertToClickHouseEvent(eventWin, "{}", time.Now())

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

	chEventLoss := p.convertToClickHouseEvent(eventLoss, "{}", time.Now())

	if chEventLoss.MatchOutcome != 0 {
		t.Errorf("Expected MatchOutcome 0 (Loss), got %d", chEventLoss.MatchOutcome)
	}
}

func TestConvertToClickHouseEvent_Turret(t *testing.T) {
	p := &Pool{}

	matchID := uuid.New().String()
	playerGUID := "test-guid"
	turret := "flak88"

	event := &models.RawEvent{
		Type:       models.EventTurretEnter,
		MatchID:    matchID,
		PlayerGUID: playerGUID,
		PlayerName: "TestPlayer",
		Turret:     turret,
		Timestamp:  float64(time.Now().Unix()),
	}

	chEvent := p.convertToClickHouseEvent(event, "{}", time.Now())

	if chEvent.TargetID != turret {
		t.Errorf("Expected TargetID to store turret %s, got %s", turret, chEvent.TargetID)
	}
}
