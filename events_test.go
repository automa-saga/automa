package automa

import (
	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewSkippedRun(t *testing.T) {
	prevSuccess := &SuccessEvent{WorkflowReport: WorkflowReport{
		WorkflowID:   "test",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       "",
		StepSequence: []string{},
		StepReports:  []*StepReport{},
	}}

	success := NewSkippedRunEvent(prevSuccess, nil)
	assert.NotNil(t, success)
	assert.NotNil(t, success.WorkflowReport)
	assert.Equal(t, 0, len(success.WorkflowReport.StepReports))

	report := NewStepReport("TEST", RunAction)
	success = NewSkippedRunEvent(prevSuccess, report)
	assert.NotNil(t, success)
	assert.NotNil(t, success.WorkflowReport)
	assert.Equal(t, 1, len(success.WorkflowReport.StepReports))
}

func TestNewSkippedRollback(t *testing.T) {
	prevFailure := &FailureEvent{
		Err: errorx.IllegalState.New("Test"),
		WorkflowReport: WorkflowReport{
			WorkflowID:   "test",
			StartTime:    time.Now(),
			EndTime:      time.Now(),
			Status:       "",
			StepSequence: []string{},
			StepReports:  []*StepReport{},
		},
	}
	failure := NewSkippedRollbackEvent(prevFailure, nil)
	assert.NotNil(t, failure)
	assert.NotNil(t, failure.WorkflowReport)
	assert.Equal(t, 0, len(failure.WorkflowReport.StepReports))

	report := NewStepReport("TEST", RunAction)
	failure = NewSkippedRollbackEvent(prevFailure, report)
	assert.NotNil(t, failure)
	assert.NotNil(t, failure.WorkflowReport)
	assert.Equal(t, 1, len(failure.WorkflowReport.StepReports))
}
