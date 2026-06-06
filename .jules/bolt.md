## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-18 - Avoid fmt.Sscanf in hot loops for string parsing
**Learning:** `fmt.Sscanf` relies heavily on reflection and dynamic type conversion, making it computationally expensive and memory-allocative compared to manual string manipulation. Micro-benchmarking showed a ~58x speedup (773 ns/op to 13 ns/op) when replacing `fmt.Sscanf` with `strings.IndexByte` and `strconv.Atoi`.
**Action:** When parsing simple, known string formats (like comma-separated Redis values), use `strings.IndexByte`, slicing, and `strconv` instead of `fmt.Sscanf` to eliminate reflection and heap allocations.
