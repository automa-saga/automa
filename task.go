package automa

import "github.com/joomcode/errorx"

type Task struct {
	ID string

	next Step
	prev Step

	// user needs to implement these methods
	Run      func(ctx *Context) error
	Skip     func(ctx *Context) bool
	Rollback func(ctx *Context) error
}

// GetID returns the step ID
func (s *Task) GetID() string {
	return s.ID
}

// SetNext sets the next step of the workflow to be able to move in the forward direction on success
func (s *Task) SetNext(next Step) {
	s.next = next
}

// SetPrev sets the previous step of the workflow to be able to move in the backward direction on error
func (s *Task) SetPrev(prev Step) {
	s.prev = prev
}

// GetNext returns the step to be used to move in the forward direction
func (s *Task) GetNext() Step {
	return s.next
}

// GetPrev returns the step to be used to move in the backward direction
func (s *Task) GetPrev() Step {
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
func (s *Task) Execute(ctx *Context) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	prevSuccess := ctx.getPrevSuccess()
	if prevSuccess == nil {
		return nil, errorx.IllegalArgument.New("previous success event cannot be nil for forward execution")
	}

	report := NewStepReport(s.GetID(), RunAction)

	// If the step's Run method is not defined or if Skip is defined and returns true, we skip the run
	if s.Run == nil || (s.Skip != nil && s.Skip(ctx)) {
		report.Status = StatusSkipped
		if s.next != nil {
			return s.next.Execute(ctx.SetValue(KeyPrevSuccess, NewSkippedRun(prevSuccess, report)))
		}

		prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)
		return &prevSuccess.workflowReport, nil
	}

	// Execute the run logic
	err := s.Run(ctx)
	if err != nil {
		return s.Reverse(ctx.SetValue(KeyPrevFailure, NewFailedRun(prevSuccess, err, report)))
	}

	// If run is successful, we proceed to the next step's execution
	return s.runNext(ctx, prevSuccess, report)
}

// Reverse implements controller logic for automa.Step interface
func (s *Task) Reverse(ctx *Context) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	prevFailure := ctx.getPrevFailure()
	if prevFailure == nil {
		return nil, errorx.IllegalArgument.New("previous failure event cannot be nil for rollback")
	}

	report := NewStepReport(s.GetID(), RollbackAction)

	// If the step's Rollback method is not defined or if Skip is defined and returns true, we skip the rollback
	if s.Rollback == nil || (s.Skip != nil && s.Skip(ctx)) {
		report.Status = StatusSkipped
		if s.prev != nil {
			return s.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewSkippedRollback(prevFailure, report)))
		}

		prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)
		return &prevFailure.workflowReport, prevFailure.err
	}

	// Execute the rollback logic
	err := s.Rollback(ctx)
	if err != nil {
		report.FailureReason = StepRollbackFailed.Wrap(err, "step rollback failed")

		if s.prev != nil {
			return s.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewFailedRollback(prevFailure, err, report)))
		}

		prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)
		return &prevFailure.workflowReport, prevFailure.err
	}

	// If rollback is successful, we proceed to the previous step's rollback
	return s.rollbackPrev(ctx, prevFailure, report)
}

// runNext is a helper method to report that current step has been successful and trigger next step's execution.
// It marks the current step as StatusSuccess.
func (s *Task) runNext(ctx *Context, prevSuccess *Success, report *StepReport) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	if prevSuccess == nil {
		return nil, errorx.IllegalArgument.New("previous success event cannot be nil for forward execution")
	}

	if report == nil {
		report = NewStepReport(s.GetID(), RunAction)
	}

	if s.next != nil {
		return s.next.Execute(ctx.SetValue(KeyPrevSuccess, NewSuccess(prevSuccess, report)))
	}

	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return &prevSuccess.workflowReport, nil
}

// rollbackPrev is a helper method to report that current rollback step has been executed and trigger previous step's rollback.
// It marks the current step as StatusFailed.
func (s *Task) rollbackPrev(ctx *Context, prevFailure *Failure, report *StepReport) (*WorkflowReport, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	if prevFailure == nil {
		return nil, errorx.IllegalArgument.New("previous failure event cannot be nil for rollback")
	}

	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.prev != nil {
		return s.prev.Reverse(ctx.SetValue(KeyPrevFailure, NewFailure(prevFailure, report)))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return &prevFailure.workflowReport, prevFailure.err
}
