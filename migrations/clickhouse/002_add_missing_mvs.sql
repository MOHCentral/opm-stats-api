-- ============================================================================
-- Add missing Materialized Views for Server Stats
-- ============================================================================

-- Match Summary Table
CREATE TABLE IF NOT EXISTS mohaa_stats.match_summary (
    day Date,
    match_id UUID,
    map_name SimpleAggregateFunction(any, String),
    server_id SimpleAggregateFunction(any, String),
    start_time SimpleAggregateFunction(min, DateTime),
    end_time SimpleAggregateFunction(max, DateTime),
    duration SimpleAggregateFunction(max, Float64),
    player_count AggregateFunction(uniqExact, String),
    kills SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (match_id, day);

CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_match_summary TO mohaa_stats.match_summary
AS SELECT
    toDate(timestamp) as day,
    match_id,
    any(map_name) as map_name,
    any(server_id) as server_id,
    min(timestamp) as start_time,
    max(timestamp) as end_time,
    dateDiff('second', min(timestamp), max(timestamp)) as duration,
    uniqExactState(actor_id) as player_count,
    countIf(event_type = 'kill') as kills
FROM mohaa_stats.raw_events
WHERE match_id != '00000000-0000-0000-0000-000000000000'
GROUP BY day, match_id;

-- Backfill Match Summary
INSERT INTO mohaa_stats.match_summary
SELECT
    toDate(timestamp) as day,
    match_id,
    any(map_name) as map_name,
    any(server_id) as server_id,
    min(timestamp) as start_time,
    max(timestamp) as end_time,
    dateDiff('second', min(timestamp), max(timestamp)) as duration,
    uniqExactState(actor_id) as player_count,
    countIf(event_type = 'kill') as kills
FROM mohaa_stats.raw_events
WHERE match_id != '00000000-0000-0000-0000-000000000000'
GROUP BY day, match_id;

-- Server Activity Table
CREATE TABLE IF NOT EXISTS mohaa_stats.server_activity (
    day Date,
    server_id String,
    last_seen SimpleAggregateFunction(max, DateTime)
) ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (server_id, day);

CREATE MATERIALIZED VIEW IF NOT EXISTS mohaa_stats.mv_server_activity TO mohaa_stats.server_activity
AS SELECT
    toDate(timestamp) as day,
    server_id,
    max(timestamp) as last_seen
FROM mohaa_stats.raw_events
WHERE server_id != ''
GROUP BY day, server_id;

-- Backfill Server Activity
INSERT INTO mohaa_stats.server_activity
SELECT
    toDate(timestamp) as day,
    server_id,
    max(timestamp) as last_seen
FROM mohaa_stats.raw_events
WHERE server_id != ''
GROUP BY day, server_id;
