package main

import (
	"database/sql"
	"strings"
)

// IsInvalidGUID checks if a GUID is legacy (like "GUID_ELGAN") or invalid (e.g. missing hyphens).
func IsInvalidGUID(guid string) bool {
	return guid == "GUID_ELGAN" || !strings.Contains(guid, "-")
}

// UpdatePlayerGUID updates the player's GUID in the database.
func UpdatePlayerGUID(db *sql.DB, memberID int, newGUID string) error {
	_, err := db.Exec("UPDATE smf_mohaa_identities SET player_guid = ? WHERE id_member = ?", newGUID, memberID)
	return err
}
