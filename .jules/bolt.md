## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-30 - Optimize fmt.Sprintf and logger allocations in hot paths
**Learning:** Using `fmt.Sprintf` and unconditional logger calls (or dynamically calling `Desugar()` on a `SugaredLogger`) in high-frequency event ingestion hot paths creates significant GC pressure and allocations due to interface reflection and new struct instantiations.
**Action:** Replace `fmt.Sprintf` with string concatenation and `strconv.Itoa` for dynamic key generation. To prevent logging allocations, store an un-sugared `zap.Logger` in the handler/worker struct and explicitly guard log calls using `logger.Core().Enabled(zapcore.InfoLevel)`.
