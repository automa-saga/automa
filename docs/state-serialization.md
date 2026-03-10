# State Serialization

This document describes how `StateBag` and `NamespacedStateBag` are serialized to JSON/YAML and explains
how the library normalizes decoded values so typed accessors remain usable after marshal/unmarshal round-trips.

## Why this matters

- Workflows often persist or transmit state for logging, debugging, sub-workflow inputs, or persistence between runs.
- JSON/YAML decoders change value shapes (numbers, maps, pointers). The library provides normalization and coercion
  helpers so typed getters keep working after round-trips.

## Summary

- `SyncStateBag` serializes as an object where keys are the string form of `Key` and values are the encoded values.
- `SyncNamespacedStateBag` serializes as an object with top-level fields: `local`, `global`, and `custom`.
- Typed accessors (Int, Float64, Bool, String, etc.) rely on `FromState` normalization and centralized coercion helpers
  to recover expected primitive values after decoding.

## What JSON/YAML encoders produce

- JSON (`encoding/json`):
  - Numbers decode to `float64` by default (unless `json.Number` is used explicitly).
  - Objects decode to `map[string]interface{}`.
  - Arrays decode to `[]interface{}`.
  - Strings, booleans, and `null` behave normally.

- YAML (`gopkg.in/yaml.v3`):
  - YAML v3 may preserve numeric types (int, int64, float64) depending on the input, but some decode paths produce
    `map[interface{}]interface{}`.
  - The library decodes `*yaml.Node` when present to obtain stable native values.

## Normalization performed by `FromState`

`FromState` (and `normalizeFromState`) canonicalizes decoded shapes before typed coercion. Key behaviors:

- Dereferences pointers recursively.
- Decodes `*yaml.Node` into native Go values when present.
- Converts `json.Number` into `int64` or `float64` (or string as fallback).
- Converts YAML `map[interface{}]interface{}` into `map[string]interface{}` (stringifying non-string keys).
- Recursively normalizes slices (`[]interface{}`) and maps (`map[string]interface{}`).

Centralized coercion helpers (`toInt64`, `toFloat64`, `toBool`, `stringify`) convert between numeric shapes,
numeric strings, and boolean strings so typed accessors can return the requested primitive type even after a round-trip.

## Typed accessor semantics

When you call an accessor such as `Int`, `Float64`, `Bool`, or `String` the library:

1. Calls `FromState` / `normalizeFromState` on the stored value.
2. Attempts safe coercions for the requested primitive type:
   - `float64` or `json.Number` → numeric target types (integer conversions truncate toward zero).
   - Numeric strings are parsed and coerced to numeric targets when requested.
   - Boolean strings (`"true"`/`"false"`) are parsed to booleans.
   - `String` returns string values; numeric values are formatted (integers preserve integer formatting).
3. Falls back to an exact type assertion for complex types.

Notes about coercion

- Truncation: converting a float to an integer truncates toward zero (e.g., `3.99 -> 3`, `-1.9 -> -1`).
- Numeric-string coercion: when the stored value is a string and the accessor requests a numeric/bool type, the
  accessor will attempt to parse the string (e.g., `"123"` → `123`).
- For strictness, `FromState` avoids coercing non-string values into strings implicitly; `String` will return only
  native strings or format other primitives when appropriate.

## Practical guidance

- Any Go numeric type is fine to store. However, because JSON round-trips convert numbers to `float64`, do not
  rely on the concrete runtime type after a marshal/unmarshal; instead, use the typed accessors provided by the
  library (`Int`, `Int64`, `Float64`, `Bool`, `String`, etc.). Those accessors call `FromState` and perform the
  necessary normalization and coercion.

- Prefer storing:
  - Primitives: `string`, `bool`, numeric types (note JSON round-trips yield `float64`).
  - Simple slices/maps of primitives: `[]string`, `map[string]int`.
  - Plain structs that implement `json.Marshal`/`json.Unmarshal` or YAML equivalents.
  - Types implementing the project `Clone()` contract when snapshotting is important.

- Avoid storing:
  - Functions, channels, OS handles, `sync` primitives — these do not serialize or clone meaningfully.
  - Pointers to types that aren't encodable unless you provide custom marshalers/wrappers.

- For complex runtime-only values, persist an identifier (string) in the `StateBag` and keep the heavy object in an
  external registry or store.

Snapshot, thread-safety, and marshal behavior

- `SyncStateBag` and `SyncNamespacedStateBag` snapshot their contents under locks to avoid races and provide a
  consistent view for persistence.
- Unmarshal replaces internal backing maps under a write lock to minimize the time the bag is locked.
- State preservation (enabled by default) snapshots step state after each successful step execution so rollback
  handlers receive deterministic snapshots; disable preservation to reduce memory cost when snapshots aren't needed.

## Examples and common pitfalls

- JSON numeric pitfall

  ```go
  state.Set("n", 42)
  b, _ := json.Marshal(state)
  var s SyncStateBag
  json.Unmarshal(b, &s)
  // s.Get("n") -> float64(42.0) not int(42)
  // Use the accessor:
  _ = s.Int("n") // returns 42 because FromState + coercion convert float64 -> int
  ```

- Using normalization/coercion helpers directly

  ```go
  v, _ := s.Get("n")
  v = normalizeFromState(v) // normalize decoded shapes
  if i, ok := toInt64(v); ok {
      n := int(i)
      // use n
  }
  ```

Examples: using the exported normalization/coercion helpers

If you need to operate on decoded values directly (for example in custom unmarshalling or migrations), use the
exported helpers to normalize and coerce values in a stable way.

```go
v, _ := state.Get("someKey")
// Normalize any yaml/json decoding shapes (pointers, yaml.Node, json.Number, maps/slices)
nv := automa.NormalizeValue(v)

// Try numeric coercion
if i, ok := automa.ToInt64(nv); ok {
    fmt.Println("int64:", i)
}

// Or float coercion
if f, ok := automa.ToFloat64(nv); ok {
    fmt.Println("float64:", f)
}

// Or boolean coercion
if b, ok := automa.ToBool(nv); ok {
    fmt.Println("bool:", b)
}

// Convert slices/maps to string forms for simple traversal
if ss, ok := automa.ToStringSlice(nv); ok {
    fmt.Println("string slice:", ss)
}
if sm, ok := automa.ToStringMap(nv); ok {
    fmt.Println("string map:", sm)
}
```

## Testing guidance

- Add unit tests that perform JSON and YAML round-trips of `SyncStateBag` and `SyncNamespacedStateBag` and assert
  typed accessors still return expected values.
- Test `Clone()` semantics for any custom types placed into the `StateBag`.
- Validate behavior when state-preservation is disabled.

## Security considerations

- Do not unmarshal state from untrusted sources directly into live workflow state without validation.
- Validate and sanitize fields after unmarshal, especially when values are used for file paths, commands, or other
  sensitive inputs.

Conclusion

- Serialization of `StateBag` is supported and convenient, but callers must be aware of decoder semantics and the
  potential for shape changes after marshal/unmarshal.
- Use typed accessors or `FromState` helpers — they normalize and coerce common shapes so primitive access remains
  robust after round-trips.
- Prefer simple, cloneable, serializable types or provide custom marshal/clone implementations for complex values.
