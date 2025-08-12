package automa

import "context"

type StepOption func(*stepBuilder)
type ExecuteFunc func(ctx context.Context) (Report, error)
type RollbackFunc func(ctx context.Context) (Report, error)
type PrepareFunc func(ctx context.Context) (context.Context, error)
type OnSuccessFunc func(ctx context.Context, report Report)

// StepBuilder is a builder for creating steps with optional prepare, execute, onSuccess, and onRollback functions.
type stepBuilder struct {
	id         string
	prepare    PrepareFunc
	execute    ExecuteFunc
	onSuccess  OnSuccessFunc
	onRollback RollbackFunc
}

func WithPrepare(f PrepareFunc) StepOption {
	return func(s *stepBuilder) {
		s.prepare = f
	}
}

func WithOnSuccess(f OnSuccessFunc) StepOption {
	return func(s *stepBuilder) {
		s.onSuccess = f
	}
}

func WithRollback(f RollbackFunc) StepOption {
	return func(s *stepBuilder) {
		s.onRollback = f
	}
}

func (s *stepBuilder) Id() string {
	return s.id
}

func (s *stepBuilder) Build() Step {
	// Build method returns a new instance of Step with the same configuration.
	// It ensures that the step can be reused and executed multiple times.
	return &basicStep{
		id:         s.id,
		prepare:    s.prepare,
		execute:    s.execute,
		onSuccess:  s.onSuccess,
		onRollback: s.onRollback,
	}
}

// NewStep creates a new step builder with the given ID and execute function.
func NewStep(id string, execute ExecuteFunc, opts ...StepOption) Builder {
	s := &stepBuilder{id: id, execute: execute}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
