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
	// AddSteps registers multiple Steps in the registry.
	AddSteps(step ...Step) Registry

	// RemoveSteps removes Step from the registry by its ID.
	RemoveSteps(stepID ...string) Registry

	// BuildWorkflow constructs a Workflow from a list of Step IDs.
	// Returns an error if any step is missing.
	BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error)
}

// Workflow defines the contract for a Saga workflow.
// Supports execution, inspection, and step sequence management.
type Workflow interface {
	// Step interface allowing it to be part of another workflow that can be executed and rolled back.
	// This interface ensures a workflow can be composed of other workflows and implements the Saga pattern.
	Step

	// HasStep checks if the workflow contains a Step with the given ID.
	HasStep(stepID string) bool

	// GetStepSequence returns the ordered list of Step IDs in the workflow.
	GetStepSequence() []string
}
