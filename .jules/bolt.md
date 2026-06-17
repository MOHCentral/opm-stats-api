## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2025-02-12 - Optimize String Filtering Hot Paths
**Learning:** Micro-benchmarking string sanitization (`sanitizeName`) reveals that guarding string replacement loops with `strings.IndexByte` and using `strings.Builder.WriteString` to bulk-copy clean prefixes is significantly faster than character-by-character iteration. This prevents allocations and iterative loops when the string is clean and improves copy speed when dirty.
**Action:** When filtering strings in Go hot paths, always use `strings.IndexByte` (or `bytes.IndexByte`) to quickly find the start of the required modification, and copy string prefixes in bulk via slicing instead of piecemeal `sb.WriteByte` loops.
