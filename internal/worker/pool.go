// Package worker implements the buffered worker pool pattern for async event processing.
// This decouples HTTP request handling from database writes, providing:
// - Backpressure handling via load shedding
// - Batch inserts for efficient ClickHouse writes
// - Graceful shutdown with flush guarantees

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/openmohaa/stats-api/internal/models"
)

// Achievement thresholds
var (
	killThresholds = map[int64]string{
		100:   "KILL_100",
		500:   "KILL_500",
		1000:  "KILL_1000",
		5000:  "KILL_5000",
		10000: "KILL_10000",
	}
	headshotThresholds = map[int64]string{
		50:   "HEADSHOT_50",
		100:  "HEADSHOT_100",
		500:  "HEADSHOT_500",
		1000: "HEADSHOT_1000",
	}
)

// Prometheus metrics
var (
	eventsIngested = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_events_ingested_total",
		Help: "Total number of events ingested",
	})

	eventsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_events_processed_total",
		Help: "Total number of events processed by workers",
	})

	eventsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_events_failed_total",
		Help: "Total number of events that failed processing",
	})

	queueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mohaa_worker_queue_depth",
		Help: "Current depth of the worker queue",
	})

	batchInsertDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mohaa_batch_insert_duration_seconds",
		Help:    "Duration of batch inserts to ClickHouse",
		Buckets: prometheus.DefBuckets,
	})

	eventsLoadShed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mohaa_events_load_shed_total",
		Help: "Total number of events dropped due to load shedding",
	})
)

// Job represents a unit of work for the worker pool
type Job struct {
	Event     *models.RawEvent
	RawJSON   string
	Timestamp time.Time
}

// PoolConfig configures the worker pool
type PoolConfig struct {
	WorkerCount   int
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
	ClickHouse    driver.Conn
	Postgres      *pgxpool.Pool
	Redis         *redis.Client
	Logger        *zap.Logger
}

// Pool manages a pool of workers for async event processing
type Pool struct {
	config            PoolConfig
	jobQueue          chan Job
	wg                sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc
	logger            *zap.SugaredLogger
	achievementWorker *AchievementWorker
}

// NewPool creates a new worker pool
func NewPool(cfg PoolConfig) *Pool {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}

	pool := &Pool{
		config:   cfg,
		jobQueue: make(chan Job, cfg.QueueSize),
		logger:   cfg.Logger.Sugar(),
	}

	// Initialize Achievement Worker with both Postgres and ClickHouse
	statStore := &RedisStatStore{client: cfg.Redis}
	pool.achievementWorker = NewAchievementWorker(cfg.Postgres, cfg.ClickHouse, statStore, cfg.Logger.Sugar())
	pool.achievementWorker.Start()

	return pool
}

// Start launches the worker goroutines
func (p *Pool) Start(ctx context.Context) {
	p.ctx, p.cancel = context.WithCancel(ctx)

	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start queue depth reporter
	go p.reportQueueDepth()

	p.logger.Infow("Worker pool started",
		"workers", p.config.WorkerCount,
		"queueSize", p.config.QueueSize,
		"batchSize", p.config.BatchSize,
	)
}

// Stop gracefully shuts down the worker pool
func (p *Pool) Stop() {
	p.logger.Info("Stopping worker pool...")

	// Stop achievement worker
	if p.achievementWorker != nil {
		p.achievementWorker.Stop()
	}

	p.cancel()
	close(p.jobQueue)
	p.wg.Wait()
	p.logger.Info("Worker pool stopped")
}

// Enqueue adds a job to the queue. Blocks if queue is full (no load shedding).
func (p *Pool) Enqueue(event *models.RawEvent) bool {
	rawJSON, _ := json.Marshal(event)

	job := Job{
		Event:     event,
		RawJSON:   string(rawJSON),
		Timestamp: time.Now(),
	}

	// Protect against sending on closed channel
	defer func() {
		if r := recover(); r != nil {
			p.logger.Warnw("Failed to enqueue event (pool stopped)", "error", r)
		}
	}()

	select {
	case p.jobQueue <- job:
		eventsIngested.Inc()
		return true
	case <-p.ctx.Done():
		p.logger.Warn("Worker pool context canceled, dropping event")
		eventsLoadShed.Inc()
		return false
	}
}

// QueueDepth returns current queue size
func (p *Pool) QueueDepth() int {
	return len(p.jobQueue)
}

