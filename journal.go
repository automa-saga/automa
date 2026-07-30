package automa

// Durability journal (schema v1). This file defines the on-disk journal types
// and the atomic write/read primitives that the rest of the durability feature
// builds on. It introduces no user-visible behavior on its own: nothing here is
// wired into Execute/Rollback yet (that is Durability Story 2).
//
// The journal is the persisted, language-neutral record of a run's progress. Its
// shape and semantics are the normative durability spec, not this code; where
// they disagree the spec governs. See docs/spec/durability-spec.md and the
// conformance fixtures under docs/spec/conformance/journal.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// JournalVersion is the journal schema version this implementation reads and
// writes (durability-spec §3.5). A journal carrying any other version is
// rejected loudly on load rather than resumed, because silently restarting an
// unrecognized journal risks re-executing side effects.
const JournalVersion = 1

// Phase is a workflow node's execution phase as recorded in the journal
// (durability-spec §4.1). It is serialized as a fixed lowercase string.
type Phase string

const (
	// PhaseForward means the node is executing its steps in topology order.
	PhaseForward Phase = "forward"
	// PhaseCompensating means the node is rolling back completed steps in
	// reverse order.
	PhaseCompensating Phase = "compensating"
	// PhaseDone is terminal: the node has finished (success, fully compensated,
	// or terminally failed per its mode).
	PhaseDone Phase = "done"
)

// StepState is a step's recorded state as its parent sees it (durability-spec
// §4.2). It is serialized as a fixed lowercase string.
type StepState string

const (
	// StepPending means the step has not started.
	StepPending StepState = "pending"
	// StepStarted means the write-ahead record was written; the side effect may
	// or may not have run. A step found in this state after a crash is the
	// ambiguous case and is re-executed on resume (durability-spec §6.3).
	StepStarted StepState = "started"
	// StepCompleted means Execute succeeded and the commit point was recorded.
	StepCompleted StepState = "completed"
	// StepFailed means Execute failed.
	StepFailed StepState = "failed"
	// StepCompensated means the step's Rollback completed.
	StepCompensated StepState = "compensated"
)

// Cursor is the execution position of one workflow node: its phase plus the
// index into that node's own steps array (durability-spec §3.4). It never
// addresses across levels; each workflow node carries its own cursor.
type Cursor struct {
	Phase Phase `json:"phase"`
	Index int   `json:"index"`
}

// Journal is the top-level record of a run (durability-spec §3.3). Because a
// workflow is a step, the structure is recursive: a step that is itself a
// workflow carries its own cursor/shared/steps (see StepJournal and §3.8). The
// top level is always a workflow, so it always carries the header plus the full
// run-state trio.
//
// On-disk keys are snake_case and fixed by the spec; implementations MUST NOT
// rename them to match language idioms.
type Journal struct {
	// Header — stable identity and configuration.
	Version       int      `json:"version"`
	WorkflowID    string   `json:"workflow_id"`
	ExecutionMode TypeMode `json:"execution_mode"`
	RollbackMode  TypeMode `json:"rollback_mode"`

	// Run state.
	Cursor Cursor                  `json:"cursor"`
	Shared *SyncNamespacedStateBag `json:"shared"`
	Steps  []*StepJournal          `json:"steps"`
}

// StepJournal is one entry in a node's steps array (durability-spec §3.3, §3.8).
//
// A leaf (ordinary) step carries id, state, and optionally snapshot/report. A
// workflow step instead carries the run-state trio cursor/shared/steps (the same
// shape the top level has, minus the header, which it inherits) and omits the
// leaf snapshot. This is the workflow-node invariant: a node is a workflow iff
// it has a steps array, in which case cursor and shared are present too; a leaf
// carries none of the three.
type StepJournal struct {
	ID    string    `json:"id"`
	State StepState `json:"state"`

	// Leaf-step fields. snapshot is the execution-time state captured for
	// rollback; report is the step's report tree. Both are OPTIONAL and omitted
	// when not yet produced. A workflow step omits snapshot entirely (§3.8).
	Snapshot *SyncNamespacedStateBag `json:"snapshot,omitempty"`
	Report   *Report                 `json:"report,omitempty"`

	// Workflow-step run-state trio, present iff this entry is itself a workflow
	// (the workflow-node invariant). A leaf step carries none of the three.
	Cursor *Cursor                 `json:"cursor,omitempty"`
	Shared *SyncNamespacedStateBag `json:"shared,omitempty"`
	Steps  []*StepJournal          `json:"steps,omitempty"`
}

