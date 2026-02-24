# Developer Guide

Welcome to the automa developer guide. This document helps you understand how to develop, test, and contribute to the automa framework.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Project Structure](#project-structure)
3. [Development Workflow](#development-workflow)
4. [Testing](#testing)
5. [Code Style](#code-style)
6. [Contributing](#contributing)
7. [Release Process](#release-process)

## Getting Started

### Prerequisites

- **Go 1.25.0+** (check with `go version`)
- **golangci-lint** (for linting)
- **git** (for version control)

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/automa-saga/automa.git
cd automa

# Install dependencies
go mod tidy

# Verify setup
go build ./...
go test ./...
```

### Install Development Tools

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installation
golangci-lint --version
```

## Project Structure

```
automa/
├── automa.go                    # Core interfaces and types
├── step_*.go                    # Step implementations
├── workflow_*.go                # Workflow implementations
├── state_*.go                   # State management
├── report_*.go                  # Report types
├── errors.go                    # Error definitions
├── type_*.go                    # Type enums (Mode, Status, Action)
├── runtime_value.go             # Runtime value types
├── registry.go                  # Builder registry
│
├── *_test.go                    # Unit tests (co-located)
│
├── automa_steps/                # Built-in step implementations
│   ├── step_bash.go
│   └── step_bash_test.go
│
├── examples/                    # Example usage
│   └── setup_local/
│
├── docs/                        # Documentation
│   ├── architecture.md
│   ├── developer-guide.md
│   ├── usage-examples.md
│   ├── state-preservation.md
│   └── thread-safety-tests.md
│
├── vendor/                      # Vendored dependencies
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── README.md                    # Project overview
├── LICENSE                      # License file
├── Taskfile.yaml                # Task runner configuration
└── .golangci.yml                # Linter configuration
```

## Development Workflow

### Making Changes

1. **Create a feature branch**

   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes**

   - Follow Go idioms and best practices
   - Write tests for new functionality
   - Update documentation as needed

3. **Run tests locally**

   ```bash
   go test ./...
   go test ./... -race  # with race detector
   ```

4. **Run linter**

   ```bash
   golangci-lint run
   ```

5. **Commit your changes**

   ```bash
   git add .
   git commit -m "feat: add new feature description"
   ```

### Commit Message Convention

Follow conventional commits format:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test additions or changes
- `refactor:` Code refactoring
- `perf:` Performance improvements
- `chore:` Build/tooling changes

Example:
```
feat: add state preservation configuration

- Add WithStatePreservation() to WorkflowBuilder
- Add preserveStatesForRollback field to workflow
- Update documentation
```

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./automa_steps

# Specific test
go test -v -run TestWorkflow_Execute

# With race detector (important!)
go test ./... -race

# With coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=cover.out
go tool cover -html=cover.out -o coverage.html
```

### Writing Tests

#### Unit Tests

Co-locate tests with source files:

```go
// workflow_test.go
package automa

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWorkflow_Execute_Success(t *testing.T) {
    // Arrange
    step := NewStepBuilder().
        WithId("test-step").
        WithExecute(func(ctx context.Context, stp Step) *Report {
            return SuccessReport(stp)
        }).
        Build()
    
    wf := NewWorkflowBuilder().
        WithId("test-workflow").
        Steps(step).
        Build()
    
    // Act
    report := wf.Execute(context.Background())
    
    // Assert
    assert.True(t, report.IsSuccess())
    assert.Equal(t, "test-workflow", report.WorkflowId)
}
```

#### Table-Driven Tests

For testing multiple scenarios:

```go
func TestTypeMode_String(t *testing.T) {
    tests := []struct {
        name     string
        mode     TypeMode
        expected string
    }{
        {"StopOnError", StopOnError, "StopOnError"},
        {"ContinueOnError", ContinueOnError, "ContinueOnError"},
        {"RollbackOnError", RollbackOnError, "RollbackOnError"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            assert.Equal(t, tt.expected, tt.mode.String())
        })
    }
}
```

#### Concurrency Tests

For thread-safety verification:

```go
func TestSyncStateBag_Concurrent(t *testing.T) {
    bag := &SyncStateBag{}
    const goroutines = 100
    
    var wg sync.WaitGroup
    wg.Add(goroutines)
    
    for i := 0; i < goroutines; i++ {
        go func(id int) {
            defer wg.Done()
            bag.Set(Key("key"), id)
            _, _ = bag.Get(Key("key"))
        }(i)
    }
    
    wg.Wait()
    // No assertions needed - race detector will catch issues
}
```

### Test Coverage Goals

- **Minimum**: 80% coverage for new code
- **Target**: 90%+ coverage for core packages
- **Focus areas**:
  - Error paths
  - Edge cases
  - Concurrency scenarios

### Running Specific Test Suites

```bash
# State management tests
go test -v -run TestSync

# Workflow tests
go test -v -run TestWorkflow

# Concurrency tests with race detector
go test -v -run Concurrent -race

# Builder tests
go test -v -run TestBuilder
```

## Code Style

### Go Formatting

Always run `gofmt` before committing:

```bash
# Format all files
gofmt -w .

# Check formatting
gofmt -l .
```

### Linting

The project uses `golangci-lint` with custom configuration (`.golangci.yml`):

```bash
# Run all linters
golangci-lint run

# Run specific linters
golangci-lint run --disable-all --enable=errcheck,govet

# Fix auto-fixable issues
golangci-lint run --fix
```

### Naming Conventions

- **Interfaces**: Describe capability (e.g., `Step`, `StateBag`, `Builder`)
- **Implementations**: Use concrete names (e.g., `defaultStep`, `SyncStateBag`)
- **Builders**: Suffix with `Builder` (e.g., `StepBuilder`, `WorkflowBuilder`)
- **Tests**: Prefix with `Test`, use descriptive names (e.g., `TestWorkflow_Execute_RollbackOnError`)

### Documentation

#### Package Documentation

Every package should have a package comment:

```go
// Package automa provides workflow orchestration primitives for composing
// and executing Steps with structured reporting, error handling and rollback support.
package automa
```

#### Function/Method Documentation

Document all exported functions:

```go
// Execute runs the workflow by executing each step in sequence.
//
// Execution behavior respects `w.executionMode`:
//   - `StopOnError`: Stop immediately when a step fails, no rollback
//   - `ContinueOnError`: Continue executing remaining steps even if one fails
//   - `RollbackOnError`: Rollback the failed step and all previously executed steps, then stop
//
// Returns a Report containing execution status and step-level reports.
func (w *workflow) Execute(ctx context.Context) *Report {
    // ...
}
```

#### Type Documentation

Document all exported types:

```go
// TypeMode defines how a workflow or step handles execution errors.
//
// Available modes:
//   - StopOnError: Stop execution immediately on first error
//   - ContinueOnError: Continue execution despite errors
//   - RollbackOnError: Rollback executed steps on error
type TypeMode int
```

### Error Handling

Use `errorx` for structured errors:

```go
// Define error types
var (
    StepExecutionError = errorx.NewType(namespace, "StepExecutionError")
)

// Create errors with context
return StepExecutionError.
    Wrap(err, "workflow %q step %q failed", w.id, step.Id()).
    WithProperty(StepIdProperty, step.Id())
```

## Contributing

### Pull Request Process

1. **Fork the repository**
2. **Create a feature branch**
3. **Make your changes**
4. **Add tests**
5. **Update documentation**
6. **Run full test suite + linter**
7. **Submit PR with clear description**

### PR Checklist

- [ ] Tests added for new functionality
- [ ] All tests pass (`go test ./...`)
- [ ] Race detector passes (`go test ./... -race`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
- [ ] Commit messages follow convention

### Code Review Guidelines

**For Authors:**
- Keep PRs focused and small
- Provide clear description and context
- Respond to feedback promptly
- Run tests locally before submitting

**For Reviewers:**
- Be constructive and specific
- Focus on design, correctness, and maintainability
- Suggest improvements, don't demand perfection
- Approve when ready, request changes when needed

## Release Process

### Versioning

Follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

### Creating a Release

1. **Update version**

   ```bash
   # Update go.mod if needed
   # Update CHANGELOG.md
   ```

2. **Tag the release**

   ```bash
   git tag -a v1.2.3 -m "Release v1.2.3"
   git push origin v1.2.3
   ```

3. **Publish release notes**

   Create GitHub release with:
   - Version number
   - Changelog excerpt
   - Breaking changes (if any)
   - Migration guide (if needed)

### Deprecation Policy

- Deprecated features remain for at least 2 minor versions
- Clear migration path provided in documentation
- Warnings logged when deprecated features are used

## Debugging

### Enabling Logging

Set logger on workflow/step:

```go
import "github.com/rs/zerolog"

logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

wf := NewWorkflowBuilder().
    WithId("debug-workflow").
    WithLogger(logger).
    Build()
```

### Common Issues

#### "Step returned nil report"

**Cause**: Step's `Execute()` returned `nil`

**Fix**: Always return a valid `Report`:

```go
func (s *myStep) Execute(ctx context.Context) *Report {
    // Do work...
    return SuccessReport(s)  // or FailureReport(s, ...)
}
```

#### Race conditions

**Symptom**: Tests fail with `-race` flag

**Debug**:
```bash
go test -race -v -run TestProblematicTest
```

**Fix**: Use proper synchronization (mutexes, channels, atomic operations)

#### Memory leaks

**Symptom**: Memory grows unbounded

**Debug**:
```bash
go test -memprofile=mem.out
go tool pprof mem.out
```

**Fix**: Ensure `lastExecutionStates` is released or disable state preservation

## Performance Profiling

### CPU Profiling

```bash
go test -cpuprofile=cpu.out -bench=.
go tool pprof cpu.out
```

### Memory Profiling

```bash
go test -memprofile=mem.out -bench=.
go tool pprof mem.out
```

### Benchmarks

Add benchmarks for performance-critical code:

```go
func BenchmarkWorkflow_Execute(b *testing.B) {
    wf := createTestWorkflow()
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        wf.Execute(ctx)
    }
}
```

## Resources

- **Go Documentation**: https://golang.org/doc/
- **Effective Go**: https://golang.org/doc/effective_go
- **Go Code Review Comments**: https://github.com/golang/go/wiki/CodeReviewComments
- **Zerolog**: https://github.com/rs/zerolog
- **Errorx**: https://github.com/joomcode/errorx
- **Testify**: https://github.com/stretchr/testify

## Getting Help

- **Issues**: https://github.com/automa-saga/automa/issues
- **Discussions**: https://github.com/automa-saga/automa/discussions
- **Documentation**: https://github.com/automa-saga/automa/tree/main/docs

## License

This project is licensed under the terms specified in the LICENSE file.
