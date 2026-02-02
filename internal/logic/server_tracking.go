package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/openmohaa/stats-api/internal/models"
)

// ServerTrackingService provides comprehensive server monitoring
type ServerTrackingService struct {
	ch    driver.Conn
	pg    PgPool
	redis RedisClient
}

func NewServerTrackingService(ch driver.Conn, pg *pgxpool.Pool, redis *redis.Client) *ServerTrackingService {
	return &ServerTrackingService{ch: ch, pg: pg, redis: redis}
}

// =============================================================================
// SERVER LIST & OVERVIEW
// =============================================================================



// GetServerList returns all servers with live status
func (s *ServerTrackingService) GetServerList(ctx context.Context) ([]models.ServerOverview, error) {
	// Get registered servers from PostgreSQL
	rows, err := s.pg.Query(ctx, `
		SELECT id, name, COALESCE(ip_address, address, ''), COALESCE(port, 0), COALESCE(region, ''), 
		       total_matches, total_players, COALESCE(last_seen, created_at), is_active
		FROM servers
		ORDER BY total_players DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}
	defer rows.Close()

	var servers []models.ServerOverview
	var serverIDs []string
	rank := 1
	for rows.Next() {
		var srv models.ServerOverview
		var isActive bool
		err := rows.Scan(&srv.ID, &srv.Name, &srv.Address, &srv.Port,
			&srv.Region, &srv.TotalMatches, &srv.UniquePlayers,
			&srv.LastSeen, &isActive)
		if err != nil {
			continue
		}

		srv.MaxPlayers = 32 // Default max players
		srv.DisplayName = fmt.Sprintf("%s:%d", srv.Name, srv.Port)
		srv.Rank = rank
		rank++

		// Base online status (can be overridden by Redis)
		srv.IsOnline = isActive && time.Since(srv.LastSeen) < 5*time.Minute

		servers = append(servers, srv)
		serverIDs = append(serverIDs, srv.ID)
	}

	if len(servers) == 0 {
		return servers, nil
	}

	// 1. Batch Redis: Get live data for all servers at once
	liveServerMap, err := s.redis.HGetAll(ctx, "live_servers").Result()
	if err != nil {
		fmt.Printf("[DEBUG] Redis HGetAll error: %v\n", err)
	}
	fmt.Printf("[DEBUG] Redis live_servers: %v\n", liveServerMap)

	// 2. Batch ClickHouse: Get stats for all servers at once
	type ServerStats struct {
		TotalKills  int64
		AvgPlayers  float64
		PeakPlayers int
	}
	statsMap := make(map[string]ServerStats)

	if len(serverIDs) > 0 {
		// Optimized query aggregating by server_id
		rowsCH, err := s.ch.Query(ctx, `
			SELECT 
				server_id,
				sum(kills) as total_kills,
				avg(player_count) as avg_players,
				max(player_count) as peak
			FROM (
				SELECT 
					server_id,
					countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
					uniqExact(actor_id) as player_count
				FROM raw_events
				WHERE server_id IN (?) AND timestamp > now() - INTERVAL 24 HOUR
				GROUP BY server_id, toStartOfHour(timestamp)
			)
			GROUP BY server_id
		`, serverIDs)

		if err == nil {
			defer rowsCH.Close()
			for rowsCH.Next() {
				var sid string
				var st ServerStats
				if err := rowsCH.Scan(&sid, &st.TotalKills, &st.AvgPlayers, &st.PeakPlayers); err == nil {
					statsMap[sid] = st
				}
			}
		}
	}

	// 3. Merge Data
	for i := range servers {
		srv := &servers[i]

		// Redis Live Data
		if liveData, ok := liveServerMap[srv.ID]; ok && liveData != "" {
			srv.IsOnline = true
			parseServerLiveData(liveData, srv)
		}

		// ClickHouse Stats
		if st, ok := statsMap[srv.ID]; ok {
			srv.TotalKills = st.TotalKills
			srv.AvgPlayers24h = st.AvgPlayers
			srv.PeakPlayers24h = st.PeakPlayers
		}
	}

	return servers, nil
}

// GetServerGlobalStats returns aggregate stats across all servers
func (s *ServerTrackingService) GetServerGlobalStats(ctx context.Context) (*models.ServerGlobalStats, error) {
	stats := &models.ServerGlobalStats{}

	// Count servers from Postgres
	s.pg.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE is_active = true)
		FROM servers
	`).Scan(&stats.TotalServers, &stats.OnlineServers)

	// Get current players from Redis
	liveServers, _ := s.redis.HGetAll(ctx, "live_servers").Result()
	for _, data := range liveServers {
		var players int
		fmt.Sscanf(data, "players:%d", &players)
		stats.TotalPlayersNow += players
	}
	if stats.OnlineServers > 0 {
		stats.AvgPlayersNow = float64(stats.TotalPlayersNow) / float64(stats.OnlineServers)
	}

	// Today's stats from ClickHouse
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills_today,
			uniq(match_id) as matches_today,
			count() as total_kills_all
		FROM raw_events
		WHERE timestamp > today()
	`).Scan(&stats.TotalKillsToday, &stats.TotalMatchesToday, &stats.TotalKillsAllTime)

	return stats, nil
}

// =============================================================================
// INDIVIDUAL SERVER DETAIL
// =============================================================================

// ServerDetail contains comprehensive server information


// ServerLifetimeStats represents all-time server statistics


// ServerTimeStats represents time-windowed stats


// ServerUptime represents uptime tracking


// GetServerDetail returns comprehensive server information
func (s *ServerTrackingService) GetServerDetail(ctx context.Context, serverID string) (*models.ServerDetail, error) {
	detail := &models.ServerDetail{ID: serverID}

	// Get basic info from Postgres
	err := s.pg.QueryRow(ctx, `
		SELECT name, address, port, region, description, max_players, 
		       is_official, is_active, last_seen, created_at
		FROM servers WHERE id = $1
	`, serverID).Scan(&detail.Name, &detail.Address, &detail.Port, &detail.Region,
		&detail.Description, &detail.MaxPlayers, &detail.IsOfficial,
		&detail.IsOnline, &detail.Uptime.LastOnline, &detail.Stats.FirstSeen)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	detail.DisplayName = fmt.Sprintf("%s:%d", detail.Name, detail.Port)

	// Check live status
	liveData, err := s.redis.HGet(ctx, "live_servers", serverID).Result()
	if err == nil && liveData != "" {
		detail.IsOnline = true
		parseServerLiveData(liveData, nil) // Could parse current map/players
	}

	// Lifetime stats from ClickHouse
	// Note: deaths = kills for global stats (each kill = one death)
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			countIf(event_type IN ('player_kill', 'bot_killed')) as deaths,
			countIf(event_type = 'headshot') as headshots,
			uniq(match_id) as matches,
			uniq(actor_id) as players,
			sum(duration) / 3600.0 as playtime,
			avgIf(duration, event_type = 'match_end') / 60.0 as avg_match
		FROM raw_events
		WHERE server_id = ?
	`, serverID).Scan(&detail.Stats.TotalKills, &detail.Stats.TotalDeaths,
		&detail.Stats.TotalHeadshots, &detail.Stats.TotalMatches,
		&detail.Stats.UniquePlayers, &detail.Stats.TotalPlaytime,
		&detail.Stats.AvgMatchDuration)

	// 24h stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			uniq(match_id) as matches,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL 24 HOUR
	`, serverID).Scan(&detail.Stats24h.Kills, &detail.Stats24h.Matches, &detail.Stats24h.UniquePlayers)

	// 7d stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			uniq(match_id) as matches,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL 7 DAY
	`, serverID).Scan(&detail.Stats7d.Kills, &detail.Stats7d.Matches, &detail.Stats7d.UniquePlayers)

	// 30d stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			uniq(match_id) as matches,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL 30 DAY
	`, serverID).Scan(&detail.Stats30d.Kills, &detail.Stats30d.Matches, &detail.Stats30d.UniquePlayers)

	return detail, nil
}

// =============================================================================
// PLAYER HISTORY CHARTS
// =============================================================================

// PlayerHistoryPoint represents a data point for player count chart


// GetServerPlayerHistory returns player count over time
func (s *ServerTrackingService) GetServerPlayerHistory(ctx context.Context, serverID string, hours int) ([]models.PlayerHistoryPoint, error) {
	if hours <= 0 {
		hours = 24
	}

	query := `
		SELECT 
			toStartOfHour(timestamp) as ts,
			toHour(timestamp) as hour,
			max(player_count) as peak,
			avg(player_count) as avg_players
		FROM (
			SELECT 
				timestamp,
				uniqExact(actor_id) OVER (PARTITION BY toStartOfFiveMinutes(timestamp)) as player_count
			FROM raw_events
			WHERE server_id = ? AND timestamp > now() - INTERVAL ? HOUR
		)
		GROUP BY ts, hour
		ORDER BY ts
	`

	rows, err := s.ch.Query(ctx, query, serverID, hours)
	if err != nil {
		return nil, fmt.Errorf("player history query: %w", err)
	}
	defer rows.Close()

	var points []models.PlayerHistoryPoint
	for rows.Next() {
		var p models.PlayerHistoryPoint
		var ts time.Time
		if err := rows.Scan(&ts, &p.Hour, &p.Peak, &p.Avg); err != nil {
			continue
		}
		p.Timestamp = ts.Format(time.RFC3339)
		p.Players = p.Peak
		points = append(points, p)
	}

	return points, nil
}

// =============================================================================
// PEAK HOURS HEATMAP
// =============================================================================

// PeakHoursHeatmap represents activity by hour and day


// GetServerPeakHours returns a heatmap of peak activity times
func (s *ServerTrackingService) GetServerPeakHours(ctx context.Context, serverID string, days int) (*models.PeakHoursHeatmap, error) {
	if days <= 0 {
		days = 30
	}

	heatmap := &models.PeakHoursHeatmap{
		Data: make([][]int, 7),
		Hours: []string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11",
			"12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23"},
		Days: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
	}

	// Initialize data array
	for i := range heatmap.Data {
		heatmap.Data[i] = make([]int, 24)
	}

	query := `
		SELECT 
			toDayOfWeek(timestamp) as dow,
			toHour(timestamp) as hour,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL ? DAY
		GROUP BY dow, hour
		ORDER BY dow, hour
	`

	rows, err := s.ch.Query(ctx, query, serverID, days)
	if err != nil {
		return nil, fmt.Errorf("peak hours query: %w", err)
	}
	defer rows.Close()

	var peakPlayers int
	for rows.Next() {
		var dow, hour, players int
		if err := rows.Scan(&dow, &hour, &players); err != nil {
			continue
		}
		// ClickHouse: 1=Monday ... 7=Sunday
		dayIdx := dow - 1
		if dayIdx >= 0 && dayIdx < 7 && hour >= 0 && hour < 24 {
			heatmap.Data[dayIdx][hour] = players
			if players > peakPlayers {
				peakPlayers = players
				heatmap.Peak = models.PeakInfo{
					Day:     heatmap.Days[dayIdx],
					Hour:    hour,
					Players: players,
				}
			}
		}
	}

	return heatmap, nil
}

// =============================================================================
// TOP PLAYERS PER SERVER
// =============================================================================

// ServerTopPlayer represents a top player on a specific server


// GetServerTopPlayers returns top players for a specific server
func (s *ServerTrackingService) GetServerTopPlayers(ctx context.Context, serverID string, limit int) ([]models.ServerTopPlayer, error) {
	if limit <= 0 {
		limit = 25
	}

	// Deaths are counted from kill events where player is target_id
	query := `
		WITH deaths_cte AS (
			SELECT target_id, count() as death_count
			FROM raw_events
			WHERE server_id = ? AND event_type IN ('player_kill', 'bot_killed') AND target_id != ''
			GROUP BY target_id
		)
		SELECT 
			a.actor_id,
			any(a.actor_name) as name,
			countIf(a.event_type IN ('player_kill', 'bot_killed')) as kills,
			ifNull(max(d.death_count), 0) as deaths,
			countIf(a.event_type = 'headshot') as headshots,
			uniq(a.match_id) as sessions,
			max(a.timestamp) as last_seen
		FROM raw_events a
		LEFT JOIN deaths_cte d ON a.actor_id = d.target_id
		WHERE a.server_id = ? AND a.actor_id != ''
		GROUP BY a.actor_id
		ORDER BY kills DESC
		LIMIT ?
	`

	rows, err := s.ch.Query(ctx, query, serverID, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("top players query: %w", err)
	}
	defer rows.Close()

	var players []models.ServerTopPlayer
	rank := 1
	for rows.Next() {
		var p models.ServerTopPlayer
		var lastSeen time.Time
		if err := rows.Scan(&p.GUID, &p.Name, &p.Kills, &p.Deaths, &p.Headshots, &p.Sessions, &lastSeen); err != nil {
			continue
		}
		p.Rank = rank
		p.LastSeen = lastSeen.Format("2006-01-02 15:04")
		if p.Deaths > 0 {
			p.KDRatio = float64(p.Kills) / float64(p.Deaths)
		} else {
			p.KDRatio = float64(p.Kills)
		}
		if p.Kills > 0 {
			p.HSPercent = float64(p.Headshots) / float64(p.Kills) * 100
		}
		players = append(players, p)
		rank++
	}

	return players, nil
}

// =============================================================================
// MAP STATISTICS PER SERVER
// =============================================================================

// ServerMapStats represents map usage on a server


// GetServerMapStats returns map statistics for a server
func (s *ServerTrackingService) GetServerMapStats(ctx context.Context, serverID string) ([]models.ServerMapStats, error) {
	query := `
		WITH totals AS (
			SELECT uniq(match_id) as total_matches
			FROM raw_events WHERE server_id = ?
		)
		SELECT 
			map_name,
			uniq(match_id) as matches,
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			avg(player_count) as avg_players,
			avgIf(duration, event_type = 'match_end') / 60.0 as avg_duration,
			max(timestamp) as last_played,
			uniq(match_id) * 100.0 / (SELECT total_matches FROM totals) as popularity
		FROM raw_events
		WHERE server_id = ? AND map_name != ''
		GROUP BY map_name
		ORDER BY matches DESC
		LIMIT 20
	`

	rows, err := s.ch.Query(ctx, query, serverID, serverID)
	if err != nil {
		return nil, fmt.Errorf("map stats query: %w", err)
	}
	defer rows.Close()

	var maps []models.ServerMapStats
	for rows.Next() {
		var m models.ServerMapStats
		var lastPlayed time.Time
		var playerCount float64
		if err := rows.Scan(&m.MapName, &m.Matches, &m.Kills, &playerCount, &m.AvgDuration, &lastPlayed, &m.Popularity); err != nil {
			continue
		}
		m.AvgPlayers = playerCount
		m.LastPlayed = lastPlayed.Format("2006-01-02")
		maps = append(maps, m)
	}

	return maps, nil
}

// =============================================================================
// WEAPON STATISTICS PER SERVER
// =============================================================================

// ServerWeaponStats represents weapon usage on a server


// GetServerWeaponStats returns weapon statistics for a server
func (s *ServerTrackingService) GetServerWeaponStats(ctx context.Context, serverID string) ([]models.ServerWeaponStats, error) {
	query := `
		WITH totals AS (
			SELECT countIf(event_type IN ('player_kill', 'bot_killed')) as total_kills
			FROM raw_events WHERE server_id = ?
		)
		SELECT 
			actor_weapon,
			count() as kills,
			countIf(event_type = 'headshot') as headshots,
			avg(distance) as avg_dist,
			count() * 100.0 / (SELECT total_kills FROM totals) as usage_rate
		FROM raw_events
		WHERE server_id = ? AND event_type IN ('player_kill', 'headshot') AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 20
	`

	rows, err := s.ch.Query(ctx, query, serverID, serverID)
	if err != nil {
		return nil, fmt.Errorf("weapon stats query: %w", err)
	}
	defer rows.Close()

	var weapons []models.ServerWeaponStats
	for rows.Next() {
		var w models.ServerWeaponStats
		if err := rows.Scan(&w.WeaponName, &w.Kills, &w.Headshots, &w.AvgDist, &w.UsageRate); err != nil {
			continue
		}
		if w.Kills > 0 {
			w.HSPercent = float64(w.Headshots) / float64(w.Kills) * 100
		}
		weapons = append(weapons, w)
	}

	return weapons, nil
}

// =============================================================================
// RECENT MATCHES
// =============================================================================

// ServerMatch represents a match played on the server


// GetServerRecentMatches returns recent matches for a server
func (s *ServerTrackingService) GetServerRecentMatches(ctx context.Context, serverID string, limit int) ([]models.ServerMatch, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT 
			match_id,
			any(map_name) as map,
			any(gametype) as gametype,
			uniq(actor_id) as players,
			max(timestamp) - min(timestamp) as duration,
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			min(timestamp) as started,
			max(timestamp) as ended
		FROM raw_events
		WHERE server_id = ? AND match_id != ''
		GROUP BY match_id
		ORDER BY ended DESC
		LIMIT ?
	`

	rows, err := s.ch.Query(ctx, query, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent matches query: %w", err)
	}
	defer rows.Close()

	var matches []models.ServerMatch
	for rows.Next() {
		var m models.ServerMatch
		var duration float64
		if err := rows.Scan(&m.MatchID, &m.MapName, &m.Gametype, &m.PlayerCount,
			&duration, &m.TotalKills, &m.StartedAt, &m.EndedAt); err != nil {
			continue
		}
		m.Duration = int(duration / 60)
		matches = append(matches, m)
	}

	return matches, nil
}

