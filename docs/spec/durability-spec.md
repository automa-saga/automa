# automa Durability Specification

> Status: **proposed** — normative spec, under review.
> Version: **journal schema v1**

This document is the **normative, language-neutral** specification for automa's
crash-recovery durability. It defines the on-disk journal format, the execution
state machine, the persistence ordering, and the resume semantics that every
conformant implementation MUST exhibit.

This spec **extends** the [core spec](core-spec.md). It reuses the core's
definitions of step, lifecycle phases, execution/rollback modes, state bag, and
report tree without redefining them; it adds persistence and resume on top. Where
a term here is undefined, it is defined in the core spec.

The conformance keywords (**MUST**, **SHOULD**, **MAY**, …) are interpreted per
[RFC 2119](https://www.rfc-editor.org/rfc/rfc2119); see [README](README.md).

For the *rationale* behind this design (what it offers, the tradeoffs, the worked
crash-and-resume walkthrough), see [docs/durability.md](../durability.md). Where
that design doc and this spec disagree, **this spec governs behavior**.

---

## 1. Scope

This spec covers crash recovery for **sequential sagas**: an ordered list of
steps executed in order, with compensation (rollback) in reverse order. It
specifies:

- the journal artifact (§3) and its lifecycle,
- the step and workflow state machines (§4),
- the persistence ordering relative to step side effects (§5),
- the resume algorithm (§6),
- the obligations on workflow authors (§7),
- conformance fixtures (§8).

It does **not** cover: durable timers / long sleeps across restarts, distributed
or multi-process execution, signals/queries, or dynamically generated topology.
These are out of scope for journal schema v1.

## 2. Terminology

- **Implementation** — a language binding of automa (Go, Kotlin, Python, Rust, …).
- **Workflow definition / topology** — the ordered list of step IDs, the
  execution mode, and the rollback mode, as constructed in caller code.
- **Run** — a single attempt to execute a workflow definition, identified by a
  workflow ID. A run MAY span multiple processes (an original attempt plus one or
  more resumes).
- **Journal** — the persisted, language-neutral record of a run's progress (§3).
- **Side effect** — any externally observable action a step performs (creating a
  resource, mutating a machine, writing to a remote system).
- **Commit point** — the moment the journal records a step as `completed`.
- **Write-ahead record** — the journal write that marks a step `started` *before*
  its side effect runs.

## 3. The journal

### 3.1 Encoding

- The journal MUST be encoded as **UTF-8 JSON**.
- Object keys are **exactly** as specified in §3.3 (snake_case). Implementations
  MUST NOT rename keys to match a language's idioms in the on-disk form.
- Unknown keys encountered on read MUST be **ignored** (forward compatibility),
  except that an unknown `version` MUST be handled per §3.5.
- Enumerated string values (§4) are **lowercase** and fixed by this spec.

### 3.2 Identity and granularity

- There is **one journal per run**.
- The journal is a **snapshot**: the entire journal is rewritten on each
  transition (it is not an append-only log). Recovery granularity is therefore
  the **step boundary** — an implementation MUST NOT claim to resume into the
  middle of a step's execution.

### 3.3 Schema (journal v1)

```jsonc
{
  "version": 1,                       // integer; schema version (§3.5)
  "workflow_id": "setup_local_dev",   // string; identifies the run's workflow
  "execution_mode": "RollbackOnError",// string; one of the execution modes (§4.3)
  "rollback_mode":  "StopOnError",    // string; one of the rollback modes (§4.3)
  "phase":  "forward",                // string; workflow phase (§4.1)
  "cursor": 1,                        // integer; index of the step currently worked on
  "shared": { /* namespaced bag */ }, // object; the workflow's shared state space:
                                       //   Global + all named rooms (§ core 7.2.1).
                                       //   Excludes per-step Local (captured per step).
  "steps": [                          // array; one entry per step, in topology order
    {
      "id": "create-network",         // string; step ID (MUST match topology)
      "state": "completed",           // string; step state (§4.2)
      "snapshot": { /* state bag */ },// object, OPTIONAL; per-step execution state for rollback
      "report":   { /* report */ }    // object, OPTIONAL; the step's report tree
    }
  ]
}
```

Field requirements:

- `version`, `workflow_id`, `execution_mode`, `rollback_mode`, `phase`,
  `cursor`, and `steps` are **REQUIRED**.
- `steps` MUST contain exactly one entry per step in the workflow definition,
  in topology order. `steps[i].id` MUST equal the i-th step ID of the topology.
- `snapshot` and `report` are **OPTIONAL** per step and MAY be omitted when not
  yet produced (e.g. a `pending` step). `snapshot`, when present, is the state
  captured for use during compensation.
- `shared` MAY be an empty object but the key SHOULD be present. It captures the
  workflow's shared state space (Global + all named rooms, per core spec §7.2.1).
- The serialization of `shared`, `snapshot`, and `report` is governed by their
  own (existing) language-neutral schemas (namespaced state bag, report tree).
  This spec treats them as opaque nested objects and constrains only their
  placement.

### 3.4 Cursor

- `cursor` is the index into `steps` of the step the run is currently working on.
- In `forward` phase, `cursor` is the index of the most recently `started` step.
- In `compensating` phase, `cursor` is the index from which compensation
  proceeds downward (toward 0).
- In `done` phase, `cursor` is unconstrained and MUST be ignored by readers.

### 3.5 Versioning

- An implementation MUST read its own `version` and any lower version it declares
  support for.
- On encountering a `version` it does not support, an implementation MUST fail
  loudly (§6.2) and MUST NOT attempt to resume. Silent restart is forbidden
  because it risks re-executing side effects.

### 3.6 Durable write (atomicity)

A journal write MUST be **atomic** with respect to crashes, including crashes
during the write itself. A reader (including a post-crash reader) MUST observe
either the complete previous journal or the complete new journal, never a torn
or partial file.

The REQUIRED procedure is:

1. Serialize the journal to bytes.
2. Write the bytes to a temporary file in the **same directory** as the target.
3. Flush the temporary file's data to stable storage (`fsync` or the platform
   equivalent) **before** the rename. This is REQUIRED to survive power loss, not
   merely process crash.
