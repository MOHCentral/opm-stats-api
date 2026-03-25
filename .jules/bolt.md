## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-03-25 - Local map initializations in hot paths cause unnecessary heap allocations
**Learning:** Defining local maps containing static data (such as dictionaries, configuration flags, or achievement milestones) inside frequently called functions (like event processors or handlers) forces the Go runtime to allocate memory for the map on the heap on every function call. Although the data is static, the map instance is not.
**Action:** Always extract static mapping data from local variables within functions to package-level global variables. This ensures the map is allocated only once on application startup, reducing unnecessary CPU cycles spent on allocation and minimizing garbage collection overhead during high-throughput operations.
