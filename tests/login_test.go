// Package tests contains integration tests for the MOHAA Stats API
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// API base URL - can be overridden with environment variable
var apiURL = getEnv("API_URL", "http://localhost:8080")

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TokenResponse represents the response from /api/v1/auth/device
type TokenResponse struct {
	UserCode  string    `json:"user_code"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in"`
	IsNew     bool      `json:"is_new"`
}

// VerifyResponse represents the response from /api/v1/auth/verify
type VerifyResponse struct {
	Valid       bool   `json:"valid"`
	ForumUserID int    `json:"forum_user_id"`
	Message     string `json:"message"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}

// TestLoginFlow tests the complete login flow:
// 1. Generate token (SMF profile page)
// 2. Verify token (tracker.scr /login command)
// 3. Check token cannot be reused
// 4. Check invalid tokens are rejected
func TestLoginFlow(t *testing.T) {
	forumUserID := 1
	playerGUID := fmt.Sprintf("test-guid-%d", time.Now().UnixNano())

	// Step 1: Check API health
	t.Run("APIHealth", func(t *testing.T) {
		resp, err := http.Get(apiURL + "/health")
		if err != nil {
			t.Fatalf("Failed to connect to API: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("API health check failed: status %d", resp.StatusCode)
		}
		t.Log("API is healthy")
	})

	// Step 2: Generate a login token
	var token string
	t.Run("GenerateToken", func(t *testing.T) {
		payload := map[string]interface{}{
			"forum_user_id": forumUserID,
			"regenerate":    true,
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/device",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Token generation failed: status %d", resp.StatusCode)
		}

		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if tokenResp.UserCode == "" {
			t.Fatal("Token is empty")
		}

		token = tokenResp.UserCode
		t.Logf("Generated token: %s (expires in %ds)", token, tokenResp.ExpiresIn)
	})

	// Step 3: Verify the token (simulates tracker.scr /login command)
	t.Run("VerifyToken", func(t *testing.T) {
		if token == "" {
			t.Skip("No token available")
		}

		payload := map[string]interface{}{
			"token":          token,
			"player_guid":    playerGUID,
			"server_name":    "Test Server",
			"server_address": "127.0.0.1:12203",
			"player_ip":      "192.168.1.100",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Failed to verify token: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			var errResp ErrorResponse
			json.NewDecoder(resp.Body).Decode(&errResp)
			t.Fatalf("Token verification failed: status %d, error: %s", resp.StatusCode, errResp.Error)
		}

		var verifyResp VerifyResponse
		if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !verifyResp.Valid {
			t.Fatal("Token should be valid")
		}

		if verifyResp.ForumUserID != forumUserID {
			t.Fatalf("Forum user ID mismatch: got %d, want %d", verifyResp.ForumUserID, forumUserID)
		}

		t.Logf("Token verified, linked to forum user %d", verifyResp.ForumUserID)
	})

	// Step 4: Verify token reuse is ALLOWED (User requested 'Never Expire')
	t.Run("TokenReuseAllowed", func(t *testing.T) {
		if token == "" {
			t.Skip("No token available")
		}

		payload := map[string]interface{}{
			"token":          token,
			"player_guid":    playerGUID,
			"server_name":    "Test Server",
			"server_address": "127.0.0.1:12203",
			"player_ip":      "192.168.1.100",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK for reusable token, got %d", resp.StatusCode)
		}

		t.Log("Token reuse correctly allowed")
	})

	// Step 5: Test invalid token
	t.Run("InvalidTokenRejected", func(t *testing.T) {
		payload := map[string]interface{}{
			"token":          "INVALID-TOKEN-12345",
			"player_guid":    playerGUID,
			"server_name":    "Test Server",
			"server_address": "127.0.0.1:12203",
			"player_ip":      "192.168.1.100",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Expected 401 Unauthorized for invalid token, got %d", resp.StatusCode)
		}

		t.Log("Invalid token correctly rejected")
	})

	// Step 6: Test login with Trusted IP (No token required)
	t.Run("LoginWithTrustedIP", func(t *testing.T) {
		// Uses the same IP "192.168.1.100" which should now be trusted from Step 3
		payload := map[string]interface{}{
			"token":          "", // Empty token
			"player_guid":    playerGUID,
			"server_name":    "Test Server",
			"server_address": "127.0.0.1:12203",
			"player_ip":      "192.168.1.100",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK for trusted IP login, got %d", resp.StatusCode)
		}

		var verifyResp VerifyResponse
		if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !verifyResp.Valid {
			t.Fatal("Trusted IP login should be valid")
		}

		if verifyResp.ForumUserID != forumUserID {
			t.Fatalf("Forum user ID mismatch: got %d, want %d", verifyResp.ForumUserID, forumUserID)
		}

		t.Log("Trusted IP login successful")
	})
}

// TestTokenGeneration tests various token generation scenarios
func TestTokenGeneration(t *testing.T) {
	t.Run("RequiresForumUserID", func(t *testing.T) {
		payload := map[string]interface{}{
			"regenerate": true,
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/device",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request, got %d", resp.StatusCode)
		}

		t.Log("Missing forum_user_id correctly rejected")
	})

	t.Run("RegenerateRevokesOldToken", func(t *testing.T) {
		forumUserID := 999 // Use a different user to avoid conflicts

		// Generate first token
		payload := map[string]interface{}{
			"forum_user_id": forumUserID,
			"regenerate":    false,
		}
		body, _ := json.Marshal(payload)

		resp1, err := http.Post(apiURL+"/api/v1/auth/device", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to generate first token: %v", err)
		}
		defer resp1.Body.Close()

		var token1 TokenResponse
		json.NewDecoder(resp1.Body).Decode(&token1)

		// Generate second token with regenerate=true
		payload["regenerate"] = true
		body, _ = json.Marshal(payload)

		resp2, err := http.Post(apiURL+"/api/v1/auth/device", "application/json", bytes.NewBuffer(body))
		if err != nil {
			t.Fatalf("Failed to regenerate token: %v", err)
		}
		defer resp2.Body.Close()

		var token2 TokenResponse
		json.NewDecoder(resp2.Body).Decode(&token2)

		if token1.UserCode == token2.UserCode {
			t.Fatal("Regenerated token should be different from original")
		}

		// Try to use the old token - should fail (revoked)
		verifyPayload := map[string]interface{}{
			"token":          token1.UserCode,
			"player_guid":    "test-guid",
			"server_name":    "Test Server",
			"server_address": "127.0.0.1:12203",
			"player_ip":      "192.168.1.100",
		}
		verifyBody, _ := json.Marshal(verifyPayload)

		resp3, err := http.Post(apiURL+"/api/v1/auth/verify", "application/json", bytes.NewBuffer(verifyBody))
		if err != nil {
			t.Fatalf("Verify request failed: %v", err)
		}
		defer resp3.Body.Close()

		if resp3.StatusCode != http.StatusUnauthorized {
			t.Fatalf("Old token should be revoked, expected 401, got %d", resp3.StatusCode)
		}

		t.Logf("Old token %s correctly revoked after regenerating to %s", token1.UserCode, token2.UserCode)
	})
}

// TestVerifyTokenValidation tests input validation for the verify endpoint
func TestVerifyTokenValidation(t *testing.T) {
	t.Run("RequiresToken", func(t *testing.T) {
		payload := map[string]interface{}{
			"player_guid": "test-guid",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request for missing token, got %d", resp.StatusCode)
		}

		t.Log("Missing token correctly rejected")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		resp, err := http.Post(
			apiURL+"/api/v1/auth/verify",
			"application/json",
			bytes.NewBufferString("not valid json"),
		)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request for invalid JSON, got %d", resp.StatusCode)
		}

		t.Log("Invalid JSON correctly rejected")
	})
}
