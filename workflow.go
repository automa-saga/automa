package automa

import (
	"github.com/rs/zerolog"
	"sync"
	"time"
)

var nolog = zerolog.Nop()

// workflow implements Workflow interface
// It implements a Saga workflow using Choreography execution pattern
//
// In order to enable Choreography pattern it forms a double linked list of AtomicSteps to traverse 'Execute'
// on Success and 'Reverse' on Failure
type workflow struct {
	id    string
	mutex sync.Mutex

	// terminal steps to maintain the double linked list of steps
	firstStep Step
	lastStep  Step

	// local cache for accumulating report from all internal states
	// this is passed along to accumulate report from all internal states
	report WorkflowReport

	logger *zerolog.Logger

	// stepSequence is the ordered list of Step IDs in the workflow
	// This is cached to avoid traversing the double linked list of steps
	// It is used to preserve the ordered list of Step IDs in the workflow since the steps is a map
	// and does not maintain order
	stepSequence []string

	// steps is a map of Step ID to Step
	// This is used to quickly check if a Step exists in the workflow
	steps map[string]Step
}

// addStep add a Step in the internal double linked list of steps
func (wf *workflow) addStep(s Step) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	if wf.firstStep == nil {
		wf.firstStep = s
	} else {
		wf.lastStep.SetNext(s)
		s.SetPrev(wf.lastStep)
	}

	// cache the step in the steps map and stepSequence for quick access
	if wf.steps == nil {
		wf.steps = make(map[string]Step)
	}
	wf.steps[s.GetID()] = s
	wf.stepSequence = append(wf.stepSequence, s.GetID())

	// update the last step to the current step
	wf.lastStep = s
}

// GetID returns the id of the workflow
func (wf *workflow) GetID() string {
	return wf.id
}

// Start starts the workflow and returns the WorkflowReport
func (wf *workflow) Execute(ctx *Context) (WorkflowReport, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var err error

	if wf.firstStep != nil {
		wf.report.StepSequence = wf.stepSequence
		wf.report.Status = StatusUndefined

		wf.report, err = wf.firstStep.Execute(ctx, NewStartTrigger(wf.report))
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

// GetSteps returns all Steps in the workflow in sequence
// It returns a copy of the steps to avoid external modification, so avoid calling this method in a loop in order to
// avoid memory overhead
func (wf *workflow) GetSteps() []Step {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	// Return a copy of the steps to avoid external modification
	steps := make([]Step, 0, len(wf.stepSequence))
	for _, stepID := range wf.stepSequence {
		if step, ok := wf.steps[stepID]; ok {
			steps = append(steps, step)
		}
	}

	return steps
}

// GetStepSequence returns the ordered list of Step IDs in the workflow
func (wf *workflow) GetStepSequence() []string {
	return wf.stepSequence
}

// HasStep checks if the workflow has a Step with the given stepID
func (wf *workflow) HasStep(stepID string) bool {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	if stepID == "" {
		return false
	}

	if step, exists := wf.steps[stepID]; exists && step != nil {
		return true
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

// WithLogger allows workflow to be initialized with a logx
// By default a workflow is initialized with a NoOp logx
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
		report: *report,
		logger: &nolog,
	}

	for _, opt := range opts {
		opt(wf)
	}

	return wf
}
