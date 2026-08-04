# automa Durability Specification

> Status: **accepted** — ratified normative spec for v1; Go is the reference implementation.
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

A node (the top-level journal or any workflow step) has a stable **header**
(identity + configuration) plus three run-state siblings: `cursor` (the
position), `shared` (durable state), and `steps` (structure). Because a workflow
*is* a step, the structure is **recursive**: a step that is itself a workflow
carries its own `cursor`/`shared`/`steps` (§3.8). A leaf (ordinary) step carries
none of the three.

```jsonc
{
  // ── header: stable identity + configuration ──
  "version": 1,                        // integer; schema version (§3.5)
  "workflow_id": "setup_local_dev",    // string; identifies the run's workflow
  "execution_mode": "rollback",        // string; one of the execution modes (§4.3)
  "rollback_mode":  "stop",            // string; one of the rollback modes (§4.3)

  // ── run state ──
  "cursor": {                          // object; the execution position (§3.4)
    "phase": "forward",                //   string; workflow phase (§4.1)
    "index": 1                         //   integer; index into this node's steps
  },
  "shared": { /* namespaced bag */ },  // object; this workflow's shared state:
                                        //   the Global namespace (§ core 7.2).
                                        //   Excludes per-step Local (captured per step).
  "steps": [                           // array; one entry per step, in topology order
    {
      "id": "create-network",          // string; step ID (MUST match topology)
      "state": "completed",            // string; step state (§4.2)
      "snapshot": { /* state bag */ }, // object, OPTIONAL; leaf-step state for rollback
      "report":   { /* report */ }     // object, OPTIONAL; the step's report tree
      // a workflow step instead carries its own "cursor"/"shared"/"steps" (§3.8)
    }
  ]
}
```

Field requirements:

- Header: `version`, `workflow_id`, `execution_mode`, `rollback_mode` are
  **REQUIRED** (at the top level).
- **Workflow-node invariant.** A node is a workflow **iff** it has `steps`.
  Whenever `steps` is present, `cursor` (with both `phase` and `index`) and
  `shared` MUST also be present. A leaf step MUST carry none of `steps`,
  `cursor`, `shared`. (The top-level journal is always a workflow, so it always
  has all three.)
- `steps` MUST contain exactly one entry per step in the workflow definition, in
  topology order. `steps[i].id` MUST equal the i-th step ID of the topology.
- Per step entry: `id` and `state` are **REQUIRED**. `snapshot` and `report` are
  **OPTIONAL** and MAY be omitted when not yet produced (e.g. a `pending` step).
  `snapshot`, when present, is the leaf-step state captured for compensation.
- `shared` MAY be an empty object but the key MUST be present on a workflow node.
  It captures the workflow's shared state — the Global namespace (core spec §7.2).
- The serialization of `shared`, `snapshot`, and `report` is governed by their
  own (existing) language-neutral schemas (namespaced state bag, report tree).
  This spec treats them as opaque nested objects and constrains only their
  placement.

### 3.4 Cursor

`cursor` is the **execution position** of a node: `{ phase, index }`. It belongs
to one node and its `index` addresses **that node's own** `steps` array — never
across levels. Each workflow node (top level or a nested workflow step) has its
own `cursor`; the live position of the run is the **cursor path** obtained by
descending through workflow steps (§3.8, §6).

- `cursor.index` is the index into this node's `steps` of the step it is
  currently working on; `cursor.phase` is that node's phase (§4.1).
- In `forward` phase, `cursor.index` is the index of the most recently `started`
  step.
- In `compensating` phase, `cursor.index` is the index from which compensation
  proceeds downward (toward 0).
- In `done` phase, `cursor.index` is unconstrained and MUST be ignored by readers.

A step's **`state`** (how its parent sees it) is distinct from a workflow step's
own `cursor.phase` (its internal progress); the two are coupled by §3.8 (e.g. a
workflow step is `completed` only once its own node reaches `phase: done`
successfully).

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

