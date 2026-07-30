package automa

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestNewReport_Defaults(t *testing.T) {
	r := NewReport("id1")
	assert.Equal(t, "id1", r.Id)
	assert.Equal(t, StatusSuccess, r.Status)
	assert.WithinDuration(t, time.Now(), r.StartTime, time.Second)
	assert.WithinDuration(t, time.Now(), r.EndTime, time.Second)
}

func TestReportOptions(t *testing.T) {
	meta := StringMap{"foo": "bar"}
	err := errors.New("fail")
	start := time.Now().Add(-time.Minute)
	end := time.Now()
	step := NewReport("step1")
	rollback := NewReport("rb")

	r := NewReport("id2",
		WithMetadata(meta),
		WithError(err),
		WithStatus(StatusFailed),
		WithStartTime(start),
		WithEndTime(end),
		WithDetail("detail"),
		WithStepReports(step),
		WithRollbackReport(rollback),
		WithActionType(ActionRollback),
	)

	assert.Equal(t, meta, r.Metadata)
	assert.Equal(t, err, r.Error)
	assert.Equal(t, StatusFailed, r.Status)
	assert.Equal(t, start, r.StartTime)
	assert.Equal(t, end, r.EndTime)
	assert.Equal(t, "detail", r.Detail)
	assert.Len(t, r.StepReports, 1)
	assert.Equal(t, rollback, r.Rollback)
	assert.Equal(t, ActionRollback, r.Action)
}

func TestWithReport_MergesFields(t *testing.T) {
	src := NewReport("src",
		WithDetail("d"),
		WithActionType(ActionRollback),
		WithStartTime(time.Now().Add(-2*time.Minute)),
		WithEndTime(time.Now().Add(-1*time.Minute)),
		WithStatus(StatusFailed),
		WithError(errors.New("err")),
		WithMetadata(StringMap{"k": "v"}),
		WithStepReports(NewReport("s1")),
		WithRollbackReport(NewReport("rb")),
	)
	dst := NewReport("dst")
	WithReport(src)(dst)

	assert.Equal(t, src.Detail, dst.Detail)
	assert.Equal(t, src.Action, dst.Action)
	assert.Equal(t, src.StartTime, dst.StartTime)
	assert.Equal(t, src.EndTime, dst.EndTime)
	assert.Equal(t, src.Status, dst.Status)
	assert.Equal(t, src.Error.Error(), dst.Error.Error())
	assert.Equal(t, src.Metadata, dst.Metadata)
	assert.Len(t, dst.StepReports, 1)
	assert.Equal(t, src.Rollback, dst.Rollback)
}

func TestStepReportHelpers(t *testing.T) {
	r1 := StepSuccessReport("s1")
	assert.Equal(t, StatusSuccess, r1.Status)
	r2 := StepFailureReport("s2")
	assert.Equal(t, StatusFailed, r2.Status)
	r3 := StepSkippedReport("s3")
	assert.Equal(t, StatusSkipped, r3.Status)
}

func TestReport_MarshalJSON(t *testing.T) {
	err := errors.New("fail")
	r := NewReport("id", WithError(err), WithDetail("d"), WithStatus(StatusFailed))
	b, e := json.Marshal(r)
	assert.NoError(t, e)
	assert.Contains(t, string(b), `"id":"id"`)
	assert.Contains(t, string(b), `"detail":"d"`)
	assert.Contains(t, string(b), `"status":"failed"`)
	assert.Contains(t, string(b), `"error":"fail"`)
}

func TestReport_MarshalYAML(t *testing.T) {
	err := errors.New("fail")
	r := NewReport("id", WithError(err), WithDetail("d"), WithStatus(StatusFailed))
	out, e := r.MarshalYAML()
	assert.NoError(t, e)
	m, ok := out.(marshalReport)
	assert.True(t, ok)
	assert.Equal(t, "id", m.Id)
	assert.Equal(t, "fail", m.Error)
	assert.Equal(t, "d", m.Detail)
	assert.Equal(t, StatusFailed, m.Status)
}

