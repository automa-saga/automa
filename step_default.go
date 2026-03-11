package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// defaultStep is the concrete implementation of [Step] produced by
// [StepBuilder]. All fields are optional except id; a nil function field
// causes the corresponding lifecycle phase to be skipped (returning a
// Skipped [Report]).
//
// defaultStep is not safe for concurrent use from multiple goroutines. A
// single instance is meant to be owned and driven by exactly one workflow
// execution at a time.
type defaultStep struct {
	// id is the unique identifier of this step within its workflow.
	id string

	// logger is an optional structured logger. When nil the step does not emit
	// log lines of its own; the workflow logger is used at the workflow level.
	logger *zerolog.Logger

	// prepare is called once before Execute to enrich the context or validate
	// preconditions. nil means no preparation is needed.
	prepare PrepareFunc

	// execute is the primary work function. nil causes Execute to return a
	// Skipped report.
	execute ExecuteFunc

	// rollback undoes Execute's work. nil causes Rollback to return a Skipped
	// report.
	rollback RollbackFunc

	// onCompletion is an optional hook invoked after a successful or skipped
	// Execute. It receives a copy of the final report.
	onCompletion OnCompletionFunc

	// onFailure is an optional hook invoked after a failed Execute. It receives
	// a copy of the final report.
	onFailure OnFailureFunc

	// enableAsyncCallbacks, when true, causes onCompletion and onFailure to be
	// invoked in a new goroutine. The report passed to the goroutine is a deep
	// clone so that mutations in the callback do not race with the caller.
	enableAsyncCallbacks bool

	// state is the namespaced state bag injected by the workflow before Prepare.
	// It is lazily initialised to an empty bag on first access if nil.
	state NamespacedStateBag
}

// State returns the [NamespacedStateBag] associated with this step.
// If no bag has been set (e.g. the step is used outside a workflow), an empty
// bag with isolated local and global namespaces is created on first call and
// reused on subsequent calls.
func (s *defaultStep) State() NamespacedStateBag {
	if s.state == nil {
		// lazy initialization with empty local and global namespaces
		s.state = NewNamespacedStateBag(nil, nil)
	}
	return s.state
}

// WithState attaches the provided [NamespacedStateBag] to the step and returns
// the step itself so the call can be chained. The workflow calls this before
// Prepare to inject each step's namespaced view of the workflow state.
//
// If st is identical to the current state (same pointer), the assignment is
// skipped to avoid unnecessary work.
func (s *defaultStep) WithState(st NamespacedStateBag) Step {
	// avoid redundant assignment when same state is provided
	if s.state == st {
		return s
	}

	s.state = st
	return s
}

// Id returns the unique string identifier of this step.
func (s *defaultStep) Id() string {
	return s.id
}

// Prepare runs the optional PrepareFunc, enriches the context, and returns the
// (possibly modified) context for use by Execute and Rollback.
//
// If no PrepareFunc was configured, the input context is returned unchanged.
// A non-nil error from PrepareFunc is propagated directly; the workflow treats
// any Prepare error as a step failure.
func (s *defaultStep) Prepare(ctx context.Context) (context.Context, error) {
	preparedCtx := ctx
	if s.prepare != nil {
		c, err := s.prepare(preparedCtx, s)
		if err != nil {
			return nil, err
		}
		preparedCtx = c // use the context returned by user prepare function
	}

	return preparedCtx, nil
}

