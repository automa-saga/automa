package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

type workflow struct {
	id                   string
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

// RunWorkflow builds and runs the workflow from the given WorkflowBuilder.
// It returns a Report summarizing the execution result.
// If the workflow fails to build or prepare, it returns a failure Report with the corresponding error.
// Note if the prepare step fails, no rollback is performed or handleFailure isn't invoked as no steps have been executed yet.
// If preparation files, it returns error with ActionType set to ActionPrepare so that caller can distinguish it from execution errors.
func RunWorkflow(ctx context.Context, wb *WorkflowBuilder) *Report {
	start := time.Now()
	wf, err := wb.Build()
	if err != nil {
		return NewReport(wb.Id(),
			WithIsWorkflow(true),
			WithStatus(StatusFailed),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(StepExecutionError.
				Wrap(err, "workflow %q build failed", wb.Id()).
				WithProperty(StepIdProperty, wb.Id()),
			))
	}

	preparedCtx, err := wf.Prepare(ctx)
	if err != nil {
		return FailureReport(wf,
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(StepExecutionError.
				Wrap(err, "workflow %q preparation failed: %v", wf.Id(), err).
				WithProperty(StepIdProperty, wf.Id()),
			))
	}

	return wf.Execute(preparedCtx)
}

// rollbackFrom rollbacks the workflow backward from the given index to the start.
func (w *workflow) rollbackFrom(ctx context.Context, index int) map[string]*Report {
	stepReports := map[string]*Report{}
	for i := index; i >= 0; i-- {
		step := w.steps[i]

		rollbackReport := step.Rollback(ctx)

		// Ensure rollback report has ActionRollback set for consistency
		if rollbackReport.Action != ActionRollback {
			rollbackReport.Action = ActionRollback
		}

		stepReports[step.Id()] = rollbackReport

		if rollbackReport.IsFailed() {
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
	if w.state == nil {
		w.state = &SyncStateBag{}
	}

	// merge state and w.state if w.state is already initialized
	state := StateFromContext(ctx)
	if state != nil {
		w.state.Merge(state)
	}

	preparedCtx := context.WithValue(ctx, KeyState, w.state)
	if w.prepare != nil {
		c, err := w.prepare(preparedCtx)
		if err != nil {
			return nil, err
		}
		preparedCtx = c
	}

	return preparedCtx, nil
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
			WithActionType(ActionExecute),
			WithError(StepExecutionError.New("workflow %s has no steps to execute", w.id)))
	}

	var stepReports []*Report
	for index, step := range w.steps {
		var report *Report
		stepStart := time.Now()
		stepState := w.State().Clone().
			Set(KeyId, step.Id()).
			Set(KeyIsWorkflow, IsWorkflow(step)).
			Set(KeyStartTime, stepStart)
		stepCtx, err := step.Prepare(context.WithValue(ctx, KeyState, stepState))
		if err != nil {
			report = FailureReport(step,
				WithStartTime(stepStart),
				WithActionType(ActionExecute),
				WithError(StepExecutionError.
					Wrap(err, "workflow %q step %q preparation failed", w.id, step.Id()).
					WithProperty(StepIdProperty, step.Id()),
				))

			if stepCtx == nil {
				stepCtx = ctx
			}
		} else {
			report = step.Execute(stepCtx)
			if report == nil {
				report = FailureReport(step,
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(StepExecutionError.New("workflow %q step %q returned nil report from Execute", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					),
				)
			}
		}

		stepReports = append(stepReports, report)

		if !report.IsSuccess() {
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

			w.handleFailure(stepCtx, workflowReport)

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
		report, ok := rollbackReports[step.Id()]
		if !ok || report == nil {
			report = FailureReport(step,
				WithActionType(ActionRollback),
				WithStartTime(startTime),
				WithError(StepExecutionError.New("workflow %q step %q returned nil report from Rollback", w.id, step.Id()).
					WithProperty(StepIdProperty, step.Id()),
				),
			)
		}
		stepReports = append(stepReports, report)
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
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go w.onCompletion(ctx, clonedReport)
	} else {
		w.onCompletion(ctx, report)
	}
}

func (w *workflow) handleFailure(ctx context.Context, report *Report) {
	if w.onFailure == nil {
		return
	}

	if w.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go w.onFailure(ctx, clonedReport)
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
