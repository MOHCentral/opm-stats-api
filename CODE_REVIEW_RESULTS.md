# Code Review Report: OpenMOHAA Stats API

## I. Critical Issues

### 1. Security: Critical Auth Bypass in Device Auth
**Severity:** Critical
**Location:** `internal/handlers/auth.go`, `InitDeviceAuth`
**Description:** The endpoint accepts a `client_ip` parameter in the request body and adds it to the `trusted_ips` table if `req.ClientIP` is present.
**Risk:** An attacker can whitelist any IP address (including their own) for any `forum_user_id` if they know the ID. This bypasses the IP verification mechanism intended to prevent unauthorized access.
**Remediation:** Remove the `client_ip` field from the request body. Trust only the actual request IP (`r.RemoteAddr` or `X-Forwarded-For` after verification) or require an administrative token for this operation.

### 2. Security: Unauthenticated Token Generation
**Severity:** High
**Location:** `internal/handlers/auth.go`, `InitDeviceAuth`
**Description:** The endpoint appears to be unauthenticated (outside of `users` and `ingest` routes in `main.go`).
**Risk:** Allows attackers to generate infinite tokens for valid users, potentially flooding the database or brute-forcing the token space (though 65k space `MOH-XXXX` is small, rate limiting helps but isn't strictly enforced here).
**Remediation:** Protect this endpoint with user authentication (e.g., existing session cookie or JWT) or strict rate limiting.

### 3. Stability: Panic Risk in Async Worker
**Severity:** High
**Location:** `internal/worker/pool.go`, `processBatchSideEffects`
**Description:** The method spawns a goroutine `go p.processBatchSideEffects(ctx, batchCopy)` but does not include a `recover()` block.
**Risk:** A panic in the Redis pipeline or logic within this goroutine will crash the entire application.
**Remediation:** Wrap the goroutine body in a function with `defer func() { if r := recover(); r != nil { ... } }`.

### 4. Performance: Inefficient Ingestion
**Severity:** Medium
**Location:** `internal/handlers/handlers.go`, `IngestEvents`
**Description:** Reads the entire request body into memory using `io.ReadAll`, even with a 1MB limit.
**Risk:** High throughput of large batches can cause GC pressure.
**Remediation:** Use `json.Decoder` with `Token()` streaming or `bufio.Scanner` to process the stream line-by-line without loading the full body.

## II. Refactoring Plan

### 1. Architecture: Strict Layer Separation
*   **Current:** Handlers contain business logic (e.g., `IngestEvents` parsing logic, direct SQL calls in `GetLeaderboard`).
*   **Target:** Move all database logic to `internal/repository` interfaces. Move all business logic (parsing, calculation) to `internal/service`. Handlers should only map HTTP -> Service -> HTTP.

### 2. Configuration: Validation & Type Safety
*   **Current:** `config.Load()` reads env vars with defaults but no validation.
*   **Target:** Use a library like `spf13/viper` or a custom validator to ensure required config (like `JWT_SECRET` in prod) is present and valid.

### 3. Dependency Injection: Interface Decoupling
*   **Current:** `handlers.Config` uses concrete types (`*pgxpool.Pool`, `*redis.Client`).
*   **Target:** Define `PostgresRepository`, `RedisRepository`, `ClickHouseRepository` interfaces in `internal/logic/interfaces.go` and use those. This enables full mocking.

## III. Code-Level Comments

### `internal/worker/pool.go`

```go
<<<<<<< SEARCH
	// Process side effects in batch (Redis state updates)
	// Must copy batch because the slice is reused in the worker loop
	batchCopy := make([]Job, len(batch))
	copy(batchCopy, batch)
	go p.processBatchSideEffects(ctx, batchCopy)
=======
	// Process side effects in batch (Redis state updates)
	// Must copy batch because the slice is reused in the worker loop
	batchCopy := make([]Job, len(batch))
	copy(batchCopy, batch)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Errorw("Panic in processBatchSideEffects", "error", r)
			}
		}()
		p.processBatchSideEffects(ctx, batchCopy)
	}()
>>>>>>> REPLACE
```

### `internal/handlers/auth.go`

```go
<<<<<<< SEARCH
	// Auto-trust the IP that was used to generate the token
	if req.ClientIP != "" {
		_, err = h.pg.Exec(ctx, `
			INSERT INTO trusted_ips (forum_user_id, ip_address, source, label)
			VALUES ($1, $2::inet, 'website', 'Auto-approved (website)')
=======
	// Auto-trust the IP that was used to generate the token
	// SECURITY: Use actual request IP, never trust body param without validation
	realIP := r.RemoteAddr // Should use X-Forwarded-For if behind proxy
	if realIP != "" {
		_, err = h.pg.Exec(ctx, `
			INSERT INTO trusted_ips (forum_user_id, ip_address, source, label)
			VALUES ($1, $2::inet, 'website', 'Auto-approved (website)')
>>>>>>> REPLACE
```

## IV. Test Coverage Matrix

### Ingestion (`POST /api/v1/ingest/events`)
*   **Happy Path:**
    *   `TestIngest_JSON_Batch`: Send newline-delimited JSON. Expect 202 Accepted. Verify QueueDepth increases.
    *   `TestIngest_UrlEncoded_Batch`: Send newline-delimited URL-encoded strings. Expect 202 Accepted.
*   **Edge Cases:**
    *   `TestIngest_EmptyBody`: Send empty body. Expect 202 (processed 0).
    *   `TestIngest_MaxBody`: Send body > 1MB. Expect 413 Payload Too Large.
    *   `TestIngest_MixedFormats`: Send mixed JSON and URL-encoded lines.
*   **Error States:**
    *   `TestIngest_InvalidAuth`: Send without `Authorization` header. Expect 401.
    *   `TestIngest_MalformedJSON`: Send invalid JSON. Expect valid lines processed, invalid logged/ignored.

### Auth (`POST /api/v1/auth/device`)
*   **Happy Path:**
    *   `TestDeviceAuth_Init`: Valid `forum_user_id`. Expect valid `user_code` and `expires_at`.
*   **Edge Cases:**
    *   `TestDeviceAuth_Regenerate`: Send `regenerate: true`. Verify old tokens revoked.
*   **Error States:**
    *   `TestDeviceAuth_MissingID`: Empty body or missing ID. Expect 400.

### Stats (`GET /api/v1/stats/player/{guid}`)
*   **Happy Path:**
    *   `TestGetPlayer_Exists`: GUID exists. Verify JSON structure matches `models.PlayerStats`.
*   **Edge Cases:**
    *   `TestGetPlayer_NoStats`: GUID has no events. Expect 200 with zeroed stats or 404.
    *   `TestGetPlayer_Injection`: Send GUID `' OR 1=1--`. Verify no SQL injection (parameterized query check).
*   **Error States:**
    *   `TestGetPlayer_DBDown`: Simulate CH down. Expect 500.

### Security Tests
*   `TestAuth_IPWhitelist_Bypass`: Attempt to spoof `client_ip` in `InitDeviceAuth`. Verify actual IP is used (after fix) or flag as vulnerable.
