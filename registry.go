package automa

import (
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
)

// stepRegistry implements the Registry interface.
// It manages registration, lookup, and workflow construction for Steps.
type stepRegistry struct {
	cache  map[string]Step // Internal map for storing steps by ID.
	logger *zerolog.Logger // Logger for registry-level logging.
}

// registerStep adds a Step to the registry by its ID.
// If the step is nil, it is skipped without error.
// Returns the registry for method chaining.
func (r *stepRegistry) registerStep(id string, step Step) *stepRegistry {
	if step != nil {
		r.cache[id] = step
	}
	return r
}

// AddSteps registers multiple Steps in the registry.
// Returns the registry for method chaining.
func (r *stepRegistry) AddSteps(steps ...Step) Registry {
	for _, step := range steps {
		r.registerStep(step.GetID(), step)
	}
	return r
}

// AddStep registers a single Step in the registry.
// Returns the registry for method chaining.
func (r *stepRegistry) AddStep(step Step) Registry {
	if step != nil {
		r.registerStep(step.GetID(), step)
	}
	return r
}

// GetStep retrieves a Step by its ID from the registry.
// Returns nil if the step does not exist.
func (r *stepRegistry) GetStep(id string) Step {
	if step, ok := r.cache[id]; ok {
		return step
	}
	return nil
}

// GetSteps returns all registered Steps in the registry as a slice.
func (r *stepRegistry) GetSteps() []Step {
	var steps []Step
	for _, step := range r.cache {
		steps = append(steps, step)
	}
	return steps
}

// BuildWorkflow constructs a Workflow from the provided step IDs.
// Resets each step before adding it to the workflow.
// Returns an error if any step ID is invalid or missing.
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

	wf := NewWorkflow(workflowID, WithSteps(steps...), WithLogger(r.logger))
	return wf, nil
}

// NewRegistry creates and returns a new stepRegistry instance implementing Registry.
// If logger is nil, a NoOp logger is used.
func NewRegistry(logger *zerolog.Logger) Registry {
	if logger == nil {
		logger = &nolog
	}
	return &stepRegistry{cache: map[string]Step{}, logger: logger}
}
