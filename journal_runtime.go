package automa

// Runtime journaling for a running workflow (Durability Story 2). These helpers
// project a workflow's live progress into the on-disk journal (journal.go) and
// persist it atomically at the ordering points defined by durability-spec §5.
//
// The journal is a *projection*: each workflow instance owns its own progress
// fields (jPhase/jIndex and the per-step slices); snapshotJournal derives the
// full recursive tree from them on demand. The only cross-node coupling is the
// persist closure — the root installs it and every sub-workflow inherits it, so
// any node's transition rewrites the single shared file atomically (§3.6, §3.8).
//
// All of this is inert unless the workflow was made durable with WithJournal:
// journaling reports false and every transition returns immediately.

// journaling reports whether this workflow instance writes a journal.
func (w *workflow) journaling() bool { return w.journalPersist != nil }

// initJournalProgress prepares this node's progress fields for a run: forward
// phase at index 0, with per-step slices sized to the topology (all steps
// pending). It is called at the start of Execute for every journaling node.
func (w *workflow) initJournalProgress() {
	w.jPhase = PhaseForward
	w.jIndex = 0
	n := len(w.steps)
	w.jStepStates = make([]StepState, n)
	for i := range w.jStepStates {
		w.jStepStates[i] = StepPending
	}
	w.jStepSnapshots = make([]*SyncNamespacedStateBag, n)
	w.jStepReports = make([]*Report, n)
}

// persistJournal writes the whole root journal atomically, tolerating failure.
// It is used at the *commit* points (F4/C3/D1), which record an outcome AFTER a
// side effect has already run: a lost commit record only means resume re-executes
// (or re-compensates) a step the idempotency contract (§7) already requires to be
// safe, so aborting an otherwise-progressing run would trade a recoverable state
// for an unrecoverable one. A write failure is therefore logged, not returned.
func (w *workflow) persistJournal() {
	if w.journalPersist == nil {
		return
	}
	if err := w.journalPersist(); err != nil {
		w.log().Warn("failed to persist durability journal", "workflowId", w.id, "error", err)
	}
}

// persistJournalCritical writes the journal and returns any error. It is used at
// the *write-ahead* points (F1 started, F5 enter-compensating), where the spec
// forbids proceeding unless the record is durable (durability-spec §5): running a
// side effect — or beginning compensation — without a durable record can strand an
// effect that resume can neither observe nor compensate. The caller MUST abort the
// step (or the rollback) when this returns an error.
func (w *workflow) persistJournalCritical() error {
	if w.journalPersist == nil {
		return nil
	}
	return w.journalPersist()
}

// nodeCursor returns this node's cursor, defaulting an unset phase to forward so
// a not-yet-run node projects sensibly.
func (w *workflow) nodeCursor() Cursor {
	phase := w.jPhase
	if phase == "" {
		phase = PhaseForward
	}
	return Cursor{Phase: phase, Index: w.jIndex}
}

// sharedSnapshot captures this workflow's shared state — the Global namespace,
// with an empty Local — for the journal's `shared` field (durability-spec §3.3).
// Clone failure is fatal to the snapshot so we never persist an empty shared
// bag in place of real state.
func (w *workflow) sharedSnapshot() (*SyncNamespacedStateBag, error) {
	global, err := w.State().Global().Clone()
	if err != nil {
		w.log().Warn("failed to clone global state for journal snapshot", "workflowId", w.id, "error", err)
		return nil, err
	}
	return NewNamespacedStateBag(nil, global), nil
}

// snapshotJournal projects the live workflow tree into a *Journal (the top-level
// node). It is the writer counterpart of the journal schema (durability-spec
// §3.3/§3.8) and is recomputed at every persist point.
func (w *workflow) snapshotJournal() (*Journal, error) {
	shared, err := w.sharedSnapshot()
	if err != nil {
		return nil, err
	}
	steps, err := w.snapshotSteps()
	if err != nil {
		return nil, err
	}
	return &Journal{
		Version:       JournalVersion,
		WorkflowID:    w.id,
		ExecutionMode: w.executionMode,
		RollbackMode:  w.rollbackMode,
		Cursor:        w.nodeCursor(),
		Shared:        shared,
		Steps:         steps,
	}, nil
}

