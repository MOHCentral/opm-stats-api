-- ============================================================================
-- OpenMOHAA Stats System - Postgres Migration 003
-- Player GUID Registry (Authoritative Identity Mappings)
-- ============================================================================
--
-- This table stores CONFIRMED player GUID → SMF member_id mappings.
-- It's the source of truth for identity resolution.
--
-- When a player uses /login <token> in-game:
-- 1. Token is verified against login_tokens table
-- 2. GUID is recorded here with their SMF member_id
-- 3. All future events from this GUID can be linked to their SMF account
--
-- This is stored in Postgres (not just ClickHouse) because:
-- - SMF needs to query it for profile linking
-- - It's authoritative identity data, not just stats
-- - Lower latency for real-time lookups
-- ============================================================================

-- Player GUID Registry - Confirmed identities
CREATE TABLE IF NOT EXISTS player_guid_registry (
    id SERIAL PRIMARY KEY,
    
    -- The stable identifier
    player_guid VARCHAR(64) NOT NULL UNIQUE,
    
    -- SMF forum identity (references smf_members.id_member in SMF database)
    smf_member_id INT NOT NULL,
    
    -- Last known player name (for display when SMF lookup fails)
    last_known_name VARCHAR(64) NOT NULL,
    
    -- When was this identity first verified?
    first_verified_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- When was the last successful login?
    last_verified_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- Last activity timestamp
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- How many times has this GUID logged in?
    login_count INT DEFAULT 1,
    
    -- Confidence level (100 = verified via token, 50 = GUID match only)
    confidence INT DEFAULT 100,
    
    -- Is this the PRIMARY GUID for this SMF user?
    is_primary BOOLEAN DEFAULT TRUE
    
    -- Note: No FK to smf_members as it's in a different database (SMF forum)
);

CREATE INDEX IF NOT EXISTS idx_guid_registry_smf ON player_guid_registry(smf_member_id);
CREATE INDEX IF NOT EXISTS idx_guid_registry_last_seen ON player_guid_registry(last_seen_at);

-- Trigger to ensure only one primary GUID per SMF user
CREATE OR REPLACE FUNCTION ensure_single_primary_guid()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_primary THEN
        UPDATE player_guid_registry 
        SET is_primary = FALSE 
        WHERE smf_member_id = NEW.smf_member_id 
          AND id != NEW.id 
          AND is_primary = TRUE;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_single_primary_guid ON player_guid_registry;
CREATE TRIGGER trg_single_primary_guid
AFTER INSERT OR UPDATE ON player_guid_registry
FOR EACH ROW
EXECUTE FUNCTION ensure_single_primary_guid();

-- Player name aliases - all names used by a GUID
CREATE TABLE IF NOT EXISTS player_name_aliases (
    id SERIAL PRIMARY KEY,
    player_guid VARCHAR(64) NOT NULL REFERENCES player_guid_registry(player_guid) ON DELETE CASCADE,
    player_name VARCHAR(64) NOT NULL,
    
    first_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    use_count INT DEFAULT 1,
    
    UNIQUE(player_guid, player_name)
);

CREATE INDEX IF NOT EXISTS idx_name_aliases_name ON player_name_aliases(player_name);

-- Unverified players - GUIDs we've seen but not yet linked to SMF
-- This is for future "claim this identity" functionality
CREATE TABLE IF NOT EXISTS unverified_players (
    id SERIAL PRIMARY KEY,
    player_guid VARCHAR(64) NOT NULL UNIQUE,
    
    -- Most common name used
    primary_name VARCHAR(64) NOT NULL,
    
    -- Stats for matching
    first_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
    event_count INT DEFAULT 1,
    kill_count INT DEFAULT 0,
    
    -- Server where most often seen
    primary_server_id VARCHAR(64),
    
    -- Claimed by SMF user (pending verification) - references smf_members in SMF database
    claimed_by_member_id INT NULL,
    claimed_at TIMESTAMP NULL
    
    -- Note: No FK to smf_members as it's in a different database (SMF forum)
);

CREATE INDEX IF NOT EXISTS idx_unverified_name ON unverified_players(primary_name);
CREATE INDEX IF NOT EXISTS idx_unverified_last_seen ON unverified_players(last_seen_at);

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Resolve a player GUID to SMF member_id (returns NULL if unknown)
CREATE OR REPLACE FUNCTION resolve_guid_to_smf(p_guid VARCHAR(64))
RETURNS INT AS $$
DECLARE
    v_member_id INT;
BEGIN
    SELECT smf_member_id INTO v_member_id
    FROM player_guid_registry
    WHERE player_guid = p_guid
    LIMIT 1;
    
    RETURN v_member_id;
END;
$$ LANGUAGE plpgsql;

-- Get all GUIDs for an SMF member
CREATE OR REPLACE FUNCTION get_member_guids(p_member_id INT)
RETURNS TABLE(guid VARCHAR(64), name VARCHAR(64), is_primary BOOLEAN, last_seen TIMESTAMP) AS $$
BEGIN
    RETURN QUERY
    SELECT player_guid, last_known_name, player_guid_registry.is_primary, last_seen_at
    FROM player_guid_registry
    WHERE smf_member_id = p_member_id
    ORDER BY player_guid_registry.is_primary DESC, last_seen_at DESC;
END;
$$ LANGUAGE plpgsql;

-- Register or update a GUID → SMF mapping (called on successful /login)
CREATE OR REPLACE FUNCTION register_player_guid(
    p_guid VARCHAR(64),
    p_smf_id INT,
    p_name VARCHAR(64)
) RETURNS VOID AS $$
BEGIN
    -- Insert or update the registry
    INSERT INTO player_guid_registry (player_guid, smf_member_id, last_known_name)
    VALUES (p_guid, p_smf_id, p_name)
    ON CONFLICT (player_guid) DO UPDATE SET
        smf_member_id = p_smf_id,
        last_known_name = p_name,
        last_verified_at = NOW(),
        last_seen_at = NOW(),
        login_count = player_guid_registry.login_count + 1;
    
    -- Also record the name alias
    INSERT INTO player_name_aliases (player_guid, player_name)
    VALUES (p_guid, p_name)
    ON CONFLICT (player_guid, player_name) DO UPDATE SET
        last_seen_at = NOW(),
        use_count = player_name_aliases.use_count + 1;
    
    -- Remove from unverified if present
    DELETE FROM unverified_players WHERE player_guid = p_guid;
END;
$$ LANGUAGE plpgsql;

-- Record an unverified player (called when we see events from unknown GUIDs)
CREATE OR REPLACE FUNCTION record_unverified_player(
    p_guid VARCHAR(64),
    p_name VARCHAR(64),
    p_server_id VARCHAR(64)
) RETURNS VOID AS $$
BEGIN
    -- Skip if already verified
    IF EXISTS (SELECT 1 FROM player_guid_registry WHERE player_guid = p_guid) THEN
        RETURN;
    END IF;
    
    INSERT INTO unverified_players (player_guid, primary_name, primary_server_id)
    VALUES (p_guid, p_name, p_server_id)
    ON CONFLICT (player_guid) DO UPDATE SET
        last_seen_at = NOW(),
        event_count = unverified_players.event_count + 1,
        primary_name = CASE 
            WHEN length(p_name) > 0 THEN p_name 
            ELSE unverified_players.primary_name 
        END;
END;
$$ LANGUAGE plpgsql;
