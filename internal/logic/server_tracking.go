package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ServerTrackingService provides comprehensive server monitoring
type ServerTrackingService struct {
	ch    driver.Conn
	pg    *pgxpool.Pool
	redis *redis.Client
}

func NewServerTrackingService(ch driver.Conn, pg *pgxpool.Pool, redis *redis.Client) *ServerTrackingService {
	return &ServerTrackingService{ch: ch, pg: pg, redis: redis}
}

// =============================================================================
// SERVER LIST & OVERVIEW
// =============================================================================

// ServerOverview represents a server in the list view
type ServerOverview struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Address        string    `json:"address"`
	Port           int       `json:"port"`
	DisplayName    string    `json:"display_name"` // Name:Port format
	IsOnline       bool      `json:"is_online"`
	CurrentPlayers int       `json:"current_players"`
	MaxPlayers     int       `json:"max_players"`
	CurrentMap     string    `json:"current_map"`
	Gametype       string    `json:"gametype"`
	Rank           int       `json:"rank"` // Server ranking
	TotalKills     int64     `json:"total_kills"`
	TotalMatches   int64     `json:"total_matches"`
	UniquePlayers  int64     `json:"unique_players"`
	AvgPlayers24h  float64   `json:"avg_players_24h"`
	PeakPlayers24h int       `json:"peak_players_24h"`
	UptimePercent  float64   `json:"uptime_percent"`
	LastSeen       time.Time `json:"last_seen"`
	Country        string    `json:"country"`
	Region         string    `json:"region"`
}

// ServerGlobalStats represents aggregate stats across all servers
type ServerGlobalStats struct {
	TotalServers      int     `json:"total_servers"`
	OnlineServers     int     `json:"online_servers"`
	TotalPlayersNow   int     `json:"total_players_now"`
	TotalKillsToday   int64   `json:"total_kills_today"`
	TotalMatchesToday int64   `json:"total_matches_today"`
	PeakPlayersToday  int     `json:"peak_players_today"`
	AvgPlayersNow     float64 `json:"avg_players_now"`
	TotalKillsAllTime int64   `json:"total_kills_all_time"`
}

