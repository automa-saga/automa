package automa

import (
	"context"
	"github.com/cockroachdb/errors"
)

// SagaRun is a func definition to contain the run logic
//
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type SagaRun func(ctx context.Context) (skipped bool, err error)

// SagaUndo is a func definition to contain the compensating logic
//
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type SagaUndo func(ctx context.Context) (skipped bool, err error)

// Step is the kernel for AtomicStep implementation containing SagaRun and SagaUndo function
// It is to be used as inheritance by composition pattern by actual Step implementations
// If the saga methods are not registered, then Step will skip those operations during invocation of Run and Rollback
// Note that user may override the Run and Rollback methods in the actual implementation in order to change the control logic
type Step struct {
	ID   string
	Next Forward
	Prev Backward

	// holder of saga methods to be executed during Run and Rollback method of the AtomicStep
	run      SagaRun
	rollback SagaUndo
}

// RegisterSaga register saga logic for run and undo in order to leverage the default controller logic for Run and Rollback
// This is just a helper function where user would like to use the default Run and Rollback logic.
// This method usage is optional and user is free to implement Run and Rollback method of AtomicStep as they wish.
func (s *Step) RegisterSaga(run SagaRun, undo SagaUndo) *Step {
	s.run = run
	s.rollback = undo

	return s
}

// GetID returns the step ID
func (s *Step) GetID() string {
	return s.ID
}

// SetNext sets the next step of the Workflow to be able to move in the forward direction on success
func (s *Step) SetNext(next Forward) {
	s.Next = next
}

// SetPrev sets the previous step of the Workflow to be able to move in the backward direction on error
func (s *Step) SetPrev(prev Backward) {
	s.Prev = prev
}

// GetNext returns the step to be used to move in the forward direction
func (s *Step) GetNext() Forward {
	return s.Next
}

// GetPrev returns the step to be used to move in the backward direction
func (s *Step) GetPrev() Backward {
	return s.Prev
}

// Run implements Run controller logic for automa.AtomicStep interface
// This is a wrapper function to help simplify AtomicStep implementations
// Note that user may implement Run method in order to change the control logic as required.
func (s *Step) Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error) {
	report := NewStepReport(s.GetID(), RunAction)

	if s.run == nil {
		return s.SkippedRun(ctx, prevSuccess, report)
	}

	skipped, err := s.run(ctx)
	if err != nil {
		return s.Rollback(ctx, NewFailedRun(ctx, prevSuccess, err, report))
	}

	if skipped {
		return s.SkippedRun(ctx, prevSuccess, report)
	}

	return s.RunNext(ctx, prevSuccess, report)
}

// Rollback implements Rollback controller logic for automa.AtomicStep interface
// This is a wrapper function to help simplify AtomicStep implementations
// Note that user may implement Rollback method in order to change the control logic as required.
func (s *Step) Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, error) {
	report := NewStepReport(s.GetID(), RollbackAction)

	if s.rollback == nil {
		return s.SkippedRollback(ctx, prevFailure, report)
	}

	skipped, err := s.rollback(ctx)
	if err != nil {
		return s.FailedRollback(ctx, prevFailure, err, report)
	}

	if skipped {
		return s.SkippedRollback(ctx, prevFailure, report)
	}

	return s.RollbackPrev(ctx, prevFailure, report)
}

// SkippedRun is a helper method to report that current step has been skipped and trigger next step's execution
// It marks the current step as StatusSkipped
func (s *Step) SkippedRun(ctx context.Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RunAction)
	}

	if s.Next != nil {
		return s.Next.Run(ctx, NewSkippedRun(prevSuccess, report))
	}

	prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)

	return prevSuccess.workflowReport, nil
}

// SkippedRollback is a helper method to report that current step's rollback has been skipped and trigger previous step's rollback
// It marks the current step as StatusSkipped
func (s *Step) SkippedRollback(ctx context.Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewSkippedRollback(prevFailure, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)

	return prevFailure.workflowReport, nil
}

// FailedRollback is a helper method to report that current step's rollback has failed and trigger previous step's rollback
// It marks the current step RollbackAction as StatusFailed
func (s *Step) FailedRollback(ctx context.Context, prevFailure *Failure, err error, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	report.FailureReason = errors.EncodeError(ctx, err)

	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailedRollback(ctx, prevFailure, err, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)

	return prevFailure.workflowReport, nil
}

// RunNext is a helper method to report that current step has been successful and trigger next step's execution
// It marks the current step as StatusSuccess
func (s *Step) RunNext(ctx context.Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RunAction)
	}

	if s.Next != nil {
		return s.Next.Run(ctx, NewSuccess(prevSuccess, report))
	}

	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return prevSuccess.workflowReport, nil
}

// RollbackPrev is a helper method to report that current rollback step has been executed and trigger previous step's rollback
// It marks the current step as StatusFailed
func (s *Step) RollbackPrev(ctx context.Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailure(prevFailure, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return prevFailure.workflowReport, nil
}

// failedStep defines the failed state of the Workflow that implements Backward interface only
// This is one of the terminal states of the Workflow that works as the prev step of the first AtomicStep of the Workflow
type failedStep struct {
}

// successStep defines the success state of the Workflow that implements Forward interface only
// This is one of the terminal states of the Workflow that works as the next step of the last AtomicStep of the Workflow
type successStep struct {
}

// Rollback implements Backward interface for failedStep
func (fs *failedStep) Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, error) {
	return prevFailure.workflowReport, prevFailure.error
}

// Run implements the Forward interface for successStep
func (ss *successStep) Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error) {
	return prevSuccess.workflowReport, nil
}
