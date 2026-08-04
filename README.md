# Automa
Automa is a Saga Workflow Engine for Go, designed for sequential and transactional business processes. 

The name `automa` is derived from the word `automate`.

## Features

- Sequential execution of workflow steps
- Automatic rollback on error
- Compensating actions for non-reversible steps
- Step-level execution reporting (with JSON/YAML marshalling support)
- Extensible step interface
- Opt-in crash recovery: a durable journal plus resume (forward or compensating)

## Getting Started

**API stability:** As of `v1.0.0` the public API is stable and follows
[semantic versioning](https://semver.org/). Breaking changes to the public API
(builders, the `Step`/`Workflow`/`Report`/`State` interfaces, and the resume +
retention API) will only ship in a new major version. The behavior is pinned by
the conformance fixtures in [`docs/spec/conformance`](docs/spec/conformance) and
the normative [core](docs/spec/core-spec.md) and
[durability](docs/spec/durability-spec.md) specs.

Migrating from `0.x`: the `v1.0.0` surface is the `0.11.x` API with the spec
adaptations applied — review the [spec](docs/spec) and the conformance fixtures
if you relied on any pre-`1.0` behavior.

### Installation

```sh
go get -u github.com/automa-saga/automa
```

See an [example](https://github.com/automa-saga/automa/blob/master/examples) in the examples directory. 

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

See the runnable [`examples/resumable_setup`](examples/resumable_setup) (crash it
mid-workflow, re-run it, watch it resume), the design rationale in
[`docs/durability.md`](docs/durability.md), and the normative
[durability spec](docs/spec/durability-spec.md).

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

## Development
 - `task test` runs the tests (install `task` tool: https://taskfile.dev/installation/).
 - In order to build example, do `cd docs/examples && go build`. Then the example can be then run using `./example`.

## Contribution
Any feedback, comment and contributions are very much welcome. 

Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. Developers are also encouraged to use [commitizen](https://commitizen-tools.github.io/commitizen/) 
for commits messages.

Please note the PR will be squashed merge to master with commitizen format for the PR title. So even if commitizen is not
used for individual commits in the PR, the repository maintainer are requested to ensure that the PR title follows 
commitizen format before squash-merging the PR.

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.
