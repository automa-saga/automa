package automa

// Story 3 tests: the crash-and-resume scenarios. Each test hand-crafts an
// on-disk journal representing a specific crash point, then calls ResumeWorkflow
// with a fresh (identical) definition and asserts exactly which steps run. This
// is deterministic where a real crash is not.

import (
	"context"
	"os"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resumeRecorder captures which steps run Execute / Rollback on resume.
type resumeRecorder struct {
	exec     []string
	rollback []string
}

// recStep builds a leaf step that records its Execute and Rollback. execFail
// makes its Execute return a failure report.
func recStep(rec *resumeRecorder, id string, execFail bool) *StepBuilder {
	return NewStepBuilder().WithId(id).
		WithExecute(func(ctx context.Context, stp Step) *Report {
			rec.exec = append(rec.exec, id)
			if execFail {
				return FailureReport(stp, WithError(StepExecutionError.New("fixture: %s failed", id)))
			}
			return SuccessReport(stp)
		}).
		WithRollback(func(ctx context.Context, stp Step) *Report {
			rec.rollback = append(rec.rollback, id)
			return SuccessReport(stp)
		})
}

// craftJournal writes a journal for workflow "wf" with the given modes, cursor,
// and leaf step entries.
func craftJournal(t *testing.T, path string, exec, rb TypeMode, phase Phase, idx int, entries []*StepJournal) {
	t.Helper()
	j := &Journal{
		Version:       JournalVersion,
		WorkflowID:    "wf",
		ExecutionMode: exec,
		RollbackMode:  rb,
		Cursor:        Cursor{Phase: phase, Index: idx},
		Shared:        &SyncNamespacedStateBag{},
		Steps:         entries,
	}
	require.NoError(t, j.persist(path))
}

// TestResume_CrashAfterCommit_SkipsCompleted: a crash after a clean commit
// leaves completed steps that MUST NOT re-run; the remaining steps run.
func TestResume_CrashAfterCommit_SkipsCompleted(t *testing.T) {
	path := journalPath(t)
	craftJournal(t, path, RollbackOnError, ContinueOnError, PhaseForward, 0, []*StepJournal{
		{ID: "a", State: StepCompleted},
		{ID: "b", State: StepPending},
	})

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsSuccess(), "resumed run should succeed")
	assert.Equal(t, []string{"b"}, rec.exec, "only b should run; a was already completed")

	j, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase)
	assert.Equal(t, StepCompleted, j.Steps[1].State)
}

// TestResume_StartedStepRerunsOnce: a step recorded `started` but not
// `completed` (the ambiguous case) re-runs exactly once on resume.
func TestResume_StartedStepRerunsOnce(t *testing.T) {
	path := journalPath(t)
	craftJournal(t, path, RollbackOnError, ContinueOnError, PhaseForward, 1, []*StepJournal{
		{ID: "a", State: StepCompleted},
		{ID: "b", State: StepStarted},
	})

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsSuccess())
	assert.Equal(t, []string{"b"}, rec.exec, "b must re-run exactly once; a skipped")
}

// TestResume_ContinuesCompensation: a crash mid-compensation resumes the
// rollback from the cursor, skipping already-compensated steps.
func TestResume_ContinuesCompensation(t *testing.T) {
	path := journalPath(t)
	// Forward reached c and failed; c and b already compensated; a remains.
	craftJournal(t, path, RollbackOnError, ContinueOnError, PhaseCompensating, 1, []*StepJournal{
		{ID: "a", State: StepCompleted},
		{ID: "b", State: StepCompensated},
		{ID: "c", State: StepCompensated},
	})

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false), recStep(rec, "c", true))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsFailed(), "a compensated run is a failure outcome")
	assert.Equal(t, []string{"a"}, rec.rollback, "only a remains to compensate; b and c already were")
	assert.Empty(t, rec.exec, "no forward execution during compensation resume")

	j, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j.Cursor.Phase)
	assert.Equal(t, StepCompensated, j.Steps[0].State)
}

