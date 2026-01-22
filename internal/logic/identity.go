// Package logic provides the identity resolution service for linking game GUIDs to SMF members.
package logic

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// IdentityResolver resolves player GUIDs to SMF member IDs.
// It uses a multi-layer cache to minimize database lookups.
type IdentityResolver struct {
	postgres    *pgxpool.Pool
	redis       *redis.Client
	localCache  map[string]int64 // GUID -> SMF ID
	cacheMu     sync.RWMutex
	cacheExpiry time.Duration
}

// IdentityInfo contains the full identity information for a player.
type IdentityInfo struct {
	GUID          string    `json:"guid"`
	SMFID         int64     `json:"smf_id"`
	LastKnownName string    `json:"last_known_name"`
	ConfirmedAt   time.Time `json:"confirmed_at,omitempty"`
	FirstSeenAt   time.Time `json:"first_seen_at,omitempty"`
	VerifiedBySMF bool      `json:"verified"`
}

// NewIdentityResolver creates a new identity resolver with caching.
func NewIdentityResolver(postgres *pgxpool.Pool, redis *redis.Client) *IdentityResolver {
	return &IdentityResolver{
		postgres:    postgres,
		redis:       redis,
		localCache:  make(map[string]int64),
		cacheExpiry: 5 * time.Minute,
	}
}

// ResolveSMFID looks up the SMF member ID for a given GUID.
// Returns 0 if the GUID is not registered to any SMF account.
//
// Lookup order:
// 1. Local in-memory cache
// 2. Redis cache
// 3. Postgres player_guid_registry table
func (ir *IdentityResolver) ResolveSMFID(ctx context.Context, guid string) (int64, error) {
	if guid == "" {
		return 0, nil
	}

	// 1. Check local cache first (fastest)
	ir.cacheMu.RLock()
	if smfID, ok := ir.localCache[guid]; ok {
		ir.cacheMu.RUnlock()
		return smfID, nil
	}
	ir.cacheMu.RUnlock()

	// 2. Check Redis cache
	if ir.redis != nil {
		key := "guid:" + guid + ":smf_id"
		val, err := ir.redis.Get(ctx, key).Int64()
		if err == nil {
			ir.cacheMu.Lock()
			ir.localCache[guid] = val
			ir.cacheMu.Unlock()
			return val, nil
		}
	}

	// 3. Query Postgres
	var smfID int64
	query := `SELECT smf_member_id FROM player_guid_registry WHERE player_guid = $1`
	err := ir.postgres.QueryRow(ctx, query, guid).Scan(&smfID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// GUID not registered - cache the negative result as 0
			ir.cacheResult(ctx, guid, 0)
			return 0, nil
		}
		return 0, err
	}

	// Cache the result
	ir.cacheResult(ctx, guid, smfID)
	return smfID, nil
}

// ResolveBatch resolves multiple GUIDs at once for efficiency.
// Returns a map of GUID -> SMF ID (0 for unknown GUIDs).
func (ir *IdentityResolver) ResolveBatch(ctx context.Context, guids []string) (map[string]int64, error) {
	result := make(map[string]int64)
	if len(guids) == 0 {
		return result, nil
	}

	// Deduplicate and filter empty GUIDs
	guidSet := make(map[string]bool)
	var toResolve []string
	for _, guid := range guids {
		if guid != "" && !guidSet[guid] {
			guidSet[guid] = true
			toResolve = append(toResolve, guid)
		}
	}

	// Check local cache first
	var uncached []string
	ir.cacheMu.RLock()
	for _, guid := range toResolve {
		if smfID, ok := ir.localCache[guid]; ok {
			result[guid] = smfID
		} else {
			uncached = append(uncached, guid)
		}
	}
	ir.cacheMu.RUnlock()

	if len(uncached) == 0 {
		return result, nil
	}

	// Query Postgres for remaining GUIDs
	// Build parameterized query with $1, $2, etc.
	query := `SELECT player_guid, smf_member_id FROM player_guid_registry WHERE player_guid = ANY($1)`
	rows, err := ir.postgres.Query(ctx, query, uncached)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	foundGuids := make(map[string]bool)
	for rows.Next() {
		var guid string
		var smfID int64
		if err := rows.Scan(&guid, &smfID); err != nil {
			continue
		}
		result[guid] = smfID
		foundGuids[guid] = true
		ir.cacheResult(ctx, guid, smfID)
	}

	// Mark not-found GUIDs as 0 in cache
	for _, guid := range uncached {
		if !foundGuids[guid] {
			result[guid] = 0
			ir.cacheResult(ctx, guid, 0)
		}
	}

	return result, nil
}

