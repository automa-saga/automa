package automa_test

// Conformance harness. Loads the language-neutral fixtures under
// docs/spec/conformance/ and asserts the Go reference implementation produces
// the behavior the spec fixtures describe. Every conformant implementation
// (Go first, then other languages) MUST pass the same fixtures unchanged.
//
// See docs/spec/conformance/README.md for the fixture format and conventions.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/automa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	conformanceBehaviorDir      = "docs/spec/conformance/behavior"
	conformanceSerializationDir = "docs/spec/conformance/serialization"
)

// ---------------------------------------------------------------------------
// Behavior fixtures
// ---------------------------------------------------------------------------

type behaviorFixture struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SpecRefs    []string        `json:"specRefs"`
	Workflow    fixtureWorkflow `json:"workflow"`
	Expect      behaviorExpect  `json:"expect"`
}

type fixtureWorkflow struct {
	ID            string        `json:"id"`
	ExecutionMode string        `json:"executionMode"`
	RollbackMode  string        `json:"rollbackMode"`
	Steps         []fixtureStep `json:"steps"`
}

type fixtureStep struct {
	ID string `json:"id"`

	// Leaf-step declared outcomes.
	Prepare  string `json:"prepare"`  // "" (ok) | failed
	Execute  string `json:"execute"`  // success | failed | skipped | none (omit Execute → invalid step)
	Rollback string `json:"rollback"` // success | failed (default: success)

	// When Steps is non-empty this step is a sub-workflow; Execute/Rollback are
	// ignored and these fields configure the nested workflow.
	ExecutionMode string        `json:"executionMode"`
	RollbackMode  string        `json:"rollbackMode"`
	Steps         []fixtureStep `json:"steps"`
}

func (s fixtureStep) isWorkflow() bool { return len(s.Steps) > 0 }

type behaviorExpect struct {
	WorkflowStatus string                  `json:"workflowStatus"`
	WorkflowAction string                  `json:"workflowAction"` // optional; e.g. "prepare" for a build/validation failure
	ExecutionOrder []string                `json:"executionOrder"`
	RollbackOrder  []string                `json:"rollbackOrder"`
	Steps          map[string]expectedStep `json:"steps"`
}

type expectedStep struct {
	Status         string `json:"status"`
	Action         string `json:"action"`
	RollbackStatus string `json:"rollbackStatus"`
}

// recorder captures the order in which leaf steps run Execute and Rollback.
// Execution is strictly sequential (core-spec §4.1) so a plain slice is safe.
type recorder struct {
	execOrder     []string
	rollbackOrder []string
}

// assertAllowed fails the test when a fixture declares an outcome outside the
// permitted set, so a malformed fixture cannot silently pass.
func assertAllowed(t *testing.T, stepID, field, value string, allowed ...string) {
	t.Helper()
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	t.Fatalf("fixture step %q: unknown %s outcome %q (allowed: %v)", stepID, field, value, allowed)
}

func parseMode(t *testing.T, s string) automa.TypeMode {
	t.Helper()
	switch s {
	case "continue":
		return automa.ContinueOnError
	case "stop":
		return automa.StopOnError
	case "rollback":
		return automa.RollbackOnError
	default:
		t.Fatalf("unknown mode %q in fixture", s)
		return 0
	}
}

func buildFixtureStep(t *testing.T, rec *recorder, fs fixtureStep) automa.Builder {
	t.Helper()

	if fs.isWorkflow() {
		wb := automa.NewWorkflowBuilder().WithId(fs.ID)
		if fs.ExecutionMode != "" {
			wb.WithExecutionMode(parseMode(t, fs.ExecutionMode))
		}
		if fs.RollbackMode != "" {
			wb.WithRollbackMode(parseMode(t, fs.RollbackMode))
		}
		subs := make([]automa.Builder, 0, len(fs.Steps))
		for _, child := range fs.Steps {
			subs = append(subs, buildFixtureStep(t, rec, child))
		}
		wb.Steps(subs...)
		return wb
	}

	id := fs.ID
	prepOutcome := fs.Prepare
	execOutcome := fs.Execute
	rbOutcome := fs.Rollback

	// Reject unknown declared outcomes up front. Fixtures are authoritative, so a
	// typo must fail the suite rather than silently fall through to a default.
	assertAllowed(t, id, "prepare", prepOutcome, "", "failed")
	assertAllowed(t, id, "execute", execOutcome, "success", "failed", "skipped", "none")
	assertAllowed(t, id, "rollback", rbOutcome, "", "success", "failed")

	sb := automa.NewStepBuilder().WithId(id)
	if prepOutcome == "failed" {
		sb.WithPrepare(func(ctx context.Context, stp automa.Step) (context.Context, error) {
			// Honor the context contract: return the incoming ctx, not nil, on error.
			return ctx, errors.New("fixture: declared prepare failure")
		})
	}

	// "none" omits Execute entirely, producing an invalid step (core-spec §3.2.1)
	// so the fixture can assert build/validation behavior.
	if execOutcome == "none" {
		return sb
	}

	return sb.
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			rec.execOrder = append(rec.execOrder, id)
			switch execOutcome {
			case "failed":
				return automa.FailureReport(stp, automa.WithError(errors.New("fixture: declared failure")))
			case "skipped":
				return automa.SkippedReport(stp)
			default:
				return automa.SuccessReport(stp)
			}
		}).
		WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
			rec.rollbackOrder = append(rec.rollbackOrder, id)
			if rbOutcome == "failed" {
				return automa.FailureReport(stp, automa.WithError(errors.New("fixture: declared rollback failure")))
			}
			return automa.SuccessReport(stp)
		})
}

