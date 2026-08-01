package automa

import (
	"context"
	"log/slog"
	"time"
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
	logger               *slog.Logger
	steps                []Step
	executionMode        TypeMode
	rollbackMode         TypeMode
	enableAsyncCallbacks bool

	// preserve step states after execution for potential rollback; keyed by step ID
	lastExecutionStates map[string]NamespacedStateBag

	// lastExecutedStepIDs tracks which step IDs actually reached Execute (i.e.,
	// both statePrepError and ctxPrepError were nil). Steps that failed in Prepare
	// are excluded so rollbackFrom can skip compensating them (spec D5 / §5.3).
	lastExecutedStepIDs map[string]struct{}

	// compensatedStepIDs tracks step IDs already compensated during this run so a
	// step's Rollback is invoked at most once (spec §5.3.5 / D10). A sub-workflow
	// that self-compensates on internal failure (during its own Execute) records
	// its steps here; when the parent later drives this sub-workflow's Rollback,
	// rollbackFrom finds them already compensated and skips them instead of
	// double-compensating. The set is per instance (workflows are single-use), so
	// a manual Rollback after an automatic one is likewise a no-op for steps the
	// automatic pass already handled.
	//
	// Like lastExecutionStates and lastExecutedStepIDs, this is keyed by step ID
	// and so relies on IDs being unique within the workflow and non-empty (D9);
	// duplicate/empty IDs are rejected at build time (workflow_builder.go).
	compensatedStepIDs map[string]struct{}

	// preserveStatesForRollback controls whether state snapshots are cloned and preserved
	// after each step execution. When true (default), enables deterministic rollback but
	// increases memory usage. When false, skips state cloning to reduce overhead.
	//
	// When true (default):
	//   - State is cloned and stored for each step after execution
	//   - Rollback steps receive their execution-time state snapshots (local + global namespaces)
	//   - Higher memory usage due to deep state cloning
	//
	// When false:
	//   - State is NOT cloned or stored for rollback
	//   - Rollback steps receive workflow.State() (global state only)
	//   - Per-step local namespaces are NOT available during rollback
	//   - Lower memory usage (no cloning or storage overhead)
	//
	// Use preservation=false when:
	//
	//   - Rollback steps are idempotent (don't need per-step state)
	//   - Rollback only needs global state
	//   - Rollback uses external state (database, files, APIs)
	//   - executionMode is StopOnError or ContinueOnError (no rollback triggered)
	//
	// Keep preservation=true (default) when:
	//
	//   - Rollback needs per-step local namespaces
	//   - Rollback needs exact state snapshot from execution time
	//   - Multiple steps might mutate shared state before rollback
	preserveStatesForRollback bool

	// prepared is set to true after Prepare has been called, preventing double-invocation
	// when Execute is called via RunWorkflow (which calls Prepare explicitly before Execute).
	prepared bool

	// --- durability journaling (opt-in via WithJournal; Durability Story 2) ---
	//
	// journalPath is the on-disk journal file path. It is set only on the root
	// workflow (via WithJournal); an empty path disables journaling. Sub-workflows
	// never carry a path — they persist through the root's closure (below).
	journalPath string

	// journalPersist writes the entire root journal atomically (durability-spec
	// §3.6). The root installs this closure at the start of Execute; every
	// sub-workflow inherits the same closure, so any node's transition rewrites
	// the single shared file. A nil closure means journaling is off for this
	// instance, and every journal* transition is a no-op.
	journalPersist func() error

	// jPhase/jIndex are this node's cursor (durability-spec §3.4); the per-step
	// slices are aligned with steps by index and record each step's journal state,
	// its rollback snapshot (leaf steps only), and its report. They are populated
	// as the node runs and read by snapshotJournal to project the on-disk tree.
	jPhase         Phase
	jIndex         int
	jStepStates    []StepState
	jStepSnapshots []*SyncNamespacedStateBag
	jStepReports   []*Report

	// resuming is set by rehydrate (via ResumeWorkflow) so Execute continues an
	// existing journal instead of starting a fresh one: it keeps the rehydrated
	// progress rather than resetting it, and skips steps already recorded
	// completed rather than re-executing them (durability-spec §6.3).
	resuming bool

	// callbacks and hooks
	prepare      PrepareFunc
	rollback     RollbackFunc // optional user-defined rollback function for the entire workflow
	onCompletion OnCompletionFunc
	onFailure    OnFailureFunc
}

