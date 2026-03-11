package automa

import (
	"fmt"

	"github.com/rs/zerolog"
)

// WorkflowBuilder is a fluent builder for constructing [Workflow] instances.
//
// Call [NewWorkflowBuilder] to obtain an instance, configure it with the
// With* methods, add steps with [WorkflowBuilder.Steps] or
// [WorkflowBuilder.NamedSteps], and finalise it with [WorkflowBuilder.Build]:
//
//	wf, err := automa.NewWorkflowBuilder().
//	    WithId("deploy").
//	    WithExecutionMode(automa.RollbackOnError).
//	    Steps(
//	        automa.NewStepBuilder().WithId("step-a").WithExecute(execA),
//	        automa.NewStepBuilder().WithId("step-b").WithExecute(execB),
//	    ).
//	    Build()
//
// Step ordering is preserved in the order of Steps/NamedSteps calls.
// Duplicate step IDs are silently ignored (the first registration wins).
//
// Build resets the builder's internal [workflow] so the same WorkflowBuilder
// instance can immediately be used to construct a second, independent workflow.
//
// WorkflowBuilder is not safe for concurrent use.
type WorkflowBuilder struct {
	// workflow is the workflow being assembled. Build swaps this out with a
	// fresh default workflow after it returns.
	workflow *workflow

	// registry is an optional source of named [Builder] instances used by
	// NamedSteps. nil means NamedSteps is a no-op.
	registry Registry

	// stepSequence preserves the insertion order of step IDs so that Build
	// constructs the step list in the order the caller specified.
	stepSequence []string

	// stepBuilders is a map of step ID → Builder, populated by Steps and
	// NamedSteps. Build iterates stepSequence and looks up each Builder here.
	stepBuilders map[string]Builder
}

// Id returns the workflow ID that has been configured so far.
// This satisfies the [Builder] interface.
func (wb *WorkflowBuilder) Id() string {
	return wb.workflow.id
}

// Build validates the builder, constructs all registered steps in order,
// wires them into the workflow, and returns the finished [Step] (a *workflow).
//
// Nested workflow propagation: if any registered step produces a *workflow
// (i.e. is itself a sub-workflow), its executionMode and rollbackMode are
// overwritten with the parent's modes so that all nested workflows follow the
// same error-handling and rollback strategy as the enclosing workflow — unless
// those modes have been explicitly set on the nested builder beforehand, in
// which case they are still overwritten. Use [WorkflowBuilder.WithExecutionMode]
// and [WorkflowBuilder.WithRollbackMode] on each nested builder before adding
// it if independent modes are required.
//
// After Build returns the internal workflow is reset to a fresh default so the
// builder can be reused.
//
// Returns an error if [Validate] fails or if any step's Build fails.
func (wb *WorkflowBuilder) Build() (Step, error) {
	if err := wb.Validate(); err != nil {
		return nil, err
	}

	steps := make([]Step, 0, len(wb.stepBuilders))
	for _, stepId := range wb.stepSequence {
		builder, exists := wb.stepBuilders[stepId]
		if !exists {
			return nil, fmt.Errorf("step with id '%s' not found in builders map", stepId)
		}

		step, err := builder.Build()
		if err != nil {
			return nil, IllegalArgument.New("failed to build step '%s': %v", builder.Id(), err)
		}

		if step != nil {
			// If the step itself is a workflow, propagate the parent workflow's
			// execution and rollback modes so that nested workflows behave
			// consistently and follow the same error handling and rollback
			// strategy as the enclosing workflow.
			if wfStep, ok := step.(*workflow); ok {
				wfStep.executionMode = wb.workflow.executionMode
				wfStep.rollbackMode = wb.workflow.rollbackMode
			}

			steps = append(steps, step)
		}
	}

	wb.workflow.steps = steps
	finished := wb.workflow
	wb.workflow = newDefaultWorkflow()

	return finished, nil
}

