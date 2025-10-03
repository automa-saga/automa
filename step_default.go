package automa

import (
	"context"

	"github.com/rs/zerolog"
)

// defaultStep implements Step interfaces.
// This is the default Step implementation that is meant to be stateless. For stateful steps, you can implement your
// custom-step Builder.
// It can be used to create steps with custom prepare, execute, onCompletion, and rollback functions.
type defaultStep struct {
	id                   string
	logger               *zerolog.Logger
	ctx                  context.Context
	prepare              OnPrepareFunc
	execute              ExecuteFunc
	rollback             RollbackFunc
	onCompletion         OnCompletionFunc
	onFailure            OnFailureFunc
	enableAsyncCallbacks bool
}

func (s *defaultStep) State() StateBag {
	//TODO implement me
	panic("implement me")
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
	if s.execute != nil {
		report, err := s.execute(ctx)
		if err != nil {
			s.handleFailure(ctx, report)
			return report, err
		}

		s.handleCompletion(ctx, report)
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

func (s *defaultStep) Rollback(ctx context.Context) (*Report, error) {
	if s.rollback != nil {
		return s.rollback(ctx)
	}

	return StepSkippedReport(s.id), nil
}
