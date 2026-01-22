-- ============================================================================
-- Login Tokens for SMF Forum Integration
-- Allows forum users to link their game identity via unique tokens
-- ============================================================================

-- Login tokens for device/game authentication
CREATE TABLE login_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Forum integration (SMF member ID)
    forum_user_id INTEGER NOT NULL,
    
    -- The unique token the player types in-game: /login <token>
    token VARCHAR(12) NOT NULL UNIQUE,
    
    -- Token lifecycle
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    
    -- If token was used successfully
    used_at TIMESTAMPTZ,
    used_from_ip INET,
    used_player_guid VARCHAR(64),
    
    -- Token status
    is_active BOOLEAN DEFAULT true,
    revoked_at TIMESTAMPTZ
);

-- Index for quick token lookups (what game servers will hit)
CREATE UNIQUE INDEX idx_login_tokens_token ON login_tokens(token) WHERE is_active = true;
CREATE INDEX idx_login_tokens_forum_user ON login_tokens(forum_user_id);
CREATE INDEX idx_login_tokens_expires ON login_tokens(expires_at) WHERE is_active = true;

-- Login history (for security tracking)
CREATE TABLE login_token_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Link to forum user
    forum_user_id INTEGER NOT NULL,
    
    -- The token that was used
    token VARCHAR(12) NOT NULL,
    
    -- When/where
    attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    server_name VARCHAR(128),
    server_address VARCHAR(255),
    player_ip INET,
    player_guid VARCHAR(64),
    
    -- What happened
    success BOOLEAN NOT NULL,
    failure_reason VARCHAR(128)  -- 'expired', 'revoked', 'already_used', etc.
);

CREATE INDEX idx_login_token_history_user ON login_token_history(forum_user_id);
CREATE INDEX idx_login_token_history_time ON login_token_history(attempt_at);

-- ============================================================================
-- SMF Forum User Mappings
-- Maps SMF member IDs to our stats system users
-- ============================================================================

CREATE TABLE smf_user_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- SMF forum member ID
    smf_member_id INTEGER NOT NULL UNIQUE,
    smf_username VARCHAR(80),
    
    -- Our stats system user (optional - created on first link)
    stats_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Linked game identities (can have multiple GUIDs)
    primary_guid VARCHAR(64),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_smf_user_mappings_member ON smf_user_mappings(smf_member_id);
CREATE INDEX idx_smf_user_mappings_guid ON smf_user_mappings(primary_guid) WHERE primary_guid IS NOT NULL;

-- Auto-update trigger
CREATE TRIGGER update_smf_user_mappings_updated_at
    BEFORE UPDATE ON smf_user_mappings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Helper function to generate short unique tokens
-- ============================================================================

CREATE OR REPLACE FUNCTION generate_login_token()
RETURNS VARCHAR(12) AS $$
DECLARE
    chars VARCHAR(36) := 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';  -- No I,O,0,1 to avoid confusion
    result VARCHAR(12) := '';
    i INTEGER;
BEGIN
    FOR i IN 1..8 LOOP
        result := result || substr(chars, floor(random() * length(chars) + 1)::integer, 1);
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql;
