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
		if sid, ok := r.Context().Value("server_id").(string); ok && sid != "" {
			if event.ServerID == "" {
				event.ServerID = sid
			}
		}

		// Validate event
		if err := h.validator.Struct(event); err != nil {
			h.logger.Warnw("Event validation failed", "error", err, "event", event)
			// We skip invalid events but continue processing the batch
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

	// Helper functions for safe parsing with error propagation
	parseInt := func(key string) (int, error) {
		s := form.Get(key)
		if s == "" {
			return 0, nil
		}
		return strconv.Atoi(s)
	}

	parseFloat := func(key string) (float64, error) {
		s := form.Get(key)
		if s == "" {
			return 0, nil
		}
		return strconv.ParseFloat(s, 64)
	}

	parseFloat32 := func(key string) (float32, error) {
		s := form.Get(key)
		if s == "" {
			return 0, nil
		}
		f, err := strconv.ParseFloat(s, 32)
		return float32(f), err
	}

	parseInt64 := func(key string) (int64, error) {
		s := form.Get(key)
		if s == "" {
			return 0, nil
		}
		return strconv.ParseInt(s, 10, 64)
	}

	var err error
	if event.Timestamp, err = parseFloat("timestamp"); err != nil {
		return event, fmt.Errorf("invalid timestamp: %w", err)
	}
	if event.Damage, err = parseInt("damage"); err != nil {
		return event, fmt.Errorf("invalid damage: %w", err)
	}
	if event.AmmoRemaining, err = parseInt("ammo_remaining"); err != nil {
		return event, fmt.Errorf("invalid ammo_remaining: %w", err)
	}
	if event.AlliesScore, err = parseInt("allies_score"); err != nil {
		return event, fmt.Errorf("invalid allies_score: %w", err)
	}
	if event.AxisScore, err = parseInt("axis_score"); err != nil {
		return event, fmt.Errorf("invalid axis_score: %w", err)
	}
	if event.RoundNumber, err = parseInt("round_number"); err != nil {
		return event, fmt.Errorf("invalid round_number: %w", err)
	}
	if event.TotalRounds, err = parseInt("total_rounds"); err != nil {
		return event, fmt.Errorf("invalid total_rounds: %w", err)
	}
	if event.PlayerCount, err = parseInt("player_count"); err != nil {
		return event, fmt.Errorf("invalid player_count: %w", err)
	}
	if event.ClientNum, err = parseInt("client_num"); err != nil {
		return event, fmt.Errorf("invalid client_num: %w", err)
	}
	if event.Count, err = parseInt("count"); err != nil {
		return event, fmt.Errorf("invalid count: %w", err)
	}
	if event.Duration, err = parseFloat("duration"); err != nil {
		return event, fmt.Errorf("invalid duration: %w", err)
	}

	// Parse SMF ID fields (Int64 for member IDs)
	if event.PlayerSMFID, err = parseInt64("player_smf_id"); err != nil {
		return event, fmt.Errorf("invalid player_smf_id: %w", err)
	}
	if event.AttackerSMFID, err = parseInt64("attacker_smf_id"); err != nil {
		return event, fmt.Errorf("invalid attacker_smf_id: %w", err)
	}
	if event.VictimSMFID, err = parseInt64("victim_smf_id"); err != nil {
		return event, fmt.Errorf("invalid victim_smf_id: %w", err)
	}
	if event.TargetSMFID, err = parseInt64("target_smf_id"); err != nil {
		return event, fmt.Errorf("invalid target_smf_id: %w", err)
	}

	// Parse float fields (positions)
	if event.PosX, err = parseFloat32("pos_x"); err != nil {
		return event, fmt.Errorf("invalid pos_x: %w", err)
	}
	if event.PosY, err = parseFloat32("pos_y"); err != nil {
		return event, fmt.Errorf("invalid pos_y: %w", err)
	}
	if event.PosZ, err = parseFloat32("pos_z"); err != nil {
		return event, fmt.Errorf("invalid pos_z: %w", err)
	}
	if event.AttackerX, err = parseFloat32("attacker_x"); err != nil {
		return event, fmt.Errorf("invalid attacker_x: %w", err)
	}
	if event.AttackerY, err = parseFloat32("attacker_y"); err != nil {
		return event, fmt.Errorf("invalid attacker_y: %w", err)
	}
	if event.AttackerZ, err = parseFloat32("attacker_z"); err != nil {
		return event, fmt.Errorf("invalid attacker_z: %w", err)
	}
	if event.AttackerPitch, err = parseFloat32("attacker_pitch"); err != nil {
		return event, fmt.Errorf("invalid attacker_pitch: %w", err)
	}
	if event.AttackerYaw, err = parseFloat32("attacker_yaw"); err != nil {
		return event, fmt.Errorf("invalid attacker_yaw: %w", err)
	}
	event.AttackerStance = form.Get("attacker_stance")
	if event.VictimX, err = parseFloat32("victim_x"); err != nil {
		return event, fmt.Errorf("invalid victim_x: %w", err)
	}
	if event.VictimY, err = parseFloat32("victim_y"); err != nil {
		return event, fmt.Errorf("invalid victim_y: %w", err)
	}
	if event.VictimZ, err = parseFloat32("victim_z"); err != nil {
		return event, fmt.Errorf("invalid victim_z: %w", err)
	}
	event.VictimStance = form.Get("victim_stance")
	event.PlayerStance = form.Get("player_stance")
	event.TargetStance = form.Get("target_stance")
	if event.AimPitch, err = parseFloat32("aim_pitch"); err != nil {
		return event, fmt.Errorf("invalid aim_pitch: %w", err)
	}
	if event.AimYaw, err = parseFloat32("aim_yaw"); err != nil {
		return event, fmt.Errorf("invalid aim_yaw: %w", err)
	}
	if event.FallHeight, err = parseFloat32("fall_height"); err != nil {
		return event, fmt.Errorf("invalid fall_height: %w", err)
	}
	if event.Walked, err = parseFloat32("walked"); err != nil {
		return event, fmt.Errorf("invalid walked: %w", err)
	}
	if event.Sprinted, err = parseFloat32("sprinted"); err != nil {
		return event, fmt.Errorf("invalid sprinted: %w", err)
	}
	if event.Swam, err = parseFloat32("swam"); err != nil {
		return event, fmt.Errorf("invalid swam: %w", err)
	}
	if event.Driven, err = parseFloat32("driven"); err != nil {
		return event, fmt.Errorf("invalid driven: %w", err)
	}
	if event.Distance, err = parseFloat32("distance"); err != nil {
		return event, fmt.Errorf("invalid distance: %w", err)
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

	if err := h.validator.Struct(result); err != nil {
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
