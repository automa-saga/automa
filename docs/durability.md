# Durable Workflows (Design)

> Status: **proposed** — design under review, not yet implemented.

This document describes a design for adding **crash recovery** to automa: the
ability for a workflow to survive a process crash, restart, or power loss and
then either **resume forward** from where it stopped or **compensate backward**
(roll back) — without re-running already-completed work and without standing up
any external infrastructure.

## What automa offers with this feature

- **Zero-infrastructure durability.** Persistence is a single local file (or an
  embedded store). There is no server to run, no database to operate, and no
  network dependency. A workflow's recoverable state travels with the binary.
- **Crash recovery for sequential sagas.** If the process dies mid-workflow, a
  later invocation reads the journal and continues: it resumes the remaining
  steps, or compensates the completed ones, according to the configured
  execution mode.
- **Built on what automa already has.** The existing `NamespacedStateBag`,
  `Report`, and `TypeMode` types already serialize to JSON/YAML. The durable
  journal is largely a composition of these existing, tested pieces.
- **No new programming model.** Steps remain `Prepare`/`Execute`/`Rollback`.
  The only new obligations are an idempotency contract and keeping
  resume-relevant data in the state bag (see [Contract](#contract-for-step-authors)).

## When to use it

Durability earns its (small) complexity when **all** of the following hold:

- The workflow runs in a **single process** and is **short-to-medium length**
  (tens of steps, not thousands).
- Steps are **expensive, slow, or externally observable** — redoing them from
  scratch is costly or unsafe (e.g. provisioning cloud resources, running a
  multi-stage data migration, an installer that mutates a machine).
- Partial progress is **worth preserving** across a crash rather than restarting
  from zero.
- The set of steps is **statically determined** — the same workflow definition
  (same step IDs, same order) is available when the process restarts.

### Good fits

- Provisioning / infrastructure setup tools.
- Database or data migrations with multiple stages.
- Installers and machine-setup orchestration.
- Any CLI that performs a sequence of mutating operations that should be
  resumable or cleanly undone after an interruption.

### Not a fit (use plain `Execute`, or another tool)

- Workflows whose steps are **generated dynamically** from runtime data that is
  not reconstructible from persisted state.
- **Long-lived** workflows that must wait for hours/days across restarts (this
  needs durable timers — out of scope; see [Non-goals](#non-goals)).
- **Multi-process / distributed** execution where work is dispatched to a pool
  of workers.
- Workflows where every step is already trivially idempotent and cheap to redo —
  in that case, just re-run from the start; durability adds no value.

## Design overview

automa persists **state** (a snapshot of where the saga is), not an event
history. Recovery works by **re-attaching the same workflow definition** (which
lives in the caller's code) to the persisted state and continuing from a cursor.

This implies the central constraint of the design:

> The workflow **topology** (the ordered list of step IDs) must be the same when
> a workflow is resumed as it was when it first ran. The journal is only
> meaningful against the workflow definition that produced it.

Because the workflow definition is code in the same binary, this is a natural
fit for the single-process tools that are automa's target. A resume validates
that the rebuilt topology matches the journal and refuses to proceed otherwise.

## The journal

One file per workflow run. It is a **snapshot** (the whole state is rewritten on
each transition) rather than an append-only log. For short workflows the cost of
rewriting is negligible and the implementation is far simpler to reason about.

```go
type Phase string

const (
    PhaseForward      Phase = "forward"      // executing steps
    PhaseCompensating Phase = "compensating" // rolling back
    PhaseDone         Phase = "done"
)

type StepState string

const (
    StepPending     StepState = "pending"
    StepStarted     StepState = "started"     // written before Execute (write-ahead)
    StepCompleted   StepState = "completed"   // written after a successful Execute
    StepFailed      StepState = "failed"
    StepCompensated StepState = "compensated" // written after Rollback
)

type Journal struct {
    Version       int           `json:"version"`
    WorkflowID    string        `json:"workflow_id"`
    ExecutionMode TypeMode      `json:"execution_mode"`
    RollbackMode  TypeMode      `json:"rollback_mode"`
    Phase         Phase         `json:"phase"`
    Cursor        int           `json:"cursor"` // index currently being worked on
    Global        StateBag      `json:"global"` // shared state (already serializable)
    Steps         []StepJournal `json:"steps"`  // one per step, in workflow order
}

type StepJournal struct {
    ID       string             `json:"id"`
    State    StepState          `json:"state"`
    Snapshot NamespacedStateBag `json:"snapshot,omitempty"` // execution-time state, for rollback
    Report   *Report            `json:"report,omitempty"`
}
```

## Durable write

The journal must survive a crash *at any point*, including mid-write. A naive
overwrite can leave a corrupt, half-written file — which is worse than no
durability. The write is therefore: marshal → write to a temp file →
`fsync` → atomically `rename` over the target.

```go
func (j *Journal) persist(path string) error {
    data, err := json.Marshal(j)
    if err != nil {
        return err
    }

    tmp := path + ".tmp"
    f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
    if err != nil {
        return err
    }
    if _, err := f.Write(data); err != nil {
        _ = f.Close()
        return err
    }
    if err := f.Sync(); err != nil { // fsync: survive power loss, not just process crash
        _ = f.Close()
        return err
    }
    if err := f.Close(); err != nil {
        return err
    }
    return os.Rename(tmp, path) // atomic on POSIX: a reader sees old-or-new, never torn
}
```

`Rename` is atomic on POSIX filesystems, so a concurrent or post-crash reader
always observes either the previous complete journal or the new complete
journal, never a partial one.

## Write points in the execution loop

The persist calls slot into the existing `Execute` loop
([`workflow.go`](../workflow.go)) at points where the engine already clones state
and builds reports. Per step `i`:

```
── forward phase ──
1. Steps[i].State = Started; Cursor = i;            persist()   ← WRITE-AHEAD (before side effect)
2. step.WithState(...); step.Prepare(ctx)
3. report = step.Execute(ctx)
4. Steps[i].State = Completed | Failed
   Steps[i].Snapshot = clone(step.State())                       (already computed today)
   Steps[i].Report   = report
   Global            = w.State().Global()
                                                     persist()   ← COMMIT POINT (after side effect)
5. on failure + RollbackOnError:
   Phase = Compensating;                             persist()

── compensating phase (rollbackFrom) ──
for i := Cursor; i >= 0; i-- {
    if Steps[i].State == Compensated { continue }                 ← idempotent resume
    step.WithState(Steps[i].Snapshot); step.Rollback(ctx)
    Steps[i].State = Compensated;                    persist()    ← per-compensation commit
}

── done ──
Phase = Done;                                        persist()    (or delete the journal file)
```

This is two `fsync`s per step (write-ahead + commit). For the target workload
(tens of steps) the overhead is immaterial.

The **write-ahead** record in step 1 is what makes recovery decidable: after a
crash, a step marked `Started` but not `Completed` is the ambiguous case (its
side effect may or may not have happened), and is handled by the idempotency
contract below.

## Recovery / `Resume`

`Resume` is the new public entry point. The caller re-supplies the workflow
definition (the same builder/code); automa rehydrates state onto it and
continues.

```go
func ResumeWorkflow(ctx context.Context, wb *WorkflowBuilder, journalPath string) *Report {
    j, err := loadJournal(journalPath)
    // missing file  → start a fresh run (write a new journal)
    // corrupt file  → fail loudly; do NOT silently restart (could double-execute side effects)

    wf := wb.Build()
    if err := validateTopology(wf, j); err != nil {
        return failure(...) // step IDs / order changed since the journal was written
    }

    wf.rehydrateGlobal(j.Global)

    switch j.Phase {
    case PhaseForward:
        // Re-run any Started-but-not-Completed step (idempotency required),
        // then continue forward from the first incomplete step.
        return wf.executeFrom(ctx, j.firstIncompleteIndex(), j)
    case PhaseCompensating:
        // Continue rolling back from Cursor, skipping steps already Compensated.
        return wf.compensateFrom(ctx, j.Cursor, j)
    case PhaseDone:
        return j.finalReport()
    }
}
```

`validateTopology` compares the rebuilt step IDs and order against the journal.
A mismatch (the workflow definition changed between crash and resume) is an
error, not a silent restart — this is the single-process analogue of workflow
versioning, and it is intentionally strict.

## Worked example: crash and resume

Consider a 4-step provisioning workflow run with `WithJournal("setup.journal")`:

```
1. create-network
2. create-database     ← expensive; allocates a managed instance
3. create-cache
4. wire-config
```

**First run, crashes mid step 2.** The journal on disk, immediately before the
crash, is:

```jsonc
{
  "phase": "forward", "cursor": 1,
  "steps": [
    { "id": "create-network",  "state": "completed", "snapshot": {…} },
    { "id": "create-database", "state": "started" },   // write-ahead record
    { "id": "create-cache",    "state": "pending" },
    { "id": "wire-config",     "state": "pending" }
  ]
}
```

`create-network` committed. `create-database` was marked `started` *before* its
side effect ran, then the process died — we do not know whether the database was
actually allocated.

**Resume.** The caller re-runs the same binary, which rebuilds the identical
workflow and calls `ResumeWorkflow(ctx, wb, "setup.journal")`:

1. Load journal, `validateTopology` passes (same 4 step IDs, same order).
2. Rehydrate global state (so `create-network`'s outputs are visible).
3. Phase is `forward`; the first incomplete step is index 1.
4. **`create-database` is re-executed** — it was `started`, not `completed`.
   This is the step whose idempotency matters: it must check "does this database
   already exist?" and either adopt the existing instance or create one, so that
   running it a second time is equivalent to running it once.
5. Steps 3 and 4 then run normally. `create-network` is **never touched** — the
   expensive completed work is preserved.

**Contrast — what a non-idempotent step costs.** If `create-database` blindly
issued "allocate a new instance" with no existence check, the resume would
allocate a *second* database. Nothing in the engine can prevent this; only the
step's own idempotency can. This is the entire reason for the contract below.

## Contract for step authors

Durability shifts a small amount of responsibility onto the workflow author.
These obligations must be documented prominently and, where feasible, enforced:

1. **Steps must be idempotent.** A step marked `Started` but not `Completed` is
   re-executed on resume, because the engine cannot know whether its side effect
   completed before the crash. Running a step twice must be equivalent to
   running it once.
2. **Compensations (rollbacks) must be idempotent.** Same reasoning during the
   compensating phase.
3. **Resume-relevant data must live in the state bag.** Anything a step needs in
   order to resume or to compensate (resource IDs, handles, prior outputs) must
   be written to `State().Global()` or the step's namespaces — not held in step
   closures or struct fields, which do not survive the process.
4. **Topology must be reconstructible.** The same ordered set of step IDs must be
   produced by the caller's code at resume time. If steps are derived from
   runtime data, that data must itself be persisted (e.g. in global state) so the
   topology is deterministic across restarts.

## Non-goals

These are explicitly **out of scope** for this design. Some may be added later as
independent features; none are required for crash recovery of sequential sagas.

- **Durable timers / long sleeps across restarts** (e.g. "wait 3 days").
- **Distributed or multi-process execution** / worker pools.
- **Signals, queries, or other external interaction** with a running workflow.
- **Dynamic topology** that cannot be reconstructed from persisted state.
- **Append-only log / compaction.** The snapshot model is sufficient for the
  target workload; a WAL can be introduced later if a long-running use case
  appears, behind the same `Resume` API.

## Backward compatibility

- Durability is **opt-in**. Workflows constructed today and run via `Execute` /
  `RunWorkflow` are unchanged and write nothing to disk.
- A workflow becomes durable by configuring a journal location (e.g.
  `WithJournal(path)` on the builder) and invoking via `ResumeWorkflow`.
- The journal format carries a `Version` field so the on-disk schema can evolve.

## Open questions

- Storage backend: start with a plain JSON file (snapshot) — should an embedded
  KV (e.g. BoltDB) be offered as an alternative for many concurrent workflow
  runs?
- Journal lifecycle: delete on `PhaseDone`, or retain for audit and let the
  caller prune?
- Where should `WithJournal` live on the builder, and how is the path namespaced
  per run (workflow ID + run ID)?
- How strictly should `validateTopology` treat additive changes (e.g. new steps
  appended after the last completed one)?