4. Atomically rename the temporary file over the target path.

Implementations on POSIX filesystems MUST use `rename(2)` (atomic). Implementations
on platforms without an atomic same-volume rename MUST use the closest equivalent
that preserves the all-or-nothing guarantee.

### 3.7 Identity, lifecycle, and retention

#### 3.7.1 runID identifies one execution

- `workflow_id` identifies a workflow **definition**; a **runID** identifies a
  single **execution** of it.
- The journal path is `<dir>/<workflowID>-<runID>.journal`, where `<dir>` is the
  journal directory configured on the workflow. For v1 the runID is
  **caller-supplied** (engine-generated run IDs are a later addition).
- **To start a fresh execution, use a fresh runID.** Re-running "the same
  workflow" is a new execution and MUST use a new runID, which yields a new
  journal — so a prior run's journal can never be mistaken for the new run.
- **Reusing a runID means "resume that specific execution."** This is the only
  way a prior journal is consulted; it is intentional, never accidental, when the
  runID convention above is followed.

#### 3.7.2 Resume of a terminal journal is a safe no-op

- A resume against a journal already in `phase: done` MUST return the recorded
  final result and MUST execute and compensate **nothing** (§6.5). Therefore a
  stale terminal journal can never cause double-execution; at worst it causes a
  surprising no-op, which the retention policy and the fresh-runID rule prevent.
- Corollary: **the journal is a recovery record, not a result cache.** Callers
  obtain a run's result from that run's return value. An implementation MUST NOT
  require a retained journal in order to surface a completed run's result.

