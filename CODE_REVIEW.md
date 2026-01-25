# Code Review: OpenMOHAA Stats API

**Reviewer:** Senior Principal Go Engineer & Security Architect
**Date:** 2024-05-23
**Scope:** Full codebase analysis (Architecture, Security, Performance, Testing)

---

## I. Critical Issues

### 1. Concurrency Bomb & Resource Exhaustion (High Severity)
**Location:** `internal/worker/pool.go`
**Issue:** The worker pool implementation spawns an unbounded number of goroutines for side-effect processing.
Inside `processBatch`:
```go
// processBatch handles a batch of events
for _, job := range batch {
    // ...
    // Process side effects (Redis state updates)
    go p.processEventSideEffects(ctx, event) // <--- CRITICAL
}
```
**Impact:** If `BatchSize` is 500 and the system is under load (e.g., 20 batches/sec), this spawns 10,000 goroutines/second. This will rapidly exhaust system resources (memory, scheduler thrashing) and crash the runtime or lead to OOM kills.
**Remediation:** Process side effects sequentially within the worker or use a separate, bounded worker pool for side effects.

### 2. Data Loss on Ingestion Failure (High Severity)
**Location:** `internal/worker/pool.go`
**Issue:** If the ClickHouse batch insert fails, the entire batch is discarded with no retry mechanism.
```go
if err := p.processBatch(batch); err != nil {
    p.logger.Errorw("Batch processing failed", ...)
    eventsFailed.Add(float64(len(batch)))
}
// batch is cleared in the caller regardless of error
```
**Impact:** Permanent loss of telemetry data.
**Remediation:** Implement an exponential backoff retry loop for `processBatch`. Only discard data if it is malformed or after max retries.

### 3. Race Condition: Shutdown vs Enqueue (Medium Severity)
**Location:** `internal/worker/pool.go`
**Issue:** `Stop()` closes `p.jobQueue`, but `Enqueue()` might still try to send to it. While `recover()` is used, it is a poor practice for control flow. The `select` block in `Enqueue` has a race where `case p.jobQueue <- job:` might be selected even if context is done, causing a panic on closed channel send.
**Remediation:** Use a `sync.RWMutex` to guard the "stopping" state or rely purely on Context cancellation before closing the channel.

### 4. Security: Wildcard CORS (High Severity)
**Location:** `cmd/api/main.go`
**Issue:**
```go
AllowedOrigins:   []string{"*"},
```
**Impact:** Allows any malicious website to make authenticated requests to the API on behalf of a user if they are logged in (CSRF-like implications depending on auth storage).
**Remediation:** strict allowlist of origins loaded from configuration.

### 5. Security: Input Validation (Medium Severity)
**Location:** `internal/handlers/handlers.go`
**Issue:** Massive "God Object" handler relies on manual parsing. `IngestEvents` manually iterates strings. `strconv.Atoi` errors are often ignored (e.g., `_`).
**Impact:** Invalid data can pollute the database or cause unexpected behavior.
**Remediation:** Use `go-playground/validator` for struct validation. Handle all parsing errors.

### 6. Architecture: Dependency Injection & Globals
**Location:** `internal/handlers/`
**Issue:** `handlers.go` is over 1000 lines long, mixing routing, business logic, and response formatting.
**Impact:** Unmaintainable, untestable code.

---

## II. Refactoring Plan

### Phase 1: Stability & Safety (Immediate)
1.  **Fix Worker Concurrency:**
    *   Remove `go p.processEventSideEffects`. Call it synchronously or push to a secondary bounded queue.
2.  **Reliable Ingestion:**
    *   Add retry logic to `processBatch`.
    *   Add "Dead Letter Queue" (DLQ) for permanently failed events (dump to disk/S3).
3.  **Security Hardening:**
    *   Move `AllowedOrigins` to `config.Config`.
    *   Audit all `scan` errors.

### Phase 2: Architecture Clean-up
1.  **Split Handlers:**
    *   `internal/handlers/ingest.go`
    *   `internal/handlers/stats.go`
    *   `internal/handlers/player.go`
    *   `internal/handlers/server.go`
2.  **Domain Driven Design (Light):**
    *   Move business logic out of `handlers` completely into `internal/service`. Handlers should only parse Request -> Call Service -> Write Response.

### Phase 3: Testing Overhaul
1.  **Unit Tests:** Create `_test.go` files for all services. Mock DB interfaces.
2.  **Validation:** Replace manual parsing in `IngestEvents` with `json.Unmarshal` (already present but mixed with custom parsing) and struct tags.

---

## III. Code-Level Comments

### `internal/worker/pool.go`

```go
<<<<<<< SEARCH
		// Process side effects (Redis state updates)
		go p.processEventSideEffects(ctx, event)
	}

	// Send batch to ClickHouse FIRST
=======
		// Process side effects (Redis state updates)
		// REVIEW: Blocking call here or use a separate buffered channel.
		// Never spawn unbounded goroutines.
		if err := p.processEventSideEffects(ctx, event); err != nil {
            p.logger.Warnw("Side effect failed", "error", err)
        }
	}

	// Send batch to ClickHouse FIRST
>>>>>>> REPLACE
```

### `internal/handlers/handlers.go` - `IngestEvents`

```go
<<<<<<< SEARCH
		// Support both JSON (if line starts with {) and URL-encoded
		if strings.HasPrefix(line, "{") {
=======
		// REVIEW: This manual line splitting and detection is fragile.
        // Suggest using json.Decoder for stream processing or strict format enforcement.
		if strings.HasPrefix(line, "{") {
>>>>>>> REPLACE
```

---

## IV. Test Coverage Matrix

| Endpoint | Path | Unit Tests | Integration Tests | Status |
| :--- | :--- | :---: | :---: | :--- |
| **Ingestion** | | | | |
| Ingest Events | `POST /api/v1/ingest/events` | ❌ | ✅ (`tests/event_integration_test.go`) | **Partial** |
| Match Result | `POST /api/v1/ingest/match-result` | ❌ | ❌ | **Missing** |
| **Stats (Global)** | | | | |
| Global Stats | `GET /api/v1/stats/global` | ❌ | ❌ | **Missing** |
| Leaderboard | `GET /api/v1/stats/leaderboard` | ❌ | ❌ | **Missing** |
| **Player** | | | | |
| Player Stats | `GET /api/v1/stats/player/{guid}` | ❌ | ❌ | **Missing** |
| Deep Stats | `GET /api/v1/stats/player/{guid}/deep` | ❌ | ❌ | **Missing** |
| **Server** | | | | |
| Server Stats | `GET /api/v1/stats/server/{id}` | ❌ | ❌ | **Missing** |
| **Auth** | | | | |
| Device Auth | `POST /api/v1/auth/device` | ❌ | ❌ | **Missing** |

**Summary:** Test coverage is critically low. Only the "happy path" of event ingestion is tested via integration tests. The entire read-path (Analytics/Dashboard) is untested.

---
