## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2025-01-20 - [Hot Path Map Allocations]
**Learning:** In `internal/worker/achievements.go`, several achievement checking functions (`checkCombatAchievements`, `checkStreak`, `checkHeadshotAchievements`, `checkMovementAchievements`, `checkVehicleAchievements`, `checkSurvivalAchievements`, `checkObjectiveAchievements`, `checkTeamplayAchievements`, `checkMultikillAchievement`) allocated map literals on every single execution. Microbenchmarks showed this `map[string]int` creation overhead to be ~130ns/op compared to ~0.4ns/op when reading from a global map. Extracting these into package-level variables eliminates this allocation overhead which is significant since these functions are called for nearly every event in the high-throughput ingestion pipeline.
**Action:** Extract map literals from hot loops into package-level variables to prevent unnecessary GC pressure and CPU overhead. Ensure tests are updated if test files also need to match updated code.
