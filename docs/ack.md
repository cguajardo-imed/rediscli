### Acknowledge record creation logic (`CreateAcknowledgeRecords`)

#### Function signature
- `CreateAcknowledgeRecords(data types.AcknowledgeRequest, fingerprint string) error`

#### Input structures (as used in this function)

1. `types.AcknowledgeRequest`
   - `Action` (used to generate key/value)
   - `Uuids` (iterated collection; each item must expose `MakeKey(action, fingerprint)` and `MakeValue(action, fingerprint)`)

2. `fingerprint string`
   - Correlation component passed into key/value generation.

#### Key structure
For each UUID item:

- Key is produced by:
  - `key := uuid.MakeKey(data.Action, fingerprint)`

So the effective key format is **defined by `MakeKey` implementation** on the UUID item type, with inputs:
- `action = data.Action`
- `fingerprint = fingerprint`

#### Value/data structure
For each UUID item:

- Value is produced by:
  - `value := uuid.MakeValue(data.Action, fingerprint)`

So the stored ACK payload/data format is **defined by `MakeValue` implementation** on the UUID item type, with inputs:
- `action = data.Action`
- `fingerprint = fingerprint`

#### Processing flow
1. Initialize `errs` as `[]string`.
2. Loop over `data.Uuids`.
3. Build `key` via `MakeKey(...)`.
4. Build `value` via `MakeValue(...)`.
5. Write to Redis: `redis.RC.Set(key, value)`.
6. If write fails, append `err.Error()` to `errs`.
7. After loop:
   - if `errs` not empty, return combined error: `errors.New(strings.Join(errs, ", "))`
   - else return `nil`

#### Behavior notes
- Attempts writes for all UUID entries (no early exit).
- Aggregates all Redis write failures into one returned error.
- This function does not define concrete key/value schemas directly; those are encapsulated in `MakeKey` and `MakeValue`.
