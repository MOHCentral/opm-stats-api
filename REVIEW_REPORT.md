# Code Review Report: OpenMOHAA Stats API

**Reviewer:** Senior Principal Go Engineer & Security Architect
**Date:** 2024-05-22
**Status:** 🔴 CRITICAL ISSUES DETECTED

## I. Critical Issues (Security & Stability)

### 1. Insecure CORS Configuration (High Risk)
**Location:** `cmd/api/main.go:167`
**Issue:** The application allows requests from any origin (`AllowedOrigins: []string{"*"}`) while simultaneously allowing credentials (`AllowCredentials: true`). This configuration is invalid in modern browsers and, if bypassed or misconfigured with specific origins, exposes the API to Cross-Site Request Forgery (CSRF) and data exfiltration.
**Fix:** Explicitly whitelist allowed origins via environment variables.

### 2. SQL Injection Vulnerabilities (High Risk)
**Location:** `internal/logic/query_builder.go` & `internal/handlers/handlers.go`
**Issue:**
*   `BuildStatsQuery` uses `fmt.Sprintf` to construct SQL queries. While currently mitigated by whitelist maps, this pattern is extremely fragile. A future developer adding a key to `allowedDimensions` without realizing it flows into a raw SQL string could introduce a catastrophic SQL injection vulnerability.
*   The `GetLeaderboard` handler also constructs queries using string formatting.
**Fix:** Use a query builder library (e.g., `Masterminds/squirrel`) or strict parameterized queries. Never use `fmt.Sprintf` for SQL construction.

### 3. Unbounded Concurrency & Goroutine Leaks (Stability)
**Location:** `internal/worker/pool.go:287`, `348`
**Issue:**
*   The worker pool spawns a new goroutine for *every* achievement check and side effect (`go p.processBatchSideEffects`, `go p.achievementWorker.ProcessEvent`).
*   Under high load (e.g., 5,000 events/sec), this will spawn tens of thousands of short-lived goroutines, leading to Memory Exhaustion (OOM) and GC thrashing.
**Fix:** Use a fixed-size worker pool for side effects or process them synchronously within the existing worker pipeline.

### 4. Missing Input Validation (Data Integrity)
**Location:** `internal/handlers/handlers.go:210` (`parseFormToEvent`)
**Issue:**
*   The `parseFormToEvent` method silently ignores `strconv` errors (e.g., `timestamp, _ = strconv.ParseFloat...`). If a client sends invalid data, it defaults to zero values (0, 0.0) without returning a 400 Bad Request.
*   No strict validation struct tags (`validate:"required"`) are enforced on the `RawEvent` struct.
**Fix:** Implement `go-playground/validator`. Return `400 Bad Request` on parsing failures.

---

## II. Refactoring Plan (Architecture)

### 1. Dissolve the Monolithic Handler
**Current State:** `handlers.Handler` contains every service and dependency, violating the Single Responsibility Principle.
**Plan:**
*   Refactor `internal/handlers` into domain-specific handlers:
    *   `ingest.Handler`
    *   `player.Handler`
    *   `server.Handler`
    *   `match.Handler`
*   Register these via strict interfaces.

### 2. Introduce Repository Layer
**Current State:** Handlers and Logic layers contain raw SQL queries (`db.Query("SELECT ...")`). This couples business logic to the database implementation and makes testing impossible.
**Plan:**
*   Create `internal/repository` package.
*   Define interfaces: `PlayerRepository`, `MatchRepository`, `EventRepository`.
*   Move all SQL/ClickHouse queries into implementations of these interfaces.
*   Inject repositories into the Service layer.

### 3. Standardize Error Handling
**Current State:** Inconsistent error logging and response formats.
**Plan:**
*   Create a custom `AppError` type with HTTP status codes and user-safe messages.
*   Use middleware to catch errors and format the JSON response consistently.

---

## III. Code-Level Comments

### `cmd/api/main.go`
```diff
- 	// CORS for frontend
- 	r.Use(cors.Handler(cors.Options{
- 		AllowedOrigins:   []string{"*"},
- 		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
- 		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Server-Token"},
- 		ExposedHeaders:   []string{"Link"},
- 		AllowCredentials: true,
- 		MaxAge:           300,
- 	}))
+ 	// CORS for frontend
+ 	r.Use(cors.Handler(cors.Options{
+ 		AllowedOrigins:   config.GetAllowedOrigins(), // Load from ENV
+ 		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
+ 		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Server-Token"},
+ 		ExposedHeaders:   []string{"Link"},
+ 		AllowCredentials: true,
+ 		MaxAge:           300,
+ 	}))
```

