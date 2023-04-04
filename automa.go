package automa

import (
	"context"
)

// Forward defines the methods to execute business logic of an AtomicStep and move the workflow forward
type Forward interface {
	// Run runs the business logic to be performed in the AtomicStep
	Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, error)
}

// Backward defines the methods to be executed to move the workflow backward on error
type Backward interface {
	// Rollback defines the actions compensating the business logic executed in Run method
	// A step may skip rollback if that makes sense. In that case it would mean the AtomicStep is not Atomic in nature.
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

// AtomicStep provides interface for an atomic state
// Note that an AtomicStep may skip rollback if that makes sense and in that case it is not Atomic in nature.
type AtomicStep interface {
	// GetID returns the step ID
	GetID() string

	Forward
	Backward
	Choreographer
}

// StepIDs is just a wrapper definition for a list of string
type StepIDs []string

// AtomicStepRegistry is a registry of rollbackable steps
type AtomicStepRegistry interface {
	// RegisterSteps registers a set of AtomicStep
	// steps argument is a map where key is the step identifier
	RegisterSteps(steps map[string]AtomicStep) AtomicStepRegistry

	// GetStep returns an AtomicStep from the registry given the id
	// If it doesn't exist, it returns nil. So the caller should handle the nil AtomicStep safely.
	GetStep(id string) AtomicStep

	// BuildWorkflow builds an AtomicWorkflow comprising the list of AtomicStep identified by ids
	BuildWorkflow(id string, stepIDs StepIDs) (AtomicWorkflow, error)
}

// AtomicWorkflow defines interface for a Workflow
type AtomicWorkflow interface {
	// GetID returns the workflow ID
	GetID() string

	// Start starts the AtomicWorkflow execution
	Start(ctx context.Context) (WorkflowReport, error)

	// End performs cleanup after the AtomicWorkflow engine finish its execution
	End(ctx context.Context)
}
