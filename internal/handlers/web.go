// Web handlers - SSR HTML pages using Go templates + HTMX
package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
)

var (
	templates *template.Template
)

// Template helper functions
var funcMap = template.FuncMap{
	"formatNumber": func(n interface{}) string {
		switch v := n.(type) {
		case int:
			if v >= 1000000 {
				return formatFloat(float64(v)/1000000, 1) + "M"
			}
			if v >= 1000 {
				return formatFloat(float64(v)/1000, 1) + "K"
			}
			return formatInt(v)
		case int64:
			if v >= 1000000 {
				return formatFloat(float64(v)/1000000, 1) + "M"
			}
			if v >= 1000 {
				return formatFloat(float64(v)/1000, 1) + "K"
			}
			return formatInt64(v)
		case float64:
			if v >= 1000000 {
				return formatFloat(v/1000000, 1) + "M"
			}
			if v >= 1000 {
				return formatFloat(v/1000, 1) + "K"
			}
			return formatFloat(v, 0)
		default:
			return "0"
		}
	},
	"timeAgo": func(t time.Time) string {
		duration := time.Since(t)
		switch {
		case duration < time.Minute:
			return "just now"
		case duration < time.Hour:
			return formatInt(int(duration.Minutes())) + "m ago"
		case duration < 24*time.Hour:
			return formatInt(int(duration.Hours())) + "h ago"
		default:
			return formatInt(int(duration.Hours()/24)) + "d ago"
		}
	},
	"formatDateTime": func(t time.Time) string {
		return t.Format("Jan 2, 2006 3:04 PM")
	},
	"formatDuration": func(seconds int) string {
		h := seconds / 3600
		m := (seconds % 3600) / 60
		if h > 0 {
			return formatInt(h) + "h " + formatInt(m) + "m"
		}
		return formatInt(m) + "m"
	},
	"formatMatchTime": func(seconds int) string {
		m := seconds / 60
		s := seconds % 60
		return formatInt(m) + ":" + padZero(s)
	},
	"add": func(a, b int) int { return a + b },
	"slice": func(s string, start, end int) string {
		if len(s) < end {
			end = len(s)
		}
		return s[start:end]
	},
	"upper": func(s string) string {
		if len(s) == 0 {
			return s
		}
		return string(s[0]-32) + s[1:]
	},
	"lower": func(s string) string {
		result := make([]byte, len(s))
		for i, c := range s {
			if c >= 'A' && c <= 'Z' {
				result[i] = byte(c + 32)
			} else {
				result[i] = byte(c)
			}
		}
		return string(result)
	},
	"json": func(v interface{}) template.JS {
		// For Alpine.js data binding
		return template.JS("{}")
	},
}

