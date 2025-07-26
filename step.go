package automa

import (
	"context"
)

// AbstractStep is the kernel for Step implementation containing SagaRun and SagaUndo function
// It is to be used as inheritance by composition pattern by actual AbstractStep implementations
// If the saga methods are not registered, then AbstractStep will skip those operations during invocation of Run and Rollback
// Note that user may override the Run and Rollback methods in the actual implementation in order to change the control logic
type AbstractStep struct {
	ID   string
	Next Forward
	Prev Backward

	// holder of saga methods to be executed during Run and Rollback method of the Step
	run      SagaRun
	rollback SagaUndo
}

// SagaRun is a func definition to contain the run logic
//
// It provides a simpler method signature for the run logic of a Step
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type SagaRun func(ctx context.Context) (skipped bool, err error)

// SagaUndo is a func definition to contain the compensating logic
//
// It provides a simpler method signature for the run logic of a Step
// skipped return value denotes if the execution was skipped or not
// err return value denotes any error during execution (if any)
type SagaUndo func(ctx context.Context) (skipped bool, err error)

// RegisterSaga register saga logic for run and undo in order to leverage the default controller logic for Run and Rollback
// This is just a helper function where user would like to use the default Run and Rollback logic.
// This method usage is optional and user is free to implement Run and Rollback method of Step as they wish.
func (s *AbstractStep) RegisterSaga(run SagaRun, undo SagaUndo) *AbstractStep {
	s.run = run
	s.rollback = undo

	return s
}

// GetID returns the step ID
func (s *AbstractStep) GetID() string {
	return s.ID
}

// SetNext sets the next step of the workflow to be able to move in the forward direction on success
func (s *AbstractStep) SetNext(next Forward) {
	s.Next = next
}

// SetPrev sets the previous step of the workflow to be able to move in the backward direction on error
func (s *AbstractStep) SetPrev(prev Backward) {
	s.Prev = prev
}

// GetNext returns the step to be used to move in the forward direction
func (s *AbstractStep) GetNext() Forward {
	return s.Next
}

// GetPrev returns the step to be used to move in the backward direction
func (s *AbstractStep) GetPrev() Backward {
	return s.Prev
}

// Run implements Run controller logic for automa.Step interface
// This is a wrapper function to help simplify Step implementations
// Note that user may implement Run method in order to change the control logic as required.
func (s *AbstractStep) Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error) {
	report := NewStepReport(s.GetID(), RunAction)

	if s.run == nil {
		return s.SkippedRun(ctx, prevSuccess, report)
	}

	skipped, err := s.run(ctx)
	if err != nil {
		return s.Rollback(ctx, NewFailedRun(prevSuccess, err, report))
	}

	if skipped {
		return s.SkippedRun(ctx, prevSuccess, report)
	}

	return s.RunNext(ctx, prevSuccess, report)
}

// Rollback implements Rollback controller logic for automa.Step interface
// This is a wrapper function to help simplify Step implementations
// Note that user may implement Rollback method in order to change the control logic as required.
func (s *AbstractStep) Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, error) {
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
func (s *AbstractStep) SkippedRun(ctx context.Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
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
func (s *AbstractStep) SkippedRollback(ctx context.Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
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
func (s *AbstractStep) FailedRollback(ctx context.Context, prevFailure *Failure, err error, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	report.FailureReason = StepRollbackFailed.Wrap(err, "step rollback failed")

	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailedRollback(prevFailure, err, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)

	return prevFailure.workflowReport, nil
}

// RunNext is a helper method to report that current step has been successful and trigger next step's execution
// It marks the current step as StatusSuccess
func (s *AbstractStep) RunNext(ctx context.Context, prevSuccess *Success, report *StepReport) (WorkflowReport, error) {
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
func (s *AbstractStep) RollbackPrev(ctx context.Context, prevFailure *Failure, report *StepReport) (WorkflowReport, error) {
	if report == nil {
		report = NewStepReport(s.GetID(), RollbackAction)
	}

	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailure(prevFailure, report))
	}

	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return prevFailure.workflowReport, nil
}

// failedStep defines the failed state of the workflow that implements Backward interface only
// This is one of the terminal states of the workflow that works as the prev step of the first Step of the workflow
type failedStep struct {
}

// successStep defines the success state of the workflow that implements Forward interface only
// This is one of the terminal states of the workflow that works as the next step of the last Step of the workflow
type successStep struct {
}

// Rollback implements Backward interface for failedStep
func (fs *failedStep) Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, error) {
	return prevFailure.workflowReport, prevFailure.err
}

// Run implements the Forward interface for successStep
func (ss *successStep) Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error) {
	return prevSuccess.workflowReport, nil
}
