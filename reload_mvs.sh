#!/bin/bash
set -e

DB="mohaa_stats"
HOST="opm-stats-clickhouse"

exec_sql() {
    docker exec opm-stats-clickhouse clickhouse-client -q "$1"
}

echo "Dropping Materialized Views..."
exec_sql "DROP VIEW IF EXISTS ${DB}.mv_feed_actor_stats"
exec_sql "DROP VIEW IF EXISTS ${DB}.mv_feed_target_stats"
exec_sql "DROP VIEW IF EXISTS ${DB}.weapon_stats_mv"

echo "Recreating Materialized Views with name preservation..."

# mv_feed_actor_stats
exec_sql "CREATE MATERIALIZED VIEW ${DB}.mv_feed_actor_stats TO ${DB}.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id AS player_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,
    countIf(event_type = 'kill') AS kills,
    0 AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    countIf(event_type IN ('player_bash', 'bash')) AS bash_kills,
    countIf(event_type IN ('grenade_throw', 'explosion', 'grenade_explode', 'kill') AND actor_weapon IN ('grenade', 'm2_grenade', 'steielhandgranate')) AS grenade_kills,
    countIf(event_type IN ('player_roadkill', 'roadkill')) AS roadkills,
    countIf(event_type = 'player_telefragged') AS telefrags,
    countIf(event_type IN ('player_crushed', 'crushed')) AS crushed,
    countIf(event_type IN ('player_teamkill', 'teamkill')) AS teamkills,
    countIf(event_type IN ('player_suicide', 'suicide')) AS suicides,
    countIf(event_type IN ('weapon_reload', 'reload')) AS reloads,
    countIf(event_type IN ('weapon_change', 'weapon_swap')) AS weapon_swaps,
    countIf(event_type = 'weapon_no_ammo') AS no_ammo,
    sum(JSONExtractFloat(raw_json, 'walked')) + sum(JSONExtractFloat(raw_json, 'sprinted')) + sum(JSONExtractFloat(raw_json, 'swam')) + sum(JSONExtractFloat(raw_json, 'driven')) AS distance_units,
    sum(JSONExtractFloat(raw_json, 'sprinted')) AS sprinted,
    sum(JSONExtractFloat(raw_json, 'swam')) AS swam,
    sum(JSONExtractFloat(raw_json, 'driven')) AS driven,
    countIf(event_type = 'jump') AS jumps,
    countIf(event_type = 'crouch') AS crouch_events,
    countIf(event_type = 'prone') AS prone_events,
    countIf(event_type = 'ladder_mount') AS ladders,
    countIf(event_type = 'health_pickup') AS health_picked,
    countIf(event_type = 'ammo_pickup') AS ammo_picked,
    countIf(event_type = 'armor_pickup') AS armor_picked,
    countIf(event_type = 'item_pickup') AS items_picked,
    uniqExactState(match_id) AS matches_played,
    countIf((event_type = 'match_outcome') AND (match_outcome = 1)) AS matches_won,
    countIf((event_type = 'match_outcome')) AS games_finished,
    max(timestamp) AS last_active
FROM ${DB}.raw_events
WHERE actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_id"

# mv_feed_target_stats
exec_sql "CREATE MATERIALIZED VIEW ${DB}.mv_feed_target_stats TO ${DB}.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    target_id AS player_id,
    argMax(target_name, if(target_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,
    0 AS kills,
    count() AS deaths,
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
    uniqExactState(match_id) AS matches_played,
    0 AS matches_won,
    0 AS games_finished,
    max(timestamp) AS last_active
FROM ${DB}.raw_events
WHERE event_type = 'kill' AND target_id != '' AND target_id != 'world'
GROUP BY day, target_id"

# weapon_stats_mv
exec_sql "CREATE MATERIALIZED VIEW ${DB}.weapon_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (actor_weapon, actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_weapon,
    actor_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS actor_name,
    countIf(event_type = 'kill') AS kills,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit
FROM ${DB}.raw_events
WHERE actor_weapon != '' AND actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_weapon, actor_id"

echo "Done."