// WithState attaches the provided [NamespacedStateBag] to the workflow in place
// and returns the same instance (not a copy), so a workflow can be nested as a
// step. See [Step.WithState] for the attach-and-observe, single-use contract.
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

	// Execute owns preparation: it invokes Prepare, returns an ActionPrepare
	// failure report (carrying this workflow's actual modes) on error, and resets
	// the prepared flag for reuse. Calling Prepare here as well would be redundant
	// — and, because Build has already reset wb.workflow to a fresh default, would
	// attach the wrong (default) modes to the error report.
	return wf.Execute(ctx)
}

// rollbackFrom rollbacks the workflow backward from the given index to the start.
// executedIDs restricts compensation to steps that actually ran Execute; nil is treated
// as empty (nothing executed). Steps absent from executedIDs are skipped (spec D5 / §5.3):
// compensating a step whose Execute never ran is incorrect because its rollback may assume
// side-effects that were never produced.
func (w *workflow) rollbackFrom(ctx context.Context, index int, states map[string]NamespacedStateBag, executedIDs map[string]struct{}) map[string]*Report {
	stepReports := map[string]*Report{}
	startTime := time.Now()

	// Guard against instances built outside newDefaultWorkflow (writing to a nil
	// map panics); the compensate-once set (D10) must always be usable.
	if w.compensatedStepIDs == nil {
		w.compensatedStepIDs = make(map[string]struct{})
	}

	// Keep the in-memory phase consistent so the projection reads as compensating.
	// The DURABLE flip (write-ahead, §5 F5) is owned by the caller — Execute's
	// failure path or Rollback — which persists it before any compensation side
	// effect; this method does not persist on entry.
	if w.journaling() {
		w.jPhase = PhaseCompensating
	}

	for i := index; i >= 0; i-- {
		step := w.steps[i]

		// Skip compensation for steps that never executed (spec D5 / §5.3).
		// nil is treated as empty: no execution context means nothing to compensate.
		if _, ran := executedIDs[step.Id()]; !ran {
			w.log().Debug("skipping rollback for non-executed step", "workflowId", w.id, "stepId", step.Id(), "index", i)
			stepReports[step.Id()] = SkippedReport(step,
				WithWorkflow(w),
				WithActionType(ActionRollback),
				WithStartTime(startTime),
			)
			continue
		}

		// Compensate at most once (spec §5.3.5 / D10). A step already compensated
		// in this run (e.g. a sub-workflow that self-compensated during its own
		// Execute, then had its Rollback driven again by the parent) MUST NOT have
		// its Rollback re-invoked. Record it as skipped and move on.
		if _, done := w.compensatedStepIDs[step.Id()]; done {
			w.log().Debug("skipping already-compensated step", "workflowId", w.id, "stepId", step.Id(), "index", i)
			stepReports[step.Id()] = SkippedReport(step,
				WithWorkflow(w),
				WithActionType(ActionRollback),
				WithStartTime(startTime),
			)
			continue
		}

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

		w.log().Debug("rolling back step", "workflowId", w.id, "stepId", step.Id(), "index", i)

		rollbackReport := step.Rollback(ctx)

		if rollbackReport == nil {
			w.log().Warn("step returned nil report from Rollback; treating as failure",
				"workflowId", w.id,
				"stepId", step.Id(),
			)
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

		// Mark compensated so a subsequent pass over the same instance (e.g. the
		// parent driving this sub-workflow's Rollback after self-compensation, or
		// a manual Rollback after an automatic one) does not compensate it again
		// (spec §5.3.5 / D10).
		w.compensatedStepIDs[step.Id()] = struct{}{}

		// C3 (durability-spec §5): record this step compensated with the cursor at
		// i, persisted before compensation proceeds to a lower index so an
		// interrupted rollback does not repeat it.
		w.journalStepCompensated(i)

		if rollbackReport.IsFailed() {
			w.log().Warn("step rollback failed",
				"workflowId", w.id,
				"stepId", step.Id(),
				"index", i,
				"rollbackMode", w.rollbackMode.String(),
			)
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
	if w.prepared {
		return ctx, nil
	}

	preparedCtx := ctx
	if w.prepare != nil {
		c, err := w.prepare(preparedCtx, w)
		if err != nil {
			return nil, err
		}
		preparedCtx = c // use the context returned by user prepare function
	}

	w.prepared = true
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
//     (keyed by step ID) for potential rollback (if `preserveStatesForRollback` is enabled).
//  2. State cloning ensures immutable snapshots (global state is shared across steps, so cloning
//     prevents later mutations from affecting earlier snapshots).
//  3. State preservation can be disabled via `WithStatePreservation(false)` on the WorkflowBuilder to
//     reduce memory overhead when rollback capability isn't needed. When disabled, all state snapshot
//     operations are skipped, and manual `Rollback()` calls will use current workflow state.
//  4. If state cloning fails (when preservation is enabled), the step's current (non-cloned) state
//     reference is stored instead. This:
//     a. Ensures rollback can access the step's actual execution state (including partial success)
//     b. Accepts the risk that later steps may mutate this state before rollback occurs
//     c. Is safer than falling back to workflow state, which may be stale or incomplete
//     d. A warning is logged when state cloning fails
//     e. The step is NOT failed due to cloning failure - this ensures rollback can still be
//     triggered if `executionMode` is `RollbackOnError` and the step fails for other reasons
//  5. When `executionMode` is `RollbackOnError` and a step fails, rollback is triggered immediately for
//     the failed step and all previously executed steps (from `index` down to 0). This ensures:
//     a. The failed step can clean up any partial work it completed before failing
//     b. All successfully executed steps are rolled back in reverse order
//     c. Each step's rollback receives its captured state snapshot (cloned or non-cloned reference)
//  6. Rollback reports are attached to the corresponding step reports via `stepReport.Rollback`.
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
//     This is controlled by `preserveStatesForRollback` (default: true).
//  5. To reduce memory overhead, state preservation can be disabled via `WithStatePreservation(false)`
//     on the WorkflowBuilder. When disabled, no state snapshots are cloned or stored, and manual
//     `Rollback()` calls will use the current workflow state instead of per-step snapshots.
//  6. State cloning failures (when preservation is enabled) result in non-cloned state references being
//     stored; rollback will use these references but they may have been mutated by later steps (only
//     if execution continued past the point of cloning failure).
//
// Execute runs all steps in sequence according to the configured execution mode.
//
// State Preservation:
// - When preserveStatesForRollback=true (default), step states are cloned after execution
// - When preserveStatesForRollback=false, states are NOT preserved; rollback receives workflow.State()
// - See WithStatePreservation() for detailed tradeoffs
func (w *workflow) Execute(ctx context.Context) *Report {
	startTime := time.Now()

	// Reset prepared after Execute completes so that subsequent Execute calls re-run Prepare.
	defer func() { w.prepared = false }()

	preparedCtx, err := w.Prepare(ctx)
	if err != nil {
		return FailureReport(w,
			WithWorkflow(w),
			WithActionType(ActionPrepare),
			WithStartTime(startTime),
			WithError(StepExecutionError.
				Wrap(err, "workflow %q preparation failed: %v", w.id, err).
				WithProperty(StepIdProperty, w.id),
			))
	}
	ctx = preparedCtx

	if len(w.steps) == 0 {
		w.log().Error("workflow has no steps to execute", "workflowId", w.id)
		return FailureReport(w,
			WithWorkflow(w),
			WithStartTime(startTime),
			WithActionType(ActionExecute),
			WithError(StepExecutionError.New("workflow %s has no steps to execute", w.id)))
	}

	w.log().Debug("starting workflow execution",
		"workflowId", w.id,
		"steps", len(w.steps),
		"executionMode", w.executionMode.String(),
		"rollbackMode", w.rollbackMode.String(),
	)

	// Install journaling for this run (durability-spec §5). The root workflow —
	// the one given a path via WithJournal — installs the persist closure that
	// rewrites the whole journal atomically; sub-workflows inherit it (attached at
	// their write-ahead point below), so any node's transition rewrites the single
	// shared file.
	if w.journalPath != "" && w.journalPersist == nil {
		root := w
		w.journalPersist = func() error {
			j, err := root.snapshotJournal()
			if err != nil {
				return err
			}
			return j.persist(root.journalPath)
		}
	}
	if w.journaling() && !w.resuming {
		w.initJournalProgress()
		// Write the initial forward/0 journal from the root so a crash before the
		// first step still leaves a complete, valid journal on disk.
		if w.journalPath != "" {
			w.persistJournal()
		}
	}

	var stepReports []*Report
	var hasFailed bool

	// capture per-step state snapshots for rollback
	stepStates := make(map[string]NamespacedStateBag, len(w.steps))

	// track which steps actually reached Execute (spec D5 / §5.3)
	executedStepIDs := make(map[string]struct{}, len(w.steps))

	for index, step := range w.steps {
		var report *Report
		stepStart := time.Now()

		// Resume: a step recorded `completed` before a crash MUST NOT be
		// re-executed (durability-spec §6.3.4). Reuse its recorded report and
		// rollback snapshot, count it as executed, and carry on. A `started` leaf
		// (the ambiguous case) is not skipped here — it falls through and re-runs,
		// which is why steps must be idempotent (§7).
		if w.resuming && index < len(w.jStepStates) && w.jStepStates[index] == StepCompleted {
			rep := w.jStepReports[index]
			if rep == nil {
				rep = SuccessReport(step, WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(stepStart))
			}
			stepReports = append(stepReports, rep)
			executedStepIDs[step.Id()] = struct{}{}
			if snap := w.jStepSnapshots[index]; snap != nil {
				stepStates[step.Id()] = snap
			}
			w.log().Debug("resume: skipping already-completed step", "workflowId", w.id, "stepId", step.Id(), "index", index)
			continue
		}

		// Resume descent (durability-spec §3.8/§6.4): a sub-workflow recorded
		// in progress (`started`) is resumed by dispatching on its OWN cursor
		// phase — never re-run from scratch, which would re-execute its completed
		// inner steps. It keeps the state rehydrated onto it (re-cloning the
		// parent's global here would discard the divergence its inner steps
		// produced), so this bypasses the normal per-step state prep below. A
		// `started` leaf is not handled here — it falls through and re-runs.
		if w.resuming && w.stepStateAt(index) == StepStarted {
			if child, isWorkflowStep := step.(*workflow); isWorkflowStep {
				// Re-affirm started; brackets the descent (write-ahead, §5 F1). If it
				// cannot be persisted we do not descend — failing loudly beats running
				// the child with no durable record.
				if err := w.journalStepStarted(index); err != nil {
					report = FailureReport(child,
						WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(stepStart),
						WithError(JournalError.
							Wrap(err, "workflow %q sub-workflow %q: failed to persist write-ahead journal on resume; not descending", w.id, child.Id()).
							WithProperty(StepIdProperty, child.Id())))
					stepReports = append(stepReports, report)
					hasFailed = true
					break
				}
				report = w.resumeChildStep(ctx, child)
				if report == nil {
					report = FailureReport(child,
						WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(stepStart),
						WithError(StepExecutionError.New("workflow %q sub-workflow %q returned nil report on resume", w.id, child.Id())))
				}
				stepReports = append(stepReports, report)
				executedStepIDs[step.Id()] = struct{}{}

				// F4 commit for the sub-workflow step (no leaf snapshot for a workflow).
				committed := StepCompleted
				if report.IsFailed() {
					committed = StepFailed
				}
				w.journalStepCommitted(index, committed, nil, report)

				if report.IsFailed() {
					hasFailed = true
					w.log().Warn("sub-workflow failed on resume",
						"workflowId", w.id, "stepId", step.Id(), "index", index,
						"executionMode", w.executionMode.String())
					if w.executionMode == StopOnError {
						break
					} else if w.executionMode == RollbackOnError {
						if err := w.journalEnterCompensating(index); err != nil {
							w.log().Error("failed to persist compensating-phase journal on resume; skipping rollback to keep resume safe",
								"workflowId", w.id, "index", index, "error", err)
							break
						}
						rollbackReports := w.rollbackFrom(ctx, index, stepStates, executedStepIDs)
						for _, sr := range stepReports {
							if rollback, ok := rollbackReports[sr.Id]; ok {
								sr.Rollback = rollback
							}
						}
						break
					}
				}
				continue
			}
		}

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

				w.log().Warn("failed to clone global state for sub-workflow; falling back to empty state",
					"workflowId", w.id,
					"stepId", step.Id(),
					"error", statePrepError,
				)

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
		// It also ensures during Execute the step has the correct state. For a
		// sub-workflow this MUST happen before the F1 write-ahead below so the
		// persisted `started` snapshot records the child's real shared (cloned
		// Global) state, not an empty one (durability-spec §3.8); WithState mutates
		// the stored node, so the projection sees it.
		step = step.WithState(stepState)

		// F1 write-ahead (durability-spec §5): record `started` and the forward
		// cursor and persist BEFORE any side effect runs. A sub-workflow step
		// inherits the root persist closure here so its own transitions journal
		// inline under this entry (§3.8). The record MUST be durable before the
		// side effect: if the persist fails we fail the step without executing it,
		// so a crash can never strand an effect that resume cannot compensate.
		var journalStartErr error
		if statePrepError == nil && w.journaling() {
			if child, ok := step.(*workflow); ok {
				child.journalPersist = w.journalPersist
			}
			journalStartErr = w.journalStepStarted(index)
			if journalStartErr != nil {
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionExecute),
					WithError(JournalError.
						Wrap(journalStartErr, "workflow %q step %q: failed to persist write-ahead journal; step not executed", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					))
			}
		}

		// prepare step context
		if statePrepError == nil && journalStartErr == nil {
			w.log().Debug("executing step", "workflowId", w.id, "stepId", step.Id(), "index", index)
			stepCtx, ctxPrepError = step.Prepare(ctx)
			if ctxPrepError != nil {
				w.log().Warn("step preparation failed",
					"workflowId", w.id,
					"stepId", step.Id(),
					"error", ctxPrepError,
				)
				report = FailureReport(step,
					WithWorkflow(w),
					WithStartTime(stepStart),
					WithActionType(ActionPrepare),
					WithError(StepExecutionError.
						Wrap(ctxPrepError, "workflow %q step %q preparation failed", w.id, step.Id()).
						WithProperty(StepIdProperty, step.Id()),
					))
			}
		}

		// execute step if preparation succeeded
		if statePrepError == nil && ctxPrepError == nil && journalStartErr == nil {
			executedStepIDs[step.Id()] = struct{}{}
			report = step.Execute(stepCtx)
			if report == nil {
				w.log().Warn("step returned nil report from Execute; treating as failure",
					"workflowId", w.id,
					"stepId", step.Id(),
				)
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
		// Clone() creates an immutable snapshot by deep-cloning all namespaces (local, global).
		// This ensures later steps cannot mutate earlier snapshots, enabling deterministic rollback.
		// State preservation can be disabled via preserveStatesForRollback to reduce memory overhead.
		if w.preserveStatesForRollback {
			if state := step.State(); state != nil {
				clonedState, err := state.Clone()
				if err != nil {
					// State cloning failed; log warning and store non-cloned reference
					// Do NOT fail the step - this would prevent rollback and leave state inconsistent
					w.log().Warn(
						"failed to clone state for rollback snapshot; using current state reference (may be mutated by later steps before rollback)",
						"error", err,
						"workflowId", w.id,
						"stepId", step.Id(),
					)

					// Store non-cloned state for rollback
					stepStates[step.Id()] = state
				} else {
					stepStates[step.Id()] = clonedState
				}
			}
		}

		// collect step report
		stepReports = append(stepReports, report)

		// F4 commit (durability-spec §5): record the step's outcome, its rollback
		// snapshot (leaf steps only), and its report, persisted AFTER the side
		// effect returns and before the next step's write-ahead. Skipped when the
		// write-ahead itself failed (journalStartErr): the step never ran and the
		// journal write is already broken, so there is nothing to commit.
		if w.journaling() && journalStartErr == nil {
			// reachedExecute mirrors the executedStepIDs guard above: the step ran
			// its side effect iff state prep, the write-ahead persist, and Prepare
			// all succeeded.
			reachedExecute := statePrepError == nil && ctxPrepError == nil && journalStartErr == nil

			stepState := StepCompleted
			if report.IsFailed() {
				if reachedExecute {
					stepState = StepFailed
				} else {
					// Failed before reaching Execute (state prep or the Prepare hook):
					// no side effect ran, so per the spec `failed` — which means
					// "Execute failed" and implies compensation on resume (§4.2) —
					// MUST NOT be recorded. Preserve the pre-execute state (`started`
					// if the write-ahead ran, else `pending`) so rehydrateInto does not
					// add this step to lastExecutedStepIDs and resume does not
					// compensate a step that never executed. This matches the live D5
					// skip, which excludes prepare-failed steps from executedStepIDs.
					stepState = w.stepStateAt(index)
				}
			}
			var snapshot *SyncNamespacedStateBag
			if _, isWorkflowStep := step.(*workflow); !isWorkflowStep {
				if s, ok := stepStates[step.Id()].(*SyncNamespacedStateBag); ok {
					snapshot = s
				}
			}
			w.journalStepCommitted(index, stepState, snapshot, report)
		}

		// check for step failure
		if report.IsFailed() {
			hasFailed = true

			w.log().Warn("step failed",
				"workflowId", w.id,
				"stepId", step.Id(),
				"index", index,
				"executionMode", w.executionMode.String(),
			)

			if w.executionMode == StopOnError {
				break
			} else if w.executionMode == RollbackOnError {
				w.log().Info("initiating rollback after step failure",
					"workflowId", w.id,
					"fromStepId", step.Id(),
					"fromIndex", index,
					"rollbackMode", w.rollbackMode.String(),
				)

				// F5 (durability-spec §5): flip the phase to compensating with the
				// cursor at the failed step, persisted before compensation begins.
				// This is a write-ahead point: if it cannot be made durable we do
				// NOT compensate, because a crash mid-rollback would leave a journal
				// that still reads as forward and resume would re-execute instead of
				// continuing. Resume retries the whole compensation once it is durable.
				if err := w.journalEnterCompensating(index); err != nil {
					w.log().Error("failed to persist compensating-phase journal; skipping rollback to keep resume safe",
						"workflowId", w.id, "index", index, "error", err)
					break
				}

				// Perform rollback using recorded per-step states
				// Rollback from index (include the failed step for cleanup)
				rollbackReports := w.rollbackFrom(stepCtx, index, stepStates, executedStepIDs)

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

	// Preserve state snapshots and executed-step set for potential manual rollback later.
	// Always clear lastExecutionStates first so stale snapshots from a prior execution
	// (possibly run with preservation enabled) are never used when preservation is off.
	if w.preserveStatesForRollback {
		w.lastExecutionStates = stepStates
	} else {
		w.lastExecutionStates = nil
	}
	w.lastExecutedStepIDs = executedStepIDs

	if hasFailed {
		var failedStepIDs []string
		for _, sr := range stepReports {
			if sr.IsFailed() {
				failedStepIDs = append(failedStepIDs, sr.Id)
			}
		}

		w.log().Error("workflow execution failed",
			"workflowId", w.id,
			"failedSteps", failedStepIDs,
			"durationMs", time.Since(startTime).Milliseconds(),
		)

		workflowReport := FailureReport(w,
			WithWorkflow(w),
			WithStartTime(startTime),
			WithActionType(ActionExecute),
			WithError(StepExecutionError.New(
				"workflow %q completed with %d step failures: %v",
				w.id, len(failedStepIDs), failedStepIDs,
			)),
			WithStepReports(stepReports...))

		// D1 (durability-spec §5): the run is terminal.
		w.journalDone()

		w.handleFailure(ctx, workflowReport)

		return workflowReport
	}

	w.log().Info("workflow execution completed",
		"workflowId", w.id,
		"steps", len(stepReports),
		"durationMs", time.Since(startTime).Milliseconds(),
	)

	workflowReport := SuccessReport(w,
		WithWorkflow(w),
		WithStartTime(startTime),
		WithActionType(ActionExecute),
		WithStepReports(stepReports...))

	// D1 (durability-spec §5): the run is terminal.
	w.journalDone()

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

	w.log().Info("starting workflow rollback", "workflowId", w.id, "steps", len(w.steps))

	// A directly-invoked Rollback of a durable workflow must persist the
	// compensating-phase flip before any compensation side effect (durability-spec
	// §5 F5), exactly as Execute's failure path does. Bail out loudly if it cannot
	// be made durable rather than compensate against a journal that still reads
	// forward.
	if w.journaling() && len(w.steps) > 0 {
		if err := w.journalEnterCompensating(len(w.steps) - 1); err != nil {
			return FailureReport(w,
				WithWorkflow(w),
				WithActionType(ActionRollback),
				WithStartTime(startTime),
				WithError(JournalError.
					Wrap(err, "workflow %q: failed to persist compensating-phase journal; rollback not started", w.id)))
		}
	}

	// Use preserved states and executed-step set from last execution.
	rollbackReports := w.rollbackFrom(ctx, len(w.steps)-1, w.lastExecutionStates, w.lastExecutedStepIDs)

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

	// Mark the run terminal (§5 D1) so a later ResumeWorkflow recognises the
	// rollback as finished (`done`) instead of finding the journal stuck in
	// `compensating` and re-entering compensation.
	if w.journaling() {
		w.journalDone()
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
		executionMode:             StopOnError,
		rollbackMode:              ContinueOnError,
		logger:                    discardLogger,
		preserveStatesForRollback: true, // default: enabled for backward compatibility
		// Empty (not nil) so Rollback() called before Execute() compensates nothing.
		// nil would bypass the D5 filter; a non-nil empty map correctly skips all steps.
		lastExecutedStepIDs: make(map[string]struct{}),
		// Tracks steps compensated this run so each is rolled back at most once (D10).
		compensatedStepIDs: make(map[string]struct{}),
	}
}

// discardLogger is the shared no-op logger used when no logger has been
// configured. slog.DiscardHandler drops every record, so logging stays silent
// and allocation-free until the caller supplies a real logger via WithLogger.
var discardLogger = slog.New(slog.DiscardHandler)

// log returns the workflow's logger, falling back to discardLogger when none was
// configured (e.g. for hand-constructed workflows that bypass newDefaultWorkflow).
// Internal log call sites use this so they never have to nil-check w.logger.
func (w *workflow) log() *slog.Logger {
	if w.logger == nil {
		return discardLogger
	}
	return w.logger
}
