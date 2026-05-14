## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Hidden Allocations in bytes.ReplaceAll]
**Learning:** `bytes.ReplaceAll` unconditionally allocates a new slice even if the search pattern (e.g. null bytes `[]byte{0}`) is not present in the input. In the hot path of event ingestion, this created unnecessary 40KB allocations per request.
**Action:** Always guard `bytes.ReplaceAll` or `strings.ReplaceAll` with `bytes.IndexByte` or `strings.IndexByte` when modifying payloads where the target byte is rarely present (e.g., null byte sanitization). This bypasses the function call and allocation for clean inputs, offering a ~10-12x performance improvement.
