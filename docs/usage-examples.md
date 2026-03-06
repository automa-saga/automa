# Usage Examples

This page provides runnable and copy-paste friendly examples that demonstrate common `automa` usage patterns. The examples below are intentionally small to illustrate concepts; see `examples/setup_local` for a larger, real-world demo.

- `examples/setup_local` — CLI-style local environment setup example. This example demonstrates:
  1. Building workflows and nested workflows
  2. Prepare / Execute / Rollback callbacks
  3. Using `Report` and printing YAML output

Quick run from repository root:

```bash
# run the tiny hello example (single-file example)
go run ./examples/hello

# run the setup_local example (it is a separate module; run from its directory):
# Option A: change into its directory and run
#   cd examples/setup_local && go run .
# Option B: run in a subshell from repo root
#   (cd examples/setup_local && go run .)
```

---

## 1) Basic workflow

A minimal workflow with a single step that prints a message and returns success.

```go
// Basic workflow: one step
step := automa.NewStepBuilder().
    WithId("hello").
    WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
        fmt.Println("Hello from automa")
        return automa.SuccessReport(stp)
    })

wf, _ := automa.NewWorkflowBuilder().
    WithId("hello-workflow").
    Steps(step).
    Build()

// Execute
report := wf.Execute(context.Background())
if report.IsSuccess() {
    fmt.Println("Workflow succeeded")
}
```

Notes:
- Use `NewStepBuilder()` and `NewWorkflowBuilder()` to construct steps and workflows.
- Builders are fluent; call `Build()` to get concrete `Step`/`Workflow` values.

---

## 2) Error handling modes

`automa` supports three common execution modes:

- `StopOnError` — stop execution on first failure (default).
- `ContinueOnError` — attempt to continue executing remaining steps despite failures.
- `RollbackOnError` — execute compensating rollbacks for previously executed steps when a failure occurs.

Example of a workflow configured to rollback on error:

```go
wf, _ := automa.NewWorkflowBuilder().
    WithId("rollback-demo").
    WithExecutionMode(automa.RollbackOnError).
    Steps(step1, step2, step3).
    Build()

report := wf.Execute(context.Background())
```

Behavioral summary:
1. StopOnError — first failing step halts; no rollback is performed.
2. ContinueOnError — failures are recorded, but the workflow continues to attempt remaining steps.
3. RollbackOnError — if any step fails, previously successful steps' `Rollback` handlers are executed in reverse order.

---

## 3) State management (Local / Global / Namespaces)

`automa` provides namespaced state bags so step authors can avoid accidental key collisions.

- `stp.State().Local()` — step-private (per-step) namespace.
- `stp.State().Global()` — workflow-shared namespace visible to all steps in the workflow.
- `stp.State().WithNamespace("name")` — custom namespace for specialized isolation.

Example usage inside a step:

```go
WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
    // Store a local value (isolated to this step)
    stp.State().Local().Set("bind-target", "/mnt/app1")

    // Store a shared configuration for other steps
    stp.State().Global().Set("env", "development")

    // Store under a custom namespace
    stp.State().WithNamespace("build").Set("artifact", "app.tar.gz")

    // Read values
    target := stp.State().Local().String("bind-target")
    env := stp.State().Global().String("env")
    return automa.SuccessReport(stp, automa.WithMetadata(map[string]string{"target": target, "env": env}))
})
```

Key points:
- Local values do not clobber other steps' local namespaces.
- Global values are shared across all steps in the workflow and are useful for passing configuration.
- Custom namespaces allow arbitrary isolation beyond the local/global split.

---

## 4) Rollback scenario

This example illustrates a two-step workflow where the second step fails, triggering a rollback that reads the first step's local state snapshot.

```go
// Step 1: prepares data and succeeds
s1 := automa.NewStepBuilder().WithId("s1").
    WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
        stp.State().Local().Set("tmp-file", "/tmp/foo")
        return automa.SuccessReport(stp)
    }).
    WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
        // During rollback we expect to see the local state captured at execution time
        tmp := stp.State().Local().String("tmp-file")
        // cleanup using tmp
        return automa.SuccessReport(stp)
    })

// Step 2: fails which triggers rollback
s2 := automa.NewStepBuilder().WithId("s2").
    WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
        return automa.FailureReport(stp, automa.WithError(automa.IllegalArgument.New("simulate failure")))
    })

wf, _ := automa.NewWorkflowBuilder().
    WithId("rollback-example").
    WithExecutionMode(automa.RollbackOnError).
    Steps(s1, s2).
    Build()

report := wf.Execute(context.Background())
// report.IsFailed() == true and s1.Rollback() should have been invoked
```

Notes on rollback state snapshots:
- By default the workflow captures per-step state snapshots to provide the step's local namespaces to the rollback handlers.
- If state cloning is disabled (for memory/perf), rollback may receive the live workflow state instead of a cloned snapshot; consult the workflow builder options for `preserveStatesForRollback`.

---

## 5) Printing and exporting reports

After execution you get a `*automa.Report` which can be marshaled to JSON or YAML for logging, CI, or UI consumption.

```go
out, err := yaml.Marshal(report)
if err != nil {
    log.Fatalf("marshal error: %v", err)
}
fmt.Println(string(out))
```

---

## 6) Real-world example: `examples/setup_local`

The `examples/setup_local` directory is a runnable CLI-style demo that shows:
- Spinners and async UI updates while steps run
- Nested workflows
- Prepare / Execute / Rollback usage
- Using `stp.State().Local()` and `Global()` in realistic scenarios

Run it locally:

```bash
go run ./examples/setup_local
```

---

## 7) Contributing examples

If you add an example, create a new subdirectory under `examples/` and add a brief README describing what the example demonstrates. Update this page to add a short summary and a quick run command.

---