#### 3.7.5 Single owner per journal

- A journal MUST have at most **one live owner at a time**: at most one process
  may be executing or resuming a given journal path at any moment. The caller is
  responsible for enforcing this (e.g. a supervisor that never launches a second
  instance for the same runID).
- An implementation is **not required** to lock the journal file or otherwise
  detect concurrent owners. Two owners resuming the same journal concurrently
  races the durable write (§3.6) and defeats the idempotency contract (§7), which
  covers a single sequential retry — not simultaneous re-execution — and MAY
  double-execute every step from the crash point onward. This is undefined
  behavior, not a supported mode.

### 3.8 Sub-workflow nesting (inline, recursive)

A step MAY itself be a workflow (core §6). Because a workflow *is* a step, the
journal is **recursive**: a step that is a workflow carries its own `cursor`,
`shared`, and `steps` — the same run-state shape the top level has (§3.3) — with
`steps` nested directly, mirroring the inline recursion of the core report tree
(core §8.2). There is no separate child-journal file: one top-level runID yields
exactly one journal file that fully describes the whole tree, atomically written
(§3.6).

**A step is a workflow iff its entry has a `steps` array** (the workflow-node
invariant, §3.3): whenever `steps` is present, `cursor` and `shared` are present
too; a leaf step carries none of the three.

Rules:

- A workflow step entry carries `id`, `state`, an optional `report`, and the
  run-state trio `cursor`/`shared`/`steps`. It does **not** carry `version`,
  `workflow_id`, `execution_mode`, or `rollback_mode`: those are **inherited** —
  `workflow_id` equals the step's own `id`, and the modes are the enclosing
  workflow's (core §6.1, parent overrides), so recording them per level would
  only invite inconsistency.
- A workflow step entry **omits** the leaf `snapshot` field: its compensation is
  driven by its sub-steps' own snapshots (inside its nested `steps`), not a
  single snapshot.
- A workflow step's `steps` MUST satisfy §3.3 recursively: exactly one entry per
  sub-step, in topology order, and each MAY itself be a workflow to arbitrary
  depth. The full topology is thus present in the journal recursively.
- Resume (§6) descends into a workflow step's `steps` using the same
  phase-dispatch rules applied at the top level (§6.3–§6.5), driven by that
  node's `cursor`. A workflow step's outward `state` is coupled to its own node:
  `completed` only once its `cursor.phase` reaches `done` successfully, `failed`
  when it terminates in failure, and `compensated` when it is fully compensated.

```jsonc
{
  "version": 1,
  "workflow_id": "setup",
  "execution_mode": "rollback",
  "rollback_mode":  "continue",
  "cursor": { "phase": "forward", "index": 0 },   // working on steps[0] = "db"
  "shared": { /* namespaced bag */ },
  "steps": [
    {
      "id": "db",                                 // workflow step: has "steps"
      "state": "started",                         //   its state as the parent sees it
      "cursor": { "phase": "forward", "index": 1 },
      "shared": { /* deep-cloned from parent at start; then diverges */ },
      "steps": [
        { "id": "create-volume", "state": "completed" },     // leaf: none of the trio
        {
          "id": "start-postgres", "state": "started",        // nested workflow
          "cursor": { "phase": "forward", "index": 1 },
          "shared": { },
          "steps": [
            { "id": "pull-image",    "state": "completed" },
            { "id": "run-container", "state": "started"   }
          ]
        }
      ]
    },
    { "id": "app",        "state": "pending" },   // workflow not started yet;
    { "id": "smoke-test", "state": "pending" }    //   run-state trio added when it starts
  ]
}
```

The live position of the run is the **cursor path** — here
`db → start-postgres → run-container` — found by descending each workflow step's
`steps` and applying that node's `cursor` (§3.4, §6).

