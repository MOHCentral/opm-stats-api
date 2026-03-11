package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/openmohaa/stats-api/internal/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestPool_RaceCondition(t *testing.T) {
	// Setup dummy dependencies
	logger := zap.NewNop()

	// Dummy Redis client (won't connect, but Pipeline() works)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})

	cfg := PoolConfig{
		WorkerCount:   2,
		QueueSize:     1000,
		BatchSize:     10,
		FlushInterval: 10 * time.Millisecond,
		ClickHouse:    &MockClickHouseConn{},
		// Postgres left nil, hope we don't hit it
		Redis:  rdb,
		Logger: logger,
	}

	p := &Pool{
		config:   cfg,
		jobQueue: make(chan Job, cfg.QueueSize),
		logger:   cfg.Logger,
	}

	// Manually init achievement worker with mocks to avoid panic if called
	// We don't care if it works, just that it doesn't crash immediately
	statStore := NewMockStatStore()
	p.achievementWorker = NewAchievementWorker(&MockDBStore{}, &MockClickHouseConn{}, statStore, logger)

	// Start pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// We call Start manually because we built Pool manually
	// p.Start(ctx) uses p.config.WorkerCount
	p.ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// We skip reportQueueDepth as it might not be relevant and adds goroutine

	// Enqueue many events to trigger batching
	wg := sync.WaitGroup{}
	workers := 10
	eventsPerWorker := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerWorker; j++ {
				p.Enqueue(&models.RawEvent{
					Type:         models.EventPlayerKill,
					MatchID:      "test-match",
					AttackerGUID: fmt.Sprintf("attacker-%d", j),
					Timestamp:    float64(time.Now().Unix()),
				})
				// Small sleep to spread out events
				if j%10 == 0 {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	// Give some time for processing
	time.Sleep(500 * time.Millisecond)

	p.Stop()
}
