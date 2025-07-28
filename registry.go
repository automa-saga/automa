package automa

import (
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

// stepRegistry is an implementation of Registry interface
type stepRegistry struct {
	cache  map[string]Step
	logger *zerolog.Logger
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

// AddSteps adds multiple Steps to the registry
func (r *stepRegistry) AddSteps(steps ...Step) Registry {
	for _, step := range steps {
		r.registerStep(step.GetID(), step)
	}

	return r
}

func (r *stepRegistry) AddStep(step Step) Registry {
	// AddStep adds a Step to the registry
	// It returns itself so that chaining is possible when registering multiple steps with the registry
	if step != nil {
		r.registerStep(step.GetID(), step)
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

// GetSteps returns all Steps in the registry
func (r *stepRegistry) GetSteps() []Step {
	// GetSteps returns all Steps in the registry
	// It returns a slice of Step
	var steps []Step
	for _, step := range r.cache {
		steps = append(steps, step)
	}

	return steps
}

// BuildWorkflow is a helper method to build a workflow from the given set of Step IDs
func (r *stepRegistry) BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error) {
	var steps []Step
	for _, stepID := range stepIDs {
		step := r.GetStep(stepID)
		if step != nil {
			step.Reset()
			steps = append(steps, step)
		} else {
			return nil, errorx.IllegalState.New("invalid step: %s", stepID)
		}
	}

	workflow := NewWorkflow(workflowID, WithSteps(steps...), WithLogger(r.logger))
	return workflow, nil
}

// NewRegistry returns an instance of stepRegistry that implements Registry
// if logx is nil, it initializes itself with a NoOp logx
func NewRegistry(logger *zerolog.Logger) Registry {
	if logger == nil {
		logger = &nolog
	}

	return &stepRegistry{cache: map[string]Step{}, logger: logger}
}
