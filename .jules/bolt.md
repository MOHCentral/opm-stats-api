## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Hot Path Allocations - Sprintf and Maps]
**Learning:** High-throughput processing in `AchievementWorker.ProcessEvent` suffered from unnecessary heap allocations. Generating Redis keys via `fmt.Sprintf` allocates memory and uses reflection, while map literals in functions (e.g., `combatMilestones`) were being re-allocated on every call.
**Action:** Replace `fmt.Sprintf` with string concatenation (`strconv.Itoa` + `+`) in hot paths for caching/Redis keys. Extract static map definitions out of functions into package-level variables (`var`) to avoid per-call allocation overhead. Guard logger calls with `Desugar().Core().Enabled` if they contain multiple arguments to avoid argument boxing allocations.