// IsWorkflow reports whether this entry is a workflow node. Per the
// workflow-node invariant (durability-spec §3.3/§3.8) a node is a workflow iff
// its entry carries a steps array.
func (s *StepJournal) IsWorkflow() bool { return s.Steps != nil }

// validateStructure checks the journal against the schema invariants that a
// well-formed journal MUST satisfy (durability-spec §3.3, §3.5). It is applied
// on load so a malformed journal fails loudly rather than resuming wrongly.
func (j *Journal) validateStructure() error {
	if j.Version != JournalVersion {
		return JournalUnsupportedVersion.New(
			"journal version %d is not supported by this build (expected %d)",
			j.Version, JournalVersion)
	}
	// The top level is always a workflow node: cursor and shared are implied by
	// the struct; steps must be present.
	if j.Steps == nil {
		return JournalCorrupt.New("journal has no steps array")
	}
	for _, s := range j.Steps {
		if err := s.validateStructure(); err != nil {
			return err
		}
	}
	return nil
}

// validateStructure enforces the workflow-node invariant on a step entry and
// recurses into workflow steps (durability-spec §3.3, §3.8).
func (s *StepJournal) validateStructure() error {
	if s.ID == "" {
		return JournalCorrupt.New("journal step entry has an empty id")
	}
	if s.IsWorkflow() {
		// A workflow step MUST carry cursor and shared, and MUST NOT carry the
		// leaf snapshot.
		if s.Cursor == nil {
			return JournalCorrupt.New("workflow step %q is missing its cursor", s.ID)
		}
		if s.Shared == nil {
			return JournalCorrupt.New("workflow step %q is missing its shared state", s.ID)
		}
		if s.Snapshot != nil {
			return JournalCorrupt.New("workflow step %q must not carry a leaf snapshot", s.ID)
		}
		for _, child := range s.Steps {
			if err := child.validateStructure(); err != nil {
				return err
			}
		}
		return nil
	}
	// A leaf step MUST carry none of the run-state trio.
	if s.Cursor != nil || s.Shared != nil {
		return JournalCorrupt.New("leaf step %q must not carry cursor/shared", s.ID)
	}
	return nil
}

// persist atomically writes the journal to path (durability-spec §3.6). A
// reader — including a post-crash reader — observes either the complete previous
// journal or the complete new one, never a torn file. The procedure is:
// serialize, write to a temp file in the same directory, fsync it before the
// rename, then atomically rename it over the target. The parent directory is
// also fsynced so the rename itself survives power loss.
func (j *Journal) persist(path string) error {
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return JournalError.Wrap(err, "marshal journal")
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".journal-*.tmp")
	if err != nil {
		return JournalError.Wrap(err, "create temp journal in %s", dir)
	}
	tmpName := tmp.Name()

	// Clean up the temp file if we return before a successful rename. After a
	// successful rename the temp path no longer exists, so the removal is a
	// harmless no-op whose error is intentionally ignored.
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return JournalError.Wrap(err, "write temp journal")
	}
	// Flush data to stable storage before the rename (§3.6 step 3) — REQUIRED to
	// survive power loss, not merely a process crash.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return JournalError.Wrap(err, "fsync temp journal")
	}
	if err := tmp.Close(); err != nil {
		return JournalError.Wrap(err, "close temp journal")
	}

	// Atomic rename over the target (§3.6 step 4).
	if err := os.Rename(tmpName, path); err != nil {
		return JournalError.Wrap(err, "rename temp journal over %s", path)
	}
	renamed = true

	// fsync the parent directory so the rename is itself durable across power
	// loss. A directory that cannot be synced (e.g. some non-POSIX targets) is
	// not fatal to the all-or-nothing guarantee of the rename itself.
	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// loadJournal reads and decodes a journal from path (durability-spec §6.2).
