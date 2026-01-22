-- =============================================
-- Achievement System Schema
-- =============================================
-- Creates tables for achievement definitions and player progress tracking
-- Supports event-triggered unlocks with flexible JSONB requirements

-- Drop existing tables if they exist
DROP TABLE IF EXISTS mohaa_player_achievements CASCADE;
DROP TABLE IF EXISTS mohaa_achievements CASCADE;

-- =============================================
-- Achievement Definitions Table
-- =============================================
CREATE TABLE mohaa_achievements (
    achievement_id SERIAL PRIMARY KEY,
    achievement_code VARCHAR(100) UNIQUE NOT NULL,
    achievement_name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(50) NOT NULL, -- Combat, Movement, Tactical, Combo, Map-Specific, Weapon-Specific, Social
    tier VARCHAR(20) NOT NULL DEFAULT 'Bronze', -- Bronze, Silver, Gold, Platinum, Diamond
    requirement_type VARCHAR(50) NOT NULL, -- simple_count, combo, contextual, temporal, efficiency
    requirement_value JSONB NOT NULL, -- Flexible requirement definition
    points INT NOT NULL DEFAULT 10, -- Achievement points (Bronze=10, Silver=25, Gold=50, Platinum=100, Diamond=250)
    icon_url VARCHAR(255),
    is_secret BOOLEAN DEFAULT FALSE, -- Hidden until unlocked
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_achievements_category ON mohaa_achievements(category);
CREATE INDEX idx_achievements_tier ON mohaa_achievements(tier);
CREATE INDEX idx_achievements_code ON mohaa_achievements(achievement_code);

-- =============================================
-- Player Achievement Progress Table
-- =============================================
CREATE TABLE mohaa_player_achievements (
    player_achievement_id SERIAL PRIMARY KEY,
    smf_member_id INT NOT NULL,
    achievement_id INT NOT NULL REFERENCES mohaa_achievements(achievement_id) ON DELETE CASCADE,
    progress INT DEFAULT 0,
    target INT NOT NULL,
    unlocked BOOLEAN DEFAULT FALSE,
    unlocked_at TIMESTAMP,
    progress_data JSONB, -- Store additional progress details (e.g., streak count, last event time)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(smf_member_id, achievement_id)
);

CREATE INDEX idx_player_achievements_member ON mohaa_player_achievements(smf_member_id);
CREATE INDEX idx_player_achievements_unlocked ON mohaa_player_achievements(unlocked);
CREATE INDEX idx_player_achievements_progress ON mohaa_player_achievements(smf_member_id, unlocked);

-- =============================================
-- Achievement Unlock Log Table
-- =============================================
CREATE TABLE mohaa_achievement_unlocks (
    unlock_id SERIAL PRIMARY KEY,
    smf_member_id INT NOT NULL,
    achievement_id INT NOT NULL REFERENCES mohaa_achievements(achievement_id) ON DELETE CASCADE,
    unlocked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    event_context JSONB -- Store the event that triggered the unlock
);

CREATE INDEX idx_achievement_unlocks_member ON mohaa_achievement_unlocks(smf_member_id);
CREATE INDEX idx_achievement_unlocks_time ON mohaa_achievement_unlocks(unlocked_at DESC);

-- =============================================
-- Insert Achievement Definitions
-- =============================================

-- ============ COMBAT ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('headshot_bronze', 'Headhunter', 'Score 100 headshots', 'Combat', 'Bronze', 'simple_count', '{"event": "player_headshot", "count": 100}', 10),
('headshot_silver', 'Headshot Master', 'Score 500 headshots', 'Combat', 'Silver', 'simple_count', '{"event": "player_headshot", "count": 500}', 25),
('headshot_gold', 'Headshot Legend', 'Score 1000 headshots', 'Combat', 'Gold', 'simple_count', '{"event": "player_headshot", "count": 1000}', 50),
('rampage', 'Rampage', 'Kill 5 enemies within 10 seconds', 'Combat', 'Gold', 'combo', '{"event": "player_kill", "count": 5, "window_seconds": 10}', 50),
('unstoppable', 'Unstoppable', 'Get 10 kills without dying', 'Combat', 'Platinum', 'combo', '{"event": "player_kill", "count": 10, "without_death": true}', 100),
('surgical_strike', 'Surgical Strike', 'Get 10 consecutive kills, all headshots', 'Combat', 'Diamond', 'combo', '{"event": "player_headshot", "consecutive": 10}', 250),
('spray_pray', 'Spray & Pray', 'Fire 1000 shots with less than 10% accuracy', 'Combat', 'Bronze', 'efficiency', '{"shots_fired": 1000, "max_accuracy": 0.10}', 10),
('one_tap_king', 'One-Tap King', 'Score 50 kills with exactly 1 shot per kill', 'Combat', 'Gold', 'contextual', '{"event": "player_kill", "shots_per_kill": 1, "count": 50}', 50);

-- ============ MOVEMENT ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('marathon_bronze', 'Marathon Runner', 'Travel 10km total distance', 'Movement', 'Bronze', 'simple_count', '{"event": "player_distance", "distance_meters": 10000}', 10),
('marathon_silver', 'Ultra Runner', 'Travel 50km total distance', 'Movement', 'Silver', 'simple_count', '{"event": "player_distance", "distance_meters": 50000}', 25),
('marathon_gold', 'Endurance Legend', 'Travel 100km total distance', 'Movement', 'Gold', 'simple_count', '{"event": "player_distance", "distance_meters": 100000}', 50),
('ghost', 'The Ghost', 'Travel 1000m without taking damage', 'Movement', 'Platinum', 'contextual', '{"distance": 1000, "zero_damage": true}', 100),
('leap_frog', 'Leap Frog', 'Score 50 kills while airborne (after jump)', 'Movement', 'Gold', 'contextual', '{"event": "player_kill", "count": 50, "airborne": true}', 50),
('olympic_sprinter', 'Olympic Sprinter', 'Sprint 5km without stopping', 'Movement', 'Silver', 'contextual', '{"distance": 5000, "continuous_sprint": true}', 25),
('verticality', 'Verticality Master', 'Score 100 kills while on ladder', 'Movement', 'Gold', 'contextual', '{"event": "player_kill", "count": 100, "on_ladder": true}', 50);

-- ============ TACTICAL ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('door_camper', 'Door Camper', 'Score 50 kills within 2 seconds of opening a door', 'Tactical', 'Silver', 'combo', '{"event": "player_kill", "count": 50, "after_door_open": true, "window_seconds": 2}', 25),
('peek_boo', 'Peek-a-Boo', 'Score 100 kills while crouched', 'Tactical', 'Bronze', 'contextual', '{"event": "player_kill", "count": 100, "stance": "crouch"}', 10),
('prone_sniper', 'Prone Sniper', 'Score 50 kills while prone', 'Tactical', 'Silver', 'contextual', '{"event": "player_kill", "count": 50, "stance": "prone"}', 25),
('ambush_master', 'Ambush Master', 'Score 25 kills on enemies not facing you', 'Tactical', 'Gold', 'contextual', '{"event": "player_kill", "count": 25, "from_behind": true}', 50),
('reload_master', 'Reload Master', 'Maintain 95%+ reload efficiency over 100 reloads', 'Tactical', 'Platinum', 'efficiency', '{"reloads": 100, "efficiency": 0.95}', 100);

-- ============ VEHICLE ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('tank_destroyer', 'Tank Destroyer', 'Destroy 50 vehicles', 'Vehicle', 'Gold', 'simple_count', '{"event": "vehicle_death", "count": 50}', 50),
('road_warrior', 'Road Warrior', 'Score 100 roadkill kills', 'Vehicle', 'Gold', 'simple_count', '{"event": "vehicle_roadkill", "count": 100}', 50),
('deadly_mechanic', 'Deadly Mechanic', 'Score 10 kills within 3 seconds of exiting vehicle', 'Vehicle', 'Silver', 'combo', '{"event": "player_kill", "count": 10, "after_vehicle_exit": true, "window_seconds": 3}', 25),
('turret_terror', 'Turret Terror', 'Score 50 kills while in turret', 'Vehicle', 'Silver', 'contextual', '{"event": "player_kill", "count": 50, "in_turret": true}', 25);

-- ============ BOT/AI ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('bot_hunter_bronze', 'Bot Hunter', 'Kill 100 bots', 'Bot', 'Bronze', 'simple_count', '{"event": "bot_killed", "count": 100}', 10),
('bot_hunter_silver', 'Bot Slayer', 'Kill 500 bots', 'Bot', 'Silver', 'simple_count', '{"event": "bot_killed", "count": 500}', 25),
('bot_hunter_gold', 'Bot Terminator', 'Kill 1000 bots', 'Bot', 'Gold', 'simple_count', '{"event": "bot_killed", "count": 1000}', 50),
('bot_bully', 'Bot Bully', 'Kill 10 bots without taking damage', 'Bot', 'Silver', 'contextual', '{"event": "bot_killed", "count": 10, "zero_damage": true}', 25),
('ai_whisperer', 'AI Whisperer', 'Kill 5 curious bots before they attack', 'Bot', 'Gold', 'contextual', '{"event": "bot_killed", "count": 5, "behavior": "curious", "before_attack": true}', 50);

-- ============ SURVIVAL ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('medic', 'Field Medic', 'Achieve net positive healing in 10 matches', 'Survival', 'Silver', 'contextual', '{"matches": 10, "healing_greater_than_damage": true}', 25),
('iron_man', 'Iron Man', 'Survive 10 minutes with less than 25% HP', 'Survival', 'Platinum', 'temporal', '{"duration_seconds": 600, "max_hp_percent": 0.25}', 100),
('bullet_magnet', 'Bullet Magnet', 'Take 1000 damage without dying', 'Survival', 'Gold', 'contextual', '{"damage_taken": 1000, "single_life": true}', 50),
('comeback_king', 'Comeback King', 'Win match after being last on scoreboard at halftime', 'Survival', 'Diamond', 'contextual', '{"last_at_halftime": true, "win": true, "count": 5}', 250);

-- ============ WEAPON-SPECIFIC ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('kar98k_elite', 'Kar98K Elite', 'Score 500 kills with Kar98K', 'Weapon', 'Gold', 'simple_count', '{"event": "player_kill", "weapon": "Kar98K", "count": 500}', 50),
('thompson_terror', 'Thompson Terror', 'Score 500 kills with Thompson', 'Weapon', 'Gold', 'simple_count', '{"event": "player_kill", "weapon": "Thompson", "count": 500}', 50),
('bazooka_specialist', 'Bazooka Specialist', 'Score 100 kills with Bazooka', 'Weapon', 'Silver', 'simple_count', '{"event": "player_kill", "weapon": "Bazooka", "count": 100}', 25),
('grenadier_bronze', 'Grenadier', 'Score 50 grenade kills', 'Weapon', 'Bronze', 'simple_count', '{"event": "grenade_kill", "count": 50}', 10),
('grenadier_silver', 'Master Grenadier', 'Score 200 grenade kills', 'Weapon', 'Silver', 'simple_count', '{"event": "grenade_kill", "count": 200}', 25),
('grenadier_gold', 'Grenade Legend', 'Score 500 grenade kills', 'Weapon', 'Gold', 'simple_count', '{"event": "grenade_kill", "count": 500}', 50),
('bash_master', 'Bash Master', 'Score 100 bash/melee kills', 'Weapon', 'Silver', 'simple_count', '{"event": "player_bash", "count": 100}', 25),
('sniper_efficiency', 'Sniper Efficiency', 'Score 50 Kar98K kills with over 40% accuracy', 'Weapon', 'Platinum', 'efficiency', '{"weapon": "Kar98K", "kills": 50, "min_accuracy": 0.40}', 100);

-- ============ MAP-SPECIFIC ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('brest_dominator', 'Brest Dominator', 'Win 50 matches on Brest', 'Map', 'Gold', 'simple_count', '{"event": "team_win", "map": "Brest", "count": 50}', 50),
('v2_expert', 'V2 Rocket Expert', 'Play 100 matches on V2 Rocket', 'Map', 'Silver', 'simple_count', '{"event": "match_end", "map": "V2 Rocket", "count": 100}', 25),
('stalingrad_survivor', 'Stalingrad Survivor', 'Win 10 matches on Stalingrad', 'Map', 'Silver', 'simple_count', '{"event": "team_win", "map": "Stalingrad", "count": 10}', 25),
('bazaar_specialist', 'Bazaar Specialist', 'Score 500 kills on Bazaar', 'Map', 'Gold', 'simple_count', '{"event": "player_kill", "map": "Bazaar", "count": 500}', 50);

-- ============ OBJECTIVE ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('objective_hero', 'Objective Hero', 'Capture 100 objectives', 'Objective', 'Gold', 'simple_count', '{"event": "objective_complete", "count": 100}', 50),
('first_strike', 'First Strike', 'Get first blood in 50 matches', 'Objective', 'Silver', 'contextual', '{"first_kill": true, "count": 50}', 25),
('denied', 'Denied', 'Kill 25 enemies actively on objective', 'Objective', 'Silver', 'contextual', '{"event": "player_kill", "count": 25, "on_objective": true}', 25),
('clutch_factor', 'Clutch Factor', 'Capture 10 objectives with less than 10% HP', 'Objective', 'Platinum', 'contextual', '{"event": "objective_complete", "count": 10, "max_hp_percent": 0.10}', 100);

-- ============ SOCIAL ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('chatty_cathy', 'Chatty Cathy', 'Send 1000 chat messages', 'Social', 'Bronze', 'simple_count', '{"event": "player_say", "count": 1000}', 10),
('vote_master', 'Vote Master', 'Start 100 votes', 'Social', 'Silver', 'simple_count', '{"event": "vote_start", "count": 100}', 25),
('democracy', 'Democracy Advocate', 'Participate in 500 votes', 'Social', 'Silver', 'simple_count', '{"event": "vote_cast", "count": 500}', 25),
('meme_lord', 'Meme Lord', 'Type "gg" 100 times in chat', 'Social', 'Bronze', 'contextual', '{"event": "player_say", "message_contains": "gg", "count": 100}', 10);

-- ============ COMBO ACHIEVEMENTS ============
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('pacifist_victory', 'Pacifist Victory', 'Win match with 0 kills (support only)', 'Combo', 'Diamond', 'contextual', '{"event": "team_win", "zero_kills": true, "count": 1}', 250),
('scavenger', 'Scavenger', 'Pick up 500 items', 'Combo', 'Silver', 'simple_count', '{"event": "item_pickup", "count": 500}', 25),
('loot_goblin', 'Loot Goblin', 'Pick up 10 items in single match', 'Combo', 'Bronze', 'contextual', '{"event": "item_pickup", "count": 10, "single_match": true}', 10),
('janitor', 'The Janitor', 'Score 100 kills on enemies with less than 25% HP', 'Combo', 'Silver', 'contextual', '{"event": "player_kill", "count": 100, "victim_hp_percent": 0.25}', 25),
('spiteful', 'The Spiteful', 'Score 50 kills within 2 seconds of sending chat message', 'Combo', 'Gold', 'combo', '{"event": "player_kill", "count": 50, "after_chat": true, "window_seconds": 2}', 50);

-- =============================================
-- Achievement Summary View
-- =============================================
CREATE OR REPLACE VIEW mohaa_achievement_summary AS
SELECT 
    category,
    tier,
    COUNT(*) as achievement_count,
    SUM(points) as total_points
FROM mohaa_achievements
GROUP BY category, tier
ORDER BY category, 
    CASE tier
        WHEN 'Bronze' THEN 1
        WHEN 'Silver' THEN 2
        WHEN 'Gold' THEN 3
        WHEN 'Platinum' THEN 4
        WHEN 'Diamond' THEN 5
    END;

-- =============================================
-- Player Achievement Progress View
-- =============================================
CREATE OR REPLACE VIEW mohaa_player_achievement_stats AS
SELECT 
    pa.smf_member_id,
    COUNT(*) as total_tracked,
    SUM(CASE WHEN pa.unlocked THEN 1 ELSE 0 END) as unlocked_count,
    SUM(CASE WHEN pa.unlocked THEN a.points ELSE 0 END) as total_points,
    ROUND(AVG(CASE WHEN NOT pa.unlocked THEN (pa.progress::FLOAT / pa.target * 100) ELSE NULL END), 2) as avg_progress_percent
FROM mohaa_player_achievements pa
JOIN mohaa_achievements a ON pa.achievement_id = a.achievement_id
GROUP BY pa.smf_member_id;

COMMENT ON TABLE mohaa_achievements IS 'Achievement definitions with flexible JSONB requirements';
COMMENT ON TABLE mohaa_player_achievements IS 'Player progress tracking for each achievement';
COMMENT ON TABLE mohaa_achievement_unlocks IS 'Log of all achievement unlocks with event context';
COMMENT ON VIEW mohaa_achievement_summary IS 'Summary of achievements by category and tier';
COMMENT ON VIEW mohaa_player_achievement_stats IS 'Player-level achievement statistics';
