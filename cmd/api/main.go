// OpenMOHAA Stats API - High-Throughput Telemetry Ingestion Server
//
// Architecture:
// - Buffered Worker Pool pattern for async event processing
// - Load shedding via backpressure when queue is full
// - Dual-store: ClickHouse (OLAP) + PostgreSQL (OLTP)
// - Redis for caching, rate limiting, and real-time match state

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/openmohaa/stats-api/internal/config"
	"github.com/openmohaa/stats-api/internal/db"
	"github.com/openmohaa/stats-api/internal/handlers"
	"github.com/openmohaa/stats-api/internal/worker"
)

func main() {
	// Initialize structured logger
	logger, _ := zap.NewProduction()
	if os.Getenv("ENV") == "development" {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	sugar.Info("OpenMOHAA Stats API starting up...")

	// Load configuration
	cfg := config.Load()
	sugar.Infow("Configuration loaded",
		"port", cfg.Port,
		"workers", cfg.WorkerCount,
		"queueSize", cfg.QueueSize,
	)

	// Initialize database connections
	ctx := context.Background()

	// PostgreSQL (OLTP - auth tokens, user mappings)
	pgPool, err := db.NewPostgresPool(ctx, cfg.PostgresURL)
	if err != nil {
		sugar.Fatalw("Failed to connect to PostgreSQL", "error", err)
	}
	defer pgPool.Close()
	sugar.Info("PostgreSQL connection established")

	// ClickHouse (OLAP - telemetry events)
	chConn, err := db.NewClickHouseConn(ctx, cfg.ClickHouseURL)
	if err != nil {
		sugar.Fatalw("Failed to connect to ClickHouse", "error", err)
	}
	defer chConn.Close()
	sugar.Info("ClickHouse connection established")

	// Redis (caching, rate limiting, real-time state)
	redisClient := db.NewRedisClient(cfg.RedisURL)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		sugar.Fatalw("Failed to connect to Redis", "error", err)
	}
	sugar.Info("Redis connection established")

	// Initialize worker pool for async event processing
	workerPool := worker.NewPool(worker.PoolConfig{
		WorkerCount:   cfg.WorkerCount,
		QueueSize:     cfg.QueueSize,
		BatchSize:     cfg.BatchSize,
		FlushInterval: cfg.FlushInterval,
		ClickHouse:    chConn,
		Postgres:      pgPool,
		Redis:         redisClient,
		Logger:        logger,
	})
	workerPool.Start(ctx)
	sugar.Infow("Worker pool started",
		"workers", cfg.WorkerCount,
		"queueSize", cfg.QueueSize,
	)

	// Achievement worker is now integrated into worker pool (no separate instance needed)

	// Initialize handlers
	h := handlers.New(handlers.Config{
		WorkerPool: workerPool,
		Postgres:   pgPool,
		ClickHouse: chConn,
		Redis:      redisClient,
		Logger:     logger,
		JWTSecret:  cfg.JWTSecret,
	})

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS for frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Server-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health & Metrics
	r.Get("/health", h.Health)
	r.Get("/ready", h.Ready)
	r.Handle("/metrics", promhttp.Handler())

	// API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Ingestion endpoints (from game servers)
		r.Route("/ingest", func(r chi.Router) {
			r.Use(h.ServerAuthMiddleware)
			r.Post("/events", h.IngestEvents)
			r.Post("/match-result", h.IngestMatchResult)
		})

		// Stats endpoints (for frontend)
		r.Route("/stats", func(r chi.Router) {
			r.Get("/global", h.GetGlobalStats)
			r.Get("/global/activity", h.GetServerActivity)
			r.Get("/server/pulse", h.GetServerPulse)
			r.Get("/server/maps", h.GetServerMaps)
			r.Get("/teams/performance", h.GetFactionPerformance) // [NEW]
			r.Get("/matches", h.GetMatches)
			r.Get("/weapons", h.GetGlobalWeaponStats)
			r.Get("/weapons/list", h.GetWeaponsList)     // [NEW] Simple list for dropdowns
			r.Get("/weapon/{weapon}", h.GetWeaponDetail) // [NEW] Single weapon details

			// Map statistics endpoints
			r.Get("/maps", h.GetMapStats)      // All maps with stats
			r.Get("/maps/list", h.GetMapsList) // Simple maps list
			r.Get("/maps/popularity", h.GetMapPopularity)
			r.Get("/map/{mapId}", h.GetMapDetail) // Single map details

			// Game type statistics endpoints (derived from map prefixes)
			r.Get("/gametypes", h.GetGameTypeStats)            // All game types with stats
			r.Get("/gametypes/list", h.GetGameTypesList)       // Simple list for dropdowns
			r.Get("/gametype/{gameType}", h.GetGameTypeDetail) // Single game type details
			r.Get("/leaderboard/gametype/{gameType}", h.GetGameTypeLeaderboard)

			r.Get("/leaderboard", h.GetLeaderboard)
			r.Get("/leaderboard/cards", h.GetLeaderboardCards)
			r.Get("/leaderboard/global", h.GetLeaderboard)
			r.Get("/leaderboard/weapon/{weapon}", h.GetWeaponLeaderboard)
			r.Get("/leaderboard/map/{map}", h.GetMapLeaderboard)

			r.Get("/player/name/{name}", h.GetPlayerStatsByName)
			r.Get("/player/{guid}", h.GetPlayerStats)
			r.Get("/player/{guid}/deep", h.GetPlayerDeepStats)
			r.Get("/player/{guid}/combat", h.GetPlayerCombatStats)     // Subset of deep stats
			r.Get("/player/{guid}/movement", h.GetPlayerMovementStats) // Subset of deep stats
			r.Get("/player/{guid}/stance", h.GetPlayerStanceStats)     // Subset of deep stats
			r.Get("/player/{guid}/matches", h.GetPlayerMatches)
			r.Get("/player/{guid}/weapons", h.GetPlayerWeaponStats)
			r.Get("/player/{guid}/gametypes", h.GetPlayerStatsByGametype)
			r.Get("/player/{guid}/maps", h.GetPlayerStatsByMap)
			r.Get("/player/{guid}/heatmap/{map}", h.GetPlayerHeatmap)
			r.Get("/player/{guid}/deaths/{map}", h.GetPlayerDeathHeatmap)
			r.Get("/player/{guid}/heatmap/body", h.GetPlayerBodyHeatmap)
			r.Get("/player/{guid}/performance", h.GetPlayerPerformanceHistory)
			r.Get("/player/{guid}/playstyle", h.GetPlayerPlaystyle) // [NEW]

			// Advanced Stats endpoints - "When" analysis, drill-down, combinations
			r.Get("/player/{guid}/peak-performance", h.GetPlayerPeakPerformance)
			r.Get("/player/{guid}/combos", h.GetPlayerComboMetrics)
			r.Get("/player/{guid}/drilldown", h.GetPlayerDrillDown)
			r.Get("/player/{guid}/vehicles", h.GetPlayerVehicleStats)
			r.Get("/player/{guid}/game-flow", h.GetPlayerGameFlowStats)
			r.Get("/player/{guid}/world", h.GetPlayerWorldStats)
			r.Get("/player/{guid}/bots", h.GetPlayerBotStats)

			r.Get("/map/{map}/heatmap", h.GetMapHeatmap)

			r.Get("/match/{matchId}", h.GetMatchDetails)
			r.Get("/match/{matchId}/advanced", h.GetMatchAdvancedDetails) // [NEW]
			r.Get("/match/{matchId}/timeline", h.GetMatchTimeline)
			r.Get("/match/{matchId}/heatmap", h.GetMatchHeatmap)

			r.Get("/query", h.GetDynamicStats)
			r.Get("/server/{serverId}/stats", h.GetServerStats)
			r.Get("/live/matches", h.GetLiveMatches)
		})

		// Tournament endpoints
		r.Route("/tournaments", func(r chi.Router) {
			r.Get("/", h.GetTournaments)
			r.Get("/{id}", h.GetTournament)
			r.Get("/{id}/stats", h.GetTournamentStats)
		})

		// Server tracking endpoints (New Dashboard System)
		r.Route("/servers", func(r chi.Router) {
			r.Get("/", h.GetAllServers)                                   // List all servers with live status
			r.Get("/stats", h.GetServersGlobalStats)                      // Aggregate stats across all servers
			r.Get("/rankings", h.GetServerRankings)                       // Ranked server list
			r.Get("/favorites", h.GetUserFavoriteServers)                 // User's favorite servers
			r.Get("/{id}", h.GetServerDetail)                             // Full server details
			r.Get("/{id}/live", h.GetServerLiveStatus)                    // Real-time server status
			r.Get("/{id}/player-history", h.GetServerPlayerHistory)       // Player count history
			r.Get("/{id}/peak-hours", h.GetServerPeakHours)               // Peak hours heatmap
			r.Get("/{id}/top-players", h.GetServerTopPlayers)             // Top players on server
			r.Get("/{id}/players", h.GetServerHistoricalPlayers)          // All players historical data
			r.Get("/{id}/maps", h.GetServerMapStats)                      // Map statistics
			r.Get("/{id}/map-rotation", h.GetServerMapRotation)           // Map rotation analysis
			r.Get("/{id}/weapons", h.GetServerWeaponStats)                // Weapon statistics
			r.Get("/{id}/matches", h.GetServerRecentMatches)              // Recent matches
			r.Get("/{id}/activity-timeline", h.GetServerActivityTimeline) // Activity over time
			r.Get("/{id}/countries", h.GetServerCountryStats)             // Player country distribution
			r.Get("/{id}/favorite", h.CheckServerFavorite)                // Check if favorited
			r.Post("/{id}/favorite", h.AddServerFavorite)                 // Add to favorites
			r.Delete("/{id}/favorite", h.RemoveServerFavorite)            // Remove from favorites
		})

		// Achievement endpoints - match/tournament specific
		// r.Get("/achievements/match/{match_id}", h.GetMatchAchievements)
		// r.Get("/achievements/tournament/{tournament_id}", h.GetTournamentAchievements)

		// Auth endpoints
		r.Route("/auth", func(r chi.Router) {
			// Device Code Flow for headless game clients
			r.Post("/device", h.InitDeviceAuth)
			r.Post("/token", h.PollDeviceToken)
			r.Post("/verify", h.VerifyToken)
			r.Get("/history", h.GetLoginHistory)

			// Trusted IP management
			r.Get("/trusted-ips", h.GetTrustedIPs)
			r.Delete("/trusted-ips/{id}", h.DeleteTrustedIP)

			// Pending IP approvals
			r.Get("/pending-ips", h.GetPendingIPApprovals)
			r.Post("/pending-ips/{id}", h.ResolvePendingIPApproval)
			r.Post("/pending-ips/mark-notified", h.MarkPendingIPsNotified)

			// Identity claiming (in-game verification)
			r.Post("/claim/init", h.InitIdentityClaim)
			r.Post("/claim/verify", h.VerifyIdentityClaim)
			r.Post("/smf-verify", h.SMFVerifyToken) // Legacy tracker script auth
		})

		// User endpoints
		r.Route("/users", func(r chi.Router) {
			r.Use(h.UserAuthMiddleware)
			r.Get("/me", h.GetCurrentUser)
			r.Put("/me", h.UpdateCurrentUser)
			r.Get("/me/identities", h.GetUserIdentities)
			r.Delete("/me/identities/{id}", h.UnlinkIdentity)
		})

		// Achievement endpoints
		r.Route("/achievements", func(r chi.Router) {
			r.Get("/", h.ListAchievements)
			r.Get("/recent", h.GetRecentAchievements)
			r.Get("/leaderboard", h.GetAchievementLeaderboard)
			r.Get("/{id}", h.GetAchievement)
			r.Get("/player/{guid}", h.GetPlayerAchievements)
		})
	})

	// HTMX partial endpoints (for frontend SSR)
	r.Route("/partials", func(r chi.Router) {
		r.Get("/live-matches", h.PartialLiveMatches)
		r.Get("/leaderboard", h.PartialLeaderboard)
		r.Get("/recent-matches", h.PartialRecentMatches)
		r.Get("/player-card/{guid}", h.PartialPlayerCard)
		r.Get("/player/{guid}/matches", h.PartialPlayerMatches)
	})

	// Frontend routes (SSR HTML pages)
	r.Route("/", func(r chi.Router) {
		r.Get("/", h.PageIndex)
		r.Get("/login", h.PageLogin)
		r.Get("/player/{guid}", h.PagePlayer)
		r.Get("/leaderboard", h.PageLeaderboard)
		r.Get("/match/{matchId}", h.PageMatch)
		r.Get("/stats", h.PageStats)
		r.Get("/maps", h.PageMaps)
		r.Get("/maps/{mapId}", h.PageMapDetail)
	})

	// Static files for frontend
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		sugar.Infof("Server listening on port %d", cfg.Port)
		sugar.Infof("Workers: %d | Queue Size: %d | CPUs: %d",
			cfg.WorkerCount, cfg.QueueSize, runtime.NumCPU())

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalw("Server failed", "error", err)
		}
	}()

	<-shutdown
	sugar.Info("Shutting down gracefully...")

	// Give workers time to flush
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workerPool.Stop()
	server.Shutdown(ctx)

	sugar.Info("Server stopped")
}
