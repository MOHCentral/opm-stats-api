## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2026-04-04 - [Worker Hot Path Allocations]
**Learning:** Found that `AchievementWorker.ProcessEvent` re-allocated `map[string]int` (and similar maps for movement/vehicle/survival milestones) on every single event, resulting in significant GC pressure (~36 allocs/op and ~500 B/op). In addition, using `fmt.Sprintf` for Redis keys and unguarded `Infow` calls generated unnecessary allocations for events.
**Action:** Extract immutable map definitions into package-level variables. Replace `fmt.Sprintf` with string concatenation in hot paths. Guard log statements with explicit level checks (`w.logger.Desugar().Core().Enabled(zap.DebugLevel)`). This drops allocations from 36 allocs/op down to 17 allocs/op.
