-- ============================================================================
-- PostgreSQL Schema for MOHAA Stats
-- Stateful data - OLTP workloads (users, tournaments, achievements)
-- ============================================================================

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(64) UNIQUE,
    email VARCHAR(255) UNIQUE,
    avatar_url TEXT,
    
    -- OAuth providers
    discord_id VARCHAR(64) UNIQUE,
    discord_username VARCHAR(255),
    steam_id VARCHAR(64) UNIQUE,
    steam_username VARCHAR(255),
    
    -- Profile
    display_name VARCHAR(64),
    bio TEXT,
    country CHAR(2),
    
    -- Stats cache (updated periodically)
    total_kills BIGINT DEFAULT 0,
    total_deaths BIGINT DEFAULT 0,
    total_matches BIGINT DEFAULT 0,
    
    -- Metadata
    role VARCHAR(32) DEFAULT 'user',  -- user, moderator, admin
    is_admin BOOLEAN DEFAULT false,
    is_banned BOOLEAN DEFAULT false,
    banned_reason TEXT,
    banned_until TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_discord_id ON users(discord_id) WHERE discord_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_steam_id ON users(steam_id) WHERE steam_id IS NOT NULL;

-- Player identities (links web users to game GUIDs)
CREATE TABLE IF NOT EXISTS user_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_guid VARCHAR(64) NOT NULL,
    player_name VARCHAR(64),
    is_primary BOOLEAN DEFAULT false,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(user_id, player_guid)
);

-- Player aliases (track name changes)
CREATE TABLE IF NOT EXISTS player_aliases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_guid VARCHAR(64) NOT NULL,
    alias VARCHAR(64) NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    times_used INTEGER DEFAULT 1,
    
    UNIQUE(player_guid, alias)
);

-- ============================================================================
-- SERVERS
-- ============================================================================

CREATE TABLE IF NOT EXISTS servers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    token VARCHAR(256) UNIQUE NOT NULL, -- Increased length for potential future tokens
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Connection info
    address VARCHAR(255),
    ip_address VARCHAR(45),
    port INTEGER,
    region VARCHAR(64),
    description TEXT,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_official BOOLEAN DEFAULT false,
    is_verified BOOLEAN DEFAULT false,
    
    -- Stats cache
    max_players INTEGER DEFAULT 32,
    last_seen TIMESTAMPTZ,
    total_matches BIGINT DEFAULT 0,
    total_players BIGINT DEFAULT 0,
    total_events BIGINT DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_server_address UNIQUE (ip_address, port)
);

-- ============================================================================
-- ACHIEVEMENTS
-- ============================================================================

CREATE TABLE IF NOT EXISTS mohaa_achievements (
    achievement_id SERIAL PRIMARY KEY,
    achievement_code VARCHAR(100) UNIQUE NOT NULL,
    achievement_name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(50) NOT NULL,
    tier VARCHAR(20) NOT NULL DEFAULT 'Bronze',
    requirement_type VARCHAR(50) NOT NULL,
    requirement_value JSONB NOT NULL,
    points INT NOT NULL DEFAULT 10,
    icon_url VARCHAR(255),
    is_secret BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS mohaa_player_achievements (
    player_achievement_id SERIAL PRIMARY KEY,
    smf_member_id INT NOT NULL,
    achievement_id INT NOT NULL REFERENCES mohaa_achievements(achievement_id) ON DELETE CASCADE,
    progress INT DEFAULT 0,
    target INT NOT NULL,
    unlocked BOOLEAN DEFAULT FALSE,
    unlocked_at TIMESTAMP,
    progress_data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(smf_member_id, achievement_id)
);

-- ============================================================================
-- TOURNAMENTS
-- ============================================================================

CREATE TABLE IF NOT EXISTS tournaments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    description TEXT,
    format VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    max_participants INTEGER NOT NULL DEFAULT 32,
    min_participants INTEGER DEFAULT 4,
    team_size INTEGER DEFAULT 1,
    game_mode VARCHAR(64),
    best_of INTEGER DEFAULT 1,
    start_time TIMESTAMPTZ,
    organizer_id UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- TEAMS
-- ============================================================================

CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(64) NOT NULL,
    tag VARCHAR(8),
    captain_id UUID REFERENCES users(id),
    total_matches BIGINT DEFAULT 0,
    wins BIGINT DEFAULT 0,
    losses BIGINT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- LOGIN TOKENS (SMF Integration)
-- ============================================================================

CREATE TABLE IF NOT EXISTS login_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    forum_user_id INTEGER NOT NULL,
    token VARCHAR(12) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    used_player_guid VARCHAR(64),
    is_active BOOLEAN DEFAULT true
);

-- SMF User Mappings
CREATE TABLE IF NOT EXISTS smf_user_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    smf_member_id INTEGER NOT NULL UNIQUE,
    smf_username VARCHAR(80),
    stats_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    primary_guid VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- PLAYER IDENTITY REGISTRY
-- ============================================================================

CREATE TABLE IF NOT EXISTS player_guid_registry (
    id SERIAL PRIMARY KEY,
    player_guid VARCHAR(64) NOT NULL UNIQUE,
    smf_member_id INT NOT NULL,
    last_known_name VARCHAR(64) NOT NULL,
    first_verified_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_verified_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    login_count INT DEFAULT 1,
    is_primary BOOLEAN DEFAULT TRUE
);

-- ============================================================================
-- TRIGGERS & FUNCTIONS
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_servers_updated_at BEFORE UPDATE ON servers FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON teams FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_smf_user_mappings_updated_at BEFORE UPDATE ON smf_user_mappings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
