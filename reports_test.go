package automa

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestWorkflowReport_Append(t *testing.T) {
	workflowReport := NewWorkflowReport("test", nil)
	stepReport := NewStepReport("step-1", RunAction)
	workflowReport.Append(stepReport, RunAction, StatusSuccess)
	assert.Equal(t, 1, len(workflowReport.StepReports))
	assert.Equal(t, 0, len(workflowReport.StepSequence))
	assert.NotNil(t, workflowReport.StepReports[stepReport.StepID])
	assert.Equal(t, StatusSuccess, workflowReport.StepReports[stepReport.StepID].Actions[RunAction].Status)

	workflowReport = &WorkflowReport{}
	workflowReport.Append(stepReport, RunAction, StatusSuccess)
	assert.Equal(t, 0, len(workflowReport.StepSequence))
	assert.Equal(t, 1, len(workflowReport.StepReports))
}

func TestReportYAML(t *testing.T) {
	stepReport1 := NewStepReport("step-1", RunAction)
	stepReport2 := NewStepReport("step-2", RunAction)
	workflowReport := NewWorkflowReport("workflow-id", []string{stepReport1.StepID, stepReport2.StepID})
	workflowReport.Append(stepReport1, RunAction, StatusSuccess)
	workflowReport.Append(stepReport2, RunAction, StatusSuccess)
	workflowReport.Append(stepReport2, RollbackAction, StatusSuccess)
	workflowReport.Append(stepReport1, RollbackAction, StatusSuccess)

	out, err := yaml.Marshal(workflowReport)
	assert.NoError(t, err)
	assert.NotNil(t, out)
}
