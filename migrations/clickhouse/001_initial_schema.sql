-- ============================================================================
-- ClickHouse Schema for MOHAA Stats
-- High-velocity telemetry data - OLAP workloads
-- ============================================================================

-- Create the database
CREATE DATABASE IF NOT EXISTS mohaa_stats;

-- Main events table with ReplacingMergeTree for deduplication
CREATE TABLE IF NOT EXISTS mohaa_stats.raw_events
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
    actor_stance LowCardinality(String) DEFAULT '',
    actor_smf_id UInt64 DEFAULT 0,
    
    -- Target (for actions involving another player)
    target_id String CODEC(ZSTD(1)),
    target_name String CODEC(ZSTD(1)),
    target_team LowCardinality(String),
    target_pos_x Float32 CODEC(Gorilla, ZSTD(1)),
    target_pos_y Float32 CODEC(Gorilla, ZSTD(1)),
    target_pos_z Float32 CODEC(Gorilla, ZSTD(1)),
    target_stance LowCardinality(String) DEFAULT '',
    target_smf_id UInt64 DEFAULT 0,
    
    -- Combat data
    damage UInt32 CODEC(Delta, ZSTD(1)),
    hitloc LowCardinality(String),
    distance Float32 CODEC(Gorilla, ZSTD(1)),
    match_outcome UInt8 DEFAULT 0,
    round_number UInt16 DEFAULT 0,
    
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
-- Player Identity & Session Tables
-- ============================================================================

-- Player sessions table - tracks every player connection with identity info
CREATE TABLE IF NOT EXISTS mohaa_stats.player_sessions (
    session_id String,
    server_id String,
    match_id String DEFAULT '',
    player_guid String,
    player_name String,
    smf_member_id UInt64 DEFAULT 0,
    auth_token String DEFAULT '',
    authenticated_at DateTime64(3) DEFAULT toDateTime64(0, 3),
    connected_at DateTime,
    disconnected_at DateTime DEFAULT toDateTime(0),
    last_activity DateTime,
    team String DEFAULT '',
    is_active UInt8 DEFAULT 1,
    client_ip String DEFAULT '',
    _partition_date Date DEFAULT toDate(connected_at)
) ENGINE = ReplacingMergeTree(last_activity)
ORDER BY (server_id, player_guid, connected_at)
PARTITION BY toYYYYMM(_partition_date)
TTL _partition_date + INTERVAL 1 YEAR;

-- Player GUID registry
CREATE TABLE IF NOT EXISTS mohaa_stats.player_guid_registry (
    player_guid String,
    smf_member_id UInt64,
    last_known_name String,
    verified_at DateTime64(3),
    last_seen DateTime64(3),
    login_count UInt32 DEFAULT 1
) ENGINE = ReplacingMergeTree(last_seen)
ORDER BY (player_guid);

-- Name history table
CREATE TABLE IF NOT EXISTS mohaa_stats.player_name_history (
    player_guid String,
    player_name String,
    smf_member_id UInt64 DEFAULT 0,
    first_seen DateTime64(3),
    last_seen DateTime64(3),
    use_count UInt32 DEFAULT 1
) ENGINE = ReplacingMergeTree(last_seen)
ORDER BY (player_guid, player_name);

-- ============================================================================
-- Unified Player Stats (The "One Table" Pattern)
-- ============================================================================

-- 1. The Target Table (SummingMergeTree)
-- This table holds the aggregated state. It does NOT do the calculation itself,
-- it just sums up whatever the Materialized Views feed it.
CREATE TABLE IF NOT EXISTS mohaa_stats.player_stats_daily
(
    day DateTime,
    player_id String,
    player_name SimpleAggregateFunction(anyLast, String), -- Keep latest name
    
    -- Metrics
    kills UInt64,
    deaths UInt64, -- Now lives alongside kills!
    headshots UInt64,
    shots_fired UInt64,
    shots_hit UInt64,
    total_damage UInt64,
    
    bash_kills UInt64,
    grenade_kills UInt64,
    roadkills UInt64,
    telefrags UInt64,
    crushed UInt64,
    teamkills UInt64,
    suicides UInt64,
    
    reloads UInt64,
    weapon_swaps UInt64,
    no_ammo UInt64,
    
    distance_units Float64,
    sprinted Float64,
    swam Float64,
    driven Float64,
    jumps UInt64,
    crouch_events UInt64,
    prone_events UInt64,
    ladders UInt64,
    
    health_picked UInt64,
    ammo_picked UInt64,
    armor_picked UInt64,
    items_picked UInt64,
    
    matches_played AggregateFunction(uniqExact, UUID),
    matches_won UInt64,
    games_finished UInt64,
    
    last_active SimpleAggregateFunction(max, DateTime64(3))
)
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (player_id, day);

