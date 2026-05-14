## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-17 - Fast String Concatenation Avoids Allocations in Hot Paths
**Learning:** Using `fmt.Sprintf` for high-frequency string generation (like creating Redis keys in a worker pool or event processor) incurs significant memory allocation overhead due to reflection and interface boxing for variadic arguments. `strconv.Itoa` combined with standard string concatenation (`+`) completely bypasses this, offering lower allocations per operation.
**Action:** When generating strings frequently inside a loop or hot path (especially simple ID-to-string mapping like Redis keys), prefer string concatenation with `strconv` over `fmt.Sprintf` to reduce the GC pressure and increase throughput.
