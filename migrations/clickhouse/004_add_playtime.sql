-- Migration: Add playtime_seconds to player_stats_daily
-- Aggregates playtime from MatchOutcome events (which now include Duration)

-- Step 1: Add the new column to the aggregation table
ALTER TABLE mohaa_stats.player_stats_daily ADD COLUMN IF NOT EXISTS playtime_seconds UInt64 DEFAULT 0;

-- Step 2: Drop and recreate the actor MV to include playtime_seconds
DROP VIEW IF EXISTS mohaa_stats.mv_feed_actor_stats;

CREATE MATERIALIZED VIEW mohaa_stats.mv_feed_actor_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id AS player_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,

    -- Combat (Actor side)
    countIf(event_type = 'player_kill') AS kills,
    0 AS deaths,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,

    -- Bot kills tracked separately
    countIf(event_type = 'bot_killed') AS bot_kills,

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

    -- Playtime (extracted from match_outcome event duration)
    sumIf(toUInt64(JSONExtractFloat(raw_json, 'duration')), event_type = 'match_outcome') AS playtime_seconds,

    max(timestamp) AS last_active
FROM mohaa_stats.raw_events
WHERE actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_id;

-- Force merge to update schema
OPTIMIZE TABLE mohaa_stats.player_stats_daily FINAL;
