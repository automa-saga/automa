# State Numeric Boundaries

This document explains how `automa` safely converts decoded numeric values into integer types.

It is written for readers who are new to engineering and new to floating-point numbers.

## Why this matters

Values stored in a `StateBag` may come from:

- normal Go code
- JSON decoding
- YAML decoding
- user input stored as strings

After decoding, numbers are not always stored in the type you expect.

For example:

- JSON numbers usually become `float64`
- YAML numbers may become `int`, `int64`, or `float64`
- numeric strings such as `"123"` may need parsing

That means `automa` must convert values carefully when code asks for:

- `Int()`
- `Int64()`
- `Uint64()`
- `FromState[int64](...)`
- `FromState[uint64](...)`

If those conversions are done carelessly, the code can silently overflow or return the wrong number.

## The simple problem

Suppose you have a decimal value like this:

```text
3.9
```

If code asks for an integer, what should happen?

`automa` uses **truncation toward zero**.

So:

- `3.9 -> 3`
- `-3.9 -> -3`

That rule is simple.

The harder problem is this:

```text
What if the number is too large to fit in int64 or uint64?
```

That is what the boundary handling code protects against.

## Integer ranges

### `int64`

An `int64` can store values from:

```text
-9223372036854775808   to   9223372036854775807
```

That is:

- minimum = `-2^63`
- maximum = `2^63 - 1`

### `uint64`

A `uint64` can store values from:

```text
0   to   18446744073709551615
```

That is:

- minimum = `0`
- maximum = `2^64 - 1`

## Why `float64` is tricky

A `float64` can represent very large numbers, but not every integer exactly.

For small values, this is not a problem.

For very large values, it is a big problem.

That means these two numbers may not behave the same way:

- the exact integer value in math
- the nearest value that `float64` can actually store

So before converting a `float64` to `int64` or `uint64`, `automa` checks:

1. Is the value inside the legal range?
2. Is the value exact enough to convert safely?

## The boundary constants

In `state.go`, `automa` uses these constants:

```go
const (
    int64MinAsFloat64    = -9223372036854775808.0 // -(1 << 63), exact
    int64MaxExclFloat64  = 9223372036854775808.0  // (1 << 63), exclusive upper bound
    uint64MaxExclFloat64 = 18446744073709551616.0 // (1 << 64), exact exclusive upper bound for uint64
)
```

Let us explain each one.

### `int64MinAsFloat64`

```text
-9223372036854775808.0
```

This is `-2^63`.

It is the **smallest valid `int64` value**.

It is used as an **inclusive lower bound**.

That means:

- `value < int64MinAsFloat64` -> reject
- `value == int64MinAsFloat64` -> allow

### `int64MaxExclFloat64`

```text
9223372036854775808.0
```

This is `2^63`.

It is **not** a valid `int64` value.

The maximum valid `int64` is actually:

```text
2^63 - 1
```

So this constant is used as an **exclusive upper bound**.

That means:

- `value < int64MaxExclFloat64` may be valid
- `value >= int64MaxExclFloat64` must be rejected

Using an exclusive upper bound is safer than trying to compare with `2^63 - 1` through floating-point math.

### `uint64MaxExclFloat64`

```text
18446744073709551616.0
```

This is `2^64`.

Again, this is **not** a valid `uint64` value.

The maximum valid `uint64` is:

```text
2^64 - 1
```

So this is also used as an **exclusive upper bound**.

That means:

- `value < uint64MaxExclFloat64` may be valid
- `value >= uint64MaxExclFloat64` must be rejected

## Why exclusive upper bounds are helpful

Instead of saying:

- valid `int64` values go up to `2^63 - 1`
- valid `uint64` values go up to `2^64 - 1`

The code uses:

- valid `int64` values are in `[-2^63, 2^63)`
- valid `uint64` values are in `[0, 2^64)`

This means:

- lower bound is included
- upper bound is not included

This style avoids off-by-one mistakes and works well with floating-point checks.

## What `toInt64` does

The helper `toInt64` accepts many input types:

- integer types
- unsigned integer types
- `float32`
- `float64`
- numeric strings
- `json.Number`

For floating-point values, it does this:

1. truncate toward zero with `math.Trunc`
2. check the truncated value is inside the safe `int64` range
3. only then convert it to `int64`

In plain language:

- if it is too small, reject it
- if it is too large, reject it
- if it fits, convert it

### Examples

Accepted:

```text
42.9   -> 42
-42.9  -> -42
```

Rejected:

```text
9223372036854775808.0   // 2^63, too large for int64
-9223372036854775809.0  // smaller than -2^63
1e30                    // much too large
```

## What `toUint64Safe` does

The helper `toUint64Safe` converts values to `uint64` safely.

It follows these rules:

1. the value must not be negative
2. the value must be a whole number
3. the value must be below `2^64`
4. for float inputs, the conversion must round-trip exactly

That last point is very important.

The code does this kind of check:

```go
u := uint64(f)
if float64(u) != f {
    return 0, false
}
```

This means:

- convert the float to `uint64`
- convert it back to `float64`
- compare with the original

If the number changes, then the original float was not exact enough to trust.

### Examples

Accepted:

```text
0
255
9223372036854775808.0   // 2^63, valid uint64
18446744073709551615    // max uint64, valid from string
```

Rejected:

```text
-1
18446744073709551616.0  // 2^64, too large
300.0 for uint8         // too large for 8-bit unsigned integer
```

## Why we do not just cast directly

A direct conversion like this is dangerous:

```go
return int64(f)
```

or

```go
return uint64(f)
```

If `f` is too large, too small, or not exact, the result may be wrong.

The checks in `automa` exist to prevent that kind of silent bug.

## Real examples from the codebase

These behaviors are covered by tests in `state_coerce_test.go`.

Examples tested there include:

- truncation toward zero for float-to-int conversion
- rejection of values above `2^63` for `int64`
- rejection of values above or equal to `2^64` for `uint64`
- safe handling of `json.Number`
- safe handling of numeric strings

## Mental model for beginners

Think of an integer type as a box with fixed capacity.

- `int64` box: from `-2^63` up to `2^63 - 1`
- `uint64` box: from `0` up to `2^64 - 1`

A float is like a measuring tool that can become blurry for very large values.

So before putting a float into the box, `automa` asks:

1. Does the number fit in the box?
2. Is the measurement exact enough?

Only if both answers are yes does the conversion happen.

## Summary

The boundary logic is there to make conversions safe.

The key ideas are:

- use truncation toward zero for float-to-int conversions
- use exact lower and upper boundary constants
- use exclusive upper bounds for `int64` and `uint64`
- reject values that are out of range
- reject float values that are not exact enough to convert safely

This keeps `FromState`, `Int()`, `Int64()`, and `Uint64()` reliable even when values come from JSON, YAML, or user input.

## See also

- [State Serialization](state-serialization.md)
- `state.go` (`toInt64`, `toUint64Safe`, `FromState`)
- `state_coerce_test.go`

