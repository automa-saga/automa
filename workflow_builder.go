package automa

import (
	"fmt"

	"github.com/rs/zerolog"
)

// WorkflowBuilder is a builder for creating workflows with a sequence of steps.
// This is a mutable builder, and should not be used concurrently.
type WorkflowBuilder struct {
	workflow     *workflow
	registry     Registry
	stepSequence []string
	stepBuilders map[string]Builder
}

func (wb *WorkflowBuilder) Id() string {
	return wb.workflow.id
}

func (wb *WorkflowBuilder) Build() (Step, error) {
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
	wb.workflow = newDefaultWorkflow()

	return finished, nil
}

func (wb *WorkflowBuilder) Steps(steps ...Builder) *WorkflowBuilder {
	for _, step := range steps {
		if _, exists := wb.stepBuilders[step.Id()]; exists {
			continue
		}
		wb.stepBuilders[step.Id()] = step
		wb.stepSequence = append(wb.stepSequence, step.Id())
	}
	return wb
}

func (wb *WorkflowBuilder) NamedSteps(stepIds ...string) *WorkflowBuilder {
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

func (wb *WorkflowBuilder) Validate() error {
	if wb.workflow.id == "" {
		return IllegalArgument.New("workflow id cannot be empty")
	}

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

func (wb *WorkflowBuilder) WithId(id string) *WorkflowBuilder {
	wb.workflow.id = id
	return wb
}

func (wb *WorkflowBuilder) WithRegistry(sr Registry) *WorkflowBuilder {
	wb.registry = sr
	return wb
}

func (wb *WorkflowBuilder) WithLogger(logger zerolog.Logger) *WorkflowBuilder {
	wb.workflow.logger = logger
	return wb
}

func (wb *WorkflowBuilder) WithExecutionMode(mode TypeMode) *WorkflowBuilder {
	wb.workflow.executionMode = mode
	return wb
}

func (wb *WorkflowBuilder) WithRollbackMode(mode TypeMode) *WorkflowBuilder {
	wb.workflow.rollbackMode = mode
	return wb
}

func (wb *WorkflowBuilder) WithOnCompletion(f OnCompletionFunc) *WorkflowBuilder {
	wb.workflow.onCompletion = f
	return wb
}

func (wb *WorkflowBuilder) WithOnFailure(f OnFailureFunc) *WorkflowBuilder {
	wb.workflow.onFailure = f
	return wb
}

func (wb *WorkflowBuilder) WithAsyncCallbacks(enable bool) *WorkflowBuilder {
	wb.workflow.enableAsyncCallbacks = enable
	return wb
}

func (wb *WorkflowBuilder) WithPrepare(prepareFunc PrepareFunc) *WorkflowBuilder {
	wb.workflow.prepare = prepareFunc
	return wb
}

func (wb *WorkflowBuilder) WithState(state StateBag) *WorkflowBuilder {
	wb.workflow.state = state
	return wb
}

func (wb *WorkflowBuilder) WithRollback(rollback RollbackFunc) *WorkflowBuilder {
	wb.workflow.rollback = rollback
	return wb
}

func NewWorkflowBuilder() *WorkflowBuilder {
	return &WorkflowBuilder{
		workflow:     newDefaultWorkflow(),
		stepBuilders: make(map[string]Builder),
		stepSequence: []string{},
	}
}
