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
	*Task

	// mutex ensures thread-safe access to the workflow's state.
	mutex sync.Mutex

	// first and last are terminal steps for the double linked list.
	first Step
	last  Step

	// report contains execution details of the workflow.
	report *WorkflowReport

	// logger for workflow-level logging.
	logger *zerolog.Logger

	stepSequence []string // Ordered list of Step IDs for execution order.

	stepMap map[string]Step
}

// addStep adds a Step to the internal double linked list.
// Ensures thread safety and maintains correct order.
func (wf *workflow) addStep(s Step) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	// add into the double linked list of steps
	if wf.first == nil {
		wf.first = s
	} else {
		wf.last.SetNext(s)
		s.SetPrev(wf.last)
	}
	wf.last = s

	// cache the step in the map for quick lookup
	if wf.stepMap == nil {
		wf.stepMap = make(map[string]Step)
	}
	wf.stepMap[s.GetID()] = s

	// append the step ID to the sequence for ordered execution
	if wf.stepSequence == nil {
		wf.stepSequence = make([]string, 0, 5) // initial capacity for performance
	}
	wf.stepSequence = append(wf.stepSequence, s.GetID())
}

// Execute starts the workflow execution and returns a WorkflowReport.
// Traverses steps forward on success, and updates report status accordingly.
func (wf *workflow) Execute(ctx *Context) (*WorkflowReport, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	wf.report.StepSequence = wf.GetStepSequence()

	if wf.first != nil {
		var err error

		// Start execution
		// If there is an error, it will roll back and return the final report.
		wf.report, err = wf.first.Execute(ctx.SetValue(KeyPrevSuccess, NewStartTrigger(wf.report)))
		if err != nil {
			wf.report.Status = StatusFailed
		} else {
			wf.report.Status = StatusSuccess
		}

		wf.report.EndTime = time.Now()
		return wf.report, err
	}

	// if we have no steps, we return the report as is
	return wf.report, nil
}

// GetSteps returns all Steps in the workflow in execution order.
// Returns a copy to prevent external modification.
func (wf *workflow) GetSteps() []Step {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	steps := make([]Step, 0, len(wf.stepMap))
	for _, step := range wf.stepMap {
		steps = append(steps, step)
	}
	return steps
}

// GetStepSequence returns the ordered list of Step IDs in the workflow.
// Returns a copy to prevent external modification.
func (wf *workflow) GetStepSequence() []string {
	return wf.stepSequence
}

// HasStep checks if the workflow contains a Step with the given stepID.
func (wf *workflow) HasStep(stepID string) bool {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	_, exists := wf.stepMap[stepID]
	return exists
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
	wf := &workflow{
		Task: &Task{
			ID: id,
		},
		logger:       &nolog,
		stepSequence: make([]string, 0, 5), // initial capacity for performance
		stepMap:      make(map[string]Step),
		report:       NewWorkflowReport(id, []string{}),
	}
	for _, opt := range opts {
		opt(wf)
	}
	return wf
}
