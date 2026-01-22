-- Server Favorites Table
-- Allows users to favorite servers for quick access

CREATE TABLE IF NOT EXISTS server_favorites (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_id VARCHAR(64) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    nickname VARCHAR(100), -- Custom nickname for the server
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Each user can only favorite a server once
    UNIQUE(user_id, server_id)
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_server_favorites_user ON server_favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_server_favorites_server ON server_favorites(server_id);
CREATE INDEX IF NOT EXISTS idx_server_favorites_created ON server_favorites(created_at DESC);

-- Add country column to servers if not exists
DO $$ BEGIN
    ALTER TABLE servers ADD COLUMN IF NOT EXISTS country VARCHAR(2);
    ALTER TABLE servers ADD COLUMN IF NOT EXISTS country_name VARCHAR(64);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

-- Add country column to players if not exists  
DO $$ BEGIN
    ALTER TABLE players ADD COLUMN IF NOT EXISTS country VARCHAR(2);
    ALTER TABLE players ADD COLUMN IF NOT EXISTS country_name VARCHAR(64);
EXCEPTION WHEN OTHERS THEN NULL;
END $$;

-- Server uptime tracking table
CREATE TABLE IF NOT EXISTS server_uptime_log (
    id SERIAL PRIMARY KEY,
    server_id VARCHAR(64) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status VARCHAR(10) NOT NULL CHECK (status IN ('online', 'offline')),
    player_count INTEGER DEFAULT 0,
    current_map VARCHAR(64),
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_uptime_server_time ON server_uptime_log(server_id, timestamp DESC);

-- View for server uptime percentages
CREATE OR REPLACE VIEW server_uptime_stats AS
SELECT 
    server_id,
    -- 24h uptime
    COALESCE(
        COUNT(*) FILTER (WHERE status = 'online' AND timestamp > NOW() - INTERVAL '24 hours') * 100.0 /
        NULLIF(COUNT(*) FILTER (WHERE timestamp > NOW() - INTERVAL '24 hours'), 0),
        0
    ) as uptime_24h,
    -- 7d uptime  
    COALESCE(
        COUNT(*) FILTER (WHERE status = 'online' AND timestamp > NOW() - INTERVAL '7 days') * 100.0 /
        NULLIF(COUNT(*) FILTER (WHERE timestamp > NOW() - INTERVAL '7 days'), 0),
        0
    ) as uptime_7d,
    -- 30d uptime
    COALESCE(
        COUNT(*) FILTER (WHERE status = 'online' AND timestamp > NOW() - INTERVAL '30 days') * 100.0 /
        NULLIF(COUNT(*) FILTER (WHERE timestamp > NOW() - INTERVAL '30 days'), 0),
        0
    ) as uptime_30d,
    MAX(timestamp) FILTER (WHERE status = 'online') as last_online,
    MAX(timestamp) FILTER (WHERE status = 'offline') as last_offline
FROM server_uptime_log
GROUP BY server_id;

COMMENT ON TABLE server_favorites IS 'User favorite servers for quick access';
COMMENT ON TABLE server_uptime_log IS 'Server status log for uptime tracking';
