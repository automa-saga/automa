package automa

// Story 2 tests: a workflow with WithJournal produces a correct on-disk journal
// as it runs. These are internal (package automa) so they can load the journal
// with loadJournal and inspect the recorded transitions directly.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func journalPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "wf-run1.journal")
}

// TestJournal_ForwardSuccess checks the forward ordering (§5 F1/F4/D1): the
// write-ahead marks a step started BEFORE its side effect, the commit marks it
// completed with a snapshot AFTER, and the run ends terminal (done).
func TestJournal_ForwardSuccess(t *testing.T) {
	path := journalPath(t)

	// Step a, mid-execute, MUST see itself started and (later) b sees a completed.
	execA := func(ctx context.Context, stp Step) *Report {
		j, err := loadJournal(path)
		require.NoError(t, err)
		assert.Equal(t, StepStarted, j.Steps[0].State, "a must be `started` during its own Execute (write-ahead)")
		assert.Equal(t, StepPending, j.Steps[1].State, "b must still be pending while a runs")
		return SuccessReport(stp)
	}
	execB := func(ctx context.Context, stp Step) *Report {
		j, err := loadJournal(path)
		require.NoError(t, err)
		assert.Equal(t, StepCompleted, j.Steps[0].State, "a must be committed `completed` before b's side effect")
		assert.Equal(t, StepStarted, j.Steps[1].State, "b must be `started` during its own Execute")
		return SuccessReport(stp)
	}

	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(path).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(execA),
			NewStepBuilder().WithId("b").WithExecute(execB),
		).Build()
	require.NoError(t, err)

	report := step.Execute(context.Background())
	require.True(t, report.IsSuccess(), "workflow should succeed")

	j, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase, "terminal run must be `done`")
	require.Len(t, j.Steps, 2)
	assert.Equal(t, StepCompleted, j.Steps[0].State)
	assert.Equal(t, StepCompleted, j.Steps[1].State)
	assert.NotNil(t, j.Steps[0].Snapshot, "a completed leaf must carry a rollback snapshot")
	assert.NotNil(t, j.Steps[0].Report, "a completed leaf must carry its report")
}

// TestJournal_FailureRollback checks that a failure under RollbackOnError flips
// the phase to compensating (§5 F5) and records each compensated step (§5 C3),
// including the failed step itself.
func TestJournal_FailureRollback(t *testing.T) {
	path := journalPath(t)

	var phaseDuringRollbackOfA Phase
	rollbackA := func(ctx context.Context, stp Step) *Report {
		if j, err := loadJournal(path); err == nil {
			phaseDuringRollbackOfA = j.Cursor.Phase
		}
		return SuccessReport(stp)
	}

	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(path).
		Steps(
			NewStepBuilder().WithId("a").
				WithExecute(func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }).
				WithRollback(rollbackA),
			NewStepBuilder().WithId("b").
				WithExecute(func(ctx context.Context, stp Step) *Report {
					return FailureReport(stp, WithError(StepExecutionError.New("boom")))
				}),
		).Build()
	require.NoError(t, err)

	report := step.Execute(context.Background())
	require.True(t, report.IsFailed(), "workflow should fail")

	assert.Equal(t, PhaseCompensating, phaseDuringRollbackOfA,
		"phase must be compensating while a's Rollback runs")

	j, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase, "run is terminal after rollback")
	require.Len(t, j.Steps, 2)
	assert.Equal(t, StepCompensated, j.Steps[0].State, "a (completed) must be compensated")
	assert.Equal(t, StepCompensated, j.Steps[1].State, "b (failed) must be compensated for cleanup")
}

// TestJournal_Disabled_WritesNothing verifies WithJournal is opt-in: a workflow
// built without it has journaling switched off (no persist closure), so every
// transition is inert and nothing is written to disk (backward compatibility).
func TestJournal_Disabled_WritesNothing(t *testing.T) {
	built, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }),
		).Build()
	require.NoError(t, err)

	wf, ok := built.(*workflow)
	require.True(t, ok)
	assert.Empty(t, wf.journalPath, "a workflow without WithJournal must have no journal path")
	assert.Nil(t, wf.journalPersist, "no persist closure should be installed before Execute")
	assert.False(t, wf.journaling(), "journaling must be disabled without WithJournal")

	require.True(t, wf.Execute(context.Background()).IsSuccess())

	// After a full run the persist closure must still be nil: nothing was journaled.
	assert.Nil(t, wf.journalPersist, "journaling must remain disabled through a full run")
	assert.False(t, wf.journaling(), "journaling must remain disabled through a full run")
}

// TestJournal_ManualRollbackReachesDone verifies a directly-invoked Rollback of a
// durable workflow leaves the journal terminal (`done`), not stuck in
// `compensating`, so a later ResumeWorkflow does not re-enter compensation.
func TestJournal_ManualRollbackReachesDone(t *testing.T) {
	path := journalPath(t)
	ok := func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }

	built, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(path).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(ok).WithRollback(ok),
			NewStepBuilder().WithId("b").WithExecute(ok).WithRollback(ok),
		).Build()
	require.NoError(t, err)
	wf := built.(*workflow)

	require.True(t, wf.Execute(context.Background()).IsSuccess())
	j, err := loadJournal(path)
	require.NoError(t, err)
	require.Equal(t, PhaseDone, j.Cursor.Phase, "a successful run must end `done`")

	// Manually roll the durable workflow back.
	require.True(t, wf.Rollback(context.Background()).IsSuccess())

	j, err = loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase,
		"manual rollback must mark the journal terminal (§5 D1), not leave it compensating")
}