> A not-yet-started workflow step (e.g. `app`) is leaf-shaped (no `steps`/
> `cursor`/`shared`) until it is reached; its sub-steps are enumerated when it
> starts. The authoritative topology is reconstructed from the supplied
> definition on resume (§6.2); the journal mirrors progress into it.

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
pending ──▶ started ──▶ completed ──┐
                  │                  ├──▶ compensated   (during compensating phase)
                  └────▶ failed ─────┘
```

| State | Meaning |
|-------|---------|
| `pending` | Not yet started. |
| `started` | Write-ahead record written; side effect MAY or MAY NOT have run. |
| `completed` | Execute succeeded; commit point recorded. |
| `failed` | Execute failed. |
| `compensated` | Rollback for this step completed. |

- Both `completed` and `failed` can transition to `compensated`. When a step
  fails under `RollbackOnError`, the compensating phase begins **at the failed
  step** (§5 F5 sets `cursor.index` to it, and the C-phase runs from there down),
  so the failed step's own rollback runs to clean up any partial work before the
  earlier `completed` steps are compensated in reverse.

- A step found in `started` but not `completed` after a crash is the **ambiguous
  case**: its side effect's completion is unknown. It MUST be re-executed on
  resume (§6.3), which is why §7 requires step idempotency.
- `failed` means **Execute** failed — the side effect ran and returned an error.
  A step that fails **before reaching Execute** (e.g. its per-step preparation or
  a state-setup error) MUST NOT be recorded as `failed`, because on resume a
  `failed` step is treated as executed and therefore eligible for compensation
  (§5.3 / D5). Such a step MUST retain its pre-execute state — `started` if the
  write-ahead record (F1) was written, otherwise `pending` — so resume does not
  compensate a step whose side effect never ran. This matches live execution,
  which excludes pre-execute failures from the executed set.
- A `skipped` outcome (e.g. a step the engine deliberately did not run) MAY be
  represented; if so it MUST be treated as not requiring compensation. (Skip
  semantics are governed by the engine's execution-mode rules and are not
  expanded here.)

### 4.3 Modes

The journal records two modes, both as strings:

- `execution_mode` ∈ { `stop`, `continue`, `rollback` }.
- `rollback_mode` ∈ { `stop`, `continue` }.

These values MUST be spelled exactly as above. They are automa's existing
`TypeMode` JSON encoding — the same short, lowercase strings used by the core
spec and the report/behavior fixtures (`StopOnError` → `stop`,
`ContinueOnError` → `continue`, `RollbackOnError` → `rollback`) — because the
journal reuses `TypeMode` rather than defining a second wire form. The modes
recorded in the journal MUST equal the modes of the workflow definition supplied
at resume (§6.2 topology validation includes mode agreement).

## 5. Persistence ordering

The following ordering is **normative**. It is what makes recovery decidable.
Per step at index `i` in `forward` phase:

```
F1. steps[i].state = "started"; cursor = {phase:"forward", index:i};
                                               PERSIST   ← write-ahead, BEFORE side effect
F2. run the step's prepare (if any)
F3. run the step's execute  (THE SIDE EFFECT happens here)
F4. steps[i].state   = "completed" | "failed"
    steps[i].snapshot = <execution-time state>          (when completed, for rollback)
    steps[i].report   = <step report>
    shared            = <current shared state: Global>
                                               PERSIST   ← commit point, AFTER side effect
F5. on failure with execution_mode = RollbackOnError:
    cursor = {phase:"compensating", index:i};  PERSIST
