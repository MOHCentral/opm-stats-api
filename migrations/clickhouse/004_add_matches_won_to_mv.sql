-- Add matches_won to player_stats_daily_mv
-- Strategy: Create new MV with matches_won, populate from raw_events, swap names

-- 1. Drop old view
DROP TABLE IF EXISTS mohaa_stats.player_stats_daily_mv;

-- 2. Create new view with matches_won
CREATE MATERIALIZED VIEW mohaa_stats.player_stats_daily_mv
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (actor_id, day)
SETTINGS index_granularity = 8192
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
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;

-- 3. Backfill from raw_events (this will populate .inner table)
INSERT INTO mohaa_stats.player_stats_daily_mv
SELECT
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
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;

