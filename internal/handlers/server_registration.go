package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type RegisterServerRequest struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
}

type RegisterServerResponse struct {
	ServerID string `json:"server_id"`
	Token    string `json:"token"`
}

// RegisterServer handles new server registration
// POST /api/v1/servers/register
func (h *Handler) RegisterServer(w http.ResponseWriter, r *http.Request) {
	var req RegisterServerRequest
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
		INSERT INTO servers (id, name, ip_address, port, server_token_hash, is_active, last_seen)
		VALUES ($1, $2, $3, $4, $5, true, NOW())
		ON CONFLICT (ip_address, port) 
		DO UPDATE SET 
			name = EXCLUDED.name,
			server_token_hash = EXCLUDED.server_token_hash,
			is_active = true,
			last_seen = NOW()
		RETURNING id
	`, serverID, req.Name, req.IPAddress, req.Port, tokenHash)

	if err != nil {
		h.logger.Errorw("Failed to register server", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to register server")
		return
	}

	// Return credentials
	h.jsonResponse(w, http.StatusOK, RegisterServerResponse{
		ServerID: serverID,
		Token:    token,
	})
}
