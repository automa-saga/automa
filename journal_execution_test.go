package automa

// Story 2 tests: a workflow with WithJournal produces a correct on-disk journal
// as it runs. These are internal (package automa) so they can load the journal
// with loadJournal and inspect the recorded transitions directly.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
// built without it writes no file (backward compatibility).
func TestJournal_Disabled_WritesNothing(t *testing.T) {
	dir := t.TempDir()

	step, err := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(func(ctx context.Context, stp Step) *Report { return SuccessReport(stp) }),
		).Build()
	require.NoError(t, err)

	require.True(t, step.Execute(context.Background()).IsSuccess())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a workflow without WithJournal must write nothing to disk")
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
