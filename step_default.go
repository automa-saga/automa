package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// defaultStep implements Step interfaces.
type defaultStep struct {
	id                   string
	logger               *zerolog.Logger
	ctx                  context.Context
	prepare              PrepareFunc
	execute              ExecuteFunc
	rollback             RollbackFunc
	onCompletion         OnCompletionFunc
	onFailure            OnFailureFunc
	enableAsyncCallbacks bool
	state                StateBag
}

func (s *defaultStep) State() StateBag {
	if s.state == nil {
		s.state = &SyncStateBag{} // lazy initialization
	}
	return s.state
}

func (s *defaultStep) Id() string {
	return s.id
}

func (s *defaultStep) Context() context.Context {
	return s.ctx
}

func (s *defaultStep) Prepare(ctx context.Context) (context.Context, error) {
	s.ctx = ctx

	if s.prepare != nil {
		c, err := s.prepare(ctx)
		if err != nil {
			return nil, err
		}

		s.ctx = c
	}

	return s.ctx, nil
}

func (s *defaultStep) Execute(ctx context.Context) (*Report, error) {
	start := time.Now()
	if s.execute != nil {
		report, err := s.execute(ctx)
		if err != nil {
			failureReport := StepFailureReport(s.id, WithStartTime(start), WithError(err), WithReport(report))
			s.handleFailure(ctx, failureReport)
			return failureReport, err
		}

		successReport := StepSuccessReport(s.id, WithReport(report), WithStartTime(start))
		s.handleCompletion(ctx, successReport)
		return successReport, nil
	}

	return StepSkippedReport(s.id), nil
}

func (s *defaultStep) Rollback(ctx context.Context) (*Report, error) {
	start := time.Now()
	if s.rollback != nil {
		report, err := s.rollback(ctx)
		if err != nil {
			failureReport := StepFailureReport(s.id, WithStartTime(start), WithError(err), WithReport(report))
			s.handleFailure(ctx, failureReport)
			return failureReport, err
		}

		successReport := StepSuccessReport(s.id, WithReport(report), WithStartTime(start))
		s.handleCompletion(ctx, successReport)
		return successReport, nil
	}

	return StepSkippedReport(s.id), nil
}

func (s *defaultStep) handleCompletion(ctx context.Context, report *Report) {
	if s.onCompletion == nil {
		return
	}

	if s.enableAsyncCallbacks {
		go s.onCompletion(ctx, report)
	} else {
		s.onCompletion(ctx, report)
	}
}

func (s *defaultStep) handleFailure(ctx context.Context, report *Report) {
	if s.onFailure == nil {
		return
	}

	if s.enableAsyncCallbacks {
		go s.onFailure(ctx, report)
	} else {
		s.onFailure(ctx, report)
	}
}

func newDefaultStep() *defaultStep {
	return &defaultStep{}
}
