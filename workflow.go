package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// workflow provides orchestration primitives for composing and executing
// Steps with structured reporting, error handling and rollback support.
//
// Overview:
//   - A workflow exposes a workflow-level StateBag via Workflow.State() that represents
//     workflow-wide state.
//   - Ordinary Steps receive the shared workflow StateBag (mutations are visible to later
//     steps and to the workflow).
//   - Steps that are themselves Workflows receive a cloned StateBag so sub-workflows cannot
//     unintentionally mutate parent workflow state. The parent workflow may still mutate its
//     own state between steps and thus pass different versions to subsequent steps/sub-workflows.
//   - Execute records per-step state snapshots to enable deterministic rollback. When a
//     rollback is triggered by execution failure, rollback routines operate against the state
//     snapshot that existed when each step executed. Direct calls to Rollback fall back to the
//     current workflow state (preserving prior behavior).
//
// Hooks and callbacks:
//   - A workflow-level prepare hook (w.prepare) can return a context used for subsequent
//     step Prepare/Execute calls. Each Step's Prepare is invoked after the step's StateBag
//     has been attached so Prepare can access state.
//   - onCompletion and onFailure callbacks are supported; when enableAsyncCallbacks is true,
//     reports are cloned and callbacks are invoked asynchronously.
//
// Execution/rollback modes:
//   - Execution and rollback semantics respect w.executionMode and w.rollbackMode (StopOnError,
//     ContinueOnError, RollbackOnError).
//   - Execute and Rollback produce aggregated Reports containing per-step reports and, when
//     applicable, per-step rollback reports.
type workflow struct {
	id                   string
	state                StateBag
	logger               zerolog.Logger
	steps                []Step
	executionMode        TypeMode
	rollbackMode         TypeMode
	enableAsyncCallbacks bool

	// callbacks and hooks
	prepare      PrepareFunc
	rollback     RollbackFunc // optional user-defined rollback function for the entire workflow
	onCompletion OnCompletionFunc
	onFailure    OnFailureFunc
}

func (w *workflow) WithState(s StateBag) Step {
	if w.state == s {
		// avoid redundant assignment when same state is provided
		return w
	}

	w.state = s
	return w
}

