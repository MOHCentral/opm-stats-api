## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Achievement Map Allocations]
**Learning:** Found multiple map allocations (e.g., `combatMilestones`) inside hot path methods like `checkCombatAchievements` in `internal/worker/achievements.go`. Benchmarking showed that moving these immutable maps to package-level variables reduced execution time by ~50% (from ~200ns to ~95ns per call).
**Action:** Always define immutable lookup tables as package-level variables or consts, especially in high-throughput worker loops.
