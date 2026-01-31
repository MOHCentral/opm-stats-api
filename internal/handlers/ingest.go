package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/openmohaa/stats-api/internal/models"
)

// IngestEvents handles POST /api/v1/ingest/events
// @Summary Ingest Game Events
// @Description Accepts newline-separated JSON events from game servers
// @Tags Ingestion
// @Accept json
// @Produce json
// @Security ServerToken
// @Param body body []models.RawEvent true "Events"
// @Success 202 {object} map[string]string "Accepted"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /ingest/events [post]
func (h *Handler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.errorResponse(w, http.StatusRequestEntityTooLarge, "Request body too large")
		return
	}
	defer r.Body.Close()

	h.logger.Infow("IngestEvents called", "bodyLength", len(body), "preview", string(body[:min(len(body), 200)]))

	lines := strings.Split(string(body), "\n")
	h.logger.Infow("Split body into lines", "lineCount", len(lines))
	processed := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			h.logger.Debugw("Skipping empty line", "lineNum", i)
			continue
		}

		h.logger.Infow("Processing line", "lineNum", i, "preview", line[:min(len(line), 100)])
		var event models.RawEvent
		// Support both JSON (if line starts with {) and URL-encoded
		if strings.HasPrefix(line, "{") {
			h.logger.Infow("Parsing as JSON", "lineNum", i)
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				h.logger.Warnw("Failed to unmarshal JSON event in batch", "error", err, "line", line)
				continue
			}
			h.logger.Infow("JSON parsed successfully", "eventType", event.Type)
		} else {
			h.logger.Infow("Parsing as URL-encoded", "lineNum", i)
			values, err := url.ParseQuery(line)
			if err != nil {
				h.logger.Warnw("Failed to parse URL-encoded event in batch", "error", err, "line", line)
				continue
			}
			var parseErr error
			event, parseErr = h.parseFormToEvent(values)
			if parseErr != nil {
				h.logger.Warnw("Failed to parse event fields", "error", parseErr, "line", line)
				continue
			}
			h.logger.Infow("URL-encoded parsed", "eventType", event.Type)
		}

		// Inject/Enforce ServerID from context if authenticated to prevent spoofing
		if sid, ok := r.Context().Value(ServerIDKey).(string); ok && sid != "" {
			event.ServerID = sid
		}

		// Validate event structure
		if err := ValidateStruct(&event); err != nil {
			h.logger.Warnw("Validation failed for event", "error", err, "lineNum", i, "type", event.Type)
			continue
		}

		if event.Type == "" {
			h.logger.Warnw("Event has empty type, skipping", "lineNum", i, "line", line[:min(len(line), 100)])
			continue
		}

		h.logger.Infow("Enqueueing event", "type", event.Type, "match_id", event.MatchID)
		if !h.pool.Enqueue(&event) {
			h.logger.Warn("Worker pool queue full, dropping remaining events in batch")
			break
		}
		processed++
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "accepted",
		"processed": processed,
	})
}

type eventParser struct {
	err error
}

func (p *eventParser) parseFloat(s string) float64 {
	if p.err != nil {
		return 0
	}
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		p.err = fmt.Errorf("invalid float %q: %w", s, err)
		return 0
	}
	return f
}

func (p *eventParser) parseInt(s string) int {
	if p.err != nil {
		return 0
	}
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		p.err = fmt.Errorf("invalid int %q: %w", s, err)
		return 0
	}
	return i
}

func (p *eventParser) parseInt64(s string) int64 {
	if p.err != nil {
		return 0
	}
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		p.err = fmt.Errorf("invalid int64 %q: %w", s, err)
		return 0
	}
	return i
}

func (p *eventParser) parseFloat32(s string) float32 {
	if p.err != nil {
		return 0
	}
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		p.err = fmt.Errorf("invalid float32 %q: %w", s, err)
		return 0
	}
	return float32(f)
}

