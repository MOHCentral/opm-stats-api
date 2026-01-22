-- ============================================================================
-- ClickHouse Schema for MOHAA Stats
-- High-velocity telemetry data - OLAP workloads
-- ============================================================================

-- Main events table with ReplacingMergeTree for deduplication
CREATE TABLE IF NOT EXISTS raw_events
(
    -- Temporal fields
    timestamp DateTime64(3) CODEC(DoubleDelta, ZSTD(1)),
    
    -- Match context
    match_id UUID,
    server_id String CODEC(ZSTD(1)),
    map_name LowCardinality(String),
    
    -- Event classification
    event_type LowCardinality(String),
    
    -- Actor (the player who performed the action)
    actor_id String CODEC(ZSTD(1)),
    actor_name String CODEC(ZSTD(1)),
    actor_team LowCardinality(String),
    actor_weapon LowCardinality(String),
    actor_pos_x Float32 CODEC(Gorilla, ZSTD(1)),
    actor_pos_y Float32 CODEC(Gorilla, ZSTD(1)),
    actor_pos_z Float32 CODEC(Gorilla, ZSTD(1)),
    actor_pitch Float32 CODEC(Gorilla, ZSTD(1)),
    actor_yaw Float32 CODEC(Gorilla, ZSTD(1)),
    
    -- Target (for actions involving another player)
    target_id String CODEC(ZSTD(1)),
    target_name String CODEC(ZSTD(1)),
    target_team LowCardinality(String),
    target_pos_x Float32 CODEC(Gorilla, ZSTD(1)),
    target_pos_y Float32 CODEC(Gorilla, ZSTD(1)),
    target_pos_z Float32 CODEC(Gorilla, ZSTD(1)),
    
    -- Combat data
    damage UInt32 CODEC(Delta, ZSTD(1)),
    hitloc LowCardinality(String),
    distance Float32 CODEC(Gorilla, ZSTD(1)),
    
    -- Raw JSON for debugging/replay
    raw_json String CODEC(ZSTD(3)),
    
    -- Partition key
    _partition_date Date DEFAULT toDate(timestamp)
)
ENGINE = ReplacingMergeTree(_partition_date)
PARTITION BY toYYYYMM(_partition_date)
ORDER BY (event_type, actor_id, match_id, timestamp)
TTL _partition_date + INTERVAL 2 YEAR
SETTINGS index_granularity = 8192;

-- ============================================================================
-- Materialized Views for Pre-Aggregation
-- ============================================================================

-- Hourly player kill aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS player_kills_hourly_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (actor_id, actor_name, map_name, hour)
AS SELECT
    toStartOfHour(timestamp) AS hour,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    map_name,
    count() AS kills,
    countIf(hitloc = 'head') AS headshots
FROM raw_events
WHERE event_type = 'kill' AND actor_id != '' AND actor_id != 'world'
GROUP BY hour, actor_id, map_name;

-- Daily player stats (comprehensive)
CREATE MATERIALIZED VIEW IF NOT EXISTS player_stats_daily_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    countIf(event_type = 'kill') AS kills,
    countIf(event_type = 'death') AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    uniqExact(match_id) AS matches_played,
    countIf(event_type = 'match_outcome' AND damage = 1) AS matches_won,
    max(timestamp) AS last_active
FROM raw_events
WHERE actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_id;

