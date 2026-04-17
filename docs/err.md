### Error record creation logic (`CreateErrorRecords`)

1. Input:
   - `data []types.ErrorData`
   - `fingerprint string`

2. For each error item `d` in `data`:
   - Build Redis key as:
     `ERRORS:<fingerprint>:<d.Message>`
   - Marshal `d` to JSON.
   - If marshaling succeeds:
     - Attempt to write to Redis with `redis.RC.Set(key, string(value))`.
     - Ignore Redis write failure intentionally (`_ = ...`), since this step is non-critical.

3. Always return `nil`.

**Behavior notes:**
- Redis persistence here is best-effort only.
- JSON marshal errors skip that individual record silently.
- Redis key includes the raw message text, so identical messages under the same fingerprint map to the same key.
