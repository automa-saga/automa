# Automa

[![Go Reference](https://pkg.go.dev/badge/github.com/automa-saga/automa.svg)](https://pkg.go.dev/github.com/automa-saga/automa)
[![Go Report Card](https://goreportcard.com/badge/github.com/automa-saga/automa)](https://goreportcard.com/report/github.com/automa-saga/automa)
[![Checks](https://github.com/automa-saga/automa/actions/workflows/flow-pull-request-checks.yaml/badge.svg)](https://github.com/automa-saga/automa/actions/workflows/flow-pull-request-checks.yaml)
[![Release](https://img.shields.io/github/v/release/automa-saga/automa)](https://github.com/automa-saga/automa/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**Crash-safe saga workflows for Go — automatic rollback and resume-after-crash, backed by a single local file. No server, no database, no dependencies.**

Automa runs a sequence of steps as a transactional saga: if any step fails, it
automatically compensates the steps that already ran, in reverse order. Opt into
durability and a workflow will also survive a process crash, restart, or power
loss and continue exactly where it stopped — all from a single journal file, with
no external infrastructure to operate.

## Features

- **Saga semantics** — sequential steps with automatic, reverse-order rollback on failure.
- **Compensating actions** — clean up non-reversible work (releases, refunds, deletes) when a later step fails.
- **Opt-in crash recovery** — a durable journal plus resume (forward or compensating) from one local file. Off by default; writes nothing to disk until you enable it.
- **Structured reports** — a step-level execution report tree with JSON/YAML marshalling for logging, auditing, and debugging.
- **Zero required dependencies** — no server, database, or broker; logs through the standard library's `log/slog`.
- **Spec-backed & stable** — a `v1.0.0` API pinned by a normative [spec](docs/spec) and cross-implementation [conformance fixtures](docs/spec/conformance).

## When to use automa

| Reach for automa when… | Reach for a hosted engine (Temporal, Restate, …) when… |
| --- | --- |
| You need saga/rollback semantics **inside a single process** — a CLI, installer, provisioner, migration, embedded/edge service, or daemon. | You need **distributed, multi-process** durable execution across a fleet of workers. |
| You want crash recovery with **no infrastructure to run** — just a local file. | You already operate (or want) a workflow **server/cluster**. |
| Your workflow is **sequential and transactional**. | You need long-running orchestration with signals, timers, and human-in-the-loop across services. |

automa deliberately targets the single-process, sequential saga model. It is a
library, not a platform.

## Installation

```sh
go get github.com/automa-saga/automa
```

Requires Go 1.26+.

## Quick start

A workflow is an ordered list of steps; each step has an `Execute` and an
optional `Rollback` (its compensating action). With `RollbackOnError`, a failure
in any step compensates the executed steps in reverse order.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/automa-saga/automa"
)

func main() {
    reserveInventory := automa.NewStepBuilder().
        WithId("reserve_inventory").
        WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
            fmt.Println("→ reserving inventory")
            return automa.SuccessReport(stp)
        }).
        WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
            fmt.Println("← releasing inventory")
            return automa.SuccessReport(stp)
        })

    chargeCard := automa.NewStepBuilder().
        WithId("charge_card").
        WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
            fmt.Println("→ charging card")
            return automa.FailureReport(stp, automa.WithError(fmt.Errorf("card declined")))
        }).
        WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
            fmt.Println("← refunding card")
            return automa.SuccessReport(stp)
        })

    wb := automa.NewWorkflowBuilder().
        WithId("checkout").
        WithExecutionMode(automa.RollbackOnError).
        Steps(reserveInventory, chargeCard)

    report := automa.RunWorkflow(context.Background(), wb)

    out, _ := json.MarshalIndent(report, "", "  ")
    fmt.Println(string(out))

    if report.IsFailed() {
        os.Exit(1)
    }
}
```

`charge_card` fails, so automa halts and compensates in reverse order:

```
→ reserving inventory
→ charging card
← refunding card
← releasing inventory
```

The full runnable source is in [`examples/quickstart`](examples/quickstart):

```sh
go run ./examples/quickstart
```

### The report tree

`RunWorkflow` returns a structured `*Report` you can inspect or marshal to
JSON/YAML. Each step's compensating rollback is attached under that step's report
(abridged below — timestamps omitted):

```json
{
  "id": "checkout",
  "isWorkflow": true,
  "action": "execute",
  "status": "failed",
  "error": "workflow \"checkout\" completed with 1 step failures: [charge_card]",
  "steps": [
    {
      "id": "reserve_inventory",
      "action": "execute",
      "status": "success",
      "rollback": { "id": "reserve_inventory", "action": "rollback", "status": "success" }
    },
    {
      "id": "charge_card",
      "action": "execute",
      "status": "failed",
      "error": "card declined",
      "rollback": { "id": "charge_card", "action": "rollback", "status": "success" }
    }
  ],
  "executionMode": "rollback"
}
```

## Durability (crash recovery)

Automa can make a workflow survive a process crash, restart, or power loss and
then continue where it stopped — resuming the remaining steps or completing an
interrupted rollback — with no external infrastructure (the state is a single
local journal file). It is **opt-in**: without it, workflows behave exactly as
before and write nothing to disk.

Make a workflow durable by giving it a journal path and running it through
`ResumeWorkflow`, which starts fresh when there is no journal yet and resumes
when there is:

```go
wb := automa.NewWorkflowBuilder().
    WithId("setup").
    WithExecutionMode(automa.RollbackOnError).
    WithJournal("/var/lib/myapp/setup.journal"). // opt in
    Steps(/* ... */)

