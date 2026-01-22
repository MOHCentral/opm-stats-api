-- Drop and recreate player_stats_daily_mv with distance, jumps, playtime

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
    countIf(event_type = 'kill') AS kills,
    countIf(event_type = 'death') AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    
    -- Weapon stats
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    
    -- Movement stats (NEW) - convert game units to kilometers
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
    
    -- Playtime (NEW) - placeholder for now
    0 AS playtime_seconds,
    
    max(timestamp) AS last_active
    
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;
