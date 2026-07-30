package automa

// Unit tests for the durability journal foundation (Story 1): journal types and
// the atomic persist / load / validateTopology primitives. These are internal
// (package automa) because they exercise unexported helpers; the cross-language
// round-trip guarantees live in the conformance harness (conformance_test.go).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noopExec(_ context.Context, stp Step) *Report { return SuccessReport(stp) }

// sampleJournal returns a small but structurally complete journal: a top-level
// workflow with one leaf step and one nested workflow step.
func sampleJournal() *Journal {
	return &Journal{
		Version:       JournalVersion,
		WorkflowID:    "wf",
		ExecutionMode: RollbackOnError,
		RollbackMode:  ContinueOnError,
		Cursor:        Cursor{Phase: PhaseForward, Index: 1},
		Shared:        &SyncNamespacedStateBag{},
		Steps: []*StepJournal{
			{ID: "a", State: StepCompleted, Snapshot: &SyncNamespacedStateBag{}},
			{
				ID:     "sub",
				State:  StepStarted,
				Cursor: &Cursor{Phase: PhaseForward, Index: 0},
				Shared: &SyncNamespacedStateBag{},
				Steps: []*StepJournal{
					{ID: "x", State: StepCompleted},
					{ID: "y", State: StepPending},
				},
			},
		},
	}
}

// sampleWorkflow builds the workflow definition that matches sampleJournal.
func sampleWorkflow(t *testing.T) *workflow {
	t.Helper()
	sub := NewWorkflowBuilder().WithId("sub").
		WithExecutionMode(RollbackOnError).WithRollbackMode(ContinueOnError).
		Steps(
			NewStepBuilder().WithId("x").WithExecute(noopExec),
			NewStepBuilder().WithId("y").WithExecute(noopExec),
		)
	wb := NewWorkflowBuilder().WithId("wf").
		WithExecutionMode(RollbackOnError).WithRollbackMode(ContinueOnError).
		Steps(
			NewStepBuilder().WithId("a").WithExecute(noopExec),
			sub,
		)
	step, err := wb.Build()
	require.NoError(t, err)
	wf, ok := step.(*workflow)
	require.True(t, ok, "expected a *workflow")
	return wf
}

func TestJournal_JSONRoundTripsLosslessly(t *testing.T) {
	j := sampleJournal()
	data, err := json.Marshal(j)
	require.NoError(t, err)

	var got Journal
	require.NoError(t, json.Unmarshal(data, &got))

	// Re-serialize and compare structurally: a lossless round-trip.
	again, err := json.Marshal(&got)
	require.NoError(t, err)

	var a, b interface{}
	require.NoError(t, json.Unmarshal(data, &a))
	require.NoError(t, json.Unmarshal(again, &b))
	assert.Equal(t, a, b, "journal did not round-trip losslessly")

	// Spot-check that the recursive structure survived.
	require.Len(t, got.Steps, 2)
	assert.False(t, got.Steps[0].IsWorkflow(), "leaf step must not be a workflow node")
	assert.True(t, got.Steps[1].IsWorkflow(), "nested step must be a workflow node")
	require.NotNil(t, got.Steps[1].Cursor)
	assert.Equal(t, PhaseForward, got.Steps[1].Cursor.Phase)
}

func TestJournal_PersistThenLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wf-run1.journal")

	require.NoError(t, sampleJournal().persist(path))

	loaded, err := loadJournal(path)
	require.NoError(t, err)
	assert.Equal(t, "wf", loaded.WorkflowID)
	assert.Equal(t, RollbackOnError, loaded.ExecutionMode)
	require.Len(t, loaded.Steps, 2)

	// No stray temp files were left behind.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "persist must leave only the target file")
}

