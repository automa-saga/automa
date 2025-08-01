// Package automa provides interfaces for implementing the Saga pattern and Choreography-based workflows.
package automa

// Saga defines the contract for a transactional step in a Saga workflow.
// Each step must support forward execution and optional rollback.
type Saga interface {
	// Execute runs the step in the forward direction.
	// Returns a WorkflowReport and error if execution fails.
	Execute(ctx *Context) (*WorkflowReport, error)

	// Reverse rolls back the step in the backward direction.
	// Returns a WorkflowReport and error if rollback fails.
	Reverse(ctx *Context) (*WorkflowReport, error)
}

// Choreographer defines methods for managing a double linked list of workflow steps.
// Enables chaining steps for Choreography execution in a Saga workflow.
type Choreographer interface {
	// SetNext sets the next step in the workflow sequence.
	SetNext(next Step)
	// SetPrev sets the previous step in the workflow sequence.
	SetPrev(prev Step)

	// GetNext retrieves the next step in the workflow sequence.
	GetNext() Step
	// GetPrev retrieves the previous step in the workflow sequence.
	GetPrev() Step

	// Reset restores the step to its initial state.
	Reset() Step
}

// Step represents a single transactional unit within a workflow.
// Combines Saga and Choreographer interfaces for execution and chaining.
type Step interface {
	// GetID returns the unique identifier for the step.
	GetID() string
	Saga
	Choreographer
}

// Registry manages registration and lookup of workflow steps.
// Supports building workflows from registered steps.
type Registry interface {
	// AddStep registers a single Step in the registry.
	AddStep(step Step) Registry

	// AddSteps registers multiple Steps in the registry.
	AddSteps(step ...Step) Registry

	// GetStep retrieves a Step by its ID.
	// Returns nil if the step does not exist.
	GetStep(id string) Step

	// GetSteps returns all registered Steps in the registry.
	GetSteps() []Step

	// BuildWorkflow constructs a Workflow from a list of Step IDs.
	// Returns an error if any step is missing.
	BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error)
}

// Workflow defines the contract for a Saga workflow.
// Supports execution, inspection, and step sequence management.
type Workflow interface {
	// GetID returns the workflow's unique identifier.
	GetID() string

	// Execute starts the workflow and returns a WorkflowReport.
	Execute(ctx *Context) (*WorkflowReport, error)

	// HasStep checks if the workflow contains a Step with the given ID.
	HasStep(stepID string) bool

	// GetStepSequence returns the ordered list of Step IDs in the workflow.
	GetStepSequence() []string

	// GetSteps returns the ordered list of Steps in the workflow.
	GetSteps() []Step
}