### `internal/worker/pool.go`
**Inefficient JSON Marshaling:**
```diff
 func (p *Pool) Enqueue(event *models.RawEvent) bool {
- 	rawJSON, _ := json.Marshal(event)
-
	job := Job{
		Event:     event,
- 		RawJSON:   string(rawJSON),
+       // Defer marshaling until batch processing or ClickHouse insertion
+       // or better: let ClickHouse driver handle struct marshaling if possible,
+       // but since we need the RawJSON column, do it once.
+       // However, doing it in the HTTP path slows down ingestion.
+       // Move to worker goroutine.
		Timestamp: time.Now(),
	}
```

### `internal/handlers/handlers.go`
**Silent Error Swallowing:**
```diff
 func (h *Handler) parseFormToEvent(form url.Values) models.RawEvent {
     // ...
- 	event.Timestamp, _ = strconv.ParseFloat(form.Get("timestamp"), 64)
- 	event.Damage, _ = strconv.Atoi(form.Get("damage"))
+   var err error
+ 	event.Timestamp, err = strconv.ParseFloat(form.Get("timestamp"), 64)
+   if err != nil {
+       // Must return error or handle invalid data!
+       // This requires changing the function signature to return (models.RawEvent, error)
+   }
```

---

## IV. Test Coverage Matrix & Endpoint Validation Plan

All tests must be **Table-Driven** and safe to run with `-race`.

### Group 1: Ingestion Endpoints (`/api/v1/ingest`)
| Endpoint | Method | Test Case | Expected Status | Validation Checks |
| :--- | :--- | :--- | :--- | :--- |
| `/events` | POST | **Happy Path** (Valid JSON Array) | 202 Accepted | Response: `{"status": "accepted"}`, DB: Event stored. |
| `/events` | POST | **Happy Path** (Legacy Newline) | 202 Accepted | Verify parsing of legacy format. |
| `/events` | POST | **Edge Case** (Empty Body) | 400 Bad Request | Error message present. |
| `/events` | POST | **Edge Case** (>1MB Body) | 413 Payload Too Large | Middleware rejection. |
| `/events` | POST | **Error** (Invalid JSON) | 400 Bad Request | JSON syntax error handled. |
| `/events` | POST | **Security** (Missing Token) | 401 Unauthorized | Auth middleware rejects. |

### Group 2: Player Stats (`/api/v1/stats/player/{guid}`)
| Endpoint | Method | Test Case | Expected Status | Validation Checks |
| :--- | :--- | :--- | :--- | :--- |
| `/{guid}` | GET | **Happy Path** (Existing Player) | 200 OK | JSON structure matches `PlayerStatsResponse`. |
| `/{guid}` | GET | **Edge Case** (Non-existent GUID) | 200 OK | Should return empty/zeroed stats, not 404/500 (per business logic). |
| `/{guid}` | GET | **Error** (SQL Injection in GUID) | 200/400 | Verify GUID is sanitized or param query works. |

### Group 3: Leaderboards (`/api/v1/stats/leaderboard`)
| Endpoint | Method | Test Case | Expected Status | Validation Checks |
| :--- | :--- | :--- | :--- | :--- |
| `/` | GET | **Happy Path** (Default) | 200 OK | Returns Top 25 Kills. |
| `/{stat}` | GET | **Edge Case** (Invalid Stat) | 200 OK | Defaults to 'kills' or handles gracefully. |
| `/{stat}` | GET | **Performance** (Offset 10000) | 200 OK | DB should not time out. |

### Group 4: Server Tracking (`/api/v1/servers`)
| Endpoint | Method | Test Case | Expected Status | Validation Checks |
| :--- | :--- | :--- | :--- | :--- |
| `/{id}` | GET | **Happy Path** | 200 OK | Server details returned. |
| `/{id}/live` | GET | **Concurrency** | 200 OK | Race detector check on Redis/Memory access. |

### Global Testing Strategy
1.  **Mocks:**
    *   Mock `pgxpool.Pool` using `pashagolub/pgxmock`.
    *   Mock `clickhouse.Conn` using interface mocking.
    *   Mock `redis.Client` using `go-redis/redismock`.
2.  **Benchmark:**
    *   Benchmark `IngestEvents` to ensure <1ms allocation per event.
    *   Benchmark `GetLeaderboard` for complex aggregations.
