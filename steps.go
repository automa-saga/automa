package automa

import (
	"context"
)

// Step is the kernel for AtomicStep implementation to be used as inheritance by composition pattern
type Step struct {
	ID   string
	Next Forward
	Prev Backward
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

// SkippedRun is a helper method to report that current step has been skipped and trigger next step's execution
// It marks the current step as StatusSkipped
func (s *Step) SkippedRun(ctx context.Context, prevSuccess *Success, report *Report) (Reports, error) {
	if s.Next != nil {
		return s.Next.Run(ctx, NewSkipped(prevSuccess, report))
	}

	var reports Reports
	if report != nil {
		reports = report.Append(prevSuccess.reports, StatusSkipped)
	}

	return reports, nil
}

// RunNext is a helper method to report that current step has been successful and trigger next step's execution
// It marks the current step as StatusSuccess
func (s *Step) RunNext(ctx context.Context, prevSuccess *Success, report *Report) (Reports, error) {
	if s.Next != nil {
		return s.Next.Run(ctx, NewSuccess(prevSuccess, report))
	}

	var reports Reports
	if report != nil {
		reports = report.Append(prevSuccess.reports, StatusSuccess)
	}

	return reports, nil
}

// RollbackPrev is a helper method to report that current rollback step has been executed and trigger previous step's rollback
// It marks the current step as StatusFailed
func (s *Step) RollbackPrev(ctx context.Context, prevFailure *Failure, report *Report) (Reports, error) {
	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailure(prevFailure, report))
	}

	var reports Reports
	if report != nil {
		reports = report.Append(prevFailure.reports, StatusFailed)
	}

	return reports, nil
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
func (fs *failedStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	return prevFailure.reports, prevFailure.error
}

// Run implements the Forward interface for successStep
func (ss *successStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	return prevSuccess.reports, nil
}
