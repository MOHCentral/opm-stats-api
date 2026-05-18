package worker

import (
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
)

func BenchmarkConvertToClickHouseEvent(b *testing.B) {
	p := &Pool{}
	event := &models.RawEvent{
		Type:    models.EventPlayerKill,
		MatchID: "invalid-uuid-string",
	}
	rawJSON := "{}"
	receivedAt := time.Now()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.convertToClickHouseEvent(event, rawJSON, receivedAt)
	}
}
