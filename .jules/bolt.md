## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Hash object allocations in Hot Paths]
**Learning:** Found that `hashToken` in the handlers was using `sha256.New()` and `h.Write()` to hash tokens, which allocates a dynamic hash object. Switching to `sha256.Sum256([]byte(token))` allocates on the stack instead and reduces heap allocations (from 4 to 3 allocs/op) and improves throughput by ~11%.
**Action:** Use static size functions like `sha256.Sum256` instead of `sha256.New()` where possible to avoid unnecessary heap allocations.
