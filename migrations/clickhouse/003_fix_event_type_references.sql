-- Migration: Fix event type references in materialized views
-- Aligns ClickHouse queries with actual event types from game scripts
-- 
-- Changes:
-- - 'headshot' -> derived from player_kill with hitloc IN ('head', 'helmet')
-- - 'bash' -> 'player_bash'
-- - 'roadkill' -> 'player_roadkill'
-- - 'crushed' -> 'player_crushed'
-- - 'teamkill' -> 'player_teamkill'
-- - 'suicide' -> 'player_suicide'
-- - Remove non-existent 'vehicle_death' reference
-- - Add 'vehicle_crash' support

-- Step 1: Drop and recreate the actor MV with corrected event types
DROP VIEW IF EXISTS mohaa_stats.mv_feed_actor_stats;

CREATE MATERIALIZED VIEW mohaa_stats.mv_feed_actor_stats TO mohaa_stats.player_stats_daily
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id AS player_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS player_name,
    
    -- Combat (Actor side)
    countIf(event_type = 'player_kill') AS kills,
    0 AS deaths,
    -- Headshots derived from player_kill with head hitloc
    countIf(event_type = 'player_kill' AND hitloc IN ('head', 'helmet')) AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    
    -- Bot kills tracked separately
    countIf(event_type = 'bot_killed') AS bot_kills,
    
    -- Special Kills (using canonical event type names)
    countIf(event_type = 'player_bash') AS bash_kills,
    countIf(
        (event_type = 'grenade_explode') OR 
        (event_type = 'player_kill' AND actor_weapon IN ('grenade', 'm2_grenade', 'stielhandgranate', 'nebelhandgranate'))
    ) AS grenade_kills,
    countIf(event_type = 'player_roadkill') AS roadkills,
    countIf(event_type = 'player_telefragged') AS telefrags,
    countIf(event_type = 'player_crushed') AS crushed,
    countIf(event_type = 'player_teamkill') AS teamkills,
    countIf(event_type = 'player_suicide') AS suicides,
    
    -- Weapons
    countIf(event_type = 'reload') AS reloads,
    countIf(event_type = 'weapon_change') AS weapon_swaps,
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

-- Step 2: Recreate weapon stats MV with corrected headshot detection
DROP VIEW IF EXISTS mohaa_stats.weapon_stats_mv;

CREATE MATERIALIZED VIEW mohaa_stats.weapon_stats_mv
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(day)
ORDER BY (actor_weapon, actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_weapon,
    actor_id,
    argMax(actor_name, if(actor_name != '', toUnixTimestamp64Nano(timestamp), 0)) AS actor_name,
    countIf(event_type = 'player_kill') AS kills,
    countIf(event_type = 'player_kill' AND hitloc IN ('head', 'helmet')) AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit
FROM mohaa_stats.raw_events
WHERE actor_weapon != '' AND actor_id != '' AND actor_id != 'world'
GROUP BY day, actor_weapon, actor_id;

-- Note: The target stats MV is already correct (only tracks deaths from player_kill)
-- No changes needed to mv_feed_target_stats

-- Optimize tables after migration  
OPTIMIZE TABLE mohaa_stats.player_stats_daily FINAL;
