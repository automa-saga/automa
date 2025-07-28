package automa

import (
	"github.com/rs/zerolog"
	"sync"
	"time"
)

var nolog = zerolog.Nop()

// workflow implements Workflow interface.
// It implements a Saga workflow using Choreography execution pattern
// In order to enable Choreography pattern it forms a double linked list of Steps to traverse 'Execute'
// on Success and 'Reverse' on Failure
type workflow struct {
	id    string
	mutex sync.Mutex

	// terminal steps to maintain the double linked list of steps
	first Step
	last  Step

	// local cache for accumulating report from all internal states
	// this is passed along to accumulate report from all internal states
	report *WorkflowReport

	logger *zerolog.Logger
}

// addStep add a Step in the internal double linked list of steps
func (wf *workflow) addStep(s Step) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	if wf.first == nil {
		wf.first = s
	} else {
		wf.last.SetNext(s)
		s.SetPrev(wf.last)
	}

	// update the last step to the current step
	wf.last = s
}

// GetID returns the id of the workflow
func (wf *workflow) GetID() string {
	return wf.id
}

// Execute starts the workflow and returns the WorkflowReport
func (wf *workflow) Execute(ctx *Context) (*WorkflowReport, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var err error

	if wf.first != nil {
		wf.report.StepSequence = wf.GetStepSequence()
		wf.report.Status = StatusUndefined

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

// GetSteps returns all Steps in the workflow in sequence.
func (wf *workflow) GetSteps() []Step {
	// Return a copy of the steps to avoid external modification
	var steps []Step
	for step := wf.first; step != nil; step = step.GetNext() {
		steps = append(steps, step)
	}

	return steps
}

// GetStepSequence returns the ordered list of Step IDs in the workflow
func (wf *workflow) GetStepSequence() []string {
	// Return a copy of the step sequence to avoid external modification
	var stepSequence []string
	for step := wf.first; step != nil; step = step.GetNext() {
		stepSequence = append(stepSequence, step.GetID())
	}

	return stepSequence
}

// HasStep checks if the workflow has a Step with the given stepID
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

// WorkflowOption exposes "constructor with option" pattern for workflow
type WorkflowOption func(wf *workflow)

// WithSteps allow workflow to be initialized with the list of ordered steps
func WithSteps(steps ...Step) WorkflowOption {
	return func(wf *workflow) {
		for _, step := range steps {
			wf.addStep(step)
		}
	}
}

// WithLogger allows workflow to be initialized with a logger.
func WithLogger(logger *zerolog.Logger) WorkflowOption {
	return func(wf *workflow) {
		if logger != nil {
			wf.logger = logger
		}
	}
}

// NewWorkflow returns an instance of WorkFlow that implements Workflow interface
func NewWorkflow(id string, opts ...WorkflowOption) Workflow {
	//fs := &failedStep{}
	//ss := &successStep{}
	report := NewWorkflowReport(id, nil)

	wf := &workflow{
		id: id,
		//failedStep:  fs,
		//successStep: ss,
		report: report,
		logger: &nolog,
	}

	for _, opt := range opts {
		opt(wf)
	}

	return wf
}
