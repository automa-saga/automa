package automa

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkflowReport_InitializesFields(t *testing.T) {
	stepIDs := []string{"a", "b"}
	wfr := NewWorkflowReport("wf1", stepIDs)

	assert.Equal(t, "wf1", wfr.WorkflowID)
	assert.Equal(t, StatusInitialized, wfr.Status)
	assert.Equal(t, stepIDs, wfr.StepSequence)
	assert.NotZero(t, wfr.StartTime)
	assert.NotZero(t, wfr.EndTime)
	assert.Empty(t, wfr.StepReports)
	assert.Nil(t, wfr.FirstFailureOnForward)
	assert.Nil(t, wfr.LastFailureOnReverse)
}

func TestNewWorkflowReport_NilStepIDs(t *testing.T) {
	wfr := NewWorkflowReport("wf2", nil)
	assert.NotNil(t, wfr.StepSequence)
	assert.Empty(t, wfr.StepSequence)
}

func TestNewStepReport_InitializesFields(t *testing.T) {
	sr := NewStepReport("step1", "run")
	assert.Equal(t, "step1", sr.StepID)
	assert.Equal(t, "run", sr.Action)
	assert.Equal(t, StatusInitialized, sr.Status)
	assert.NotZero(t, sr.StartTime)
	assert.NotZero(t, sr.EndTime)
	assert.Nil(t, sr.Error)
	assert.NotNil(t, sr.Metadata)
	assert.Empty(t, sr.Metadata)
}

func TestWorkflowReport_Append_AddsStepReport(t *testing.T) {
	wfr := NewWorkflowReport("wf", []string{"a"})
	sr := NewStepReport("a", "run")
	start := time.Now()
	wfr.Append(sr, StatusSuccess)

	assert.Len(t, wfr.StepReports, 1)
	assert.Equal(t, sr, wfr.StepReports[0])
	assert.Equal(t, StatusSuccess, sr.Status)
	assert.True(t, sr.EndTime.After(start) || sr.EndTime.Equal(start))
}

func TestWorkflowReport_Append_NilStepReport(t *testing.T) {
	wfr := NewWorkflowReport("wf", nil)
	wfr.Append(nil, StatusSuccess)
	assert.Empty(t, wfr.StepReports)
}

func TestStepReport_ErrorAndMetadata(t *testing.T) {
	sr := NewStepReport("x", "run")
	err := errors.New("fail")
	sr.Error = err
	sr.Metadata["foo"] = []byte("bar")

	assert.Equal(t, err, sr.Error)
	assert.Equal(t, []byte("bar"), sr.Metadata["foo"])
}
