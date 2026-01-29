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
		var parseErr error

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
			event, parseErr = h.parseFormToEvent(values)
			if parseErr != nil {
				h.logger.Warnw("Failed to convert form to event", "error", parseErr, "line", line)
				continue
			}
			h.logger.Infow("URL-encoded parsed", "eventType", event.Type)
		}

		// Inject ServerID from context if authenticated
		if sid, ok := r.Context().Value(serverIDKey).(string); ok && sid != "" {
			if event.ServerID == "" {
				event.ServerID = sid
			}
		}

		// Validate event
		if err := h.validator.Struct(event); err != nil {
			h.logger.Warnw("Event validation failed", "error", err, "event_type", event.Type)
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

// parseFormToEvent converts URL-encoded form data to RawEvent
func (h *Handler) parseFormToEvent(form url.Values) (models.RawEvent, error) {
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

	var err error

	// Parse numeric fields with strict error checking
	if s := form.Get("timestamp"); s != "" {
		event.Timestamp, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return event, fmt.Errorf("invalid timestamp: %w", err)
		}
	}

	if s := form.Get("damage"); s != "" {
		event.Damage, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid damage: %w", err)
		}
	}

	if s := form.Get("ammo_remaining"); s != "" {
		event.AmmoRemaining, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid ammo_remaining: %w", err)
		}
	}

	if s := form.Get("allies_score"); s != "" {
		event.AlliesScore, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid allies_score: %w", err)
		}
	}

	if s := form.Get("axis_score"); s != "" {
		event.AxisScore, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid axis_score: %w", err)
		}
	}

	if s := form.Get("round_number"); s != "" {
		event.RoundNumber, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid round_number: %w", err)
		}
	}

	if s := form.Get("total_rounds"); s != "" {
		event.TotalRounds, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid total_rounds: %w", err)
		}
	}

	if s := form.Get("player_count"); s != "" {
		event.PlayerCount, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid player_count: %w", err)
		}
	}

	if s := form.Get("client_num"); s != "" {
		event.ClientNum, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid client_num: %w", err)
		}
	}

	if s := form.Get("count"); s != "" {
		event.Count, err = strconv.Atoi(s)
		if err != nil {
			return event, fmt.Errorf("invalid count: %w", err)
		}
	}

	if s := form.Get("duration"); s != "" {
		event.Duration, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return event, fmt.Errorf("invalid duration: %w", err)
		}
	}

	// Parse SMF ID fields (Int64 for member IDs)
	if s := form.Get("player_smf_id"); s != "" {
		event.PlayerSMFID, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return event, fmt.Errorf("invalid player_smf_id: %w", err)
		}
	}

	if s := form.Get("attacker_smf_id"); s != "" {
		event.AttackerSMFID, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_smf_id: %w", err)
		}
	}

	if s := form.Get("victim_smf_id"); s != "" {
		event.VictimSMFID, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return event, fmt.Errorf("invalid victim_smf_id: %w", err)
		}
	}

	if s := form.Get("target_smf_id"); s != "" {
		event.TargetSMFID, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return event, fmt.Errorf("invalid target_smf_id: %w", err)
		}
	}

	// Parse float fields (positions)
	if s := form.Get("pos_x"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid pos_x: %w", err)
		}
		event.PosX = float32(v)
	}
	if s := form.Get("pos_y"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid pos_y: %w", err)
		}
		event.PosY = float32(v)
	}
	if s := form.Get("pos_z"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid pos_z: %w", err)
		}
		event.PosZ = float32(v)
	}

	if s := form.Get("attacker_x"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_x: %w", err)
		}
		event.AttackerX = float32(v)
	}
	if s := form.Get("attacker_y"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_y: %w", err)
		}
		event.AttackerY = float32(v)
	}
	if s := form.Get("attacker_z"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_z: %w", err)
		}
		event.AttackerZ = float32(v)
	}

	if s := form.Get("attacker_pitch"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_pitch: %w", err)
		}
		event.AttackerPitch = float32(v)
	}
	if s := form.Get("attacker_yaw"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid attacker_yaw: %w", err)
		}
		event.AttackerYaw = float32(v)
	}
	event.AttackerStance = form.Get("attacker_stance")

	if s := form.Get("victim_x"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid victim_x: %w", err)
		}
		event.VictimX = float32(v)
	}
	if s := form.Get("victim_y"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid victim_y: %w", err)
		}
		event.VictimY = float32(v)
	}
	if s := form.Get("victim_z"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid victim_z: %w", err)
		}
		event.VictimZ = float32(v)
	}
	event.VictimStance = form.Get("victim_stance")
	event.PlayerStance = form.Get("player_stance")
	event.TargetStance = form.Get("target_stance")

	if s := form.Get("aim_pitch"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid aim_pitch: %w", err)
		}
		event.AimPitch = float32(v)
	}
	if s := form.Get("aim_yaw"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid aim_yaw: %w", err)
		}
		event.AimYaw = float32(v)
	}

	if s := form.Get("fall_height"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid fall_height: %w", err)
		}
		event.FallHeight = float32(v)
	}
	if s := form.Get("walked"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid walked: %w", err)
		}
		event.Walked = float32(v)
	}
	if s := form.Get("sprinted"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid sprinted: %w", err)
		}
		event.Sprinted = float32(v)
	}
	if s := form.Get("swam"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid swam: %w", err)
		}
		event.Swam = float32(v)
	}
	if s := form.Get("driven"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid driven: %w", err)
		}
		event.Driven = float32(v)
	}
	if s := form.Get("distance"); s != "" {
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return event, fmt.Errorf("invalid distance: %w", err)
		}
		event.Distance = float32(v)
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

	// Tournament match results are handled by SMF plugin
	// See: smf-plugins/mohaa_tournaments/ for bracket management

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "processed",
	})
}