func IsWorkflow(stp Step) bool {
	_, ok := stp.(Workflow)
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
			WithWorkflow(wb.workflow),
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
			WithWorkflow(wb.workflow),
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
func (w *workflow) rollbackFrom(ctx context.Context, index int, states []StateBag) map[string]*Report {
	stepReports := map[string]*Report{}
	startTime := time.Now()

	for i := index; i >= 0; i-- {
		step := w.steps[i]

		// choose snapshot if provided, otherwise use workflow state
		var state StateBag
		if states != nil && i < len(states) && states[i] != nil {
			state = states[i]
		} else {
			state = w.State()
		}

		// pass the chosen state in context for the rollback
		stepCtx := context.WithValue(ctx, KeyState, state.
			Set(KeyId, w.Id()).
			Set(KeyIsWorkflow, true).
			Set(KeyStartTime, startTime),
		)

		rollbackReport := step.Rollback(stepCtx)

		if rollbackReport == nil {
			rollbackReport = FailureReport(step,
				WithWorkflow(w),
				WithActionType(ActionRollback),
				WithStartTime(startTime),
				WithError(StepExecutionError.New("workflow %q step %q returned nil report from Rollback", w.id, step.Id()).
					WithProperty(StepIdProperty, step.Id()),
				),
			)
		}

		// Ensure rollback report has ActionRollback set for consistency
		if rollbackReport.Action != ActionRollback {
			rollbackReport.Action = ActionRollback
		}

		stepReports[step.Id()] = rollbackReport

		if rollbackReport.IsFailed() {
			switch w.rollbackMode {
			case ContinueOnError:
				continue
			case StopOnError:
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
	preparedCtx := ctx
	if w.prepare != nil {
		c, err := w.prepare(preparedCtx, w)
		if err != nil {
			return nil, err
		}
		preparedCtx = c // use the context returned by user prepare function
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

// Execute runs the workflow by executing each step in sequence.
//
// Preparation and state handling:
//   - If a workflow-level `prepare` hook is configured it is invoked first with the incoming `ctx` and the
//     workflow instance. The returned context (if non-nil) is used for step preparation and execution.
//   - The workflow exposes a shared `StateBag` via `w.State()` that represents workflow-wide state.
//   - For ordinary steps the shared `StateBag` is used as-is (state is shared between workflow and step).
//   - For steps that are themselves workflows (detected with `IsWorkflow(step)`), `Execute` clones the
//     workflow state by calling `StateBag.Clone()` and provides the cloned `StateBag` to the sub-workflow.
//     This prevents unintended state sharing and isolates sub-workflow mutations from the parent workflow.
//   - If cloning the state fails or context preparation fails, the step is treated as failed and a failure `Report`
//     is produced; the step is not executed.
//   - After state preparation each step's `Prepare` hook is invoked; the context returned by the step's
//     `Prepare` (if any) is used for the step's execution.
//     A nil `Report` from a step is treated as a failure.
//
// Execution semantics:
//   - Execution behavior respects `w.executionMode` (StopOnError, ContinueOnError, RollbackOnError).
//   - When rollback is required, rollback reports from executed steps are attached to the corresponding
//     step reports and returned as part of the workflow `Report`.
func (w *workflow) Execute(ctx context.Context) *Report {
	startTime := time.Now()

	if w.steps == nil || len(w.steps) == 0 {
		return FailureReport(w,
			WithWorkflow(w),
			WithStartTime(startTime),
			WithActionType(ActionExecute),
			WithError(StepExecutionError.New("workflow %s has no steps to execute", w.id)))
	}

	var stepReports []*Report
	var hasFailed bool

	// capture per-step state snapshots for rollback
	stepStates := make([]StateBag, 0, len(w.steps))

	for index, step := range w.steps {
		var report *Report
		stepStart := time.Now()

		var stepCtx context.Context
		var stepState StateBag
		var statePrepError error
		var ctxPrepError error

		// prepare step state (start from current workflow state)
		stepState = w.State()
		if IsWorkflow(step) { // clone state for sub-workflows
			stepState, statePrepError = stepState.Clone()
			if statePrepError != nil {
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(StepExecutionError.
						Wrap(statePrepError, "workflow %q step %q failed to clone state for sub-workflow execution", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					))
			}
		}

		// attach snapshot for possible rollback (keeps alignment even if nil)
		stepStates = append(stepStates, stepState)

		// make sure step has its state before calling Prepare so Prepare can access it
		step = step.WithState(stepState)

		// prepare step context
		if statePrepError == nil {
			stepCtx, ctxPrepError = step.Prepare(ctx)
			if ctxPrepError != nil {
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(StepExecutionError.
						Wrap(ctxPrepError, "workflow %q step %q preparation failed", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					))
			}
		}

		// execute step if preparation succeeded
		if statePrepError == nil && ctxPrepError == nil {
			report = step.Execute(stepCtx)
			if report == nil {
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(StepExecutionError.New("workflow %q step %q returned nil report from Execute", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					),
				)
			}
		}

		// collect step report
		stepReports = append(stepReports, report)

		// check for step failure
		if report.IsFailed() {
			hasFailed = true

			if w.executionMode == StopOnError {
				break
			} else if w.executionMode == RollbackOnError {
				// perform rollback using recorded per-step states
				rollbackReports := w.rollbackFrom(stepCtx, index, stepStates)

				// Attach rollback reports to corresponding step reports
				for _, stepReport := range stepReports {
					if rollback, ok := rollbackReports[stepReport.Id]; ok {
						stepReport.Rollback = rollback
					}
				}

				break
			}
		}
	}

	if hasFailed {
		var failedStepIDs []string
		for _, sr := range stepReports {
			if sr.IsFailed() {
				failedStepIDs = append(failedStepIDs, sr.Id)
			}
		}

		workflowReport := FailureReport(w,
			WithWorkflow(w),
			WithStartTime(startTime),
			WithActionType(ActionExecute),
			WithError(StepExecutionError.New(
				"workflow %q completed with %d step failures: %v",
				w.id, len(failedStepIDs), failedStepIDs,
			)),
			WithStepReports(stepReports...))

		w.handleFailure(ctx, workflowReport)

		return workflowReport
	}

	workflowReport := SuccessReport(w,
		WithWorkflow(w),
		WithStartTime(startTime),
		WithActionType(ActionExecute),
		WithStepReports(stepReports...))

	w.handleCompletion(ctx, workflowReport)

	return workflowReport
}

// invokeRollbackFunc invokes the user-defined rollback function for the entire workflow.
// It ensures the returned report is valid and sets the appropriate action type.
func (w *workflow) invokeRollbackFunc(ctx context.Context) *Report {
	workflowReport := w.rollback(ctx, w)
	if workflowReport == nil {
		return FailureReport(w,
			WithWorkflow(w),
			WithActionType(ActionRollback),
			WithError(StepExecutionError.New("workflow %q returned nil report from Rollback", w.id)),
		)
	}

	if workflowReport.IsFailed() {
		if workflowReport.Error == nil {
			// this should not happen, but just in case
			workflowReport.Error = StepExecutionError.New("workflow %q rollback failed", w.id)
		}

		return FailureReport(w,
			WithWorkflow(w),
			WithReport(workflowReport),
			WithActionType(ActionRollback),
		)
	}

	if workflowReport.Action != ActionRollback {
		workflowReport.Action = ActionRollback
	}

	return SuccessReport(w,
		WithWorkflow(w),
		WithReport(workflowReport),
		WithActionType(ActionRollback),
	)
}

func (w *workflow) Rollback(ctx context.Context) *Report {
	if w.rollback != nil {
		return w.invokeRollbackFunc(ctx)
	}

	startTime := time.Now()
	stepCtx := context.WithValue(ctx, KeyState, w.State().
		Set(KeyId, w.Id()).
		Set(KeyIsWorkflow, true).
		Set(KeyStartTime, startTime),
	)
	// call with nil states so rollbackFrom falls back to current workflow state
	rollbackReports := w.rollbackFrom(stepCtx, len(w.steps)-1, nil)

	var stepReports []*Report
	for _, step := range w.steps {
		report, ok := rollbackReports[step.Id()]
		if !ok || report == nil {
			report = FailureReport(step,
				WithWorkflow(w),
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
		WithWorkflow(w),
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
		go w.onCompletion(ctx, w, clonedReport)
	} else {
		w.onCompletion(ctx, w, report)
	}
}

func (w *workflow) handleFailure(ctx context.Context, report *Report) {
	if w.onFailure == nil {
		return
	}

	if w.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go w.onFailure(ctx, w, clonedReport)
	} else {
		w.onFailure(ctx, w, report)
	}
}

func newDefaultWorkflow() *workflow {
	return &workflow{
		executionMode: StopOnError,
		rollbackMode:  ContinueOnError,
		logger:        zerolog.Nop(),
	}
}