#### 3.7.3 Retention policy (configurable per workflow)

Retention is configured on the workflow. The policy governs what happens to the
journal when a run reaches a **terminal** outcome (completed successfully,
fully compensated, or terminally failed):

| Policy | On terminal outcome |
|--------|---------------------|
| `retain` (**default**) | Keep the journal for **all** terminal outcomes, including success. Provides a full audit trail. The caller prunes explicitly (§3.7.4). |
| `delete_on_success` | Delete the journal on **successful** completion; retain it on failure or rollback (the cases worth inspecting/retrying). |
| `delete_on_done` | Delete the journal on **any** terminal outcome. |

- The default is `retain` — provisioning, migration, and installer tools commonly
  want a durable record of even successful runs.
- Any delete performed by a policy MUST be **crash-safe**: `done` is terminal and
  idempotent, so a crash between the final commit and the delete leaves a valid
  `done` journal that a later resume returns and MAY re-delete. Deletion MUST
  occur only after the `done` state is durably persisted.

#### 3.7.4 Pruning retained journals

- Because the caller owns the journal directory, pruning is **explicit**, not
  background magic. An implementation SHOULD offer a prune utility, e.g.
  `PruneJournals(dir, policy)`, supporting at least:
  - by **age** (older than a given duration), and
  - by **status** (e.g. only `done`, or only successfully-completed).
- A caller MAY equivalently delete journal files directly; each journal is a
  self-contained file.
- Implementations MUST document that journals accumulate under `retain` until
  pruned, so operators size storage accordingly.

## 4. State machine

### 4.1 Workflow phases

```
forward ──▶ done
   │
   └──▶ compensating ──▶ done
```

| Phase | Meaning |
|-------|---------|
| `forward` | Executing steps in topology order. |
| `compensating` | Rolling back completed steps in reverse order. |
| `done` | Terminal. The run is finished (success, fully compensated, or terminally failed per mode). |

- A run starts in `forward`.
- A run enters `compensating` only when a step fails **and** the execution mode is
  `RollbackOnError` (§4.3).
- `done` is terminal; once written, no further step transitions occur for the run.

### 4.2 Step states

```
pending ──▶ started ──▶ completed
                  │
                  └────▶ failed

completed ──▶ compensated      (during compensating phase)
```

| State | Meaning |
|-------|---------|
| `pending` | Not yet started. |
| `started` | Write-ahead record written; side effect MAY or MAY NOT have run. |
| `completed` | Execute succeeded; commit point recorded. |
| `failed` | Execute failed. |
| `compensated` | Rollback for this step completed. |

- A step found in `started` but not `completed` after a crash is the **ambiguous
  case**: its side effect's completion is unknown. It MUST be re-executed on
  resume (§6.3), which is why §7 requires step idempotency.
- A `skipped` outcome (e.g. a step the engine deliberately did not run) MAY be
  represented; if so it MUST be treated as not requiring compensation. (Skip
  semantics are governed by the engine's execution-mode rules and are not
  expanded here.)

### 4.3 Modes

The journal records two modes, both as strings:

- `execution_mode` ∈ { `StopOnError`, `ContinueOnError`, `RollbackOnError` }.
- `rollback_mode` ∈ { `StopOnError`, `ContinueOnError` }.

These values MUST be spelled exactly as above (matching automa's existing
`TypeMode` values). The modes recorded in the journal MUST equal the modes of the
workflow definition supplied at resume (§6.2 topology validation includes mode
agreement).

## 5. Persistence ordering

The following ordering is **normative**. It is what makes recovery decidable.
Per step at index `i` in `forward` phase:

