# Automa Architecture

This document describes the architecture and design principles of the automa workflow orchestration framework.

## Overview

Automa is a workflow orchestration framework designed for composing and executing automated steps with structured reporting, error handling, and rollback support. It follows a **builder pattern** for construction and provides **flexible execution modes** for different error handling strategies.

## Core Components

### 1. Step

The fundamental unit of work in automa.

```
┌─────────────────────────────────────────┐
│              Step Interface             │
├─────────────────────────────────────────┤
│  + Id() string                          │
│  + Prepare(ctx) (context, error)        │
│  + Execute(ctx) *Report                 │
│  + Rollback(ctx) *Report                │
│  + State() NamespacedStateBag           │
│  + WithState(s) Step                    │
├─────────────────────────────────────────┤
│  Implementations:                       │
│  - defaultStep (ordinary step)          │
│  - workflow (composite step)            │
└─────────────────────────────────────────┘
```

**Responsibilities:**
- Execute business logic
- Manage step-specific state
- Provide rollback capability
- Report execution status

**Lifecycle:**
1. **Prepare**: Initialize context and state
2. **Execute**: Perform the actual work
3. **Rollback**: Undo changes if needed

### 2. Workflow

A composite step that orchestrates multiple steps in sequence.

```
┌────────────────────────────────────────────────┐
│              Workflow                          │
├────────────────────────────────────────────────┤
│  - id: string                                  │
│  - steps: []Step                               │
│  - state: NamespacedStateBag                   │
│  - executionMode: TypeMode                     │
│  - rollbackMode: TypeMode                      │
│  - preserveStatesForRollback: bool             │
│  - lastExecutionStates: map[string]NSB         │
├────────────────────────────────────────────────┤
│  + Execute(ctx) *Report                        │
│  + Rollback(ctx) *Report                       │
│  + State() NamespacedStateBag                  │
├────────────────────────────────────────────────┤
│  Callbacks:                                    │
│  - prepare: PrepareFunc                        │
│  - onCompletion: OnCompletionFunc              │
│  - onFailure: OnFailureFunc                    │
│  - rollback: RollbackFunc                      │
└────────────────────────────────────────────────┘
```

**Key Features:**
- Sequential execution of steps
- Configurable error handling (StopOnError, ContinueOnError, RollbackOnError)
- State isolation for sub-workflows
- State snapshot preservation for rollback
- Async callbacks support

### 3. State Management

Automa uses a **namespaced state bag** design for flexible state management.

```
┌─────────────────────────────────────────────────┐
│        NamespacedStateBag Interface             │
├─────────────────────────────────────────────────┤
│  + Local() StateBag                             │
│  + Global() StateBag                            │
│  + WithNamespace(name) StateBag                 │
│  + Clone() (NamespacedStateBag, error)          │
│  + Merge(other) (NamespacedStateBag, error)     │
└─────────────────────────────────────────────────┘
                        │
                        │ implements
                        ▼
┌─────────────────────────────────────────────────┐
│       SyncNamespacedStateBag                    │
├─────────────────────────────────────────────────┤
│  - local: StateBag                              │
│  - global: StateBag                             │
│  - custom: map[string]StateBag                  │
│  - mu: sync.RWMutex                             │
│  - localOnce: sync.Once                         │
├─────────────────────────────────────────────────┤
│  Thread-safe implementation                     │
│  Lazy initialization of local namespace         │
└─────────────────────────────────────────────────┘
```

**Namespace Types:**

1. **Local**: Step-private state (isolated, not visible to other steps)
2. **Global**: Workflow-shared state (visible to all steps)
3. **Custom**: Named namespaces for specific use cases

**State Flow:**

```
Workflow Execution:
┌────────────┐
│  Workflow  │ Global State (shared)
└─────┬──────┘
      │
      ├─────► Step1 ──► Local State (isolated)
      │                 Global State (shared reference)
      │
      ├─────► Step2 ──► Local State (isolated)
      │                 Global State (shared reference)
      │
      └─────► Sub-Workflow ──► Global State (cloned)
                               Local State (isolated)
```

### 4. Report

Structured execution results with metadata and error information.

```
┌─────────────────────────────────────────┐
│              Report                     │
├─────────────────────────────────────────┤
│  + Id: string                           │
│  + Status: TypeStatus                   │
│  + Action: TypeAction                   │
│  + StartTime: time.Time                 │
│  + EndTime: time.Time                   │
│  + Duration: time.Duration              │
│  + Err: error                           │
│  + Meta: StateBag                       │
│  + Steps: []*Report                     │
│  + Rollback: *Report                    │
│  + WorkflowId: string                   │
│  + StepId: string                       │
└─────────────────────────────────────────┘
```

