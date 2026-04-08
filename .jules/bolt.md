## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2025-04-08 - Use uuid.Nil instead of parsing zero UUIDs
**Learning:** `uuid.MustParse("00000000-0000-0000-0000-000000000000")` involves string parsing and allocations which adds overhead (e.g., 50ns/op) in a hot loop, whereas using the built-in `uuid.Nil` avoids the parsing completely and is virtually instant.
**Action:** Always prefer `uuid.Nil` over string parsing when working with empty UUID strings, especially in hot paths like event conversion loops.
