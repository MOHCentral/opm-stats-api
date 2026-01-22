-- ============================================================================
-- OpenMOHAA Stats System - ClickHouse Migration 004
-- Player Identity Resolution System
-- ============================================================================
-- 
-- This migration adds SMF identity fields to raw_events and creates
-- a player_sessions table for identity resolution.
--
-- The identity resolution flow:
-- 1. When a player logs in with /login <token>, we record their GUID → SMF ID
-- 2. When events occur, if the player is logged in, we include their SMF ID
-- 3. For non-logged-in players, we just have name/GUID
-- 4. The player_sessions table lets us resolve GUIDs to SMF IDs retroactively
-- ============================================================================

-- Add SMF ID columns to raw_events
-- These store the SMF member_id for actor and target when known
ALTER TABLE raw_events ADD COLUMN IF NOT EXISTS actor_smf_id UInt64 DEFAULT 0;
ALTER TABLE raw_events ADD COLUMN IF NOT EXISTS target_smf_id UInt64 DEFAULT 0;

-- Player sessions table - tracks every player connection with identity info
-- This is the KEY table for identity resolution
CREATE TABLE IF NOT EXISTS player_sessions (
    -- Session identification
    session_id String,                      -- Unique session ID
    server_id String,                       -- Server identifier
    match_id String DEFAULT '',             -- Current match (may change during session)
    
    -- Player identification (ALWAYS available)
    player_guid String,                     -- Game GUID (stable identifier)
    player_name String,                     -- Current in-game name
    
    -- SMF Identity (only if player logged in with /login <token>)
    smf_member_id UInt64 DEFAULT 0,         -- SMF forum user ID (0 = not logged in)
    auth_token String DEFAULT '',           -- Token used for login (hashed for security)
    authenticated_at DateTime64(3) DEFAULT toDateTime64(0, 3),
    
    -- Session timing
    connected_at DateTime,                  -- When player connected
    disconnected_at DateTime DEFAULT toDateTime(0),
    last_activity DateTime,                 -- Last event timestamp
    
    -- Session state
    team String DEFAULT '',
    is_active UInt8 DEFAULT 1,              -- 1 = still connected
    
    -- Metadata
    client_ip String DEFAULT '',            -- Player's IP (for security)
    
    -- Partition key
    _partition_date Date DEFAULT toDate(connected_at),
    
    PRIMARY KEY (server_id, player_guid, connected_at)
) ENGINE = ReplacingMergeTree(last_activity)
ORDER BY (server_id, player_guid, connected_at)
PARTITION BY toYYYYMM(_partition_date)
TTL _partition_date + INTERVAL 1 YEAR;

-- Player GUID registry - CONFIRMED GUID → SMF ID mappings
-- Only populated when a player successfully authenticates via /login
CREATE TABLE IF NOT EXISTS player_guid_registry (
    player_guid String,                     -- Game GUID (stable)
    smf_member_id UInt64,                   -- SMF forum user ID
    
    -- Last known name for this GUID
    last_known_name String,
    
    -- Verification info
    verified_at DateTime64(3),              -- When verified via token
    last_seen DateTime64(3),                -- Last activity with this GUID
    
    -- Statistics
    login_count UInt32 DEFAULT 1,           -- How many times logged in
    
    PRIMARY KEY (player_guid)
) ENGINE = ReplacingMergeTree(last_seen)
ORDER BY (player_guid);

-- Name history table - tracks all names used by each GUID
CREATE TABLE IF NOT EXISTS player_name_history (
    player_guid String,
    player_name String,
    smf_member_id UInt64 DEFAULT 0,
    
    first_seen DateTime64(3),
    last_seen DateTime64(3),
    use_count UInt32 DEFAULT 1,
    
    PRIMARY KEY (player_guid, player_name)
) ENGINE = ReplacingMergeTree(last_seen)
ORDER BY (player_guid, player_name);

-- Materialized view to automatically update player_guid_registry from auth events
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_player_auth_registry
TO player_guid_registry
AS SELECT
    actor_id AS player_guid,
    actor_smf_id AS smf_member_id,
    actor_name AS last_known_name,
    timestamp AS verified_at,
    timestamp AS last_seen,
    toUInt32(1) AS login_count
FROM raw_events
WHERE event_type = 'player_auth' AND actor_smf_id > 0;

-- Materialized view to track name history
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_player_name_history
TO player_name_history
AS SELECT
    actor_id AS player_guid,
    actor_name AS player_name,
    actor_smf_id AS smf_member_id,
    timestamp AS first_seen,
    timestamp AS last_seen,
    toUInt32(1) AS use_count
FROM raw_events
WHERE actor_id != '' AND actor_name != '';

-- ============================================================================
-- HELPER QUERIES FOR IDENTITY RESOLUTION
-- ============================================================================

-- Find SMF ID for a GUID:
-- SELECT smf_member_id FROM player_guid_registry WHERE player_guid = 'xxx' LIMIT 1;

-- Find all names used by a GUID:
-- SELECT player_name, use_count, last_seen 
-- FROM player_name_history 
-- WHERE player_guid = 'xxx' 
-- ORDER BY last_seen DESC;

-- Find all GUIDs for an SMF user:
-- SELECT player_guid, last_known_name, last_seen 
-- FROM player_guid_registry 
-- WHERE smf_member_id = 123;

-- Current active sessions on a server:
-- SELECT player_guid, player_name, smf_member_id, connected_at
-- FROM player_sessions
-- WHERE server_id = 'xxx' AND is_active = 1;
