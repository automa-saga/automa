package automa

import (
	"context"
	"go.uber.org/zap"
	"sync"
)

// Workflow defines the structure of a Workflow
type Workflow struct {
	mutex       sync.Mutex
	successStep *successStep
	failedStep  *failedStep
	firstStep   RollbackableStep
	lastStep    RollbackableStep
	reports     Reports
	logger      *zap.Logger
}

type Option func(wf *Workflow)

// WithSteps allow Workflow to be initialized with the list of ordered steps
func WithSteps(steps ...RollbackableStep) Option {
	return func(wf *Workflow) {
		for _, step := range steps {
			wf.addStep(step)
		}
	}
}

// WithLogger allows Workflow to be initialized with a logger
// By default a Workflow is initialized with a NoOp logger
func WithLogger(logger *zap.Logger) Option {
	return func(wf *Workflow) {
		wf.logger = logger
	}
}

// NewWorkflow returns an instance of WorkFlow that implements WorkflowEngine interface
func NewWorkflow(opts ...Option) *Workflow {
	fs := &failedStep{}
	ss := &successStep{}

	wf := &Workflow{
		failedStep:  fs,
		successStep: ss,
		reports:     Reports{},
		logger:      zap.NewNop(),
	}

	for _, opt := range opts {
		opt(wf)
	}

	return wf
}

func (wf *Workflow) Start(ctx context.Context) (Reports, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var err error

	if wf.firstStep != nil {
		wf.reports, err = wf.firstStep.Run(ctx, NewStartTrigger(wf.reports))
		return wf.reports, err
	}

	return wf.reports, nil
}

func (wf *Workflow) End(ctx context.Context) {
	// placeholder for any future cleanup
}

// addStep add a step in the internal double linked list of steps
func (wf *Workflow) addStep(s RollbackableStep) {
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