func formatInt(n int) string {
	return string(rune('0'+n/1000000%10)) + string(rune('0'+n/100000%10)) +
		string(rune('0'+n/10000%10)) + string(rune('0'+n/1000%10)) +
		string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

func formatInt64(n int64) string {
	return formatInt(int(n))
}

func formatFloat(f float64, decimals int) string {
	// Simple float formatting
	return string(rune('0'+int(f)%10)) + "." + string(rune('0'+int(f*10)%10))
}

func padZero(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// InitTemplates loads all templates at startup
func InitTemplates(templateDir string) error {
	pattern := filepath.Join(templateDir, "*.html")
	partialsPattern := filepath.Join(templateDir, "partials", "*.html")

	templates = template.Must(template.New("").Funcs(funcMap).ParseGlob(pattern))
	templates = template.Must(templates.ParseGlob(partialsPattern))

	return nil
}

// render renders a template with the given data
func (h *Handler) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	// Add common data
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Year"] = time.Now().Year()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check if templates are loaded
	if templates == nil {
		// Fallback to simple JSON for now
		h.jsonResponse(w, http.StatusOK, data)
		return
	}

	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Errorw("Template render error", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderPartial renders a partial template for HTMX
func (h *Handler) renderPartial(w http.ResponseWriter, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if templates == nil {
		h.jsonResponse(w, http.StatusOK, data)
		return
	}

	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Errorw("Partial render error", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ====== Page Handlers ======

// PageIndex renders the main dashboard
func (h *Handler) PageIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch dashboard stats
	stats, err := h.getDashboardStats(ctx)
	if err != nil {
		h.logger.Errorw("Failed to get dashboard stats", "error", err)
		stats = &DashboardStats{} // Use defaults
	}

	h.render(w, "index.html", map[string]interface{}{
		"Title":       "Dashboard",
		"Stats":       stats,
		"LiveMatches": h.getLiveMatchCount(ctx),
	})
}

// PageLogin renders the login/signup page
func (h *Handler) PageLogin(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in via cookie/session
	user, _ := h.getUserFromSession(r)

	h.render(w, "login.html", map[string]interface{}{
		"Title": "Sign In",
		"User":  user,
	})
}

// PagePlayer renders a player's profile page
func (h *Handler) PagePlayer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guid := chi.URLParam(r, "guid")

	player, err := h.getPlayerProfile(ctx, guid)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	stats, _ := h.getPlayerStats(ctx, guid)
	topWeapons, _ := h.getPlayerTopWeapons(ctx, guid, 5)
	achievements, _ := h.getPlayerAchievements(ctx, guid)
	weaponStats, _ := h.getPlayerAllWeaponStats(ctx, guid)

	h.render(w, "player.html", map[string]interface{}{
		"Title":        player.Name + " - Player Stats",
		"Player":       player,
		"Stats":        stats,
		"TopWeapons":   topWeapons,
		"WeaponStats":  weaponStats,
		"Achievements": achievements,
	})
}

// PageLeaderboard renders the global leaderboard
func (h *Handler) PageLeaderboard(w http.ResponseWriter, r *http.Request) {
	user, _ := h.getUserFromSession(r)

	h.render(w, "leaderboard.html", map[string]interface{}{
		"Title": "Leaderboards",
		"User":  user,
	})
}

// PageMatch renders a match details page
func (h *Handler) PageMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	matchID := chi.URLParam(r, "matchId")

	match, err := h.getMatchDetails(ctx, matchID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h.render(w, "match.html", map[string]interface{}{
		"Title": "Match " + matchID,
		"Match": match,
	})
}

// PageStats renders the global statistics page
func (h *Handler) PageStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	records, _ := h.getGlobalRecords(ctx)

	h.render(w, "stats.html", map[string]interface{}{
		"Title":   "Global Statistics",
		"Records": records,
	})
}

// PageMaps renders the maps overview page
func (h *Handler) PageMaps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	maps, _ := h.getMapsList(ctx)

	h.render(w, "maps.html", map[string]interface{}{
		"Title": "Maps",
		"Maps":  maps,
	})
}

// PageMapDetail renders a specific map's page
func (h *Handler) PageMapDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mapID := chi.URLParam(r, "mapId")

	maps, _ := h.getMapsList(ctx)
	selectedMap, err := h.getMapDetails(ctx, mapID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	h.render(w, "maps.html", map[string]interface{}{
		"Title":       selectedMap.Name + " - Map Details",
		"Maps":        maps,
		"SelectedMap": selectedMap,
	})
}

// NOTE: Tournament page handlers removed - SMF MariaDB is the source of truth
// See: smf-plugins/mohaa_tournaments/ for tournament management

// ====== Partial Handlers (HTMX) ======

// PartialLiveMatches returns HTML fragment of live matches
func (h *Handler) PartialLiveMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	matches, _ := h.getLiveMatches(ctx)

	h.renderPartial(w, "partials/live-matches.html", map[string]interface{}{
		"LiveMatches": matches,
	})
}

// PartialLeaderboard returns HTML fragment of leaderboard
func (h *Handler) PartialLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 10
	players, _ := h.getTopPlayers(ctx, limit)

	h.renderPartial(w, "partials/leaderboard.html", map[string]interface{}{
		"Players": players,
	})
}

