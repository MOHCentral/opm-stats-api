-- ============================================================================
-- PostgreSQL Schema for MOHAA Stats
-- Stateful data - OLTP workloads (users, tournaments, achievements)
-- ============================================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE TABLE users (
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
    is_banned BOOLEAN DEFAULT false,
    banned_reason TEXT,
    banned_until TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE INDEX idx_users_discord_id ON users(discord_id) WHERE discord_id IS NOT NULL;
CREATE INDEX idx_users_steam_id ON users(steam_id) WHERE steam_id IS NOT NULL;

-- Player identities (links web users to game GUIDs)
CREATE TABLE user_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    player_guid VARCHAR(64) NOT NULL,
    player_name VARCHAR(64),
    is_primary BOOLEAN DEFAULT false,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(user_id, player_guid)
);

CREATE INDEX idx_user_identities_player_guid ON user_identities(player_guid);
CREATE INDEX idx_user_identities_user_id ON user_identities(user_id);

-- Player aliases (track name changes)
CREATE TABLE player_aliases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_guid VARCHAR(64) NOT NULL,
    alias VARCHAR(64) NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    times_used INTEGER DEFAULT 1,
    
    UNIQUE(player_guid, alias)
);

CREATE INDEX idx_player_aliases_guid ON player_aliases(player_guid);

-- Sessions for JWT refresh tokens
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(64) NOT NULL,
    user_agent TEXT,
    ip_address INET,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ============================================================================
-- SERVERS
-- ============================================================================

CREATE TABLE servers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    token VARCHAR(64) UNIQUE NOT NULL,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Connection info
    address VARCHAR(255),
    port INTEGER,
    region VARCHAR(64),
    description TEXT,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    is_official BOOLEAN DEFAULT false,
    is_verified BOOLEAN DEFAULT false,
    
    -- Stats cache
    last_seen TIMESTAMPTZ,
    total_matches BIGINT DEFAULT 0,
    total_players BIGINT DEFAULT 0,
    total_events BIGINT DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_servers_owner_id ON servers(owner_id);
CREATE INDEX idx_servers_is_active ON servers(is_active);

-- ============================================================================
-- ACHIEVEMENTS
-- ============================================================================

CREATE TABLE achievements (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(64) NOT NULL,  -- combat, movement, social, tournament, etc
    icon_url TEXT,
    
    -- Requirements (JSON for flexibility)
    requirements JSONB NOT NULL DEFAULT '{}',
    
    -- Rewards
    points INTEGER DEFAULT 0,
    title VARCHAR(64),  -- Unlockable title
    
    -- Display
    is_hidden BOOLEAN DEFAULT false,
    is_secret BOOLEAN DEFAULT false,
    display_order INTEGER DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_achievements_category ON achievements(category);

-- Player achievement progress
CREATE TABLE player_achievements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    player_guid VARCHAR(64) NOT NULL,
    achievement_id VARCHAR(64) NOT NULL REFERENCES achievements(id),
    progress JSONB DEFAULT '{}',
    unlocked_at TIMESTAMPTZ,
    notified BOOLEAN DEFAULT false,
    
    UNIQUE(player_guid, achievement_id)
);

CREATE INDEX idx_player_achievements_guid ON player_achievements(player_guid);
CREATE INDEX idx_player_achievements_unlocked ON player_achievements(unlocked_at) WHERE unlocked_at IS NOT NULL;

-- ============================================================================
-- TOURNAMENTS
-- ============================================================================

