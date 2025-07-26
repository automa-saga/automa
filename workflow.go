package automa

import (
	"context"
	"github.com/rs/zerolog"
	"sync"
	"time"
)

var nolog = zerolog.Nop()

// workflow implements Workflow interface
// It implements a Saga workflow using Choreography execution pattern
//
// In order to enable Choreography pattern it forms a double linked list of AtomicSteps to traverse 'Forward'
// on Success and 'Backward' on Failure
type workflow struct {
	id    string
	mutex sync.Mutex

	successStep *successStep
	failedStep  *failedStep

	// terminal steps
	firstStep Step
	lastStep  Step

	// local cache for accumulating report from all internal states
	// this is passed along to accumulate report from all internal states
	report WorkflowReport

	logger  *zerolog.Logger
	stepIDs []string
}

// addStep add a Step in the internal double linked list of steps
func (wf *workflow) addStep(s Step) {
	if wf.firstStep == nil {
		wf.firstStep = s
		wf.firstStep.SetPrev(wf.failedStep)
	} else {
		wf.lastStep.SetNext(s)
		s.SetPrev(wf.lastStep)
	}

	wf.lastStep = s
	wf.lastStep.SetNext(wf.successStep)
}

// WorkflowOption exposes "constructor with option" pattern for workflow
type WorkflowOption func(wf *workflow)

// WithSteps allow workflow to be initialized with the list of ordered steps
func WithSteps(steps ...Step) WorkflowOption {
	return func(wf *workflow) {
		for _, step := range steps {
			wf.addStep(step)
			wf.stepIDs = append(wf.stepIDs, step.GetID())
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
	fs := &failedStep{}
	ss := &successStep{}
	report := NewWorkflowReport(id, nil)

	wf := &workflow{
		id:          id,
		failedStep:  fs,
		successStep: ss,
		report:      *report,
		logger:      &nolog,
	}

	for _, opt := range opts {
		opt(wf)
	}

	return wf
}

// GetID returns the id of the workflow
func (wf *workflow) GetID() string {
	return wf.id
}

// Start starts the workflow and returns the WorkflowReport
func (wf *workflow) Start(ctx context.Context) (WorkflowReport, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var err error

	if wf.firstStep != nil {
		wf.report.StepSequence = wf.stepIDs
		wf.report.Status = StatusUndefined

		wf.report, err = wf.firstStep.Run(ctx, NewStartTrigger(wf.report))
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

// End performs any cleanup after the workflow execution
// This is a NOOP currently, but left as  placeholder for any future cleanup steps if required
func (wf *workflow) End(ctx context.Context) {
}
