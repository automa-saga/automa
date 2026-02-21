package automa

import (
	"context"

	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

type ExecuteFunc func(ctx context.Context, stp Step) *Report
type RollbackFunc func(ctx context.Context, stp Step) *Report
type PrepareFunc func(ctx context.Context, stp Step) (context.Context, error)
type OnCompletionFunc func(ctx context.Context, stp Step, report *Report)
type OnFailureFunc func(ctx context.Context, stp Step, report *Report)

// StepBuilder is a builder for creating steps with optional prepare, execute, completion, and rollback functions.
type StepBuilder struct {
	Step *defaultStep
}

func (s *StepBuilder) Id() string {
	return s.Step.id
}

func (s *StepBuilder) WithId(id string) *StepBuilder {
	s.Step.id = id
	return s
}

func (s *StepBuilder) WithLogger(logger zerolog.Logger) *StepBuilder {
	s.Step.logger = &logger
	return s
}

func (s *StepBuilder) WithPrepare(f PrepareFunc) *StepBuilder {
	s.Step.prepare = f
	return s
}

func (s *StepBuilder) WithExecute(f ExecuteFunc) *StepBuilder {
	s.Step.execute = f
	return s
}

func (s *StepBuilder) WithRollback(f RollbackFunc) *StepBuilder {
	s.Step.rollback = f
	return s
}

func (s *StepBuilder) WithOnCompletion(f OnCompletionFunc) *StepBuilder {
	s.Step.onCompletion = f
	return s
}

func (s *StepBuilder) WithOnFailure(f OnFailureFunc) *StepBuilder {
	s.Step.onFailure = f
	return s
}

func (s *StepBuilder) WithAsyncCallbacks(enable bool) *StepBuilder {
	s.Step.enableAsyncCallbacks = enable
	return s
}

func (s *StepBuilder) WithState(state NamespacedStateBag) *StepBuilder {
	s.Step.state = state
	return s
}

func (s *StepBuilder) Validate() error {
	// Ensure that the step has a valid id and an execute function.
	if s.Step.id == "" {
		return IllegalArgument.New("step id cannot be empty")
	}

	if s.Step.execute == nil {
		return IllegalArgument.New("execute function cannot be nil")
	}

	return nil
}

func (s *StepBuilder) Build() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	finishedStep := s.Step
	s.Step = newDefaultStep()

	return finishedStep, nil
}

func (s *StepBuilder) BuildAndCopy() (Step, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	finishedStep := s.Step

	s.Step = newDefaultStep()

	s.Step.id = "" // reset id to force setting a new one
	s.Step.logger = finishedStep.logger
	s.Step.prepare = finishedStep.prepare
	s.Step.execute = finishedStep.execute
	s.Step.onCompletion = finishedStep.onCompletion
	s.Step.rollback = finishedStep.rollback
	s.Step.onFailure = finishedStep.onFailure
	s.Step.enableAsyncCallbacks = finishedStep.enableAsyncCallbacks

	// Clone the NamespacedStateBag (clones local, global, and custom namespaces)
	var err error
	if finishedStep.state != nil {
		s.Step.state, err = finishedStep.state.Clone()
		if err != nil {
			return nil, errorx.IllegalState.Wrap(err, "failed to clone state for step %q", finishedStep.id)
		}
	} else {
		s.Step.state = nil
	}

	return finishedStep, nil
}

// NewStepBuilder creates a step builder with options
func NewStepBuilder() *StepBuilder {
	s := &StepBuilder{
		Step: newDefaultStep(),
	}

	return s
}