// TestResume_TopologyMismatchRefused: a definition that no longer matches the
// journal is refused, and nothing runs.
func TestResume_TopologyMismatchRefused(t *testing.T) {
	path := journalPath(t)
	craftJournal(t, path, RollbackOnError, ContinueOnError, PhaseForward, 0, []*StepJournal{
		{ID: "a", State: StepCompleted},
		{ID: "b", State: StepPending},
	})

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "different", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsFailed())
	assert.True(t, errorx.IsOfType(report.Error, JournalTopologyMismatch),
		"want a topology-mismatch error, got %v", report.Error)
	assert.Empty(t, rec.exec, "nothing must run on a refused resume")
}

// TestResume_CorruptJournalFailsLoudly: a corrupt journal fails loudly and runs
// nothing (never silently restarts).
func TestResume_CorruptJournalFailsLoudly(t *testing.T) {
	path := journalPath(t)
	require.NoError(t, os.WriteFile(path, []byte("{ not json"), 0o600))

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsFailed())
	assert.True(t, errorx.IsOfType(report.Error, JournalCorrupt), "want JournalCorrupt, got %v", report.Error)
	assert.Empty(t, rec.exec, "nothing must run for a corrupt journal")
}

// TestResume_MissingJournalRunsFresh: resuming a never-started run is a normal
// start and creates the journal.
func TestResume_MissingJournalRunsFresh(t *testing.T) {
	path := journalPath(t) // does not exist yet

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsSuccess())
	assert.Equal(t, []string{"a", "b"}, rec.exec, "a fresh run executes every step")

	j, err := loadJournal(path)
	require.NoError(t, err, "a fresh resumed run must create the journal")
	assert.Equal(t, PhaseDone, j.Cursor.Phase)
}

// TestResume_DonePhaseReturnsFinalRunsNothing: a terminal journal returns the
// recorded result and executes/compensates nothing.
func TestResume_DonePhaseReturnsFinalRunsNothing(t *testing.T) {
	path := journalPath(t)
	craftJournal(t, path, RollbackOnError, ContinueOnError, PhaseDone, 1, []*StepJournal{
		{ID: "a", State: StepCompleted},
		{ID: "b", State: StepCompleted},
	})

	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsSuccess(), "a done journal with all steps completed returns success")
	assert.Empty(t, rec.exec, "a terminal resume runs nothing")
	assert.Empty(t, rec.rollback)
}

// TestResume_EndToEnd_CrashThenResume exercises the real S2→S3 integration: a
// first attempt journals a completed and then crashes mid-b (simulated with a
// panic, which leaves b `started` because its write-ahead persisted before the
// side effect). A resume with a healthy b re-runs only b, exactly once.
func TestResume_EndToEnd_CrashThenResume(t *testing.T) {
	path := journalPath(t)

	// First attempt: a succeeds, b crashes after its write-ahead record.
	func() {
		defer func() { _ = recover() }()
		wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).WithJournal(path).
			Steps(
				recStep(&resumeRecorder{}, "a", false),
				NewStepBuilder().WithId("b").WithExecute(func(ctx context.Context, stp Step) *Report {
					panic("simulated crash during b")
				}),
			)
		step, err := wb.Build()
		require.NoError(t, err)
		_ = step.Execute(context.Background())
	}()

	// The engine-written journal captures the crash point: a completed, b started.
	j, err := loadJournal(path)
	require.NoError(t, err)
	require.Equal(t, PhaseForward, j.Cursor.Phase)
	require.Equal(t, StepCompleted, j.Steps[0].State)
	require.Equal(t, StepStarted, j.Steps[1].State, "b's write-ahead must have persisted before the crash")

	// Resume with a healthy b.
	rec := &resumeRecorder{}
	wb := NewWorkflowBuilder().WithId("wf").WithExecutionMode(RollbackOnError).
		Steps(recStep(rec, "a", false), recStep(rec, "b", false))

	report := ResumeWorkflow(context.Background(), wb, path)
	require.True(t, report.IsSuccess())
	assert.Equal(t, []string{"b"}, rec.exec, "resume re-runs only b; a stays completed")

	j2, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, PhaseDone, j2.Cursor.Phase)
}