// Execute runs the step's primary work function and returns a [Report].
//
// Execution semantics:
//   - If no ExecuteFunc was configured, a Skipped report is returned immediately.
//   - If ExecuteFunc returns nil, a Failure report is generated with a
//     [StepExecutionError] explaining that a nil report was returned.
//   - If ExecuteFunc returns a failed report whose Error field is nil, the
//     field is populated with a [StepExecutionError] for safety.
//   - The final report always has ActionType set to [ActionExecute] and
//     StartTime set to the wall-clock time at the start of this call.
//   - On success or skip, [OnCompletionFunc] is invoked (asynchronously if
//     [enableAsyncCallbacks] is true).
//   - On failure, [OnFailureFunc] is invoked (asynchronously if enabled).
func (s *defaultStep) Execute(ctx context.Context) *Report {
	start := time.Now()
	if s.execute != nil {
		report := s.execute(ctx, s)
		if report == nil {
			return FailureReport(s,
				WithError(StepExecutionError.New("step %q execution returned nil report", s.id)),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		var execReport *Report
		if report.IsFailed() {
			if report.Error == nil {
				// this should not happen, but just in case
				report.Error = StepExecutionError.New("step %q failed", s.id)
			}

			execReport = FailureReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))

			s.handleFailure(ctx, execReport)

			return execReport
		}

		if report.Status == StatusSkipped {
			execReport = SkippedReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		} else {
			execReport = SuccessReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		s.handleCompletion(ctx, execReport)
		return execReport
	}

	return SkippedReport(s, WithActionType(ActionExecute), WithStartTime(start))
}

// Rollback runs the step's rollback function and returns a [Report].
//
// Rollback semantics:
//   - If no RollbackFunc was configured, a Skipped report is returned
//     immediately (no-op rollback).
//   - If RollbackFunc returns nil, a Failure report is generated with a
//     [StepExecutionError] explaining that a nil report was returned.
//   - If RollbackFunc returns a failed report whose Error field is nil, the
//     field is populated with a [StepExecutionError] for safety.
//   - The final report always has ActionType set to [ActionRollback] and
//     StartTime set to the wall-clock time at the start of this call.
//   - Rollback does not invoke the onCompletion or onFailure callbacks; those
//     hooks are reserved for the Execute phase.
//   - The report returned by the user's RollbackFunc is re-wrapped to ensure
//     consistent field population regardless of what the user returns.
func (s *defaultStep) Rollback(ctx context.Context) *Report {
	start := time.Now()
	if s.rollback != nil {
		report := s.rollback(ctx, s)
		if report == nil {
			return FailureReport(s,
				WithError(StepExecutionError.New("step %q rollback returned nil report", s.id)),
				WithActionType(ActionRollback),
				WithStartTime(start))
		}

		// we regenerate the completion report here to ensure consistency
		// in case the user-provided rollback function does not
		// follow the expected conventions
		var rollbackReport *Report
		if report.IsFailed() {
			if report.Error == nil {
				// this should not happen, but just in case
				report.Error = StepExecutionError.New("step %q rollback failed", s.id)
			}

			rollbackReport = FailureReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else if report.Status == StatusSkipped {
			rollbackReport = SkippedReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else {
			rollbackReport = SuccessReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		}

		return rollbackReport
	}

	return SkippedReport(s,
		WithActionType(ActionRollback),
		WithStartTime(start))
}

// handleCompletion invokes onCompletion after a successful or skipped Execute.
// If enableAsyncCallbacks is true, the callback runs in a new goroutine and
// receives a deep clone of the report to prevent data races. Otherwise the
// callback runs synchronously and receives the original report pointer.
func (s *defaultStep) handleCompletion(ctx context.Context, report *Report) {
	if s.onCompletion == nil {
		return
	}

	if s.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go s.onCompletion(ctx, s, clonedReport)
	} else {
		s.onCompletion(ctx, s, report)
	}
}

// handleFailure invokes onFailure after a failed Execute.
// If enableAsyncCallbacks is true, the callback runs in a new goroutine and
// receives a deep clone of the report to prevent data races. Otherwise the
// callback runs synchronously and receives the original report pointer.
func (s *defaultStep) handleFailure(ctx context.Context, report *Report) {
	if s.onFailure == nil {
		return
	}

	if s.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go s.onFailure(ctx, s, clonedReport)
	} else {
		s.onFailure(ctx, s, report)
	}
}

// newDefaultStep returns a zero-valued *defaultStep ready to be populated by
// [StepBuilder]. All function fields are nil (no-op) and state is nil
// (lazily initialised on first access).
func newDefaultStep() *defaultStep {
	return &defaultStep{}
}
