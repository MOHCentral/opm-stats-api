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

	// Dummy Redis client (won't connect, but Pipeline() works if mocked properly,
	// but here we use the actual client struct which might panic if used without connection.
	// However, Pool uses cfg.Redis.Pipeline(). In tests we should ideally mock it.
	// The current test uses a real client pointed to localhost:0 which might fail fast.
	// But let's see if we can use the MockStatStore for everything?
	// PoolConfig expects *redis.Client. We can't mock *redis.Client easily without an interface.
	// But the Pool struct uses *redis.Client directly.
	// This is a design flaw in Pool (should use interface).
	// For now, we'll assume NewClient doesn't panic on init.
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})

	cfg := PoolConfig{
		WorkerCount:   2,
		QueueSize:     1000,
		BatchSize:     10,
		FlushInterval: 10 * time.Millisecond,
		ClickHouse:    &MockClickHouseConn{},
		// Postgres left nil, hope we don't hit it in side effects if we are careful
		// But processBatchSideEffects DOES use p.config.Postgres.Exec for achievements.
		// We might panic there.
		// We need to set Postgres to something safe?
		// PoolConfig.Postgres is *pgxpool.Pool. Hard to mock directly.
		// However, processBatchSideEffects is running asynchronously.
		Redis:         rdb,
		Logger:        logger,
	}

	// We can't easily use NewPool because it initializes AchievementWorker which uses Postgres.
	// We will manually initialize Pool but INCLUDE the semaphore.
	p := &Pool{
		config:        cfg,
		jobQueue:      make(chan Job, cfg.QueueSize),
		logger:        cfg.Logger.Sugar(),
		sideEffectSem: make(chan struct{}, cfg.WorkerCount*2),
	}

	// Manually init achievement worker with mocks to avoid panic if called
	statStore := NewMockStatStore()
	p.achievementWorker = NewAchievementWorker(&MockDBStore{}, &MockClickHouseConn{}, statStore, logger.Sugar())

	// Start pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Enqueue many events to trigger batching
	wg := sync.WaitGroup{}
	workers := 10
	eventsPerWorker := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerWorker; j++ {
				// Use different event types to trigger different code paths
				evtType := models.EventKill
				if j%2 == 0 {
					evtType = models.EventMatchStart // triggers side effects
				}

				p.Enqueue(&models.RawEvent{
					Type:         evtType,
					MatchID:      "test-match",
					AttackerGUID: fmt.Sprintf("attacker-%d", j),
					PlayerGUID:   fmt.Sprintf("player-%d", j),
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