-- 2. Actor View (Feeds actions: kills, movement, pickups)
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_feed_actor_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id AS player_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,
    
    -- Combat (Actor side)
    countIf(event_type = 'player_kill') AS kills,
    0 AS deaths, -- Actor doesn't die in a kill event (usually)
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    
    -- Special Kills
    countIf(event_type IN ('player_bash', 'bash')) AS bash_kills,
    countIf(event_type IN ('grenade_throw', 'explosion', 'grenade_explode', 'player_kill') AND actor_weapon IN ('grenade', 'm2_grenade', 'steielhandgranate')) AS grenade_kills,
    countIf(event_type IN ('player_roadkill', 'roadkill')) AS roadkills,
    countIf(event_type = 'player_telefragged') AS telefrags,
    countIf(event_type IN ('player_crushed', 'crushed')) AS crushed,
    countIf(event_type IN ('player_teamkill', 'teamkill')) AS teamkills,
    countIf(event_type IN ('player_suicide', 'suicide')) AS suicides,
    
    -- Weapons
    countIf(event_type IN ('reload', 'reload')) AS reloads,
    countIf(event_type IN ('weapon_change', 'weapon_change')) AS weapon_swaps,
    countIf(event_type = 'weapon_no_ammo') AS no_ammo,
    
    -- Movement
    sum(JSONExtractFloat(raw_json, 'walked')) + sum(JSONExtractFloat(raw_json, 'sprinted')) + sum(JSONExtractFloat(raw_json, 'swam')) + sum(JSONExtractFloat(raw_json, 'driven')) AS distance_units,
    sum(JSONExtractFloat(raw_json, 'sprinted')) AS sprinted,
    sum(JSONExtractFloat(raw_json, 'swam')) AS swam,
    sum(JSONExtractFloat(raw_json, 'driven')) AS driven,
    countIf(event_type = 'jump') AS jumps,
    countIf(event_type = 'crouch') AS crouch_events,
    countIf(event_type = 'prone') AS prone_events,
    countIf(event_type = 'ladder_mount') AS ladders,
    
    -- Survival
    countIf(event_type = 'health_pickup') AS health_picked,
    countIf(event_type = 'ammo_pickup') AS ammo_picked,
    countIf(event_type = 'armor_pickup') AS armor_picked,
    countIf(event_type = 'item_pickup') AS items_picked,
    
    -- Results
    uniqExactState(match_id) AS matches_played,
    countIf((event_type = 'match_outcome') AND (match_outcome = 1)) AS matches_won,
    countIf((event_type = 'match_outcome')) AS games_finished,
    
    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_id;

-- 3. Target View (Feeds passive events: DEATHS)
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_feed_target_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    target_id AS player_id,
    argMax(target_name, if(target_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,
    
    0 AS kills,
    count() AS deaths, -- Target of a 'player_kill' event IS the death
    0 AS headshots,
    0 AS shots_fired,
    0 AS shots_hit,
    0 AS total_damage,
    
    0 AS bash_kills,
    0 AS grenade_kills,
    0 AS roadkills,
    0 AS telefrags,
    0 AS crushed,
    0 AS teamkills,
    0 AS suicides,
    
    0 AS reloads,
    0 AS weapon_swaps,
    0 AS no_ammo,
    
    0 AS distance_units,
    0 AS sprinted,
    0 AS swam,
    0 AS driven,
    0 AS jumps,
    0 AS crouch_events,
    0 AS prone_events,
    0 AS ladders,
    
    0 AS health_picked,
    0 AS ammo_picked,
    0 AS armor_picked,
    0 AS items_picked,
    
    uniqExactState(match_id) AS matches_played, -- Being killed counts as playing!
    0 AS matches_won,
    0 AS games_finished,
    
    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE event_type = 'player_kill' AND target_id != '' AND target_id != 'world'
GROUP BY day, target_id;

-- Weapon usage stats
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.weapon_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (actor_weapon, actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_weapon,
    actor_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS actor_name,
    countIf(event_type = 'player_kill') AS kills,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit
FROM mohaa_stats.raw_events
WHERE actor_weapon != '' AND actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_weapon, actor_id;

-- Map popularity and stats
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.map_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (map_name, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    map_name,
    countIf(event_type = 'match_start') AS matches_started,
    countIf(event_type = 'player_kill') AS total_kills,
    uniqExact(actor_id) AS unique_players
FROM mohaa_stats.raw_events
WHERE map_name != ''
GROUP BY day, map_name;

-- Kill/Death heatmap
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.kill_heatmap_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (map_name, bucket_x, bucket_y, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    map_name,
    round(actor_pos_x / 100) * 100 AS bucket_x,
    round(actor_pos_y / 100) * 100 AS bucket_y,
    count() AS kill_count
FROM mohaa_stats.raw_events
WHERE event_type = 'player_kill' AND map_name != '' AND actor_pos_x != 0
GROUP BY day, map_name, bucket_x, bucket_y;

-- Identity Materilized Views
CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_player_auth_registry
TO mohaa_stats.player_guid_registry
AS SELECT
    actor_id AS player_guid,
    actor_smf_id AS smf_member_id,
    actor_name AS last_known_name,
    timestamp AS verified_at,
    timestamp AS last_seen,
    toUInt32(1) AS login_count
FROM mohaa_stats.raw_events
WHERE event_type = 'player_auth' AND actor_smf_id > 0;

CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_player_name_history
TO mohaa_stats.player_name_history
AS SELECT
    actor_id AS player_guid,
    actor_name AS player_name,
    actor_smf_id AS smf_member_id,
    timestamp AS first_seen,
    timestamp AS last_seen,
    toUInt32(1) AS use_count
FROM mohaa_stats.raw_events
WHERE actor_id != '' AND actor_name != '';

-- ============================================================================
-- Indexes for common query patterns
-- ============================================================================
ALTER TABLE mohaa_stats.raw_events ADD INDEX IF NOT EXISTS idx_actor_id actor_id TYPE bloom_filter() GRANULARITY 4;
ALTER TABLE mohaa_stats.raw_events ADD INDEX IF NOT EXISTS idx_target_id target_id TYPE bloom_filter() GRANULARITY 4;
ALTER TABLE mohaa_stats.raw_events ADD INDEX IF NOT EXISTS idx_match_id match_id TYPE bloom_filter() GRANULARITY 4;

-- ============================================================================
-- Leaderboard Tables
-- ============================================================================
CREATE TABLE IF NOT EXISTS mohaa_stats.leaderboard_global (
    player_id String,
    player_name String,
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
) ENGINE = ReplacingMergeTree(updated_at) ORDER BY (rank);
