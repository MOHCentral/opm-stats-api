## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2026-05-28 - [SHA256 Optimization]
**Learning:** Using `sha256.New()` followed by `Write` and `Sum` allocates the hash object on the heap dynamically. In hot paths, it is significantly faster to use the `sha256.Sum256()` function instead which creates an array on the stack, reducing allocations.
**Action:** Always prefer `sha256.Sum256([]byte(data))` over `sha256.New()` when computing a hash of a single byte slice to reduce dynamic allocations on the heap.