```

Per step at index `i` in `compensating` phase (iterating `cursor.index → 0`):

```
C1. if steps[i].state == "compensated": skip (idempotent resume)
C2. restore the step's snapshot; run the step's rollback (THE COMPENSATING SIDE EFFECT)
C3. steps[i].state = "compensated"; cursor.index = i;   PERSIST   ← per-compensation commit
```

On completion:

```
D1. cursor.phase = "done";                     PERSIST   (then apply retention policy, §3.7.3)
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
- **PERSIST failure.** The two *write-ahead* PERSISTs, F1 (`started`) and F5
  (entering `compensating`), guard a side effect that has **not yet run**. If
  either fails, the implementation MUST NOT run the side effect (F1) or begin
  compensation (F5): it MUST abort that step (or the rollback) and surface the
  failure, because running an effect with no durable record would strand it beyond
  what resume can observe or compensate. The *commit* PERSISTs — F4
  (`completed`/`failed`), C3 (`compensated`), and D1 (`done`) — record an outcome
  **after** its side effect has already returned; an implementation MAY tolerate
  (log and continue) a failure at these points, since a lost commit record only
  causes resume to re-execute or re-compensate a step, which §7 idempotency already
  requires to be safe.
- **Recursion.** When step `i` is itself a workflow, its F3 (execute) is the
  recursive run of that sub-workflow, which performs its own F1–D1 against its
  own node (`steps[i].cursor`/`shared`/`steps`). The parent's F1/F4 still bracket
  it: the parent records `started` before descending and `completed`/`failed`
  after the sub-workflow reaches its terminal `cursor.phase`. Each PERSIST writes
  the whole journal file atomically (§3.6), so a nested transition and its
  ancestors' bracketing are captured consistently.

## 6. Resume

### 6.1 Entry

Resume is the public recovery entry point. The caller MUST re-supply the same
workflow definition (the same builder/code that produced the original run); the
implementation rehydrates the journal onto it and continues.

A resume:

1. Loads the journal (§6.2).
2. Validates topology and modes against the supplied definition (§6.2).
3. Rehydrates `shared` (Global) onto the workflow.
4. Dispatches on `cursor.phase` (§6.3, §6.4, §6.5), descending recursively into
   workflow steps (§3.8): a workflow step is resumed by dispatching on its own
   node's `cursor.phase` against its own `steps`.

### 6.2 Loading and validation

- **Missing journal** → the implementation MUST treat this as a fresh run: begin
  in `forward` at index 0 and write a new journal. (Resume of a never-started run
  is a normal start.)
- **Topology validation is recursive** → the supplied definition's ordered step
  IDs MUST match the journal's `steps[i].id` at every level, descending into each
  workflow step's own `steps` (§3.8). A structural mismatch at any depth is a
  mismatch (below).
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

### 6.3 Forward resume (`cursor.phase == "forward"`)

1. Identify the first step not in state `completed` (the lowest index whose state
   is `pending`, `started`, or `failed`).
2. If that step is in state `started` (ambiguous case), it MUST be **re-executed**
   — the side effect's completion before the crash is unknown. Re-execution
   relies on step idempotency (§7).
3. Continue executing forward from that index per §5, honoring `execution_mode`.
4. Steps already in state `completed` MUST NOT be re-executed.

### 6.4 Compensation resume (`cursor.phase == "compensating"`)

1. Continue compensating from `cursor.index` downward toward index 0 per §5 (C1–C3).
2. Steps already in state `compensated` MUST be skipped.
3. Honoring `rollback_mode` governs whether a failed compensation stops the
   rollback or continues to lower indices.

### 6.5 Done (`cursor.phase == "done"`)

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
5. **Workflow-level preparation MUST be idempotent.** A forward resume re-runs the
   workflow's own preparation hook (if the implementation exposes one), just as it
   re-runs a `started` step. Any workflow-level preparation MUST therefore be safe
   to run more than once across a crash.

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
- **Sub-workflow journal nesting.** *Resolved:* a nested run's journal is
  represented **inline** under the parent step's entry, recursively (§3.8),
  mirroring the core spec's inline sub-workflow rollback-report shape (core §8.2).
  A linked/separate child-journal file was rejected because it adds a second
  on-disk artifact to version and cross-file resume coordination, for no benefit
  to the target single-process workload.

**Still open:**

- None blocking durability v1.