// worker processes jobs from the queue in batches
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	p.logger.Infow("Worker started", "worker", id)

	batch := make([]Job, 0, p.config.BatchSize)
	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			p.logger.Infow("Flush called with empty batch", "worker", id)
			return
		}

		p.logger.Infow("Flushing batch", "worker", id, "batchSize", len(batch))

		start := time.Now()
		if err := p.processBatch(batch); err != nil {
			p.logger.Errorw("Batch processing failed",
				"worker", id,
				"batchSize", len(batch),
				"error", err,
			)
			eventsFailed.Add(float64(len(batch)))
		} else {
			p.logger.Infow("Batch processed successfully", "worker", id, "batchSize", len(batch), "duration", time.Since(start))
			eventsProcessed.Add(float64(len(batch)))
		}
		batchInsertDuration.Observe(time.Since(start).Seconds())

		batch = batch[:0]
	}

	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				// Channel closed, flush remaining
				p.logger.Infow("Job queue closed, flushing remaining batch", "worker", id)
				flush()
				return
			}

			p.logger.Infow("Received job", "worker", id, "eventType", job.Event.Type)
			batch = append(batch, job)
			if len(batch) >= p.config.BatchSize {
				p.logger.Infow("Batch size reached, flushing", "worker", id, "batchSize", len(batch))
				flush()
			}

		case <-ticker.C:
			p.logger.Infow("Ticker fired", "worker", id, "batchSize", len(batch))
			flush()

		case <-p.ctx.Done():
			p.logger.Infow("Context done, flushing final batch", "worker", id)
			flush()
			return
		}
	}
}

// processBatch handles a batch of events
func (p *Pool) processBatch(batch []Job) error {
	if len(batch) == 0 {
		return nil
	}

	// Prepare ClickHouse batch insert
	ctx := context.Background()

	chBatch, err := p.config.ClickHouse.PrepareBatch(ctx, `
		INSERT INTO mohaa_stats.raw_events (
			timestamp, match_id, server_id, map_name, event_type,
			actor_id, actor_name, actor_team, actor_weapon,
			actor_pos_x, actor_pos_y, actor_pos_z, actor_pitch, actor_yaw, actor_stance,
			target_id, target_name, target_team,
			target_pos_x, target_pos_y, target_pos_z, target_stance,
			damage, hitloc, distance, raw_json, actor_smf_id, target_smf_id, match_outcome, round_number
		)
	`)
	if err != nil {
		return err
	}

	for _, job := range batch {
		event := job.Event

		// Convert to ClickHouse event, using job receipt time as fallback for game-relative timestamps
		chEvent := p.convertToClickHouseEvent(event, job.RawJSON, job.Timestamp)

		err := chBatch.Append(
			chEvent.Timestamp,
			chEvent.MatchID,
			chEvent.ServerID,
			chEvent.MapName,
			chEvent.EventType,
			chEvent.ActorID,
			chEvent.ActorName,
			chEvent.ActorTeam,
			chEvent.ActorWeapon,
			chEvent.ActorPosX,
			chEvent.ActorPosY,
			chEvent.ActorPosZ,
			chEvent.ActorPitch,
			chEvent.ActorYaw,
			chEvent.ActorStance,
			chEvent.TargetID,
			chEvent.TargetName,
			chEvent.TargetTeam,
			chEvent.TargetPosX,
			chEvent.TargetPosY,
			chEvent.TargetPosZ,
			chEvent.TargetStance,
			chEvent.Damage,
			chEvent.Hitloc,
			chEvent.Distance,
			chEvent.RawJSON,
			chEvent.ActorSMFID,
			chEvent.TargetSMFID,
			chEvent.MatchOutcome,
			chEvent.RoundNumber,
		)
		if err != nil {
			p.logger.Warnw("Failed to append event to batch", "error", err, "event_type", event.Type)
			continue
		}

		// Process side effects (Redis state updates)
		// Batch processed later to optimize goroutines and I/O
	}

	// Process side effects in batch (Redis state updates)
	// Must copy batch because the slice is reused in the worker loop
	batchCopy := make([]Job, len(batch))
	copy(batchCopy, batch)
	go p.processBatchSideEffects(ctx, batchCopy)

	// Send batch to ClickHouse FIRST
	err = chBatch.Send()
	if err != nil {
		p.logger.Errorw("Failed to send batch to ClickHouse", "error", err, "batchSize", len(batch))
		return err
	}

	// THEN process achievements (after data is in ClickHouse)
	for _, job := range batch {
		event := job.Event
		if p.achievementWorker != nil {
			p.logger.Infow("Calling achievement worker", "event_type", event.Type, "attacker_smf_id", event.AttackerSMFID)
			go func(evt *models.RawEvent) {
				defer func() {
					if r := recover(); r != nil {
						p.logger.Errorw("Achievement worker panic", "error", r, "event_type", evt.Type)
					}
				}()
				p.achievementWorker.ProcessEvent(evt)
			}(event)
		}
	}

	return nil
}