// =============================================================================
// SERVER ACTIVITY TIMELINE
// =============================================================================

// ActivityTimelinePoint represents activity at a point in time


// GetServerActivityTimeline returns hourly activity for the last N days
func (s *ServerTrackingService) GetServerActivityTimeline(ctx context.Context, serverID string, days int) ([]models.ActivityTimelinePoint, error) {
	if days <= 0 {
		days = 7
	}

	// Note: deaths = kills for global timeline stats (each kill = one death)
	query := `
		SELECT 
			toStartOfHour(timestamp) as ts,
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			countIf(event_type IN ('player_kill', 'bot_killed')) as deaths,
			uniq(actor_id) as players,
			countIf(event_type = 'match_start') as match_starts
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL ? DAY
		GROUP BY ts
		ORDER BY ts
	`

	rows, err := s.ch.Query(ctx, query, serverID, days)
	if err != nil {
		return nil, fmt.Errorf("activity timeline query: %w", err)
	}
	defer rows.Close()

	var points []models.ActivityTimelinePoint
	for rows.Next() {
		var p models.ActivityTimelinePoint
		var ts time.Time
		if err := rows.Scan(&ts, &p.Kills, &p.Deaths, &p.Players, &p.MatchStarts); err != nil {
			continue
		}
		p.Timestamp = ts.Format(time.RFC3339)
		points = append(points, p)
	}

	return points, nil
}

