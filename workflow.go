package automa

import (
	"github.com/rs/zerolog"
	"sync"
	"time"
)

var nolog = zerolog.Nop()

// workflow implements the Workflow interface.
// It manages a Saga workflow using a double linked list of Steps,
// enabling forward execution on success and reverse traversal on failure.
type workflow struct {
	id    string
	mutex sync.Mutex

	// first and last are terminal steps for the double linked list.
	first Step
	last  Step

	// report contains execution details of the workflow.
	report *WorkflowReport

	// logger for workflow-level logging.
	logger *zerolog.Logger
}

// addStep adds a Step to the internal double linked list.
// Ensures thread safety and maintains correct order.
func (wf *workflow) addStep(s Step) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	if wf.first == nil {
		wf.first = s
	} else {
		wf.last.SetNext(s)
		s.SetPrev(wf.last)
	}
	wf.last = s
}

// GetID returns the workflow's unique identifier.
func (wf *workflow) GetID() string {
	return wf.id
}

// Execute starts the workflow execution and returns a WorkflowReport.
// Traverses steps forward on success, and updates report status accordingly.
func (wf *workflow) Execute(ctx *Context) (*WorkflowReport, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var err error

	if wf.first != nil {
		wf.report.StepSequence = wf.GetStepSequence()
		wf.report.Status = StatusUndefined

		// Start execution from the first step.
		wf.report, err = wf.first.Execute(ctx.SetValue(KeyPrevSuccess, NewStartTrigger(wf.report)))
		if err != nil {
			wf.report.Status = StatusFailed
		} else {
			wf.report.Status = StatusSuccess
		}
		wf.report.EndTime = time.Now()
		return wf.report, err
	}
	return wf.report, nil
}

// GetSteps returns all Steps in the workflow in execution order.
// Returns a copy to prevent external modification.
func (wf *workflow) GetSteps() []Step {
	var steps []Step
	for step := wf.first; step != nil; step = step.GetNext() {
		steps = append(steps, step)
	}
	return steps
}

// GetStepSequence returns the ordered list of Step IDs in the workflow.
// Returns a copy to prevent external modification.
func (wf *workflow) GetStepSequence() []string {
	var stepSequence []string
	for step := wf.first; step != nil; step = step.GetNext() {
		stepSequence = append(stepSequence, step.GetID())
	}
	return stepSequence
}

// HasStep checks if the workflow contains a Step with the given stepID.
func (wf *workflow) HasStep(stepID string) bool {
	if stepID == "" {
		return false
	}
	for step := wf.first; step != nil; step = step.GetNext() {
		if step.GetID() == stepID {
			return true
		}
	}
	return false
}

// WorkflowOption defines a functional option for configuring a workflow.
type WorkflowOption func(wf *workflow)

// WithSteps initializes the workflow with an ordered list of steps.
func WithSteps(steps ...Step) WorkflowOption {
	return func(wf *workflow) {
		for _, step := range steps {
			wf.addStep(step)
		}
	}
}

// WithLogger sets a custom logger for the workflow.
func WithLogger(logger *zerolog.Logger) WorkflowOption {
	return func(wf *workflow) {
		if logger != nil {
			wf.logger = logger
		}
	}
}

// NewWorkflow creates a new Workflow instance with the given ID and options.
// Applies all provided WorkflowOptions for configuration.
func NewWorkflow(id string, opts ...WorkflowOption) Workflow {
	report := NewWorkflowReport(id, nil)
	wf := &workflow{
		id:     id,
		report: report,
		logger: &nolog,
	}
	for _, opt := range opts {
		opt(wf)
	}
	return wf
}