// processBatchSideEffects processes side effects for a batch of events
func (p *Pool) processBatchSideEffects(ctx context.Context, batch []Job) {
	if len(batch) == 0 {
		return
	}

	// Phase 1: Segregation & Pipelining
	pipe := p.config.Redis.Pipeline()

	// Track what we need to check after pipeline execution
	type killCheck struct {
		guid string
		cmd  *redis.IntCmd
	}
	type headshotCheck struct {
		guid string
		cmd  *redis.IntCmd
	}

	var killChecks []killCheck
	var headshotChecks []headshotCheck
	var deferredEvents []*models.RawEvent

	for _, job := range batch {
		event := job.Event

		switch event.Type {
		case models.EventPlayerKill:
			if event.AttackerGUID != "" && event.AttackerGUID != "world" {
				key := "player:" + event.AttackerGUID + ":kills"
				cmd := pipe.Incr(ctx, key)
				killChecks = append(killChecks, killCheck{guid: event.AttackerGUID, cmd: cmd})
				// Also count headshots (derived from hitloc)
				if event.Hitloc == "head" || event.Hitloc == "helmet" {
					hsKey := "player:" + event.AttackerGUID + ":headshots"
					hsCmd := pipe.Incr(ctx, hsKey)
					headshotChecks = append(headshotChecks, headshotCheck{guid: event.AttackerGUID, cmd: hsCmd})
				}
			}
		case models.EventConnect:
			if event.PlayerGUID != "" {
				pipe.HSet(ctx, "player_names", event.PlayerGUID, event.PlayerName)
				pipe.SAdd(ctx, "match:"+event.MatchID+":players", event.PlayerGUID)
				if event.PlayerSMFID > 0 {
					pipe.HSet(ctx, "player_smfids", event.PlayerGUID, event.PlayerSMFID)
				}
			}
		case models.EventDisconnect:
			if event.PlayerGUID != "" {
				pipe.SRem(ctx, "match:"+event.MatchID+":players", event.PlayerGUID)
			}
		case models.EventTeamJoin:
			if event.PlayerGUID != "" && event.NewTeam != "" {
				pipe.HSet(ctx, "match:"+event.MatchID+":teams", event.PlayerGUID, event.NewTeam)
			}
		case models.EventPlayerSpawn:
			if event.PlayerGUID != "" && event.PlayerTeam != "" {
				pipe.HSet(ctx, "match:"+event.MatchID+":teams", event.PlayerGUID, event.PlayerTeam)
			}
		case models.EventMatchStart, models.EventMatchEnd, models.EventHeartbeat, models.EventChat, models.EventTeamWin:
			deferredEvents = append(deferredEvents, event)
		default:
			deferredEvents = append(deferredEvents, event)
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		p.logger.Errorw("Redis pipeline failed", "error", err)
	}

	// Phase 2: Achievement Verification
	type potentialUnlock struct {
		guid          string
		achievementID string
		sIsMemberCmd  *redis.BoolCmd
	}
	var potentialUnlocks []potentialUnlock

	verifyPipe := p.config.Redis.Pipeline()

	for _, check := range killChecks {
		val, err := check.cmd.Result()
		if err == nil {
			if achievementID, ok := killThresholds[val]; ok {
				key := "player:" + check.guid + ":achievements"
				cmd := verifyPipe.SIsMember(ctx, key, achievementID)
				potentialUnlocks = append(potentialUnlocks, potentialUnlock{
					guid:          check.guid,
					achievementID: achievementID,
					sIsMemberCmd:  cmd,
				})
			}
		}
	}

	for _, check := range headshotChecks {
		val, err := check.cmd.Result()
		if err == nil {
			if achievementID, ok := headshotThresholds[val]; ok {
				key := "player:" + check.guid + ":achievements"
				cmd := verifyPipe.SIsMember(ctx, key, achievementID)
				potentialUnlocks = append(potentialUnlocks, potentialUnlock{
					guid:          check.guid,
					achievementID: achievementID,
					sIsMemberCmd:  cmd,
				})
			}
		}
	}

	if len(potentialUnlocks) > 0 {
		_, err := verifyPipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			p.logger.Errorw("Redis verification pipeline failed", "error", err)
		}
	}

	// Phase 3: Bulk Persistence
	type unlockToPersist struct {
		guid          string
		achievementID string
	}
	var newUnlocks []unlockToPersist

	for _, check := range potentialUnlocks {
		// If SIsMember returned false (not member), it's a new unlock
		if !check.sIsMemberCmd.Val() {
			newUnlocks = append(newUnlocks, unlockToPersist{
				guid:          check.guid,
				achievementID: check.achievementID,
			})
		}
	}

	if len(newUnlocks) > 0 {
		// 1. Bulk Insert to Postgres
		// Construct query: INSERT INTO player_achievements (player_guid, achievement_id, unlocked_at) VALUES ...
		var sb strings.Builder
		sb.WriteString("INSERT INTO player_achievements (player_guid, achievement_id, unlocked_at) VALUES ")
		vals := []interface{}{}
		now := time.Now()

		for i, unlock := range newUnlocks {
			n := i * 3
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "($%d, $%d, $%d)", n+1, n+2, n+3)
			vals = append(vals, unlock.guid, unlock.achievementID, now)
		}
		sb.WriteString(" ON CONFLICT (player_guid, achievement_id) DO NOTHING")

		_, err := p.config.Postgres.Exec(ctx, sb.String(), vals...)
		if err != nil {
			p.logger.Errorw("Failed to bulk insert achievements", "error", err, "count", len(newUnlocks))
		} else {
			for _, unlock := range newUnlocks {
				p.logger.Infow("Achievement unlocked", "player", unlock.guid, "achievement", unlock.achievementID)
			}
		}

		// 2. Mark as unlocked in Redis
		persistPipe := p.config.Redis.Pipeline()
		for _, unlock := range newUnlocks {
			key := "player:" + unlock.guid + ":achievements"
			persistPipe.SAdd(ctx, key, unlock.achievementID)
		}
		_, err = persistPipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			p.logger.Errorw("Redis persistence pipeline failed", "error", err)
		}
	}

	// Phase 4: Deferred Processing
	for _, event := range deferredEvents {
		p.processEventSideEffects(ctx, event)
	}
}