// Steps registers one or more [Builder] instances in the order they are
// provided. Each builder is keyed by its ID; if a builder with the same ID has
// already been registered, the duplicate is silently ignored (first-wins).
//
// Steps can be called multiple times to append to the step sequence:
//
//	wb.Steps(a, b).Steps(c)  // order: a, b, c
func (wb *WorkflowBuilder) Steps(steps ...Builder) *WorkflowBuilder {
	for _, step := range steps {
		if _, exists := wb.stepBuilders[step.Id()]; exists {
			continue
		}
		wb.stepBuilders[step.Id()] = step
		wb.stepSequence = append(wb.stepSequence, step.Id())
	}
	return wb
}

// NamedSteps looks up step IDs in the configured [Registry] and registers the
// matching [Builder] instances. IDs that are not found in the registry or that
// are already registered are silently skipped. The relative order of the
// provided IDs is preserved.
//
// NamedSteps is a no-op when no registry has been configured via
// [WorkflowBuilder.WithRegistry] or when stepIds is empty.
func (wb *WorkflowBuilder) NamedSteps(stepIds ...string) *WorkflowBuilder {
	if wb.registry == nil || len(stepIds) == 0 {
		return wb
	}
	for _, id := range stepIds {
		builder := wb.registry.Of(id)
		if builder == nil {
			continue
		}
		if _, exists := wb.stepBuilders[id]; exists {
			continue
		}
		wb.stepBuilders[id] = builder
		wb.stepSequence = append(wb.stepSequence, id)
	}
	return wb
}

// Validate checks that the workflow under construction is complete and
// consistent:
//   - The workflow ID must be non-empty.
//   - At least one step must be registered.
//   - Every registered step's [Builder.Validate] must pass.
//
// Validate is called automatically by [Build]. It can also be called
// explicitly to surface configuration errors early.
func (wb *WorkflowBuilder) Validate() error {
	if wb.workflow.id == "" {
		return IllegalArgument.New("workflow id cannot be empty")
	}

	if len(wb.stepBuilders) == 0 {
		return StepNotFound.New("no steps provided for workflow")
	}

	var errs []error
	for id, builder := range wb.stepBuilders {
		if err := builder.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("step with id %s failed validation: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %v", errs)
	}
	return nil
}

// WithId sets the unique identifier for the workflow being built.
// The ID must be non-empty; [Validate] returns an error otherwise.
func (wb *WorkflowBuilder) WithId(id string) *WorkflowBuilder {
	wb.workflow.id = id
	return wb
}

// WithRegistry attaches a [Registry] that [NamedSteps] uses to resolve step
// IDs to [Builder] instances. Optional — omit if all steps are provided
// directly via [WorkflowBuilder.Steps].
func (wb *WorkflowBuilder) WithRegistry(sr Registry) *WorkflowBuilder {
	wb.registry = sr
	return wb
}

// WithLogger attaches a [zerolog.Logger] to the workflow. The logger is passed
// to the workflow's internal execution and rollback loops for structured
// diagnostics.
func (wb *WorkflowBuilder) WithLogger(logger zerolog.Logger) *WorkflowBuilder {
	wb.workflow.logger = logger
	return wb
}

// WithExecutionMode sets the [TypeMode] that controls how the workflow reacts
// when a step fails during execution:
//
//   - [StopOnError] (default) — stop immediately; no rollback.
//   - [ContinueOnError] — continue executing remaining steps; no rollback.
//   - [RollbackOnError] — stop and roll back all previously executed steps in
//     reverse order.
//
// The same mode is propagated to any nested sub-workflows at Build time unless
// those sub-workflows set their own mode explicitly.
func (wb *WorkflowBuilder) WithExecutionMode(mode TypeMode) *WorkflowBuilder {
	wb.workflow.executionMode = mode
	return wb
}

// WithRollbackMode sets the [TypeMode] that controls how the workflow reacts
// when a step's [RollbackFunc] fails during rollback:
//
//   - [StopOnError] — stop rolling back further steps on the first rollback failure.
//   - [ContinueOnError] (default) — continue rolling back remaining steps even
//     if an earlier rollback fails.
//
// This mode is only relevant when [WithExecutionMode] is [RollbackOnError].
// The same mode is propagated to nested sub-workflows at Build time.
func (wb *WorkflowBuilder) WithRollbackMode(mode TypeMode) *WorkflowBuilder {
	wb.workflow.rollbackMode = mode
	return wb
}

