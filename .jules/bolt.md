## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Memory Allocations in Event Ingestion Hot Paths]
**Learning:** Found that `bytes.ReplaceAll` unconditionally allocates memory even if the search byte is not found. Guarding it with `bytes.IndexByte(body, 0) != -1` avoids allocations for clean payloads. Additionally, `strings.Split` allocates a large slice of strings when parsing newline-delimited legacy payloads in `IngestEvents`. Replacing it with `bufio.Scanner` initialized with a fixed buffer reduces per-event allocations and GC pressure.
**Action:** Guard `bytes.ReplaceAll` with `bytes.IndexByte` in hot paths. When iterating through lines in large strings or byte slices, prefer `bufio.Scanner` over `strings.Split` to avoid massive string slice allocations.