// minValidUnixTimestamp is 2020-01-01 00:00:00 UTC in seconds.
// Any event.Timestamp below this is treated as game-relative time (e.g. level.time),
// not a real Unix epoch, and we substitute the ingestion wall-clock time instead.
const minValidUnixTimestamp = 1577836800

// convertToClickHouseEvent normalizes a raw event for ClickHouse.
// receivedAt is the wall-clock time when the event was enqueued, used as fallback
// when event.Timestamp is game-relative (level.time) rather than Unix epoch.
func (p *Pool) convertToClickHouseEvent(event *models.RawEvent, rawJSON string, receivedAt time.Time) *models.ClickHouseEvent {
	// Parse match_id as UUID or generate a consistent one from the string
	matchID, err := uuid.Parse(event.MatchID)
	if err != nil {
		// Use a consistent namespace for non-standard match IDs
		namespace := uuid.MustParse("00000000-0000-0000-0000-000000000000")
		matchID = uuid.NewMD5(namespace, []byte(event.MatchID))
	}

	// Determine real wall-clock timestamp.
	// Game scripts send level.time (seconds since map load, e.g. 73.6),
	// which is NOT a Unix epoch. Detect this and use ingestion time instead.
	var ts time.Time
	if event.Timestamp >= minValidUnixTimestamp {
		sec := int64(event.Timestamp)
		nsec := int64((event.Timestamp - float64(sec)) * 1e9)
		ts = time.Unix(sec, nsec)
	} else {
		ts = receivedAt
	}

	ch := &models.ClickHouseEvent{
		Timestamp:    ts,
		MatchID:      matchID,
		ServerID:     event.ServerID,
		MapName:      event.MapName,
		EventType:    string(event.Type),
		Damage:       uint32(event.Damage),
		Hitloc:       event.Hitloc,
		Distance:     event.Distance,
		RoundNumber:  uint16(event.RoundNumber),
		RawJSON:      rawJSON,
		MatchOutcome: event.MatchOutcome,
	}

	// Set actor/target based on event type
	switch event.Type {
	case models.EventPlayerKill, models.EventPlayerBash, "bash", models.EventPlayerRoadkill, models.EventPlayerTeamkill, models.EventPlayerSuicide, models.EventPlayerCrushed, models.EventPlayerTelefragged, models.EventBotKilled:
		ch.ActorID = event.AttackerGUID
		ch.ActorName = sanitizeName(event.AttackerName)
		ch.ActorTeam = event.AttackerTeam
		ch.ActorSMFID = event.AttackerSMFID
		ch.ActorWeapon = event.Weapon
		ch.ActorPosX = event.AttackerX
		ch.ActorPosY = event.AttackerY
		ch.ActorPosZ = event.AttackerZ
		ch.ActorPitch = event.AttackerPitch
		ch.ActorYaw = event.AttackerYaw
		ch.ActorStance = event.AttackerStance

		ch.TargetID = event.VictimGUID
		ch.TargetName = sanitizeName(event.VictimName)
		ch.TargetTeam = event.VictimTeam
		ch.TargetSMFID = event.VictimSMFID
		ch.TargetPosX = event.VictimX
		ch.TargetPosY = event.VictimY
		ch.TargetPosZ = event.VictimZ
		ch.TargetStance = event.VictimStance

		ch.Hitloc = event.Hitloc

	case models.EventDamage, models.EventPlayerPain:
		ch.ActorID = event.AttackerGUID
		ch.ActorName = sanitizeName(event.AttackerName)
		ch.ActorSMFID = event.AttackerSMFID
		ch.ActorWeapon = event.Weapon
		ch.ActorStance = event.AttackerStance // If available

		ch.TargetID = event.VictimGUID
		ch.TargetName = sanitizeName(event.VictimName)
		ch.TargetSMFID = event.VictimSMFID
		ch.TargetStance = event.VictimStance

		ch.Damage = uint32(event.Damage)

	case models.EventWeaponFire, models.EventReload, models.EventWeaponChange:
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.ActorWeapon = event.Weapon
		ch.ActorPosX = event.PosX
		ch.ActorPosY = event.PosY
		ch.ActorPosZ = event.PosZ
		ch.ActorPitch = event.AimPitch
		ch.ActorYaw = event.AimYaw
		ch.ActorStance = event.PlayerStance

	case models.EventWeaponHit:
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.TargetID = event.TargetGUID
		ch.TargetName = sanitizeName(event.TargetName)
		ch.TargetSMFID = event.TargetSMFID
		ch.Hitloc = event.Hitloc
		ch.ActorWeapon = event.Weapon
		ch.ActorStance = event.PlayerStance
		ch.TargetStance = event.TargetStance

	case models.EventMatchOutcome:
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.ActorTeam = event.PlayerTeam
		// Use MatchOutcome column for Win/Loss flag (1=Win, 0=Loss)
		ch.MatchOutcome = event.MatchOutcome
		// Use ActorWeapon column for Gametype storage
		ch.ActorWeapon = event.Gametype

	case models.EventObjectiveCapture, models.EventObjectiveUpdate:
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.ActorTeam = event.PlayerTeam
		// Store objective string in ActorWeapon or TargetName if needed?
		// Actually raw_json has it, but lets put it in ActorWeapon for now
		ch.ActorWeapon = event.Objective

	case models.EventVehicleEnter, models.EventVehicleExit, models.EventVehicleCrash:
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.TargetID = event.Entity // Store vehicle entity name here
		ch.Hitloc = event.Seat     // Reuse Hitloc for Seat

	default:
		// Generic player event (Movement, Interaction, Items, etc.)
		ch.ActorID = event.PlayerGUID
		ch.ActorName = sanitizeName(event.PlayerName)
		ch.ActorSMFID = event.PlayerSMFID
		ch.ActorTeam = event.PlayerTeam
		ch.ActorPosX = event.PosX
		ch.ActorPosY = event.PosY
		ch.ActorPosZ = event.PosZ
		ch.ActorWeapon = event.Item // Pickup events store item in ActorWeapon
	}

	return ch
}