```
F1. steps[i].state = "started"; cursor = i;   PERSIST   ← write-ahead, BEFORE side effect
F2. run the step's prepare (if any)
F3. run the step's execute  (THE SIDE EFFECT happens here)
F4. steps[i].state   = "completed" | "failed"
    steps[i].snapshot = <execution-time state>          (when completed, for rollback)
    steps[i].report   = <step report>
    shared            = <current shared state space: Global + named rooms>
                                               PERSIST   ← commit point, AFTER side effect
F5. on failure with execution_mode = RollbackOnError:
    phase = "compensating"; cursor = i;        PERSIST
```

Per step at index `i` in `compensating` phase (iterating `cursor → 0`):

```
C1. if steps[i].state == "compensated": skip (idempotent resume)
C2. restore the step's snapshot; run the step's rollback (THE COMPENSATING SIDE EFFECT)
C3. steps[i].state = "compensated"; cursor = i;   PERSIST   ← per-compensation commit
```

On completion:

```
D1. phase = "done";                                PERSIST   (then apply retention policy, §3.7.3)
```

Requirements:

- F1 MUST happen, and its PERSIST MUST be durable (§3.6), **before** F3 runs the
  side effect. An implementation MUST NOT run a step's side effect before its
  `started` record is durably written.
- F4's PERSIST MUST happen **after** the side effect returns and **before** the
  next step's F1.
- In `compensating` phase, each step's `compensated` record (C3) MUST be durably
  written before proceeding to the next-lower index, so an interrupted rollback
  resumes without repeating already-compensated steps.

## 6. Resume

### 6.1 Entry

Resume is the public recovery entry point. The caller MUST re-supply the same
workflow definition (the same builder/code that produced the original run); the
implementation rehydrates the journal onto it and continues.

A resume:

1. Loads the journal (§6.2).
2. Validates topology and modes against the supplied definition (§6.2).
3. Rehydrates `shared` (Global + named rooms) onto the workflow.
4. Dispatches on `phase` (§6.3, §6.4, §6.5).

### 6.2 Loading and validation

- **Missing journal** → the implementation MUST treat this as a fresh run: begin
  in `forward` at index 0 and write a new journal. (Resume of a never-started run
  is a normal start.)
- **Corrupt or unreadable journal**, or **unsupported `version`** → the
  implementation MUST fail loudly and MUST NOT resume or restart. Silently
  restarting could re-execute side effects.
- **Topology / mode mismatch** → if the supplied definition's ordered step IDs,
  `execution_mode`, or `rollback_mode` do not equal those recorded in the
  journal, the implementation MUST refuse to resume and report the mismatch. This
  is the single-process analogue of workflow versioning; it is intentionally
  strict. Implementations MAY offer an explicit, opt-in relaxation for additive
  changes (e.g. steps appended after the last completed step), but the default
  MUST be strict refusal.

### 6.3 Forward resume (`phase == "forward"`)

1. Identify the first step not in state `completed` (the lowest index whose state
   is `pending`, `started`, or `failed`).
2. If that step is in state `started` (ambiguous case), it MUST be **re-executed**
   — the side effect's completion before the crash is unknown. Re-execution
   relies on step idempotency (§7).
3. Continue executing forward from that index per §5, honoring `execution_mode`.
4. Steps already in state `completed` MUST NOT be re-executed.

### 6.4 Compensation resume (`phase == "compensating"`)

1. Continue compensating from `cursor` downward toward index 0 per §5 (C1–C3).
2. Steps already in state `compensated` MUST be skipped.
3. Honoring `rollback_mode` governs whether a failed compensation stops the
   rollback or continues to lower indices.

### 6.5 Done (`phase == "done"`)

- The run is terminal. Resume MUST return the recorded final result and MUST NOT
  execute or compensate any step (§3.7.2).
- On reaching `done`, the configured **retention policy** (§3.7.3) is applied:
  under the default `retain`, the journal is kept; under a delete policy it may be
  removed, in which case a subsequent resume of the same runID sees a missing
  journal and treats it as a fresh run (§6.2).

## 7. Workflow-author contract

Durability shifts a bounded, well-defined set of obligations onto the workflow
author. These are **part of the spec** because they are a cross-language promise
to users, identical in every implementation. Implementations MUST document them
prominently.