// =============================================================================
// LIVE SERVER STATUS
// =============================================================================

// LiveServerStatus represents real-time server status
type LiveServerStatus struct {
	ServerID     string         `json:"server_id"`
	Name         string         `json:"name"`
	IsOnline     bool           `json:"is_online"`
	CurrentMap   string         `json:"current_map"`
	Gametype     string         `json:"gametype"`
	PlayerCount  int            `json:"player_count"`
	MaxPlayers   int            `json:"max_players"`
	Players      []LivePlayer   `json:"players"`
	CurrentMatch *LiveMatchInfo `json:"current_match"`
	TeamScores   *TeamScores    `json:"team_scores"`
}

type LivePlayer struct {
	GUID   string `json:"guid"`
	Name   string `json:"name"`
	Team   string `json:"team"`
	Score  int    `json:"score"`
	Kills  int    `json:"kills"`
	Deaths int    `json:"deaths"`
	Ping   int    `json:"ping"`
}

type LiveMatchInfo struct {
	MatchID  string `json:"match_id"`
	Duration int    `json:"duration_secs"`
	RoundNum int    `json:"round_num"`
}

type TeamScores struct {
	Allies int `json:"allies"`
	Axis   int `json:"axis"`
}

// GetLiveServerStatus returns real-time status for a server
func (s *ServerTrackingService) GetLiveServerStatus(ctx context.Context, serverID string) (*models.ServerLiveStatusResponse, error) {
	status := &models.ServerLiveStatusResponse{}

	// Get server info from Postgres
	var name string
	var maxPlayers int
	s.pg.QueryRow(ctx, `
		SELECT name, max_players FROM servers WHERE id = $1
	`, serverID).Scan(&name, &maxPlayers)
	
	status.MaxPlayers = maxPlayers

	// Get live data from Redis
	matchData, err := s.redis.HGet(ctx, "live_matches", serverID).Result()
	if err != nil || matchData == "" {
		status.IsOnline = false
		return status, nil
	}

	status.IsOnline = true
	// Parse match data (JSON format expected)
	// This would need proper JSON parsing in production
	// For now, assuming matchData contains some info

	// Get current players from Redis
	playerData, _ := s.redis.HGetAll(ctx, "match:"+serverID+":players").Result()
	
	status.CurrentPlayers = len(playerData)
	status.LastUpdate = time.Now().Format(time.RFC3339)

	return status, nil
}

