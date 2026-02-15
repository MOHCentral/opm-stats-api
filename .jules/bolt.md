## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.
## 2026-02-15 - [String Sanitization Optimization]
**Learning:**  was re-scanning strings and allocating  even for simple prefix copies. By using  to find the first dirty character and  to block-copy the clean prefix, we reduced execution time for 'Late Dirty' strings (common in chat) by ~27%.
**Action:** For string manipulation hot paths, always look for opportunities to block-copy 'clean' sections using  (memcpy) instead of byte-by-byte loops.
## 2026-02-15 - [String Sanitization Optimization]
**Learning:** `sanitizeName` was re-scanning strings and allocating `strings.Builder` even for simple prefix copies. By using `strings.IndexByte` to find the first dirty character and `sb.WriteString` to block-copy the clean prefix, we reduced execution time for 'Late Dirty' strings (common in chat) by ~27%.
**Action:** For string manipulation hot paths, always look for opportunities to block-copy 'clean' sections using `WriteString` (memcpy) instead of byte-by-byte loops.