// TestJournal_PersistIsAtomic verifies the all-or-nothing guarantee (§3.6): a
// concurrent reader always observes a complete journal, never a torn file, even
// while the writer rewrites the snapshot many times.
func TestJournal_PersistIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wf-run1.journal")
	require.NoError(t, sampleJournal().persist(path))

	const writes = 200
	var stop atomic.Bool
	var wg sync.WaitGroup

	// Failures are reported back to the test goroutine via channels: calling
	// require.* (t.FailNow) from a non-test goroutine is unsafe.
	writerErr := make(chan error, 1)
	readerErr := make(chan error, 1)

	// Writer: rewrite the journal repeatedly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stop.Store(true)
		for i := 0; i < writes; i++ {
			j := sampleJournal()
			j.Cursor.Index = i % 2 // vary the content between writes
			if err := j.persist(path); err != nil {
				writerErr <- fmt.Errorf("persist %d: %w", i, err)
				return
			}
		}
	}()

	// Reader: every read must decode to a complete, valid journal. Because the
	// rename is atomic, the target always exists — the old file survives until the
	// new one replaces it — so ANY read error (including os.ErrNotExist) is an
	// atomicity violation, and a decode failure means the reader saw a torn file.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			data, err := os.ReadFile(path)
			if err != nil {
				readerErr <- fmt.Errorf("read observed a missing/unreadable target (atomicity violation): %w", err)
				return
			}
			var j Journal
			if err := json.Unmarshal(data, &j); err != nil {
				readerErr <- fmt.Errorf("reader observed a torn file: %w", err)
				return
			}
			if j.Version != JournalVersion {
				readerErr <- fmt.Errorf("reader observed version %d, want %d", j.Version, JournalVersion)
				return
			}
		}
	}()

	wg.Wait()
	close(writerErr)
	close(readerErr)
	for err := range writerErr {
		t.Errorf("writer goroutine: %v", err)
	}
	for err := range readerErr {
		t.Errorf("reader goroutine: %v", err)
	}
}

func TestLoadJournal_MissingFileIsNotExist(t *testing.T) {
	_, err := loadJournal(filepath.Join(t.TempDir(), "does-not-exist.journal"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist),
		"missing journal must be detectable with errors.Is(os.ErrNotExist) for the fresh-run case")
}

// TestLoadJournal_NullStepEntryFailsLoudly verifies a null element in a steps
// array (`"steps": [null]`) is treated as corruption and fails loudly on load,
// rather than panicking on a nil *StepJournal dereference.
func TestLoadJournal_NullStepEntryFailsLoudly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wf.journal")
	raw := `{"version":1,"workflow_id":"wf","execution_mode":"rollback","rollback_mode":"continue",` +
		`"cursor":{"phase":"forward","index":0},"shared":{"local":{},"global":{}},"steps":[null]}`
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	_, err := loadJournal(path)
	require.Error(t, err)
	assert.True(t, errorx.IsOfType(err, JournalCorrupt),
		"a null step entry must fail as JournalCorrupt, got %v", err)
}

func TestLoadJournal_CorruptFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.journal")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o600))

	_, err := loadJournal(path)
	require.Error(t, err)
	assert.True(t, errorx.IsOfType(err, JournalCorrupt), "corrupt journal must fail as JournalCorrupt, got %v", err)
}

func TestLoadJournal_UnsupportedVersionFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "future.journal")
	j := sampleJournal()
	data, err := json.Marshal(j)
	require.NoError(t, err)
	// Rewrite the version to an unsupported one.
	bumped := strings.Replace(string(data), `"version":1`, `"version":999`, 1)
	require.NoError(t, os.WriteFile(path, []byte(bumped), 0o600))

	_, err = loadJournal(path)
	require.Error(t, err)
	assert.True(t, errorx.IsOfType(err, JournalUnsupportedVersion),
		"unsupported version must fail as JournalUnsupportedVersion, got %v", err)
}

