package automa

import (
	"context"
	"go.uber.org/zap"
	"sync"
	"time"
)

// Workflow implements AtomicWorkflow interface
// It implements a Saga workflow using Choreography execution pattern
//
// In order to enable Choreography pattern it forms a double linked list of AtomicSteps to traverse 'Forward'
// on Success and 'Backward' on Failure
type Workflow struct {
	id    string
	mutex sync.Mutex

	successStep *successStep
	failedStep  *failedStep

	// terminal steps
	firstStep AtomicStep
	lastStep  AtomicStep

	// local cache for accumulating report from all internal states
	// this is passed along to accumulate report from all internal states
	report *WorkflowReport

	logger  *zap.Logger
	stepIDs []string
}

// addStep add an AtomicStep in the internal double linked list of steps
func (wf *Workflow) addStep(s AtomicStep) {
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

// WorkflowOption exposes "constructor with option" pattern for Workflow
type WorkflowOption func(wf *Workflow)

// WithSteps allow Workflow to be initialized with the list of ordered steps
func WithSteps(steps ...AtomicStep) WorkflowOption {
	return func(wf *Workflow) {
		for _, step := range steps {
			wf.addStep(step)
			wf.stepIDs = append(wf.stepIDs, step.GetID())
		}
	}
}

// WithLogger allows Workflow to be initialized with a logger
// By default a Workflow is initialized with a NoOp logger
func WithLogger(logger *zap.Logger) WorkflowOption {
	return func(wf *Workflow) {
		wf.logger = logger
	}
}

// NewWorkflow returns an instance of WorkFlow that implements AtomicWorkflow interface
func NewWorkflow(id string, opts ...WorkflowOption) *Workflow {
	fs := &failedStep{}
	ss := &successStep{}

	wf := &Workflow{
		id:          id,
		failedStep:  fs,
		successStep: ss,
		report:      NewWorkflowReport(id, nil),
		logger:      zap.NewNop(),
	}

	for _, opt := range opts {
		opt(wf)
	}

	return wf
}

// GetID returns the id of the Workflow
func (wf *Workflow) GetID() string {
	return wf.id
}

// Start starts the workflow and returns the WorkflowReport
func (wf *Workflow) Start(ctx context.Context) (*WorkflowReport, error) {
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

// End performs any cleanup after the Workflow execution
// This is a NOOP currently, but left as  placeholder for any future cleanup steps if required
func (wf *Workflow) End(ctx context.Context) {
}
