## 2024-05-23 - [Hot Path Map Allocations]
**Learning:** Found that `checkKillAchievements` and `checkHeadshotAchievements` in the worker pool were re-allocating `map[int64]string` on every single event. In a high-throughput event ingestion system, this created significant GC pressure and CPU overhead (~90ns/op vs ~5ns/op).
**Action:** Always inspect hot loops/worker handlers for hidden allocations like map literals or slice creations. Move immutable lookup tables to package-level variables.

## 2024-05-24 - [Regex in Hot Paths]
**Learning:** `regexp.ReplaceAllString` was used for sanitizing player names (stripping color codes) in the ingestion worker. This function is called multiple times per event. Replacing regex with a manual string builder loop reduced execution time from ~1000ns to ~130ns per call (~7x speedup).
**Action:** Avoid regex in hot paths (ingestion workers) for simple string patterns. Use `strings` functions or manual loops with `strings.Builder`.

## 2024-05-24 - Strings vs Bytes Split
**Learning:** `strings.Split(string(body), "\n")` was ~2x faster than `bytes.Split(body, []byte{'\n'})` + `string(line)` conversion loop for legacy URL-encoded parsing. Even though `string(body)` allocates a large string, the subsequent splitting seems optimized. `bytes.Split` requires converting every line to string for `url.ParseQuery`, incurring N allocations/copies which dominated.
**Action:** When working with APIs that require strings (`url.ParseQuery`), prefer working with strings if the input is already loaded, unless you can avoid the conversion entirely (e.g., `json.Unmarshal` takes `[]byte`).

## 2024-05-24 - bytes.ReplaceAll Allocation
**Learning:** `bytes.ReplaceAll(s, old, new)` returns a *copy* of the slice even if no replacements are made (unlike `strings.ReplaceAll` which returns the original string). This means a proactive check `if bytes.Index(s, old) != -1` can save a full allocation of the data size if the pattern is rare.
**Action:** In hot paths with large byte slices, check for existence before replacing if the replacement is expected to be rare.
