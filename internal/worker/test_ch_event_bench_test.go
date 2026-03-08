package worker

import (
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
)

func BenchmarkConvertToClickHouseEvent(b *testing.B) {
	p := &Pool{}
	event := &models.RawEvent{
		MatchID: "some-invalid-match-id",
		Type:    models.EventPlayerKill,
		Timestamp: float64(time.Now().Unix()),
	}
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.convertToClickHouseEvent(event, "{}", now)
	}
}
