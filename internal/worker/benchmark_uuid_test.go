package worker

import (
	"testing"

	"github.com/google/uuid"
	"github.com/openmohaa/stats-api/internal/models"
)

var defaultUUIDNamespaceForBench = uuid.MustParse("00000000-0000-0000-0000-000000000000")

func BenchmarkUUIDParseMustParseInLoop(b *testing.B) {
	event := &models.RawEvent{MatchID: "invalid-uuid"}
	for i := 0; i < b.N; i++ {
		_, err := uuid.Parse(event.MatchID)
		if err != nil {
			namespace := uuid.MustParse("00000000-0000-0000-0000-000000000000")
			_ = uuid.NewMD5(namespace, []byte(event.MatchID))
		}
	}
}

func BenchmarkUUIDParsePackageLevel(b *testing.B) {
	event := &models.RawEvent{MatchID: "invalid-uuid"}
	for i := 0; i < b.N; i++ {
		_, err := uuid.Parse(event.MatchID)
		if err != nil {
			_ = uuid.NewMD5(defaultUUIDNamespaceForBench, []byte(event.MatchID))
		}
	}
}