// processEventSideEffects handles real-time updates (Redis, achievements)
func (p *Pool) processEventSideEffects(ctx context.Context, event *models.RawEvent) {
	switch event.Type {
	case models.EventMatchStart:
		p.handleMatchStart(ctx, event)
	case models.EventMatchEnd:
		p.handleMatchEnd(ctx, event)
	case models.EventHeartbeat:
		p.handleHeartbeat(ctx, event)
	case models.EventPlayerKill:
		p.handleKill(ctx, event)
	case models.EventBotKilled:
		p.handleKill(ctx, event) // Bot kills count as kills
	case models.EventConnect:
		p.handleConnect(ctx, event)
	case models.EventDisconnect:
		p.handleDisconnect(ctx, event)
	case models.EventChat:
		p.handleChat(ctx, event)
	case models.EventTeamJoin:
		p.handleTeamChange(ctx, event)
	case models.EventPlayerSpawn:
		p.handleSpawn(ctx, event)
	case models.EventTeamWin:
		p.handleTeamWin(ctx, event)
	}
}

// handleMatchStart creates live match state in Redis
func (p *Pool) handleMatchStart(ctx context.Context, event *models.RawEvent) {
	liveMatch := models.LiveMatch{
		MatchID:     event.MatchID,
		ServerID:    event.ServerID,
		MapName:     event.MapName,
		Gametype:    event.Gametype,
		StartedAt:   time.Now(),
		RoundNumber: 1,
	}

	data, _ := json.Marshal(liveMatch)
	p.config.Redis.HSet(ctx, "live_matches", event.MatchID, data)
	p.config.Redis.SAdd(ctx, "active_match_ids", event.MatchID)

	// Clear any stale team data for this match
	p.config.Redis.Del(ctx, "match:"+event.MatchID+":teams")

	// Update server status
	p.updateServerStatus(ctx, event)
}

