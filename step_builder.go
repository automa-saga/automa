package automa

import (
	"context"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

// ExecuteFunc is the signature for a step's primary work function.
// It receives the execution context and the owning [Step] (for access to its
// ID, state, and logger), and must return a non-nil [Report]. Returning nil
// causes [defaultStep.Execute] to synthesise a Failure report automatically.
type ExecuteFunc func(ctx context.Context, stp Step) *Report

// RollbackFunc is the signature for a step's rollback function.
// It is called in reverse step order when a later step fails and the workflow
// is configured with [RollbackOnError]. It receives the execution context and
// the owning [Step], and must return a non-nil [Report]. Returning nil causes
// [defaultStep.Rollback] to synthesise a Failure report automatically.
// Rollback functions should be idempotent.
type RollbackFunc func(ctx context.Context, stp Step) *Report

// PrepareFunc is the signature for a step's preparation function.
// It is called before Execute and may enrich ctx (e.g. inject a request-scoped
// logger or deadline), validate preconditions, or pre-populate the step's
// local state. The returned context is forwarded to Execute and Rollback.
// A non-nil error aborts the step and is treated as a failure by the workflow.
type PrepareFunc func(ctx context.Context, stp Step) (context.Context, error)

// OnCompletionFunc is an optional callback invoked after a successful or
// skipped Execute. It receives the execution context, the owning [Step], and
// the final [Report]. When [StepBuilder.WithAsyncCallbacks] is enabled the
// callback is called in a separate goroutine with a deep-cloned report.
type OnCompletionFunc func(ctx context.Context, stp Step, report *Report)

// OnFailureFunc is an optional callback invoked after a failed Execute.
// It receives the execution context, the owning [Step], and the failure
// [Report]. When [StepBuilder.WithAsyncCallbacks] is enabled the callback is
// called in a separate goroutine with a deep-cloned report.
type OnFailureFunc func(ctx context.Context, stp Step, report *Report)

// StepBuilder is a fluent builder for constructing [Step] instances without
// implementing the [Step] interface directly.
//
// Every method returns the same *StepBuilder so calls can be chained:
//
//	step := automa.NewStepBuilder().
//	    WithId("provision-db").
//	    WithPrepare(func(ctx context.Context, stp automa.Step) (context.Context, error) {
//	        // validate config, inject logger …
//	        return ctx, nil
//	    }).
//	    WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
//	        // provision the database …
//	        return automa.SuccessReport(stp)
//	    }).
//	    WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
//	        // tear down the database …
//	        return automa.SuccessReport(stp)
//	    })
//
// Call [StepBuilder.Build] to finalise and obtain the [Step]. Build resets the
// internal state so the builder can be reused to construct another step.
//
// StepBuilder is not safe for concurrent use.
type StepBuilder struct {
	// Step is the defaultStep being assembled. It is exported only so that
	// WorkflowBuilder can access it when inheriting execution modes; callers
	// should use the With* methods rather than modifying this field directly.
	Step *defaultStep
}

// Id returns the ID that has been set on the step under construction.
// This satisfies the [Builder] interface and is also used by [WorkflowBuilder]
// to key the step in its internal builder map.
func (s *StepBuilder) Id() string {
	return s.Step.id
}

// WithId sets the unique identifier for the step being built and returns the
// builder for chaining. The ID must be non-empty; [Validate] will return an
// error if it is not set before [Build] is called.
func (s *StepBuilder) WithId(id string) *StepBuilder {
	s.Step.id = id
	return s
}

// WithLogger attaches a structured [zerolog.Logger] to the step. The logger is
// available inside Execute, Rollback, and Prepare via the Step parameter.
func (s *StepBuilder) WithLogger(logger zerolog.Logger) *StepBuilder {
	s.Step.logger = &logger
	return s
}

// WithPrepare sets the [PrepareFunc] that will be called before Execute.
// Use it to enrich the context, validate preconditions, or pre-populate
// local state. Omitting this call means no preparation is performed.
func (s *StepBuilder) WithPrepare(f PrepareFunc) *StepBuilder {
	s.Step.prepare = f
	return s
}

// WithExecute sets the [ExecuteFunc] that performs the step's primary work.
// This field is required; [Validate] returns an error if it is nil.
func (s *StepBuilder) WithExecute(f ExecuteFunc) *StepBuilder {
	s.Step.execute = f
	return s
}

// WithRollback sets the [RollbackFunc] that undoes the step's work when a
// later step fails and the workflow uses [RollbackOnError]. Omitting this call
// means rollback is a no-op (returns Skipped).
func (s *StepBuilder) WithRollback(f RollbackFunc) *StepBuilder {
	s.Step.rollback = f
	return s
}

// WithOnCompletion sets a callback that is invoked after a successful or
// skipped Execute. See [OnCompletionFunc] for the calling conventions and the
// async-callback note.
func (s *StepBuilder) WithOnCompletion(f OnCompletionFunc) *StepBuilder {
	s.Step.onCompletion = f
	return s
}

// WithOnFailure sets a callback that is invoked after a failed Execute.
// See [OnFailureFunc] for the calling conventions and the async-callback note.
func (s *StepBuilder) WithOnFailure(f OnFailureFunc) *StepBuilder {
	s.Step.onFailure = f
	return s
}

// WithAsyncCallbacks controls whether the onCompletion and onFailure hooks are
// invoked synchronously (false, the default) or in a new goroutine (true).
// When async is enabled the callback receives a deep clone of the [Report] so
// that mutations in the goroutine cannot race with the workflow.
func (s *StepBuilder) WithAsyncCallbacks(enable bool) *StepBuilder {
	s.Step.enableAsyncCallbacks = enable
	return s
}

// WithState pre-populates the [NamespacedStateBag] for the step under
// construction. This is rarely needed directly; the workflow injects state
// automatically via [Step.WithState] before calling Prepare. Use this method
// only when running a step outside a workflow or in tests.
func (s *StepBuilder) WithState(state NamespacedStateBag) *StepBuilder {
	s.Step.state = state
	return s
}

// Validate checks that the step under construction has all required fields:
//   - id must be non-empty.
//   - execute must be non-nil.
//
// It is called automatically by [Build] and [BuildAndCopy], and also by
// [WorkflowBuilder.Validate] when assembling a workflow.
func (s *StepBuilder) Validate() error {
	// Ensure that the step has a valid id and an execute function.
	if s.Step.id == "" {
		return IllegalArgument.New("step id cannot be empty")
	}

	if s.Step.execute == nil {
		return IllegalArgument.New("execute function cannot be nil")
	}

	return nil
}

// Build finalises the step under construction and returns it as a [Step].
//
// After Build returns, the builder is reset to a fresh empty [defaultStep] so
// it can be used to construct another step. The returned Step and the next
// construction cycle are fully independent.
//
// Returns an error if [Validate] fails.
func (s *StepBuilder) Build() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	finishedStep := s.Step
	s.Step = newDefaultStep()

	return finishedStep, nil
}

