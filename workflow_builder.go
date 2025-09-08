package automa

import (
	"fmt"
	"github.com/automa-saga/automa/types"
	"sync"

	"github.com/rs/zerolog"
)

type workflowBuilder struct {
	id           string
	registry     Registry
	rollbackMode types.RollbackMode
	logger       zerolog.Logger
	stepBuilders map[string]Builder
	mu           sync.Mutex
}

func (wb *workflowBuilder) Id() string {
	return wb.id
}

func (wb *workflowBuilder) Build() (Step, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	steps := make([]Step, 0, len(wb.stepBuilders))
	for _, builder := range wb.stepBuilders {
		step, err := builder.Build()
		if err != nil {
			return nil, IllegalArgument.New("failed to build step: %w", err)
		}
		if step != nil {
			steps = append(steps, step)
		}
	}
	return NewWorkflow(wb.id, steps, WithLogger(wb.logger), WithRollbackMode(wb.rollbackMode)), nil
}

func (wb *workflowBuilder) Steps(steps ...Builder) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	for _, step := range steps {
		if _, exists := wb.stepBuilders[step.Id()]; exists {
			wb.logger.Warn().Str("step_id", step.Id()).Msg("duplicate step, skipping")
			continue
		}
		wb.stepBuilders[step.Id()] = step
	}
	return wb
}

func (wb *workflowBuilder) NamedSteps(stepIds ...string) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.registry == nil || len(stepIds) == 0 {
		return wb
	}
	for _, id := range stepIds {
		builder := wb.registry.Of(id)
		if builder == nil {
			wb.logger.Warn().Str("step_id", id).Msg("step not found in registry")
			continue
		}
		if _, exists := wb.stepBuilders[id]; exists {
			wb.logger.Warn().Str("step_id", id).Msg("duplicate step, skipping")
			continue
		}
		wb.stepBuilders[id] = builder
	}
	return wb
}

func (wb *workflowBuilder) Validate() error {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if len(wb.stepBuilders) == 0 {
		return StepNotFound.New("no steps provided for workflow")
	}
	var errs []error
	for id, builder := range wb.stepBuilders {
		if err := builder.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("step with ID %s failed validation: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %v", errs)
	}
	return nil
}

func (wb *workflowBuilder) WithRegistry(sr Registry) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.registry = sr
	return wb
}

func (wb *workflowBuilder) WithLogger(logger zerolog.Logger) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.logger = logger
	return wb
}

func (wb *workflowBuilder) WithRollbackMode(mode types.RollbackMode) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.rollbackMode = mode
	return wb
}

func NewWorkFlowBuilder(id string) WorkFlowBuilder {
	if id == "" {
		panic("workflow id must not be empty")
	}
	return &workflowBuilder{
		id:           id,
		rollbackMode: types.RollbackModeContinueOnError,
		logger:       zerolog.Nop(),
		stepBuilders: make(map[string]Builder),
	}
}