// WithOnCompletion sets a callback that is invoked after the workflow completes
// successfully. The callback receives the execution context, the workflow as a
// [Step], and the final [Report]. When [WithAsyncCallbacks] is enabled the
// callback runs in a new goroutine with a deep-cloned report.
func (wb *WorkflowBuilder) WithOnCompletion(f OnCompletionFunc) *WorkflowBuilder {
	wb.workflow.onCompletion = f
	return wb
}

// WithOnFailure sets a callback that is invoked after the workflow fails (i.e.
// the final report status is Failed). The callback receives the execution
// context, the workflow as a [Step], and the failure [Report]. When
// [WithAsyncCallbacks] is enabled the callback runs in a new goroutine with a
// deep-cloned report.
func (wb *WorkflowBuilder) WithOnFailure(f OnFailureFunc) *WorkflowBuilder {
	wb.workflow.onFailure = f
	return wb
}

// WithAsyncCallbacks controls whether the onCompletion and onFailure hooks are
// invoked synchronously (false, the default) or in a new goroutine (true).
// When async is enabled the callback receives a deep clone of the [Report] so
// that mutations in the goroutine cannot race with the workflow.
func (wb *WorkflowBuilder) WithAsyncCallbacks(enable bool) *WorkflowBuilder {
	wb.workflow.enableAsyncCallbacks = enable
	return wb
}

// WithPrepare sets an optional [PrepareFunc] that is called once before the
// workflow's step loop begins. Use it to enrich the context (e.g. attach a
// trace span) or initialise global state that all steps will share.
func (wb *WorkflowBuilder) WithPrepare(prepareFunc PrepareFunc) *WorkflowBuilder {
	wb.workflow.prepare = prepareFunc
	return wb
}

// WithState pre-populates the workflow's [NamespacedStateBag]. The global
// namespace of this bag is shared with all steps during execution; each step
// additionally receives its own private local namespace.
//
// This is optional — if not set, the workflow lazily initialises an empty bag
// on first access.
func (wb *WorkflowBuilder) WithState(state NamespacedStateBag) *WorkflowBuilder {
	wb.workflow.state = state
	return wb
}

// WithStatePreservation controls whether per-step state snapshots are cloned
// and stored for use during rollback (default: true).
//
// When true (default):
//   - After each step executes successfully, its [NamespacedStateBag] is
//     deep-cloned and stored keyed by step ID.
//   - During rollback each step receives the snapshot taken at its execution
//     time, so local namespaces and the exact global state at that moment are
//     available.
//   - Higher memory usage due to deep cloning after every step.
//
// When false:
//   - No cloning or storage occurs after execution.
//   - During rollback each step receives the workflow's current State() (global
//     namespace only); per-step local namespaces from execution time are lost.
//   - Lower memory overhead.
//
// Set to false when:
//   - Rollback functions are idempotent and only need global state.
//   - Rollback reads from external sources (database, files, APIs) rather than
//     the in-memory state bag.
//   - The execution mode is [StopOnError] or [ContinueOnError] so rollback is
//     never triggered.
func (wb *WorkflowBuilder) WithStatePreservation(enable bool) *WorkflowBuilder {
	wb.workflow.preserveStatesForRollback = enable
	return wb
}

// WithRollback sets an optional user-defined [RollbackFunc] for the workflow
// itself. This is called in addition to (or instead of, depending on
// configuration) the individual step rollbacks. Use it for workflow-level
// cleanup that does not belong to any single step.
func (wb *WorkflowBuilder) WithRollback(rollback RollbackFunc) *WorkflowBuilder {
	wb.workflow.rollback = rollback
	return wb
}

// NewWorkflowBuilder returns a new, empty [WorkflowBuilder] with sensible
// defaults (StopOnError execution, ContinueOnError rollback, state
// preservation enabled). Configure it with the With* methods and call
// [WorkflowBuilder.Build] when ready.
func NewWorkflowBuilder() *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow:     newDefaultWorkflow(),
		stepBuilders: make(map[string]Builder),
		stepSequence: []string{},
	}
}
