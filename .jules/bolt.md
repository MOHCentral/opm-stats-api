## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Bulk String Copying in Hot Paths]
**Learning:** Found that `sanitizeName` in `internal/worker/pool.go` was using a character-by-character loop to sanitize player names. Using `strings.IndexByte` and bulk copying segments of the string with `sb.WriteString` avoids iterating character by character over clean parts of the string, which significantly speeds up strings without or with few color codes.
**Action:** When sanitizing strings in hot paths, avoid iterating character by character. Use `strings.IndexByte` to locate target characters and bulk-copy safe prefixes using `WriteString`.