func TestReport_MarshalYAML_Integration(t *testing.T) {
	r := NewReport("id", WithDetail("d"))
	b, err := yaml.Marshal(r)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "id: id")
	assert.Contains(t, string(b), "detail: d")
}

func TestReport_Duration(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	end := time.Now()
	r := NewReport("id", WithStartTime(start), WithEndTime(end))
	assert.Equal(t, end.Sub(start), r.Duration())
}

func TestReport_StatusMethods(t *testing.T) {
	r := NewReport("id", WithStatus(StatusSuccess))
	assert.True(t, r.IsSuccess())
	assert.False(t, r.IsFailed())
	assert.False(t, r.IsSkipped())

	r.Status = StatusFailed
	assert.True(t, r.IsFailed())
	assert.False(t, r.IsSuccess())

	r.Status = StatusSkipped
	assert.True(t, r.IsSkipped())
}

func TestReport_HasError(t *testing.T) {
	r := NewReport("id")
	assert.False(t, r.HasError())
	r.Error = errors.New("err")
	assert.True(t, r.HasError())
}

func TestReport_WithIsWorkflowAndActionType(t *testing.T) {
	r := NewReport("id", WithIsWorkflow(true), WithActionType(ActionExecute))
	assert.True(t, r.IsWorkflow)
	assert.Equal(t, ActionExecute, r.Action)
}

func TestReport_Clone(t *testing.T) {
	meta := StringMap{"foo": "bar"}
	step := NewReport("step1", WithDetail("step detail"))
	rollback := NewReport("rb", WithDetail("rollback detail"))
	r := NewReport("id",
		WithMetadata(meta),
		WithStatus(StatusFailed),
		WithDetail("detail"),
		WithStepReports(step),
		WithRollbackReport(rollback),
	)

	clone := r.Clone()
	assert.Equal(t, r.Id, clone.Id)
	assert.Equal(t, r.Status, clone.Status)
	assert.Equal(t, r.Detail, clone.Detail)
	assert.Equal(t, r.Metadata, clone.Metadata)
	assert.Len(t, clone.StepReports, 1)
	assert.NotEqual(t, unsafe.Pointer(&r.Metadata), unsafe.Pointer(&clone.Metadata))
	assert.Equal(t, "step detail", clone.StepReports[0].Detail)
	assert.NotSame(t, r.StepReports[0], clone.StepReports[0])
	assert.NotNil(t, clone.Rollback)
	assert.Equal(t, "rollback detail", clone.Rollback.Detail)
	assert.NotSame(t, r.Rollback, clone.Rollback)

	// Mutate clone and check original is unchanged
	clone.Metadata["foo"] = "baz"
	assert.Equal(t, "bar", r.Metadata["foo"])
	clone.StepReports[0].Detail = "changed"
	assert.Equal(t, "step detail", r.StepReports[0].Detail)
}

func TestNewReport_DefaultModes(t *testing.T) {
	r := NewReport("id")
	assert.Equal(t, StopOnError, r.ExecutionMode)
	assert.Equal(t, ContinueOnError, r.RollbackMode)
}

func TestWithExecuteAndRollbackModeOptions(t *testing.T) {
	r := NewReport("id", WithExecutionMode(ContinueOnError), WithRollbackMode(StopOnError))
	assert.Equal(t, ContinueOnError, r.ExecutionMode)
	assert.Equal(t, StopOnError, r.RollbackMode)
}

func TestWithWorkflow_SetsModes(t *testing.T) {
	wf := &workflow{executionMode: ContinueOnError, rollbackMode: StopOnError}
	r := NewReport("id", WithWorkflow(wf))
	assert.Equal(t, ContinueOnError, r.ExecutionMode)
	assert.Equal(t, StopOnError, r.RollbackMode)
}