// handleMatchEnd removes from live matches, triggers tournament advancement
func (p *Pool) handleMatchEnd(ctx context.Context, event *models.RawEvent) {
	// Retrieve winning team from live match cache if not in event
	winningTeam := event.WinningTeam
	if winningTeam == "" {
		data, err := p.config.Redis.HGet(ctx, "live_matches", event.MatchID).Bytes()
		if err == nil {
			var liveMatch models.LiveMatch
			if err := json.Unmarshal(data, &liveMatch); err == nil {
				// We might store winning team in liveMatch structure if we update it on team_win
				// But for now, let's assume event.WinningTeam is populated or we rely on team_win event
			}
		}
	}

	// Synthesize Match Outcome Events
	// Get all players and their teams
	teams, err := p.config.Redis.HGetAll(ctx, "match:"+event.MatchID+":teams").Result()
	if err == nil {
		// Get Gametype from LiveMatch to pass to event
		var gametype string
		if data, err := p.config.Redis.HGet(ctx, "live_matches", event.MatchID).Bytes(); err == nil {
			var lm models.LiveMatch
			if json.Unmarshal(data, &lm) == nil {
				gametype = lm.Gametype
			}
		}

		// Prepare pipeline for SMF ID and Name lookups
		pipe := p.config.Redis.Pipeline()
		smfLookups := make(map[string]*redis.StringCmd)
		nameLookups := make(map[string]*redis.StringCmd)
		for guid := range teams {
			smfLookups[guid] = pipe.HGet(ctx, "player_smfids", guid)
			nameLookups[guid] = pipe.HGet(ctx, "player_names", guid)
		}
		pipe.Exec(ctx)

		for guid, team := range teams {
			outcome := 0 // Loss
			if team == winningTeam {
				outcome = 1 // Win
			}

			// Get SMFID and Name from lookup result
			var smfid int64
			if cmd, ok := smfLookups[guid]; ok {
				if val, err := cmd.Result(); err == nil {
					fmt.Sscanf(val, "%d", &smfid)
				}
			}
			playerName := ""
			if cmd, ok := nameLookups[guid]; ok {
				playerName, _ = cmd.Result()
			}

			// Create Outcome Event
			go func(playerGUID, playerTeam, name string, won int, gType string, pid int64) {
				outcomeEvent := &models.RawEvent{
					Type:         models.EventMatchOutcome,
					MatchID:      event.MatchID,
					ServerID:     event.ServerID,
					MapName:      event.MapName,
					Timestamp:    float64(time.Now().Unix()),
					PlayerGUID:   playerGUID,
					PlayerName:   name,
					PlayerTeam:   playerTeam,
					Gametype:     gType,
					MatchOutcome: uint8(won), // 1 = win, 0 = loss
					PlayerSMFID:  pid,
				}
				p.Enqueue(outcomeEvent)
			}(guid, team, playerName, outcome, gametype, smfid)
		}
	}

	p.config.Redis.HDel(ctx, "live_matches", event.MatchID)
	p.config.Redis.SRem(ctx, "active_match_ids", event.MatchID)
	// Cleanup team data
	p.config.Redis.Del(ctx, "match:"+event.MatchID+":teams")
	p.config.Redis.Del(ctx, "match:"+event.MatchID+":players")

	// Tournament bracket advancement is handled by SMF plugin
	// See: smf-plugins/mohaa_tournaments/ for bracket management
}

