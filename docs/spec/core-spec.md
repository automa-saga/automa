# automa Core Specification

> Status: **proposed** — normative spec, under review.
> Version: **core model v1**

This document is the **normative, language-neutral** specification for automa's
core saga model: the step lifecycle, the workflow execution loop, execution and
rollback modes, composability, the state model, and the report tree. It is the
foundation every conformant implementation (Go first, then others) MUST satisfy.
The [durability spec](durability-spec.md) is an **extension** layered on top of
this core.

Conformance keywords (**MUST**, **SHOULD**, **MAY**, …) are interpreted per
[RFC 2119](https://www.rfc-editor.org/rfc/rfc2119); see [README](README.md).

This spec is **derived from** the Go reference implementation but is written to
be correct in its own right. Where it intentionally departs from the current Go
behavior, the divergence is called out inline as:

> **⚠ Spec decision (differs from current Go implementation).** … The Go
> implementation is to be adapted to match this spec.

A consolidated list of such decisions is in §11.

---

## 1. Scope

This spec covers the **single-process, sequential saga** model:

- Steps and their lifecycle (§3).
- Workflows: an ordered list of steps, executed in order, compensated in reverse
  (§4, §5).
- Execution modes and rollback modes (§5).
- Composability: a workflow is itself a step (§6).
- The state model and its serialization contract (§7).
- The report tree and its serialization contract (§8).
- Registry of step builders (§9).

It does **not** cover (each MAY be a separate, versioned spec later): durability
and resume (see [durability-spec.md](durability-spec.md)), timeouts, retries,
backoff, parallel/concurrent step execution, distributed execution, signals.

## 2. Terminology

- **Implementation** — a language binding of automa.
- **Step** — the smallest unit of work; has a unique ID and up to three lifecycle
  phases (§3).
- **Workflow** — a step that owns an ordered list of child steps and drives their
  execution (§4). A workflow IS a step (§6).
- **Topology** — the ordered list of a workflow's child step IDs, plus its
  execution mode and rollback mode.
- **Report** — the structured, serializable record of an outcome (§8).
- **State bag** — a key-value store; **namespaced state bag** — the partitioned
  per-step view of state (§7).
- **Compensation / rollback** — undoing a completed step's work.

## 3. Step

### 3.1 Identity

- Every step MUST have a non-empty, unique **ID** within its owning workflow.
- An implementation MUST reject (at build/validation time) a workflow containing
  a step with an empty ID or a duplicate ID.

### 3.2 Lifecycle phases

A step has three phases. Each is OPTIONAL to define; an undefined phase has the
default behavior below.

| Phase | Purpose | If not defined |
|-------|---------|----------------|
| **Prepare** | Enrich context, validate preconditions, pre-populate local state. Runs before Execute. | No-op; context passes through unchanged. |
| **Execute** | Perform the step's primary work. Produce a report. | **Not allowed** — a step MUST define Execute (§3.2.1). |
| **Rollback** | Undo Execute's work (compensation). | No-op; produces a **skipped** outcome. |

#### 3.2.1 Execute is required

- A step **MUST** define an Execute function. A step without Execute is invalid
  and MUST be rejected at build/validation time (alongside the empty/duplicate-ID
  checks of §3.1). This matches the principle that every step is a unit of *work*;
  no-op grouping is expressed with a sub-workflow (§6), not an empty step.
- The `skipped` status (§3.4) therefore arises only when a step's own Execute
  logic decides no work is needed (e.g. "already provisioned"), or when an
  undefined Rollback runs — never from a missing Execute.

### 3.3 Phase contract

- **Prepare** receives an execution context and returns a (possibly enriched)
  context plus an optional error. A non-nil error MUST abort the step and be
  treated by the workflow as a **step failure** (§5).
  - The context returned by Prepare MUST be the context passed to that step's
    Execute (and, where applicable, Rollback).
- **Execute** receives the prepared context and MUST produce a non-nil report.
  - If an implementation's Execute can return "no report," the engine MUST
    synthesize a **failure** report rather than treat it as success.
- **Rollback** receives the context and MUST produce a non-nil report; a missing
  report MUST be synthesized as a **failure**.
- **Rollback MUST be idempotent** — it MAY be invoked more than once for the same
  step (e.g. a manual rollback after an automatic one, or a resumed compensation
  under the durability extension).

### 3.4 Step outcome (status)

Every phase produces a report with exactly one **status**:

| Status | Meaning |
|--------|---------|
| `success` | The phase completed without error. |
| `failed` | The phase encountered an error. |
| `skipped` | The phase performed no work (e.g. no Execute defined, or the step determined the work was already done). |

- A report is **failed** if its status is `failed` **or** it carries a non-nil
  error. (Status `success` with an attached error MUST be treated as failed.)
- A report is **successful** only if status is `success` **and** it carries no
  error.
- `skipped` is **not** a failure: the workflow continues past a skipped step.

### 3.5 Single-use and statefulness

- A step instance is **single-use within one workflow execution**: it is owned
  and driven by exactly one execution at a time and MUST NOT be shared across
  concurrent executions.
- The engine attaches a per-step state view to the step before Prepare (§7.3).

  > **⚠ Spec decision (differs from current Go implementation).** The Go `Step`
  > interface documents `WithState` as returning "a shallow copy of the step,"
  > but both concrete implementations mutate in place and return the same
  > instance. This spec does **not** require copy-on-attach; it requires only
  > that, after the engine attaches state to a step, calls to that step's
  > `State()` observe the attached bag, and that step instances are single-use
  > (above). The misleading "shallow copy" wording MUST be corrected in the Go
  > implementation to match this contract. *(See review issues #80–#84.)*

## 4. Workflow execution

### 4.1 Ordering

- A workflow executes its steps **strictly sequentially**, in topology order,
  one at a time. There is no concurrency between steps in core model v1.
- Compensation (rollback) proceeds in **reverse** topology order.

### 4.2 Workflow-level prepare

- A workflow MAY define a workflow-level prepare hook, run once before any step.
- If the workflow-level prepare fails, **no steps execute** and **no rollback
  occurs** (nothing has run yet). The workflow produces a failure report whose
  action is **prepare** (§8.3).

### 4.3 Per-step execution loop

For each step at index `i` in topology order:

1. Construct and attach the step's state view (§7.3).
2. Run the step's Prepare.
   - On Prepare failure → record a **failed** step report (action = **prepare**,
     see decision below) and apply the execution mode (§5) as for any failure.

     > **⚠ Spec decision (differs from current Go implementation).** The Go loop
     > records a step-Prepare failure with action **execute**. This spec requires
     > action **prepare** for a Prepare-phase failure, so the report faithfully
     > identifies the phase that failed. Implementations MUST be adapted.
3. Run the step's Execute; collect its report. (Execute is always defined, §3.2.1.)
4. Apply the execution mode based on the report's failed/not-failed status (§5).

### 4.4 Workflow result

- A workflow report is **successful** iff every executed step is non-failed
  (success or skipped).
- Otherwise the workflow report is **failed** and MUST include the IDs of the
  failed steps and the per-step reports (§8).

## 5. Execution and rollback modes

Two independent modes are configured per workflow. Both are serialized as the
lowercase strings below.

### 5.1 Execution mode

`execution_mode` governs what happens when a step's Execute (or Prepare) fails.

| Value (string) | On step failure |
|----------------|-----------------|
| `stop` | Halt immediately. Skip all remaining steps. **No rollback.** (Default.) |
| `continue` | Record the failure and **continue** executing remaining steps. No rollback. The workflow result is failed if any step failed. |
| `rollback` | Halt, then **compensate** the failed step and all previously executed steps in reverse order (§5.3), then stop. |

- The default execution mode is `stop`.
- The exact decision is a pure function of `(execution_mode, step failed?)`:

```
step failed?  no  → proceed to next step (any mode)
step failed?  yes → stop      : break, no rollback
                    continue  : continue to next step
                    rollback  : run compensation from i..0, then break
```

### 5.2 Rollback mode

`rollback_mode` governs what happens when a **compensation** (a step's Rollback)
itself fails, during a `rollback`-mode compensation pass.

| Value (string) | On compensation failure |
|----------------|--------------------------|
| `continue` | Continue compensating the remaining (lower-index) steps. (Default.) |
| `stop` | Stop the compensation pass immediately. |

- The default rollback mode is `continue`.
- `rollback_mode` is only consulted when `execution_mode` is `rollback`.

  > **⚠ Spec decision (differs from current Go implementation).** Go's
  > `TypeMode` allows `rollback` as a *rollback_mode* value (the enum is shared).
  > This spec restricts `rollback_mode` to `{ continue, stop }`; `rollback` as a
  > rollback_mode is meaningless and MUST be rejected at validation time.

### 5.3 Compensation pass

When `execution_mode` is `rollback` and step `i` fails:

1. Compensation runs from index `i` down to `0` (the **failed step is included**,
   so it can clean up partial work it performed before failing).
2. Each step's Rollback receives that step's **execution-time state snapshot**
   (§7.4), not the current live state.
3. A step that was **skipped** (never executed) MUST NOT be compensated; its
   Rollback MUST NOT be invoked.

   > **⚠ Spec decision (differs from current Go implementation).** The Go
   > `rollbackFrom` loop invokes `Rollback` on *every* step from `i` down to `0`
   > unconditionally (a no-op Rollback merely returns skipped, but a *defined*
   > Rollback on a step whose Execute never ran would still fire). This spec
   > requires compensation to be limited to steps that actually executed
   > (status `success` or `failed`), never `skipped`/`pending` steps. This is
   > also required for correct durability resume. Implementations MUST be
   > adapted to track per-step execution status and skip non-executed steps.

4. Each compensation produces a rollback report; these are attached under the
   corresponding step's report (§8.2).

### 5.4 Manual rollback

- An implementation MAY expose a direct "rollback the whole workflow" operation
  independent of an execution failure.
- If invoked, it compensates steps in reverse order using the most recent
  execution-time snapshots if available, otherwise the current workflow state.
- The §5.3 rule (do not compensate non-executed steps) applies equally.

## 6. Composability

- A workflow **is** a step: it satisfies the same lifecycle contract (§3) and
  MAY be included as a child step of another workflow.
- A nested (sub-)workflow:
  - Receives an **isolated** view of state: a fresh local namespace and a
    **deep clone** of the parent's global namespace (§7.3). Mutations inside the
    sub-workflow MUST NOT propagate back to the parent's global state.
  - Produces a workflow report that nests as one entry in the parent's
    `StepReports` (§8.2).
#### 6.1 Mode inheritance

- A nested workflow **inherits its parent's** execution mode and rollback mode:
  during assembly, the parent's modes are propagated to its child workflows, so
  the entire tree executes under one consistent error-handling policy.
- This is a deliberate uniformity guarantee: an operator configuring a top-level
  workflow as `rollback` gets rollback semantics throughout the tree without
  having to set the mode on every nested workflow.
- A nested workflow's modes as configured on its own builder are therefore
  advisory: they MAY be set for documentation or for standalone execution of that
  workflow, but when it runs as a child its parent's modes apply.

  > **Note.** The current Go `WorkflowBuilder` doc comment contradicts the actual
  > overwrite behavior. The behavior (parent overrides) is correct per this spec;
  > the **doc comment** MUST be corrected to match.

- **Nested compensation.** When a parent compensates a child-workflow step, the
  child workflow's own compensation logic runs (it rolls back its own steps in
  reverse). How the nested rollback report is represented under the parent is
  defined in §8.2.

## 7. State model

### 7.1 State bag

- A **state bag** is a key-value store (string keys → arbitrary values).
- It MUST support: get (raw), set, delete, clear, list keys, size, snapshot of
  items, merge, and deep clone.
- It MUST be serializable to and from a language-neutral object form (§7.5).

### 7.2 Namespaces

A **namespaced state bag** partitions state into:

| Namespace | Visibility |
|-----------|-----------|
| **Local** | Private to one step. Never visible to any other step. |
| **Global** | Shared across all steps in a workflow. Writes by one step are visible to subsequent steps. |
| **Custom (named)** | An isolated, explicitly named partition, created on first access. Isolated from Local, Global, and other custom namespaces. |

### 7.3 State injection (per step)

Before a step's Prepare, the engine MUST attach a namespaced state bag built as
follows:

- **Ordinary step:** fresh empty **Local**; **Global** is the *same shared*
  global bag as the workflow's (mutations are visible to later steps).
- **Sub-workflow step:** fresh empty **Local**; **Global** is a *deep clone* of
  the workflow's global bag (isolation, per §6).

Custom namespaces are created on demand within a step's own bag and are scoped to
that step's bag (they are not shared across steps).

> **Note (portability hazard, not yet a decision).** The current model does not
> propagate custom namespaces from the workflow to a step's injected bag, nor
> across steps. This is intentional today, but cross-language implementations
> MUST agree on it. It is captured as an open question in §10.

### 7.4 Execution-time snapshots

- After each step is processed (whether it succeeded or failed), the engine
  SHOULD capture a **deep-cloned snapshot** of that step's state (Local + Global
  + custom namespaces at that moment), keyed by step ID, for use during
  compensation (§5.3).
- Snapshot capture MAY be disabled as an optimization. When disabled,
  compensation receives the current workflow (global) state instead of a
  per-step snapshot, and per-step Local namespaces are not available during
  compensation. Implementations that offer this toggle MUST document the
  tradeoff. (Default: snapshots enabled.)
- If snapshot cloning fails, the engine MUST NOT fail the step for that reason
  alone; it SHOULD fall back to retaining a non-cloned reference and log a
  warning, so that compensation can still be attempted.

### 7.5 Serialization contract (cross-language)

This is the part most likely to diverge across languages and is therefore
normative:

- A state bag serializes to a JSON object: key (string) → value.
- After a JSON round-trip, numeric values MAY be decoded as floating point.
  Implementations MUST provide **coercing typed accessors** (integer, float,
  bool, string) that recover the intended type from the round-tripped value.
- Implementations MUST agree on numeric range/precision behavior so that a state
  bag written by one language loads losslessly in another. The boundary rules
  (e.g. large 64-bit integers vs. float64 mantissa) MUST be specified and shared
  as conformance fixtures (§7.6 references `state-numeric-boundaries.md`).
- `null` / nil values MUST be storable and MUST be distinguishable from an absent
  key.

### 7.6 Reference

The existing design docs `state-serialization.md`, `state-numeric-boundaries.md`,
and `state-preservation.md` describe the Go behavior. The normative cross-language
rules (and their fixtures) are owned by this spec; those docs are the rationale.

## 8. Report

The report is automa's primary observability primitive and a key differentiator:
it is a fully serializable tree describing an entire run. Its schema is therefore
a cross-language contract.

### 8.1 Report fields

| Field | Type | Meaning |
|-------|------|---------|
| `id` | string | The step or workflow ID that produced the report. |
| `isWorkflow` | bool | True if produced by a workflow. |
| `action` | string | Phase that produced it: `prepare` / `execute` / `rollback`. |
| `status` | string | `success` / `failed` / `skipped`. |
| `startTime`, `endTime` | timestamp | Phase wall-clock bounds. |
| `detail` | string, optional | Human-readable context. |
| `error` | string, optional | Failure message (the error rendered as a string). |
| `metadata` | object, optional | String→string structured diagnostics. |
| `steps` | array, optional | Child step reports (workflow reports only), in execution order. |
| `rollback` | object, optional | Nested rollback report tree (§8.2). |
| `executionMode`, `rollbackMode` | string, optional | Modes in effect when produced. |

- `action`, `status`, `executionMode`, and `rollbackMode` MUST serialize as the
  lowercase string forms defined in this spec, never as numeric ordinals.

  > **⚠ Spec decision (differs from current Go implementation).** Go's
  > `TypeAction` and `TypeStatus` silently map *unknown* string values to a
  > default on unmarshal (`action`→`prepare`, etc.), unlike `TypeMode` which
  > errors. For a cross-language wire contract this is unsafe: an unknown enum
  > value MUST be a decode error, not a silent default, so format drift is
  > caught. Implementations MUST be adapted to fail on unknown enum values for
  > all three enums.

### 8.2 Nesting

- A **workflow report** nests its per-step reports under `steps`, in execution
  order.
- When a step is itself a sub-workflow, that sub-workflow's report is the entry
  under the parent's `steps`.
- When compensation runs, the rollback outcomes are nested under the `rollback`
  field of the report for the step that triggered the rollback. The rollback
  report is itself a (workflow-shaped) report whose `steps` are the per-step
  rollback reports, in compensation order (reverse execution order).

### 8.3 Action and status invariants

- A `prepare`-phase failure MUST carry action `prepare` (§4.3 decision).
- An `execute`-phase report MUST carry action `execute`.
- A rollback report MUST carry action `rollback`.
- `isFailed` ≡ (`status == failed`) OR (`error` present).
- `isSuccess` ≡ (`status == success`) AND (no `error`).

## 9. Registry (optional component)

- A **registry** is a thread-safe store of named step **builders**, enabling
  workflows to be assembled by ID rather than by direct constructor reference.
- Registry semantics:
  - Adding a builder whose ID already exists MUST fail, and a batch add MUST be
    atomic (all-or-nothing).
  - Lookups by ID return the builder or a not-found indication.
- The registry is OPTIONAL for conformance; it is an assembly convenience, not
  part of the execution model.

## 10. Open questions

These MUST be resolved before core model v1 is frozen, because they affect
cross-language behavior:

1. **Custom namespace propagation (§7.3).** Should custom namespaces ever be
   shared across steps or inherited by sub-workflows, or remain strictly
   step-scoped? Current behavior is strictly step-scoped.
2. **Numeric precision boundaries (§7.5).** The exact rule for 64-bit integers
   that exceed float64's exact-integer range, across languages that lack a
   distinct integer type after JSON decode.
3. **Timestamp format.** `startTime`/`endTime` wire format (RFC 3339 string vs.
   epoch) and required precision, for cross-language report exchange.
4. **Metadata value typing.** Metadata is currently string→string. Should values
   be allowed to be arbitrary JSON, or remain strings for portability?
5. **Sub-workflow rollback report shape (§8.2).** Pin the exact nesting for a
   compensated child workflow before claiming nested durability.

## 11. Consolidated spec decisions (Go to be adapted)

Each is a place where this spec is written "the right way" and the Go reference
implementation is to be adapted (or the decision confirmed before freezing):

| # | Decision | §  |
|---|----------|----|
| D2 | `WithState` contract: attach-and-observe + single-use; fix the misleading "shallow copy" docs. | 3.5 |
| D3 | Step **Prepare** failure reports action `prepare`, not `execute`. | 4.3 |
| D4 | `rollback_mode` restricted to `{ continue, stop }`; reject `rollback`. | 5.2 |
| D5 | Compensation MUST skip non-executed (`skipped`/`pending`) steps. | 5.3 |
| D7 | Report enums (`action`, `status`) MUST fail on unknown values on decode, like `mode`. | 8.1 |

**Resolved to match current behavior (no code change to logic; docs only):**

| # | Decision | §  |
|---|----------|----|
| D1 | A step **MUST** define Execute; a no-execute step is a validation error. | 3.2.1 |
| D6 | Nested workflows **inherit** the parent's modes (parent overrides). Fix the contradicting builder doc comment. | 6.1 |

## 12. Conformance

- Cross-language agreement is verified by shared fixtures under
  `docs/spec/conformance/`, of two kinds:
  - **Behavior fixtures** — a workflow definition (steps with declared
    success/fail/skip outcomes) + the expected report tree and execution order,
    for each mode combination. These verify §4–§6 and §8 identically across
    implementations without real side effects.
  - **Serialization fixtures** — state bags and report trees in JSON that every
    implementation MUST load and re-serialize equivalently (§7.5, §8).
- The **Go** implementation is the first conformant implementation and a
  reference, not the definition. A divergence between Go behavior and this spec
  is a Go bug (or a §11 item pending adaptation), not a spec amendment.
