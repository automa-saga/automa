package automa

import (
	"context"
	"github.com/rs/zerolog"
)

type StepOption func(*StepBuilder)
type OnExecuteFunc func(ctx context.Context) (*Report, error)
type OnRollbackFunc func(ctx context.Context) (*Report, error)
type OnPrepareFunc func(ctx context.Context) (context.Context, error)
type OnCompletionFunc func(ctx context.Context, report *Report)
type OnBuildFunc func() (Step, error)
type OnValidateFunc func() error

// StepBuilder is a builder for creating steps with optional OnPrepare, OnExecute, OnSuccess, and OnRollback functions.
type StepBuilder struct {
	ID         string
	Logger     zerolog.Logger
	OnValidate OnValidateFunc
	OnBuild    OnBuildFunc
	OnPrepare  OnPrepareFunc
	OnExecute  OnExecuteFunc
	OnSuccess  OnCompletionFunc
	OnRollback OnRollbackFunc
}

func (s *StepBuilder) Validate() error {
	// Ensure that the step has a valid ID and an OnExecute function.
	if s.ID == "" {
		return IllegalArgument.New("step ID cannot be empty")
	}

	if s.OnExecute == nil {
		return IllegalArgument.New("OnExecute function cannot be nil")
	}

	if s.OnValidate != nil {
		return s.OnValidate()
	}

	return nil
}

func WithLogger(logger zerolog.Logger) StepOption {
	return func(s *StepBuilder) {
		s.Logger = logger
	}
}

func WithOnValidate(f OnValidateFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnValidate = f
	}
}

func WithOnExecute(f OnExecuteFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnExecute = f
	}
}

func WithOnPrepare(f OnPrepareFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnPrepare = f
	}
}

func WithOnCompletion(f OnCompletionFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnSuccess = f
	}
}

func WithOnRollback(f OnRollbackFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnRollback = f
	}
}

func WithOnBuild(f OnBuildFunc) StepOption {
	return func(s *StepBuilder) {
		s.OnBuild = f
	}
}

func (s *StepBuilder) Id() string {
	return s.ID
}

func (s *StepBuilder) Build() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// if a OnBuild function is provided, invoke it to create the step
	if s.OnBuild != nil {
		return s.OnBuild()
	}

	return &defaultStep{
		id:           s.ID,
		logger:       s.Logger,
		onPrepare:    s.OnPrepare,
		onExecute:    s.OnExecute,
		onCompletion: s.OnSuccess,
		onRollback:   s.OnRollback,
	}, nil

}

// NewStepBuilder creates a step builder with options
func NewStepBuilder(id string, opts ...StepOption) *StepBuilder {
	s := &StepBuilder{ID: id}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
