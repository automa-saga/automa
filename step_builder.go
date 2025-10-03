package automa

import (
	"context"
	"github.com/rs/zerolog"
)

type StepOption func(*StepBuilder)
type ExecuteFunc func(ctx context.Context) (*Report, error)
type RollbackFunc func(ctx context.Context) (*Report, error)
type OnPrepareFunc func(ctx context.Context) (context.Context, error)
type OnCompletionFunc func(ctx context.Context, report *Report)
type OnFailureFunc func(ctx context.Context, report *Report)

// StepBuilder is a builder for creating steps with optional prepare, execute, completion, and rollback functions.
type StepBuilder struct {
	id         string
	logger     *zerolog.Logger
	prepare    OnPrepareFunc
	execute    ExecuteFunc
	completion OnCompletionFunc
	rollback   RollbackFunc
}

func (s *StepBuilder) Validate() error {
	// Ensure that the step has a valid id and an execute function.
	if s.id == "" {
		return IllegalArgument.New("step id cannot be empty")
	}

	if s.execute == nil {
		return IllegalArgument.New("execute function cannot be nil")
	}

	return nil
}

func (s *StepBuilder) Logger() *zerolog.Logger {
	return s.logger
}

func WithLogger(logger zerolog.Logger) StepOption {
	return func(s *StepBuilder) {
		s.logger = &logger
	}
}

func WithOnExecute(f ExecuteFunc) StepOption {
	return func(s *StepBuilder) {
		s.execute = f
	}
}

func WithOnPrepare(f OnPrepareFunc) StepOption {
	return func(s *StepBuilder) {
		s.prepare = f
	}
}

func WithOnCompletion(f OnCompletionFunc) StepOption {
	return func(s *StepBuilder) {
		s.completion = f
	}
}

func WithOnRollback(f RollbackFunc) StepOption {
	return func(s *StepBuilder) {
		s.rollback = f
	}
}

func (s *StepBuilder) Id() string {
	return s.id
}

func (s *StepBuilder) Build() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	return &defaultStep{
		id:           s.id,
		logger:       s.logger,
		onPrepare:    s.prepare,
		onExecute:    s.execute,
		onCompletion: s.completion,
		onRollback:   s.rollback,
	}, nil

}

// NewStepBuilder creates a step builder with options
func NewStepBuilder(id string, opts ...StepOption) *StepBuilder {
	s := &StepBuilder{id: id}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