**Report Hierarchy:**

```
Workflow Report
├── Step1 Report
│   └── Rollback Report (if rolled back)
├── Step2 Report
│   └── Rollback Report (if rolled back)
└── Sub-Workflow Report
    ├── Sub-Step1 Report
    └── Sub-Step2 Report
```

## Execution Modes

### StopOnError

Stops execution immediately when a step fails. No rollback is performed.

```
Step1 ✓ → Step2 ✗ → [STOP]
          Step3 (not executed)
```

### ContinueOnError

Continues executing remaining steps even if one fails.

```
Step1 ✓ → Step2 ✗ → Step3 ✓ → Step4 ✗ → [COMPLETE]
```

### RollbackOnError (Default)

Rolls back all previously executed steps when a step fails, then stops.

```
Step1 ✓ → Step2 ✓ → Step3 ✗ → [ROLLBACK]
                               ↓
Step2 Rollback ✓ ← Step1 Rollback ✓
```

**Rollback includes failed step:**
- Failed step's `Rollback()` is called to clean up partial work
- All successfully executed steps are rolled back in reverse order
- Each step receives its state snapshot from execution time

## State Snapshot Design

### With State Preservation Enabled (Default)

```
Execution Phase:
Step1.Execute() → Clone(Step1.State()) → stepStates["step1"]
Step2.Execute() → Clone(Step2.State()) → stepStates["step2"]
Step3.Execute() → Clone(Step3.State()) → stepStates["step3"]

Rollback Phase:
Step3.WithState(stepStates["step3"]) → Step3.Rollback()
Step2.WithState(stepStates["step2"]) → Step2.Rollback()
Step1.WithState(stepStates["step1"]) → Step1.Rollback()
```

