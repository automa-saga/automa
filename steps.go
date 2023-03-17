package automa

import (
	"context"
)

type Step struct {
	ID   string
	Next Forward
	Prev Backward
}

func (s *Step) SetNext(next Forward) {
	s.Next = next
}

func (s *Step) SetPrev(prev Backward) {
	s.Prev = prev
}

func (s *Step) GetNext() Forward {
	return s.Next
}

func (s *Step) GetPrev() Backward {
	return s.Prev
}

func (s *Step) SkippedRun(ctx context.Context, prevSuccess *Success, report *Report) (Reports, error) {
	if s.Next != nil {
		return s.Next.Run(ctx, NewSkipped(prevSuccess, report))
	}

	var reports Reports
	if report != nil {
		reports = report.End(prevSuccess.reports, StatusSkipped)
	}

	return reports, nil
}

func (s *Step) SkippedRollback(ctx context.Context, prevFailure *Failure, report *Report) (Reports, error) {
	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewSkippedFailure(prevFailure, report))
	}

	var reports Reports
	if report != nil {
		reports = report.End(prevFailure.reports, StatusSkipped)
	}

	return reports, nil
}

func (s *Step) RunNext(ctx context.Context, prevSuccess *Success, report *Report) (Reports, error) {
	if s.Next != nil {
		return s.Next.Run(ctx, NewSuccess(prevSuccess, report))
	}

	var reports Reports
	if report != nil {
		reports = report.End(prevSuccess.reports, StatusSuccess)
	}

	return reports, nil
}

func (s *Step) RollbackPrev(ctx context.Context, prevFailure *Failure, report *Report) (Reports, error) {
	if s.Prev != nil {
		return s.Prev.Rollback(ctx, NewFailure(prevFailure, report))
	}

	var reports Reports
	if report != nil {
		reports = report.End(prevFailure.reports, StatusFailed)
	}

	return reports, nil
}

// failedStep defines the failed state of the Workflow
// This is one of the terminal state of the Workflow
type failedStep struct {
}

// successStep defines the success state of the Workflow
// This is one of the terminal state of the Workflow
type successStep struct {
}

// Rollback implements Backward interface for failedStep
// It outputs the failure event to the failureOut channel
func (fs *failedStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	return prevFailure.reports, prevFailure.error
}

// Run implements the Forward interface for successStep
// It outputs the success event to the successOut channel
func (ss *successStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	return prevSuccess.reports, nil
}