func TestReport_MarshalJSON_IncludesModes(t *testing.T) {
	r := NewReport("id", WithExecutionMode(ContinueOnError), WithRollbackMode(StopOnError))
	b, err := json.Marshal(r)
	assert.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, "executionMode")
	assert.Contains(t, s, "rollbackMode")
}

func TestReport_Clone_PreservesModes(t *testing.T) {
	r := NewReport("id", WithExecutionMode(ContinueOnError), WithRollbackMode(StopOnError))
	clone := r.Clone()
	assert.Equal(t, r.ExecutionMode, clone.ExecutionMode)
	assert.Equal(t, r.RollbackMode, clone.RollbackMode)
}

// TestReport_UnmarshalJSON_RoundTrip verifies a report tree survives a
// marshal → unmarshal → marshal cycle unchanged, including nested step reports,
// an inline rollback sub-report, timestamps, and error-as-string.
func TestReport_UnmarshalJSON_RoundTrip(t *testing.T) {
	start := time.Date(2026, 6, 17, 10, 30, 0, 123000000, time.UTC)
	end := time.Date(2026, 6, 17, 10, 30, 1, 456000000, time.UTC)

	original := &Report{
		Id:            "wf",
		IsWorkflow:    true,
		Action:        ActionExecute,
		Status:        StatusFailed,
		StartTime:     start,
		EndTime:       end,
		Error:         errors.New("boom"),
		ExecutionMode: RollbackOnError,
		RollbackMode:  ContinueOnError,
		StepReports: []*Report{
			{
				Id:        "s1",
				Action:    ActionExecute,
				Status:    StatusSuccess,
				StartTime: start,
				EndTime:   end,
				Rollback: &Report{
					Action:    ActionRollback,
					Status:    StatusSuccess,
					StartTime: start,
					EndTime:   end,
				},
			},
		},
	}

	firstPass, err := json.Marshal(original)
	assert.NoError(t, err)

	var loaded Report
	assert.NoError(t, json.Unmarshal(firstPass, &loaded))

	// Error is reconstructed from its string form.
	assert.Equal(t, "boom", loaded.Error.Error())
	assert.Equal(t, ActionExecute, loaded.Action)
	assert.Equal(t, StatusFailed, loaded.Status)
	assert.True(t, loaded.StartTime.Equal(start))
	if assert.Len(t, loaded.StepReports, 1) {
		assert.Equal(t, "s1", loaded.StepReports[0].Id)
		assert.NotNil(t, loaded.StepReports[0].Rollback)
	}

	secondPass, err := json.Marshal(&loaded)
	assert.NoError(t, err)
	assert.JSONEq(t, string(firstPass), string(secondPass))
}

// TestReport_UnmarshalYAML_RoundTrip verifies the YAML unmarshaler mirrors the
// JSON one.
func TestReport_UnmarshalYAML_RoundTrip(t *testing.T) {
	original := &Report{
		Id:        "s1",
		Action:    ActionExecute,
		Status:    StatusFailed,
		StartTime: time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 6, 17, 10, 30, 1, 0, time.UTC),
		Error:     errors.New("kaboom"),
	}

	firstPass, err := yaml.Marshal(original)
	assert.NoError(t, err)

	var loaded Report
	assert.NoError(t, yaml.Unmarshal(firstPass, &loaded))
	assert.Equal(t, "kaboom", loaded.Error.Error())
	assert.Equal(t, StatusFailed, loaded.Status)

	secondPass, err := yaml.Marshal(&loaded)
	assert.NoError(t, err)

	// Compare structurally, not as raw text: YAML has no guaranteed stable
	// textual form across versions/formatting.
	var firstV, secondV interface{}
	assert.NoError(t, yaml.Unmarshal(firstPass, &firstV))
	assert.NoError(t, yaml.Unmarshal(secondPass, &secondV))
	assert.Equal(t, firstV, secondV)
}
