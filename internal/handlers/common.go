package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type contextKey string

const serverIDKey contextKey = "server_id"

// hashToken creates a SHA256 hash of a token for secure storage lookup
func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// Health check endpoint
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
	})
}

// Ready check endpoint
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check all dependencies
	checks := map[string]bool{
		"postgres":   h.pg.Ping(ctx) == nil,
		"clickhouse": h.ch.Ping(ctx) == nil,
		"redis":      h.redis.Ping(ctx).Err() == nil,
	}

	allHealthy := true
	for _, ok := range checks {
		if !ok {
			allHealthy = false
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if !allHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":      allHealthy,
		"checks":     checks,
		"queueDepth": h.pool.QueueDepth(),
	})
}

// ServerAuthMiddleware validates server tokens
func (h *Handler) ServerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Server-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
		}

		if token == "" {
			// DO NOT call r.FormValue here as it consumes the body
			// Game servers use the header now
			h.errorResponse(w, http.StatusUnauthorized, "Missing server token")
			return
		}

		// Validate token against database - lookup server by token hash
		ctx := r.Context()
		var serverID string
		hashedToken := hashToken(token)
		h.logger.Infow("Auth Debug", "received_token", token, "computed_hash", hashedToken)

		err := h.pg.QueryRow(ctx,
			"SELECT id FROM servers WHERE token = $1 AND is_active = true",
			hashedToken).Scan(&serverID)

		if err != nil {
			h.logger.Errorw("Auth Database Error", "error", err, "hash", hashedToken)
		}

		if err != nil || serverID == "" {
			h.errorResponse(w, http.StatusUnauthorized, "Invalid server token")
			return
		}

		// Add server ID to context for handlers
		ctx = context.WithValue(ctx, serverIDKey, serverID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getUserIDFromContext extracts user ID from request context (currently unused since JWT removal)
func (h *Handler) getUserIDFromContext(ctx context.Context) int {
	return 0
}

func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