// PartialRecentMatches returns HTML fragment of recent matches
func (h *Handler) PartialRecentMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	offset := 0 // Parse from query if needed
	limit := 10

	matches, hasMore := h.getRecentMatches(ctx, offset, limit)

	h.renderPartial(w, "partials/recent-matches.html", map[string]interface{}{
		"Matches":    matches,
		"HasMore":    hasMore,
		"NextOffset": offset + limit,
	})
}

// PartialPlayerCard returns HTML fragment of a player card
func (h *Handler) PartialPlayerCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guid := chi.URLParam(r, "guid")

	player, _ := h.getPlayerProfile(ctx, guid)
	stats, _ := h.getPlayerStats(ctx, guid)

	h.renderPartial(w, "partials/player-card.html", map[string]interface{}{
		"Player": player,
		"Stats":  stats,
	})
}

// PartialPlayerMatches returns HTML fragment of player's match history
func (h *Handler) PartialPlayerMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guid := chi.URLParam(r, "guid")
	offset := 0 // Parse from query
	limit := 10

	matches, hasMore := h.getPlayerMatchHistory(ctx, guid, offset, limit)

	h.renderPartial(w, "partials/player-matches.html", map[string]interface{}{
		"Matches":    matches,
		"HasMore":    hasMore,
		"NextOffset": offset + limit,
		"PlayerGUID": guid,
	})
}

// NOTE: PartialBracket removed - tournaments are managed by SMF

// ====== Helper types for pages ======

type DashboardStats struct {
	TotalKills    int64
	ActivePlayers int64
	MatchesPlayed int64
	LiveMatches   int
}

type PlayerProfile struct {
	GUID       string
	Name       string
	Verified   bool
	Rank       int
	LastActive time.Time
}

type PlayerStats struct {
	Kills         int64
	Deaths        int64
	KDRatio       float64
	Headshots     int64
	HSPercent     float64
	Accuracy      float64
	Matches       int64
	PlaytimeHours int
}

type WeaponStat struct {
	Name       string
	Category   string
	Kills      int64
	Headshots  int64
	Accuracy   float64
	HSRate     float64
	Percentage float64
}

type Achievement struct {
	ID          string
	Name        string
	Description string
	Unlocked    bool
	UnlockedAt  time.Time
	Progress    int
}

type MapInfo struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	TotalMatches    int64        `json:"total_matches"`
	TotalKills      int64        `json:"total_kills"`
	PopularWeapon   string       `json:"popular_weapon,omitempty"`
	AvgKillDistance float64      `json:"avg_kill_distance"`
	HSPercent       float64      `json:"hs_percent"`
	AvgDuration     int          `json:"avg_duration"`
	AvgPlayers      float64      `json:"avg_players"`
	TopWeapons      []WeaponStat `json:"top_weapons,omitempty"`
}

type MatchDetails struct {
	ID            string
	MapName       string
	GameMode      string
	StartTime     time.Time
	Duration      int
	PlayerCount   int
	TotalKills    int
	Headshots     int
	AvgAccuracy   float64
	LongestKill   float64
	AvgPing       int
	TeamMatch     bool
	AlliesScore   int
	AxisScore     int
	AlliesPlayers []MatchPlayer
	AxisPlayers   []MatchPlayer
	Players       []MatchPlayer
	Kills         []KillEvent
}

type MatchPlayer struct {
	GUID      string
	Name      string
	Team      string
	Kills     int
	Deaths    int
	KDRatio   float64
	Headshots int
	Accuracy  float64
	Score     int
}

type KillEvent struct {
	Time       int
	KillerGUID string
	KillerName string
	TargetGUID string
	TargetName string
	Weapon     string
	Headshot   bool
	Distance   float64
}

type GlobalRecords struct {
	MostKillsMatch RecordHolder
	LongestStreak  RecordHolder
	HighestKD      RecordHolder
	LongestShot    RecordHolder
}

type RecordHolder struct {
	PlayerGUID string
	PlayerName string
	Value      float64
}

// Tournament struct removed - SMF MariaDB is the source of truth
