package worker

import (
	"context"
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

func BenchmarkProcessEvent(b *testing.B) {
	// Setup
	mockCH := &MockClickHouseConn{
		QueryDuration: 1 * time.Millisecond, // Simulate 1ms DB latency
	}

	worker := &AchievementWorker{
		db:              &MockDBStore{},
		ch:              mockCH,
		statStore:       NewMockStatStore(),
		logger:          zap.NewNop().Sugar(),
		achievementDefs: make(map[string]*AchievementDefinition),
		ctx:             context.Background(),
		cancel:          func() {},
	}

	event := &models.RawEvent{
		Type:          models.EventPlayerKill,
		AttackerSMFID: 123,
		Timestamp:     float64(time.Now().Unix()),
		Weapon:        "mp40",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		worker.ProcessEvent(event)
	}
}
