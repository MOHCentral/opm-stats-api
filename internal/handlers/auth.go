package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/openmohaa/stats-api/internal/models"
)

// ============================================================================
// DEVICE AUTH HANDLERS (SMF Login Token System)
// ============================================================================

// InitDeviceAuth generates a unique login token for SMF forum users
// POST /api/v1/auth/device
// Body: {"forum_user_id": 123, "client_ip": "1.2.3.4"} or {"forum_user_id": 123, "regenerate": true, "client_ip": "1.2.3.4"}
// The client_ip is automatically added to trusted_ips so the user can login from the same IP they used to generate the token
func (h *Handler) InitDeviceAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ForumUserID int    `json:"forum_user_id"`
		Regenerate  bool   `json:"regenerate"`
		ClientIP    string `json:"client_ip"` // IP of the user generating the token (auto-trusted)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.ForumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	// If regenerating, revoke all existing tokens for this user
	// NOTE: We do NOT clear trusted IPs - user must manually remove them if compromised
	if req.Regenerate {
		_, err := h.pg.Exec(ctx, `
			UPDATE login_tokens 
			SET is_active = false, revoked_at = NOW() 
			WHERE forum_user_id = $1 AND is_active = true
		`, req.ForumUserID)
		if err != nil {
			h.logger.Errorw("Failed to revoke old tokens", "error", err, "forum_user_id", req.ForumUserID)
		}
	}

	// Check for existing active, unused token
	if !req.Regenerate {
		var existingToken string
		var expiresAt time.Time
		err := h.pg.QueryRow(ctx, `
			SELECT token, expires_at FROM login_tokens 
			WHERE forum_user_id = $1 AND is_active = true AND used_at IS NULL AND expires_at > NOW()
			ORDER BY created_at DESC LIMIT 1
		`, req.ForumUserID).Scan(&existingToken, &expiresAt)

		if err == nil && existingToken != "" {
			// Return existing valid token
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"user_code":  existingToken,
				"expires_in": int(time.Until(expiresAt).Seconds()),
				"expires_at": expiresAt,
				"is_new":     false,
			})
			return
		}
	}

	// Generate a new unique token (8 chars, no confusing chars)
	userCode := generateUserCode()
	expiresAt := time.Now().Add(10 * time.Minute)

	// Insert new token into Postgres
	_, err := h.pg.Exec(ctx, `
		INSERT INTO login_tokens (forum_user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, req.ForumUserID, userCode, expiresAt)

	if err != nil {
		h.logger.Errorw("Failed to create login token", "error", err, "forum_user_id", req.ForumUserID)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Auto-trust the IP that was used to generate the token
	if req.ClientIP != "" {
		_, err = h.pg.Exec(ctx, `
			INSERT INTO trusted_ips (forum_user_id, ip_address, source, label)
			VALUES ($1, $2::inet, 'website', 'Auto-approved (website)')
			ON CONFLICT (forum_user_id, ip_address) 
			DO UPDATE SET 
				is_active = true,
				last_used_at = NOW(),
				revoked_at = NULL
		`, req.ForumUserID, req.ClientIP)
		if err != nil {
			h.logger.Errorw("Failed to auto-trust client IP", "error", err, "ip", req.ClientIP)
			// Don't fail the request, just log it
		}
	}

	// Also cache in Redis for quick game server lookups
	h.redis.Set(ctx, "login_token:"+userCode, fmt.Sprintf("%d", req.ForumUserID), 10*time.Minute)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"user_code":  userCode,
		"expires_in": 600,
		"expires_at": expiresAt,
		"is_new":     true,
	})
}

// PollDeviceToken polls for completed device auth
func (h *Handler) PollDeviceToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	data, err := h.redis.Get(ctx, "device:"+req.DeviceCode).Bytes()
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Invalid or expired device code")
		return
	}

	var state models.DeviceAuthState
	json.Unmarshal(data, &state)

	switch state.Status {
	case "pending":
		h.errorResponse(w, http.StatusBadRequest, "authorization_pending")
	case "authorized":
		// Generate JWT
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": state.UserID.String(),
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, _ := token.SignedString(h.jwtSecret)

		h.jsonResponse(w, http.StatusOK, map[string]string{
			"access_token": tokenString,
			"token_type":   "Bearer",
		})

		// Clean up
		h.redis.Del(ctx, "device:"+req.DeviceCode)
	default:
		h.errorResponse(w, http.StatusBadRequest, "expired")
	}
}

// VerifyToken validates a login token from the game server
// This is called when a player types /login <token> in-game
// POST /api/v1/auth/verify
// Body: {"token": "ABC12345", "player_guid": "xxx", "server_name": "My Server", "player_ip": "1.2.3.4"}
//
// Security model:
// 1. First use: Token verified, IP becomes trusted
// 2. Same IP reconnect: Token already used but IP is trusted → Allow
// 3. New IP: Token already used, IP not trusted → Create pending approval request
func (h *Handler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Token         string `json:"token"`
		PlayerGUID    string `json:"player_guid"`
		ServerName    string `json:"server_name"`
		ServerAddress string `json:"server_address"`
		PlayerIP      string `json:"player_ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Token == "" {
		h.errorResponse(w, http.StatusBadRequest, "token is required")
		return
	}

	// Look up the token in database
	var forumUserID int
	var tokenID string
	var expiresAt time.Time
	var usedAt *time.Time
	var usedFromIP *string
	var isActive bool

	err := h.pg.QueryRow(ctx, `
		SELECT id, forum_user_id, expires_at, used_at, used_from_ip::text, is_active
		FROM login_tokens
		WHERE token = $1
	`, req.Token).Scan(&tokenID, &forumUserID, &expiresAt, &usedAt, &usedFromIP, &isActive)

	// Log the attempt
	logAttempt := func(success bool, reason string) {
		if forumUserID > 0 {
			h.pg.Exec(ctx, `
				INSERT INTO login_token_history 
				(forum_user_id, token, server_name, server_address, player_ip, player_guid, success, failure_reason)
				VALUES ($1, $2, $3, $4, $5::inet, $6, $7, $8)
			`, forumUserID, req.Token, req.ServerName, req.ServerAddress, req.PlayerIP, req.PlayerGUID, success, reason)
		}
	}

	if err != nil {
		logAttempt(false, "token_not_found")
		h.errorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Check if token is active
	if !isActive {
		logAttempt(false, "token_revoked")
		h.errorResponse(w, http.StatusUnauthorized, "Token has been revoked")
		return
	}

	// Check if token expired
	if time.Now().After(expiresAt) {
		logAttempt(false, "token_expired")
		h.errorResponse(w, http.StatusUnauthorized, "Token has expired")
		return
	}

	// Check if token was already used
	if usedAt != nil {
		// Token was used before - check if IP is trusted
		var isTrusted bool
		err := h.pg.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM trusted_ips 
				WHERE forum_user_id = $1 
				AND ip_address = $2::inet 
				AND is_active = true
			)
		`, forumUserID, req.PlayerIP).Scan(&isTrusted)

		if err != nil {
			h.logger.Errorw("Failed to check trusted IP", "error", err)
		}

		if isTrusted {
			// IP is trusted! Allow the login
			// Update last_used_at for this trusted IP
			h.pg.Exec(ctx, `
				UPDATE trusted_ips 
				SET last_used_at = NOW() 
				WHERE forum_user_id = $1 AND ip_address = $2::inet AND is_active = true
			`, forumUserID, req.PlayerIP)

			logAttempt(true, "trusted_ip_reconnect")

			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"valid":         true,
				"forum_user_id": forumUserID,
				"message":       "Reconnected from trusted IP",
				"trusted_ip":    true,
			})
			return
		}

		// IP is NOT trusted - check if there's already a pending request
		var pendingExists bool
		h.pg.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pending_ip_approvals 
				WHERE forum_user_id = $1 
				AND ip_address = $2::inet 
				AND status = 'pending'
				AND expires_at > NOW()
			)
		`, forumUserID, req.PlayerIP).Scan(&pendingExists)

		if !pendingExists {
			// Create a pending approval request
			_, err := h.pg.Exec(ctx, `
				INSERT INTO pending_ip_approvals 
				(forum_user_id, ip_address, player_guid, server_name, server_address)
				VALUES ($1, $2::inet, $3, $4, $5)
				ON CONFLICT (forum_user_id, ip_address) 
				DO UPDATE SET 
					player_guid = EXCLUDED.player_guid,
					server_name = EXCLUDED.server_name,
					server_address = EXCLUDED.server_address,
					requested_at = NOW(),
					expires_at = NOW() + INTERVAL '24 hours',
					status = 'pending',
					resolved_at = NULL
			`, forumUserID, req.PlayerIP, req.PlayerGUID, req.ServerName, req.ServerAddress)

			if err != nil {
				h.logger.Errorw("Failed to create pending IP approval", "error", err)
			}
		}

		logAttempt(false, "new_ip_pending_approval")
		h.jsonResponse(w, http.StatusForbidden, map[string]interface{}{
			"valid":            false,
			"error":            "new_ip_detected",
			"message":          "New IP detected. Please approve this IP on the website or generate a new token.",
			"pending_approval": true,
		})
		return
	}

	// Token is valid and unused! Mark it as used
	_, err = h.pg.Exec(ctx, `
		UPDATE login_tokens 
		SET used_at = NOW(), used_from_ip = $1::inet, used_player_guid = $2
		WHERE id = $3
	`, req.PlayerIP, req.PlayerGUID, tokenID)

	if err != nil {
		h.logger.Errorw("Failed to mark token as used", "error", err)
	}

	// Add this IP to trusted IPs
	_, err = h.pg.Exec(ctx, `
		INSERT INTO trusted_ips (forum_user_id, ip_address, source, player_guid)
		VALUES ($1, $2::inet, 'token_login', $3)
		ON CONFLICT (forum_user_id, ip_address) 
		DO UPDATE SET 
			is_active = true,
			last_used_at = NOW(),
			player_guid = COALESCE(EXCLUDED.player_guid, trusted_ips.player_guid),
			revoked_at = NULL
	`, forumUserID, req.PlayerIP, req.PlayerGUID)

	if err != nil {
		h.logger.Errorw("Failed to add trusted IP", "error", err)
	}

	// Link the player GUID to the forum user (create or update mapping)
	if req.PlayerGUID != "" {
		_, err = h.pg.Exec(ctx, `
			INSERT INTO smf_user_mappings (smf_member_id, primary_guid, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (smf_member_id) 
			DO UPDATE SET primary_guid = $2, updated_at = NOW()
		`, forumUserID, req.PlayerGUID)
		if err != nil {
			h.logger.Errorw("Failed to link GUID to user", "error", err)
		}
	}

	// Log success
	logAttempt(true, "")

	// Clean up from Redis
	h.redis.Del(ctx, "login_token:"+req.Token)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"valid":         true,
		"forum_user_id": forumUserID,
		"message":       "Identity verified successfully",
		"ip_trusted":    true,
	})
}

