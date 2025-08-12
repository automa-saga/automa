package automa

import "github.com/rs/zerolog"

type workflowBuilder struct {
	id           string
	registry     Registry
	rollbackMode RollbackMode
	logger       zerolog.Logger
	stepBuilders map[string]Builder
}

func (wb *workflowBuilder) Id() string {
	return wb.id
}

func (wb *workflowBuilder) Build() Step {
	steps := make([]Step, 0, len(wb.stepBuilders))
	for _, builder := range wb.stepBuilders {
		step := builder.Build()
		if step == nil {
			continue // skip nil steps
		}
		steps = append(steps, step)
	}
	return NewWorkflow(wb.id, steps, WithLogger(wb.logger), WithRollbackMode(wb.rollbackMode))
}

func (wb *workflowBuilder) AddSteps(steps ...Builder) error {
	for _, step := range steps {
		if _, exists := wb.stepBuilders[step.Id()]; exists {
			return StepAlreadyExists.New("step with ID %s already exists", step.Id())
		}
		wb.stepBuilders[step.Id()] = step
	}

	return nil
}

func (wb *workflowBuilder) WithNamed(stepIds ...string) error {
	if wb.registry == nil {
		return RegistryNotProvided.New("registry is not set, cannot resolve step builders by ID")
	}

	if len(stepIds) == 0 {
		return StepIdsNotProvided.New("no step IDs provided, cannot resolve step builders")
	}

	for _, id := range stepIds {
		builder := wb.registry.Of(id)
		if builder == nil {
			return StepNotFound.New("step with ID %s not found", id)
		}
		if _, exists := wb.stepBuilders[id]; exists {
			return StepAlreadyExists.New("step with ID %s already exists", id)
		}
		wb.stepBuilders[id] = builder
	}
	return nil
}

func (wb *workflowBuilder) WithRegistry(sr Registry) WorkFlowBuilder {
	wb.registry = sr
	return wb
}

func (wb *workflowBuilder) WithLogger(logger zerolog.Logger) WorkFlowBuilder {
	wb.logger = logger
	return wb
}

func (wb *workflowBuilder) WithRollbackMode(mode RollbackMode) WorkFlowBuilder {
	wb.rollbackMode = mode
	return wb
}

func NewWorkFlowBuilder(id string) WorkFlowBuilder {
	wb := &workflowBuilder{
		id:           id,
		rollbackMode: RollbackModeContinueOnError, // Default rollback mode
		registry:     nil,                         // Registry can be set later
		logger:       zerolog.Nop(),               // Default logger is a no-op
		stepBuilders: make(map[string]Builder),
	}

	return wb
}