// GetServerList returns all servers with live status
func (s *ServerTrackingService) GetServerList(ctx context.Context) ([]ServerOverview, error) {
	// Get registered servers from PostgreSQL
	rows, err := s.pg.Query(ctx, `
		SELECT id, name, address, port, region, max_players, 
		       total_matches, total_players, last_seen, is_active
		FROM servers 
		ORDER BY total_players DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}
	defer rows.Close()

	var servers []ServerOverview
	rank := 1
	for rows.Next() {
		var srv ServerOverview
		var maxPlayers int
		var isActive bool
		err := rows.Scan(&srv.ID, &srv.Name, &srv.Address, &srv.Port,
			&srv.Region, &maxPlayers, &srv.TotalMatches, &srv.UniquePlayers,
			&srv.LastSeen, &isActive)
		if err != nil {
			continue
		}

		srv.MaxPlayers = maxPlayers
		srv.DisplayName = fmt.Sprintf("%s:%d", srv.Name, srv.Port)
		srv.Rank = rank
		rank++

		// Check if server is live (has heartbeat in last 2 minutes)
		liveData, err := s.redis.HGet(ctx, "live_servers", srv.ID).Result()
		if err == nil && liveData != "" {
			srv.IsOnline = true
			// Parse live data for current players/map
			// Format: "players:5,map:mohdm6,gametype:dm"
			parseServerLiveData(liveData, &srv)
		} else {
			srv.IsOnline = isActive && time.Since(srv.LastSeen) < 5*time.Minute
		}

		// Get stats from ClickHouse
		s.ch.QueryRow(ctx, `
			SELECT 
				countIf(event_type = 'kill') as kills,
				avg(player_count) as avg_players,
				max(player_count) as peak
			FROM (
				SELECT 
					countIf(event_type = 'kill') as kills,
					uniqExact(actor_id) as player_count
				FROM raw_events
				WHERE server_id = ? AND timestamp > now() - INTERVAL 24 HOUR
				GROUP BY toStartOfHour(timestamp)
			)
		`, srv.ID).Scan(&srv.TotalKills, &srv.AvgPlayers24h, &srv.PeakPlayers24h)

		servers = append(servers, srv)
	}

	return servers, nil
}

// GetServerGlobalStats returns aggregate stats across all servers
func (s *ServerTrackingService) GetServerGlobalStats(ctx context.Context) (*ServerGlobalStats, error) {
	stats := &ServerGlobalStats{}

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
			countIf(event_type = 'kill') as kills_today,
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
type ServerDetail struct {
	// Basic Info
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	IsOnline    bool   `json:"is_online"`
	IsOfficial  bool   `json:"is_official"`

	// Current Status
	CurrentPlayers int      `json:"current_players"`
	MaxPlayers     int      `json:"max_players"`
	CurrentMap     string   `json:"current_map"`
	Gametype       string   `json:"gametype"`
	PlayerList     []string `json:"player_list"`

	// Rankings
	Rank       int `json:"rank"`
	WorldRank  int `json:"world_rank"`
	RegionRank int `json:"region_rank"`

	// Lifetime Stats
	Stats ServerLifetimeStats `json:"stats"`

	// Time-based Stats
	Stats24h ServerTimeStats `json:"stats_24h"`
	Stats7d  ServerTimeStats `json:"stats_7d"`
	Stats30d ServerTimeStats `json:"stats_30d"`

	// Uptime
	Uptime ServerUptime `json:"uptime"`
}

// ServerLifetimeStats represents all-time server statistics
type ServerLifetimeStats struct {
	TotalKills       int64   `json:"total_kills"`
	TotalDeaths      int64   `json:"total_deaths"`
	TotalHeadshots   int64   `json:"total_headshots"`
	TotalMatches     int64   `json:"total_matches"`
	UniquePlayers    int64   `json:"unique_players"`
	TotalPlaytime    float64 `json:"total_playtime_hours"`
	AvgMatchDuration float64 `json:"avg_match_duration_mins"`
	FirstSeen        string  `json:"first_seen"`
	TotalDays        int     `json:"total_days"`
}

// ServerTimeStats represents time-windowed stats
type ServerTimeStats struct {
	Kills         int64   `json:"kills"`
	Matches       int64   `json:"matches"`
	UniquePlayers int64   `json:"unique_players"`
	AvgPlayers    float64 `json:"avg_players"`
	PeakPlayers   int     `json:"peak_players"`
	PeakTime      string  `json:"peak_time"`
	Playtime      float64 `json:"playtime_hours"`
}

// ServerUptime represents uptime tracking
type ServerUptime struct {
	Uptime24h  float64 `json:"uptime_24h"`
	Uptime7d   float64 `json:"uptime_7d"`
	Uptime30d  float64 `json:"uptime_30d"`
	LastOnline string  `json:"last_online"`
	LastDown   string  `json:"last_down"`
}

// GetServerDetail returns comprehensive server information
func (s *ServerTrackingService) GetServerDetail(ctx context.Context, serverID string) (*ServerDetail, error) {
	detail := &ServerDetail{ID: serverID}

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
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'death') as deaths,
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
			countIf(event_type = 'kill') as kills,
			uniq(match_id) as matches,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL 24 HOUR
	`, serverID).Scan(&detail.Stats24h.Kills, &detail.Stats24h.Matches, &detail.Stats24h.UniquePlayers)

	// 7d stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as kills,
			uniq(match_id) as matches,
			uniq(actor_id) as players
		FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL 7 DAY
	`, serverID).Scan(&detail.Stats7d.Kills, &detail.Stats7d.Matches, &detail.Stats7d.UniquePlayers)

	// 30d stats
	s.ch.QueryRow(ctx, `
		SELECT 
			countIf(event_type = 'kill') as kills,
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
type PlayerHistoryPoint struct {
	Timestamp string  `json:"timestamp"`
	Hour      int     `json:"hour"`
	Players   int     `json:"players"`
	Peak      int     `json:"peak"`
	Avg       float64 `json:"avg"`
}

// GetServerPlayerHistory returns player count over time
func (s *ServerTrackingService) GetServerPlayerHistory(ctx context.Context, serverID string, hours int) ([]PlayerHistoryPoint, error) {
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

	var points []PlayerHistoryPoint
	for rows.Next() {
		var p PlayerHistoryPoint
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
type PeakHoursHeatmap struct {
	Data  [][]int  `json:"data"`  // [day][hour] = player count
	Hours []string `json:"hours"` // 0-23
	Days  []string `json:"days"`  // Mon-Sun
	Peak  PeakInfo `json:"peak"`
}

type PeakInfo struct {
	Day     string `json:"day"`
	Hour    int    `json:"hour"`
	Players int    `json:"players"`
}

// GetServerPeakHours returns a heatmap of peak activity times
func (s *ServerTrackingService) GetServerPeakHours(ctx context.Context, serverID string, days int) (*PeakHoursHeatmap, error) {
	if days <= 0 {
		days = 30
	}

	heatmap := &PeakHoursHeatmap{
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
				heatmap.Peak = PeakInfo{
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
type ServerTopPlayer struct {
	Rank       int     `json:"rank"`
	GUID       string  `json:"guid"`
	Name       string  `json:"name"`
	Kills      int64   `json:"kills"`
	Deaths     int64   `json:"deaths"`
	KDRatio    float64 `json:"kd_ratio"`
	Headshots  int64   `json:"headshots"`
	HSPercent  float64 `json:"hs_percent"`
	TimePlayed float64 `json:"time_played_hours"`
	LastSeen   string  `json:"last_seen"`
	Sessions   int64   `json:"sessions"`
}

// GetServerTopPlayers returns top players for a specific server
func (s *ServerTrackingService) GetServerTopPlayers(ctx context.Context, serverID string, limit int) ([]ServerTopPlayer, error) {
	if limit <= 0 {
		limit = 25
	}

	query := `
		SELECT 
			actor_id,
			any(actor_name) as name,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'death') as deaths,
			countIf(event_type = 'headshot') as headshots,
			uniq(match_id) as sessions,
			max(timestamp) as last_seen
		FROM raw_events
		WHERE server_id = ? AND actor_id != ''
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT ?
	`

	rows, err := s.ch.Query(ctx, query, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("top players query: %w", err)
	}
	defer rows.Close()

	var players []ServerTopPlayer
	rank := 1
	for rows.Next() {
		var p ServerTopPlayer
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
type ServerMapStats struct {
	MapName     string  `json:"map_name"`
	Matches     int64   `json:"matches"`
	Kills       int64   `json:"kills"`
	AvgPlayers  float64 `json:"avg_players"`
	AvgDuration float64 `json:"avg_duration_mins"`
	Popularity  float64 `json:"popularity_pct"`
	LastPlayed  string  `json:"last_played"`
}

// GetServerMapStats returns map statistics for a server
func (s *ServerTrackingService) GetServerMapStats(ctx context.Context, serverID string) ([]ServerMapStats, error) {
	query := `
		WITH totals AS (
			SELECT uniq(match_id) as total_matches
			FROM raw_events WHERE server_id = ?
		)
		SELECT 
			map_name,
			uniq(match_id) as matches,
			countIf(event_type = 'kill') as kills,
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

	var maps []ServerMapStats
	for rows.Next() {
		var m ServerMapStats
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
type ServerWeaponStats struct {
	WeaponName string  `json:"weapon_name"`
	Kills      int64   `json:"kills"`
	Headshots  int64   `json:"headshots"`
	HSPercent  float64 `json:"hs_percent"`
	AvgDist    float64 `json:"avg_distance"`
	UsageRate  float64 `json:"usage_rate_pct"`
}

// GetServerWeaponStats returns weapon statistics for a server
func (s *ServerTrackingService) GetServerWeaponStats(ctx context.Context, serverID string) ([]ServerWeaponStats, error) {
	query := `
		WITH totals AS (
			SELECT countIf(event_type = 'kill') as total_kills
			FROM raw_events WHERE server_id = ?
		)
		SELECT 
			actor_weapon,
			count() as kills,
			countIf(event_type = 'headshot') as headshots,
			avg(distance) as avg_dist,
			count() * 100.0 / (SELECT total_kills FROM totals) as usage_rate
		FROM raw_events
		WHERE server_id = ? AND event_type IN ('kill', 'headshot') AND actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 20
	`

	rows, err := s.ch.Query(ctx, query, serverID, serverID)
	if err != nil {
		return nil, fmt.Errorf("weapon stats query: %w", err)
	}
	defer rows.Close()

	var weapons []ServerWeaponStats
	for rows.Next() {
		var w ServerWeaponStats
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
type ServerMatch struct {
	MatchID     string    `json:"match_id"`
	MapName     string    `json:"map_name"`
	Gametype    string    `json:"gametype"`
	PlayerCount int       `json:"player_count"`
	Duration    int       `json:"duration_mins"`
	TotalKills  int64     `json:"total_kills"`
	Winner      string    `json:"winner"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
}

// GetServerRecentMatches returns recent matches for a server
func (s *ServerTrackingService) GetServerRecentMatches(ctx context.Context, serverID string, limit int) ([]ServerMatch, error) {
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
			countIf(event_type = 'kill') as kills,
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

	var matches []ServerMatch
	for rows.Next() {
		var m ServerMatch
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
type ActivityTimelinePoint struct {
	Timestamp   string `json:"timestamp"`
	Kills       int64  `json:"kills"`
	Deaths      int64  `json:"deaths"`
	Players     int    `json:"players"`
	MatchStarts int64  `json:"match_starts"`
}

// GetServerActivityTimeline returns hourly activity for the last N days
func (s *ServerTrackingService) GetServerActivityTimeline(ctx context.Context, serverID string, days int) ([]ActivityTimelinePoint, error) {
	if days <= 0 {
		days = 7
	}

	query := `
		SELECT 
			toStartOfHour(timestamp) as ts,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'death') as deaths,
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

	var points []ActivityTimelinePoint
	for rows.Next() {
		var p ActivityTimelinePoint
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
func (s *ServerTrackingService) GetLiveServerStatus(ctx context.Context, serverID string) (*LiveServerStatus, error) {
	status := &LiveServerStatus{ServerID: serverID}

	// Get server info from Postgres
	s.pg.QueryRow(ctx, `
		SELECT name, max_players FROM servers WHERE id = $1
	`, serverID).Scan(&status.Name, &status.MaxPlayers)

	// Get live data from Redis
	matchData, err := s.redis.HGet(ctx, "live_matches", serverID).Result()
	if err != nil || matchData == "" {
		status.IsOnline = false
		return status, nil
	}

	status.IsOnline = true
	// Parse match data (JSON format expected)
	// This would need proper JSON parsing in production

	// Get current players from Redis
	playerData, _ := s.redis.HGetAll(ctx, "match:"+serverID+":players").Result()
	for guid, data := range playerData {
		var p LivePlayer
		p.GUID = guid
		fmt.Sscanf(data, "%s %d %d %d", &p.Name, &p.Kills, &p.Deaths, &p.Score)
		status.Players = append(status.Players, p)
	}
	status.PlayerCount = len(status.Players)

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
			countIf(event_type = 'kill') as kills,
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
	rank := 1
	for rows.Next() {
		var r ServerRanking
		if err := rows.Scan(&r.ServerID, &r.Kills24h, &r.Players24h, &r.Matches24h); err != nil {
			continue
		}
		r.Rank = rank
		r.Score = float64(r.Kills24h) + float64(r.Players24h)*10 + float64(r.Matches24h)*5

		// Get server name from Postgres
		s.pg.QueryRow(ctx, "SELECT name FROM servers WHERE id = $1", r.ServerID).Scan(&r.Name)
		if r.Name == "" {
			r.Name = r.ServerID[:8] + "..."
		}

		rankings = append(rankings, r)
		rank++
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
func (s *ServerTrackingService) GetUserFavoriteServers(ctx context.Context, userID int) ([]ServerOverview, error) {
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

	var servers []ServerOverview
	for rows.Next() {
		var srv ServerOverview
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

	query := `
		SELECT 
			actor_id,
			any(actor_name) as name,
			min(timestamp) as first_seen,
			max(timestamp) as last_seen,
			uniq(match_id) as sessions,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'death') as deaths,
			countIf(event_type = 'headshot') as headshots,
			countIf(event_type = 'kill' AND timestamp > now() - INTERVAL 7 DAY) as kills_7d,
			countIf(event_type = 'kill' AND timestamp > now() - INTERVAL 30 DAY) as kills_30d,
			argMax(actor_weapon, countIf(event_type = 'kill')) as fav_weapon,
			argMax(map_name, count()) as fav_map
		FROM raw_events
		WHERE server_id = ? AND actor_id != ''
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.ch.Query(ctx, query, serverID, limit, offset)
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
	NextMapProb map[string]float64 `json:"next_map_probability"`
}

// MapRotationAnalysis represents full map rotation data
type MapRotationAnalysis struct {
	Maps                []MapRotationEntry `json:"maps"`
	MostPlayed          string             `json:"most_played"`
	LeastPlayed         string             `json:"least_played"`
	AvgMapsPerDay       float64            `json:"avg_maps_per_day"`
	TotalMapsInRotation int                `json:"total_maps"`
	RotationPattern     []string           `json:"rotation_pattern"` // Recent map sequence
}

// GetServerMapRotation returns detailed map rotation analysis
func (s *ServerTrackingService) GetServerMapRotation(ctx context.Context, serverID string, days int) (*MapRotationAnalysis, error) {
	if days <= 0 {
		days = 30
	}

	analysis := &MapRotationAnalysis{}

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
			countIf(event_type = 'kill') as kills,
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
		var m MapRotationEntry
		if err := rows.Scan(&m.MapName, &m.PlayCount, &m.AvgDuration, &m.AvgPlayers,
			&m.TotalKills, &m.PeakHour, &m.Popularity); err != nil {
			continue
		}
		if m.AvgDuration > 0 {
			m.KillsPerMin = float64(m.TotalKills) / (m.AvgDuration * float64(m.PlayCount))
		}
		analysis.Maps = append(analysis.Maps, m)
	}

	if len(analysis.Maps) > 0 {
		analysis.MostPlayed = analysis.Maps[0].MapName
		analysis.LeastPlayed = analysis.Maps[len(analysis.Maps)-1].MapName
		analysis.TotalMapsInRotation = len(analysis.Maps)
	}

	// Get avg maps per day
	s.ch.QueryRow(ctx, `
		SELECT uniq(match_id) / ? FROM raw_events
		WHERE server_id = ? AND timestamp > now() - INTERVAL ? DAY
	`, days, serverID, days).Scan(&analysis.AvgMapsPerDay)

	// Get recent rotation pattern (last 10 maps played)
	patternRows, err := s.ch.Query(ctx, `
		SELECT DISTINCT map_name
		FROM raw_events
		WHERE server_id = ? AND event_type = 'match_start' AND map_name != ''
		ORDER BY timestamp DESC
		LIMIT 10
	`, serverID)
	if err == nil {
		defer patternRows.Close()
		for patternRows.Next() {
			var mapName string
			patternRows.Scan(&mapName)
			analysis.RotationPattern = append(analysis.RotationPattern, mapName)
		}
	}

	// Calculate next map probabilities for each map
	for i := range analysis.Maps {
		analysis.Maps[i].NextMapProb = make(map[string]float64)
		// Get transition probabilities
		nextRows, err := s.ch.Query(ctx, `
			WITH transitions AS (
				SELECT 
					map_name,
					leadInFrame(map_name) OVER (ORDER BY timestamp) as next_map
				FROM raw_events
				WHERE server_id = ? AND event_type = 'match_start'
			)
			SELECT next_map, count() * 100.0 / sum(count()) OVER () as prob
			FROM transitions
			WHERE map_name = ? AND next_map != ''
			GROUP BY next_map
			ORDER BY prob DESC
			LIMIT 5
		`, serverID, analysis.Maps[i].MapName)
		if err == nil {
			for nextRows.Next() {
				var nextMap string
				var prob float64
				nextRows.Scan(&nextMap, &prob)
				analysis.Maps[i].NextMapProb[nextMap] = prob
			}
			nextRows.Close()
		}
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
func (s *ServerTrackingService) GetServerCountryStats(ctx context.Context, serverID string) (map[string]int, error) {
	// Note: This would need to be adapted based on actual schema
	// For now, return from postgres
	result := make(map[string]int)

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
		var count int
		rows.Scan(&country, &count)
		result[country] = count
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

func parseServerLiveData(data string, srv *ServerOverview) {
	// Parse format: "players:5,map:mohdm6,gametype:dm"
	if srv == nil {
		return
	}
	fmt.Sscanf(data, "players:%d,map:%s,gametype:%s",
		&srv.CurrentPlayers, &srv.CurrentMap, &srv.Gametype)
}
