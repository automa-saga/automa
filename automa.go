package automa

// Step provides interface for a transactional step within a workflow.
// Note that a Step may skip rollback if that makes sense and in that case it is not Transactional in nature.
type Step interface {
	// GetID returns the step ID
	GetID() string

	Forward
	Backward
	Choreographer
}

// Registry is a registry for steps
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

// Workflow defines interface for a workflow
type Workflow interface {
	// GetID returns the workflow ID
	GetID() string

	// Execute starts the workflow and returns the WorkflowReport
	Execute(ctx *Context) (WorkflowReport, error)

	// HasStep checks if the workflow has a Step with the given stepID
	HasStep(stepID string) bool

	// GetStepSequence returns the ordered list of Step IDs in the workflow
	GetStepSequence() []string

	// GetSteps returns the ordered list of Step in the workflow
	GetSteps() []Step
}

// Forward defines the methods to execute business logic of a Step and move the workflow forward
type Forward interface {
	Execute(ctx *Context, prevSuccess *Success) (WorkflowReport, error)
}

// Backward defines the methods to be executed to move the workflow backward on error
type Backward interface {
	Reverse(ctx *Context, prevFailure *Failure) (WorkflowReport, error)
}

// Choreographer interface defines the methods to support double link list of states
// This is needed to support Choreography execution of the Saga workflow
type Choreographer interface {
	SetNext(next Forward)
	SetPrev(prev Backward)
	GetNext() Forward
	GetPrev() Backward
	Reset() Step
}