CREATE TABLE tournaments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    description TEXT,
    
    -- Format
    format VARCHAR(32) NOT NULL,  -- single_elim, double_elim, swiss, round_robin
    status VARCHAR(32) NOT NULL DEFAULT 'draft',  -- draft, open, closed, in_progress, completed, cancelled
    
    -- Participation
    max_participants INTEGER NOT NULL DEFAULT 32,
    min_participants INTEGER DEFAULT 4,
    team_size INTEGER DEFAULT 1,  -- 1 for 1v1, 5 for 5v5, etc
    
    -- Game settings
    game_mode VARCHAR(64),  -- ffa, tdm, obj, etc
    timelimit INTEGER,
    fraglimit INTEGER,
    roundlimit INTEGER,
    best_of INTEGER DEFAULT 1,
    
    -- Schedule
    registration_start TIMESTAMPTZ,
    registration_end TIMESTAMPTZ,
    checkin_start TIMESTAMPTZ,
    checkin_end TIMESTAMPTZ,
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    
    -- Administration
    organizer_id UUID REFERENCES users(id),
    
    -- Cached counts
    participant_count INTEGER DEFAULT 0,
    current_round INTEGER DEFAULT 0,
    
    -- Seeding
    seeding_type VARCHAR(32) DEFAULT 'random',  -- random, manual, elo
    
    -- Prize info
    prize_pool TEXT,
    prize_distribution JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tournaments_status ON tournaments(status);
CREATE INDEX idx_tournaments_start_time ON tournaments(start_time);
CREATE INDEX idx_tournaments_organizer ON tournaments(organizer_id);

-- Tournament participants
CREATE TABLE tournament_participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    
    -- Player or team
    player_id VARCHAR(64),  -- player_guid for individuals
    team_id UUID,
    
    -- Status
    seed INTEGER,
    checked_in BOOLEAN DEFAULT false,
    checkin_time TIMESTAMPTZ,
    disqualified BOOLEAN DEFAULT false,
    disqualified_reason TEXT,
    
    -- Stats for Swiss/RR
    wins INTEGER DEFAULT 0,
    losses INTEGER DEFAULT 0,
    draws INTEGER DEFAULT 0,
    points INTEGER DEFAULT 0,
    buchholz NUMERIC(10,2) DEFAULT 0,  -- Swiss tiebreaker
    
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(tournament_id, player_id)
);

CREATE INDEX idx_tournament_participants_tournament ON tournament_participants(tournament_id);
CREATE INDEX idx_tournament_participants_player ON tournament_participants(player_id);

-- Tournament matches
CREATE TABLE tournament_matches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    
    -- Bracket position
    bracket_type VARCHAR(32) NOT NULL DEFAULT 'upper',  -- upper, lower, grand_final
    round_number INTEGER NOT NULL,
    match_number INTEGER NOT NULL,
    
    -- Participants
    participant1_id UUID REFERENCES tournament_participants(id),
    participant2_id UUID REFERENCES tournament_participants(id),
    
    -- Results
    participant1_score INTEGER DEFAULT 0,
    participant2_score INTEGER DEFAULT 0,
    winner_id UUID REFERENCES tournament_participants(id),
    
    -- Status
    status VARCHAR(32) NOT NULL DEFAULT 'pending',  -- pending, ready, in_progress, completed, bye
    scheduled_time TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- Game integration
    match_token VARCHAR(64) UNIQUE,  -- For game server to report results
    game_match_id UUID,  -- Links to ClickHouse match_id
    game_server_id UUID REFERENCES servers(id),
    
    -- Bracket advancement
    winner_advances_to UUID REFERENCES tournament_matches(id),
    loser_drops_to UUID REFERENCES tournament_matches(id),  -- For double elim
    
    -- Dispute handling
    disputed BOOLEAN DEFAULT false,
    dispute_reason TEXT,
    admin_notes TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(tournament_id, bracket_type, round_number, match_number)
);

CREATE INDEX idx_tournament_matches_tournament ON tournament_matches(tournament_id);
CREATE INDEX idx_tournament_matches_status ON tournament_matches(status);
CREATE INDEX idx_tournament_matches_token ON tournament_matches(match_token) WHERE match_token IS NOT NULL;

-- ============================================================================
-- TEAMS
-- ============================================================================

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(64) NOT NULL,
    tag VARCHAR(8),
    description TEXT,
    logo_url TEXT,
    banner_url TEXT,
    
    -- Leadership
    captain_id UUID REFERENCES users(id),
    
    -- Stats cache
    total_matches BIGINT DEFAULT 0,
    wins BIGINT DEFAULT 0,
    losses BIGINT DEFAULT 0,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    disbanded_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_teams_captain ON teams(captain_id);
