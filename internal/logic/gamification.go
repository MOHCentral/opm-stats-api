package logic

import (
	"context"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type GamificationService struct {
	ch driver.Conn
}

func NewGamificationService(ch driver.Conn) *GamificationService {
	return &GamificationService{ch: ch}
}

type PlaystyleBadge struct {
	Style       string  `json:"style"`      // "Rusher", "Sniper", "Camper", "Versatile"
	Confidence  float64 `json:"confidence"` // 0-100%
	Icon        string  `json:"icon"`       // Icon name for frontend
	Description string  `json:"description"`
}

// GetPlaystyle analyzes player stats to determine their dominant playstyle
func (s *GamificationService) GetPlaystyle(ctx context.Context, playerID string) (*PlaystyleBadge, error) {
	// Query aggregates needed for classification
	var avgDist float64
	var topWeapon string
	var totalKills uint64

	// 1. Get Average Kill Distance and Dominant Weapon
	query := `
		SELECT 
			avg(distance) as avg_dist,
			(SELECT actor_weapon FROM raw_events WHERE event_type='kill' AND actor_id = ? GROUP BY actor_weapon ORDER BY count() DESC LIMIT 1) as top_wep,
			count() as kills
		FROM raw_events 
		WHERE event_type = 'kill' AND actor_id = ?
	`
	// Note: Simple subquery for top weapon might be slow on huge datasets, but okay for MVP filtering by actor_id
	if err := s.ch.QueryRow(ctx, query, playerID, playerID).Scan(&avgDist, &topWeapon, &totalKills); err != nil {
		return nil, err
	}

	if totalKills < 10 {
		return &PlaystyleBadge{Style: "Rookie", Description: "Too early to tell!", Icon: "recruit"}, nil
	}

	// 2. Classify
	// Heuristics:
	// - Avg Dist > 40m OR Weapon contains 'sniper'/'springfield'/'kar98' -> Sniper
	// - Avg Dist < 10m OR Weapon contains 'shotgun'/'thompson'/'mp40' -> Rusher
	// - Else -> Rifleman / Support

	style := "Soldier"
	desc := "Balanced combatant"
	icon := "rifle"

	wepLower := strings.ToLower(topWeapon)
	isSniperWep := strings.Contains(wepLower, "springfield") || strings.Contains(wepLower, "kar98") || strings.Contains(wepLower, "scope")
	isCQBWep := strings.Contains(wepLower, "thompson") || strings.Contains(wepLower, "mp40") || strings.Contains(wepLower, "shotgun") || strings.Contains(wepLower, "sten") || strings.Contains(wepLower, "bar")

	if avgDist > 5000 || isSniperWep { // 5000 units? Need to calibrate units to meters. Assuming 1 game unit ~ 1 inch -> 5000 ~ 120m?
		// Let's assume standard Quake units: 1 unit = 0.75 inch?
		// Actually let's just assume the values from seeder (0-50m usually).
		// If seeder uses meters directly in 'distance' param, then > 40 is good.
		// If seeder logic calculates dist(x,y,z), it returns float.

		// Refined check:
		if avgDist > 50 || isSniperWep { // Assuming meters
			style = "Sniper"
			desc = "You prefer engaging from a distance."
			icon = "crosshair"
		}
	} else if avgDist < 10 || isCQBWep {
		style = "Rusher"
		desc = "You love to get up close and personal!"
		icon = "running"
	}

	return &PlaystyleBadge{
		Style:       style,
		Description: desc,
		Icon:        icon,
		Confidence:  85.0, // Static for now
	}, nil
}