// BuildAndCopy finalises the step under construction, returns it as a [Step],
// and resets the builder with a copy of every field except id (which is
// cleared to force the caller to assign a new unique ID before the next Build).
//
// This is useful when creating several steps that share the same logger,
// prepare function, execute logic, or callbacks but need distinct IDs:
//
//	base := automa.NewStepBuilder().
//	    WithExecute(sharedExecuteFunc).
//	    WithRollback(sharedRollbackFunc)
//
//	step1, _ := base.WithId("step-1").BuildAndCopy()
//	step2, _ := base.WithId("step-2").BuildAndCopy()
//
// If the step has a [NamespacedStateBag] it is deep-cloned via
// [NamespacedStateBag.Clone] so the new construction cycle starts with an
// independent copy of the state. Returns an error if [Validate] fails or if
// state cloning fails.
func (s *StepBuilder) BuildAndCopy() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	finishedStep := s.Step

	s.Step = newDefaultStep()

	s.Step.id = "" // reset id to force setting a new one
	s.Step.logger = finishedStep.logger
	s.Step.prepare = finishedStep.prepare
	s.Step.execute = finishedStep.execute
	s.Step.onCompletion = finishedStep.onCompletion
	s.Step.rollback = finishedStep.rollback
	s.Step.onFailure = finishedStep.onFailure
	s.Step.enableAsyncCallbacks = finishedStep.enableAsyncCallbacks

	// Clone the NamespacedStateBag (clones local, global, and custom namespaces)
	var err error
	if finishedStep.state != nil {
		s.Step.state, err = finishedStep.state.Clone()
		if err != nil {
			return nil, errorx.IllegalState.Wrap(err, "failed to clone state for step %q", finishedStep.id)
		}
	} else {
		s.Step.state = nil
	}

	return finishedStep, nil
}

// NewStepBuilder returns a new, empty [StepBuilder] ready for configuration
// via the With* methods.
func NewStepBuilder() *StepBuilder {
	s := &StepBuilder{
		Step: newDefaultStep(),
	}

	return s
}
