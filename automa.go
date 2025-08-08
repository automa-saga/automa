// Package automa provides interfaces for implementing the Saga pattern and Choreography-based workflows.
package automa

// Step represents a single transactional unit within a workflow.
// It defines the contract for executing a step in both forward and backward directions.
// A reverse operation means rollback in case of failure. If rollback isn't possible, a compensating action should be defined.
type Step interface {
	// GetID returns the unique identifier for the step.
	GetID() string

	// Forward executes the step in the forward direction.
	Execute(ctx *Context) (*Result, error)
	OnSuccess(ctx *Context) (*Result, error)

	// Reverse rolls back the step in the backward direction.
	OnRollback(ctx *Context) (*Result, error)
}

// WorkflowBuilder manages registration and lookup of workflow steps.
// Supports building workflows from registered steps.
type WorkflowBuilder interface {
	// AddSteps registers multiple Steps in the registry.
	AddSteps(step ...Step) WorkflowBuilder

	// RemoveSteps removes Step from the registry by its ID.
	RemoveSteps(stepID ...string) WorkflowBuilder

	// BuildWorkflow constructs a Workflow from a list of Step IDs.
	// Returns an error if any step is missing.
	Build(workflowID string, stepIDs []string) (Workflow, error)
}

// Workflow defines the contract for a Saga workflow.
// Supports execution, inspection, and step sequence management.
type Workflow interface {
	// Step interface allows a workflow be part of another workflow
	// A workflow can be composed of multiple steps, including other workflows.
	// A workflow can be executed in a forward direction (Run) and rolled back in reverse direction (Rollback).
	Step

	// AddSteps appends one or more Steps to the workflow.
	AddSteps(steps ...Step) error

	// RemoveSteps removes Step from the workflow by its ID.
	RemoveSteps(stepID ...string) error

	// HasStep checks if the workflow contains a Step with the given ID.
	HasStep(stepID string) bool

	// GetStepSequence returns the ordered list of Step IDs in the workflow.
	GetStepSequence() []string
}
