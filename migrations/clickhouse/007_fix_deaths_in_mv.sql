-- Fix deaths calculation in player_stats_daily_mv
-- Deaths should be counted as kills where player is target_id, not actor_id

DROP VIEW IF EXISTS mohaa_stats.player_stats_daily_mv;

CREATE MATERIALIZED VIEW mohaa_stats.player_stats_daily_mv
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (actor_id, day)
SETTINGS index_granularity = 8192
POPULATE
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    
    -- Combat stats
    -- Kills: events where player is actor_id
    countIf(event_type = 'kill') AS kills,
    -- Deaths: NOTE - This MV aggregates by actor_id, so we cannot directly count 
    -- deaths (where player is target_id) in the same row.
    -- For accurate deaths, the leaderboard query should use target_id aggregation.
    -- As a workaround, we set deaths=0 here and query it separately.
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
    countIf((event_type = 'match_outcome') AND (damage = 1)) AS matches_won,
    
    -- Playtime - placeholder for now
    0 AS playtime_seconds,
    
    max(timestamp) AS last_active
    
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;

-- Create a separate MV for deaths (by target_id)
DROP VIEW IF EXISTS mohaa_stats.player_deaths_daily_mv;

CREATE MATERIALIZED VIEW mohaa_stats.player_deaths_daily_mv
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (target_id, day)
SETTINGS index_granularity = 8192
POPULATE
AS SELECT
    toStartOfDay(timestamp) AS day,
    target_id,
    count() AS deaths
FROM mohaa_stats.raw_events
WHERE event_type = 'kill' AND target_id != '' AND target_id != 'world'
GROUP BY day, target_id;