// TestJournal_Nested verifies a sub-workflow is journaled inline under its parent
// step entry, recursively (§3.8): the parent entry is a workflow node carrying
// its own cursor/shared/steps, each reaching done.
func TestJournal_Nested(t *testing.T) {
	path := journalPath(t)

	ok := func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }
	sub := NewWorkflowBuilder().WithId("sub").
		Steps(
			NewStepBuilder().WithId("x").WithExecute(ok),
			NewStepBuilder().WithId("y").WithExecute(ok),
		)
	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(path).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(ok),
			sub,
		).Build()
	require.NoError(t, err)

	require.True(t, step.Execute(context.Background()).IsSuccess())

	j, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase)
	require.Len(t, j.Steps, 2)

	// steps[0] "a" is a leaf.
	assert.False(t, j.Steps[0].IsWorkflow())
	assert.Equal(t, StepCompleted, j.Steps[0].State)

	// steps[1] "sub" is a workflow node with its own recursive run-state.
	subEntry := j.Steps[1]
	require.True(t, subEntry.IsWorkflow(), "sub must be a workflow node (has steps)")
	assert.Equal(t, "sub", subEntry.ID)
	assert.Equal(t, StepCompleted, subEntry.State, "sub step is completed once its node is done")
	require.NotNil(t, subEntry.Cursor)
	assert.Equal(t, PhaseDone, subEntry.Cursor.Phase, "sub node reached done")
	require.NotNil(t, subEntry.Shared)
	require.Len(t, subEntry.Steps, 2)
	assert.Equal(t, StepCompleted, subEntry.Steps[0].State)
	assert.Equal(t, StepCompleted, subEntry.Steps[1].State)

	// The loaded journal must satisfy the schema invariants and match its own
	// topology when validated against the definition.
	require.NoError(t, j.validateStructure())
	require.NoError(t, validateTopology(step.(*workflow), j))
}

// TestJournal_NestedStartedSnapshotCarriesParentGlobal is a regression test for
// the write-ahead ordering of a sub-workflow (durability-spec §3.8): the parent
// records the child `started` (F1) with the child's cloned Global BEFORE the child
// runs, so the child node's `shared` snapshot on disk must already carry the
// parent's Global. The sub-workflow's own WithPrepare runs in exactly that window
// — after the parent's F1, before the child persists any of its own progress — so
// it observes the parent's write-ahead snapshot of the child.
func TestJournal_NestedStartedSnapshotCarriesParentGlobal(t *testing.T) {
	path := journalPath(t)

	var sawChildStarted bool
	var sharedAtChildPrepare string

	childPrepare := func(ctx context.Context, stp Step) (context.Context, error) {
		j, err := loadJournal(path)
		if err != nil {
			return ctx, nil
		}
		if len(j.Steps) == 2 && j.Steps[1].IsWorkflow() {
			sawChildStarted = j.Steps[1].State == StepStarted
			if j.Steps[1].Shared != nil {
				if v, ok := j.Steps[1].Shared.Global().String(Key("region")); ok {
					sharedAtChildPrepare = v
				}
			}
		}
		return ctx, nil
	}

	seedGlobal := func(ctx context.Context, stp Step) *Report {
		stp.State().Global().Set(Key("region"), "us-east-1")
		return SuccessReport(stp)
	}
	ok := func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }

	sub := NewWorkflowBuilder().WithId("sub").
		WithPrepare(childPrepare).
		Steps(NewStepBuilder().WithId("x").WithExecute(ok))

	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(path).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(seedGlobal),
			sub,
		).Build()
	require.NoError(t, err)

	require.True(t, step.Execute(context.Background()).IsSuccess())

	assert.True(t, sawChildStarted, "child must be recorded `started` (F1) before it runs any inner step")
	assert.Equal(t, "us-east-1", sharedAtChildPrepare,
		"the F1 snapshot of the child node must carry the parent's Global (write-ahead ordering, §3.8)")
}

// TestJournal_WriteAheadFailureAbortsStep verifies the write-ahead contract
// (durability-spec §5 F1): if the `started` record cannot be made durable, the
// step's side effect MUST NOT run and the step fails with a JournalError, rather
// than executing an effect that a crash could strand with no journal record.
func TestJournal_WriteAheadFailureAbortsStep(t *testing.T) {
	// A journal path inside a directory that does not exist makes every persist
	// fail (the temp file cannot be created), so the write-ahead never becomes
	// durable.
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "wf.journal")

	var executed bool
	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		WithJournal(badPath).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(func(ctx context.Context, stp Step) *Report {
				executed = true
				return SuccessReport(stp)
			}),
		).Build()
	require.NoError(t, err)

	report := step.Execute(context.Background())
	require.True(t, report.IsFailed(), "run must fail when the write-ahead journal cannot be persisted")
	assert.False(t, executed, "the side effect MUST NOT run when its write-ahead record is not durable")

	// The failure must carry a JournalError so callers can distinguish a durability
	// failure from an ordinary step failure.
	var carriesJournalError func(r *Report) bool
	carriesJournalError = func(r *Report) bool {
		if r == nil {
			return false
		}
		if r.Error != nil && errorx.IsOfType(r.Error, JournalError) {
			return true
		}
		for _, c := range r.StepReports {
			if carriesJournalError(c) {
				return true
			}
		}
		return false
	}
	assert.True(t, carriesJournalError(report), "failure must carry a JournalError")
}
