package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// GetGlobalWeaponStats returns weapon usage statistics
// @Summary Get Global Weapon Stats
// @Tags Server
// @Produce json
// @Success 200 {array} models.WeaponStats "Weapon Stats"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /stats/weapons [get]
func (h *Handler) GetGlobalWeaponStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT
			actor_weapon as weapon,
			countIf(event_type = 'kill') as kills,
			countIf(event_type = 'headshot') as headshots
		FROM mohaa_stats.raw_events
		WHERE actor_weapon != ''
		GROUP BY actor_weapon
		ORDER BY kills DESC
		LIMIT 10
	`)
	if err != nil {
		h.logger.Errorw("Failed to query weapon stats", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Query failed")
		return
	}
	defer rows.Close()

	type WeaponStats struct {
		Name      string `json:"name"`
		Kills     uint64 `json:"kills"`
		Headshots uint64 `json:"headshots"`
	}

	stats := make([]WeaponStats, 0)
	for rows.Next() {
		var s WeaponStats
		if err := rows.Scan(&s.Name, &s.Kills, &s.Headshots); err != nil {
			continue
		}
		stats = append(stats, s)
	}

	h.jsonResponse(w, http.StatusOK, stats)
}

// GetWeaponsList returns all weapons for dropdowns
func (h *Handler) GetWeaponsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.ch.Query(ctx, `
		SELECT DISTINCT actor_weapon
		FROM mohaa_stats.raw_events
		WHERE actor_weapon != '' AND event_type IN ('kill', 'weapon_fire')
		ORDER BY actor_weapon
	`)
	if err != nil {
		h.logger.Errorw("Failed to get weapons list", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	defer rows.Close()

	type weaponItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var result []weaponItem
	for rows.Next() {
		var wName string
		if err := rows.Scan(&wName); err == nil {
			result = append(result, weaponItem{
				ID:   wName,
				Name: wName,
			})
		}
	}
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{"weapons": result})
}

// GetWeaponDetail returns detailed statistics for a single weapon
func (h *Handler) GetWeaponDetail(w http.ResponseWriter, r *http.Request) {
	weapon := chi.URLParam(r, "weapon")
	if weapon == "" {
		h.errorResponse(w, http.StatusBadRequest, "Weapon required")
		return
	}

	ctx := r.Context()

	// Aggregate stats
	row := h.ch.QueryRow(ctx, `
		SELECT
			countIf(event_type = 'kill') as total_kills,
			countIf(event_type = 'headshot') as total_headshots,
			countIf(event_type = 'weapon_fire') as shots_fired,
			countIf(event_type = 'weapon_hit') as shots_hit,
			uniq(actor_id) as unique_users,
			max(timestamp) as last_used,
			avgIf(distance, event_type='kill') as avg_kill_distance
		FROM mohaa_stats.raw_events
		WHERE actor_weapon = ?
	`, weapon)

	var stats struct {
		Name            string    `json:"name"`
		TotalKills      uint64    `json:"total_kills"`
		TotalHeadshots  uint64    `json:"total_headshots"`
		ShotsFired      uint64    `json:"shots_fired"`
		ShotsHit        uint64    `json:"shots_hit"`
		UniqueUsers     uint64    `json:"unique_users"`
		LastUsed        time.Time `json:"last_used"`
		AvgKillDistance float64   `json:"avg_kill_distance"`
		Accuracy        float64   `json:"accuracy"`
		HeadshotRatio   float64   `json:"headshot_ratio"`
	}
	stats.Name = weapon

	if err := row.Scan(
		&stats.TotalKills,
		&stats.TotalHeadshots,
		&stats.ShotsFired,
		&stats.ShotsHit,
		&stats.UniqueUsers,
		&stats.LastUsed,
		&stats.AvgKillDistance,
	); err != nil {
		h.logger.Errorw("Failed to get weapon details", "error", err, "weapon", weapon)
	}

	if stats.ShotsFired > 0 {
		stats.Accuracy = float64(stats.ShotsHit) / float64(stats.ShotsFired) * 100
	}
	if stats.TotalKills > 0 {
		stats.HeadshotRatio = float64(stats.TotalHeadshots) / float64(stats.TotalKills) * 100
	}

	// Get top users for this weapon
	rows, err := h.ch.Query(ctx, `
		SELECT
			actor_id,
			any(actor_name) as name,
			count() as kills,
			countIf(event_type = 'headshot') as headshots,
			if(count() > 0, toFloat64(countIf(event_type='headshot'))/count()*100, 0) as hs_ratio
		FROM mohaa_stats.raw_events
		WHERE event_type = 'kill' AND actor_weapon = ? AND actor_id != ''
		GROUP BY actor_id
		ORDER BY kills DESC
		LIMIT 10
	`, weapon)

	type TopUser struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Kills     uint64  `json:"kills"`
		Headshots uint64  `json:"headshots"`
		HSRatio   float64 `json:"hs_ratio"`
	}
	var topUsers []TopUser

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var u TopUser
			if err := rows.Scan(&u.ID, &u.Name, &u.Kills, &u.Headshots, &u.HSRatio); err == nil {
				topUsers = append(topUsers, u)
			}
		}
	}

	response := map[string]interface{}{
		"stats":       stats,
		"top_players": topUsers,
	}

	h.jsonResponse(w, http.StatusOK, response)
}
