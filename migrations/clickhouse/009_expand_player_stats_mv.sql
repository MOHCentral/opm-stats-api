-- Drop and recreate player_stats_daily_mv with all 38 categories
DROP VIEW IF EXISTS mohaa_stats.player_stats_daily_mv;

CREATE MATERIALIZED VIEW mohaa_stats.player_stats_daily_mv
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (actor_id, day)
AS SELECT
    toStartOfDay(timestamp) AS day,
    actor_id,
    argMax(actor_name, timestamp) AS actor_name,
    
    -- Combat
    countIf(event_type = 'kill') AS kills,
    countIf(event_type = 'headshot') AS headshots,
    countIf(event_type = 'weapon_fire') AS shots_fired,
    countIf(event_type = 'weapon_hit') AS shots_hit,
    sumIf(damage, event_type = 'damage') AS total_damage,
    
    -- Special Kills
    countIf(event_type IN ('player_bash', 'bash')) AS bash_kills,
    countIf(event_type IN ('grenade_throw', 'explosion', 'grenade_explode', 'kill') AND actor_weapon IN ('grenade', 'm2_grenade', 'steielhandgranate')) AS grenade_kills, -- Heuristic
    countIf(event_type IN ('player_roadkill', 'roadkill')) AS roadkills,
    countIf(event_type = 'player_telefragged') AS telefrags,
    countIf(event_type IN ('player_crushed', 'crushed')) AS crushed,
    countIf(event_type IN ('player_teamkill', 'teamkill')) AS teamkills,
    countIf(event_type IN ('player_suicide', 'suicide')) AS suicides,
    
    -- Weapons
    countIf(event_type IN ('weapon_reload', 'reload')) AS reloads,
    countIf(event_type IN ('weapon_change', 'weapon_swap')) AS weapon_swaps,
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
    uniqExact(match_id) AS matches_played,
    countIf((event_type = 'match_outcome') AND (damage = 1)) AS matches_won,
    countIf(event_type = 'match_outcome') AS games_finished,
    
    max(timestamp) AS last_active
    
FROM mohaa_stats.raw_events
WHERE (actor_id != '') AND (actor_id != 'world')
GROUP BY
    day,
    actor_id;