// SMFVerifyToken handles the specific format sent by tracker.scr (Form Data)
// POST /api/v1/auth/smf-verify
// Form Data: token, guid, player_name, server_ip, server_port, server_id
func (h *Handler) SMFVerifyToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	token := r.Form.Get("token")
	playerGUID := r.Form.Get("guid")
	playerName := r.Form.Get("player_name")
	// serverIP := r.Form.Get("server_ip")
	// serverPort := r.Form.Get("server_port")
	serverID := r.Form.Get("server_id")

	// IP of the player is the request IP, effectively
	// In a real setup, we might trust the game server to report the player's IP
	// but for now we'll use the request RemoteAddr if server_ip is the game server itself.
	// Actually, tracker.scr usually runs on the SERVER, so the request comes from the SERVER IP.
	// The player's IP might not be passed correctly unless we add it to the script.
	// For now, allow the server to proxy the auth.

	if token == "" {
		h.jsonResponse(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Token is required",
		})
		return
	}

	// Look up token
	var forumUserID int
	var tokenID string
	var expiresAt time.Time
	var usedAt *time.Time
	var isActive bool

	err := h.pg.QueryRow(ctx, `
		SELECT id, forum_user_id, expires_at, used_at, is_active
		FROM login_tokens
		WHERE token = $1
	`, token).Scan(&tokenID, &forumUserID, &expiresAt, &usedAt, &isActive)

	if err != nil {
		h.jsonResponse(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Invalid token",
		})
		return
	}

	if !isActive || time.Now().After(expiresAt) {
		h.jsonResponse(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Token expired or inactive",
		})
		return
	}

	// If used, we check if it was used for the SAME player recently?
	// For now, let's allow reuse if it's the valid session, or just enforce one-time use logic same as VerifyToken.
	// Simplified: Mark as used.

	if usedAt == nil {
		_, err = h.pg.Exec(ctx, `
			UPDATE login_tokens 
			SET used_at = NOW(), used_player_guid = $2
			WHERE id = $3
		`, "tracker_proxy", playerGUID, tokenID)
		if err != nil {
			h.logger.Errorw("Failed to mark token used", "error", err)
		}
	}

	// Link GUID
	if playerGUID != "" {
		_, err = h.pg.Exec(ctx, `
			INSERT INTO smf_user_mappings (smf_member_id, primary_guid, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (smf_member_id) 
			DO UPDATE SET primary_guid = $2, updated_at = NOW()
		`, forumUserID, playerGUID)
		if err != nil {
			h.logger.Errorw("Failed to link GUID", "error", err)
		}
	}

	// Log attempt
	h.pg.Exec(ctx, `
		INSERT INTO login_token_history 
		(forum_user_id, token, server_name, player_guid, success)
		VALUES ($1, $2, $3, $4, true)
	`, forumUserID, token, serverID, playerGUID)

	// Return format expected by tracker.scr
	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"member_id":   forumUserID,
		"member_name": playerName, // In real integration, fetch from forum DB
		"guid":        playerGUID,
	})
}

