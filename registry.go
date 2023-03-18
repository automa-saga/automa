package automa

import (
	"go.uber.org/zap"
)

// StepRegistry is an implementation of AtomicStepRegistry interface
type StepRegistry struct {
	cache  map[string]AtomicStep
	logger *zap.Logger
}

// NewStepRegistry returns an instance of StepRegistry that implements AtomicStepRegistry
// if logger is nil, it initializes itself with a NoOp logger
func NewStepRegistry(logger *zap.Logger) *StepRegistry {
	if logger == nil {
		logger = zap.NewNop()
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
func (r *StepRegistry) RegisterSteps(steps map[string]AtomicStep) *StepRegistry {
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
func (r *StepRegistry) BuildWorkflow(stepIDs []string) *Workflow {
	var steps []AtomicStep
	for _, id := range stepIDs {
		step := r.GetStep(id)
		if step != nil {
			steps = append(steps, step)
		}
	}

	return NewWorkflow(WithSteps(steps...), WithLogger(r.logger))
}
