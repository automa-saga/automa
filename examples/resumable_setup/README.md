# resumable_setup

A runnable demonstration of automa's crash-recovery durability
([durability spec](../../docs/spec/durability-spec.md),
[design](../../docs/durability.md)). The workflow provisions a few marker files
under a work directory while journaling its progress. Crash it partway through
and re-run it — it resumes from where it stopped instead of starting over.

## Run it

First run, crashing right after the `write_config` step:

```bash
go run ./examples/resumable_setup -crash-at=write_config
```

```
No journal yet — starting fresh.
  + create_workdir
  + write_config
  ! simulating a crash right after write_config
exit status 1
```

Re-run with no crash — it resumes:

```bash
go run ./examples/resumable_setup
```

```
Found an existing journal — resuming.
  = write_config (already done; skipping side effect)
  + provision_database
  + finalize

✔ workflow completed.
```

`create_workdir` isn't re-run (it was recorded `completed`), `write_config` is
re-run but is an idempotent no-op (its marker already exists), and the remaining
steps run to completion. Run it a third time and it's a safe no-op: the journal
is `done`.

Flags: `-journal <path>` (journal file), `-work <dir>` (provisioned directory),
`-crash-at <stepID>` (simulate a crash after a step; omit on the resume run).

## How it works

- **One entry point.** [`main.go`](main.go) always calls
  `automa.ResumeWorkflow(ctx, wb, journalPath)`. A missing journal is a normal
  fresh start; an existing one is resumed (durability-spec §6.2).
- **Opt-in journaling.** `ResumeWorkflow` writes the journal at the given path as
  the workflow runs (write-ahead before each side effect, commit after).
- **The author contract.** Each step in [`steps.go`](steps.go) is **idempotent**:
  it checks whether its marker already exists and does nothing if so. That is
  what makes it safe for resume to re-run a step that was `started` but never
  confirmed `completed` before the crash (durability-spec §7).
