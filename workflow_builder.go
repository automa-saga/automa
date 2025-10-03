package automa

import (
	"fmt"
	"github.com/rs/zerolog"
	"sync"
)

type workflowBuilder struct {
	workflow     *workflow
	registry     Registry
	stepSequence []string
	stepBuilders map[string]Builder
	mu           sync.Mutex
}

func (wb *workflowBuilder) Id() string {
	return wb.workflow.id
}

func (wb *workflowBuilder) Build() (Step, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if err := wb.Validate(); err != nil {
		return nil, err
	}

	steps := make([]Step, 0, len(wb.stepBuilders))
	for _, stepId := range wb.stepSequence {
		builder, exists := wb.stepBuilders[stepId]
		if !exists {
			return nil, fmt.Errorf("step with id '%s' not found in builders map", stepId)
		}

		step, err := builder.Build()
		if err != nil {
			return nil, IllegalArgument.New("failed to build step '%s': %v", builder.Id(), err)
		}
		if step != nil {
			steps = append(steps, step)
		}
	}

	wb.workflow.steps = steps
	finished := wb.workflow
	wb.workflow = newWorkflow()

	return finished, nil
}

func (wb *workflowBuilder) Steps(steps ...Builder) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	for _, step := range steps {
		if _, exists := wb.stepBuilders[step.Id()]; exists {
			continue
		}
		wb.stepBuilders[step.Id()] = step
		wb.stepSequence = append(wb.stepSequence, step.Id())
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
			continue
		}
		if _, exists := wb.stepBuilders[id]; exists {
			continue
		}
		wb.stepBuilders[id] = builder
		wb.stepSequence = append(wb.stepSequence, id)
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
			errs = append(errs, fmt.Errorf("step with id %s failed validation: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %v", errs)
	}
	return nil
}

func (wb *workflowBuilder) WithId(id string) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.workflow.id = id
	return wb
}

func (wb *workflowBuilder) WithRegistry(sr Registry) WorkFlowBuilder {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.registry = sr
	return wb
}

func (wb *workflowBuilder) WithLogger(logger zerolog.Logger) WorkFlowBuilder {
	wb.workflow.logger = logger
	return wb
}

func (wb *workflowBuilder) WithRollbackMode(mode TypeRollbackMode) WorkFlowBuilder {
	wb.workflow.rollbackMode = mode
	return wb
}

func (wb *workflowBuilder) WithOnCompletion(f OnCompletionFunc) WorkFlowBuilder {
	wb.workflow.onCompletion = f
	return wb
}

func (wb *workflowBuilder) WithOnFailure(f OnFailureFunc) WorkFlowBuilder {
	wb.workflow.onFailure = f
	return wb
}

func NewWorkFlowBuilder() WorkFlowBuilder {
	return &workflowBuilder{
		workflow:     newWorkflow(),
		stepBuilders: make(map[string]Builder),
		stepSequence: []string{},
	}
}