// collectStepReports indexes every step report in the tree by step ID.
func collectStepReports(r *automa.Report, out map[string]*automa.Report) {
	for _, sr := range r.StepReports {
		if sr == nil {
			continue
		}
		out[sr.Id] = sr
		collectStepReports(sr, out)
	}
}

func runBehaviorFixture(t *testing.T, fx behaviorFixture) {
	rec := &recorder{}

	wb := automa.NewWorkflowBuilder().WithId(fx.Workflow.ID)
	if fx.Workflow.ExecutionMode != "" {
		wb.WithExecutionMode(parseMode(t, fx.Workflow.ExecutionMode))
	}
	if fx.Workflow.RollbackMode != "" {
		wb.WithRollbackMode(parseMode(t, fx.Workflow.RollbackMode))
	}
	subs := make([]automa.Builder, 0, len(fx.Workflow.Steps))
	for _, s := range fx.Workflow.Steps {
		subs = append(subs, buildFixtureStep(t, rec, s))
	}
	wb.Steps(subs...)

	report := automa.RunWorkflow(context.Background(), wb)
	require.NotNil(t, report, "RunWorkflow returned nil report")

	// Workflow-level status.
	assert.Equal(t, fx.Expect.WorkflowStatus, report.Status.String(), "workflow status")

	// Workflow-level action, when the fixture asserts one (e.g. build failure).
	if fx.Expect.WorkflowAction != "" {
		assert.Equal(t, fx.Expect.WorkflowAction, report.Action.String(), "workflow action")
	}

	// Execution order (leaf steps that actually ran Execute, in order).
	assert.Equal(t, fx.Expect.ExecutionOrder, normalizeOrder(rec.execOrder), "execution order")

	// Rollback order, when the fixture declares one.
	if fx.Expect.RollbackOrder != nil {
		assert.Equal(t, fx.Expect.RollbackOrder, normalizeOrder(rec.rollbackOrder), "rollback order")
	}

	// Per-step report expectations.
	if len(fx.Expect.Steps) > 0 {
		index := map[string]*automa.Report{}
		collectStepReports(report, index)
		for id, want := range fx.Expect.Steps {
			got, ok := index[id]
			if !assert.Truef(t, ok, "expected a report for step %q", id) {
				continue
			}
			if want.Status != "" {
				assert.Equalf(t, want.Status, got.Status.String(), "step %q status", id)
			}
			if want.Action != "" {
				assert.Equalf(t, want.Action, got.Action.String(), "step %q action", id)
			}
			if want.RollbackStatus != "" {
				if assert.NotNilf(t, got.Rollback, "step %q expected a rollback report", id) {
					assert.Equalf(t, want.RollbackStatus, got.Rollback.Status.String(), "step %q rollback status", id)
				}
			}
		}
	}
}

// normalizeOrder returns a non-nil empty slice for an empty recording so it
// compares equal to a fixture's explicit [] rather than requiring null.
func normalizeOrder(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func TestConformance_Behavior(t *testing.T) {
	fixtures := loadFixtures(t, conformanceBehaviorDir)
	for path, data := range fixtures {
		var fx behaviorFixture
		require.NoErrorf(t, json.Unmarshal(data, &fx), "decode %s", path)
		name := fx.Name
		if name == "" {
			name = filepath.Base(path)
		}
		t.Run(name, func(t *testing.T) {
			runBehaviorFixture(t, fx)
		})
	}
}

// ---------------------------------------------------------------------------
// Serialization fixtures
// ---------------------------------------------------------------------------

type serializationFixture struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SpecRefs    []string        `json:"specRefs"`
	Kind        string          `json:"kind"` // statebag
	JSON        json.RawMessage `json:"json"`
}

func TestConformance_Serialization(t *testing.T) {
	fixtures := loadFixtures(t, conformanceSerializationDir)
	for path, data := range fixtures {
		var fx serializationFixture
		require.NoErrorf(t, json.Unmarshal(data, &fx), "decode %s", path)
		name := fx.Name
		if name == "" {
			name = filepath.Base(path)
		}
		t.Run(name, func(t *testing.T) {
			switch fx.Kind {
			case "statebag":
				var bag automa.SyncNamespacedStateBag
				require.NoError(t, json.Unmarshal(fx.JSON, &bag), "load state bag")
				out, err := json.Marshal(&bag)
				require.NoError(t, err, "re-serialize state bag")
				assertJSONEqual(t, fx.JSON, out)
			case "report":
				var rep automa.Report
				require.NoError(t, json.Unmarshal(fx.JSON, &rep), "load report")
				out, err := json.Marshal(&rep)
				require.NoError(t, err, "re-serialize report")
				assertJSONEqual(t, fx.JSON, out)
			default:
				t.Fatalf("unknown serialization fixture kind %q", fx.Kind)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// loadFixtures reads every *.json file in dir and returns path -> contents.
// It fails the test if the directory is missing or contains no fixtures, so a
// mis-placed fixture directory cannot silently pass conformance.
func loadFixtures(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoErrorf(t, err, "read fixture dir %s", dir)

	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		require.NoErrorf(t, err, "read fixture %s", p)
		out[p] = data
	}
	require.NotEmptyf(t, out, "no fixtures found in %s", dir)
	return out
}

// assertJSONEqual compares two JSON documents structurally (key order and
// insignificant whitespace are ignored).
func assertJSONEqual(t *testing.T, want, got []byte) {
	t.Helper()
	var wantV, gotV interface{}
	require.NoError(t, json.Unmarshal(want, &wantV), "decode expected JSON")
	require.NoError(t, json.Unmarshal(got, &gotV), "decode produced JSON")
	assert.Equal(t, wantV, gotV, "JSON round-trip mismatch\nexpected: %s\nproduced: %s", want, got)
}
