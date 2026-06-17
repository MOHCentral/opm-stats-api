## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-25 - [Unconditional Byte Replacement Allocations]
**Learning:** `bytes.ReplaceAll(body, []byte{0}, []byte{})` unconditionally allocates a new byte slice in Go, even if the search target is not present in the payload. In hot paths (like event ingestion where 99% of requests don't have null bytes), this creates a massive unnecessary allocation proportional to the body size.
**Action:** Guard string/byte replacements with `bytes.IndexByte` (or `strings.IndexByte`) checks to skip the function entirely if the target character isn't present, achieving a ~10x speedup for clean payloads.
