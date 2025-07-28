package automa

// Saga interface defines the methods to support Saga pattern where steps can be executed and rolled back.
type Saga interface {
	// Execute executes the step in the forward direction.
	Execute(ctx *Context) (*WorkflowReport, error)

	// Reverse rolls back the step in the backward direction.
	Reverse(ctx *Context) (*WorkflowReport, error)
}

// Choreographer interface defines the methods to support double link list of workflow steps.
// This is needed to support Choreography execution of the Saga workflow.
type Choreographer interface {
	// SetNext and SetPrev are used to set the next and previous steps in the double linked list of steps
	SetNext(next Step)
	SetPrev(prev Step)

	// GetNext and GetPrev are used to get the next and previous steps in the double linked list of steps
	GetNext() Step
	GetPrev() Step

	// Reset resets the step to its initial state
	Reset() Step
}

// Step provides interface for a transactional step within a workflow.
// Note that a Step may skip rollback if that makes sense and in that case it is not Transactional in nature.
type Step interface {
	GetID() string
	Saga
	Choreographer
}

// Registry is a registry for steps.
type Registry interface {
	// AddStep adds a Step to the registry
	AddStep(step Step) Registry

	// AddSteps adds multiple Steps to the registry
	AddSteps(step ...Step) Registry

	// GetStep returns a Step from the registry given the id
	// If it doesn't exist, it returns nil. So the caller should handle the nil Step safely.
	GetStep(id string) Step

	// GetSteps returns all Steps in the registry
	GetSteps() []Step

	// BuildWorkflow builds a Workflow comprising the list of Step identified by ids
	BuildWorkflow(workflowID string, stepIDs []string) (Workflow, error)
}

// Workflow defines interface for a workflow.
type Workflow interface {
	// GetID returns the workflow ID
	GetID() string

	// Execute starts the workflow and returns the WorkflowReport
	Execute(ctx *Context) (*WorkflowReport, error)

	// HasStep checks if the workflow has a Step with the given stepID
	HasStep(stepID string) bool

	// GetStepSequence returns the ordered list of Step IDs in the workflow
	GetStepSequence() []string

	// GetSteps returns the ordered list of Step in the workflow
	GetSteps() []Step
}
