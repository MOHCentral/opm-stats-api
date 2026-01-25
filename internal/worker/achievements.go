package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/openmohaa/stats-api/internal/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DBStore abstracts the database operations
type DBStore interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// StatStore abstracts the storage for player statistics (e.g., Redis)
type StatStore interface {
	Incr(ctx context.Context, key string) (int64, error)
	IncrByFloat(ctx context.Context, key string, value float64) (float64, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}

// RedisStatStore implements StatStore using Redis
type RedisStatStore struct {
	client *redis.Client
}

func (s *RedisStatStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

func (s *RedisStatStore) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	return s.client.IncrByFloat(ctx, key, value).Result()
}

func (s *RedisStatStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s *RedisStatStore) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

// AchievementWorker processes events and unlocks achievements
type AchievementWorker struct {
	db              DBStore            // Postgres for achievement defs and unlocks
	ch              driver.Conn        // ClickHouse for stats queries
	statStore       StatStore          // Redis for stats
	logger          *zap.SugaredLogger // Logger for debugging
	achievementDefs map[string]*AchievementDefinition
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
}

// AchievementDefinition holds criteria for unlocking
type AchievementDefinition struct {
	Slug        string
	Category    string
	Tier        string
	Points      int
	Criteria    string // JSON criteria
	Description string
}

// NewAchievementWorker creates a new achievement processing worker
func NewAchievementWorker(db DBStore, ch driver.Conn, statStore StatStore, logger *zap.SugaredLogger) *AchievementWorker {
	ctx, cancel := context.WithCancel(context.Background())

	worker := &AchievementWorker{
		db:              db,
		ch:              ch,
		statStore:       statStore,
		logger:          logger,
		achievementDefs: make(map[string]*AchievementDefinition),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Load achievement definitions from database
	if err := worker.loadAchievementDefinitions(); err != nil {
		logger.Errorw("Failed to load achievement definitions", "error", err)
	}

	return worker
}

// Start begins the achievement worker
func (w *AchievementWorker) Start() {
	w.logger.Info("Achievement Worker started")
}

// Stop gracefully stops the worker
func (w *AchievementWorker) Stop() {
	w.cancel()
	w.logger.Info("Achievement Worker stopped")
}

// loadAchievementDefinitions loads all achievements from database
func (w *AchievementWorker) loadAchievementDefinitions() error {
	query := `
		SELECT achievement_code, category, tier, points, requirement_value::text, achievement_name
		FROM mohaa_achievements
	`

	rows, err := w.db.Query(w.ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query achievements: %w", err)
	}
	defer rows.Close()

	w.mu.Lock()
	defer w.mu.Unlock()

	count := 0
	for rows.Next() {
		def := &AchievementDefinition{}
		err := rows.Scan(
			&def.Slug,
			&def.Category,
			&def.Tier,
			&def.Points,
			&def.Criteria,
			&def.Description,
		)
		if err != nil {
			w.logger.Errorw("Failed to scan achievement", "error", err)
			continue
		}

		w.achievementDefs[def.Slug] = def
		count++
	}

	w.logger.Infow("Loaded achievement definitions", "count", count)
	return nil
}

// ProcessEvent checks if an event triggers any achievements
func (w *AchievementWorker) ProcessEvent(event *models.RawEvent) {
	// Determine Actor ID based on event type
	actorSMFID := w.getActorSMFID(event)
	w.logger.Infow("Processing achievement event",
		"type", event.Type,
		"actorSMFID", actorSMFID,
		"timestamp", event.Timestamp,
	)

	if actorSMFID == 0 {
		w.logger.Infow("Skipping achievement check - no authenticated player", "type", event.Type)
		return // Only process for authenticated players
	}

	// Check different event types
	switch event.Type {
	case models.EventKill:
		w.logger.Infow("Checking combat achievements", "smfID", actorSMFID)
		w.checkCombatAchievements(actorSMFID, event)
	case models.EventHeadshot:
		w.checkHeadshotAchievements(actorSMFID, event)
	case models.EventDistance:
		w.checkMovementAchievements(actorSMFID, event)
	case models.EventVehicleEnter:
		w.checkVehicleAchievements(actorSMFID, event)
	case models.EventItemPickup, models.EventHealthPickup:
		w.checkSurvivalAchievements(actorSMFID, event)
	case models.EventObjectiveUpdate: // Assuming objective_complete maps to this or similar
		w.checkObjectiveAchievements(actorSMFID, event)
	case models.EventTeamWin: // Assuming round_win maps to this
		w.checkTeamplayAchievements(actorSMFID, event)
	}
}

// getActorSMFID resolves the primary actor's SMF ID for the event
func (w *AchievementWorker) getActorSMFID(event *models.RawEvent) int64 {
	// For combat events where the killer is the actor
	if event.Type == models.EventKill || event.Type == models.EventHeadshot || event.Type == models.EventDamage {
		return event.AttackerSMFID
	}
	// For most other events, it's the player
	return event.PlayerSMFID
}

// checkCombatAchievements checks for combat-related achievements
func (w *AchievementWorker) checkCombatAchievements(smfID int64, event *models.RawEvent) {
	w.logger.Infow("[ACHIEVEMENT] checkCombatAchievements called", "smfID", smfID)
	// Get player's total kills
	totalKills := w.incrementPlayerStat(int(smfID), "total_kills")

	// Check for vehicle kills
	if strings.Contains(event.Inflictor, "vehicle") {
		w.incrementPlayerStat(int(smfID), "vehicle_kills")
	}

	w.logger.Infow("Player kill stats",
		"smfID", smfID,
		"totalKills", totalKills,
	)

	serverID := 0
	// Try parsing ServerID if needed, or default to 0

	ts := time.Unix(int64(event.Timestamp), 0)

	// Check milestone achievements
	milestones := map[string]int{
		"first-blood":     1,
		"killer-bronze":   10,
		"killer-silver":   50,
		"killer-gold":     100,
		"killer-platinum": 500,
		"killer-diamond":  1000,
		"killing-spree":   5,  // In single match
		"unstoppable":     10, // In single match
		"legendary":       20, // In single match
	}

	w.logger.Infow("Checking milestones", "totalKills", totalKills, "milestoneCount", len(milestones))

	for slug, threshold := range milestones {
		w.logger.Debugw("Checking milestone", "slug", slug, "threshold", threshold, "totalKills", totalKills, "passes", totalKills >= threshold)
		if totalKills >= threshold {
			w.logger.Infow("Achievement milestone reached!",
				"slug", slug,
				"threshold", threshold,
				"totalKills", totalKills,
				"smfID", smfID,
			)
			// unlockAchievement checks if already unlocked, so it's safe to call multiple times
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}

	// Check weapon-specific achievements
	if event.Weapon != "" {
		w.checkWeaponMasteryAchievement(int(smfID), event.Weapon, serverID, ts)
	}

	// Check multikill achievements
	w.checkMultikillAchievement(int(smfID), event)
}

// checkHeadshotAchievements checks headshot-based achievements
func (w *AchievementWorker) checkHeadshotAchievements(smfID int64, event *models.RawEvent) {
	totalHeadshots := w.incrementPlayerStat(int(smfID), "total_headshots")

	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	milestones := map[string]int{
		"sharpshooter-bronze":   10,
		"sharpshooter-silver":   50,
		"sharpshooter-gold":     100,
		"sharpshooter-platinum": 250,
		"sharpshooter-diamond":  500,
	}

	for slug, threshold := range milestones {
		if totalHeadshots == threshold {
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}

	// Check headshot streak
	w.checkHeadshotStreakAchievement(int(smfID), event)
}

// checkMovementAchievements checks distance and movement achievements
func (w *AchievementWorker) checkMovementAchievements(smfID int64, event *models.RawEvent) {
	delta := float64(event.Walked + event.Sprinted + event.Swam + event.Driven)
	totalDistance := w.incrementPlayerStatFloat(int(smfID), "total_distance", delta)

	// Convert to kilometers
	distanceKM := totalDistance / 1000.0

	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	milestones := map[string]float64{
		"marathoner-bronze":   10,
		"marathoner-silver":   50,
		"marathoner-gold":     100,
		"marathoner-platinum": 250,
		"marathoner-diamond":  500,
	}

	for slug, threshold := range milestones {
		if distanceKM >= threshold && distanceKM < threshold+0.1 {
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}
}

// checkVehicleAchievements checks vehicle-related achievements
func (w *AchievementWorker) checkVehicleAchievements(smfID int64, event *models.RawEvent) {
	vehicleKills := w.getPlayerStat(int(smfID), "vehicle_kills")

	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	milestones := map[string]int{
		"tanker-bronze":   5,
		"tanker-silver":   25,
		"tanker-gold":     50,
		"tanker-platinum": 100,
		"tanker-diamond":  250,
	}

	for slug, threshold := range milestones {
		if vehicleKills == threshold {
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}
}

// checkSurvivalAchievements checks survival and healing achievements
func (w *AchievementWorker) checkSurvivalAchievements(smfID int64, event *models.RawEvent) {
	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	if event.Type == models.EventHealthPickup {
		healthPickups := w.incrementPlayerStat(int(smfID), "health_pickups")

		milestones := map[string]int{
			"medic-bronze":   10,
			"medic-silver":   50,
			"medic-gold":     100,
			"medic-platinum": 250,
			"medic-diamond":  500,
		}

		for slug, threshold := range milestones {
			if healthPickups == threshold {
				w.unlockAchievement(int(smfID), slug, serverID, ts)
			}
		}
	}
}

// checkObjectiveAchievements checks objective-based achievements
func (w *AchievementWorker) checkObjectiveAchievements(smfID int64, event *models.RawEvent) {
	var totalObjectives int
	if event.Type == models.EventObjectiveCapture {
		totalObjectives = w.incrementPlayerStat(int(smfID), "objectives_completed")
	} else {
		totalObjectives = w.getPlayerStat(int(smfID), "objectives_completed")
	}

	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	milestones := map[string]int{
		"objective-bronze":   5,
		"objective-silver":   25,
		"objective-gold":     50,
		"objective-platinum": 100,
		"objective-diamond":  250,
	}

	for slug, threshold := range milestones {
		if totalObjectives == threshold {
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}
}

// checkTeamplayAchievements checks team-based achievements
func (w *AchievementWorker) checkTeamplayAchievements(smfID int64, event *models.RawEvent) {
	totalWins := w.incrementPlayerStat(int(smfID), "total_wins")

	serverID := 0
	ts := time.Unix(int64(event.Timestamp), 0)

	milestones := map[string]int{
		"winner-bronze":   10,
		"winner-silver":   25,
		"winner-gold":     50,
		"winner-platinum": 100,
		"winner-diamond":  250,
	}

	for slug, threshold := range milestones {
		if totalWins == threshold {
			w.unlockAchievement(int(smfID), slug, serverID, ts)
		}
	}
}

// Helper functions

func (w *AchievementWorker) checkWeaponMasteryAchievement(smfID int, weapon string, serverID int, ts time.Time) {
	weaponKills := w.getWeaponKills(smfID, weapon)

	// Example: 100 kills with Kar98k unlocks "Sniper Master"
	if weapon == "kar98k" && weaponKills == 100 {
		w.unlockAchievement(smfID, "sniper-master", serverID, ts)
	}
}

func (w *AchievementWorker) checkMultikillAchievement(smfID int, event *models.RawEvent) {
	// Would check recent kills within time window
	// For now, simplified
}

func (w *AchievementWorker) checkHeadshotStreakAchievement(smfID int, event *models.RawEvent) {
	// Would check consecutive headshots
	// For now, simplified
}

// incrementPlayerStat increments a stat in Redis and backfills from ClickHouse if needed
func (w *AchievementWorker) incrementPlayerStat(smfID int, statName string) int {
	key := fmt.Sprintf("stats:smf:%d:%s", smfID, statName)

	// Increment in Redis
	val, err := w.statStore.Incr(w.ctx, key)
	if err != nil {
		w.logger.Errorw("Failed to increment player stat", "key", key, "error", err)
		return w.fetchFromDB(smfID, statName)
	}

	// If value is 1, check if we need to initialize from DB
	if val == 1 {
		baseline := w.fetchFromDB(smfID, statName)
		if baseline > 1 {
			// Backfill Redis with correct value
			w.statStore.Set(w.ctx, key, baseline, 0)
			return baseline
		}
	}

	return int(val)
}

// incrementPlayerStatFloat increments a float stat (like distance)
func (w *AchievementWorker) incrementPlayerStatFloat(smfID int, statName string, incrAmount float64) float64 {
	key := fmt.Sprintf("stats:smf:%d:%s", smfID, statName)

	val, err := w.statStore.IncrByFloat(w.ctx, key, incrAmount)
	if err != nil {
		w.logger.Errorw("Failed to increment player stat float", "key", key, "error", err)
		return float64(w.fetchFromDB(smfID, statName))
	}

	// Check if this looks like a fresh key (val is close to increment amount)
	if val <= incrAmount+0.1 {
		baseline := w.fetchFromDB(smfID, statName)
		if float64(baseline) > val {
			w.statStore.Set(w.ctx, key, baseline, 0)
			return float64(baseline)
		}
	}

	return val
}

// getPlayerStat retrieves a player stat from Redis, falling back to ClickHouse
func (w *AchievementWorker) getPlayerStat(smfID int, statName string) int {
	key := fmt.Sprintf("stats:smf:%d:%s", smfID, statName)

	valStr, err := w.statStore.Get(w.ctx, key)
	if err == nil {
		val, _ := strconv.Atoi(valStr)
		return val
	}

	// Fallback to DB
	baseline := w.fetchFromDB(smfID, statName)
	w.statStore.Set(w.ctx, key, baseline, 0)
	return baseline
}

// fetchFromDB retrieves a player stat from ClickHouse (DB fallback)
func (w *AchievementWorker) fetchFromDB(smfID int, statName string) int {
	// Map stat names to ClickHouse queries
	var query string
	switch statName {
	case "total_kills":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE actor_smf_id = ? AND event_type = 'kill'`
	case "total_headshots":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE actor_smf_id = ? AND event_type = 'kill' AND hitloc = 'head'`
	case "total_distance":
		query = `SELECT SUM(walked + sprinted + swam + driven) FROM mohaa_stats.raw_events WHERE player_smf_id = ? AND event_type = 'distance'`
	case "vehicle_kills":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE actor_smf_id = ? AND event_type = 'kill' AND inflictor LIKE '%vehicle%'`
	case "health_pickups":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE player_smf_id = ? AND event_type = 'item_pickup' AND item LIKE '%health%'`
	case "objectives_completed":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE player_smf_id = ? AND event_type = 'objective_capture'`
	case "total_wins":
		query = `SELECT count() FROM mohaa_stats.raw_events WHERE player_smf_id = ? AND event_type = 'team_win'`
	default:
		return 0
	}

	var value uint64
	err := w.ch.QueryRow(w.ctx, query, smfID).Scan(&value)
	if err != nil {
		w.logger.Errorw("ClickHouse query error",
			"statName", statName,
			"smfID", smfID,
			"query", query,
			"error", err,
		)
		return 0
	}

	w.logger.Debugw("Retrieved player stat from DB",
		"statName", statName,
		"smfID", smfID,
		"value", value,
	)
	return int(value)
}

// getWeaponKills gets kills for specific weapon
func (w *AchievementWorker) getWeaponKills(smfID int, weapon string) int {
	// TODO: This needs to query ClickHouse, not Postgres
	// query := `
	// 	SELECT COALESCE(COUNT(*), 0)
	// 	FROM raw_events
	// 	WHERE actor_smf_id = $1
	// 	  AND event_type = 'player_kill'
	// 	  AND extra->>'weapon' = $2
	// `

	// var count int
	// err := w.db.QueryRow(w.ctx, query, smfID, weapon).Scan(&count)
	// if err != nil {
	// 	return 0
	// }

	return 0
}

// unlockAchievement records an achievement unlock
func (w *AchievementWorker) unlockAchievement(smfID int, slug string, serverID int, timestamp time.Time) {
	// Get achievement ID from code
	var achievementID int
	getIDQuery := `
		SELECT achievement_id FROM mohaa_achievements WHERE achievement_code = $1
	`
	err := w.db.QueryRow(w.ctx, getIDQuery, slug).Scan(&achievementID)
	if err != nil {
		w.logger.Errorw("Achievement code not found in database",
			"slug", slug,
			"error", err,
		)
		return
	}

	// Check if already unlocked
	var exists bool
	checkQuery := `
		SELECT EXISTS(
			SELECT 1 FROM mohaa_player_achievements
			WHERE smf_member_id = $1 AND achievement_id = $2 AND unlocked = true
		)
	`
	err = w.db.QueryRow(w.ctx, checkQuery, smfID, achievementID).Scan(&exists)
	if err != nil {
		w.logger.Errorw("Error checking existing achievement", "error", err)
		return
	}
	if exists {
		w.logger.Debugw("Achievement already unlocked", "slug", slug, "smfID", smfID)
		return // Already unlocked
	}

	// Get achievement details
	w.mu.RLock()
	def, exists := w.achievementDefs[slug]
	w.mu.RUnlock()

	if !exists {
		w.logger.Errorw("Achievement definition not found in memory", "slug", slug)
		return
	}

	// Update or insert player achievement record
	insertQuery := `
		INSERT INTO mohaa_player_achievements
		(smf_member_id, achievement_id, target, unlocked, unlocked_at, progress)
		VALUES ($1, $2, $3, true, $4, $3)
		ON CONFLICT (smf_member_id, achievement_id) 
		DO UPDATE SET unlocked = true, unlocked_at = $4, progress = EXCLUDED.target
	`

	_, err = w.db.Exec(w.ctx, insertQuery, smfID, achievementID, 100, timestamp)
	if err != nil {
		w.logger.Errorw("Failed to insert achievement unlock",
			"slug", slug,
			"smfID", smfID,
			"achievementID", achievementID,
			"error", err,
		)
		return
	}

	// Note: Player achievement points can be calculated via SUM query
	// No need to maintain separate counter

	w.logger.Infow("🏆 Achievement unlocked!",
		"slug", slug,
		"smfID", smfID,
		"points", def.Points,
		"description", def.Description,
	)

	// TODO: Send notification to player
	w.notifyPlayer(smfID, slug, def)
}

// notifyPlayer sends achievement notification (placeholder)
func (w *AchievementWorker) notifyPlayer(smfID int, slug string, def *AchievementDefinition) {
	// Would send WebSocket notification or queue for next page load
	notification := map[string]interface{}{
		"type":        "achievement_unlock",
		"smf_id":      smfID,
		"slug":        slug,
		"title":       def.Description,
		"tier":        def.Tier,
		"points":      def.Points,
		"unlocked_at": time.Now(),
	}

	jsonData, _ := json.Marshal(notification)
	w.logger.Debugw("Achievement notification", "data", string(jsonData))

	// In production, would use Redis pub/sub or WebSocket
}

// ProcessBatch processes multiple events in batch
func (w *AchievementWorker) ProcessBatch(events []*models.RawEvent) {
	for _, event := range events {
		w.ProcessEvent(event)
	}
}

// ReloadDefinitions reloads achievement definitions from database
func (w *AchievementWorker) ReloadDefinitions() error {
	return w.loadAchievementDefinitions()
}
