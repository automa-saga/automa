package automa

// Do is a func definition to contain the run logic
//
// It provides a simpler method signature for the run logic of a Step
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type Do func(ctx *Context) error

// Undo is a func definition to contain the compensating logic
//
// It provides a simpler method signature for the run logic of a Step
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type Undo func(ctx *Context) error

type Task struct {
	ID string

	next Forward
	prev Backward

	Run      func(ctx *Context) error
	Skip     func(ctx *Context) bool
	Rollback func(ctx *Context) error
}

// GetID returns the step ID
func (s *Task) GetID() string {
	return s.ID
}

// SetNext sets the next step of the workflow to be able to move in the forward direction on success
func (s *Task) SetNext(next Forward) {
	s.next = next
}

// SetPrev sets the previous step of the workflow to be able to move in the backward direction on error
func (s *Task) SetPrev(prev Backward) {
	s.prev = prev
}

// GetNext returns the step to be used to move in the forward direction
func (s *Task) GetNext() Forward {
	return s.next
}

// GetPrev returns the step to be used to move in the backward direction
func (s *Task) GetPrev() Backward {
	return s.prev
}

func (s *Task) Reset() Step {
	// Resetting the next and previous steps to nil
	// This is useful when the step is reused in a different workflow
	s.next = nil
	s.prev = nil
	return s
}

// Execute implements controller logic for automa.Step interface
// This is a wrapper function to help simplify Step implementations
// Note that user may implement Run method in order to change the control logic as required.
func (s *Task) Execute(ctx *Context, prevSuccess *Success) (WorkflowReport, error) {
	report := NewStepReport(s.GetID(), RunAction)

	if s.Run == nil || (s.Skip != nil && s.Skip(ctx)) {
		report.Status = StatusSkipped
		return s.skipRun(ctx, prevSuccess, report)
	}

	err := s.Run(ctx)
	if err != nil {
		return s.Reverse(ctx, NewFailedRun(prevSuccess, err, report))
	}

	return s.runNext(ctx, prevSuccess, report)
}

// Reverse implements controller logic for automa.Step interface
// This is a wrapper function to help simplify Step implementations
// Note that user may implement Rollback method in order to change the control logic as required.
func (s *Task) Reverse(ctx *Context, prevFailure *Failure) (WorkflowReport, error) {
	report := NewStepReport(s.GetID(), RollbackAction)

	if s.Rollback == nil || (s.Skip != nil && s.Skip(ctx)) {
		report.Status = StatusSkipped
		return s.skipRollback(ctx, prevFailure, report)
	}

	err := s.Rollback(ctx)
	if err != nil {
		return s.failedRollback(ctx, prevFailure, err, report)
	}

	return s.rollbackPrev(ctx, prevFailure, report)
}

// skipRun is a helper method to report that current step has been skipped and trigger next step's execution
// It marks the current step as StatusSkipped
func (s *Task) skipRun(ctx *Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RunAction)
	}

	if s.next != nil {
		return s.next.Execute(ctx, NewSkippedRun(prevSuccess, report))
	}

	prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)

	return prevSuccess.workflowReport, nil
}

// skippedRollback is a helper method to report that current step's rollback has been skipped and trigger previous step's rollback
// It marks the current step as StatusSkipped
func (s *Task) skipRollback(ctx *Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.prev != nil {
		return s.prev.Reverse(ctx, NewSkippedRollback(prevFailure, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)

	return prevFailure.workflowReport, prevFailure.err
}

// failedRollback is a helper method to report that current step's rollback has failed and trigger previous step's rollback
// It marks the current step RollbackAction as StatusFailed
func (s *Task) failedRollback(ctx *Context, prevFailure *Failure, err error, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	report.FailureReason = StepRollbackFailed.Wrap(err, "step rollback failed")

	if s.prev != nil {
		return s.prev.Reverse(ctx, NewFailedRollback(prevFailure, err, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)

	return prevFailure.workflowReport, prevFailure.err
}

// runNext is a helper method to report that current step has been successful and trigger next step's execution
// It marks the current step as StatusSuccess
func (s *Task) runNext(ctx *Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RunAction)
	}

	if s.next != nil {
		return s.next.Execute(ctx, NewSuccess(prevSuccess, report))
	}

	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return prevSuccess.workflowReport, nil
}

// rollbackPrev is a helper method to report that current rollback step has been executed and trigger previous step's rollback
// It marks the current step as StatusFailed
func (s *Task) rollbackPrev(ctx *Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.prev != nil {
		return s.prev.Reverse(ctx, NewFailure(prevFailure, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return prevFailure.workflowReport, prevFailure.err
}

// RegisterSaga register saga logic for run and undo functions with simpler method signature.
// This method usage is optional, but it may help user to set the Run and Rollback methods with custom methods after
// creating the Task instance.
func (s *Task) RegisterSaga(run Do, undo Undo) *Task {
	s.Run = run
	s.Rollback = undo

	return s
}
