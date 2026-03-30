## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.
## 2024-05-24 - Optimize sanitizeName with IndexByte
**Learning:** `strings.IndexByte` provides a much faster fast-path check than `strings.Contains` for single characters and returning the index allows bulk-copying the safe prefix into `strings.Builder`. This skips unnecessary loop iterations, providing a ~30% performance boost for clean strings and ~10% for mixed strings.
**Action:** When filtering strings in hot paths, use `IndexByte` to locate the target and copy the safe prefix as a block (`WriteString(s[:idx])`) before iterating, rather than checking char-by-char from the beginning.
