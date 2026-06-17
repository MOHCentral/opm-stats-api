## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [IndexByte vs ReplaceAll in Hot Path]
**Learning:** In `internal/handlers/handlers.go`, `bytes.ReplaceAll` was used unconditionally to sanitize null bytes from incoming payloads. `bytes.ReplaceAll` allocates a new byte slice even if the search target isn't found. Guarding it with `bytes.IndexByte` yielded a ~20% performance improvement (52900ns to 44266ns) by preventing an unnecessary allocation for the vast majority of clean payloads.
**Action:** Guard `bytes.ReplaceAll` or `strings.ReplaceAll` with `IndexByte` or `Contains` in high-throughput hot paths when the target byte/string is usually absent.

## 2024-05-25 - [High-Frequency Logging Costs]
**Learning:** Inside `internal/worker/pool.go` and `internal/handlers/handlers.go`, the usage of `logger.Infow` for high-frequency logs like "IngestEvents called" and "Received job" caused high overhead due to string formatting and allocation, even if production logs are set to only emit errors/warnings. Using `logger.Debugw` and guarding it with an explicit level check (`logger.Desugar().Core().Enabled(zap.DebugLevel)`) drops the overhead from ~320ns to ~12ns when debugging is disabled.
**Action:** Use `Debugw` for high-frequency operations, and always wrap debug logging in a level-check in hot loops to eliminate parameter boxing/evaluation costs.
