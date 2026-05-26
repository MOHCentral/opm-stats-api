## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-26 - [fmt.Sprintf in Hot Paths]
**Learning:** Using `fmt.Sprintf` for Redis key generation in high-frequency event ingestion workers (`achievements.go`) causes unnecessary interface reflection and heap allocations (~3 allocs, ~280 ns/op).
**Action:** Replace `fmt.Sprintf` with string concatenation and `strconv.Itoa` for simple strings and integers in hot paths. This reduces allocations to ~0-1 and cuts execution time by ~70-90%.