// First run starts fresh and journals; a later run with the same path resumes.
report := automa.ResumeWorkflow(context.Background(), wb, "/var/lib/myapp/setup.journal")
```

The runnable [`examples/resumable_setup`](examples/resumable_setup) provisions a
few files while journaling its progress. Crash it partway through, then re-run it
and it resumes from where it stopped:

```sh
# First run: crash right after the "write_config" step.
$ go run ./examples/resumable_setup -crash-at=write_config
No journal yet — starting fresh.
  + create_workdir
  + write_config
  ! simulating a crash right after write_config
exit status 1

# Re-run with no crash: it resumes — completed steps are skipped, the rest run.
$ go run ./examples/resumable_setup
Found an existing journal — resuming.
  = write_config (already done; skipping side effect)
  + provision_database
  + finalize

✔ workflow completed.
```

See the design rationale in [`docs/durability.md`](docs/durability.md) and the
normative [durability spec](docs/spec/durability-spec.md).

### Step-author contract

Durability shifts a small, well-defined set of obligations onto the step author
(durability spec §7). The engine cannot enforce these — they are your
responsibility, and resume correctness depends on them:

1. **Steps must be idempotent.** A step recorded `started` but not `completed`
   before a crash is re-executed on resume (its side effect's completion is
   unknown). Running it twice must equal running it once — check "does this
   already exist?" and adopt the existing resource rather than creating a second.
2. **Compensations must be idempotent.** A rollback may be retried across a crash
   during the compensating phase.
3. **Resume-relevant data must live in the state bag.** Anything a step needs to
   resume or compensate (resource IDs, handles, prior outputs) must be written to
   `State().Global()` or the step's namespace — not held in closures or struct
   fields, which do not survive the process.
4. **Topology must be reconstructible.** The same ordered step IDs and modes must
   be produced by your code at resume time; if steps are derived from runtime
   data, persist that data so the topology is deterministic across restarts.

## Logging

Automa logs through the standard library's [`log/slog`](https://pkg.go.dev/log/slog) and stays
logging-backend agnostic — it has no logging dependency of its own. Attach a `*slog.Logger` to a
workflow via `WithLogger` for the engine's own diagnostics (e.g. rollback-snapshot warnings):

```go
wf := automa.NewWorkflowBuilder().
    WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))).
    // ...
    Build()
```

If no logger is provided, automa stays silent (it uses `slog.DiscardHandler`).

We recommend [`logx`](https://github.com/automa-saga/logx) so you get `zerolog` + `lumberjack`
(console writer, rolling files, level, etc.) while still driving automa through the `slog` API:

```go
logx.Initialize(logx.LoggingConfig{
    Level: "info", ConsoleLogging: true,
    FileLogging: true, Directory: "/var/log/myapp", Filename: "daemon.log",
    MaxSize: 50, MaxBackups: 3, MaxAge: 30, Compress: true,
})

wf := automa.NewWorkflowBuilder().
    WithLogger(slog.New(logx.NewSlogHandler())).
    // ...
    Build()
```

## API stability

As of `v1.0.0` the public API is stable and follows
[semantic versioning](https://semver.org/). Breaking changes to the public API
(builders, the `Step`/`Workflow`/`Report`/`State` interfaces, and the resume +
retention API) will only ship in a new major version. The behavior is pinned by
the conformance fixtures in [`docs/spec/conformance`](docs/spec/conformance) and
the normative [core](docs/spec/core-spec.md) and
[durability](docs/spec/durability-spec.md) specs.

**Migrating from `0.x`:** the `v1.0.0` surface is the `0.11.x` API with the spec
adaptations applied — review the [spec](docs/spec) and the conformance fixtures
if you relied on any pre-`1.0` behavior.

## Documentation

- [Examples](examples) — [`quickstart`](examples/quickstart), [`resumable_setup`](examples/resumable_setup), [`setup_local`](examples/setup_local).
- [Core spec](docs/spec/core-spec.md) — the normative, language-neutral model.
- [Durability spec](docs/spec/durability-spec.md) and [design rationale](docs/durability.md).
- [Conformance fixtures](docs/spec/conformance) — behavior, journal, and serialization contracts.
- [API reference](https://pkg.go.dev/github.com/automa-saga/automa) on pkg.go.dev.

## Development
 - This repository uses a git submodule at `docs/spec` for the language-neutral
   [automa spec and conformance fixtures](https://github.com/automa-saga/automa-spec).
   Clone with `git clone --recursive`, or after cloning run
   `git submodule update --init`. The conformance tests read the fixtures from
   this submodule, so they will fail if it has not been checked out.
 - `task test` runs the tests (install `task` tool: https://taskfile.dev/installation/).
 - In order to build example, do `cd docs/examples && go build`. Then the example can be then run using `./example`.
 - `task test` runs the tests (install the `task` tool: https://taskfile.dev/installation/).
 - Run an example directly, e.g. `go run ./examples/quickstart` or `go run ./examples/resumable_setup`.

## Contributing

Contributions are very welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for how to
set up your environment, the expected checks (`task build`, `task lint`,
`task test`), the spec/conformance workflow, and the commit sign-off and PR
conventions.

New to open source? [First Contributions](https://github.com/firstcontributions/first-contributions)
is a friendly starting point.

## License

Automa is licensed under the [Apache License 2.0](LICENSE).