// RegisterGUID links a GUID to an SMF member ID.
// This is called when a player authenticates via /login.
func (ir *IdentityResolver) RegisterGUID(ctx context.Context, guid string, smfID int64, playerName string) error {
	if guid == "" || smfID <= 0 {
		return nil
	}

	query := `
		INSERT INTO player_guid_registry (player_guid, smf_member_id, last_known_name, first_seen_at, last_seen_at, confirmed_at)
		VALUES ($1, $2, $3, NOW(), NOW(), NOW())
		ON CONFLICT (player_guid) DO UPDATE SET
			smf_member_id = EXCLUDED.smf_member_id,
			last_known_name = EXCLUDED.last_known_name,
			last_seen_at = NOW(),
			confirmed_at = NOW()
	`

	_, err := ir.postgres.Exec(ctx, query, guid, smfID, playerName)
	if err != nil {
		return err
	}

	// Update caches
	ir.cacheResult(ctx, guid, smfID)
	return nil
}

// UpdateLastSeen updates the last_seen timestamp for a GUID.
// Called when we see a player event (even if not authenticated).
func (ir *IdentityResolver) UpdateLastSeen(ctx context.Context, guid string, playerName string) error {
	if guid == "" {
		return nil
	}

	query := `
		INSERT INTO player_guid_registry (player_guid, smf_member_id, last_known_name, first_seen_at, last_seen_at)
		VALUES ($1, 0, $2, NOW(), NOW())
		ON CONFLICT (player_guid) DO UPDATE SET
			last_known_name = EXCLUDED.last_known_name,
			last_seen_at = NOW()
	`

	_, err := ir.postgres.Exec(ctx, query, guid, playerName)
	return err
}

// GetPlayerInfo returns the full identity information for a GUID.
func (ir *IdentityResolver) GetPlayerInfo(ctx context.Context, guid string) (*IdentityInfo, error) {
	var info IdentityInfo
	query := `
		SELECT player_guid, smf_member_id, last_known_name, confirmed_at, first_seen_at
		FROM player_guid_registry 
		WHERE player_guid = $1
	`
	err := ir.postgres.QueryRow(ctx, query, guid).Scan(&info.GUID, &info.SMFID, &info.LastKnownName, &info.ConfirmedAt, &info.FirstSeenAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	info.VerifiedBySMF = info.SMFID > 0 && !info.ConfirmedAt.IsZero()
	return &info, nil
}

// GetAllNameAliases returns all known name aliases for a GUID.
func (ir *IdentityResolver) GetAllNameAliases(ctx context.Context, guid string) ([]string, error) {
	query := `SELECT player_name FROM player_name_aliases WHERE player_guid = $1 ORDER BY last_used_at DESC`
	rows, err := ir.postgres.Query(ctx, query, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// RecordNameAlias records a name used by a GUID.
func (ir *IdentityResolver) RecordNameAlias(ctx context.Context, guid string, name string) error {
	if guid == "" || name == "" {
		return nil
	}

	query := `
		INSERT INTO player_name_aliases (player_guid, player_name, first_used_at, last_used_at, times_used)
		VALUES ($1, $2, NOW(), NOW(), 1)
		ON CONFLICT (player_guid, player_name) DO UPDATE SET
			last_used_at = NOW(),
			times_used = player_name_aliases.times_used + 1
	`
	_, err := ir.postgres.Exec(ctx, query, guid, name)
	return err
}

// cacheResult stores a GUID -> SMF ID mapping in both local and Redis caches.
func (ir *IdentityResolver) cacheResult(ctx context.Context, guid string, smfID int64) {
	// Local cache
	ir.cacheMu.Lock()
	ir.localCache[guid] = smfID
	ir.cacheMu.Unlock()

	// Redis cache
	if ir.redis != nil {
		key := "guid:" + guid + ":smf_id"
		ir.redis.Set(ctx, key, smfID, ir.cacheExpiry)
	}
}

// ClearCache clears all cached identity mappings.
func (ir *IdentityResolver) ClearCache() {
	ir.cacheMu.Lock()
	ir.localCache = make(map[string]int64)
	ir.cacheMu.Unlock()
}
