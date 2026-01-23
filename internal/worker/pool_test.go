package worker

import (
	"context"
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
	"go.uber.org/zap"
)

func TestEnqueueFull(t *testing.T) {
	// Create a pool manually to avoid external dependencies
	cfg := PoolConfig{
		QueueSize: 1,
		Logger:    zap.NewNop(),
	}

	pool := &Pool{
		config:   cfg,
		jobQueue: make(chan Job, cfg.QueueSize),
		logger:   cfg.Logger.Sugar(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool.ctx = ctx
	pool.cancel = cancel
	defer cancel()

	// Fill the queue
	event1 := &models.RawEvent{MatchID: "1"}
	if !pool.Enqueue(event1) {
		t.Fatal("Failed to enqueue first event")
	}

	// Try to enqueue second event, it should return false immediately
	event2 := &models.RawEvent{MatchID: "2"}

	start := time.Now()
	enqueued := pool.Enqueue(event2)
	duration := time.Since(start)

	if enqueued {
		t.Error("Enqueue should have returned false when queue is full")
	}

	if duration > 10*time.Millisecond {
		t.Errorf("Enqueue took too long (%v), expected immediate return", duration)
	}
}
