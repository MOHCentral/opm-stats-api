package worker

import (
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
)

func BenchmarkConvertToClickHouseEvent(b *testing.B) {
	p := &Pool{}
	event := &models.RawEvent{
		MatchID:      "invalid-uuid-string-to-trigger-md5",
		ServerID:     "server-1",
		MapName:      "dm/mohdm1",
		Type:         models.EventPlayerKill,
		AttackerGUID: "guid-1",
		AttackerName: "Player1",
		VictimGUID:   "guid-2",
		VictimName:   "Player2",
		Weapon:       "thompson",
		Hitloc:       "head",
		Timestamp:    1600000000,
	}
	receivedAt := time.Now()
	rawJSON := "{}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.convertToClickHouseEvent(event, rawJSON, receivedAt)
	}
}