// GetLoginHistory returns login history for a forum user
// GET /api/v1/auth/history?forum_user_id=123
func (h *Handler) GetLoginHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	forumUserIDStr := r.URL.Query().Get("forum_user_id")
	if forumUserIDStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	forumUserID := 0
	fmt.Sscanf(forumUserIDStr, "%d", &forumUserID)
	if forumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "invalid forum_user_id")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT 
			attempt_at,
			server_name,
			server_address,
			player_ip::text,
			player_guid,
			success,
			failure_reason
		FROM login_token_history
		WHERE forum_user_id = $1
		ORDER BY attempt_at DESC
		LIMIT 20
	`, forumUserID)
	if err != nil {
		h.logger.Errorw("Failed to fetch login history", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type LoginHistoryEntry struct {
		AttemptAt     time.Time `json:"attempt_at"`
		ServerName    *string   `json:"server_name"`
		ServerAddress *string   `json:"server_address"`
		PlayerIP      *string   `json:"player_ip"`
		PlayerGUID    *string   `json:"player_guid"`
		Success       bool      `json:"success"`
		FailureReason *string   `json:"failure_reason"`
	}

	history := []LoginHistoryEntry{}
	for rows.Next() {
		var entry LoginHistoryEntry
		err := rows.Scan(
			&entry.AttemptAt,
			&entry.ServerName,
			&entry.ServerAddress,
			&entry.PlayerIP,
			&entry.PlayerGUID,
			&entry.Success,
			&entry.FailureReason,
		)
		if err != nil {
			h.logger.Errorw("Failed to scan login history row", "error", err)
			continue
		}
		history = append(history, entry)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forum_user_id": forumUserID,
		"history":       history,
		"count":         len(history),
	})
}

// ============================================================================
// TRUSTED IP MANAGEMENT
// ============================================================================

// GetTrustedIPs returns all trusted IPs for a forum user
// GET /api/v1/auth/trusted-ips?forum_user_id=123
func (h *Handler) GetTrustedIPs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	forumUserIDStr := r.URL.Query().Get("forum_user_id")
	if forumUserIDStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	forumUserID := 0
	fmt.Sscanf(forumUserIDStr, "%d", &forumUserID)
	if forumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "invalid forum_user_id")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT 
			id::text,
			host(ip_address),
			source,
			label,
			player_guid,
			created_at,
			last_used_at
		FROM trusted_ips
		WHERE forum_user_id = $1 AND is_active = true
		ORDER BY last_used_at DESC
	`, forumUserID)
	if err != nil {
		h.logger.Errorw("Failed to fetch trusted IPs", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type TrustedIP struct {
		ID         string    `json:"id"`
		IPAddress  string    `json:"ip_address"`
		Source     string    `json:"source"`
		Label      *string   `json:"label"`
		PlayerGUID *string   `json:"player_guid"`
		CreatedAt  time.Time `json:"created_at"`
		LastUsedAt time.Time `json:"last_used_at"`
	}

	trustedIPs := []TrustedIP{}
	for rows.Next() {
		var ip TrustedIP
		err := rows.Scan(
			&ip.ID,
			&ip.IPAddress,
			&ip.Source,
			&ip.Label,
			&ip.PlayerGUID,
			&ip.CreatedAt,
			&ip.LastUsedAt,
		)
		if err != nil {
			h.logger.Errorw("Failed to scan trusted IP row", "error", err)
			continue
		}
		trustedIPs = append(trustedIPs, ip)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forum_user_id": forumUserID,
		"trusted_ips":   trustedIPs,
		"count":         len(trustedIPs),
	})
}

