-- Insert missing achievement definitions for Worker compatibility

-- Killer Tiers (Combat)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('killer_bronze', 'Killer I', 'Get 100 kills', 'Combat', 'Bronze', 'simple_count', '{"event": "player_kill", "count": 100}', 10),
('killer_silver', 'Killer II', 'Get 500 kills', 'Combat', 'Silver', 'simple_count', '{"event": "player_kill", "count": 500}', 25),
('killer_gold', 'Killer III', 'Get 1000 kills', 'Combat', 'Gold', 'simple_count', '{"event": "player_kill", "count": 1000}', 50),
('killer_platinum', 'Killer IV', 'Get 5000 kills', 'Combat', 'Platinum', 'simple_count', '{"event": "player_kill", "count": 5000}', 100),
('killer_diamond', 'Killer V', 'Get 10000 kills', 'Combat', 'Diamond', 'simple_count', '{"event": "player_kill", "count": 10000}', 250)
ON CONFLICT (achievement_code) DO NOTHING;

-- Streak Milestones (Combat)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('killing_spree', 'Killing Spree', 'Kill 5 enemies without dying', 'Combat', 'Bronze', 'combo', '{"event": "player_kill", "count": 5, "without_death": true}', 10),
('legendary', 'Legendary', 'Kill 20 enemies without dying', 'Combat', 'Diamond', 'combo', '{"event": "player_kill", "count": 20, "without_death": true}', 250)
ON CONFLICT (achievement_code) DO NOTHING;

-- Tank Destroyer Tiers (Vehicle)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('tank_destroyer_bronze', 'Tank Hunter I', 'Destroy 5 vehicles', 'Vehicle', 'Bronze', 'simple_count', '{"event": "vehicle_death", "count": 5}', 10),
('tank_destroyer_silver', 'Tank Hunter II', 'Destroy 25 vehicles', 'Vehicle', 'Silver', 'simple_count', '{"event": "vehicle_death", "count": 25}', 25),
('tank_destroyer_platinum', 'Tank Hunter IV', 'Destroy 100 vehicles', 'Vehicle', 'Platinum', 'simple_count', '{"event": "vehicle_death", "count": 100}', 100),
('tank_destroyer_diamond', 'Tank Hunter V', 'Destroy 250 vehicles', 'Vehicle', 'Diamond', 'simple_count', '{"event": "vehicle_death", "count": 250}', 250)
ON CONFLICT (achievement_code) DO NOTHING;

-- Health Hoarder (Survival)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('health_hoarder_bronze', 'Survivor I', 'Pickup 10 health packs', 'Survival', 'Bronze', 'simple_count', '{"event": "health_pickup", "count": 10}', 10),
('health_hoarder_silver', 'Survivor II', 'Pickup 50 health packs', 'Survival', 'Silver', 'simple_count', '{"event": "health_pickup", "count": 50}', 25),
('health_hoarder_gold', 'Survivor III', 'Pickup 100 health packs', 'Survival', 'Gold', 'simple_count', '{"event": "health_pickup", "count": 100}', 50),
('health_hoarder_platinum', 'Survivor IV', 'Pickup 250 health packs', 'Survival', 'Platinum', 'simple_count', '{"event": "health_pickup", "count": 250}', 100),
('health_hoarder_diamond', 'Survivor V', 'Pickup 500 health packs', 'Survival', 'Diamond', 'simple_count', '{"event": "health_pickup", "count": 500}', 250)
ON CONFLICT (achievement_code) DO NOTHING;

-- Objective Hero Tiers (Objective)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('objective_hero_bronze', 'Objective Player I', 'Capture 5 objectives', 'Objective', 'Bronze', 'simple_count', '{"event": "objective_capture", "count": 5}', 10),
('objective_hero_silver', 'Objective Player II', 'Capture 25 objectives', 'Objective', 'Silver', 'simple_count', '{"event": "objective_capture", "count": 25}', 25),
('objective_hero_platinum', 'Objective Player IV', 'Capture 250 objectives', 'Objective', 'Platinum', 'simple_count', '{"event": "objective_capture", "count": 250}', 100),
('objective_hero_diamond', 'Objective Player V', 'Capture 500 objectives', 'Objective', 'Diamond', 'simple_count', '{"event": "objective_capture", "count": 500}', 250)
ON CONFLICT (achievement_code) DO NOTHING;

-- Victor Tiers (Teamplay)
INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points) VALUES
('victor_bronze', 'Winner I', 'Win 10 matches', 'Social', 'Bronze', 'simple_count', '{"event": "match_win", "count": 10}', 10),
('victor_silver', 'Winner II', 'Win 25 matches', 'Social', 'Silver', 'simple_count', '{"event": "match_win", "count": 25}', 25),
('victor_gold', 'Winner III', 'Win 50 matches', 'Social', 'Gold', 'simple_count', '{"event": "match_win", "count": 50}', 50),
('victor_platinum', 'Winner IV', 'Win 100 matches', 'Social', 'Platinum', 'simple_count', '{"event": "match_win", "count": 100}', 100),
('victor_diamond', 'Winner V', 'Win 250 matches', 'Social', 'Diamond', 'simple_count', '{"event": "match_win", "count": 250}', 250)
ON CONFLICT (achievement_code) DO NOTHING;
