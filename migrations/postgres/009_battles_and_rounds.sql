-- Battle and Round Tracking System
-- Handles FFA (1 round), TDM (multiple rounds), OBJ (multiple rounds)
-- Created: 2026-01-21

-- =============================================================================
-- TABLE: battles (The parent container for each map load)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battles (
    battle_id SERIAL PRIMARY KEY,
    battle_guid UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    
    -- Context
    server_id INTEGER NOT NULL,
    map_name VARCHAR(100) NOT NULL,
    game_type VARCHAR(20) NOT NULL, -- 'ffa', 'tdm', 'obj', 'rbm', 'ctf'
    
    -- Timing
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration_seconds INTEGER GENERATED ALWAYS AS (EXTRACT(EPOCH FROM (ended_at - started_at))) STORED,
    
    -- Outcome
    winner_team VARCHAR(20), -- 'allies', 'axis', or player_guid for FFA
    final_score_allies INTEGER DEFAULT 0,
    final_score_axis INTEGER DEFAULT 0,
    
    -- Player counts
    total_players INTEGER DEFAULT 0,
    peak_players INTEGER DEFAULT 0,
    
    -- Quick stats (denormalized for performance)
    total_kills INTEGER DEFAULT 0,
    total_deaths INTEGER DEFAULT 0,
    total_damage BIGINT DEFAULT 0,
    total_headshots INTEGER DEFAULT 0,
    total_objectives INTEGER DEFAULT 0,
    
    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_battles_server ON battles(server_id);
CREATE INDEX idx_battles_map ON battles(map_name);
CREATE INDEX idx_battles_gametype ON battles(game_type);
CREATE INDEX idx_battles_started ON battles(started_at DESC);

-- =============================================================================
-- TABLE: battle_rounds (Child rounds within a battle)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_rounds (
    round_id SERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_number INTEGER NOT NULL, -- 1, 2, 3...
    
    -- Timing
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration_seconds INTEGER,
    
    -- Outcome
    winner_team VARCHAR(20),
    score_allies INTEGER DEFAULT 0,
    score_axis INTEGER DEFAULT 0,
    
    -- Round stats
    total_kills INTEGER DEFAULT 0,
    total_deaths INTEGER DEFAULT 0,
    total_damage BIGINT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE (battle_id, round_number)
);

CREATE INDEX idx_rounds_battle ON battle_rounds(battle_id);

-- =============================================================================
-- TABLE: battle_players (Players who participated in a battle)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_players (
    battle_player_id SERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_id INTEGER REFERENCES battle_rounds(round_id) ON DELETE CASCADE, -- NULL = full battle stats
    
    -- Player identity
    player_guid VARCHAR(64) NOT NULL,
    smf_member_id INTEGER DEFAULT 0,
    player_name VARCHAR(100),
    team VARCHAR(20), -- 'allies', 'axis', 'spectator'
    
    -- Core stats
    kills INTEGER DEFAULT 0,
    deaths INTEGER DEFAULT 0,
    assists INTEGER DEFAULT 0,
    score INTEGER DEFAULT 0,
    
    -- Combat stats
    damage_dealt BIGINT DEFAULT 0,
    damage_taken BIGINT DEFAULT 0,
    headshots INTEGER DEFAULT 0,
    melee_kills INTEGER DEFAULT 0,
    grenade_kills INTEGER DEFAULT 0,
    
    -- Movement stats
    distance_walked FLOAT DEFAULT 0,
    distance_sprinted FLOAT DEFAULT 0,
    distance_swam FLOAT DEFAULT 0,
    jump_count INTEGER DEFAULT 0,
    
    -- Stance stats
    time_standing_seconds INTEGER DEFAULT 0,
    time_crouching_seconds INTEGER DEFAULT 0,
    time_prone_seconds INTEGER DEFAULT 0,
    
    -- Survival stats
    health_pickups INTEGER DEFAULT 0,
    ammo_pickups INTEGER DEFAULT 0,
    weapon_pickups INTEGER DEFAULT 0,
    
    -- Objectives
    objectives_completed INTEGER DEFAULT 0,
    objectives_defended INTEGER DEFAULT 0,
    
    -- Vehicle stats
    vehicle_kills INTEGER DEFAULT 0,
    roadkills INTEGER DEFAULT 0,
    turret_kills INTEGER DEFAULT 0,
    
    -- Accuracy
    shots_fired INTEGER DEFAULT 0,
    shots_hit INTEGER DEFAULT 0,
    accuracy_percent DECIMAL(5,2) GENERATED ALWAYS AS (
        CASE WHEN shots_fired > 0 THEN (shots_hit::DECIMAL / shots_fired * 100) ELSE 0 END
    ) STORED,
    
    -- Calculated metrics
    kd_ratio DECIMAL(10,2) GENERATED ALWAYS AS (
        CASE WHEN deaths > 0 THEN (kills::DECIMAL / deaths) ELSE kills::DECIMAL END
    ) STORED,
    
    damage_per_minute DECIMAL(10,2), -- Calculated in application
    kill_streak_best INTEGER DEFAULT 0,
    
    -- Playstyle classification
    playstyle_rusher_score INTEGER DEFAULT 0, -- High movement + kills
    playstyle_camper_score INTEGER DEFAULT 0, -- High prone + low movement
    playstyle_support_score INTEGER DEFAULT 0, -- High assists + objectives
    
    -- Timestamps
    joined_at TIMESTAMP,
    left_at TIMESTAMP,
    time_in_battle_seconds INTEGER,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_battle_players_battle ON battle_players(battle_id);
CREATE INDEX idx_battle_players_round ON battle_players(round_id);
CREATE INDEX idx_battle_players_guid ON battle_players(player_guid);
CREATE INDEX idx_battle_players_smf ON battle_players(smf_member_id);
CREATE INDEX idx_battle_players_kd ON battle_players(kd_ratio DESC);

-- =============================================================================
-- TABLE: battle_weapons (Weapon usage per battle/round)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_weapons (
    battle_weapon_id SERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_id INTEGER REFERENCES battle_rounds(round_id) ON DELETE CASCADE,
    player_guid VARCHAR(64) NOT NULL,
    
    weapon_name VARCHAR(100) NOT NULL,
    
    kills INTEGER DEFAULT 0,
    shots_fired INTEGER DEFAULT 0,
    shots_hit INTEGER DEFAULT 0,
    headshots INTEGER DEFAULT 0,
    damage_dealt BIGINT DEFAULT 0,
    
    accuracy DECIMAL(5,2) GENERATED ALWAYS AS (
        CASE WHEN shots_fired > 0 THEN (shots_hit::DECIMAL / shots_fired * 100) ELSE 0 END
    ) STORED
);

CREATE INDEX idx_battle_weapons_battle ON battle_weapons(battle_id);
CREATE INDEX idx_battle_weapons_player ON battle_weapons(player_guid);
CREATE INDEX idx_battle_weapons_weapon ON battle_weapons(weapon_name);

-- =============================================================================
-- TABLE: battle_timeline (Event timeline for match replay)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_timeline (
    timeline_id BIGSERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_id INTEGER REFERENCES battle_rounds(round_id),
    
    timestamp TIMESTAMP NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    
    -- Event participants
    actor_guid VARCHAR(64),
    actor_name VARCHAR(100),
    target_guid VARCHAR(64),
    target_name VARCHAR(100),
    
    -- Event details
    description TEXT,
    metadata JSONB, -- Flexible storage for event-specific data
    
    -- Impact score (for highlighting key moments)
    impact_score INTEGER DEFAULT 0 -- 1-100, calculated based on event significance
);

CREATE INDEX idx_timeline_battle ON battle_timeline(battle_id);
CREATE INDEX idx_timeline_timestamp ON battle_timeline(battle_id, timestamp);
CREATE INDEX idx_timeline_impact ON battle_timeline(battle_id, impact_score DESC);

-- =============================================================================
-- TABLE: battle_momentum (Tug-of-war tracking, sampled every 10 seconds)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_momentum (
    momentum_id BIGSERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_id INTEGER REFERENCES battle_rounds(round_id),
    
    timestamp TIMESTAMP NOT NULL,
    seconds_elapsed INTEGER NOT NULL,
    
    -- Score differential (positive = allies winning, negative = axis winning)
    score_allies INTEGER DEFAULT 0,
    score_axis INTEGER DEFAULT 0,
    score_differential INTEGER GENERATED ALWAYS AS (score_allies - score_axis) STORED,
    
    -- Active players
    players_allies INTEGER DEFAULT 0,
    players_axis INTEGER DEFAULT 0,
    
    -- Recent activity (last 10 seconds)
    recent_kills_allies INTEGER DEFAULT 0,
    recent_kills_axis INTEGER DEFAULT 0,
    recent_damage_allies BIGINT DEFAULT 0,
    recent_damage_axis BIGINT DEFAULT 0
);

CREATE INDEX idx_momentum_battle ON battle_momentum(battle_id, seconds_elapsed);

-- =============================================================================
-- TABLE: battle_heatmap (Kill/death locations)
-- =============================================================================
CREATE TABLE IF NOT EXISTS battle_heatmap (
    heatmap_id BIGSERIAL PRIMARY KEY,
    battle_id INTEGER NOT NULL REFERENCES battles(battle_id) ON DELETE CASCADE,
    round_id INTEGER REFERENCES battle_rounds(round_id),
    
    event_type VARCHAR(20) NOT NULL, -- 'kill', 'death', 'objective'
    x_coord FLOAT,
    y_coord FLOAT,
    z_coord FLOAT,
    
    player_guid VARCHAR(64),
    team VARCHAR(20),
    weapon VARCHAR(100),
    
    timestamp TIMESTAMP NOT NULL
);

CREATE INDEX idx_heatmap_battle ON battle_heatmap(battle_id);
CREATE INDEX idx_heatmap_coords ON battle_heatmap(battle_id, x_coord, y_coord);

-- =============================================================================
-- VIEW: battle_summary (Quick overview for battle list)
-- =============================================================================
CREATE OR REPLACE VIEW battle_summary AS
SELECT 
    b.battle_id,
    b.battle_guid,
    b.server_id,
    b.map_name,
    b.game_type,
    b.started_at,
    b.ended_at,
    b.duration_seconds,
    b.winner_team,
    b.final_score_allies,
    b.final_score_axis,
    b.total_players,
    b.peak_players,
    b.total_kills,
    b.total_deaths,
    b.total_damage,
    
    -- Intensity metric (kills + damage per minute)
    CASE 
        WHEN b.duration_seconds > 0 THEN 
            ((b.total_kills + (b.total_damage / 100.0)) / (b.duration_seconds / 60.0))
        ELSE 0 
    END AS intensity_score,
    
    -- Top fragger
    (
        SELECT bp.player_name 
        FROM battle_players bp 
        WHERE bp.battle_id = b.battle_id 
        AND bp.round_id IS NULL 
        ORDER BY bp.kills DESC 
        LIMIT 1
    ) AS top_fragger,
    
    -- MVP (highest score)
    (
        SELECT bp.player_name 
        FROM battle_players bp 
        WHERE bp.battle_id = b.battle_id 
        AND bp.round_id IS NULL 
        ORDER BY bp.score DESC 
        LIMIT 1
    ) AS mvp,
    
    -- Round count
    (
        SELECT COUNT(*) 
        FROM battle_rounds br 
        WHERE br.battle_id = b.battle_id
    ) AS total_rounds

FROM battles b;

-- =============================================================================
-- VIEW: battle_player_rankings (Player performance within a battle)
-- =============================================================================
CREATE OR REPLACE VIEW battle_player_rankings AS
SELECT 
    bp.battle_id,
    bp.player_guid,
    bp.player_name,
    bp.team,
    bp.kills,
    bp.deaths,
    bp.assists,
    bp.score,
    bp.kd_ratio,
    bp.damage_dealt,
    bp.headshots,
    bp.accuracy_percent,
    
    -- Rankings within battle
    RANK() OVER (PARTITION BY bp.battle_id ORDER BY bp.score DESC) AS rank_by_score,
    RANK() OVER (PARTITION BY bp.battle_id ORDER BY bp.kills DESC) AS rank_by_kills,
    RANK() OVER (PARTITION BY bp.battle_id ORDER BY bp.kd_ratio DESC) AS rank_by_kd,
    
    -- Team rankings
    RANK() OVER (PARTITION BY bp.battle_id, bp.team ORDER BY bp.score DESC) AS team_rank

FROM battle_players bp
WHERE bp.round_id IS NULL; -- Only full battle stats

-- =============================================================================
-- FUNCTION: Create battle from match start
-- =============================================================================
CREATE OR REPLACE FUNCTION create_battle(
    p_server_id INTEGER,
    p_map_name VARCHAR,
    p_game_type VARCHAR,
    p_started_at TIMESTAMP
) RETURNS INTEGER AS $$
DECLARE
    v_battle_id INTEGER;
BEGIN
    INSERT INTO battles (server_id, map_name, game_type, started_at)
    VALUES (p_server_id, p_map_name, p_game_type, p_started_at)
    RETURNING battle_id INTO v_battle_id;
    
    -- For FFA, create a single "ghost round"
    IF p_game_type = 'ffa' THEN
        INSERT INTO battle_rounds (battle_id, round_number, started_at)
        VALUES (v_battle_id, 1, p_started_at);
    END IF;
    
    RETURN v_battle_id;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- FUNCTION: Update battle stats (called periodically)
-- =============================================================================
CREATE OR REPLACE FUNCTION update_battle_stats(p_battle_id INTEGER) RETURNS VOID AS $$
BEGIN
    UPDATE battles
    SET 
        total_kills = (SELECT COALESCE(SUM(kills), 0) FROM battle_players WHERE battle_id = p_battle_id AND round_id IS NULL),
        total_deaths = (SELECT COALESCE(SUM(deaths), 0) FROM battle_players WHERE battle_id = p_battle_id AND round_id IS NULL),
        total_damage = (SELECT COALESCE(SUM(damage_dealt), 0) FROM battle_players WHERE battle_id = p_battle_id AND round_id IS NULL),
        total_headshots = (SELECT COALESCE(SUM(headshots), 0) FROM battle_players WHERE battle_id = p_battle_id AND round_id IS NULL),
        total_players = (SELECT COUNT(DISTINCT player_guid) FROM battle_players WHERE battle_id = p_battle_id),
        updated_at = CURRENT_TIMESTAMP
    WHERE battle_id = p_battle_id;
END;
$$ LANGUAGE plpgsql;

-- =============================================================================
-- INDEXES for performance
-- =============================================================================
CREATE INDEX IF NOT EXISTS idx_battles_composite ON battles(server_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_battle_players_stats ON battle_players(battle_id, kills DESC, kd_ratio DESC);
CREATE INDEX IF NOT EXISTS idx_timeline_events ON battle_timeline(battle_id, event_type, timestamp);

-- =============================================================================
-- COMMENTS
-- =============================================================================
COMMENT ON TABLE battles IS 'Parent container for each map load (match/battle)';
COMMENT ON TABLE battle_rounds IS 'Individual rounds within a battle (TDM/OBJ have multiple)';
COMMENT ON TABLE battle_players IS 'Player statistics per battle/round';
COMMENT ON TABLE battle_timeline IS 'Event-by-event timeline for match replay';
COMMENT ON TABLE battle_momentum IS 'Score/activity snapshots for momentum visualization';
COMMENT ON TABLE battle_heatmap IS 'Spatial data for kill/death/objective heatmaps';
COMMENT ON COLUMN battles.game_type IS 'ffa, tdm, obj, rbm, ctf';
COMMENT ON COLUMN battle_players.playstyle_rusher_score IS 'High movement + aggression';
COMMENT ON COLUMN battle_players.playstyle_camper_score IS 'High prone time + low movement';
COMMENT ON COLUMN battle_players.playstyle_support_score IS 'High assists + objectives';
