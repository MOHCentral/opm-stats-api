-- Add kill milestone achievements that match code expectations
-- These achievements track cumulative kill count across all matches

INSERT INTO mohaa_achievements (achievement_code, achievement_name, description, category, tier, requirement_type, requirement_value, points, icon_url) VALUES
('first-blood', 'First Blood', 'Score your first kill in the arena', 'Combat', 'Bronze', 'simple_count', '{"count": 1, "event": "player_kill"}', 10, 'first_blood.png'),
('killer-bronze', 'Killer - Bronze', 'Prove your combat skills with 10 kills', 'Combat', 'Bronze', 'simple_count', '{"count": 10, "event": "player_kill"}', 25, 'killer_bronze.png'),
('killer-silver', 'Killer - Silver', 'Demonstrate lethality with 50 kills', 'Combat', 'Silver', 'simple_count', '{"count": 50, "event": "player_kill"}', 50, 'killer_silver.png'),
('killer-gold', 'Killer - Gold', 'Master the battlefield with 100 kills', 'Combat', 'Gold', 'simple_count', '{"count": 100, "event": "player_kill"}', 100, 'killer_gold.png'),
('killer-platinum', 'Killer - Platinum', 'Elite warrior status with 500 kills', 'Combat', 'Platinum', 'simple_count', '{"count": 500, "event": "player_kill"}', 250, 'killer_platinum.png'),
('killer-diamond', 'Killer - Diamond', 'Legendary status with 1000 kills', 'Combat', 'Diamond', 'simple_count', '{"count": 1000, "event": "player_kill"}', 500, 'killer_diamond.png');