//
// A missing file returns an error satisfying errors.Is(err, os.ErrNotExist);
// callers implementing resume treat that as a fresh run (§6.2). A file that
// cannot be decoded, or one carrying an unsupported version, fails loudly: this
// function never silently discards or restarts a journal, because doing so could
// re-execute side effects.
func loadJournal(path string) (*Journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Return os errors (including not-exist) unwrapped so callers can use
		// errors.Is(err, os.ErrNotExist) to detect the fresh-run case.
		return nil, err
	}

	var j Journal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, JournalCorrupt.Wrap(err, "decode journal at %s", path)
	}
	if err := j.validateStructure(); err != nil {
		return nil, err
	}
	return &j, nil
}

// validateTopology checks that a workflow definition matches the topology and
// modes recorded in the journal (durability-spec §6.2). Resume refuses to
// proceed on any mismatch — differing step IDs or order at any depth, or
// differing execution/rollback modes. This is the single-process analogue of
// workflow versioning and is intentionally strict, because rehydrating a journal
// onto a changed definition could re-run or skip the wrong side effects.
//
// Only the top-level modes are compared: a sub-workflow step inherits its
// enclosing workflow's modes (durability-spec §3.8) and does not record its own.
func validateTopology(wf *workflow, j *Journal) error {
	if wf == nil || j == nil {
		return JournalTopologyMismatch.New("cannot validate a nil workflow or journal")
	}
	if wf.Id() != j.WorkflowID {
		return JournalTopologyMismatch.New(
			"workflow id %q does not match journal %q", wf.Id(), j.WorkflowID)
	}
	if wf.executionMode != j.ExecutionMode {
		return JournalTopologyMismatch.New(
			"execution mode %q does not match journal %q",
			wf.executionMode.String(), j.ExecutionMode.String())
	}
	if wf.rollbackMode != j.RollbackMode {
		return JournalTopologyMismatch.New(
			"rollback mode %q does not match journal %q",
			wf.rollbackMode.String(), j.RollbackMode.String())
	}
	return validateStepsTopology(wf.Id(), wf.Steps(), j.Steps)
}

// validateStepsTopology compares an ordered list of definition steps against the
// journal's step entries at one level, recursing into workflow steps. path names
// the enclosing workflow for error messages.
func validateStepsTopology(path string, steps []Step, entries []*StepJournal) error {
	if len(steps) != len(entries) {
		return JournalTopologyMismatch.New(
			"workflow %q defines %d steps but journal records %d",
			path, len(steps), len(entries))
	}
	for i, step := range steps {
		entry := entries[i]
		if step.Id() != entry.ID {
			return JournalTopologyMismatch.New(
				"workflow %q step %d: definition id %q does not match journal id %q",
				path, i, step.Id(), entry.ID)
		}
		child, defIsWorkflow := step.(*workflow)
		if defIsWorkflow != entry.IsWorkflow() {
			return JournalTopologyMismatch.New(
				"step %q: definition is %s but journal records %s",
				entry.ID, stepKind(defIsWorkflow), stepKind(entry.IsWorkflow()))
		}
		if defIsWorkflow {
			if err := validateStepsTopology(entry.ID, child.Steps(), entry.Steps); err != nil {
				return err
			}
		}
	}
	return nil
}

func stepKind(isWorkflow bool) string {
	if isWorkflow {
		return "a workflow step"
	}
	return "a leaf step"
}
