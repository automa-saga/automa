package automa

import (
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

// StepRegistry is an implementation of AtomicStepRegistry interface
type StepRegistry struct {
	cache  map[string]AtomicStep
	logger *zerolog.Logger
}

// NewStepRegistry returns an instance of StepRegistry that implements AtomicStepRegistry
// if logx is nil, it initializes itself with a NoOp logx
func NewStepRegistry(logger *zerolog.Logger) *StepRegistry {
	if logger == nil {
		logger = &nolog
	}

	return &StepRegistry{cache: map[string]AtomicStep{}, logger: logger}
}

// registerStep registers an AtomicStep with the registry
// If a nil step is provided, it skips adding it to the registry without throwing eny error
// It returns itself so that chaining is possible when registering multiple steps with the registry
func (r *StepRegistry) registerStep(id string, step AtomicStep) *StepRegistry {
	if step != nil {
		r.cache[id] = step
	}

	return r
}

// RegisterSteps is a helper method to register multiple AtomicSteps at a time
func (r *StepRegistry) RegisterSteps(steps map[string]AtomicStep) AtomicStepRegistry {
	for id, step := range steps {
		r.registerStep(id, step)
	}

	return r
}

// GetStep returns an AtomicStep by the id
// It returns error if a step cannot be found by the given ID
func (r *StepRegistry) GetStep(id string) AtomicStep {
	if step, ok := r.cache[id]; ok {
		return step
	}

	return nil
}

// BuildWorkflow is a helper method to build a Workflow from the given set of AtomicStep IDs
func (r *StepRegistry) BuildWorkflow(workflowID string, stepIDs StepIDs) (AtomicWorkflow, *errorx.Error) {
	var steps []AtomicStep
	for _, stepID := range stepIDs {
		step := r.GetStep(stepID)
		if step != nil {
			steps = append(steps, step)
		} else {
			return nil, errorx.IllegalState.New("invalid step: %s", stepID)
		}
	}

	workflow := NewWorkflow(workflowID, WithSteps(steps...), WithLogger(r.logger))
	return workflow, nil
}
