# Examples

This folder contains runnable examples demonstrating typical `automa` usage patterns.

Available examples:

- `setup_local` — a larger CLI-style local setup example (spinners, nested workflows, rollback).
- `hello` — tiny one-file example demonstrating a basic workflow, state usage, and printing the report.

Run an example from the repository root:

```bash
# run the tiny hello example
go run ./examples/hello

# run the setup_local example
go run ./examples/setup_local
```

If you add a new example, add a short README inside the example folder and update this file with a one-line description.

