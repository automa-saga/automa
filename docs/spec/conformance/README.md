# Conformance fixtures

Language-neutral fixtures that define correct automa behavior. Every conformant
implementation (Go first, then Kotlin/Python/Rust/…) MUST pass **these same
fixtures, unchanged**. They are the source of truth for cross-language
agreement; the prose specs ([core](../core-spec.md), [durability](../durability-spec.md))
are the rationale.

> **Rule:** adding or changing a specified behavior REQUIRES adding or updating a
> fixture here. A spec change without a fixture change is incomplete.

Fixtures are plain JSON with no language dependency. The Go reference harness
lives in [`conformance_test.go`](../../../conformance_test.go) and runs every
fixture under `go test`.

## Layout

```
docs/spec/conformance/
  behavior/        # workflow execution outcomes (core-spec §4–§6, §8)
  serialization/   # load → re-serialize round-trips (core-spec §7.5, §8)
  journal/         # durability journal + resume (durability-spec §8) — TODO with #86
```

## Behavior fixtures (`behavior/*.json`)

A workflow definition whose steps have **declared outcomes**, plus the expected
observable result. The harness builds the workflow, runs it with no real side
effects, and asserts the execution/rollback order and the report tree.

```json
{
  "name": "stop_on_error_halts",
  "description": "StopOnError stops at the first failure and runs no rollback.",
  "specRefs": ["§5.1"],
  "workflow": {
    "id": "wf",
    "executionMode": "stop",          // stop | continue | rollback
    "rollbackMode": "continue",       // continue | stop
    "steps": [
      { "id": "s1", "execute": "success" },
      { "id": "s2", "execute": "failed" },
      { "id": "s3", "execute": "success" }
    ]
  },
  "expect": {
    "workflowStatus": "failed",        // success | failed | skipped
    "executionOrder": ["s1", "s2"],    // leaf steps that ran Execute, in order
    "rollbackOrder": [],               // leaf steps that ran Rollback, in order
    "steps": {                          // per-step report assertions (subset ok)
      "s1": { "status": "success", "action": "execute" },
      "s2": { "status": "failed",  "action": "execute" }
    }
  }
}
```

Step fields:

- **Leaf step:** `execute` is `success` | `failed` | `skipped`. `rollback` is
  `success` | `failed` (default `success`). `prepare` is `""` (ok) | `failed`;
  a failed Prepare means the step never Executes.
- **Sub-workflow step:** provide a non-empty `steps` array (and optionally its
  own `executionMode` / `rollbackMode`); `prepare`/`execute`/`rollback` are then
  ignored. A workflow is a step (core-spec §6), so nesting is recursive.

Expectations:

- `executionOrder` / `rollbackOrder` list **leaf** step IDs in the order their
  Execute / Rollback ran (a sub-workflow contributes its leaves, flattened).
- `steps[id]` asserts a subset of that step's report: `status`, `action`, and
  `rollbackStatus` (the status of the step's rollback sub-report). Omitted keys
  are not asserted. Steps absent from the map are not asserted, so fixtures need
  not enumerate steps that never produced a report.

## Serialization fixtures (`serialization/*.json`)

A canonical JSON document that every implementation MUST load and re-serialize
to an equivalent document (structural equality; key order and whitespace do not
matter).

```json
{
  "name": "statebag_numeric_boundary",
  "description": "Safe integers round-trip; larger IDs use strings (core-spec §7.5).",
  "specRefs": ["§7.5"],
  "kind": "statebag",
  "json": {
    "local":  {},
    "global": { "safeInt": 9007199254740992, "bigId": "9007199254740993" }
  }
}
```

- `kind` is one of:
  - `statebag` — the namespaced state bag (`local` + `global`, core-spec §7.2).
    Both namespace keys are always present, even when empty.
  - `report` — a report tree (core-spec §8): step and workflow reports, nested
    `steps`, an inline `rollback` sub-report, RFC 3339 millisecond timestamps,
    and error-as-string.
- **Numeric precision (§7.5):** JSON has one number type; a round-trip through
  floating point is exact only up to 2^53. Values beyond that MUST be stored as
  strings to survive losslessly. Fixtures encode both the safe-integer case and
  the string-encoded large-ID case.
- **Timestamps (§8):** serialize as RFC 3339 with a timezone designator;
  trailing-zero fractional seconds are trimmed (e.g. `...00.5Z`, not `...00.500Z`).
