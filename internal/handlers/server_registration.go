package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/openmohaa/stats-api/internal/models"
)

// RegisterServer handles new server registration
// @Summary Register Server
// @Description Registers a new game server
// @Tags Server
// @Accept json
// @Produce json
// @Param body body models.RegisterServerRequest true "Server Info"
// @Success 200 {object} models.RegisterServerResponse "Server Credentials"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 500 {object} map[string]string "Internal Error"
// @Router /servers/register [post]
func (h *Handler) RegisterServer(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" || req.IPAddress == "" {
		h.errorResponse(w, http.StatusBadRequest, "Name and IP address are required")
		return
	}

	// Generate ID and Token
	serverID := uuid.New().String()
	token := uuid.New().String()
	tokenHash := hashToken(token) // Reuse existing hashToken function

	// Store in Postgres
	_, err := h.pg.Exec(r.Context(), `
		INSERT INTO servers (id, name, ip_address, port, token, is_active, last_seen)
		VALUES ($1, $2, $3, $4, $5, true, NOW())
		ON CONFLICT (ip_address, port) 
		DO UPDATE SET 
			name = EXCLUDED.name,
			token = EXCLUDED.token,
			is_active = true,
			last_seen = NOW()
		RETURNING id
	`, serverID, req.Name, req.IPAddress, string(req.Port), tokenHash)

	if err != nil {
		h.logger.Errorw("Failed to register server", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to register server")
		return
	}

	// Return credentials
	h.jsonResponse(w, http.StatusOK, models.RegisterServerResponse{
		ServerID: serverID,
		Token:    token,
	})
}