// DeleteTrustedIP removes a trusted IP
// DELETE /api/v1/auth/trusted-ips/{id}?forum_user_id=123
func (h *Handler) DeleteTrustedIP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ipID := chi.URLParam(r, "id")

	forumUserIDStr := r.URL.Query().Get("forum_user_id")
	if forumUserIDStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	forumUserID := 0
	fmt.Sscanf(forumUserIDStr, "%d", &forumUserID)
	if forumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "invalid forum_user_id")
		return
	}

	// Soft delete - mark as inactive
	result, err := h.pg.Exec(ctx, `
		UPDATE trusted_ips 
		SET is_active = false, revoked_at = NOW()
		WHERE id = $1 AND forum_user_id = $2
	`, ipID, forumUserID)

	if err != nil {
		h.logger.Errorw("Failed to delete trusted IP", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}

	if result.RowsAffected() == 0 {
		h.errorResponse(w, http.StatusNotFound, "Trusted IP not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "deleted",
		"message": "IP removed from trusted list",
	})
}

// GetPendingIPApprovals returns pending IP approval requests
// GET /api/v1/auth/pending-ips?forum_user_id=123
func (h *Handler) GetPendingIPApprovals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	forumUserIDStr := r.URL.Query().Get("forum_user_id")
	if forumUserIDStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	forumUserID := 0
	fmt.Sscanf(forumUserIDStr, "%d", &forumUserID)
	if forumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "invalid forum_user_id")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT 
			id::text,
			host(ip_address),
			player_guid,
			player_name,
			server_name,
			server_address,
			requested_at,
			expires_at,
			notified_at
		FROM pending_ip_approvals
		WHERE forum_user_id = $1 AND status = 'pending' AND expires_at > NOW()
		ORDER BY requested_at DESC
	`, forumUserID)
	if err != nil {
		h.logger.Errorw("Failed to fetch pending IPs", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type PendingIP struct {
		ID            string     `json:"id"`
		IPAddress     string     `json:"ip_address"`
		PlayerGUID    *string    `json:"player_guid"`
		PlayerName    *string    `json:"player_name"`
		ServerName    *string    `json:"server_name"`
		ServerAddress *string    `json:"server_address"`
		RequestedAt   time.Time  `json:"requested_at"`
		ExpiresAt     time.Time  `json:"expires_at"`
		NotifiedAt    *time.Time `json:"notified_at"`
	}

	pendingIPs := []PendingIP{}
	for rows.Next() {
		var ip PendingIP
		err := rows.Scan(
			&ip.ID,
			&ip.IPAddress,
			&ip.PlayerGUID,
			&ip.PlayerName,
			&ip.ServerName,
			&ip.ServerAddress,
			&ip.RequestedAt,
			&ip.ExpiresAt,
			&ip.NotifiedAt,
		)
		if err != nil {
			h.logger.Errorw("Failed to scan pending IP row", "error", err)
			continue
		}
		pendingIPs = append(pendingIPs, ip)
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forum_user_id": forumUserID,
		"pending_ips":   pendingIPs,
		"count":         len(pendingIPs),
	})
}

// ResolvePendingIPApproval approves or denies a pending IP request
// POST /api/v1/auth/pending-ips/{id}
// Body: {"action": "approve"} or {"action": "deny"}
func (h *Handler) ResolvePendingIPApproval(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	approvalID := chi.URLParam(r, "id")

	var req struct {
		ForumUserID int    `json:"forum_user_id"`
		Action      string `json:"action"` // "approve" or "deny"
		Label       string `json:"label"`  // Optional label for approved IP
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.ForumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	if req.Action != "approve" && req.Action != "deny" {
		h.errorResponse(w, http.StatusBadRequest, "action must be 'approve' or 'deny'")
		return
	}

	// Get the pending request details
	var ipAddress, playerGUID string
	err := h.pg.QueryRow(ctx, `
		SELECT host(ip_address), COALESCE(player_guid, '')
		FROM pending_ip_approvals
		WHERE id = $1 AND forum_user_id = $2 AND status = 'pending'
	`, approvalID, req.ForumUserID).Scan(&ipAddress, &playerGUID)

	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Pending request not found")
		return
	}

	// Update the pending request status
	status := "denied"
	if req.Action == "approve" {
		status = "approved"
	}

	_, err = h.pg.Exec(ctx, `
		UPDATE pending_ip_approvals
		SET status = $2, resolved_at = NOW()
		WHERE id = $1
	`, approvalID, status)

	if err != nil {
		h.logger.Errorw("Failed to update pending IP", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}

	// If approved, add to trusted IPs
	if req.Action == "approve" {
		label := req.Label
		if label == "" {
			label = "Approved manually"
		}

		_, err = h.pg.Exec(ctx, `
			INSERT INTO trusted_ips (forum_user_id, ip_address, source, label, player_guid)
			VALUES ($1, $2::inet, 'manual_approval', $3, NULLIF($4, ''))
			ON CONFLICT (forum_user_id, ip_address) 
			DO UPDATE SET 
				is_active = true,
				last_used_at = NOW(),
				label = EXCLUDED.label,
				revoked_at = NULL
		`, req.ForumUserID, ipAddress, label, playerGUID)

		if err != nil {
			h.logger.Errorw("Failed to add trusted IP after approval", "error", err)
		}
	}

	message := "IP request denied"
	if req.Action == "approve" {
		message = "IP approved and added to trusted list"
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  status,
		"message": message,
	})
}

// MarkPendingIPsNotified marks pending IP approvals as email-notified
// POST /api/v1/auth/pending-ips/mark-notified
// Body: {"forum_user_id": 1, "ids": ["uuid1", "uuid2", ...]}
func (h *Handler) MarkPendingIPsNotified(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ForumUserID int      `json:"forum_user_id"`
		IDs         []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.ForumUserID <= 0 {
		h.errorResponse(w, http.StatusBadRequest, "forum_user_id is required")
		return
	}

	if len(req.IDs) == 0 {
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"updated": 0,
		})
		return
	}

	// Mark the specified pending IPs as notified
	result, err := h.pg.Exec(ctx, `
		UPDATE pending_ip_approvals
		SET notified_at = NOW()
		WHERE forum_user_id = $1 
		  AND id = ANY($2::uuid[]) 
		  AND status = 'pending' 
		  AND notified_at IS NULL
	`, req.ForumUserID, req.IDs)

	if err != nil {
		h.logger.Errorw("Failed to mark pending IPs as notified", "error", err)
		h.errorResponse(w, http.StatusInternalServerError, "database error")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"updated": result.RowsAffected(),
	})
}

// ============================================================================
// IDENTITY CLAIM HANDLERS
// ============================================================================

// InitIdentityClaim starts the identity claim process
func (h *Handler) InitIdentityClaim(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from JWT context (set by UserAuth middleware)
	userIDVal := ctx.Value("user_id")
	if userIDVal == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		h.errorResponse(w, http.StatusUnauthorized, "Invalid user context")
		return
	}

	code := generateUserCode()
	claim := models.IdentityClaim{
		ID:        uuid.New(),
		UserID:    userID,
		Code:      code,
		Status:    "pending",
		ExpiresAt: time.Now().Add(10 * time.Minute),
		CreatedAt: time.Now(),
	}

	// Store pending claim
	data, _ := json.Marshal(claim)
	h.redis.Set(ctx, "claim:"+code, data, 10*time.Minute)

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"code":       code,
		"expires_at": claim.ExpiresAt,
		"message":    fmt.Sprintf("In game, type: claim %s", code),
	})
}

// VerifyIdentityClaim verifies a claim code from the game
func (h *Handler) VerifyIdentityClaim(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Code       string `json:"code"`
		PlayerGUID string `json:"player_guid"`
		PlayerName string `json:"player_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	data, err := h.redis.Get(ctx, "claim:"+req.Code).Bytes()
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, "Invalid or expired code")
		return
	}

	var claim models.IdentityClaim
	json.Unmarshal(data, &claim)

	// Link identity in Postgres
	_, err = h.pg.Exec(ctx, `
		INSERT INTO user_identities (id, user_id, player_guid, player_name, is_primary, verified_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (user_id, player_guid) DO UPDATE SET player_name = $4, verified_at = $6
	`, uuid.New(), claim.UserID, req.PlayerGUID, req.PlayerName, true, time.Now())
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Failed to link identity")
		return
	}

	// Clean up
	h.redis.Del(ctx, "claim:"+req.Code)

	h.jsonResponse(w, http.StatusOK, map[string]string{"status": "claimed"})
}