// parseFormToEvent converts URL-encoded form data to RawEvent
func (h *Handler) parseFormToEvent(form url.Values) (models.RawEvent, error) {
	p := &eventParser{}
	event := models.RawEvent{
		Type:        models.EventType(form.Get("type")),
		MatchID:     form.Get("match_id"),
		SessionID:   form.Get("session_id"),
		ServerID:    form.Get("server_id"),
		ServerToken: form.Get("server_token"),
		MapName:     form.Get("map_name"),

		PlayerName: form.Get("player_name"),
		PlayerGUID: form.Get("player_guid"),
		PlayerTeam: form.Get("player_team"),

		AttackerName: form.Get("attacker_name"),
		AttackerGUID: form.Get("attacker_guid"),
		AttackerTeam: form.Get("attacker_team"),

		VictimName: form.Get("victim_name"),
		VictimGUID: form.Get("victim_guid"),
		VictimTeam: form.Get("victim_team"),

		Weapon:    form.Get("weapon"),
		OldWeapon: form.Get("old_weapon"),
		NewWeapon: form.Get("new_weapon"),
		Hitloc:    form.Get("hitloc"),
		Inflictor: form.Get("inflictor"),

		TargetName: form.Get("target_name"),
		TargetGUID: form.Get("target_guid"),

		OldTeam: form.Get("old_team"),
		NewTeam: form.Get("new_team"),
		Message: form.Get("message"),

		Gametype:    form.Get("gametype"),
		Timelimit:   form.Get("timelimit"),
		Fraglimit:   form.Get("fraglimit"),
		Maxclients:  form.Get("maxclients"),
		WinningTeam: form.Get("winning_team"),

		Item:       form.Get("item"),
		Entity:     form.Get("entity"),
		Projectile: form.Get("projectile"),
		Code:       form.Get("code"),

		Objective:       form.Get("objective"), // Also check objective_index if needed
		ObjectiveStatus: form.Get("objective_status"),
		BotID:           form.Get("bot_id"),
		Seat:            form.Get("seat"),
	}

	// Parse numeric fields
	event.Timestamp = p.parseFloat(form.Get("timestamp"))
	event.Damage = p.parseInt(form.Get("damage"))
	event.AmmoRemaining = p.parseInt(form.Get("ammo_remaining"))
	event.AlliesScore = p.parseInt(form.Get("allies_score"))
	event.AxisScore = p.parseInt(form.Get("axis_score"))
	event.RoundNumber = p.parseInt(form.Get("round_number"))
	event.TotalRounds = p.parseInt(form.Get("total_rounds"))
	event.PlayerCount = p.parseInt(form.Get("player_count"))
	event.ClientNum = p.parseInt(form.Get("client_num"))
	event.Count = p.parseInt(form.Get("count"))
	event.Duration = p.parseFloat(form.Get("duration"))

	// Parse SMF ID fields (Int64 for member IDs)
	event.PlayerSMFID = p.parseInt64(form.Get("player_smf_id"))
	event.AttackerSMFID = p.parseInt64(form.Get("attacker_smf_id"))
	event.VictimSMFID = p.parseInt64(form.Get("victim_smf_id"))
	event.TargetSMFID = p.parseInt64(form.Get("target_smf_id"))

	// Parse float fields (positions)
	event.PosX = p.parseFloat32(form.Get("pos_x"))
	event.PosY = p.parseFloat32(form.Get("pos_y"))
	event.PosZ = p.parseFloat32(form.Get("pos_z"))
	event.AttackerX = p.parseFloat32(form.Get("attacker_x"))
	event.AttackerY = p.parseFloat32(form.Get("attacker_y"))
	event.AttackerZ = p.parseFloat32(form.Get("attacker_z"))
	event.AttackerPitch = p.parseFloat32(form.Get("attacker_pitch"))
	event.AttackerYaw = p.parseFloat32(form.Get("attacker_yaw"))
	event.AttackerStance = form.Get("attacker_stance")
	event.VictimX = p.parseFloat32(form.Get("victim_x"))
	event.VictimY = p.parseFloat32(form.Get("victim_y"))
	event.VictimZ = p.parseFloat32(form.Get("victim_z"))
	event.VictimStance = form.Get("victim_stance")
	event.PlayerStance = form.Get("player_stance")
	event.TargetStance = form.Get("target_stance")
	event.AimPitch = p.parseFloat32(form.Get("aim_pitch"))
	event.AimYaw = p.parseFloat32(form.Get("aim_yaw"))
	event.FallHeight = p.parseFloat32(form.Get("fall_height"))
	event.Walked = p.parseFloat32(form.Get("walked"))
	event.Sprinted = p.parseFloat32(form.Get("sprinted"))
	event.Swam = p.parseFloat32(form.Get("swam"))
	event.Driven = p.parseFloat32(form.Get("driven"))
	event.Distance = p.parseFloat32(form.Get("distance"))

	if p.err != nil {
		return event, p.err
	}
	return event, nil
}

// IngestMatchResult handles POST /api/v1/ingest/match-result
// Synchronous processing for tournament integration
// @Summary Ingest Match Result
// @Tags Ingestion
// @Security ServerToken
// @Accept json
// @Produce json
// @Param body body models.MatchResult true "Match Result"
// @Success 200 {object} map[string]string "Processed"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /ingest/match-result [post]
func (h *Handler) IngestMatchResult(w http.ResponseWriter, r *http.Request) {
	var result models.MatchResult

	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if err := ValidateStruct(&result); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Validation failed: "+err.Error())
		return
	}

	// Tournament match results are handled by SMF plugin
	// See: smf-plugins/mohaa_tournaments/ for bracket management

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "processed",
	})
}
