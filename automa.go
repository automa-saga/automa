package automa

import (
	"context"
)

// Step provides interface for an atomic state
// Note that a Step may skip rollback if that makes sense and in that case it is not Atomic in nature.
type Step interface {
	// GetID returns the step ID
	GetID() string

	Forward
	Backward
	Choreographer
}

// StepRegistry is a registry of rollbackable steps
type StepRegistry interface {
	// RegisterSteps registers a set of Step
	// steps argument is a map where key is the step identifier
	RegisterSteps(steps map[string]Step) StepRegistry

	// GetStep returns a Step from the registry given the id
	// If it doesn't exist, it returns nil. So the caller should handle the nil Step safely.
	GetStep(id string) Step

	// BuildWorkflow builds a Workflow comprising the list of Step identified by ids
	BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error)
}

// Workflow defines interface for a workflow
type Workflow interface {
	// GetID returns the workflow ID
	GetID() string

	// Start starts the Workflow execution
	Start(ctx context.Context) (WorkflowReport, error)

	// End performs cleanup after the Workflow engine finish its execution
	End(ctx context.Context)
}

// Forward defines the methods to execute business logic of a Step and move the workflow forward
type Forward interface {
	// Run runs the business logic to be performed in the Step
	Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error)
}

// Backward defines the methods to be executed to move the workflow backward on error
type Backward interface {
	// Rollback defines the actions compensating the business logic executed in Run method
	// A step may skip rollback if that makes sense. In that case it would mean the Step is not Atomic in nature.
	Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, error)
}

// Choreographer interface defines the methods to support double link list of states
// This is needed to support Choreography execution of the Saga workflow
type Choreographer interface {
	SetNext(next Forward)
	SetPrev(prev Backward)
	GetNext() Forward
	GetPrev() Backward
}