func TestJournal_ValidateStructure_Invariants(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(j *Journal)
		wantErr bool
	}{
		{name: "valid", mutate: func(*Journal) {}, wantErr: false},
		{
			name:    "top-level cursor out of range",
			mutate:  func(j *Journal) { j.Cursor.Index = 100 },
			wantErr: true,
		},
		{
			name:    "top-level cursor unknown phase",
			mutate:  func(j *Journal) { j.Cursor.Phase = Phase("garbage") },
			wantErr: true,
		},
		{
			name:    "top-level shared missing",
			mutate:  func(j *Journal) { j.Shared = nil },
			wantErr: true,
		},
		{
			name:    "leaf carries a cursor",
			mutate:  func(j *Journal) { j.Steps[0].Cursor = &Cursor{Phase: PhaseForward} },
			wantErr: true,
		},
		{
			name:    "workflow step missing shared",
			mutate:  func(j *Journal) { j.Steps[1].Shared = nil },
			wantErr: true,
		},
		{
			name:    "workflow step carries a leaf snapshot",
			mutate:  func(j *Journal) { j.Steps[1].Snapshot = &SyncNamespacedStateBag{} },
			wantErr: true,
		},
		{
			name:    "empty step id",
			mutate:  func(j *Journal) { j.Steps[0].ID = "" },
			wantErr: true,
		},
		{
			name:    "leaf has unknown state",
			mutate:  func(j *Journal) { j.Steps[0].State = StepState("quantum") },
			wantErr: true,
		},
		{
			name: "done cursor ignores index",
			mutate: func(j *Journal) {
				j.Cursor.Phase = PhaseDone
				j.Cursor.Index = 100
			},
			wantErr: false,
		},
		{
			name: "nested workflow cursor out of range",
			mutate: func(j *Journal) {
				j.Steps[1].Cursor.Index = 99
			},
			wantErr: true,
		},
		{
			name: "nested workflow step has unknown state",
			mutate: func(j *Journal) {
				j.Steps[1].Steps[0].State = StepState("quantum")
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			j := sampleJournal()
			tc.mutate(j)
			err := j.validateStructure()
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errorx.IsOfType(err, JournalCorrupt), "want JournalCorrupt, got %v", err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTopology_MatchAndMismatch(t *testing.T) {
	wf := sampleWorkflow(t)

	// Exact match succeeds.
	require.NoError(t, validateTopology(wf, sampleJournal()))

	t.Run("pending workflow may be leaf-shaped", func(t *testing.T) {
		j := sampleJournal()
		j.Steps[1] = &StepJournal{
			ID:       "sub",
			State:    StepPending,
			Snapshot: &SyncNamespacedStateBag{},
			Report:   SuccessReport(&defaultStep{id: "sub"}),
		}
		require.NoError(t, validateTopology(wf, j))
	})

	tests := []struct {
		name   string
		mutate func(j *Journal)
	}{
		{"workflow id", func(j *Journal) { j.WorkflowID = "other" }},
		{"execution mode", func(j *Journal) { j.ExecutionMode = StopOnError }},
		{"rollback mode", func(j *Journal) { j.RollbackMode = StopOnError }},
		{"step count", func(j *Journal) { j.Steps = j.Steps[:1] }},
		{"step order", func(j *Journal) { j.Steps[0].ID, j.Steps[1].ID = j.Steps[1].ID, j.Steps[0].ID }},
		{"nested step id", func(j *Journal) { j.Steps[1].Steps[0].ID = "zzz" }},
		{"kind mismatch (leaf vs workflow)", func(j *Journal) {
			// Make the definition's workflow step appear as a leaf in the journal.
			j.Steps[1].Steps = nil
			j.Steps[1].Cursor = nil
			j.Steps[1].Shared = nil
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			j := sampleJournal()
			tc.mutate(j)
			err := validateTopology(wf, j)
			require.Error(t, err)
			assert.True(t, errorx.IsOfType(err, JournalTopologyMismatch), "want JournalTopologyMismatch, got %v", err)
		})
	}
}