CREATE INDEX idx_teams_is_active ON teams(is_active);

-- Team members
CREATE TABLE team_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) DEFAULT 'member',  -- captain, officer, member
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    
    UNIQUE(team_id, user_id)
);

CREATE INDEX idx_team_members_team ON team_members(team_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);

-- Team invites
CREATE TABLE team_invites (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    invited_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    invited_by_id UUID NOT NULL REFERENCES users(id),
    status VARCHAR(32) DEFAULT 'pending',  -- pending, accepted, declined, expired
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_team_invites_team ON team_invites(team_id);
CREATE INDEX idx_team_invites_user ON team_invites(invited_user_id);

-- ============================================================================
-- AUDIT LOG
-- ============================================================================

CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor_id UUID REFERENCES users(id),
    actor_type VARCHAR(32) NOT NULL,  -- user, system, admin
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID,
    old_values JSONB,
    new_values JSONB,
    metadata JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at);

-- ============================================================================
-- FUNCTIONS & TRIGGERS
-- ============================================================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_servers_updated_at
    BEFORE UPDATE ON servers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tournaments_updated_at
    BEFORE UPDATE ON tournaments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tournament_matches_updated_at
    BEFORE UPDATE ON tournament_matches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- SEED DATA: Achievements
-- ============================================================================

INSERT INTO achievements (id, name, description, category, points) VALUES
    -- Combat achievements
    ('KILL_100', 'First Blood Hunter', 'Get 100 kills', 'combat', 10),
    ('KILL_500', 'Seasoned Soldier', 'Get 500 kills', 'combat', 25),
    ('KILL_1000', 'Veteran', 'Get 1,000 kills', 'combat', 50),
    ('KILL_5000', 'War Hero', 'Get 5,000 kills', 'combat', 100),
    ('KILL_10000', 'Legend', 'Get 10,000 kills', 'combat', 200),
    
    ('HEADSHOT_50', 'Sharpshooter', 'Get 50 headshots', 'combat', 15),
    ('HEADSHOT_100', 'Marksman', 'Get 100 headshots', 'combat', 30),
    ('HEADSHOT_500', 'Sniper', 'Get 500 headshots', 'combat', 75),
    ('HEADSHOT_1000', 'Deadeye', 'Get 1,000 headshots', 'combat', 150),
    
    ('KILLSTREAK_5', 'Rampage', 'Get 5 kills without dying', 'combat', 20),
    ('KILLSTREAK_10', 'Unstoppable', 'Get 10 kills without dying', 'combat', 50),
    ('KILLSTREAK_15', 'Godlike', 'Get 15 kills without dying', 'combat', 100),
    
    -- Weapon mastery
    ('GARAND_1000', 'M1 Garand Master', 'Get 1,000 kills with M1 Garand', 'weapons', 100),
    ('THOMPSON_1000', 'Thompson Master', 'Get 1,000 kills with Thompson', 'weapons', 100),
    ('MP40_1000', 'MP40 Master', 'Get 1,000 kills with MP40', 'weapons', 100),
    ('KAR98_1000', 'Kar98k Master', 'Get 1,000 kills with Kar98k', 'weapons', 100),
    ('SPRINGFIELD_1000', 'Springfield Master', 'Get 1,000 kills with Springfield', 'weapons', 100),
    
    -- Tournament achievements
    ('TOURNAMENT_FIRST', 'Champion', 'Win a tournament', 'tournament', 200),
    ('TOURNAMENT_TOP3', 'Podium Finish', 'Finish top 3 in a tournament', 'tournament', 75),
    ('TOURNAMENT_10', 'Tournament Regular', 'Participate in 10 tournaments', 'tournament', 50),
    
    -- Social achievements
    ('MATCHES_100', 'Dedicated Player', 'Play 100 matches', 'social', 25),
    ('MATCHES_500', 'Loyal Soldier', 'Play 500 matches', 'social', 75),
    ('PLAYTIME_100', 'Century', 'Play for 100 hours', 'social', 100)
ON CONFLICT (id) DO NOTHING;
