## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-19 - [Optimize String Processing with IndexByte]
**Learning:** Using `strings.IndexByte` to locate target characters in a string, and bulk-copying the non-matching prefix (`sb.WriteString(s[:idx])`) before switching to character-by-character iteration avoids unnecessary operations for clean prefixes and completely avoids iteration over character arrays when no target matches exist.
**Action:** Use `strings.IndexByte` or `bytes.IndexByte` when filtering a character from a string to quickly skip over clean prefixes instead of iterating char-by-char or using a generic library function that allocates.
