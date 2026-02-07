-- Migration: Add playtime and objectives to stats
-- Adds duration tracking to raw_events and aggregation to player_stats_daily

-- 1. Add duration to raw_events for time tracking (e.g. from disconnect events)
ALTER TABLE mohaa_stats.raw_events ADD COLUMN IF NOT EXISTS duration Float32 CODEC(Gorilla, ZSTD(1));

-- 2. Add columns to player_stats_daily
ALTER TABLE mohaa_stats.player_stats_daily ADD COLUMN IF NOT EXISTS objectives UInt64 DEFAULT 0;
ALTER TABLE mohaa_stats.player_stats_daily ADD COLUMN IF NOT EXISTS playtime_seconds UInt64 DEFAULT 0;

-- 3. Drop existing MVs to recreate with new logic
DROP VIEW IF EXISTS mohaa_stats.mv_feed_actor_stats;
DROP VIEW IF EXISTS mohaa_stats.mv_feed_target_stats;

-- 4. Recreate mv_feed_actor_stats with objectives and playtime
CREATE MATERIALIZED VIEW mohaa_stats.mv_feed_actor_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id AS player_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,

    countIf(event_type = 'player_kill') AS kills,
    0 AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,

    countIf(event_type = 'bot_killed') AS bot_kills,

    countIf(event_type IN ('player_bash', 'bash')) AS bash_kills,
    countIf(event_type IN ('grenade_throw', 'explosion', 'grenade_explode', 'player_kill') AND actor_weapon IN ('grenade', 'm2_grenade', 'steielhandgranate')) AS grenade_kills,
    countIf(event_type IN ('player_roadkill', 'roadkill')) AS roadkills,
    countIf(event_type = 'player_telefragged') AS telefrags,
    countIf(event_type IN ('player_crushed', 'crushed')) AS crushed,
    countIf(event_type IN ('player_teamkill', 'teamkill')) AS teamkills,
    countIf(event_type IN ('player_suicide', 'suicide')) AS suicides,

    countIf(event_type IN ('reload', 'reload')) AS reloads,
    countIf(event_type IN ('weapon_change', 'weapon_change')) AS weapon_swaps,
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

    -- New Columns
    countIf(event_type IN ('objective_capture', 'objective_complete')) AS objectives,
    sumIf(duration, event_type IN ('player_disconnect', 'disconnect')) AS playtime_seconds,

    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_id;

-- 5. Recreate mv_feed_target_stats with expanded death definition
CREATE MATERIALIZED VIEW mohaa_stats.mv_feed_target_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    target_id AS player_id,
    argMax(target_name, if(target_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,

    0 AS kills,
    count() AS deaths, -- Counts any event where this player is the target of a fatal event
    0 AS headshots,
    0 AS shots_fired,
    0 AS shots_hit,
    0 AS total_damage,

    0 AS bot_kills,

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

    0 AS objectives,
    0 AS playtime_seconds,

    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE event_type IN ('player_kill', 'bot_killed', 'player_suicide', 'suicide', 'player_crushed', 'crushed', 'player_fall', 'fall')
  AND target_id != '' AND target_id != 'world'
GROUP BY day, target_id;
