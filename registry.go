package automa

import (
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

// stepRegistry is an implementation of StepRegistry interface
type stepRegistry struct {
	cache  map[string]Step
	logger *zerolog.Logger
}

// NewStepRegistry returns an instance of stepRegistry that implements StepRegistry
// if logx is nil, it initializes itself with a NoOp logx
func NewStepRegistry(logger *zerolog.Logger) StepRegistry {
	if logger == nil {
		logger = &nolog
	}

	return &stepRegistry{cache: map[string]Step{}, logger: logger}
}

// registerStep registers a Step with the registry
// If a nil step is provided, it skips adding it to the registry without throwing eny error
// It returns itself so that chaining is possible when registering multiple steps with the registry
func (r *stepRegistry) registerStep(id string, step Step) *stepRegistry {
	if step != nil {
		r.cache[id] = step
	}

	return r
}

// RegisterSteps is a helper method to register multiple AtomicSteps at a time
func (r *stepRegistry) RegisterSteps(steps map[string]Step) StepRegistry {
	for id, step := range steps {
		r.registerStep(id, step)
	}

	return r
}

// GetStep returns a Step by the id
// It returns error if a step cannot be found by the given ID
func (r *stepRegistry) GetStep(id string) Step {
	if step, ok := r.cache[id]; ok {
		return step
	}

	return nil
}

// BuildWorkflow is a helper method to build a workflow from the given set of Step IDs
func (r *stepRegistry) BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error) {
	var steps []Step
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