// handleTeamWin records the winner in Redis so match_end can pick it up
func (p *Pool) handleTeamWin(ctx context.Context, event *models.RawEvent) {
	// Update live match with winner
	// We need to extend LiveMatch struct or just store it in a side key
	// distinct key for winner?
	p.config.Redis.HSet(ctx, "match:"+event.MatchID+":winner", "team", event.WinningTeam)
}

// handleTeamChange updates player team in Redis
func (p *Pool) handleTeamChange(ctx context.Context, event *models.RawEvent) {
	if event.PlayerGUID == "" || event.NewTeam == "" {
		return
	}
	p.config.Redis.HSet(ctx, "match:"+event.MatchID+":teams", event.PlayerGUID, event.NewTeam)
}

// handleSpawn also ensures team is set (backup for team_change)
func (p *Pool) handleSpawn(ctx context.Context, event *models.RawEvent) {
	if event.PlayerGUID == "" || event.PlayerTeam == "" {
		return
	}
	p.config.Redis.HSet(ctx, "match:"+event.MatchID+":teams", event.PlayerGUID, event.PlayerTeam)
}

// handleHeartbeat updates live match state and server status
func (p *Pool) handleHeartbeat(ctx context.Context, event *models.RawEvent) {
	// Update live match data
	data, err := p.config.Redis.HGet(ctx, "live_matches", event.MatchID).Bytes()
	if err == nil {
		var liveMatch models.LiveMatch
		if json.Unmarshal(data, &liveMatch) == nil {
			liveMatch.AlliesScore = event.AlliesScore
			liveMatch.AxisScore = event.AxisScore
			liveMatch.PlayerCount = event.PlayerCount
			liveMatch.RoundNumber = event.RoundNumber

			newData, _ := json.Marshal(liveMatch)
			p.config.Redis.HSet(ctx, "live_matches", event.MatchID, newData)
		}
	}

	// Update server status (Redis + DB)
	p.updateServerStatus(ctx, event)
}

// handleKill increments kill counters for achievements
func (p *Pool) handleKill(ctx context.Context, event *models.RawEvent) {
	if event.AttackerGUID == "" || event.AttackerGUID == "world" {
		return
	}

	// Increment kill counter
	key := "player:" + event.AttackerGUID + ":kills"
	newCount, _ := p.config.Redis.Incr(ctx, key).Result()

	// Check achievement thresholds
	p.checkKillAchievements(ctx, event.AttackerGUID, newCount)

	// If this was a headshot (hitloc is head or helmet), also count as headshot
	if event.Hitloc == "head" || event.Hitloc == "helmet" {
		p.handleHeadshot(ctx, event)
	}
}

// handleHeadshot increments headshot counters
func (p *Pool) handleHeadshot(ctx context.Context, event *models.RawEvent) {
	// Use attacker GUID since headshots are derived from player_kill events
	guid := event.AttackerGUID
	if guid == "" {
		guid = event.PlayerGUID // fallback
	}
	if guid == "" {
		return
	}

	key := "player:" + guid + ":headshots"
	newCount, _ := p.config.Redis.Incr(ctx, key).Result()

	p.checkHeadshotAchievements(ctx, guid, newCount)
}

// handleConnect updates player alias tracking
func (p *Pool) handleConnect(ctx context.Context, event *models.RawEvent) {
	if event.PlayerGUID == "" {
		return
	}

	// Update last known name
	p.config.Redis.HSet(ctx, "player_names", event.PlayerGUID, event.PlayerName)

	// Track player online status
	p.config.Redis.SAdd(ctx, "match:"+event.MatchID+":players", event.PlayerGUID)

	// Track player SMF ID if available
	if event.PlayerSMFID > 0 {
		p.config.Redis.HSet(ctx, "player_smfids", event.PlayerGUID, event.PlayerSMFID)
	}
}

// handleDisconnect updates player state
func (p *Pool) handleDisconnect(ctx context.Context, event *models.RawEvent) {
	if event.PlayerGUID == "" {
		return
	}

	p.config.Redis.SRem(ctx, "match:"+event.MatchID+":players", event.PlayerGUID)
}

// handleChat checks for claim codes
func (p *Pool) handleChat(ctx context.Context, event *models.RawEvent) {
	// Check if message is a claim code (format: !claim MOH-XXXX)
	msg := event.Message
	if len(msg) > 7 && msg[:7] == "!claim " {
		code := msg[7:]
		// Verify claim code exists in pending claims
		claimKey := "identity_claim:" + code
		userIDStr, err := p.config.Redis.Get(ctx, claimKey).Result()
		if err == nil && userIDStr != "" {
			// Mark claim as verified with player GUID
			p.config.Redis.HSet(ctx, claimKey+":verified",
				"player_guid", event.PlayerGUID,
				"verified_at", time.Unix(int64(event.Timestamp), 0).Format(time.RFC3339),
			)
			p.config.Logger.Sugar().Infow("Claim code verified", "code", code, "guid", event.PlayerGUID)
		}
	}
}

