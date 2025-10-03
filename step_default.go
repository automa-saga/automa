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

func (s *defaultStep) Execute(ctx context.Context) *Report {
	start := time.Now()
	if s.execute != nil {
		report := s.execute(ctx)

		if report.Error != nil {
			failureReport := FailureReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))

			s.handleFailure(ctx, failureReport)

			return failureReport
		}

		successReport := SuccessReport(s,
			WithReport(report),
			WithActionType(ActionExecute),
			WithStartTime(start))

		s.handleCompletion(ctx, successReport)

		return successReport
	}

	return SkippedReport(s, WithActionType(ActionExecute), WithStartTime(start))
}

func (s *defaultStep) Rollback(ctx context.Context) *Report {
	start := time.Now()
	if s.rollback != nil {
		report := s.rollback(ctx)
		if report.Error != nil {
			failureReport := FailureReport(s,
				WithReport(report),
				WithActionType(ActionRollback),
				WithStartTime(start))
			s.handleFailure(ctx, failureReport)
			return failureReport
		}

		successReport := SuccessReport(s,
			WithReport(report),
			WithActionType(ActionRollback),
			WithStartTime(start))
		s.handleCompletion(ctx, successReport)
		return successReport
	}

	return SkippedReport(s,
		WithActionType(ActionRollback),
		WithStartTime(start))
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
