## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [High Frequency Logging in Hot Paths]
**Learning:** Found that `zap.SugaredLogger` calls (e.g., `Infow`) with variadic arguments in high-throughput hot paths (like `IngestEvents` and worker pool event loops) incur significant allocation overhead due to interface boxing and slice allocation, even if the logs are effectively ignored.
**Action:** Guard high-frequency log statements with explicit log level checks (e.g., `logger.Desugar().Core().Enabled(zap.DebugLevel)`) to eliminate allocation overhead when the log level is disabled in production. Replace `Infow` with `Debugw` for per-event logging.

## 2024-05-25 - [fmt.Sprintf Allocation Overhead]
**Learning:** `fmt.Sprintf` causes interface reflection and heap allocation overhead, making it inefficient for string formatting in hot paths, such as Redis key generation in achievements or authentication address construction.
**Action:** Replace `fmt.Sprintf` with string concatenation and `strconv.Itoa` (or `strconv.FormatInt`) in high-frequency functions. This simple change drastically reduces per-operation allocations.
## 2024-05-25 - [Test Breakages and Scope Creep]
**Learning:** Found that running `go test ./internal/...` after a focused performance improvement in handlers and workers exposes multiple pre-existing test failures across unrelated packages (`logic`, `tests` e2e).
**Action:** When implementing performance optimizations, strictly avoid scope creep. Do not attempt to fix unrelated broken tests if the underlying code was not modified by the optimization. Focus only on testing the explicitly modified packages to verify the performance changes haven't introduced *new* regressions.
