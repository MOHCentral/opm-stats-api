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

func BenchmarkConvertToClickHouseEvent(b *testing.B) {
	p := &Pool{}

	event := &models.RawEvent{
		Type:          models.EventPlayerKill,
		MatchID:       "00000000-0000-0000-0000-000000000000",
		PlayerGUID:    "test-guid",
		PlayerName:    "^1Test^2Player",
		Timestamp:     float64(time.Now().Unix()),
		AttackerGUID:  "attacker-guid",
		AttackerName:  "^3Attacker^4Name",
		AttackerSMFID: 123,
		Weapon:        "mp40",
		VictimGUID:    "victim-guid",
		VictimName:    "^5Victim^6Name",
		Hitloc:        "head",
	}
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.convertToClickHouseEvent(event, "{}", now)
	}
}

func BenchmarkNormalizeWeaponLabel(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeWeaponLabel("  mp40  ", "  MOD_MP40  ", "  ")
	}
}
