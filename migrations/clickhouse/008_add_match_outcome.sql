-- Add match_outcome column to raw_events and update player_stats_daily_mv

-- 1. Add column to raw_events
ALTER TABLE mohaa_stats.raw_events ADD COLUMN IF NOT EXISTS match_outcome UInt8 DEFAULT 0;

-- 2. Drop old view
DROP TABLE IF EXISTS mohaa_stats.player_stats_daily_mv;

-- 3. Create new view with match_outcome (and preserve distance_km, jumps, playtime_seconds from 007)
CREATE MATERIALIZED VIEW mohaa_stats.player_stats_daily_mv
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (actor_id, day)
SETTINGS index_granularity = 8192
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    
    -- Combat stats
    countIf(event_type = 'kill') AS kills,
    -- Deaths should be queried from player_deaths_daily_mv (aggregated by target_id)
    0 AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    
    -- Weapon stats
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    
    -- Movement stats - convert game units to kilometers
    sum(
        JSONExtractFloat(raw_json, 'walked') + 
        JSONExtractFloat(raw_json, 'sprinted') + 
        JSONExtractFloat(raw_json, 'swam') + 
        JSONExtractFloat(raw_json, 'driven')
    ) / 100000.0 AS distance_km,
    
    countIf(event_type = 'jump') AS jumps,
    
    -- Session stats
    uniqExact(match_id) AS matches_played,
    countIf(event_type = 'match_outcome' AND match_outcome = 1) AS matches_won,
    
    -- Playtime - placeholder for now
    0 AS playtime_seconds,
    
    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;

-- 4. Backfill from raw_events
INSERT INTO mohaa_stats.player_stats_daily_mv
SELECT
    toStartOfDay(timestamp) AS day,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    countIf(event_type = 'kill') AS kills,
    0 AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    sum(
        JSONExtractFloat(raw_json, 'walked') + 
        JSONExtractFloat(raw_json, 'sprinted') + 
        JSONExtractFloat(raw_json, 'swam') + 
        JSONExtractFloat(raw_json, 'driven')
    ) / 100000.0 AS distance_km,
    countIf(event_type = 'jump') AS jumps,
    uniqExact(match_id) AS matches_played,
    countIf(event_type = 'match_outcome' AND match_outcome = 1) AS matches_won,
    0 AS playtime_seconds,
    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;
