## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2026-04-23 - Optimize hot path formatting with string concatenation
**Learning:** Using `fmt.Sprintf` in hot paths for simple string and integer formatting (e.g. generating cache keys) causes significant interface boxing reflection overhead and heap allocations. Benchmarks show `fmt.Sprintf` is roughly ~3x slower with 3 allocations compared to string concatenation using `+` and `strconv.Itoa`, which is allocation-free or minimal.
**Action:** Replace `fmt.Sprintf` with simple string concatenation and `strconv.Itoa` when constructing dynamic string keys in hot loops (like in `internal/worker/achievements.go` and `internal/worker/pool.go`). Ensure new packages like `strconv` are explicitly imported.
