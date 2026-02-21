package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// workflow provides orchestration primitives for composing and executing
// Steps with structured reporting, error handling and rollback support.
//
// Thread-safety:
//   - Workflow instances are NOT thread-safe and must not be shared across goroutines.
//   - Each workflow instance is designed for single execution. Create a new instance for
//     concurrent executions.
//   - Callbacks (onCompletion, onFailure) may run asynchronously but operate on cloned
//     reports, ensuring the workflow instance itself is not accessed concurrently.
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
	state                NamespacedStateBag
	logger               zerolog.Logger
	steps                []Step
	executionMode        TypeMode
	rollbackMode         TypeMode
	enableAsyncCallbacks bool

	// preserve step states after execution for potential rollback; keyed by step ID
	lastExecutionStates map[string]NamespacedStateBag

	// callbacks and hooks
	prepare      PrepareFunc
	rollback     RollbackFunc // optional user-defined rollback function for the entire workflow
	onCompletion OnCompletionFunc
	onFailure    OnFailureFunc
}

func (w *workflow) WithState(s NamespacedStateBag) Step {
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
func (w *workflow) rollbackFrom(ctx context.Context, index int, states map[string]NamespacedStateBag) map[string]*Report {
	stepReports := map[string]*Report{}
	startTime := time.Now()

	for i := index; i >= 0; i-- {
		step := w.steps[i]

		// choose snapshot if provided, otherwise use workflow state
		var state NamespacedStateBag
		if states != nil {
			if snapshot, ok := states[step.Id()]; ok {
				state = snapshot
			} else {
				state = w.State()
			}
		} else {
			state = w.State()
		}

		// Update the step's state to the snapshot before calling Rollback
		step = step.WithState(state)

		rollbackReport := step.Rollback(ctx)

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

func (w *workflow) State() NamespacedStateBag {
	if w.state == nil {
		// lazy initialization with empty local and global namespaces
		w.state = NewNamespacedStateBag(nil, nil)
	}

	return w.state
}

// Execute runs the workflow by executing each step in sequence.
//
// Preparation and state handling:
//  1. If a workflow-level `prepare` hook is configured it is invoked first with the incoming `ctx` and the
//     workflow instance. The returned context (if non-nil) is used for step preparation and execution.
//  2. The workflow exposes a shared `NamespacedStateBag` via `w.State()` that represents workflow-wide state.
//  3. For ordinary steps, a new `NamespacedStateBag` is created with:
//     a. An empty local namespace (isolated to the step)
//     b. A shared global namespace (points to the workflow's global state)
//  4. For steps that are themselves workflows (detected with `IsWorkflow(step)`), `Execute` creates a new
//     `NamespacedStateBag` with:
//     a. An empty local namespace (isolated to the sub-workflow)
//     b. A cloned global namespace (inherits parent's shared state but prevents mutations from
//     propagating back to the parent workflow)
//  5. This ensures sub-workflows have access to parent's global state but cannot mutate it, while
//     ordinary steps share the global namespace and can mutate it (visible to later steps).
//
// State snapshot and rollback:
//  1. After each step executes (successfully or not), its state is cloned and stored in `stepStates`
//     (keyed by step ID) for potential rollback.
//  2. State cloning ensures immutable snapshots (global state is shared across steps, so cloning
//     prevents later mutations from affecting earlier snapshots).
//  3. If state cloning fails, the step's current (non-cloned) state reference is stored instead. This:
//     a. Ensures rollback can access the step's actual execution state (including partial success)
//     b. Accepts the risk that later steps may mutate this state before rollback occurs
//     c. Is safer than falling back to workflow state, which may be stale or incomplete
//     d. A warning is logged when state cloning fails
//     e. The step is NOT failed due to cloning failure - this ensures rollback can still be
//     triggered if `executionMode` is `RollbackOnError` and the step fails for other reasons
//  4. When `executionMode` is `RollbackOnError` and a step fails, rollback is triggered immediately for
//     the failed step and all previously executed steps (from `index` down to 0). This ensures:
//     a. The failed step can clean up any partial work it completed before failing
//     b. All successfully executed steps are rolled back in reverse order
//     c. Each step's rollback receives its captured state snapshot (cloned or non-cloned reference)
//  5. Rollback reports are attached to the corresponding step reports via `stepReport.Rollback`.
//
// Execution semantics:
//  1. Execution behavior respects `w.executionMode`:
//     a. `StopOnError`: Stop immediately when a step fails, no rollback
//     b. `ContinueOnError`: Continue executing remaining steps even if one fails
//     c. `RollbackOnError`: Rollback the failed step and all previously executed steps, then stop
//  2. When rollback is performed, rollback reports from executed steps are attached to the corresponding
//     step reports and returned as part of the workflow `Report`.
//  3. The workflow completes successfully only if all steps succeed; otherwise returns a failure `Report`.
//  4. State cloning failures are logged as warnings but do not fail the step. This ensures that if
//     `executionMode` is `RollbackOnError` and the step fails for execution-related reasons, rollback
//     can still be triggered using the non-cloned state reference. Without this behavior, cloning
//     failures would prevent rollback and leave state inconsistent.
//
// State management and persistence:
//  1. The workflow exposes a shared `NamespacedStateBag` via `w.State()` that represents workflow-wide state.
//  2. Steps can access and mutate state during execution via `step.State()`.
//  3. For persistence needs (e.g., saving state to disk, database), users should handle this in their
//     step implementations or workflow callbacks (onCompletion, onFailure):
//     a. Write state to files/databases within step.Execute()
//     b. Use onCompletion callback to persist final workflow state
//     c. Attach custom metadata to reports via Meta field for tracking
//  4. After execution (successful or failed), state snapshots are preserved internally via
//     `w.lastExecutionStates` (keyed by step ID) for potential manual rollback via `Rollback()`.
//  5. State cloning failures result in non-cloned state references being stored; rollback will use these
//     references but they may have been mutated by later steps (only if execution continued past the
//     point of cloning failure).
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
	stepStates := make(map[string]NamespacedStateBag, len(w.steps))

	for index, step := range w.steps {
		var report *Report
		stepStart := time.Now()

		stepCtx := ctx
		var stepState NamespacedStateBag
		var statePrepError error
		var ctxPrepError error

		// prepare step state with namespace support
		if IsWorkflow(step) {
			var clonedGlobal StateBag
			// Sub-workflows get a new NamespacedStateBag with:
			// - Empty local namespace (isolated to the sub-workflow)
			// - Cloned global namespace (inherits parent's shared state)
			clonedGlobal, statePrepError = w.State().Global().Clone()
			if statePrepError != nil {
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(StepExecutionError.
						Wrap(statePrepError, "workflow %q step %q failed to clone global state for sub-workflow execution", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					))

				// Fall back to empty state for consistency since we always assume there is a state attached to the step
				// when calling Prepare and Execute, even though sub-workflow won't have access to parent's global state
				// in this case.
				stepState = NewNamespacedStateBag(nil, nil)
			} else {
				stepState = NewNamespacedStateBag(nil, clonedGlobal)
			}
		} else {
			// Ordinary steps get namespaced state with:
			// - Empty local namespace (isolated to this step)
			// - Shared global namespace (points to workflow's global state)
			stepState = NewNamespacedStateBag(nil, w.State().Global())
		}

		// make sure step has its state before calling Prepare so Prepare can access it.
		// It also ensures during Execute the step has the correct state
		if stepState != nil {
			step = step.WithState(stepState)
		}

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

		// Capture state snapshot after step processing (successful or failed)
		if state := step.State(); state != nil {
			clonedState, err := state.Clone()
			if err != nil {
				// State cloning failed; log warning and store non-cloned reference
				// Do NOT fail the step - this would prevent rollback and leave state inconsistent
				w.logger.Warn().
					Err(err).
					Str("workflowId", w.id).
					Str("stepId", step.Id()).
					Msg("failed to clone state for rollback snapshot; using current state reference (may be mutated by later steps before rollback)")

				// Store non-cloned state for rollback
				stepStates[step.Id()] = state
			} else {
				stepStates[step.Id()] = clonedState
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
				// Perform rollback using recorded per-step states
				// Rollback from index (include the failed step for cleanup)
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

	// Preserve state snapshots for potential manual rollback later
	// (even if workflow failed, manual Rollback() can use these snapshots)
	w.lastExecutionStates = stepStates

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

	// Use preserved states from last execution, fall back to nil if none available
	rollbackReports := w.rollbackFrom(ctx, len(w.steps)-1, w.lastExecutionStates)

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
