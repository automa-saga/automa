package automa

import "github.com/joomcode/errorx"

// Task implements the automa.Step interface and represents a workflow step.
// Each Task can execute (Run) and rollback (Rollback) its operation.
// Optionally, a Skip function can be defined to conditionally skip execution or rollback.
type Task struct {
	ID string // Unique identifier for the step.

	next Step // Reference to the next step in the workflow (forward direction).
	prev Step // Reference to the previous step in the workflow (backward direction).

	// User-defined execution logic for the step.
	// If not specified, the operation will be skipped.
	Run      func(ctx *Context) error
	Rollback func(ctx *Context) error

	// Optional function to determine if the step should be skipped.
	Skip func(ctx *Context) bool
}

// GetID returns the unique identifier of the step.
func (t *Task) GetID() string {
	return t.ID
}

// SetNext sets the next step in the workflow for forward progression.
func (t *Task) SetNext(next Step) {
	t.next = next
}

// SetPrev sets the previous step in the workflow for backward progression (rollback).
func (t *Task) SetPrev(prev Step) {
	t.prev = prev
}

// GetNext returns the next step for forward execution.
func (t *Task) GetNext() Step {
	return t.next
}

// GetPrev returns the previous step for backward execution (rollback).
func (t *Task) GetPrev() Step {
	return t.prev
}

// Reset clears the next and previous step references.
// Useful when reusing the step in a different workflow.
func (t *Task) Reset() Step {
	t.next = nil
	t.prev = nil
	return t
}

// Execute runs the step's forward logic.
// If Run is not defined or Skip returns true, the step is skipped.
// On error, triggers rollback via Reverse. On success, proceeds to the next step.
func (t *Task) Execute(ctx *Context) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	prevSuccess := ctx.getPrevSuccess()
	if prevSuccess == nil {
		return nil, errorx.IllegalArgument.New("previous success event cannot be nil for forward execution")
	}

	report := NewStepReport(t.GetID(), RunAction)

	// Skip execution if Run is not defined or Skip returns true.
	if t.Run == nil || (t.Skip != nil && t.Skip(ctx)) {
		report.Status = StatusSkipped
		if t.next != nil {
			// Proceed to next step if available.
			return t.next.Execute(ctx.SetValue(KeyPrevSuccess, NewSkippedRunEvent(prevSuccess, report)))
		}
		// Append skipped report and finish.
		prevSuccess.WorkflowReport.Append(report, RunAction, StatusSkipped)
		return &prevSuccess.WorkflowReport, nil
	}

	// Execute the Run logic.
	err := t.Run(ctx)
	if err != nil {
		// On error, trigger rollback.
		return t.Reverse(ctx.SetValue(KeyPrevFailure, NewFailedRunEvent(prevSuccess, err, report)))
	}

	// On success, proceed to next step.
	return t.runNext(ctx, prevSuccess, report)
}

// Reverse runs the step's rollback logic.
// If Rollback is not defined or Skip returns true, the rollback is skipped.
// On error, triggers rollback of previous steps. On success, proceeds to previous step's rollback.
func (t *Task) Reverse(ctx *Context) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	prevFailure := ctx.getPrevFailure()
	if prevFailure == nil {
		return nil, errorx.IllegalArgument.New("previous failure event cannot be nil for rollback")
	}

	report := NewStepReport(t.GetID(), RollbackAction)

	// Skip rollback if Rollback is not defined or Skip returns true.
	if t.Rollback == nil || (t.Skip != nil && t.Skip(ctx)) {
		report.Status = StatusSkipped
		if t.prev != nil {
			// Proceed to previous step's rollback if available.
			return t.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewSkippedRollbackEvent(prevFailure, report)))
		}
		// Append skipped report and finish.
		prevFailure.WorkflowReport.Append(report, RollbackAction, StatusSkipped)
		return &prevFailure.WorkflowReport, prevFailure.Err
	}

	// Execute the Rollback logic.
	err := t.Rollback(ctx)
	if err != nil {
		report.FailureReason = StepRollbackFailed.Wrap(err, "step rollback failed")
		if t.prev != nil {
			// On error, trigger rollback of previous steps.
			return t.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewFailedRollbackEvent(prevFailure, err, report)))
		}
		// Append failed report and finish.
		prevFailure.WorkflowReport.Append(report, RollbackAction, StatusFailed)
		return &prevFailure.WorkflowReport, prevFailure.Err
	}

	// On success, proceed to previous step's rollback.
	return t.rollbackPrev(ctx, prevFailure, report)
}

// runNext reports the current step as successful and triggers the next step's execution.
// Marks the current step as StatusSuccess.
func (t *Task) runNext(ctx *Context, prevSuccess *SuccessEvent, report *StepReport) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}
	if prevSuccess == nil {
		return nil, errorx.IllegalArgument.New("previous success event cannot be nil for forward execution")
	}
	if report == nil {
		report = NewStepReport(t.GetID(), RunAction)
	}

	if t.next != nil {
		// Proceed to next step.
		return t.next.Execute(ctx.SetValue(KeyPrevSuccess, NewSuccessEvent(prevSuccess, report)))
	}

	// Append success report and finish.
	prevSuccess.WorkflowReport.Append(report, RunAction, StatusSuccess)
	return &prevSuccess.WorkflowReport, nil
}

// rollbackPrev reports the current rollback step as executed and triggers previous step's rollback.
// Marks the current step as StatusSuccess.
func (t *Task) rollbackPrev(ctx *Context, prevFailure *FailureEvent, report *StepReport) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}
	if prevFailure == nil {
		return nil, errorx.IllegalArgument.New("previous failure event cannot be nil for rollback")
	}
	if report == nil {
		report = NewStepReport(t.GetID(), RollbackAction)
	}

	if t.prev != nil {
		// Proceed to previous step's rollback.
		return t.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewFailureEvent(prevFailure, report)))
	}

	// Append success report and finish.
	prevFailure.WorkflowReport.Append(report, RollbackAction, StatusSuccess)
	return &prevFailure.WorkflowReport, prevFailure.Err
}
