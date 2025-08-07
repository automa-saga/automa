package automa

import (
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"sync"
)

// stepRegistry implements the Registry interface.
// It manages registration, lookup, and workflow construction for Steps.
type stepRegistry struct {
	mu      sync.RWMutex
	stepMap map[string]Step // Internal map for storing steps by ID.
	logger  *zerolog.Logger // Logger for registry-level logging.
}

// AddSteps registers multiple Steps in the registry.
// Returns the registry for method chaining.
func (r *stepRegistry) AddSteps(steps ...Step) Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, step := range steps {
		if step != nil {
			r.stepMap[step.GetID()] = step
		}
	}
	return r
}

// RemoveSteps removes Steps from the registry by their IDs.
func (r *stepRegistry) RemoveSteps(stepIDs ...string) Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	var removedSteps []Step
	for _, stepID := range stepIDs {
		if step, exists := r.stepMap[stepID]; exists {
			removedSteps = append(removedSteps, step)
			delete(r.stepMap, stepID)
		}
	}

	return r
}

// GetStep retrieves a Step by its ID from the registry.
// Returns nil if the step does not exist.
func (r *stepRegistry) GetStep(id string) Step {
	r.mu.RLock()
	defer r.mu.RUnlock()

	step, exists := r.stepMap[id]
	if !exists {
		return nil // Step not found
	}
	return step
}

// GetSteps returns all registered Steps in the registry as a slice.
func (r *stepRegistry) GetSteps() []Step {
	r.mu.RLock()
	defer r.mu.RUnlock()

	steps := make([]Step, 0, len(r.stepMap))
	for _, step := range r.stepMap {
		steps = append(steps, step)
	}
	return steps
}

// BuildWorkflow constructs a Workflow from the provided step IDs.
// Resets each step before adding it to the workflow.
// Returns an error if any step ID is invalid or missing.
func (r *stepRegistry) BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

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

	return &stepRegistry{stepMap: map[string]Step{}, logger: logger}
}