// =============================================================================
// SERVER RANKINGS
// =============================================================================

// ServerRanking represents a server's ranking
type ServerRanking struct {
	ServerID   string  `json:"server_id"`
	Name       string  `json:"name"`
	Rank       int     `json:"rank"`
	Score      float64 `json:"score"`
	Trend      int     `json:"trend"` // +1, 0, -1
	Kills24h   int64   `json:"kills_24h"`
	Players24h int64   `json:"players_24h"`
	Matches24h int64   `json:"matches_24h"`
}

// GetServerRankings returns ranked list of servers
func (s *ServerTrackingService) GetServerRankings(ctx context.Context, limit int) ([]ServerRanking, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT 
			server_id,
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			uniq(actor_id) as players,
			uniq(match_id) as matches
		FROM raw_events
		WHERE timestamp > now() - INTERVAL 24 HOUR AND server_id != ''
		GROUP BY server_id
		ORDER BY kills DESC
		LIMIT ?
	`

	rows, err := s.ch.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("rankings query: %w", err)
	}
	defer rows.Close()

	var rankings []ServerRanking
	var serverIDs []string
	rank := 1
	for rows.Next() {
		var r ServerRanking
		if err := rows.Scan(&r.ServerID, &r.Kills24h, &r.Players24h, &r.Matches24h); err != nil {
			continue
		}
		r.Rank = rank
		r.Score = float64(r.Kills24h) + float64(r.Players24h)*10 + float64(r.Matches24h)*5

		// Set default name fallback
		if len(r.ServerID) >= 8 {
			r.Name = r.ServerID[:8] + "..."
		} else {
			r.Name = r.ServerID
		}

		rankings = append(rankings, r)
		serverIDs = append(serverIDs, r.ServerID)
		rank++
	}

	// Bulk fetch server names from Postgres
	if len(serverIDs) > 0 {
		pgRows, err := s.pg.Query(ctx, "SELECT id, name FROM servers WHERE id = ANY($1)", serverIDs)
		if err == nil {
			defer pgRows.Close()
			serverNames := make(map[string]string)
			for pgRows.Next() {
				var id, name string
				if err := pgRows.Scan(&id, &name); err == nil && name != "" {
					serverNames[id] = name
				}
			}

			// Update names in rankings
			for i := range rankings {
				if name, ok := serverNames[rankings[i].ServerID]; ok {
					rankings[i].Name = name
				}
			}
		}
	}

	return rankings, nil
}

// =============================================================================
// SERVER FAVORITES
// =============================================================================

// ServerFavorite represents a user's favorite server
type ServerFavorite struct {
	UserID   int       `json:"user_id"`
	ServerID string    `json:"server_id"`
	AddedAt  time.Time `json:"added_at"`
	Nickname string    `json:"nickname,omitempty"`
}

// AddServerFavorite adds a server to user's favorites
func (s *ServerTrackingService) AddServerFavorite(ctx context.Context, userID int, serverID string, nickname string) error {
	_, err := s.pg.Exec(ctx, `
		INSERT INTO server_favorites (user_id, server_id, nickname, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, server_id) DO UPDATE SET nickname = $3
	`, userID, serverID, nickname)
	return err
}

// RemoveServerFavorite removes a server from user's favorites
func (s *ServerTrackingService) RemoveServerFavorite(ctx context.Context, userID int, serverID string) error {
	_, err := s.pg.Exec(ctx, `
		DELETE FROM server_favorites WHERE user_id = $1 AND server_id = $2
	`, userID, serverID)
	return err
}

// GetUserFavoriteServers returns user's favorite servers
func (s *ServerTrackingService) GetUserFavoriteServers(ctx context.Context, userID int) ([]models.ServerOverview, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT s.id, s.name, s.address, s.port, s.region, s.max_players,
		       s.total_matches, s.total_players, s.last_seen, s.is_active,
		       f.nickname, f.created_at
		FROM server_favorites f
		JOIN servers s ON f.server_id = s.id
		WHERE f.user_id = $1
		ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.ServerOverview
	for rows.Next() {
		var srv models.ServerOverview
		var nickname string
		var addedAt time.Time
		var maxPlayers int
		var isActive bool
		err := rows.Scan(&srv.ID, &srv.Name, &srv.Address, &srv.Port,
			&srv.Region, &maxPlayers, &srv.TotalMatches, &srv.UniquePlayers,
			&srv.LastSeen, &isActive, &nickname, &addedAt)
		if err != nil {
			continue
		}
		srv.MaxPlayers = maxPlayers
		if nickname != "" {
			srv.DisplayName = nickname
		} else {
			srv.DisplayName = fmt.Sprintf("%s:%d", srv.Name, srv.Port)
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

// IsServerFavorite checks if server is in user's favorites
func (s *ServerTrackingService) IsServerFavorite(ctx context.Context, userID int, serverID string) (bool, error) {
	var count int
	err := s.pg.QueryRow(ctx, `
		SELECT COUNT(*) FROM server_favorites 
		WHERE user_id = $1 AND server_id = $2
	`, userID, serverID).Scan(&count)
	return count > 0, err
}

// =============================================================================
// HISTORICAL PLAYER DATA
// =============================================================================

// ServerPlayerHistory represents a player's history on a server
type ServerPlayerHistory struct {
	GUID            string  `json:"guid"`
	Name            string  `json:"name"`
	Country         string  `json:"country"`
	CountryFlag     string  `json:"country_flag"`
	FirstSeen       string  `json:"first_seen"`
	LastSeen        string  `json:"last_seen"`
	TotalSessions   int64   `json:"total_sessions"`
	TotalTimePlayed float64 `json:"total_time_played_hours"`
	TotalKills      int64   `json:"total_kills"`
	TotalDeaths     int64   `json:"total_deaths"`
	KDRatio         float64 `json:"kd_ratio"`
	TotalHeadshots  int64   `json:"total_headshots"`
	HSPercent       float64 `json:"hs_percent"`
	FavoriteWeapon  string  `json:"favorite_weapon"`
	FavoriteMap     string  `json:"favorite_map"`
	IsOnline        bool    `json:"is_online"`
	// Trend data
	Kills7d  int64 `json:"kills_7d"`
	Kills30d int64 `json:"kills_30d"`
	Trend    int   `json:"trend"` // +1 improving, -1 declining, 0 stable
}

// GetServerHistoricalPlayers returns all players with historical data for a server
func (s *ServerTrackingService) GetServerHistoricalPlayers(ctx context.Context, serverID string, limit int, offset int) ([]ServerPlayerHistory, int64, error) {
	if limit <= 0 {
		limit = 50
	}

	// Get total count
	var totalCount int64
	s.ch.QueryRow(ctx, `
		SELECT uniq(actor_id) FROM raw_events WHERE server_id = ?
	`, serverID).Scan(&totalCount)

	// Deaths are counted from kill events where player is target_id
	query := `
		WITH deaths_cte AS (
			SELECT target_id, count() as death_count
			FROM raw_events
			WHERE server_id = ? AND event_type IN ('player_kill', 'bot_killed') AND target_id != ''
			GROUP BY target_id
		)
		SELECT 
			a.actor_id,
			any(a.actor_name) as name,
			min(a.timestamp) as first_seen,
			max(a.timestamp) as last_seen,
			uniq(a.match_id) as sessions,
			countIf(a.event_type IN ('player_kill', 'bot_killed')) as kills,
			ifNull(max(d.death_count), 0) as deaths,
			countIf(a.event_type = 'headshot') as headshots,
			countIf(a.event_type IN ('player_kill', 'bot_killed') AND a.timestamp > now() - INTERVAL 7 DAY) as kills_7d,
			countIf(a.event_type IN ('player_kill', 'bot_killed') AND a.timestamp > now() - INTERVAL 30 DAY) as kills_30d,
			argMax(a.actor_weapon, countIf(a.event_type IN ('player_kill', 'bot_killed'))) as fav_weapon,
			argMax(a.map_name, count()) as fav_map
		FROM raw_events a
		LEFT JOIN deaths_cte d ON a.actor_id = d.target_id
		WHERE a.server_id = ? AND a.actor_id != ''
		GROUP BY a.actor_id
		ORDER BY kills DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.ch.Query(ctx, query, serverID, serverID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("historical players query: %w", err)
	}
	defer rows.Close()

	var players []ServerPlayerHistory
	for rows.Next() {
		var p ServerPlayerHistory
		var firstSeen, lastSeen time.Time
		if err := rows.Scan(&p.GUID, &p.Name, &firstSeen, &lastSeen,
			&p.TotalSessions, &p.TotalKills, &p.TotalDeaths, &p.TotalHeadshots,
			&p.Kills7d, &p.Kills30d, &p.FavoriteWeapon, &p.FavoriteMap); err != nil {
			continue
		}
		p.FirstSeen = firstSeen.Format("2006-01-02")
		p.LastSeen = lastSeen.Format("2006-01-02 15:04")
		if p.TotalDeaths > 0 {
			p.KDRatio = float64(p.TotalKills) / float64(p.TotalDeaths)
		} else {
			p.KDRatio = float64(p.TotalKills)
		}
		if p.TotalKills > 0 {
			p.HSPercent = float64(p.TotalHeadshots) / float64(p.TotalKills) * 100
		}
		// Calculate trend
		if p.Kills7d*4 > p.Kills30d/3 {
			p.Trend = 1 // Improving
		} else if p.Kills7d*4 < p.Kills30d/5 {
			p.Trend = -1 // Declining
		}

		// Get country from Postgres player table
		s.pg.QueryRow(ctx, `
			SELECT country FROM players WHERE guid = $1
		`, p.GUID).Scan(&p.Country)
		if p.Country != "" {
			p.CountryFlag = getCountryFlag(p.Country)
		}

		players = append(players, p)
	}

	return players, totalCount, nil
}

