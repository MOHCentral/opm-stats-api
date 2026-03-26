## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Fast Path For Strings]
**Learning:** `strings.Contains` loops through the string and requires extra logic. Using `strings.IndexByte` directly returns the index, saving an operation in custom parsing loops. By writing the clean prefix in bulk (`sb.WriteString(s[:idx])`), performance for strings with colors at the end or no colors improved (~25% speedup).
**Action:** Use `strings.IndexByte` combined with bulk slice writing when stripping or modifying character patterns in string builders, instead of checking character-by-character from the start.
