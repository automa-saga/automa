package automa

import (
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewSkippedRun(t *testing.T) {
	prevSuccess := &Success{workflowReport: &WorkflowReport{
		WorkflowID:   "test",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       "",
		StepSequence: []string{},
		StepReports:  []*StepReport{},
	}}

	success := NewSkippedRun(prevSuccess, nil)
	assert.NotNil(t, success)
	assert.NotNil(t, success.workflowReport)
	assert.Equal(t, 0, len(success.workflowReport.StepReports))

	report := NewStepReport("TEST", RunAction)
	success = NewSkippedRun(prevSuccess, report)
	assert.NotNil(t, success)
	assert.NotNil(t, success.workflowReport)
	assert.Equal(t, 1, len(success.workflowReport.StepReports))
}

func TestNewSkippedRollback(t *testing.T) {
	prevFailure := &Failure{
		error: errors.New("Test"),
		workflowReport: &WorkflowReport{
			WorkflowID:   "test",
			StartTime:    time.Now(),
			EndTime:      time.Now(),
			Status:       "",
			StepSequence: []string{},
			StepReports:  []*StepReport{},
		},
	}
	failure := NewSkippedRollback(prevFailure, nil)
	assert.NotNil(t, failure)
	assert.NotNil(t, failure.workflowReport)
	assert.Equal(t, 0, len(failure.workflowReport.StepReports))

	report := NewStepReport("TEST", RunAction)
	failure = NewSkippedRollback(prevFailure, report)
	assert.NotNil(t, failure)
	assert.NotNil(t, failure.workflowReport)
	assert.Equal(t, 1, len(failure.workflowReport.StepReports))
}