// =============================================================================
// MAP ROTATION ANALYSIS
// =============================================================================

// MapRotationEntry represents a map in the rotation
type MapRotationEntry struct {
	MapName     string             `json:"map_name"`
	PlayCount   int64              `json:"play_count"`
	AvgDuration float64            `json:"avg_duration_mins"`
	AvgPlayers  float64            `json:"avg_players"`
	TotalKills  int64              `json:"total_kills"`
	KillsPerMin float64            `json:"kills_per_minute"`
	Popularity  float64            `json:"popularity_pct"`
	PeakHour    int                `json:"peak_hour"`
}

// MapRotationAnalysis represents full map rotation data
type MapRotationAnalysis struct {
	Maps                []models.ServerMapRotationResponse `json:"maps"`
	MostPlayed          string                             `json:"most_played"`
	LeastPlayed         string                             `json:"least_played"`
	AvgMapsPerDay       float64                            `json:"avg_maps_per_day"`
	TotalMapsInRotation int                                `json:"total_maps"`
	RotationPattern     []string                           `json:"rotation_pattern"` // Recent map sequence
}

// GetServerMapRotation returns detailed map rotation analysis
func (s *ServerTrackingService) GetServerMapRotation(ctx context.Context, serverID string, days int) ([]models.ServerMapRotationResponse, error) {
	if days <= 0 {
		days = 30
	}

	analysis := []models.ServerMapRotationResponse{}

	// Get map stats
	query := `
		WITH totals AS (
			SELECT uniq(match_id) as total_matches
			FROM raw_events WHERE server_id = ? AND timestamp > now() - INTERVAL ? DAY
		)
		SELECT 
			map_name,
			uniq(match_id) as plays,
			avgIf(duration, event_type = 'match_end') / 60.0 as avg_duration,
			avg(player_count) as avg_players,
			countIf(event_type IN ('player_kill', 'bot_killed')) as kills,
			toHour(argMax(timestamp, count())) as peak_hour,
			uniq(match_id) * 100.0 / (SELECT total_matches FROM totals) as popularity
		FROM (
			SELECT 
				map_name, match_id, event_type, duration, timestamp,
				uniqExact(actor_id) OVER (PARTITION BY match_id) as player_count
			FROM raw_events
			WHERE server_id = ? AND timestamp > now() - INTERVAL ? DAY
		)
		WHERE map_name != ''
		GROUP BY map_name
		ORDER BY plays DESC
	`

	rows, err := s.ch.Query(ctx, query, serverID, days, serverID, days)
	if err != nil {
		return nil, fmt.Errorf("map rotation query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m models.ServerMapRotationResponse
		var avgPlayers, totalKills, mapDuration float64
		var peakHour int
		if err := rows.Scan(&m.MapName, &m.RotationCount, &mapDuration, &avgPlayers,
			&totalKills, &peakHour, &m.Popularity); err != nil {
			continue
		}
		if m.RotationCount > 0 && mapDuration > 0 {
			m.AvgDuration = mapDuration
		}
		analysis = append(analysis, m)
	}
	return analysis, nil
}




// =============================================================================
// COUNTRY/REGION HELPERS
// =============================================================================

// CountryInfo represents country data
type CountryInfo struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Flag      string `json:"flag"`
	Continent string `json:"continent"`
}

