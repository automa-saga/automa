# Hello Example

This tiny example demonstrates a minimal `automa` workflow built with two step builders.

What it shows:
- Creating steps with `NewStepBuilder()` (no `.Build()` on individual steps when passing to a `WorkflowBuilder`).
- Using `stp.State().Global()` and `stp.State().Local()`.
- Building a workflow with `NewWorkflowBuilder().Steps(step1, step2).Build()`.
- Executing the workflow and printing the YAML report.

Run it from the repository root:

```bash
go run ./examples/hello
```

Expected outcome:
- Workflow executes both steps successfully.
- The second step's report metadata includes `env: dev` (global value set by the first step) and `tmp: ""` (the local value set by the first step is not visible to the second step).

This example is fast and has no network dependencies; it's useful for quick local testing and CI sanity checks.