// ============================================================================
// USER HANDLERS
// User profile management is primarily handled by SMF forum
// These endpoints provide API access to linked identity data
// ============================================================================

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get forum user ID from context (set by AuthMiddleware)
	forumUserID, ok := ctx.Value("forum_user_id").(int)
	if !ok || forumUserID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Query linked identities from PostgreSQL
	var identities []map[string]interface{}
	rows, err := h.pg.Query(ctx, `
		SELECT player_guid, player_name, verified_at, last_seen
		FROM player_identities
		WHERE forum_user_id = $1 AND verified = true
	`, forumUserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var guid, name string
			var verifiedAt, lastSeen *time.Time
			if err := rows.Scan(&guid, &name, &verifiedAt, &lastSeen); err == nil {
				identities = append(identities, map[string]interface{}{
					"guid":        guid,
					"name":        name,
					"verified_at": verifiedAt,
					"last_seen":   lastSeen,
				})
			}
		}
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"forum_user_id": forumUserID,
		"identities":    identities,
	})
}

func (h *Handler) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	// User profile updates are managed via SMF forum
	h.jsonResponse(w, http.StatusNotImplemented, map[string]string{
		"error":   "managed_by_smf",
		"message": "Profile updates are managed through the SMF forum profile page.",
		"url":     "/index.php?action=profile",
	})
}