// getCountryFlag returns emoji flag for country code
func getCountryFlag(countryCode string) string {
	if len(countryCode) != 2 {
		return "🌐"
	}
	// Convert country code to regional indicator symbols (emoji flags)
	firstLetter := rune(countryCode[0]) - 'A' + 0x1F1E6
	secondLetter := rune(countryCode[1]) - 'A' + 0x1F1E6
	return string([]rune{firstLetter, secondLetter})
}

// GetServerCountryStats returns player distribution by country
func (s *ServerTrackingService) GetServerCountryStats(ctx context.Context, serverID string) ([]models.ServerCountryStatsResponse, error) {
	// Note: This would need to be adapted based on actual schema
	// For now, return from postgres
	var result []models.ServerCountryStatsResponse
	var totalPlayers int64

	// Get total for percentage calculation
	s.pg.QueryRow(ctx, "SELECT COUNT(*) FROM players WHERE country IS NOT NULL AND country != ''").Scan(&totalPlayers)
	if totalPlayers == 0 {
		return result, nil
	}

	rows, err := s.pg.Query(ctx, `
		SELECT country, COUNT(*) FROM players 
		WHERE country IS NOT NULL AND country != ''
		GROUP BY country ORDER BY count DESC LIMIT 20
	`)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var country string
		var count int64
		rows.Scan(&country, &count)
		result = append(result, models.ServerCountryStatsResponse{
			CountryCode: country,
			PlayerCount: count,
			Percentage:  float64(count) / float64(totalPlayers) * 100,
		})
	}
	return result, nil
}

// LookupCountryFromIP performs GeoIP lookup (placeholder - needs maxmind integration)
func LookupCountryFromIP(ip string) string {
	// In production, use maxmind GeoIP2 database
	// For now, return empty - can be populated from player registration
	return ""
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func parseServerLiveData(data string, srv *models.ServerOverview) {
	// Parse format: "players:5,map:mohdm6,gametype:dm"
	if srv == nil {
		return
	}
	parts := strings.Split(data, ",")
	fmt.Printf("[DEBUG] Parsing server data: %v\n", parts)
	for _, part := range parts {
		if strings.HasPrefix(part, "players:") {
			fmt.Sscanf(part, "players:%d", &srv.CurrentPlayers)
		} else if strings.HasPrefix(part, "map:") {
			srv.CurrentMap = strings.TrimPrefix(part, "map:")
		} else if strings.HasPrefix(part, "gametype:") {
			srv.Gametype = strings.TrimPrefix(part, "gametype:")
		}
	}
}
