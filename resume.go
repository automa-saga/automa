package automa

// Resume (Durability Story 3): the public recovery entry point. ResumeWorkflow
// rehydrates a journal onto a re-supplied workflow definition and continues from
// where a previous run left off — skipping work already committed, re-running
// the one ambiguous step, or finishing an interrupted rollback. The behavior is
// the normative durability spec §6; this is the Go reference implementation.

import (
	"context"
	"errors"
	"os"
	"time"
)

// ResumeWorkflow recovers a run from its journal at journalPath and continues it
// (durability-spec §6). The caller MUST re-supply the same workflow definition
// (the same builder) that produced the original run; the topology and modes are
// validated against the journal before anything runs.
//
//   - Missing journal → a fresh run is started and journaled at journalPath
//     (resuming a never-started run is a normal start, §6.2).
//   - Corrupt or unsupported-version journal → fails loudly and runs nothing;
//     silently restarting could re-execute side effects (§6.2).
//   - Topology/mode mismatch → refused (§6.2).
//   - PhaseForward → completed steps are skipped, a started-but-not-completed
//     step is re-executed once (idempotency contract, §7), execution continues.
//   - PhaseCompensating → rollback continues from the cursor, skipping steps
//     already compensated.
//   - PhaseDone → the recorded final result is returned; nothing runs (§6.5).
//
// Sub-workflows resume recursively (§3.8): a nested workflow that was in
// progress at the crash is resumed by dispatching on its own cursor phase
// (forward, compensating, or done), so its already-completed inner steps are not
// re-executed.
func ResumeWorkflow(ctx context.Context, wb *WorkflowBuilder, journalPath string) *Report {
	start := time.Now()

	// An empty path is a programming error: loadJournal("") reports os.ErrNotExist,
	// which would otherwise be taken as a fresh run and silently execute WITHOUT a
	// journal (journalPath == "" disables journaling in Execute). Fail fast instead.
	if journalPath == "" {
		return NewReport(wb.Id(),
			WithWorkflow(wb.workflow),
			WithStatus(StatusFailed),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(IllegalArgument.New("ResumeWorkflow requires a non-empty journalPath")))
	}

	built, err := wb.Build()
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
	wf, ok := built.(*workflow)
	if !ok {
		return NewReport(wb.Id(),
			WithStatus(StatusFailed),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(StepExecutionError.New("workflow %q did not build to a *workflow", wb.Id())))
	}

	j, err := loadJournal(journalPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Missing journal → fresh run, journaled at this path (§6.2). Execute
			// owns Prepare (see RunWorkflow), so start it directly.
			wf.journalPath = journalPath
			return wf.Execute(ctx)
		}
		// Corrupt or unsupported version → fail loudly, never restart (§6.2).
		return FailureReport(wf,
			WithWorkflow(wf),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(err))
	}

	// Topology/mode agreement gate (§6.2).
	if err := validateTopology(wf, j); err != nil {
		return FailureReport(wf,
			WithWorkflow(wf),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(err))
	}

	// Install the root persist closure and rehydrate the whole tree onto the
	// definition (§6.1). Every node shares the one closure, so continued
	// execution keeps rewriting the single journal file atomically.
	root := wf
	persist := func() error {
		j, err := root.snapshotJournal()
		if err != nil {
			return err
		}
		return j.persist(journalPath)
	}
	wf.journalPath = journalPath
	if err := rehydrateInto(wf, j.Cursor, j.Shared, j.Steps, persist); err != nil {
		return FailureReport(wf,
			WithWorkflow(wf),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(err))
	}

	switch j.Cursor.Phase {
	case PhaseDone:
		// Terminal: return the recorded result, run nothing (§6.5).
		return wf.reconstructFinalReport(start)
	case PhaseCompensating:
		return wf.resumeCompensation(ctx, start)
	case PhaseForward:
		// Execute owns Prepare (see RunWorkflow) and is resume-aware via the
		// progress rehydrated above.
		return wf.Execute(ctx)
	default:
		return FailureReport(wf,
			WithWorkflow(wf),
			WithActionType(ActionPrepare),
			WithStartTime(start),
			WithError(JournalCorrupt.New("journal cursor has unknown phase %q", j.Cursor.Phase)))
	}
}

// rehydrateInto loads a journal node's recorded progress onto workflow w and
// recurses into sub-workflow steps (durability-spec §6.1). It marks w resuming,
// installs the shared persist closure, restores the shared (Global) state and
// the per-step cursor/state/snapshot/report, and rebuilds the executed and
// compensated sets so a continued forward run or rollback behaves correctly.
func rehydrateInto(w *workflow, cursor Cursor, shared *SyncNamespacedStateBag, steps []*StepJournal, persist func() error) error {
	w.resuming = true
	w.journalPersist = persist
	w.jPhase = cursor.Phase
	w.jIndex = cursor.Index

	// Restore this node's shared (Global) state onto its live bag (§6.1.3).
	if shared == nil {
		return JournalCorrupt.New("workflow %q is missing shared state", w.id)
	}
	if g, err := shared.Global().Clone(); err != nil {
		return JournalCorrupt.Wrap(err, "workflow %q failed to restore shared state", w.id)
	} else {
		w.state = NewNamespacedStateBag(nil, g)
	}

	n := len(w.steps)
	w.jStepStates = make([]StepState, n)
	w.jStepSnapshots = make([]*SyncNamespacedStateBag, n)
	w.jStepReports = make([]*Report, n)
	if w.compensatedStepIDs == nil {
		w.compensatedStepIDs = make(map[string]struct{})
	}
	w.lastExecutedStepIDs = make(map[string]struct{})
	w.lastExecutionStates = make(map[string]NamespacedStateBag)

	for i := 0; i < n && i < len(steps); i++ {
		e := steps[i]
		w.jStepStates[i] = e.State
		if e.Snapshot != nil {
			w.jStepSnapshots[i] = e.Snapshot
			w.lastExecutionStates[e.ID] = e.Snapshot
		}
		if e.Report != nil {
			w.jStepReports[i] = e.Report
		}
		switch e.State {
		case StepCompleted, StepFailed, StepCompensated:
			// These states all imply the step reached Execute, so it is eligible
			// for compensation (D5 / §5.3).
			w.lastExecutedStepIDs[e.ID] = struct{}{}
		}
		if e.State == StepCompensated {
			w.compensatedStepIDs[e.ID] = struct{}{}
		}

		// Recurse into a sub-workflow step so nested compensation resumes with the
		// child's own state, executed set, and already-compensated set.
		if child, ok := w.steps[i].(*workflow); ok && e.IsWorkflow() {
			childCursor := Cursor{Phase: PhaseForward}
			if e.Cursor != nil {
				childCursor = *e.Cursor
			}
			if err := rehydrateInto(child, childCursor, e.Shared, e.Steps, persist); err != nil {
				return err
			}
		}
	}
	return nil
}