1. **Steps MUST be idempotent.** A step found `started`-but-not-`completed` is
   re-executed on resume (§6.3). Running a step twice MUST be equivalent to
   running it once.
2. **Compensations MUST be idempotent.** A compensation MAY be retried across a
   crash during the `compensating` phase.
3. **Resume-relevant data MUST live in serialized state.** Anything a step needs
   to resume or to compensate (resource IDs, handles, prior outputs) MUST be
   written to global state or the step's namespaced state (which is persisted),
   not held only in in-memory closures or fields that do not survive the process.
4. **Topology MUST be reconstructible.** The same ordered set of step IDs (and
   the same modes) MUST be produced by the caller's code at resume time. If steps
   are derived from runtime data, that data MUST itself be persisted so the
   topology is deterministic across restarts.

## 8. Conformance

### 8.1 Fixtures

Cross-language agreement is verified by **shared conformance fixtures**: plain
JSON, no language dependency. Fixtures live under `docs/spec/conformance/` and
are of two kinds:

- **Journal fixtures** — a `journal.json` plus assertions about how a conformant
  reader MUST classify it (phase, first-incomplete index, which steps re-run,
  which are skipped). These verify the §6 resume *decision logic* without running
  side effects.
- **Round-trip fixtures** — a `journal.json` that every implementation MUST be
  able to load and re-serialize to a byte-equivalent (modulo insignificant JSON
  whitespace and key-order-independent) journal, verifying schema agreement (§3).

Each implementation MUST include a test harness that loads every fixture and
asserts the expected outcome. Adding a behavior to the spec REQUIRES adding or
updating a fixture.

### 8.2 What conformance does and does not prove

- Fixtures prove **schema agreement** and **resume decision agreement** across
  implementations — the parts that must be identical for a journal to be portable
  and for recovery to be predictable.
- Fixtures do **not** prove that a given workflow's steps are idempotent (§7);
  that is the author's responsibility and is outside what the engine can verify.

### 8.3 Reference implementation

The **Go** implementation is the first conformant implementation. It is a
*reference*, not the definition: a divergence between the Go behavior and this
spec is a bug in the Go implementation, not an amendment to the spec.

## 9. Non-goals (journal schema v1)

Explicitly out of scope; each MAY be specified later as a separate, versioned
addition, but none is required for crash recovery of sequential sagas:

- Durable timers / long sleeps across restarts.
- Distributed or multi-process execution / worker pools.
- Signals, queries, or other interaction with a running workflow.
- Dynamically generated topology not reconstructible from persisted state.
- Append-only log / compaction (the snapshot model is sufficient for the target
  workload).

## 10. Open questions

**Resolved (folded into the spec):**

- **Run-ID namespacing.** *Resolved:* a workflow is made durable by supplying a
  journal **directory**; the engine writes `<dir>/<workflowID>-<runID>.journal`.
  For v1 the `runID` is **caller-supplied** (so concurrent runs of the same
  workflow coexist); engine-generated run IDs are a later addition.
- **Storage backend.** *Resolved for v1:* a single JSON file. An embedded KV
  backend is deferred; the `Resume` API and journal semantics are backend-neutral
  so it can be added later without breaking the contract.
- **State-bag numeric/precision portability.** *Resolved:* governed by the core
  spec's numeric policy (core §7.5) — document the 2^53 boundary, recommend
  string-typed numerics beyond it. The shared state space serialized in the
  journal follows the same rule.

**Still open:**

- **Sub-workflow journal nesting.** A step MAY itself be a workflow. How is a
  nested run's journal represented — inline under the parent step, or as a linked
  child journal? This MUST be pinned before sub-workflow durability is claimed,
  and depends on the core spec's resolved sub-workflow rollback-report shape
  (core §8.2). Tracked in the durability spec issue (#91).
