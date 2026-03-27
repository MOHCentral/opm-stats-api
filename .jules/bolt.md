## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2025-05-27 - [Struct Reuse in Hot Paths]
**Learning:** `convertToClickHouseEvent` was allocating a new `models.ClickHouseEvent` struct (~384 bytes) for every event processed. In a high-throughput worker pool, this created significant GC pressure (3 allocs/op). Changing the function to `fillClickHouseEvent` and reusing a single struct instance per batch loop reduced allocations to 2/op (strings only) and improved speed by ~43% (440ns -> 250ns).
**Action:** In hot loops, prefer passing a pointer to a reusable struct for output filling rather than returning a new pointer/struct, to avoid heap allocations.
