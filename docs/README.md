# Automa Documentation

Welcome to the automa workflow orchestration framework documentation.

## Table of Contents

- [Architecture](architecture.md) - Framework design and components
- [Developer Guide](developer-guide.md) - Development, testing, and contribution guidelines  
- [Usage Examples](usage-examples.md) - Practical examples and best practices
- [State Preservation](state-preservation.md) - Memory optimization and configuration

## Examples and Quick Demo

This repository includes runnable examples in the `examples/` directory. A small demo is provided at:

- `examples/setup_local` - CLI-style local environment setup example (uses spinners and nested workflows)
- `examples/hello` - tiny one-file example for a quick local run (builds a small workflow and prints a YAML report)

Quick run from repository root (recommended):

```bash
# Run the tiny hello example (single-file example)
go run ./examples/hello

# Run the setup_local example (it is a separate module with its own go.mod):
# Option A: change into its directory and run
cd examples/setup_local && go run .
# Option B: run in a subshell from repo root (keeps current shell clean)
(cd examples/setup_local && go run .)
```

Notes:
- `examples/setup_local` is a standalone module (it contains a `go.mod` with a `replace` directive referencing the repo root). Running `go run ./examples/setup_local` from the repo root may fail in some setups; use one of the two commands above to run it reliably.
- `setup_local` performs network downloads and writes to `/tmp` — run it on macOS or Linux and ensure you have network access and write permission.

## Quick Start

### Installation

You can install the package for local development using Go modules. A couple of common options:

```bash
# (legacy) fetch into module cache - compatible with many older docs
go get -u github.com/automa-saga/automa

# (modern) install a specific version (recommended in newer Go toolchains)
go install github.com/automa-saga/automa@latest
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"

    "github.com/automa-saga/automa"
)

func main() {
    // Define a simple step
    step := automa.NewStepBuilder().
      WithId("hello-step").
      WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
         fmt.Println("Hello from automa!")
         return automa.SuccessReport(stp)
      })

    // Build a workflow
    wf, _ := automa.NewWorkflowBuilder().
      WithId("hello-workflow").
      Steps(step).
      Build()

    // Execute the workflow
    report := wf.Execute(context.Background())

    if report.IsSuccess() {
      fmt.Println("Workflow completed successfully!")
    }
}
```

## Documentation Overview

### For Users

Start here if you want to use automa in your projects:

1. **[Usage Examples](usage-examples.md)** - Learn by example
   - Basic workflows
   - Error handling modes
   - State management
   - Rollback scenarios
   - Real-world examples

2. **[State Preservation](state-preservation.md)** - Optimize memory usage
   - When to enable/disable state preservation
   - Memory impact analysis
   - Configuration guide

3. **[Architecture](architecture.md)** - Understand how it works
   - Core components
   - Execution modes
   - State management design
   - Thread safety model

## Core Concepts

### Step

The fundamental unit of work. A step can:
- Execute business logic
- Prepare context and state
- Rollback changes if needed
- Report execution status

### Workflow

A composite step that orchestrates multiple steps in sequence. Features:
- Sequential execution
- Configurable error handling (StopOnError, ContinueOnError, RollbackOnError)
- State isolation for sub-workflows
- Async callback support

### State Management

Namespaced state bags provide flexible state isolation:
- **Local**: Step-private state
- **Global**: Workflow-shared state
- **Custom**: Named namespaces for specific use cases

### Report

Structured execution results with:
- Status (Success, Failure, Skipped)
- Error information
- Timing metadata
- Hierarchical step reports

## Execution Modes

### RollbackOnError 

Safest mode - rolls back all executed steps when one fails:

```
Step1 ✓ → Step2 ✓ → Step3 ✗ → [ROLLBACK]
                               ↓
Step2 Rollback ✓ ← Step1 Rollback ✓
```

### StopOnError (Default)

Stops immediately on first failure, no rollback:

```
Step1 ✓ → Step2 ✗ → [STOP]
          Step3 (not executed)
```

### ContinueOnError

Best-effort mode - continues despite failures:

```
Step1 ✓ → Step2 ✗ → Step3 ✓ → [COMPLETE]
```

## Key Features

✅ **Composable** - Workflows are steps, enabling nesting  
✅ **Type-safe** - Fluent builder API with compile-time checks  
✅ **Thread-safe** - Concurrent-safe state management  
✅ **Flexible** - Multiple execution and rollback modes  
✅ **Observable** - Structured reports with rich metadata  
✅ **Testable** - Clean interfaces and dependency injection  
✅ **Memory-efficient** - Optional state preservation  

## Common Use Cases

- **Infrastructure automation** - Server provisioning, configuration management
- **Database migrations** - Schema changes with rollback support
- **Deployment pipelines** - Multi-environment deployments
- **Data processing** - ETL workflows with error handling
- **Test automation** - Setup/teardown with cleanup
- **Workflow orchestration** - Complex multi-step processes

## Support

- **Issues**: [GitHub Issues](https://github.com/automa-saga/automa/issues)
- **Discussions**: [GitHub Discussions](https://github.com/automa-saga/automa/discussions)
- **Examples**: See `examples/` directory in the repository

## License

See LICENSE file in the repository root.

## Contributing

We welcome contributions! See [Developer Guide](developer-guide.md) for details.