**Benefits:**
- Deterministic rollback (each step sees its execution-time state)
- Immutable snapshots (later steps can't mutate earlier snapshots)
- Safe for complex workflows with state mutations

**Cost:**
- Memory: O(steps × state_size)
- CPU: Clone overhead for each step

### With State Preservation Disabled

```
Execution Phase:
Step1.Execute() → [no snapshot]
Step2.Execute() → [no snapshot]
Step3.Execute() → [no snapshot]

Rollback Phase:
Step3.WithState(currentWorkflowState) → Step3.Rollback()
Step2.WithState(currentWorkflowState) → Step2.Rollback()
Step1.WithState(currentWorkflowState) → Step1.Rollback()
```

**Benefits:**
- Zero memory overhead
- No clone CPU cost

**Tradeoffs:**
- Rollback uses current state (may have been mutated)
- Less deterministic for complex workflows

## Thread Safety

### SyncNamespacedStateBag

Thread-safe via `sync.RWMutex`:

- **Read operations** (`Local()`, `Global()`, `WithNamespace()`): `RLock`
- **Write operations** (`Merge()`, custom namespace creation): `Lock`
- **Clone operations**: `RLock` (reads all fields atomically)
- **Lazy initialization**: `sync.Once` (ensures single init)

### Workflow Execution

**Not thread-safe** - designed for single-execution per instance:

- Each workflow instance is executed by **one goroutine**
- Steps execute **sequentially** (not concurrently)
- Async callbacks operate on **cloned reports** (safe)

**Best Practice**: Create new workflow instance for concurrent executions

## Builder Pattern

Automa uses builders for fluent API construction:

```go
// StepBuilder
step := automa.NewStepBuilder().
    WithId("my-step").
    WithExecute(executeFunc).
    WithRollback(rollbackFunc).
    Build()

// WorkflowBuilder
wf := automa.NewWorkflowBuilder().
    WithId("my-workflow").
    WithExecutionMode(automa.RollbackOnError).
    WithStatePreservation(true).
    Steps(step1, step2, step3).
    Build()
```

**Benefits:**
- Fluent, readable API
- Compile-time type safety
- Validation before construction
- Immutable after build

## Error Handling Strategy

### Errorx Integration

Automa uses `github.com/joomcode/errorx` for structured errors:

```go
var (
    StepExecutionError = errorx.NewType(namespace, "StepExecutionError")
    IllegalArgument    = errorx.NewType(namespace, "IllegalArgument")
    IllegalState       = errorx.NewType(namespace, "IllegalState")
)
```

**Benefits:**
- Error types with properties
- Stack traces
- Error wrapping and context

### Error Propagation

```
Step Error → Report.Err → Workflow Report.Err
                        ↓
                  OnFailure Callback
                        ↓
                  Return to Caller
```

## Design Principles

### 1. Composition Over Inheritance

Workflows are steps, enabling nested composition:

```go
subWorkflow := NewWorkflowBuilder().Steps(s1, s2).Build()
mainWorkflow := NewWorkflowBuilder().Steps(s3, subWorkflow, s4).Build()
```

### 2. Explicit State Management

State is explicitly passed and managed:

- Steps don't implicitly share state
- Namespaces provide clear isolation/sharing semantics
- Sub-workflows get cloned state (cannot mutate parent)

### 3. Fail-Safe Defaults

- State preservation: **enabled** (safer)
- Execution mode: **RollbackOnError** (safer)
- Rollback mode: **ContinueOnError** (complete rollback)

### 4. Extensibility

- Custom steps via `Step` interface
- Custom state bags via `StateBag` interface
- Custom error types via `errorx`
- Callbacks for custom behavior

## Memory Model

### Workflow Instance

```
Workflow (heap-allocated)
├── State: NamespacedStateBag (shared reference)
│   ├── Local: StateBag (lazy-allocated)
│   ├── Global: StateBag (heap-allocated)
│   └── Custom: map[string]StateBag (heap-allocated)
├── Steps: []Step (slice of interfaces)
└── lastExecutionStates: map[string]NamespacedStateBag
    └── [step-id]: NamespacedStateBag (cloned snapshots)
```

**Memory Growth:**

- **Without state preservation**: O(1) - constant overhead
- **With state preservation**: O(steps × state_size) - linear growth

### Garbage Collection

Workflow instances are GC'd when:

1. No references remain
2. `lastExecutionStates` can be GC'd
3. Report references are released

**Long-running workflows:** Consider disabling state preservation if memory is constrained.

## Concurrency Model

### Sequential Execution

```
┌──────────┐
│  Caller  │
└────┬─────┘
     │
     ▼
┌──────────────┐
│  Workflow    │ (single goroutine)
│  Execute()   │
└────┬─────────┘
     │
     ├──► Step1.Execute() (sequential)
     │
     ├──► Step2.Execute() (sequential)
     │
     └──► Step3.Execute() (sequential)
```

### Async Callbacks (Optional)

```
Step.Execute() completes
     │
     ├──► OnCompletion(cloned_report) ← async goroutine
     │
     └──► Continue to next step (main goroutine)
```

**Enabled via:** `WithAsyncCallbacks(true)`

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Execute (n steps) | O(n) | O(n) with preservation, O(1) without |
| Rollback (n steps) | O(n) | O(1) |
| State Clone | O(state_size) | O(state_size) |
| State Merge | O(keys) | O(keys) |
| Report Generation | O(1) | O(1) |

## Extension Points

### Custom Steps

```go
type MyCustomStep struct {
    id string
    state NamespacedStateBag
}

func (s *MyCustomStep) Id() string { return s.id }
func (s *MyCustomStep) Execute(ctx context.Context) *Report { /* custom logic */ }
// ... implement other Step methods
```

### Custom State Bags

```go
type MyStateBag struct {
    // custom implementation
}

func (s *MyStateBag) Get(key Key) (interface{}, bool) { /* ... */ }
// ... implement other StateBag methods
```

### Custom Reporters

Use `Meta` field in reports to attach custom metadata:

```go
report := automa.SuccessReport(step,
    automa.WithMetadata(map[string]string{
        "custom_metric": "value",
        "trace_id": "abc123",
    }))
```

## Best Practices

1. **Use namespaced state wisely**
   - Local for step-private data
   - Global for shared configuration
   - Custom for domain-specific isolation

2. **Enable state preservation for critical workflows**
   - Ensures deterministic rollback
   - Acceptable overhead for most workflows

3. **Disable state preservation for high-volume workflows**
   - Reduces memory pressure
   - Trade determinism for performance

4. **Handle errors in steps**
   - Return structured `Report` objects
   - Use `errorx` for rich error context
   - Don't panic unless unrecoverable

5. **Keep workflow instances short-lived**
   - Create per-execution, don't reuse
   - Avoids state pollution
   - Enables garbage collection

6. **Test rollback logic**
   - Verify idempotency
   - Check partial execution cleanup
   - Validate state restoration

## Future Enhancements

Potential areas for extension:

- [ ] Parallel step execution (fan-out/fan-in)
- [ ] Conditional step execution (skip based on conditions)
- [ ] Step retry mechanisms with backoff
- [ ] Workflow pause/resume capabilities
- [ ] Event-driven step triggers
- [ ] Workflow versioning and migration
- [ ] Distributed workflow execution
- [ ] Workflow visualization and debugging tools