func (h *Handler) GetUserIdentities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	forumUserID, ok := ctx.Value("forum_user_id").(int)
	if !ok || forumUserID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT player_guid, player_name, verified, verified_at, last_seen, created_at
		FROM player_identities
		WHERE forum_user_id = $1
		ORDER BY verified DESC, last_seen DESC
	`, forumUserID)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var identities []map[string]interface{}
	for rows.Next() {
		var guid, name string
		var verified bool
		var verifiedAt, lastSeen, createdAt *time.Time
		if err := rows.Scan(&guid, &name, &verified, &verifiedAt, &lastSeen, &createdAt); err == nil {
			identities = append(identities, map[string]interface{}{
				"guid":        guid,
				"name":        name,
				"verified":    verified,
				"verified_at": verifiedAt,
				"last_seen":   lastSeen,
				"created_at":  createdAt,
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, identities)
}

func (h *Handler) UnlinkIdentity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	guid := chi.URLParam(r, "guid")

	forumUserID, ok := ctx.Value("forum_user_id").(int)
	if !ok || forumUserID == 0 {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Remove the identity link
	result, err := h.pg.Exec(ctx, `
		DELETE FROM player_identities
		WHERE forum_user_id = $1 AND player_guid = $2
	`, forumUserID, guid)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, "Database error")
		return
	}

	if result.RowsAffected() == 0 {
		h.errorResponse(w, http.StatusNotFound, "Identity not found")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "unlinked",
		"message": "Game identity unlinked successfully",
	})
}

// HELPERS
// ============================================================================

func generateSecureCode(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateUserCode() string {
	// Generate short human-readable code like "MOH-9921"
	bytes := make([]byte, 2)
	rand.Read(bytes)
	return fmt.Sprintf("MOH-%04d", int(bytes[0])<<8|int(bytes[1])%10000)
}
