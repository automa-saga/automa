# quickstart

The smallest complete automa workflow: two steps, each with a compensating
rollback, run as a saga. The second step (`charge_card`) fails, so automa halts
and compensates the executed steps in reverse order.

## Run it

```bash
go run ./examples/quickstart
```

```
→ reserving inventory
→ charging card
← refunding card
← releasing inventory
```

followed by the full JSON report tree (see the top-level
[README](../../README.md#the-report-tree) for an annotated, abridged version).

## What to look at

- [`main.go`](main.go) — building steps with `WithExecute`/`WithRollback`,
  assembling them into a workflow with `RollbackOnError`, running it with
  `RunWorkflow`, and inspecting the returned `*Report`.

For crash-recovery durability, see [`../resumable_setup`](../resumable_setup).
