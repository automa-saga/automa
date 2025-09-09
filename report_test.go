package automa

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

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
	meta := map[string]string{"foo": "bar"}
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
		WithMetadata(map[string]string{"k": "v"}),
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
