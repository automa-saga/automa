package automa

import (
	"context"
	"github.com/rs/zerolog"
)

// defaultStep implements Step interfaces.
// This is the default Step implementation that is meant to be stateless. For stateful steps, you can implement your
// custom-step Builder.
// It can be used to create steps with custom onPrepare, onExecute, onCompletion, and onRollback functions.
type defaultStep struct {
	id           string
	logger       zerolog.Logger
	ctx          context.Context
	onPrepare    OnPrepareFunc
	onExecute    OnExecuteFunc
	onCompletion OnCompletionFunc
	onRollback   OnRollbackFunc
}

func (s *defaultStep) Id() string {
	return s.id
}

func (s *defaultStep) Context() context.Context {
	return s.ctx
}

func (s *defaultStep) Prepare(ctx context.Context) (context.Context, error) {
	s.ctx = ctx

	if s.onPrepare != nil {
		c, err := s.onPrepare(ctx)
		if err != nil {
			return nil, err
		}

		s.ctx = c
	}

	return s.ctx, nil
}

func (s *defaultStep) Execute(ctx context.Context) (*Report, error) {
	if s.onExecute != nil {
		return s.onExecute(ctx)
	}

	return StepSkippedReport(s.id), nil
}

func (s *defaultStep) OnCompletion(ctx context.Context, report *Report) {
	if s.onCompletion != nil {
		s.onCompletion(ctx, report)
	}
}

func (s *defaultStep) OnRollback(ctx context.Context) (*Report, error) {
	if s.onRollback != nil {
		return s.onRollback(ctx)
	}

	return StepSkippedReport(s.id), nil
}
