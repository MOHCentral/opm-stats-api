package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

func BenchmarkFillClickHouseEvent(b *testing.B) {
	p := &Pool{
		logger: zap.NewNop().Sugar(),
	}

	matchID := uuid.New().String()
	event := &models.RawEvent{
		Type:         models.EventPlayerKill,
		MatchID:      matchID,
		ServerID:     "test-server",
		MapName:      "obj/obj_team2",
		Timestamp:    float64(time.Now().Unix()),
		AttackerGUID: "1234567890",
		AttackerName: "TestPlayer^1One",
		AttackerTeam: "axis",
		Weapon:       "mp40",
		VictimGUID:   "0987654321",
		VictimName:   "TestPlayer^2Two",
		VictimTeam:   "allies",
		Damage:       100,
		Hitloc:       "head",
		Distance:     500.5,
		RoundNumber:  1,
	}
	rawJSON := "{}"
	ts := time.Now()

	b.ResetTimer()
	b.ReportAllocs()
	var chEvent models.ClickHouseEvent
	for i := 0; i < b.N; i++ {
		p.fillClickHouseEvent(event, rawJSON, ts, &chEvent)
	}
}