// resumeChildStep resumes an in-progress sub-workflow by dispatching on its own
// cursor phase (durability-spec §3.8, §6.4). The child was already rehydrated by
// rehydrateInto, so this continues it from where it crashed: forward execution,
// an interrupted compensation, or (defensively) a terminal node.
func (w *workflow) resumeChildStep(ctx context.Context, child *workflow) *Report {
	switch child.jPhase {
	case PhaseCompensating:
		// The child was rolling itself back when the crash hit; finish it.
		return child.resumeCompensation(ctx, time.Now())
	case PhaseDone:
		// Already terminal (an unusual crash window); return its recorded result.
		return child.reconstructFinalReport(time.Now())
	case PhaseForward:
		return child.Execute(ctx)
	default:
		return FailureReport(child,
			WithWorkflow(w),
			WithActionType(ActionExecute),
			WithStartTime(time.Now()),
			WithError(JournalCorrupt.New("journal cursor has unknown phase %q", child.jPhase)))
	}
}

// resumeCompensation continues an interrupted rollback from the recorded cursor
// (durability-spec §6.4). Already-compensated steps are skipped (their IDs are
// in compensatedStepIDs, rehydrated above); rollbackFrom journals each remaining
// compensation. The run is then marked terminal.
func (w *workflow) resumeCompensation(ctx context.Context, start time.Time) *Report {
	rollbackReports := w.rollbackFrom(ctx, w.jIndex, w.lastExecutionStates, w.lastExecutedStepIDs)

	stepReports := make([]*Report, 0, len(w.steps))
	for i, s := range w.steps {
		rep := w.jStepReports[i]
		if rep == nil {
			rep = FailureReport(s, WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(start))
		}
		if rb, ok := rollbackReports[s.Id()]; ok {
			rep.Rollback = rb
		}
		stepReports = append(stepReports, rep)
	}

	w.journalDone()

	report := FailureReport(w,
		WithWorkflow(w),
		WithActionType(ActionExecute),
		WithStartTime(start),
		WithStepReports(stepReports...),
		WithError(StepExecutionError.New("workflow %q resumed and completed compensation", w.id)))

	// Fire onFailure, mirroring Execute's failure path. A `compensating` journal
	// means the original run crashed mid-rollback, BEFORE it reached journalDone
	// and its own handleFailure — so this is the first and only firing for the
	// run (no double-fire). Terminal (`done`) resumes go through
	// reconstructFinalReport instead, which deliberately does not re-fire.
	w.handleFailure(ctx, report)

	return report
}

// reconstructFinalReport rebuilds a terminal run's report from the journal
// (durability-spec §6.5). The journal records per-step outcomes but not a single
// top-level report, so the workflow status is derived: success iff every step is
// completed; otherwise failed (a fully-compensated run is a failure that rolled
// back cleanly).
//
// This is a pure replay of an already-terminal (`done`) journal and MUST run
// nothing (§3.7.2). In particular it does NOT fire onCompletion/onFailure. Those
// callbacks are fired exactly once by the live-completing paths — Execute, and
// resumeCompensation for an interrupted rollback — so re-firing them on a terminal
// replay would duplicate the callback and any side effect it performs. (Execute
// writes `done` and then fires its callback, so a crash in that narrow window can
// drop the callback; the replay still deliberately does not re-fire, preferring a
// possibly-missed callback over ever double-firing.)
func (w *workflow) reconstructFinalReport(start time.Time) *Report {
	stepReports := make([]*Report, 0, len(w.steps))
	anyNotCompleted := false
	for i, s := range w.steps {
		state := w.stepStateAt(i)
		if state != StepCompleted {
			anyNotCompleted = true
		}
		rep := w.jStepReports[i]
		if rep == nil {
			switch state {
			case StepCompleted:
				rep = SuccessReport(s, WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(start))
			case StepFailed:
				rep = FailureReport(s, WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(start))
			default:
				rep = SkippedReport(s, WithWorkflow(w), WithActionType(ActionExecute), WithStartTime(start))
			}
		}
		stepReports = append(stepReports, rep)
	}

	if anyNotCompleted {
		return FailureReport(w,
			WithWorkflow(w),
			WithActionType(ActionExecute),
			WithStartTime(start),
			WithStepReports(stepReports...),
			WithError(StepExecutionError.New("workflow %q resumed from a terminal (non-success) journal", w.id)))
	}
	return SuccessReport(w,
		WithWorkflow(w),
		WithActionType(ActionExecute),
		WithStartTime(start),
		WithStepReports(stepReports...))
}