// checkKillAchievements checks kill-based achievements
func (p *Pool) checkKillAchievements(ctx context.Context, playerGUID string, killCount int64) {
	if achievementID, ok := killThresholds[killCount]; ok {
		p.grantAchievement(ctx, playerGUID, achievementID)
	}
}

// checkHeadshotAchievements checks headshot-based achievements
func (p *Pool) checkHeadshotAchievements(ctx context.Context, playerGUID string, count int64) {
	if achievementID, ok := headshotThresholds[count]; ok {
		p.grantAchievement(ctx, playerGUID, achievementID)
	}
}

// grantAchievement grants an achievement to a player
func (p *Pool) grantAchievement(ctx context.Context, playerGUID, achievementID string) {
	// Check if already unlocked
	key := "player:" + playerGUID + ":achievements"
	if p.config.Redis.SIsMember(ctx, key, achievementID).Val() {
		return
	}

	// Mark as unlocked
	p.config.Redis.SAdd(ctx, key, achievementID)

	// Insert into Postgres
	_, err := p.config.Postgres.Exec(ctx, `
		INSERT INTO player_achievements (player_guid, achievement_id, unlocked_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_guid, achievement_id) DO NOTHING
	`, playerGUID, achievementID, time.Now())

	if err != nil {
		p.logger.Warnw("Failed to grant achievement", "player", playerGUID, "achievement", achievementID, "error", err)
	} else {
		p.logger.Infow("Achievement unlocked", "player", playerGUID, "achievement", achievementID)
	}
}

func (p *Pool) reportQueueDepth() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			queueDepth.Set(float64(len(p.jobQueue)))
		case <-p.ctx.Done():
			return
		}
	}
}

// Helper functions

func sanitizeName(s string) string {
	// Fast path: check if any caret exists
	idx := strings.IndexByte(s, '^')
	if idx == -1 {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	current := 0
	for {
		idx := strings.IndexByte(s[current:], '^')
		if idx == -1 {
			sb.WriteString(s[current:])
			break
		}

		// Calculate absolute index
		absIdx := current + idx
		sb.WriteString(s[current:absIdx])

		// Check for color code
		if absIdx+1 < len(s) && s[absIdx+1] >= '0' && s[absIdx+1] <= '9' {
			// Skip both ^ and digit
			current = absIdx + 2
		} else {
			// Just a caret, write it
			sb.WriteByte('^')
			current = absIdx + 1
		}
	}

	return sb.String()
}

func parseOrGenerateUUID(s string) uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return id
	}
	// Generate deterministic UUID from string
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(s))
}

// updateServerStatus updates the server's live status in Redis and Postgres
func (p *Pool) updateServerStatus(ctx context.Context, event *models.RawEvent) {
	if event.ServerID == "" {
		return
	}

	// 1. Update Redis "live_servers"
	// Format: "players:%d,map:%s,gametype:%s"
	statusStr := fmt.Sprintf("players:%d,map:%s,gametype:%s",
		event.PlayerCount, event.MapName, event.Gametype)

	p.config.Redis.HSet(ctx, "live_servers", event.ServerID, statusStr)
	// Set expiration handling if needed? Redis Key itself doesn't expire, field doesn't expire.
	// Logic relies on IsOnline = true if entry exists AND LastSeen logic in Postgres which server_tracking uses
	// Actually server_tracking lines 155 checks if liveData != "" then sets IsOnline=true.
	// But if server crashes, entry remains?
	// We might need a "server_heartbeats" key with TTL or just rely on LastSeen for filtering.
	// But GetServerList uses live_servers to OVERRIDE isOnline.
	// So we should probably set an expiration or use a key with TTL per server.
	// For now, let's just set it.

	// 2. Update Postgres "servers" table "last_seen"
	// We do this asynchronously to avoid blocking worker too much, or just fire and forget
	go func() {
		defer func() { recover() }() // Safely ignore panics
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := p.config.Postgres.Exec(ctx, `
			UPDATE servers SET last_seen = NOW(), is_active = true WHERE id = $1
		`, event.ServerID)
		if err != nil {
			p.logger.Warnw("Failed to update server last_seen", "error", err, "server_id", event.ServerID)
		}
	}()
}
