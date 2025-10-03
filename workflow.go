package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

type workflow struct {
	id                   string
	ctx                  context.Context
	state                StateBag
	prepare              PrepareFunc
	steps                []Step
	logger               zerolog.Logger
	rollbackMode         TypeRollbackMode
	onCompletion         OnCompletionFunc
	onFailure            OnFailureFunc
	enableAsyncCallbacks bool
}

func IsWorkflow(step Step) bool {
	_, ok := step.(Workflow)
	return ok
}

// rollbackFrom rollbacks the workflow backward from the given index to the start.
func (w *workflow) rollbackFrom(ctx context.Context, index int) map[string]*Report {
	stepReports := map[string]*Report{}
	for i := index; i >= 0; i-- {
		step := w.steps[i]

		rollbackReport := step.Rollback(ctx)
		stepReports[step.Id()] = rollbackReport

		if rollbackReport.Error != nil {
			switch w.rollbackMode {
			case RollbackModeContinueOnError:
				continue
			case RollbackModeStopOnError:
				return stepReports
			}
		}
	}

	return stepReports
}

func (w *workflow) Id() string {
	return w.id
}

func (w *workflow) Prepare(ctx context.Context) (context.Context, error) {
	w.state = GetStateBagFromContext(ctx)

	if w.prepare != nil {
		c, err := w.prepare(ctx)
		if err != nil {
			return nil, err
		}

		w.ctx = c
	}

	return w.ctx, nil
}

func (w *workflow) Steps() []Step {
	return w.steps
}

func (w *workflow) State() StateBag {
	if w.state == nil {
		w.state = &SyncStateBag{} // lazy initialization
	}

	return w.state
}

func (w *workflow) Execute(ctx context.Context) *Report {
	startTime := time.Now()

	if w.steps == nil || len(w.steps) == 0 {
		return FailureReport(w,
			WithStartTime(startTime),
			WithActionType(ActionExecute), WithStartTime(startTime),
			WithError(StepExecutionError.New("workflow %s has no steps to execute", w.id)))
	}

	var stepReports []*Report
	for index, step := range w.steps {
		stepState := w.State().
			Set(KeyId, step.Id()).
			Set(KeyIsWorkflow, IsWorkflow(step)).
			Set(KeyStartTime, startTime)
		stepCtx, err := step.Prepare(context.WithValue(ctx, KeyState, stepState))
		if err != nil {
			return FailureReport(w,
				WithStartTime(startTime),
				WithActionType(ActionExecute),
				WithError(StepExecutionError.
					Wrap(err, "workflow %q step %q preparation failed: %v", w.id, step.Id(), err).
					WithProperty(StepIdProperty, step.Id()),
				))
		}

		report := step.Execute(stepCtx)
		stepReports = append(stepReports, report)

		if report.Error != nil {
			// Perform rollback for all executed steps up to the current one
			rollbackReports := w.rollbackFrom(stepCtx, index)

			// Attach rollback reports to corresponding step reports
			for _, stepReport := range stepReports {
				if rollback, ok := rollbackReports[stepReport.Id]; ok {
					stepReport.Rollback = rollback
				}
			}

			workflowReport := FailureReport(w,
				WithStartTime(startTime),
				WithActionType(ActionExecute),
				WithError(StepExecutionError.
					Wrap(report.Error, "workflow %q failed at step %q", w.id, step.Id()).
					WithProperty(StepIdProperty, step.Id()),
				),
				WithStepReports(stepReports...))

			w.handleFailure(ctx, workflowReport)

			return workflowReport
		}
	}

	workflowReport := SuccessReport(w,
		WithStartTime(startTime),
		WithActionType(ActionExecute),
		WithStepReports(stepReports...))

	w.handleCompletion(ctx, workflowReport)

	return workflowReport
}

func (w *workflow) Rollback(ctx context.Context) *Report {
	startTime := time.Now()
	stepCtx := context.WithValue(ctx, KeyState, w.State().
		Set(KeyId, w.Id()).
		Set(KeyIsWorkflow, true).
		Set(KeyStartTime, startTime),
	)
	rollbackReports := w.rollbackFrom(stepCtx, len(w.steps)-1)

	var stepReports []*Report
	for _, step := range w.steps {
		if report, ok := rollbackReports[step.Id()]; ok {
			stepReports = append(stepReports, report)
		}
	}

	return SuccessReport(w,
		WithActionType(ActionRollback),
		WithStartTime(startTime),
		WithStepReports(stepReports...))
}

func (w *workflow) handleCompletion(ctx context.Context, report *Report) {
	// any post successful execution logic can be added here
	// no-op for now
	if w.onCompletion == nil {
		return
	}

	if w.enableAsyncCallbacks {
		go w.onCompletion(ctx, report)
	} else {
		w.onCompletion(ctx, report)
	}
}

func (w *workflow) handleFailure(ctx context.Context, report *Report) {
	if w.onFailure == nil {
		return
	}

	if w.enableAsyncCallbacks {
		go w.onFailure(ctx, report)
	} else {
		w.onFailure(ctx, report)
	}
}

func newWorkflow() *workflow {
	return &workflow{
		rollbackMode: RollbackModeContinueOnError,
		logger:       zerolog.Nop(),
	}
}