-- Weapon usage stats
CREATE MATERIALIZED VIEW IF NOT EXISTS weapon_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (actor_weapon, actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_weapon,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    countIf(event_type = 'kill') AS kills,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit
FROM raw_events
WHERE actor_weapon != '' AND actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_weapon, actor_id;

-- Map popularity and stats
CREATE MATERIALIZED VIEW IF NOT EXISTS map_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (map_name, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    map_name,
    countIf(event_type = 'match_start') AS matches_started,
    countIf(event_type = 'kill') AS total_kills,
    uniqExact(actor_id) AS unique_players
FROM raw_events
WHERE map_name != ''
GROUP BY day, map_name;

-- Kill position heatmap data (bucketed)
CREATE MATERIALIZED VIEW IF NOT EXISTS kill_heatmap_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (map_name, bucket_x, bucket_y, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    map_name,
    round(actor_pos_x / 100) * 100 AS bucket_x,
    round(actor_pos_y / 100) * 100 AS bucket_y,
    count() AS kill_count
FROM raw_events
WHERE event_type = 'kill' AND map_name != '' AND actor_pos_x != 0
GROUP BY day, map_name, bucket_x, bucket_y;

-- Death position heatmap data (bucketed) - where players die
CREATE MATERIALIZED VIEW IF NOT EXISTS death_heatmap_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (map_name, bucket_x, bucket_y, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    map_name,
    round(target_pos_x / 100) * 100 AS bucket_x,
    round(target_pos_y / 100) * 100 AS bucket_y,
    count() AS death_count
FROM raw_events
WHERE event_type = 'kill' AND map_name != '' AND target_pos_x != 0
GROUP BY day, map_name, bucket_x, bucket_y;

-- Match summaries
CREATE MATERIALIZED VIEW IF NOT EXISTS match_summary_mv
ENGINE = ReplacingMergeTree(last_event)
PARTITION BY toYYYYMM(started_at)
ORDER BY (match_id)
AS SELECT
    match_id,
    argMin(server_id, timestamp) AS server_id,
    argMin(map_name, timestamp) AS map_name,
    min(timestamp) AS started_at,
    max(timestamp) AS last_event,
    countIf(event_type = 'kill') AS total_kills,
    uniqExact(actor_id) AS unique_players
FROM raw_events
WHERE match_id != toUUID('00000000-0000-0000-0000-000000000000')
GROUP BY match_id;

-- Server activity
CREATE MATERIALIZED VIEW IF NOT EXISTS server_activity_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    server_id,
    count() AS total_events,
    countIf(event_type = 'kill') AS total_kills,
    uniqExact(match_id) AS matches,
    uniqExact(actor_id) AS unique_players
FROM raw_events
WHERE server_id != ''
GROUP BY day, server_id;

-- ============================================================================
-- Indexes for common query patterns
-- ============================================================================

-- Skip index on player ID for fast lookups
ALTER TABLE raw_events ADD INDEX IF NOT EXISTS idx_actor_id actor_id TYPE bloom_filter() GRANULARITY 4;
ALTER TABLE raw_events ADD INDEX IF NOT EXISTS idx_target_id target_id TYPE bloom_filter() GRANULARITY 4;
ALTER TABLE raw_events ADD INDEX IF NOT EXISTS idx_match_id match_id TYPE bloom_filter() GRANULARITY 4;

-- ============================================================================
-- Aggregate tables for leaderboards (final computed tables)
-- ============================================================================

-- Global all-time leaderboard (updated periodically by scheduled job)
CREATE TABLE IF NOT EXISTS leaderboard_global
(
    player_id String CODEC(ZSTD(1)),
    player_name String CODEC(ZSTD(1)),
    total_kills UInt64,
    total_deaths UInt64,
    total_headshots UInt64,
    total_damage UInt64,
    matches_played UInt64,
    kd_ratio Float64,
    hs_percent Float64,
    last_active DateTime64(3),
    rank UInt32,
    updated_at DateTime64(3) DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (rank)
SETTINGS index_granularity = 8192;

-- Weekly leaderboard
CREATE TABLE IF NOT EXISTS leaderboard_weekly
(
    week_start Date,
    player_id String CODEC(ZSTD(1)),
    player_name String CODEC(ZSTD(1)),
    kills UInt64,
    deaths UInt64,
    headshots UInt64,
    rank UInt32,
    updated_at DateTime64(3) DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY week_start
ORDER BY (week_start, rank)
TTL week_start + INTERVAL 12 WEEK
SETTINGS index_granularity = 8192;

-- Weapon-specific leaderboard
CREATE TABLE IF NOT EXISTS leaderboard_weapon
(
    weapon LowCardinality(String),
    player_id String CODEC(ZSTD(1)),
    player_name String CODEC(ZSTD(1)),
    kills UInt64,
    headshots UInt64,
    rank UInt32,
    updated_at DateTime64(3) DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (weapon, rank)
SETTINGS index_granularity = 8192;

-- Map-specific leaderboard
CREATE TABLE IF NOT EXISTS leaderboard_map
(
    map_name LowCardinality(String),
    player_id String CODEC(ZSTD(1)),
    player_name String CODEC(ZSTD(1)),
    kills UInt64,
    deaths UInt64,
    rank UInt32,
    updated_at DateTime64(3) DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (map_name, rank)
SETTINGS index_granularity = 8192;
