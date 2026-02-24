# State Preservation Configuration

The automa workflow framework supports optional state preservation for rollback scenarios. This feature can be configured to balance between rollback capability and memory usage.

## Default Behavior (State Preservation Enabled)

By default, state preservation is **enabled**. After each step executes, its state is cloned and stored for potential rollback:

```go
wb := automa.NewWorkflowBuilder().WithId("my-workflow")
// State preservation is enabled by default
wb.Steps(step1, step2, step3)

wf, _ := wb.Build()
report := wf.Execute(ctx)

// Later: Manual rollback is possible using preserved state snapshots
wf.Rollback(ctx)
```

## Disabling State Preservation (Memory Optimization)

For workflows that don't need rollback capability or have many steps with large state, you can disable state preservation to reduce memory overhead:

```go
wb := automa.NewWorkflowBuilder().
    WithId("my-workflow").
    WithStatePreservation(false)  // Disable state preservation
    
wb.Steps(step1, step2, step3)

wf, _ := wb.Build()
report := wf.Execute(ctx)

// Note: Manual Rollback() will use current workflow state, not per-step snapshots
```

## When to Disable State Preservation

Consider disabling state preservation when:

1. **High step count**: Workflows with hundreds of steps (memory grows linearly)
2. **Large state**: Each step's state contains multi-MB of data
3. **Long-running workflows**: Workflow instances remain in memory for hours/days
4. **No rollback needed**: You never call `Rollback()` manually after execution
5. **Only automatic rollback**: You rely solely on `RollbackOnError` mode during execution (which still works with preservation disabled, using non-cloned state references)

## Memory Impact

### With State Preservation Enabled (Default)

```
Memory = (number of steps) × (average state size per step) × 2
         ^^^^^^^^^^^^^^^^     ^^^^^^^^^^^^^^^^^^^^^^^^^     ^
         |                     |                             |
         Step count            State size                    Clone overhead
```

Example: 100 steps × 10KB state = ~2MB total

### With State Preservation Disabled

```
Memory = 0 (no state snapshots stored)
```

## Rollback Behavior

### State Preservation Enabled
- Each step's `Rollback()` receives its **exact state snapshot** from when it executed
- **Deterministic**: Rollback sees the same state regardless of later mutations
- **Safe**: No risk of rollback operating on stale/mutated state

### State Preservation Disabled
- Automatic rollback during execution (via `RollbackOnError`) still works but uses **current state references** (not clones)
- Manual `Rollback()` calls use the **current workflow state**
- **Risk**: If state is mutated between execution and rollback, rollback may see inconsistent data

## Recommendations

1. **Keep enabled for most workflows** (default behavior is safe)
2. **Disable only when memory is a concern** and you understand the tradeoffs
3. **Profile your workflow** to determine actual memory usage before optimizing
4. **Document the decision** if you disable preservation (so maintainers understand why)

## Example: High-Volume Workflow

```go
// Workflow with 1000 steps, each storing 100KB of state
// With preservation: ~200MB memory overhead
// Without preservation: ~0MB memory overhead

wb := automa.NewWorkflowBuilder().
    WithId("high-volume-workflow").
    WithStatePreservation(false).  // Disable to reduce memory
    WithExecutionMode(automa.StopOnError)  // Don't need rollback anyway
    
// ... add 1000 steps ...

wf, _ := wb.Build()
report := wf.Execute(ctx)
// No manual rollback needed, so disabled preservation is fine
```