// snapshotSteps projects this node's step entries in topology order. A step that
// is itself a workflow contributes its own recursive run-state trio
// (cursor/shared/steps); a leaf contributes its optional snapshot and report.
// The projection is nil-tolerant so a not-yet-run node reads as all pending.
func (w *workflow) snapshotSteps() ([]*StepJournal, error) {
	out := make([]*StepJournal, len(w.steps))
	for i, s := range w.steps {
		e := &StepJournal{ID: s.Id(), State: w.stepStateAt(i)}
		if child, ok := s.(*workflow); ok {
			childJournal, err := child.snapshotJournal()
			if err != nil {
				return nil, err
			}
			e.Cursor = &childJournal.Cursor
			e.Shared = childJournal.Shared
			e.Steps = childJournal.Steps
		} else {
			if i < len(w.jStepSnapshots) {
				e.Snapshot = w.jStepSnapshots[i]
			}
			if i < len(w.jStepReports) {
				e.Report = w.jStepReports[i]
			}
		}
		out[i] = e
	}
	return out, nil
}

// stepStateAt returns the recorded state of the step at index i, defaulting to
// pending when progress has not been recorded yet.
func (w *workflow) stepStateAt(i int) StepState {
	if i < len(w.jStepStates) && w.jStepStates[i] != "" {
		return w.jStepStates[i]
	}
	return StepPending
}

// ---------------------------------------------------------------------------
// Transition points (durability-spec §5). Each is a no-op unless journaling.
// ---------------------------------------------------------------------------

// journalStepStarted records the write-ahead entry for the step at index i:
// state `started` and the forward cursor, persisted BEFORE the step's side
// effect runs (§5 F1). An implementation MUST NOT run a side effect before its
// started record is durable, so this returns the persist error and the caller
// MUST NOT execute the step when it is non-nil.
func (w *workflow) journalStepStarted(i int) error {
	if !w.journaling() {
		return nil
	}
	w.jPhase = PhaseForward
	w.jIndex = i
	w.jStepStates[i] = StepStarted
	return w.persistJournalCritical()
}

// journalStepCommitted records the commit entry for the step at index i: its
// final state (completed/failed), its rollback snapshot (leaf steps only; pass
// nil for a workflow step), and its report, persisted AFTER the side effect
// returns (§5 F4).
func (w *workflow) journalStepCommitted(i int, state StepState, snapshot *SyncNamespacedStateBag, report *Report) {
	if !w.journaling() {
		return
	}
	w.jStepStates[i] = state
	if snapshot != nil {
		w.jStepSnapshots[i] = snapshot
	}
	if report != nil {
		w.jStepReports[i] = report
	}
	w.persistJournal()
}

// journalEnterCompensating flips this node to the compensating phase with the
// cursor at index i, persisted before compensation begins (§5 F5). This is a
// write-ahead point: it returns the persist error and the caller MUST NOT begin
// compensation when it is non-nil, since a crash mid-rollback would leave a
// journal that still reads as forward and resume would re-run instead of continue.
func (w *workflow) journalEnterCompensating(i int) error {
	if !w.journaling() {
		return nil
	}
	w.jPhase = PhaseCompensating
	w.jIndex = i
	return w.persistJournalCritical()
}

// journalStepCompensated records that the step at index i has been compensated,
// with the cursor at i, persisted before compensation proceeds to a lower index
// (§5 C3) so an interrupted rollback does not repeat it.
func (w *workflow) journalStepCompensated(i int) {
	if !w.journaling() {
		return
	}
	if i >= 0 && i < len(w.jStepStates) {
		w.jStepStates[i] = StepCompensated
	}
	w.jIndex = i
	w.persistJournal()
}

// journalDone marks this node terminal (§5 D1). The run is finished; no further
// step transitions occur. Retention/deletion policy is applied by the caller.
func (w *workflow) journalDone() {
	if !w.journaling() {
		return
	}
	w.jPhase = PhaseDone
	w.persistJournal()
}
